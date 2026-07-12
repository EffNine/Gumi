package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/pipeline"
	"github.com/novexa/novexa/runtime/internal/provider"
	"github.com/novexa/novexa/runtime/internal/telemetry"
	"github.com/novexa/novexa/runtime/internal/version"
)

// Version is the runtime version exposed by the gateway. It mirrors
// version.Version so health and status endpoints report the same release
// metadata as the CLI, without introducing a dependency cycle.
var Version = version.Version

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
// When req.Stream is true, it uses SSE (Server-Sent Events) to stream chunks.
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

	if req.Stream {
		s.handleStreamChatCompletion(w, r, reqID, req)
		return
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

// handleStreamChatCompletion handles a streaming chat completion request via SSE.
func (s *Server) handleStreamChatCompletion(w http.ResponseWriter, r *http.Request, reqID string, req api.ChatCompletionRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("STREAMING_NOT_SUPPORTED", "streaming is not supported by the response writer", reqID))
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Create a buffered chunk channel
	chunkCh := make(chan api.ChatCompletionChunk, 64)

	// Run the streaming pipeline in a goroutine
	streamResultCh := make(chan pipeline.StreamResult, 1)
	ctx := r.Context()
	go func() {
		streamResultCh <- s.pipeline.RunChatCompletionStream(ctx, reqID, req, chunkCh)
	}()

	start := time.Now()
	var streamResult pipeline.StreamResult

	// Read chunks and write SSE events
	for chunk := range chunkCh {
		data, err := json.Marshal(chunk)
		if err != nil {
			s.log.Error("failed to marshal SSE chunk", err, "request_id", reqID)
			continue
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Get the stream result
	streamResult = <-streamResultCh

	// Handle terminal error
	if streamResult.Error.Code != "" {
		errData, _ := json.Marshal(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    string(streamResult.Error.Code),
				"message": streamResult.Error.Message,
				"type":    "runtime_error",
			},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", errData)
		flusher.Flush()
	}

	// End the stream
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Record telemetry
	latency := time.Since(start)
	s.recordStreamTelemetry(ctx, reqID, start, req, streamResult, latency)
}

// recordStreamTelemetry records telemetry for a streaming request.
func (s *Server) recordStreamTelemetry(ctx context.Context, reqID string, start time.Time, req api.ChatCompletionRequest, result pipeline.StreamResult, latency time.Duration) {
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
		Stream:      true,
		LatencyMs:   latency.Milliseconds(),
	}

	if result.Context != nil {
		record.RuntimeMode = string(result.Context.RuntimeMode)
		record.SessionID = result.Context.SessionID
		record.ProviderLatencyMs = result.Context.ProviderLatency.Milliseconds()
		record.ValidationPassed = result.Context.ValidationPassed
		record.ContextCompressed = result.Context.ContextCompressed
		if result.Context.SelectedModel != "" {
			record.Model = result.Context.SelectedModel
		}
		if result.Context.ThinkingTelemetry != nil {
			record.ThinkingEnabled = result.Context.ThinkingTelemetry.ThinkingEnabled
			record.ReasoningContentPresent = result.Context.ThinkingTelemetry.ReasoningContentPresent
		}
		// Agent-specific telemetry fields.
		record.AgentStepCount = result.Context.StepCount
		record.AgentLoopDetected = result.Context.LoopDetected
		// Streaming token counts from accumulated content
		record.PromptTokens = 0
		record.CompletionTokens = result.Context.StreamingTokenCount
		record.TotalTokens = result.Context.StreamingTokenCount
	}

	if result.Error.Code != "" {
		record.Status = "error"
		record.ErrorCode = string(result.Error.Code)
		record.ValidationPassed = false
	}

	logPrompts := s.cfg.Telemetry.LogPrompts
	record.PromptLogged = logPrompts
	record.ResponseLogged = false // streaming responses are not logged
	record.PromptPreview = telemetry.ExtractContentPreview(req, logPrompts)

	s.telemetry.RecordRequest(ctx, record)
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
		record.ValidationPassed = result.Context.ValidationPassed
		record.RepairApplied = result.Context.RepairApplied
		record.ContextCompressed = result.Context.ContextCompressed
		if result.Context.SelectedModel != "" {
			record.Model = result.Context.SelectedModel
		}
		if result.Context.ThinkingTelemetry != nil {
			record.ThinkingEnabled = result.Context.ThinkingTelemetry.ThinkingEnabled
			record.ReasoningContentPresent = result.Context.ThinkingTelemetry.ReasoningContentPresent
		}
		// Agent-specific telemetry fields.
		record.AgentStepCount = result.Context.StepCount
		record.AgentLoopDetected = result.Context.LoopDetected
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
	case provider.EmptyPrompt:
		status = http.StatusBadRequest
	case provider.ContextLimitExceeded:
		status = http.StatusBadRequest
	case provider.ValidationFailed:
		status = http.StatusUnprocessableEntity
	case provider.ProviderAuthError:
		status = http.StatusUnauthorized
	case provider.AGENT_STEP_LIMIT_EXCEEDED:
		status = http.StatusTooManyRequests
	case provider.AGENT_TOOL_CALL_LOOP:
		status = http.StatusConflict
	case provider.AGENT_INVALID_TOOL_CALL:
		status = http.StatusUnprocessableEntity
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

type configResponse struct {
	Runtime   any `json:"runtime"`
	Dashboard any `json:"dashboard"`
	Auth      any `json:"auth"`
	Provider  any `json:"provider"`
	Providers any `json:"providers"`
	Storage   any `json:"storage"`
	Telemetry any `json:"telemetry"`
}

// handleConfig exposes the resolved local configuration with secrets redacted.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	providers := make(map[string]any, len(s.cfg.Providers))
	for name, settings := range s.cfg.Providers {
		providers[name] = map[string]any{
			"enabled": settings.Enabled, "url": settings.URL,
			"default_model": settings.DefaultModel, "timeout_seconds": settings.TimeoutSeconds,
		}
	}
	resp := configResponse{
		Runtime: s.cfg.Runtime, Dashboard: s.cfg.Dashboard,
		Auth:     map[string]any{"mode": s.cfg.Auth.Mode, "local_key": "***REDACTED***"},
		Provider: s.cfg.Provider, Providers: providers,
		Storage: s.cfg.Storage, Telemetry: s.cfg.Telemetry,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type doctorCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type doctorResponse struct {
	Status string        `json:"status"`
	Checks []doctorCheck `json:"checks"`
}

// handleDoctor runs lightweight checks without changing runtime state.
func (s *Server) handleDoctor(w http.ResponseWriter, r *http.Request) {
	checks := []doctorCheck{
		{Name: "runtime_config", Status: "ok", Message: "Runtime configuration is loaded."},
		{Name: "telemetry_storage", Status: s.telemetry.StorageStatus(), Message: "Local telemetry storage is available."},
	}
	overall := "ok"
	for _, name := range s.manager.ListProviders() {
		status, err := s.manager.HealthCheck(r.Context(), name)
		check := doctorCheck{Name: "provider_" + name, Status: string(status), Message: fmt.Sprintf("Provider %s is reachable.", name)}
		if err != nil {
			check.Status = "warning"
			check.Message = fmt.Sprintf("Provider %s is not reachable.", name)
			check.Suggestion = fmt.Sprintf("Start %s or update its local URL in novexa.yaml.", name)
			overall = "warning"
		}
		checks = append(checks, check)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doctorResponse{Status: overall, Checks: checks})
}

type profileSummary struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Family       string   `json:"family"`
	Size         string   `json:"size"`
	ContextLimit int      `json:"context_limit"`
	Aliases      []string `json:"aliases,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

// handleProfiles returns non-sensitive model profile metadata.
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	data := make([]profileSummary, 0, len(s.profiles))
	for _, p := range s.profiles {
		if p == nil {
			continue
		}
		data = append(data, profileSummary{ID: p.ID, Name: p.Name, Family: p.Family, Size: p.Size, ContextLimit: p.ContextLimit, Aliases: p.Aliases, Notes: p.Notes})
	}
	sort.Slice(data, func(i, j int) bool { return data[i].ID < data[j].ID })
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "novexa.profiles", "data": data})
}
