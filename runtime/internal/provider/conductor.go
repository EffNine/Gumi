// Package provider implements thin adapters that connect the Gumi gateway
// to inference providers.

package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

const conductorDefaultURL = "https://conductor-yknfkg.fly.dev/v1"

// ConductorAdapter implements ProviderAdapter for the Conductor aggregator
// endpoint. Conductor is the user's personal OpenAI-compatible free-endpoint
// aggregator that provides access to multiple free model providers (Poolside
// Laguna, Kimi, DeepSeek, etc.) through a single OpenAI-compatible endpoint.
type ConductorAdapter struct {
	name    string
	baseURL string
	apiKey  string
	timeout time.Duration
	client  *http.Client
	log     *logger.Logger
}

// NewConductorAdapter creates a Conductor adapter from settings.
func NewConductorAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
	baseURL := settings.URL
	if baseURL == "" {
		baseURL = conductorDefaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := time.Duration(settings.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 600 * time.Second
	}

	return &ConductorAdapter{
		name:    name,
		baseURL: baseURL,
		apiKey:  settings.APIKey,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider key.
func (c *ConductorAdapter) Name() string {
	return c.name
}

// Type returns the adapter type.
func (c *ConductorAdapter) Type() string {
	return "conductor"
}

// Capabilities reports adapter capabilities. Conductor proxies frontier models
// that support streaming, tool use, and structured output.
func (c *ConductorAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming:        true,
		ToolUse:          true,
		StructuredOutput: true,
	}
}

// apiPath returns the full URL for a /v1 suffix, respecting the configured base URL.
func (c *ConductorAdapter) apiPath(suffix string) string {
	if strings.HasSuffix(c.baseURL, "/v1") {
		return c.baseURL + suffix
	}
	return c.baseURL + "/v1" + suffix
}

// authHeader sets the Authorization header if an API key is configured.
func (c *ConductorAdapter) authHeader(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// HealthCheck probes the Conductor endpoint via /models.
func (c *ConductorAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
	url := c.apiPath("/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StatusMisconfigured, err
	}
	c.authHeader(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return StatusOffline, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusDegraded, fmt.Errorf("conductor health check returned status %d", resp.StatusCode)
	}

	return StatusOK, nil
}

// ListModels returns the models available through the Conductor endpoint.
func (c *ConductorAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := c.apiPath("/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.authHeader(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("conductor model list returned status %d", resp.StatusCode)
	}

	var list api.ModelsList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(list.Data))
	for _, m := range list.Data {
		models = append(models, ModelInfo{
			Name:     m.ID,
			Provider: c.name,
		})
	}
	return models, nil
}

// Generate calls the Conductor /chat/completions endpoint.
func (c *ConductorAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	url := c.apiPath("/chat/completions")

	body, err := json.Marshal(req)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to marshal conductor request",
			Cause:   err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.authHeader(httpReq)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, NormalizeHTTPError(resp.StatusCode, nil, "conductor")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to read conductor response body",
			Cause:   err,
		}
	}

	var chatResp api.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to decode conductor response",
			Cause:   err,
		}
	}

	// Strip reasoning content from non-streaming responses.
	// Conductor-proxied reasoning models (DeepSeek, OpenCode, etc.) return
	// thinking tokens in both content and reasoning_content fields.
	// We strip the reasoning prefix from content so Gumi's validation
	// can assess the actual answer.
	for i, choice := range chatResp.Choices {
		if content, ok := choice.Message.Content.(string); ok {
			// If content is identical to reasoning_content, the model
			// is still in reasoning mode and hasn't produced an answer.
			if choice.Message.ReasoningContent != "" && content == choice.Message.ReasoningContent {
				stripped := stripThinkingPrefix(content)
				if stripped != content {
					chatResp.Choices[i].Message.Content = stripped
				}
				// Clear reasoning_content so pipeline won't double it.
				chatResp.Choices[i].Message.ReasoningContent = ""
			} else {
				chatResp.Choices[i].Message.Content = stripThinkingPrefix(content)
			}
		}
	}

	return &chatResp, nil
}

// GenerateStream performs a streaming chat completion via SSE through Conductor.
func (c *ConductorAdapter) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.ChatCompletionChunk, <-chan error, error) {
	url := c.apiPath("/chat/completions")

	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to marshal conductor streaming request",
			Cause:   err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	c.authHeader(httpReq)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, NormalizeHTTPError(resp.StatusCode, nil, "conductor")
	}

	chunkCh := make(chan api.ChatCompletionChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				errCh <- nil
				return
			}

			var chunk api.ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				c.log.Debug("conductor: skipping malformed SSE chunk", "error", err)
				continue
			}

			select {
			case chunkCh <- chunk:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- ProviderError{
				Code:    ProviderBadResponse,
				Message: "conductor SSE stream read error",
				Cause:   err,
			}
			return
		}

		errCh <- nil
	}()

	return chunkCh, errCh, nil
}

// NormalizeError maps an error to a normalized provider error.
func (c *ConductorAdapter) NormalizeError(err error) ProviderError {
	if err == nil {
		return ProviderError{}
	}

	var pe ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	return classifyNetworkError(err, "conductor")
}

