package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

// testServer builds a gateway Server bound to a random free port.
// It never uses the configured default port, so tests pass even when the
// development runtime is already running. Provider URLs are pointed at an
// unused port so tests behave deterministically regardless of which local
// providers are installed.
func testServer(t *testing.T, mode string) (*Server, *config.Config, *logger.Logger) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = mode
	// Bind to port 0 so the OS assigns an unused port.
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	// Point providers at an unreachable port so tests are deterministic.
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}
	log := logger.New("error")
	srv := New(cfg, log)
	return srv, cfg, log
}

// testServerWithConfig creates a Server from an already-mutated config so tests
// can inject mock provider URLs before the provider manager is built.
func testServerWithConfig(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	log := logger.New("error")
	return New(cfg, log)
}

func TestHealthEndpoint(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body healthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status ok, got %s", body.Status)
	}
	if body.Runtime != "novexa" {
		t.Errorf("expected runtime novexa, got %s", body.Runtime)
	}
}

func TestModelsEndpointAuthorized(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer novexa-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body api.ModelsList
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}
	if body.Object != "list" {
		t.Errorf("expected object list, got %s", body.Object)
	}
	if len(body.Data) == 0 {
		t.Error("expected at least one model")
	}
}

func TestModelsEndpointMissingAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	assertAuthError(t, rr)
}

func TestModelsEndpointInvalidAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	assertAuthError(t, rr)
}

func TestModelsEndpointDisabledAuth(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestChatCompletionsSuccessWithMockProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer novexa-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body api.ChatCompletionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	if body.Object != "chat.completion" {
		t.Errorf("expected object chat.completion, got %s", body.Object)
	}
	if len(body.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(body.Choices))
	}
	if body.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected assistant role, got %s", body.Choices[0].Message.Role)
	}
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID response header")
	}
	if rr.Header().Get("X-Novexa-Provider") != "ollama" {
		t.Errorf("expected ollama provider header, got %s", rr.Header().Get("X-Novexa-Provider"))
	}
}

func newOllamaMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/chat":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "llama3",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "hello from mock ollama",
				},
				"done": true,
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func TestChatCompletionsMissingAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	assertAuthError(t, rr)
}

func TestChatCompletionsInvalidKey(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer wrong-key")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	assertAuthError(t, rr)
}

func TestChatCompletionsDisabledAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0

	mock := newOllamaMockServer(t)
	defer mock.Close()
	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestChatCompletionsMissingModel(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	payload := `{"messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "MISSING_MODEL" {
		t.Errorf("expected MISSING_MODEL, got %s", body.Error.Code)
	}
}

func TestChatCompletionsMissingMessages(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	payload := `{"model":"local:auto"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRequestIDPropagation(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-Request-ID"); got != "custom-id-123" {
		t.Errorf("expected X-Request-ID custom-id-123, got %s", got)
	}
}

func TestRequestIDHeaderOnAuthError(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("X-Request-ID", "auth-error-id")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if got := rr.Header().Get("X-Request-ID"); got != "auth-error-id" {
		t.Errorf("expected X-Request-ID auth-error-id, got %s", got)
	}

	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.RequestID != "auth-error-id" {
		t.Errorf("expected error request_id auth-error-id, got %s", body.Error.RequestID)
	}
}

func TestServerShutdown(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	errCh := srv.Start()
	// Give the server a moment to bind.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server to stop")
	}
}

func TestServerStartAndRequest(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	errCh := srv.Start()
	time.Sleep(100 * time.Millisecond)

	url := "http://" + srv.Addr() + "/health"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to request health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	<-errCh
}

func TestChatCompletionsInvalidJSON(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestChatCompletionsExplicitProviderUnavailable(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	payload := `{"model":"ollama:llama3","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rr.Code, rr.Body.String())
	}

	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "PROVIDER_UNAVAILABLE" {
		t.Errorf("expected PROVIDER_UNAVAILABLE, got %s", body.Error.Code)
	}
	if body.Error.RequestID == "" {
		t.Error("expected request_id in provider error response")
	}
}

func TestChatCompletionsLocalAutoReturnsErrorWhenOffline(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 provider unavailable, got %d: %s", rr.Code, rr.Body.String())
	}

	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "PROVIDER_UNAVAILABLE" {
		t.Errorf("expected PROVIDER_UNAVAILABLE, got %s", body.Error.Code)
	}
	if body.Error.Suggestion == "" {
		t.Error("expected suggestion in provider unavailable error")
	}
}

func TestModelsEndpointIncludesLocalAuto(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body api.ModelsList
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	found := false
	for _, m := range body.Data {
		if m.ID == "local:auto" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected local:auto in model list")
	}
}

func TestStreamingRejected(t *testing.T) {
	srv, _, _ := testServer(t, "disabled")

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != "STREAMING_NOT_SUPPORTED" {
		t.Errorf("expected STREAMING_NOT_SUPPORTED, got %s", body.Error.Code)
	}
}

// assertAuthError validates the standard error response for auth failures.
func assertAuthError(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID response header on auth error")
	}
	var body api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Type != "auth_error" {
		t.Errorf("expected auth_error, got %s", body.Error.Type)
	}
	if body.Error.Code != "INVALID_API_KEY" {
		t.Errorf("expected INVALID_API_KEY, got %s", body.Error.Code)
	}
	if body.Error.RequestID == "" {
		t.Error("expected request_id in auth error response")
	}
}
