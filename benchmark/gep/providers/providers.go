// Package providers defines the provider interface and concrete adapters for GEP.
package providers

import (
	"context"
	"net/http"
	"time"
)

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is a standard chat completion request supported by all GEP providers.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

// ChatChoice represents a single completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatResponse is a standard chat completion response from any GEP provider.
type ChatResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *Usage       `json:"usage,omitempty"`
}

// Usage holds token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Provider is the interface that all GEP provider adapters must implement.
type Provider interface {
	// ChatCompletion sends a chat request and returns the response.
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Health checks whether the provider is reachable and responsive.
	Health(ctx context.Context) error

	// Type returns the provider type identifier.
	Type() string

	// Name returns a human-readable provider name.
	Name() string
}

// NewProvider creates a provider instance based on the given type and URL.
// Supported types: "lmstudio", "ollama".
func NewProvider(pType, baseURL, apiKey string) (Provider, error) {
	switch pType {
	case "lmstudio":
		return NewLMStudioProvider(baseURL, apiKey), nil
	case "ollama":
		return NewOllamaProvider(baseURL, apiKey), nil
	default:
		return nil, ErrUnsupportedProvider
	}
}

// ErrUnsupportedProvider is returned when an unknown provider type is requested.
var ErrUnsupportedProvider = &ProviderError{ProviderType: "unknown"}

// ProviderError describes an error that occurred within a provider adapter.
type ProviderError struct {
	ProviderType string
	Message      string
	Err          error
}

func (e *ProviderError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "provider error"
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// HTTPTimeout is the default timeout for provider HTTP requests.
var HTTPTimeout = 120 * time.Second

// newHTTPClient returns an http.Client with the default timeout.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: HTTPTimeout,
	}
}
