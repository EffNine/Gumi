package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/memory"
)

// testServerWithMemory creates a Server with memory engine enabled for testing.
func testServerWithMemory(t *testing.T) (*Server, *config.Config) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Memory.Enabled = true
	// Use a temp directory path so each test gets a fresh database.
	// pipeline.New() overrides empty DBPath with ~/.novexa/memory.db, so we
	// must provide a unique path to avoid sharing state across tests.
	cfg.Memory.DBPath = t.TempDir() + "/memory.db"
	cfg.Storage.DBPath = t.TempDir() + "/novexa.db"

	// Point providers at an unreachable port so tests are deterministic.
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)
	return srv, cfg
}

func TestMemoryFactsEndpoint(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/facts", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatal("expected enabled=true")
	}
	if facts, ok := body["facts"].([]interface{}); !ok {
		t.Fatal("expected facts array")
	} else if len(facts) != 0 {
		t.Fatalf("expected empty facts, got %d", len(facts))
	}
	if count, ok := body["count"].(float64); !ok || count != 0 {
		t.Fatalf("expected count=0, got %v", count)
	}
}

func TestMemoryFactsEndpointWithData(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	// Store a fact via the pipeline's memory engine.
	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	if err := mem.StoreFact(memory.MemoryFact{
		Key:        "test:fact",
		Value:      "test value",
		Source:     "test",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/facts", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if count, ok := body["count"].(float64); !ok || count != 1 {
		t.Fatalf("expected count=1, got %v", count)
	}
}

func TestMemoryFactsEndpointSearch(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	if err := mem.StoreFact(memory.MemoryFact{
		Key:        "project:language",
		Value:      "Go",
		Source:     "test",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/facts?search=language", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if count, ok := body["count"].(float64); !ok || count != 1 {
		t.Fatalf("expected count=1 for search, got %v", count)
	}
}

func TestMemoryModelFitEndpoint(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/model-fit", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatal("expected enabled=true")
	}
	if entries, ok := body["entries"].([]interface{}); !ok {
		t.Fatal("expected entries array")
	} else if len(entries) != 0 {
		t.Fatalf("expected empty entries, got %d", len(entries))
	}
}

func TestMemoryModelFitEndpointWithData(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	if err := mem.RecordOutcome("test-model", 3, "feature", true, 1500, 1); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/model-fit", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if count, ok := body["count"].(float64); !ok || count != 1 {
		t.Fatalf("expected count=1, got %v", count)
	}
}

func TestMemoryClearEndpoint(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	if err := mem.StoreFact(memory.MemoryFact{
		Key:        "test:fact",
		Value:      "test value",
		Source:     "test",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Clear memory.
	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/clear", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if status, ok := body["status"].(string); !ok || status != "ok" {
		t.Fatalf("expected status 'ok', got %v", status)
	}

	// Verify facts are gone.
	facts, err := mem.ListFacts(100)
	if err != nil {
		t.Fatalf("ListFacts failed: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected 0 facts after clear, got %d", len(facts))
	}
}

func TestMemoryStatusEndpoint(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/status", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatal("expected enabled=true")
	}
	if _, ok := body["facts_count"]; !ok {
		t.Fatal("expected facts_count field")
	}
	if _, ok := body["model_fit_entries"]; !ok {
		t.Fatal("expected model_fit_entries field")
	}
	if _, ok := body["injection_budget"]; !ok {
		t.Fatal("expected injection_budget field")
	}
}

func TestMemoryCreateFactEndpoint(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	payload := `{"key":"test:created","value":"created value","source":"test","confidence":0.85}`
	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/facts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if status, ok := body["status"].(string); !ok || status != "ok" {
		t.Fatalf("expected status 'ok', got %v", status)
	}

	// Verify fact was stored.
	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	fact, err := mem.GetFact("test:created")
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}
	if fact.Value != "created value" {
		t.Fatalf("expected 'created value', got %q", fact.Value)
	}
}

func TestMemoryCreateFactEndpointMissingFields(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	// Missing value.
	payload := `{"key":"test:missing"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/facts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "MISSING_FIELDS" {
		t.Fatalf("expected MISSING_FIELDS, got %s", errResp.Error.Code)
	}
}

func TestMemoryCreateFactEndpointInvalidJSON(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/facts", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("expected INVALID_REQUEST, got %s", errResp.Error.Code)
	}
}

func TestMemoryCreateFactEndpointDefaultConfidence(t *testing.T) {
	srv, _ := testServerWithMemory(t)

	// No confidence specified — should default to 0.7.
	payload := `{"key":"test:default-conf","value":"some value"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/facts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	mem := srv.pipeline.MemoryEngine()
	if mem == nil {
		t.Fatal("expected non-nil memory engine")
	}
	fact, err := mem.GetFact("test:default-conf")
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}
	if fact.Confidence != 0.7 {
		t.Fatalf("expected default confidence 0.7, got %f", fact.Confidence)
	}
}

func TestMemoryEndpointsRequireAuth(t *testing.T) {
	// Create a server with auth enabled.
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Memory.Enabled = true
	cfg.Memory.DBPath = ""
	cfg.Storage.DBPath = t.TempDir() + "/novexa.db"
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)

	endpoints := []string{
		"GET /v1/novexa/memory/facts",
		"POST /v1/novexa/memory/facts",
		"GET /v1/novexa/memory/model-fit",
		"POST /v1/novexa/memory/clear",
		"GET /v1/novexa/memory/status",
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("auth_required_%s", strings.ReplaceAll(ep, " ", "_")), func(t *testing.T) {
			parts := strings.SplitN(ep, " ", 2)
			method := parts[0]
			path := parts[1]

			var bodyReader io.Reader
			if method == "POST" {
				bodyReader = strings.NewReader(`{}`)
			}
			req := httptest.NewRequest(method, path, bodyReader)
			if bodyReader != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d", method, path, rr.Code)
			}
		})
	}
}

func TestMemoryFactsEndpointMemoryDisabled(t *testing.T) {
	// Create a server with memory disabled.
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Memory.Enabled = false
	cfg.Storage.DBPath = t.TempDir() + "/novexa.db"
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/facts", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if enabled, ok := body["enabled"].(bool); !ok || enabled {
		t.Fatal("expected enabled=false when memory is disabled")
	}
}

func TestMemoryCreateFactEndpointMemoryDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Memory.Enabled = false
	cfg.Storage.DBPath = t.TempDir() + "/novexa.db"
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)

	payload := `{"key":"test:fact","value":"value"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/novexa/memory/facts", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp api.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "MEMORY_DISABLED" {
		t.Fatalf("expected MEMORY_DISABLED, got %s", errResp.Error.Code)
	}
}

func TestMemoryStatusEndpointMemoryDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Memory.Enabled = false
	cfg.Storage.DBPath = t.TempDir() + "/novexa.db"
	for key := range cfg.Providers {
		settings := cfg.Providers[key]
		settings.URL = "http://127.0.0.1:1"
		cfg.Providers[key] = settings
	}

	srv := testServerWithConfig(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/novexa/memory/status", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if enabled, ok := body["enabled"].(bool); !ok || enabled {
		t.Fatal("expected enabled=false when memory is disabled")
	}
}
