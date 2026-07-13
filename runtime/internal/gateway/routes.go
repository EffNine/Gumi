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
	mux.HandleFunc("GET /v1/novexa/telemetry/recent", s.withAuthMiddleware(s.handleTelemetryRecent))
	mux.HandleFunc("GET /v1/novexa/status", s.withAuthMiddleware(s.handleStatus))
	mux.HandleFunc("GET /v1/novexa/config", s.withAuthMiddleware(s.handleConfig))
	mux.HandleFunc("GET /v1/novexa/doctor", s.withAuthMiddleware(s.handleDoctor))
	mux.HandleFunc("GET /v1/novexa/profiles", s.withAuthMiddleware(s.handleProfiles))
	mux.HandleFunc("GET /v1/novexa/memory/facts", s.withAuthMiddleware(s.handleMemoryFacts))
	mux.HandleFunc("POST /v1/novexa/memory/facts", s.withAuthMiddleware(s.handleMemoryCreateFact))
	mux.HandleFunc("GET /v1/novexa/memory/model-fit", s.withAuthMiddleware(s.handleMemoryModelFit))
	mux.HandleFunc("POST /v1/novexa/memory/clear", s.withAuthMiddleware(s.handleMemoryClear))
	mux.HandleFunc("GET /v1/novexa/memory/status", s.withAuthMiddleware(s.handleMemoryStatus))
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
