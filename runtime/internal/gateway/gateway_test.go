package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
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
	// Use a temporary database so tests do not write to ~/.gumi.
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")
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
	if body.Runtime != "gumi" {
		t.Errorf("expected runtime gumi, got %s", body.Runtime)
	}
}

func TestModelsEndpointAuthorized(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
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
	req.Header.Set("Authorization", "Bearer gumi-local")
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
	if rr.Header().Get("X-Gumi-Provider") != "ollama" {
		t.Errorf("expected ollama provider header, got %s", rr.Header().Get("X-Gumi-Provider"))
	}
	if rr.Header().Get("X-Gumi-Runtime-Mode") != "stabilized" {
		t.Errorf("expected stabilized runtime mode header, got %s", rr.Header().Get("X-Gumi-Runtime-Mode"))
	}
}

func TestChatCompletionsUsesPipelineModeResolution(t *testing.T) {
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

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Return JSON"}],"response_format":{"type":"json_object"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("X-Gumi-Runtime-Mode") != "structured" {
		t.Errorf("expected structured mode resolved by pipeline, got %s", rr.Header().Get("X-Gumi-Runtime-Mode"))
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
			var payload struct {
				Stream   bool `json:"stream"`
				Messages []struct {
					Content string `json:"content"`
				} `json:"messages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)

			if payload.Stream {
				// Streaming response: NDJSON
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "no flusher", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/x-ndjson")
				w.WriteHeader(http.StatusOK)

				_, _ = fmt.Fprintln(w, `{"model":"llama3","message":{"role":"assistant","content":"Hello"},"done":false}`)
				flusher.Flush()
				_, _ = fmt.Fprintln(w, `{"model":"llama3","message":{"role":"assistant","content":"Hello from"},"done":false}`)
				flusher.Flush()
				_, _ = fmt.Fprintln(w, `{"model":"llama3","message":{"role":"assistant","content":"Hello from mock ollama"},"done":true,"prompt_eval_count":5,"eval_count":3}`)
				flusher.Flush()
				return
			}

			content := "hello from mock ollama"
			for _, msg := range payload.Messages {
				if strings.Contains(msg.Content, "Return JSON") {
					content = `{"ok":true}`
					break
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"model": "llama3",
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
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

func TestStreamingChatCompletions(t *testing.T) {
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

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Hello"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	// The responseWriter wrapper now implements http.Flusher, so streaming
	// should succeed and produce SSE output.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Fatal("expected SSE data events in response")
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatal("expected [DONE] termination event")
	}
}

func TestStreamingChatCompletionsWithRealServer(t *testing.T) {
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
	errCh := srv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-errCh
	}()
	time.Sleep(100 * time.Millisecond)

	url := "http://" + srv.Addr() + "/v1/chat/completions"
	payload := `{"model":"ollama:llama3","messages":[{"role":"user","content":"Hello"}],"stream":true}`

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to send streaming request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error body for debugging
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes[:n]))
	}

	// Verify SSE headers
	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Errorf("expected X-Accel-Buffering no, got %s", resp.Header.Get("X-Accel-Buffering"))
	}

	// Read SSE events
	scanner := bufio.NewScanner(resp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			events = append(events, strings.TrimPrefix(line, "data: "))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading SSE stream: %v", err)
	}

	// Should have at least one chunk and a [DONE] event
	if len(events) < 2 {
		t.Fatalf("expected at least 2 SSE events (chunks + DONE), got %d", len(events))
	}

	// Last event should be [DONE]
	lastEvent := events[len(events)-1]
	if lastEvent != "[DONE]" {
		t.Errorf("expected last event to be [DONE], got %s", lastEvent)
	}

	// First event should be a valid ChatCompletionChunk
	var firstChunk api.ChatCompletionChunk
	if err := json.Unmarshal([]byte(events[0]), &firstChunk); err != nil {
		t.Fatalf("failed to decode first chunk: %v", err)
	}
	if firstChunk.Object != "chat.completion.chunk" {
		t.Errorf("expected object chat.completion.chunk, got %s", firstChunk.Object)
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

func TestTelemetryRecentRequiresAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/recent", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	assertAuthError(t, rr)
}

func TestTelemetryRecentReturnsRequestMetadata(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Say hello from telemetry test"}]}`
	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	chatReq.Header.Set("Authorization", "Bearer gumi-local")
	chatReq.Header.Set("Content-Type", "application/json")
	chatRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(chatRR, chatReq)
	if chatRR.Code != http.StatusOK {
		t.Fatalf("expected chat 200, got %d: %s", chatRR.Code, chatRR.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/recent", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID            string `json:"id"`
			CreatedAt     string `json:"created_at"`
			RuntimeMode   string `json:"runtime_mode"`
			Provider      string `json:"provider"`
			Model         string `json:"model"`
			Status        string `json:"status"`
			LatencyMs     int64  `json:"latency_ms"`
			ErrorCode     string `json:"error_code"`
			RepairApplied bool   `json:"repair_applied"`
			RetryCount    int    `json:"retry_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode telemetry recent response: %v", err)
	}
	if body.Object != "gumi.telemetry.recent" {
		t.Errorf("expected object gumi.telemetry.recent, got %s", body.Object)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected 1 recent request, got %d", len(body.Data))
	}
	if body.Data[0].Provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", body.Data[0].Provider)
	}
	if body.Data[0].Status != "success" {
		t.Errorf("expected status success, got %s", body.Data[0].Status)
	}
	if body.Data[0].LatencyMs == 0 {
		t.Error("expected non-zero latency")
	}
}

func TestTelemetryRecentDoesNotExposeFullPromptOrResponse(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	payload := `{"model":"local:auto","messages":[{"role":"user","content":"Say hello from telemetry test"}]}`
	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	chatReq.Header.Set("Authorization", "Bearer gumi-local")
	chatReq.Header.Set("Content-Type", "application/json")
	chatRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(chatRR, chatReq)
	if chatRR.Code != http.StatusOK {
		t.Fatalf("expected chat 200, got %d: %s", chatRR.Code, chatRR.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/recent", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	bodyStr := rr.Body.String()
	if strings.Contains(bodyStr, "Say hello from telemetry test") {
		t.Error("recent telemetry leaked full prompt content")
	}
	if strings.Contains(bodyStr, "hello from mock ollama") {
		t.Error("recent telemetry leaked full response content")
	}
}

func TestStatusEndpointRequiresAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/status", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	assertAuthError(t, rr)
}

func TestStatusEndpointReturnsRuntimeAndProviders(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/status", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if body.Runtime.Status != "running" {
		t.Errorf("expected running, got %s", body.Runtime.Status)
	}
	if body.Runtime.Version != Version {
		t.Errorf("expected version %s, got %s", Version, body.Runtime.Version)
	}
	if body.StorageStatus != "ok" {
		t.Errorf("expected storage ok, got %s", body.StorageStatus)
	}
	if !body.TelemetryEnabled {
		t.Error("expected telemetry enabled")
	}
	if len(body.Providers) == 0 {
		t.Error("expected at least one provider summary")
	}
}

func TestProviderErrorRecordedInTelemetry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)

	payload := `{"model":"ollama:llama3","messages":[{"role":"user","content":"Hello"}]}`
	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
	chatReq.Header.Set("Authorization", "Bearer gumi-local")
	chatReq.Header.Set("Content-Type", "application/json")
	chatRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(chatRR, chatReq)

	if chatRR.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", chatRR.Code, chatRR.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/recent", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	var recent telemetryRecentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &recent); err != nil {
		t.Fatalf("failed to decode recent telemetry: %v", err)
	}
	if len(recent.Data) != 1 {
		t.Fatalf("expected 1 telemetry row, got %d", len(recent.Data))
	}
	if recent.Data[0].Status != "error" {
		t.Errorf("expected error status, got %s", recent.Data[0].Status)
	}
	if recent.Data[0].ErrorCode != "PROVIDER_UNAVAILABLE" {
		t.Errorf("expected PROVIDER_UNAVAILABLE, got %s", recent.Data[0].ErrorCode)
	}
}

func TestConfigEndpointRedactsLocalKey(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/config", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Auth.LocalKey)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), cfg.Auth.LocalKey) {
		t.Fatal("config endpoint leaked local API key")
	}
	if !strings.Contains(rr.Body.String(), "***REDACTED***") {
		t.Fatal("expected redacted marker")
	}
}

func TestDoctorEndpointReturnsChecks(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/doctor", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Auth.LocalKey)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body doctorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Checks) < 2 {
		t.Fatalf("expected diagnostic checks, got %d", len(body.Checks))
	}
}

func TestProfilesEndpointReturnsLoadedProfiles(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/profiles", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Auth.LocalKey)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data []profileSummary `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) == 0 {
		t.Fatal("expected at least generic profile")
	}
}
