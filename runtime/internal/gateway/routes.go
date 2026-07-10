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
