package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider is a GEP provider adapter for Ollama.
type OllamaProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider adapter.
// baseURL should be the Ollama API base URL (e.g., http://localhost:11434).
func NewOllamaProvider(baseURL, apiKey string) *OllamaProvider {
	return &OllamaProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: newHTTPClient(),
	}
}

// ChatCompletion sends a chat completion request to Ollama and returns the response.
func (p *OllamaProvider) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	ollamaReq := struct {
		Model    string                 `json:"model"`
		Messages []ChatMessage          `json:"messages,omitempty"`
		Prompt   string                 `json:"prompt,omitempty"`
		Stream   bool                   `json:"stream"`
		Options  map[string]interface{} `json:"options,omitempty"`
		Format   string                 `json:"format,omitempty"`
	}{}

	ollamaReq.Model = req.Model
	ollamaReq.Stream = false

	if len(req.Messages) > 0 {
		ollamaReq.Messages = req.Messages
	}

	if req.MaxTokens > 0 {
		if ollamaReq.Options == nil {
			ollamaReq.Options = make(map[string]interface{})
		}
		ollamaReq.Options["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		if ollamaReq.Options == nil {
			ollamaReq.Options = make(map[string]interface{})
		}
		ollamaReq.Options["temperature"] = req.Temperature
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat completion returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp struct {
		Message ChatMessage `json:"message"`
		Model   string      `json:"model"`
		Done    bool        `json:"done"`
	}
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("decoding Ollama response: %w", err)
	}

	return &ChatResponse{
		ID:    fmt.Sprintf("ollama-%d", time.Now().UnixNano()),
		Model: req.Model,
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ollamaResp.Message,
				FinishReason: "stop",
			},
		},
	}, nil
}

// Health checks whether the Ollama provider is reachable.
func (p *OllamaProvider) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("creating health check request: %w", err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// Type returns the provider type identifier.
func (p *OllamaProvider) Type() string {
	return "ollama"
}

// Name returns a human-readable provider name.
func (p *OllamaProvider) Name() string {
	return "Ollama"
}
