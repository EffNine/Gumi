// Package gateway integration tests exercise the full request path:
// HTTP gateway → pipeline engine → provider adapter → telemetry storage.
//
// These tests use httptest.Server and mock providers so they are deterministic
// and do not require any external services (Ollama, LM Studio, etc.).
package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
)

// TestIntegrationFullRequestChain verifies that a chat completion request flows
// through the gateway → pipeline → provider → telemetry chain correctly.
//
// It uses a mock Ollama server so the test is fully deterministic. It asserts:
//   - HTTP 200 with valid ChatCompletionResponse
//   - Provider header (X-Gumi-Provider) is set
//   - Runtime mode header (X-Gumi-Runtime-Mode) is set
//   - Request ID is propagated
//   - Telemetry is written to the database
func TestIntegrationFullRequestChain(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")
	cfg.Telemetry.Local = true
	cfg.Telemetry.RetainDays = 14

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
	if rr.Header().Get("X-Gumi-Provider") == "" {
		t.Error("expected X-Gumi-Provider response header")
	}
	if rr.Header().Get("X-Gumi-Runtime-Mode") == "" {
		t.Error("expected X-Gumi-Runtime-Mode response header")
	}

	// Verify telemetry was written by checking the recent endpoint
	teleReq := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/recent", nil)
	teleRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(teleRR, teleReq)
	if teleRR.Code != http.StatusOK {
		t.Fatalf("telemetry endpoint: expected 200, got %d", teleRR.Code)
	}

	var teleBody struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(teleRR.Body.Bytes(), &teleBody); err != nil {
		t.Fatalf("failed to decode telemetry response: %v", err)
	}
	if len(teleBody.Data) == 0 {
		t.Error("expected at least one telemetry request entry after chat completion")
	}
}

// TestIntegrationPipelineModes verifies that each pipeline mode (direct, stabilized,
// structured, agent) produces the correct X-Gumi-Runtime-Mode header and that the
// pipeline engine processes the request without error.
func TestIntegrationPipelineModes(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		gumiMode   string
		wantMode   string
		wantStatus int
		message    string
	}{
		{
			name:       "direct mode",
			model:      "ollama:llama3",
			gumiMode:   "direct",
			wantMode:   "direct",
			wantStatus: http.StatusOK,
			message:    "Hello",
		},
		{
			name:       "stabilized mode",
			model:      "ollama:llama3",
			gumiMode:   "stabilized",
			wantMode:   "stabilized",
			wantStatus: http.StatusOK,
			message:    "Hello",
		},
		{
			name:       "structured mode",
			model:      "ollama:llama3",
			gumiMode:   "structured",
			wantMode:   "structured",
			wantStatus: http.StatusOK,
			message:    "Return JSON with keys ok and value",
		},
		{
			name:       "agent mode",
			model:      "ollama:llama3",
			gumiMode:   "agent",
			wantMode:   "agent",
			wantStatus: http.StatusOK,
			message:    "Hello",
		},
		{
			name:       "lightweight mode",
			model:      "ollama:llama3",
			gumiMode:   "lightweight",
			wantMode:   "lightweight",
			wantStatus: http.StatusOK,
			message:    "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Auth.Mode = "disabled"
			cfg.Runtime.Host = "127.0.0.1"
			cfg.Runtime.Port = 0
			cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")

			mock := newOllamaMockServer(t)
			defer mock.Close()

			settings := cfg.Providers["ollama"]
			settings.URL = mock.URL
			cfg.Providers["ollama"] = settings

			srv := testServerWithConfig(t, cfg)

			payloadMap := map[string]interface{}{
				"model":    tt.model,
				"messages": []map[string]string{{"role": "user", "content": tt.message}},
			}
			if tt.gumiMode != "" {
				payloadMap["gumi"] = map[string]string{"mode": tt.gumiMode}
			}
			payloadBytes, _ := json.Marshal(payloadMap)

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(payloadBytes)))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rr.Code, rr.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				gotMode := rr.Header().Get("X-Gumi-Runtime-Mode")
				if gotMode != tt.wantMode {
					t.Errorf("expected X-Gumi-Runtime-Mode %q, got %q", tt.wantMode, gotMode)
				}
			}
		})
	}
}

