package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
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

// handleHealth returns runtime health information.
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

// handleModels returns the static Sprint 2 model list.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.NewModelsList())
}

// handleChatCompletions returns a placeholder OpenAI-compatible chat response.
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

	mode := s.cfg.Runtime.Mode
	if req.Novexa != nil && req.Novexa.Mode != "" {
		mode = req.Novexa.Mode
	}

	provider := s.cfg.Provider.Default
	model := req.Model

	resp := api.ChatCompletionResponse{
		ID:      "chatcmpl_nvx_" + reqID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []api.Choice{
			{
				Index: 0,
				Message: api.Message{
					Role:    "assistant",
					Content: placeholderContent(req.Stream),
				},
				FinishReason: "stop",
			},
		},
		Usage: api.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}

	w.Header().Set("X-Novexa-Provider", provider)
	w.Header().Set("X-Novexa-Model", model)
	w.Header().Set("X-Novexa-Runtime-Mode", mode)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// placeholderContent returns a helpful placeholder message explaining the
// current sprint limitation.
func placeholderContent(stream bool) string {
	if stream {
		return "Streaming is not yet implemented in Sprint 2. Set stream=false to receive a placeholder response."
	}
	return "This is a Sprint 2 placeholder response. Real provider generation will be available in Sprint 3."
}
