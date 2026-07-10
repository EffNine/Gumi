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
// development runtime is already running.
func testServer(t *testing.T, mode string) (*Server, *config.Config, *logger.Logger) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = mode
	// Bind to port 0 so the OS assigns an unused port.
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	log := logger.New("error")
	srv := New(cfg, log)
	return srv, cfg, log
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

func TestChatCompletionsSuccess(t *testing.T) {
	srv, _, _ := testServer(t, "local")

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
	if rr.Header().Get("X-Novexa-Provider") == "" {
		t.Error("expected X-Novexa-Provider response header")
	}
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
	srv, _, _ := testServer(t, "disabled")

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
