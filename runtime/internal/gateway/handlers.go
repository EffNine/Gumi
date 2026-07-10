package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/pipeline"
	"github.com/novexa/novexa/runtime/internal/provider"
	"github.com/novexa/novexa/runtime/internal/telemetry"
)

// Version is injected by the CLI package via the Server. It mirrors cli.Version
// but avoids a direct import cycle.
const Version = "0.1.0"

// healthResponse is the payload for GET /health.
type healthResponse struct {
	Status    string `json:"status"`
	Runtime   string `json:"runtime"`
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	Timestamp string `json:"timestamp"`
}

// handleHealth returns runtime health information. It does not depend on
// providers being online so the gateway remains responsive.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:    "ok",
		Runtime:   "novexa",
		Version:   Version,
		Mode:      s.cfg.Runtime.Environment,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleModels returns the local:auto alias merged with provider-discovered models.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	list := api.NewModelsList()

	providerModels := s.manager.ListModels(r.Context())
	list.Data = append(list.Data, providerModels...)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// handleChatCompletions validates the HTTP request shape and delegates request
// execution to Pipeline Engine. Request metadata is recorded to local telemetry.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	reqID := requestIDFromContext(r.Context())

	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", fmt.Sprintf("invalid JSON body: %v", err), reqID))
		return
	}

	if req.Model == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_MODEL", "request field 'model' is required", reqID))
		return
	}

	if len(req.Messages) == 0 {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_MESSAGES", "request field 'messages' is required and must not be empty", reqID))
		return
	}

	for i, m := range req.Messages {
		if m.Role == "" {
			s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_MESSAGES", fmt.Sprintf("message at index %d is missing 'role'", i), reqID))
			return
		}
	}

	start := time.Now()
	result := s.pipeline.RunChatCompletion(r.Context(), reqID, req)
	latency := time.Since(start)

	s.recordRequestTelemetry(r.Context(), reqID, start, req, result, latency)

	if result.Error.Code != "" {
		s.writeProviderError(w, result.Error, reqID)
		return
	}

	if result.ProviderName != "" {
		w.Header().Set("X-Novexa-Provider", result.ProviderName)
	}
	if result.Response != nil {
		w.Header().Set("X-Novexa-Model", result.Response.Model)
	}
	if result.Context != nil {
		w.Header().Set("X-Novexa-Runtime-Mode", string(result.Context.RuntimeMode))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result.Response)
}

// recordRequestTelemetry writes the request summary row to local telemetry.
// Storage failures are logged but never block the response.
func (s *Server) recordRequestTelemetry(ctx context.Context, reqID string, start time.Time, req api.ChatCompletionRequest, result pipeline.Result, latency time.Duration) {
	if s.telemetry == nil {
		return
	}

	record := telemetry.RequestRecord{
		RequestID:   reqID,
		CreatedAt:   start,
		WorkspaceID: "default",
		RuntimeMode: string(s.cfg.Runtime.Mode),
		Provider:    result.ProviderName,
		Status:      "success",
		Stream:      req.Stream,
		LatencyMs:   latency.Milliseconds(),
	}

	if result.Context != nil {
		record.RuntimeMode = string(result.Context.RuntimeMode)
		record.SessionID = result.Context.SessionID
		record.ProviderLatencyMs = result.Context.ProviderLatency.Milliseconds()
		record.RetryCount = result.Context.Retry.Attempt - 1
		record.ValidationPassed = true
		record.RepairApplied = false
		record.ContextCompressed = result.Context.ContextCompressed
		if result.Context.SelectedModel != "" {
			record.Model = result.Context.SelectedModel
		}
	}

	if result.Response != nil {
		resp := result.Response
		if resp.Model != "" {
			record.Model = resp.Model
		}
		record.PromptTokens = resp.Usage.PromptTokens
		record.CompletionTokens = resp.Usage.CompletionTokens
		record.TotalTokens = resp.Usage.TotalTokens
	}

	if result.Error.Code != "" {
		record.Status = "error"
		record.ErrorCode = string(result.Error.Code)
		record.ValidationPassed = false
	}

	logPrompts := s.cfg.Telemetry.LogPrompts
	logResponses := s.cfg.Telemetry.LogResponses
	record.PromptLogged = logPrompts
	record.ResponseLogged = logResponses
	record.PromptPreview = telemetry.ExtractContentPreview(req, logPrompts)
	record.ResponsePreview = telemetry.ExtractResponsePreview(result.Response, logResponses)

	s.telemetry.RecordRequest(ctx, record)
}

