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

const lmStudioDefaultURL = "http://localhost:1234/v1"

// LMStudioAdapter implements ProviderAdapter for a local LM Studio server.
type LMStudioAdapter struct {
	name    string
	baseURL string
	timeout time.Duration
	client  *http.Client
	log     *logger.Logger
}

// NewLMStudioAdapter creates an LM Studio adapter from settings.
func NewLMStudioAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
	baseURL := settings.URL
	if baseURL == "" {
		baseURL = lmStudioDefaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := time.Duration(settings.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &LMStudioAdapter{
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
func (l *LMStudioAdapter) Name() string {
	return l.name
}

// Type returns the adapter type.
func (l *LMStudioAdapter) Type() string {
	return "lmstudio"
}

// Capabilities reports LM Studio capabilities.
func (l *LMStudioAdapter) Capabilities() Capabilities {
	return Capabilities{
		Streaming:        false,
		ToolUse:          false,
		StructuredOutput: false,
	}
}

// apiPath returns the correct endpoint path based on whether the base URL
// already includes /v1.
func (l *LMStudioAdapter) apiPath(suffix string) string {
	if strings.HasSuffix(l.baseURL, "/v1") {
		return l.baseURL + suffix
	}
	return l.baseURL + "/v1" + suffix
}

// HealthCheck probes LM Studio via the models endpoint.
func (l *LMStudioAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
	url := l.apiPath("/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StatusMisconfigured, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return StatusOffline, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusDegraded, fmt.Errorf("lmstudio health check returned status %d", resp.StatusCode)
	}

	return StatusOK, nil
}

// ListModels returns the models available on LM Studio.
func (l *LMStudioAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := l.apiPath("/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lmstudio model list returned status %d", resp.StatusCode)
	}

	var list api.ModelsList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, 0, len(list.Data))
	for _, m := range list.Data {
		models = append(models, ModelInfo{
			Name:     m.ID,
			Provider: l.name,
		})
	}
	return models, nil
}

type lmStudioChatRequest struct {
	Model            string              `json:"model"`
	Messages         []api.Message       `json:"messages"`
	Temperature      *float32            `json:"temperature,omitempty"`
	TopP             *float32            `json:"top_p,omitempty"`
	MaxTokens        *int                `json:"max_tokens,omitempty"`
	Stream           bool                `json:"stream,omitempty"`
	Stop             interface{}         `json:"stop,omitempty"`
	PresencePenalty  *float32            `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32            `json:"frequency_penalty,omitempty"`
	ResponseFormat   *api.ResponseFormat `json:"response_format,omitempty"`
	Tools            []api.Tool          `json:"tools,omitempty"`
	ToolChoice       interface{}         `json:"tool_choice,omitempty"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
	ReasoningEffort  string              `json:"reasoning_effort,omitempty"`
}

func newLMStudioChatRequest(req api.ChatCompletionRequest) lmStudioChatRequest {
	payload := lmStudioChatRequest{
		Model:            req.Model,
		Messages:         req.Messages,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		MaxTokens:        req.MaxTokens,
		Stream:           req.Stream,
		Stop:             req.Stop,
		PresencePenalty:  req.PresencePenalty,
		FrequencyPenalty: req.FrequencyPenalty,
		ResponseFormat:   lmStudioResponseFormat(req.ResponseFormat),
		Tools:            req.Tools,
		ToolChoice:       req.ToolChoice,
		Metadata:         req.Metadata,
	}

	if req.Novexa != nil && req.Novexa.Thinking != nil && req.Novexa.Thinking.Enabled != nil {
		if *req.Novexa.Thinking.Enabled {
			payload.ReasoningEffort = "medium"
		} else {
			payload.ReasoningEffort = "none"
		}
	}

	return payload
}

func lmStudioResponseFormat(format *api.ResponseFormat) *api.ResponseFormat {
	if format == nil {
		return nil
	}
	if format.Type != "json_object" {
		return format
	}
	return &api.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &api.JSONSchemaSpec{
			Name: "response",
			Schema: map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

// Generate calls the LM Studio chat completions endpoint.
func (l *LMStudioAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	url := l.apiPath("/chat/completions")

	body, err := json.Marshal(newLMStudioChatRequest(req))
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to marshal LM Studio request",
			Cause:   err,
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, NormalizeHTTPError(resp.StatusCode, nil, "lmstudio")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to read LM Studio response body",
			Cause:   err,
		}
	}

	var chatResp api.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, ProviderError{
			Code:    ProviderBadResponse,
			Message: "failed to decode LM Studio response",
			Cause:   err,
		}
	}

	return &chatResp, nil
}

// NormalizeError maps an error to a normalized provider error.
func (l *LMStudioAdapter) NormalizeError(err error) ProviderError {
	if err == nil {
		return ProviderError{}
	}

	var pe ProviderError
	if errors.As(err, &pe) {
		return pe
	}

	return classifyNetworkError(err, "lmstudio")
}
