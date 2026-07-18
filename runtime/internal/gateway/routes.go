package gateway

import (
	"net/http"
)

// registerRoutes attaches all gateway routes to the provided mux.
// Only /health is public; all other v1 endpoints require authentication.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.withPublicMiddleware(s.handleHealth))
	mux.HandleFunc("GET /v1/models", s.withAuthMiddleware(s.handleModels))
	mux.HandleFunc("POST /v1/chat/completions", s.withAuthMiddleware(s.handleChatCompletions))
	mux.HandleFunc("GET /v1/gumi/telemetry/recent", s.withAuthMiddleware(s.handleTelemetryRecent))
	mux.HandleFunc("GET /v1/gumi/telemetry/requests", s.withAuthMiddleware(s.handleTelemetryRequests))
	mux.HandleFunc("GET /v1/gumi/telemetry/aggregate", s.withAuthMiddleware(s.handleTelemetryAggregate))
	mux.HandleFunc("GET /v1/gumi/telemetry/events/{request_id}", s.withAuthMiddleware(s.handleTelemetryEvents))
	mux.HandleFunc("GET /v1/gumi/telemetry/health", s.withAuthMiddleware(s.handleTelemetryHealth))
	mux.HandleFunc("GET /v1/gumi/status", s.withAuthMiddleware(s.handleStatus))
	mux.HandleFunc("GET /v1/gumi/config", s.withAuthMiddleware(s.handleConfig))
	mux.HandleFunc("POST /v1/gumi/config", s.withAuthMiddleware(s.handleConfigSave))
	mux.HandleFunc("GET /v1/gumi/doctor", s.withAuthMiddleware(s.handleDoctor))
	mux.HandleFunc("GET /v1/gumi/profiles", s.withAuthMiddleware(s.handleProfiles))
	mux.HandleFunc("GET /v1/gumi/memory/facts", s.withAuthMiddleware(s.handleMemoryFacts))
	mux.HandleFunc("POST /v1/gumi/memory/facts", s.withAuthMiddleware(s.handleMemoryCreateFact))
	mux.HandleFunc("GET /v1/gumi/memory/model-fit", s.withAuthMiddleware(s.handleMemoryModelFit))
	mux.HandleFunc("POST /v1/gumi/memory/clear", s.withAuthMiddleware(s.handleMemoryClear))
	mux.HandleFunc("GET /v1/gumi/memory/status", s.withAuthMiddleware(s.handleMemoryStatus))
	mux.HandleFunc("GET /v1/gumi/self-tuning", s.withAuthMiddleware(s.handleSelfTuning))
	// Model registry
	mux.HandleFunc("GET /v1/gumi/models", s.withAuthMiddleware(s.handleGetModels))
	mux.HandleFunc("POST /v1/gumi/models", s.withAuthMiddleware(s.handleCreateModel))
	mux.HandleFunc("PUT /v1/gumi/models/{alias}", s.withAuthMiddleware(s.handleUpdateModel))
	mux.HandleFunc("DELETE /v1/gumi/models/{alias}", s.withAuthMiddleware(s.handleDeleteModel))
	mux.HandleFunc("POST /v1/gumi/models/{alias}/default", s.withAuthMiddleware(s.handleSetDefaultModel))
	// LM Studio model management
	mux.HandleFunc("GET /v1/gumi/providers/lmstudio/models", s.withAuthMiddleware(s.handleLMStudioModels))
	mux.HandleFunc("POST /v1/gumi/providers/lmstudio/models/load", s.withAuthMiddleware(s.handleLMStudioLoadModel))
	mux.HandleFunc("POST /v1/gumi/providers/lmstudio/models/unload", s.withAuthMiddleware(s.handleLMStudioUnloadModel))
	mux.HandleFunc("GET /v1/gumi/providers/lmstudio/loaded", s.withAuthMiddleware(s.handleLMStudioLoaded))
	// Log streaming
	mux.HandleFunc("GET /v1/gumi/logs/stream", s.withAuthMiddleware(s.handleLogStream))
}

// withPublicMiddleware applies logging and request-ID middleware without auth.
func (s *Server) withPublicMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.recoverMiddleware(
		s.requestIDMiddleware(
			s.logMiddleware(next),
		),
	)
}

// withAuthMiddleware applies the full gateway middleware chain including auth.
func (s *Server) withAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.recoverMiddleware(
		s.requestIDMiddleware(
			s.authMiddleware(
				s.logMiddleware(next),
			),
		),
	)
}
