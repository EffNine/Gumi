package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/provider"
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
// execution to Pipeline Engine.
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

	result := s.pipeline.RunChatCompletion(r.Context(), reqID, req)
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