// writeProviderError converts a ProviderError into an OpenAI-compatible error
// response and writes it to the response writer.
func (s *Server) writeProviderError(w http.ResponseWriter, perr provider.ProviderError, reqID string) {
	status := http.StatusBadGateway
	switch perr.Code {
	case provider.ProviderTimeout:
		status = http.StatusGatewayTimeout
	case provider.ModelNotFound:
		status = http.StatusNotFound
	case provider.ProviderMisconfigured:
		status = http.StatusBadRequest
	case provider.StreamingUnsupported:
		status = http.StatusBadRequest
	case provider.ProviderAuthError:
		status = http.StatusUnauthorized
	}

	errResp := api.ErrorResponse{
		Error: api.APIError{
			Code:       string(perr.Code),
			Message:    perr.Message,
			Type:       "runtime_error",
			Engine:     "pipeline",
			Retryable:  perr.Code == provider.ProviderUnavailable || perr.Code == provider.ProviderTimeout,
			Suggestion: perr.Suggestion,
			RequestID:  reqID,
		},
	}

	s.writeError(w, status, errResp)
}

// telemetryRecentResponse is the payload for GET /v1/novexa/telemetry/recent.
type telemetryRecentResponse struct {
	Object string                    `json:"object"`
	Data   []telemetry.RecentRequest `json:"data"`
}

// handleTelemetryRecent returns recent request metadata without full prompts or
// responses.
func (s *Server) handleTelemetryRecent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	recent, err := s.telemetry.RecentRequests(ctx, 100)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", "failed to read recent telemetry", requestIDFromContext(ctx)))
		return
	}
	if recent == nil {
		recent = []telemetry.RecentRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(telemetryRecentResponse{
		Object: "novexa.telemetry.recent",
		Data:   recent,
	})
}

// statusProvider describes one configured provider for the status endpoint.
type statusProvider struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// statusResponse is the payload for GET /v1/novexa/status.
type statusResponse struct {
	Runtime struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Mode    string `json:"mode"`
		APIURL  string `json:"api_url"`
	} `json:"runtime"`
	Providers        []statusProvider `json:"providers,omitempty"`
	StorageStatus    string           `json:"storage_status"`
	TelemetryEnabled bool             `json:"telemetry_enabled"`
}

// handleStatus returns runtime status and provider summary.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	providers := make([]statusProvider, 0, len(s.cfg.Providers))
	for key, settings := range s.cfg.Providers {
		if !settings.Enabled {
			continue
		}
		st := provider.StatusUnknown
		if checked, err := s.manager.HealthCheck(ctx, key); err == nil {
			st = checked
		} else {
			st = provider.StatusOffline
		}
		providers = append(providers, statusProvider{
			Name:   key,
			Status: string(st),
			URL:    settings.URL,
		})
	}

	resp := statusResponse{
		Providers:        providers,
		StorageStatus:    s.telemetry.StorageStatus(),
		TelemetryEnabled: s.cfg.Telemetry.Local,
	}
	resp.Runtime.Status = "running"
	resp.Runtime.Version = Version
	resp.Runtime.Mode = s.cfg.Runtime.Mode
	resp.Runtime.APIURL = fmt.Sprintf("http://%s:%d/v1", s.cfg.Runtime.Host, s.cfg.Runtime.Port)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
