package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
)

// contextKey is a private type for context keys.
type contextKey string

const requestIDKey contextKey = "request_id"

// requestIDHeader is the incoming/outgoing request ID header.
const requestIDHeader = "X-Request-ID"

// generateRequestID creates a short random request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return "req_" + hex.EncodeToString(b)
}

// requestIDMiddleware reads or generates a request ID and attaches it to the
// request context and response headers.
func (s *Server) requestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(requestIDHeader)
		if reqID == "" {
			reqID = generateRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		w.Header().Set(requestIDHeader, reqID)
		next(w, r.WithContext(ctx))
	}
}

// requestIDFromContext returns the request ID stored in the context, if any.
func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// authMiddleware validates the bearer token for local mode.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Auth.Mode == "disabled" {
			next(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			s.writeError(w, http.StatusUnauthorized, api.NewAuthError("missing authorization header", requestIDFromContext(r.Context())))
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			s.writeError(w, http.StatusUnauthorized, api.NewAuthError("authorization header must be Bearer <token>", requestIDFromContext(r.Context())))
			return
		}

		if s.cfg.Auth.Mode == "local" && parts[1] != s.cfg.Auth.LocalKey {
			s.writeError(w, http.StatusUnauthorized, api.NewAuthError("invalid API key", requestIDFromContext(r.Context())))
			return
		}

		next(w, r)
	}
}

// logMiddleware logs each request with method, path, status, and latency.
func (s *Server) logMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := requestIDFromContext(r.Context())

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(ww, r)

		s.log.Info("request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// recoverMiddleware catches panics and returns a 500 error response.
func (s *Server) recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID := requestIDFromContext(r.Context())
				s.log.Error("request panic", fmt.Errorf("%v", rec),
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				err := api.NewRuntimeError("RUNTIME_ERROR", "internal server error", reqID)
				s.writeError(w, http.StatusInternalServerError, err)
			}
		}()
		next(w, r)
	}
}

// writeError writes a JSON error response and sets the Content-Type header.
func (s *Server) writeError(w http.ResponseWriter, status int, resp api.ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// responseWriter captures the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write ensures WriteHeader is called before writing data.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher by delegating to the underlying writer.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
