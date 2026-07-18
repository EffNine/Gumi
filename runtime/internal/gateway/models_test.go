package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
)

func TestModelRegistry_List(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "my-model", Provider: "ollama", ModelID: "llama3", ContextLength: 8192, Enabled: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/models", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelRegistryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Enabled {
		t.Error("expected enabled=true")
	}
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Models))
	}
	if resp.Models[0].Alias != "my-model" {
		t.Errorf("expected alias my-model, got %s", resp.Models[0].Alias)
	}
}

func TestModelRegistry_CreatePersistsToConfig(t *testing.T) {
	srv, _, _ := testServer(t, "local")
	configPath := filepath.Join(t.TempDir(), "gumi.yaml")
	srv.ConfigPath = configPath

	body := `{"alias":"persisted-model","provider":"ollama","model_id":"llama3","context_length":4096,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file at %s: %v", configPath, err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if len(cfg.Models) != 1 {
		t.Fatalf("expected 1 persisted model, got %d", len(cfg.Models))
	}
	if cfg.Models[0].Alias != "persisted-model" {
		t.Errorf("expected alias persisted-model, got %s", cfg.Models[0].Alias)
	}
}

func TestModelRegistry_Create(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	body := `{"alias":"test-model","provider":"ollama","model_id":"llama3","context_length":4096,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelRegistryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Models))
	}
	if resp.Models[0].Alias != "test-model" {
		t.Errorf("expected alias test-model, got %s", resp.Models[0].Alias)
	}
}

func TestModelRegistry_CreateDuplicateAlias(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "existing", Provider: "ollama", ModelID: "llama3", ContextLength: 4096, Enabled: true},
	}

	body := `{"alias":"existing","provider":"ollama","model_id":"llama3","context_length":4096,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_CreateMissingFields(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	// Missing alias
	body := `{"provider":"ollama","model_id":"llama3"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_Update(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "my-model", Provider: "ollama", ModelID: "llama3", ContextLength: 4096, Enabled: true},
	}

	body := `{"alias":"my-model","provider":"lmstudio","model_id":"qwen3-8b","context_length":8192,"enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/v1/gumi/models/my-model", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelRegistryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp.Models))
	}
	if resp.Models[0].Provider != "lmstudio" {
		t.Errorf("expected provider lmstudio, got %s", resp.Models[0].Provider)
	}
	if resp.Models[0].ModelID != "qwen3-8b" {
		t.Errorf("expected model_id qwen3-8b, got %s", resp.Models[0].ModelID)
	}
}

func TestModelRegistry_UpdateNotFound(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	body := `{"alias":"nonexistent","provider":"ollama","model_id":"llama3"}`
	req := httptest.NewRequest(http.MethodPut, "/v1/gumi/models/nonexistent", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer gumi-local")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_Delete(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "my-model", Provider: "ollama", ModelID: "llama3", ContextLength: 4096, Enabled: true},
		{Alias: "other", Provider: "lmstudio", ModelID: "qwen3-8b", ContextLength: 8192, Enabled: true},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/gumi/models/my-model", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelRegistryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Models) != 1 {
		t.Fatalf("expected 1 model after delete, got %d", len(resp.Models))
	}
	if resp.Models[0].Alias != "other" {
		t.Errorf("expected remaining alias other, got %s", resp.Models[0].Alias)
	}
}

func TestModelRegistry_DeleteNotFound(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodDelete, "/v1/gumi/models/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_SetDefault(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "model-a", Provider: "ollama", ModelID: "llama3", ContextLength: 4096, Enabled: true, Default: true},
		{Alias: "model-b", Provider: "lmstudio", ModelID: "qwen3-8b", ContextLength: 8192, Enabled: true},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models/model-b/default", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp modelRegistryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(resp.Models))
	}
	// model-b should now be default, model-a should not
	for _, m := range resp.Models {
		if m.Alias == "model-a" && m.Default {
			t.Error("model-a should not be default after model-b was set as default")
		}
		if m.Alias == "model-b" && !m.Default {
			t.Error("model-b should be default")
		}
	}
}

func TestModelRegistry_SetDefaultNotFound(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodPost, "/v1/gumi/models/nonexistent/default", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestModelRegistry_RequiresAuth(t *testing.T) {
	srv, _, _ := testServer(t, "local")

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/models", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestModelsEndpointMergesRegistry(t *testing.T) {
	srv, cfg, _ := testServer(t, "local")
	cfg.Models = []config.ModelRegistryEntry{
		{Alias: "my-registry-model", Provider: "ollama", ModelID: "llama3", ContextLength: 8192, Enabled: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer gumi-local")
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Object string      `json:"object"`
		Data   []api.Model `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	// Should contain local:auto, plus the registry model alias
	foundRegistry := false
	for _, m := range body.Data {
		if m.OwnedBy == "gumi-registry" && m.ID == "my-registry-model" {
			foundRegistry = true
			break
		}
	}
	if !foundRegistry {
		t.Error("expected registry model to appear in /v1/models data")
	}
}