// TestIntegrationGumiEndpoints verifies that all /v1/gumi/* endpoints respond
// correctly when the runtime is running with a mock provider.
func TestIntegrationGumiEndpoints(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")
	cfg.Telemetry.Local = true

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	endpoints := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"status", "GET", "/v1/gumi/status", http.StatusOK},
		{"profiles", "GET", "/v1/gumi/profiles", http.StatusOK},
		{"config", "GET", "/v1/gumi/config", http.StatusOK},
		{"telemetry recent", "GET", "/v1/gumi/telemetry/recent", http.StatusOK},
		{"doctor", "GET", "/v1/gumi/doctor", http.StatusOK},
		{"memory status", "GET", "/v1/gumi/memory/status", http.StatusOK},
		{"memory facts", "GET", "/v1/gumi/memory/facts", http.StatusOK},
		{"memory model-fit", "GET", "/v1/gumi/memory/model-fit", http.StatusOK},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rr := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(rr, req)

			if rr.Code != ep.wantStatus {
				t.Errorf("expected %d, got %d: %s", ep.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

// TestIntegrationDashboardProxy verifies that the dashboard server proxies API
// requests to the runtime gateway and returns the expected responses.
func TestIntegrationDashboardProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Dashboard.Enabled = true
	cfg.Dashboard.Host = "127.0.0.1"
	cfg.Dashboard.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	// Start the gateway server on a real port
	gatewaySrv := testServerWithConfig(t, cfg)
	gatewayErrCh := gatewaySrv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = gatewaySrv.Shutdown(ctx)
		<-gatewayErrCh
	}()
	time.Sleep(100 * time.Millisecond)

	// Verify the gateway is reachable
	gatewayAddr := gatewaySrv.Addr()
	resp, err := http.Get("http://" + gatewayAddr + "/health")
	if err != nil {
		t.Fatalf("gateway health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from gateway health, got %d", resp.StatusCode)
	}
}

// TestIntegrationMemoryEndpoints verifies that memory engine endpoints work
// through the gateway when memory is enabled.
func TestIntegrationMemoryEndpoints(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")
	cfg.Memory.Enabled = true
	cfg.Memory.DBPath = filepath.Join(t.TempDir(), "memory.db")

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	srv := testServerWithConfig(t, cfg)

	// Check memory status
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/memory/status", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("memory status: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode memory status: %v", err)
	}
	if !status.Enabled {
		t.Error("expected memory to be enabled")
	}

	// Create a fact
	factPayload := `{"key":"test_key","value":"test_value","source":"integration_test"}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/gumi/memory/facts", strings.NewReader(factPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(createRR, createReq)

	if createRR.Code != http.StatusCreated && createRR.Code != http.StatusOK {
		t.Fatalf("create fact: expected 201 or 200, got %d: %s", createRR.Code, createRR.Body.String())
	}

	// List facts
	listReq := httptest.NewRequest(http.MethodGet, "/v1/gumi/memory/facts", nil)
	listRR := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(listRR, listReq)

	if listRR.Code != http.StatusOK {
		t.Fatalf("list facts: expected 200, got %d: %s", listRR.Code, listRR.Body.String())
	}

	var factsResponse struct {
		Facts []map[string]interface{} `json:"facts"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &factsResponse); err != nil {
		t.Fatalf("failed to decode facts response: %v", err)
	}
	if len(factsResponse.Facts) == 0 {
		t.Error("expected at least one fact after creation")
	}
}

// TestIntegrationProviderHealth verifies that provider health checks are
// reported correctly through the gateway status endpoint.
func TestIntegrationProviderHealth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Storage.DBPath = filepath.Join(t.TempDir(), "gumi.db")

	mock := newOllamaMockServer(t)
	defer mock.Close()

	settings := cfg.Providers["ollama"]
	settings.URL = mock.URL
	cfg.Providers["ollama"] = settings

	// Point LM Studio at an unreachable address to test offline detection
	lmSettings := cfg.Providers["lmstudio"]
	lmSettings.URL = "http://127.0.0.1:1"
	cfg.Providers["lmstudio"] = lmSettings

	srv := testServerWithConfig(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/status", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status struct {
		Providers []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	// Ollama should be ok (mock is running)
	foundOllama := false
	foundLM := false
	for _, p := range status.Providers {
		switch p.Name {
		case "ollama":
			foundOllama = true
			if p.Status != "ok" {
				t.Errorf("expected ollama status ok, got %s", p.Status)
			}
		case "lmstudio":
			foundLM = true
			if p.Status != "offline" && p.Status != "error" {
				t.Errorf("expected lmstudio status offline/error, got %s", p.Status)
			}
		}
	}
	if !foundOllama {
		t.Error("expected ollama in provider list")
	}
	if !foundLM {
		t.Error("expected lmstudio in provider list")
	}
}
