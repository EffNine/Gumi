package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

const ollamaDefaultURL = "http://localhost:11434"

// OllamaAdapter implements ProviderAdapter for a local Ollama server.
type OllamaAdapter struct {
	name    string
	baseURL string
	timeout time.Duration
	client  *http.Client
	log     *logger.Logger
}

// NewOllamaAdapter creates an Ollama adapter from settings.
func NewOllamaAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
	baseURL := settings.URL
	if baseURL == "" {
		baseURL = ollamaDefaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := time.Duration(settings.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &OllamaAdapter{
		name:    name,
		baseURL: baseURL,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		log: log,
	}, nil
}

// Name returns the provider key.
func (o *OllamaAdapter) Name() string {
	return o.name
}

// Type returns the adapter type.
func (o *OllamaAdapter) Type() string {
	return "ollama"
}

// Capabilities reports Ollama capabilities.
func (o *OllamaAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming:        false,
		ToolUse:          false,
		StructuredOutput: false,
	}
}

// HealthCheck probes Ollama by listing models.
func (o *OllamaAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
	url := o.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StatusMisconfigured, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return StatusOffline, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusDegraded, fmt.Errorf("ollama health check returned status %d", resp.StatusCode)
	}

	return StatusOK, nil
}

// ollamaModel is the model shape returned by /api/tags.
type ollamaModel struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
}

// ollamaListResponse is the response from /api/tags.
type ollamaListResponse struct {
	Models []ollamaModel `json:"models"`
}

// ListModels returns the models available on the Ollama server.
func (o *OllamaAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := o.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama model list returned status %d", resp.StatusCode)
	}

	var list ollamaListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(list.Models))
	for _, m := range list.Models {
		models = append(models, ModelInfo{
			Name:      m.Name,
			Provider:  o.name,
			CreatedAt: m.ModifiedAt,
		})
	}
	return models, nil
}

// ollamaMessage mirrors a chat message in Ollama format.
type ollamaMessage struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}

// ollamaChatRequest is the request body for /api/chat.
type ollamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Think    *bool                  `json:"think,omitempty"`
}

// ollamaChatResponse is the non-streaming response from /api/chat.
type ollamaChatResponse struct {
	Model   string        `json:"model"`
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// Generate calls the Ollama chat endpoint.
func (o *OllamaAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	messages := make([]ollamaMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		content, err := messageContentToString(m.Content)
		if err != nil {
			return nil, ProviderError{
				Code:    ProviderBadResponse,
				Message: fmt.Sprintf("unsupported message content type: %v", err),
				Cause:   err,
			}
		}
		messages = append(messages, ollamaMessage{
			Role:    m.Role,
			Content: content,
		})
	}

	options := make(map[string]interface{})
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		options["top_p"] = *req.TopP
	}
	if req.MaxTokens != nil {
		options["num_predict"] = *req.MaxTokens
	}

	payload := ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
		Options:  options,
	}

	// Resolve thinking from request-level novexa extension.
	if req.Novexa != nil && req.Novexa.Thinking != nil && req.Novexa.Thinking.Enabled != nil {
		payload.Think = req.Novexa.Thinking.Enabled
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to marshal Ollama request",
			Cause:   err,
		}
	}

	url := o.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, NormalizeHTTPError(resp.StatusCode, nil, "ollama")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to read Ollama response body",
			Cause:   err,
		}
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to decode Ollama response",
			Cause:   err,
		}
	}

	content := ollamaResp.Message.Content
	thinking := ollamaResp.Message.Thinking

	// If the model returned empty content but non-empty thinking, the model
	// exhausted its output budget on reasoning. Return a clear actionable error.
	if strings.TrimSpace(content) == "" && strings.TrimSpace(thinking) != "" {
		return nil, ProviderError{
			Code:       ValidationFailed,
			Message:    "model exhausted output tokens on reasoning and returned an empty final answer",
			Suggestion: "Increase max_tokens or disable thinking via novexa.thinking.enabled=false.",
		}
	}

	return &api.ChatCompletionResponse{
		ID:      "chatcmpl_ollama_" + randomID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.Choice{
			{
				Index: 0,
				Message: api.Message{
					Role:             "assistant",
					Content:          content,
					ReasoningContent: thinking,
				},
				FinishReason: "stop",
			},
		},
		Usage: api.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}, nil
}

// NormalizeError maps an error to a normalized provider error.
func (o *OllamaAdapter) NormalizeError(err error) ProviderError {
	if err == nil {
		return ProviderError{}
	}

	var pe ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	return classifyNetworkError(err, "ollama")
}