// stripThinkingPrefix removes reasoning/thinking prefixes from model responses.
// Many Conductor-proxied models (DeepSeek, OpenCode, NVIDIA NIM) return
// thinking tokens that leak into the content field. We detect common
// reasoning prefixes and strip everything before the actual response begins.
func stripThinkingPrefix(content string) string {
	// If content is short or doesn't start with a reasoning pattern, return as-is.
	if len(content) < 10 {
		return content
	}

	// Pattern 1: "Thinking." prefix (DeepSeek style)
	// "Thinking. 1. **Analyze the Request:**\n    *   Target: ...\n\nThe answer is..."
	if strings.HasPrefix(content, "Thinking") || strings.HasPrefix(content, "thinking") {
		idx := strings.Index(content, "\n\n")
		if idx > 0 && idx < len(content)-2 {
			rest := strings.TrimSpace(content[idx+2:])
			if rest != "" {
				return rest
			}
		}
		// Fallback: look for ".\n" after "Thinking."
		idx = strings.Index(content, ".\n")
		if idx > 0 && idx < len(content)-2 {
			rest := strings.TrimSpace(content[idx+2:])
			if rest != "" {
				return rest
			}
		}
	}

	// Pattern 2: "Reasoning:" prefix
	if strings.HasPrefix(content, "Reasoning") || strings.HasPrefix(content, "reasoning") {
		idx := strings.Index(content, "\n\n")
		if idx > 0 && idx < len(content)-2 {
			rest := strings.TrimSpace(content[idx+2:])
			if rest != "" {
				return rest
			}
		}
	}

	// Pattern 3: "Let me think..." / "Let's think..." / "Let me analyze..."
	if strings.HasPrefix(content, "Let me") || strings.HasPrefix(content, "Let's") {
		idx := strings.Index(content, "\n\n")
		if idx > 0 && idx < len(content)-2 {
			rest := strings.TrimSpace(content[idx+2:])
			if rest != "" {
				return rest
			}
		}
	}

	// Pattern 4: Content that is purely meta-cognitive — describing the user's
	// request in third person without providing an answer.
	// "The user wants..." / "We are asked to..." / "We need to..."
	// "The user asks: ... So answer: ..."
	if isMetaCognitivePrefix(content) {
		idx := strings.Index(content, "\n\n")
		if idx > 0 && idx < len(content)-2 {
			rest := strings.TrimSpace(content[idx+2:])
			if rest != "" {
				return rest
			}
		}
		// Look for answer transition markers that separate meta-cognitive
		// preamble from the actual response.
		if transition := findAnswerTransition(content); transition != "" {
			return transition
		}
		// For meta-cognitive without line breaks or transitions, check if
		// there's a sentence boundary (sentence-ending punctuation + space
		// or newline) that ends the preamble.
		sentenceEnd := findSentenceBoundary(content)
		if sentenceEnd > 0 && sentenceEnd < len(content)-2 {
			after := strings.TrimSpace(content[sentenceEnd:])
			if len(after) > 5 && !isMetaCognitivePrefix(after) {
				return after
			}
		}
	}

	return content
}

// findAnswerTransition looks for common answer-transition phrases in content
// and returns everything after the first one found.
func findAnswerTransition(content string) string {
	transitions := []string{
		"So answer:", "so answer:",
		"So respond:", "so respond:",
		"So the answer is:", "so the answer is:",
		"The answer is:", "the answer is:",
		"Therefore:", "therefore:",
		"Thus:", "thus:",
		"In short:", "in short:",
		"Answer:", "answer:",
	}
	lower := strings.ToLower(content)
	for _, t := range transitions {
		idx := strings.Index(lower, t)
		if idx >= 0 {
			after := strings.TrimSpace(content[idx+len(t):])
			if after != "" {
				return after
			}
		}
	}
	return ""
}

// findSentenceBoundary finds the end of the first sentence in content by
// looking for sentence-ending punctuation (.!?) followed by whitespace.
func findSentenceBoundary(content string) int {
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if ch == '.' || ch == '!' || ch == '?' {
			// Check what follows — whitespace, quote+space, or quote+newline
			if i+1 < len(content) {
				next := content[i+1]
				if next == ' ' || next == '\n' || next == '\r' || next == '	' {
					// Skip trailing chars and return the next non-space position
					j := i + 2
					for j < len(content) && (content[j] == ' ' || content[j] == '\n' || content[j] == '\r') {
						j++
					}
					return j
				}
				// Handle ."  (period followed by quote then space)
				if next == '"' || next == '\'' {
					if i+2 < len(content) {
						after := content[i+2]
						if after == ' ' || after == '\n' || after == '\r' {
							j := i + 3
							for j < len(content) && (content[j] == ' ' || content[j] == '\n' || content[j] == '\r') {
								j++
							}
							return j
						}
					}
				}
			}
		}
	}
	return -1
}

// isMetaCognitivePrefix checks if content starts with a meta-cognitive phrase
// indicating the model is reasoning about the request rather than answering it.
func isMetaCognitivePrefix(content string) bool {
	prefixes := []string{
		"The user", "the user",
		"User asks", "user asks",
		"We are", "we are",
		"We need", "we need",
		"I will", "I'll",
		"I need", "i need",
		"This is a", "this is a",
		"The instruction", "the instruction",
		"The request", "the request",
		"The prompt", "the prompt",
		"OK,", "Okay,", "okay,",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(content, p) {
			return true
		}
	}
	return false
}
