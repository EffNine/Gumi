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

const openAICompatibleDefaultURL = "http://localhost:8000/v1"

// OpenAICompatibleAdapter implements ProviderAdapter for a local server that
// exposes OpenAI-compatible /v1 endpoints. It does not support cloud OpenAI.
type OpenAICompatibleAdapter struct {
	name    string
	baseURL string
	timeout time.Duration
	client  *http.Client
	log     *logger.Logger
}

// NewOpenAICompatibleAdapter creates an adapter from settings.
func NewOpenAICompatibleAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
	baseURL := settings.URL
	if baseURL == "" {
		baseURL = openAICompatibleDefaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := time.Duration(settings.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &OpenAICompatibleAdapter{
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
func (o *OpenAICompatibleAdapter) Name() string {
	return o.name
}

// Type returns the adapter type.
func (o *OpenAICompatibleAdapter) Type() string {
	return "openai-compatible-local"
}

// Capabilities reports adapter capabilities.
func (o *OpenAICompatibleAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming:        false,
		ToolUse:          false,
		StructuredOutput: false,
	}
}

// apiPath returns the full URL for a /v1 suffix, respecting the configured base URL.
func (o *OpenAICompatibleAdapter) apiPath(suffix string) string {
	if strings.HasSuffix(o.baseURL, "/v1") {
		return o.baseURL + suffix
	}
	return o.baseURL + "/v1" + suffix
}

// HealthCheck probes the local OpenAI-compatible server via /models.
func (o *OpenAICompatibleAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
	url := o.apiPath("/models")
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
		return StatusDegraded, fmt.Errorf("openai-compatible health check returned status %d", resp.StatusCode)
	}

	return StatusOK, nil
}

// ListModels returns the models available on the local server.
func (o *OpenAICompatibleAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := o.apiPath("/models")
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
		return nil, fmt.Errorf("openai-compatible model list returned status %d", resp.StatusCode)
	}

	var list api.ModelsList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(list.Data))
	for _, m := range list.Data {
		models = append(models, ModelInfo{
			Name:     m.ID,
			Provider: o.name,
		})
	}
	return models, nil
}

// Generate calls the local /chat/completions endpoint.
func (o *OpenAICompatibleAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	url := o.apiPath("/chat/completions")

	body, err := json.Marshal(req)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to marshal openai-compatible request",
			Cause:   err,
		}
	}

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
		return nil, NormalizeHTTPError(resp.StatusCode, nil, "openai-compatible")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to read openai-compatible response body",
			Cause:   err,
		}
	}

	var chatResp api.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to decode openai-compatible response",
			Cause:   err,
		}
	}

	return &chatResp, nil
}

// NormalizeError maps an error to a normalized provider error.
func (o *OpenAICompatibleAdapter) NormalizeError(err error) ProviderError {
	if err == nil {
		return ProviderError{}
	}

	var pe ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	return classifyNetworkError(err, "openai-compatible")
}
