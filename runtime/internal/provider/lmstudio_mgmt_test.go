package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

// newLMStudioMgmtTestServer creates an httptest.Server that simulates the
// LM Studio v1 REST API for model management (load, unload, list).
func newLMStudioMgmtTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/models/load":
			var req LMStudioLoadRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}
			resp := LMStudioLoadResponse{
				Type:            "model_loaded",
				InstanceID:      "inst_" + req.Model,
				LoadTimeSeconds: 2.5,
				Status:          "loaded",
				LoadConfig: &LMStudioLoadedConfig{
					ContextLength:  req.ContextLength,
					FlashAttention: req.FlashAttention,
					OffloadKVCache: req.OffloadKVCache,
					EvalBatchSize:  req.EvalBatchSize,
					NumExperts:     req.NumExperts,
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "/api/v1/models/unload":
			var req LMStudioUnloadRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}
			resp := LMStudioUnloadResponse{InstanceID: req.InstanceID}
			json.NewEncoder(w).Encode(resp)

		case "/api/v1/models":
			resp := struct {
				Data []LMStudioModelEntry `json:"data"`
			}{
				Data: []LMStudioModelEntry{
					{Model: "qwen3-8b", Type: "local", Path: "/models/qwen3-8b.gguf", Size: "4.7 GB"},
					{Model: "qwen3-1.7b", Type: "local", Path: "/models/qwen3-1.7b.gguf", Size: "1.1 GB"},
				},
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		}
	}))
}

// newLMStudioMgmtAdapter creates an LMStudioAdapter with management config
// pointed at the given server URL.
func newLMStudioMgmtAdapter(t *testing.T, serverURL string, mgmtCfg *config.LMStudioMgmtConfig) *LMStudioAdapter {
	t.Helper()
	log := logger.New("error")
	settings := config.ProviderSettings{
		URL:             serverURL + "/v1",
		TimeoutSeconds:  5,
		ModelManagement: mgmtCfg,
	}
	adapter, err := NewLMStudioAdapter("lmstudio", settings, log)
	if err != nil {
		t.Fatalf("failed to create lmstudio adapter: %v", err)
	}
	return adapter.(*LMStudioAdapter)
}

func TestLMStudioLoadModel(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	resp, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	if resp.InstanceID != "inst_qwen3-8b" {
		t.Fatalf("expected instance_id 'inst_qwen3-8b', got %q", resp.InstanceID)
	}
	if resp.Status != "loaded" {
		t.Fatalf("expected status 'loaded', got %q", resp.Status)
	}
	if resp.LoadTimeSeconds <= 0 {
		t.Fatal("expected positive load_time_seconds")
	}
	if adapter.LoadedModelID() != "inst_qwen3-8b" {
		t.Fatalf("expected LoadedModelID 'inst_qwen3-8b', got %q", adapter.LoadedModelID())
	}
}

func TestLMStudioLoadModelWithConfig(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	ctxLen := 32768
	flashAttn := true
	offloadKV := true
	evalBatch := 512
	numExperts := 4

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	cfg := &LMStudioModelCfg{
		ContextLength:  &ctxLen,
		FlashAttention: &flashAttn,
		OffloadKVCache: &offloadKV,
		EvalBatchSize:  &evalBatch,
		NumExperts:     &numExperts,
	}

	resp, err := adapter.LoadModel(ctx, "qwen3-8b", cfg)
	if err != nil {
		t.Fatalf("LoadModel with config failed: %v", err)
	}
	if resp.LoadConfig == nil {
		t.Fatal("expected load_config in response")
	}
	if resp.LoadConfig.ContextLength == nil || *resp.LoadConfig.ContextLength != 32768 {
		t.Fatalf("expected context_length 32768, got %v", resp.LoadConfig.ContextLength)
	}
	if resp.LoadConfig.FlashAttention == nil || !*resp.LoadConfig.FlashAttention {
		t.Fatal("expected flash_attention true")
	}
	if resp.LoadConfig.OffloadKVCache == nil || !*resp.LoadConfig.OffloadKVCache {
		t.Fatal("expected offload_kv_cache true")
	}
	if resp.LoadConfig.EvalBatchSize == nil || *resp.LoadConfig.EvalBatchSize != 512 {
		t.Fatalf("expected eval_batch_size 512, got %v", resp.LoadConfig.EvalBatchSize)
	}
}

func TestLMStudioLoadModelWithDefaults(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	defaultFlashAttn := true
	defaultOffloadKV := true
	mgmtCfg := &config.LMStudioMgmtConfig{
		Enabled:               true,
		DefaultContextLength:  16384,
		DefaultFlashAttention: &defaultFlashAttn,
		DefaultOffloadKV:      &defaultOffloadKV,
		DefaultEvalBatchSize:  256,
	}

	adapter := newLMStudioMgmtAdapter(t, server.URL, mgmtCfg)
	ctx := context.Background()

	resp, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("LoadModel with defaults failed: %v", err)
	}
	if resp.LoadConfig == nil {
		t.Fatal("expected load_config in response")
	}
	if resp.LoadConfig.ContextLength == nil || *resp.LoadConfig.ContextLength != 16384 {
		t.Fatalf("expected context_length 16384 from defaults, got %v", resp.LoadConfig.ContextLength)
	}
}

func TestLMStudioLoadModelWithPerModelOverride(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	perModelCtx := 8192
	mgmtCfg := &config.LMStudioMgmtConfig{
		Enabled:              true,
		DefaultContextLength: 16384,
		ModelConfig: map[string]config.LMStudioModelCfg{
			"qwen3-8b": {ContextLength: &perModelCtx},
		},
	}

	adapter := newLMStudioMgmtAdapter(t, server.URL, mgmtCfg)
	ctx := context.Background()

	// Without explicit cfg — per-model override should apply.
	resp, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("LoadModel with per-model override failed: %v", err)
	}
	if resp.LoadConfig.ContextLength == nil || *resp.LoadConfig.ContextLength != 8192 {
		t.Fatalf("expected context_length 8192 from per-model override, got %v", resp.LoadConfig.ContextLength)
	}
}

func TestLMStudioLoadModelWithExplicitOverridesPerModel(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	perModelCtx := 8192
	explicitCtx := 4096
	mgmtCfg := &config.LMStudioMgmtConfig{
		Enabled:              true,
		DefaultContextLength: 16384,
		ModelConfig: map[string]config.LMStudioModelCfg{
			"qwen3-8b": {ContextLength: &perModelCtx},
		},
	}

	adapter := newLMStudioMgmtAdapter(t, server.URL, mgmtCfg)
	ctx := context.Background()

	// Explicit cfg is applied, but per-model config from YAML overrides it.
	cfg := &LMStudioModelCfg{ContextLength: &explicitCtx}
	resp, err := adapter.LoadModel(ctx, "qwen3-8b", cfg)
	if err != nil {
		t.Fatalf("LoadModel with explicit cfg failed: %v", err)
	}
	// Per-model config (8192) wins over explicit cfg (4096).
	if resp.LoadConfig.ContextLength == nil || *resp.LoadConfig.ContextLength != 8192 {
		t.Fatalf("expected context_length 8192 from per-model override, got %v", resp.LoadConfig.ContextLength)
	}
}

func TestLMStudioUnloadModel(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	// First load a model so we have an instance ID.
	loadResp, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}

	// Unload it.
	if err := adapter.UnloadModel(ctx, loadResp.InstanceID); err != nil {
		t.Fatalf("UnloadModel failed: %v", err)
	}
	if adapter.LoadedModelID() != "" {
		t.Fatal("expected LoadedModelID to be empty after unload")
	}
}

func TestLMStudioUnloadModelClearsInstanceID(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	// Load a model.
	loadResp, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}
	if adapter.LoadedModelID() == "" {
		t.Fatal("expected non-empty LoadedModelID after load")
	}

	// Unload a different instance ID — should not clear the loaded ID.
	if err := adapter.UnloadModel(ctx, "some-other-instance"); err != nil {
		t.Fatalf("UnloadModel failed: %v", err)
	}
	if adapter.LoadedModelID() != loadResp.InstanceID {
		t.Fatalf("expected LoadedModelID to remain %q, got %q", loadResp.InstanceID, adapter.LoadedModelID())
	}
}

// Regression: auto-unload must pass the previous model ID, not an empty string.
func TestLMStudioAutoUnloadPreviousModelID(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	// Load a first model.
	loadResp1, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err != nil {
		t.Fatalf("first LoadModel failed: %v", err)
	}
	if adapter.LoadedModelID() == "" {
		t.Fatal("expected non-empty LoadedModelID after first load")
	}

	// Load a second, different model.
	loadResp2, err := adapter.LoadModel(ctx, "qwen3-1.7b", nil)
	if err != nil {
		t.Fatalf("second LoadModel failed: %v", err)
	}

	// The two loads should have different instance IDs.
	if loadResp1.InstanceID == loadResp2.InstanceID {
		t.Fatalf("expected different instance IDs for different models, got both %s", loadResp1.InstanceID)
	}

	// Unload the first model by its instance ID (simulating auto-unload).
	if err := adapter.UnloadModel(ctx, loadResp1.InstanceID); err != nil {
		t.Fatalf("UnloadModel with previous instance ID failed: %v", err)
	}
	// The loaded ID should still be the second model.
	if adapter.LoadedModelID() != loadResp2.InstanceID {
		t.Fatalf("expected LoadedModelID to still be %s, got %s", loadResp2.InstanceID, adapter.LoadedModelID())
	}

	// Now unload the second model.
	if err := adapter.UnloadModel(ctx, loadResp2.InstanceID); err != nil {
		t.Fatalf("UnloadModel with current instance ID failed: %v", err)
	}
	if adapter.LoadedModelID() != "" {
		t.Fatal("expected LoadedModelID to be empty after unloading the current model")
	}
}

func TestLMStudioListAvailableModels(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	models, err := adapter.ListAvailableModels(ctx)
	if err != nil {
		t.Fatalf("ListAvailableModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Model != "qwen3-8b" {
		t.Fatalf("expected first model 'qwen3-8b', got %q", models[0].Model)
	}
	if models[1].Model != "qwen3-1.7b" {
		t.Fatalf("expected second model 'qwen3-1.7b', got %q", models[1].Model)
	}
}

func TestLMStudioListAvailableModelsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []LMStudioModelEntry{},
		})
	}))
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	models, err := adapter.ListAvailableModels(ctx)
	if err != nil {
		t.Fatalf("ListAvailableModels failed: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
}

func TestLMStudioLoadModelServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	_, err := adapter.LoadModel(ctx, "qwen3-8b", nil)
	if err == nil {
		t.Fatal("expected error from server error response")
	}
}

func TestLMStudioUnloadModelServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"error":"not found"}`)
	}))
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	err := adapter.UnloadModel(ctx, "inst_123")
	if err == nil {
		t.Fatal("expected error from server error response")
	}
}

func TestLMStudioListAvailableModelsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintln(w, `{"error":"unavailable"}`)
	}))
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})
	ctx := context.Background()

	_, err := adapter.ListAvailableModels(ctx)
	if err == nil {
		t.Fatal("expected error from server error response")
	}
}

func TestLMStudioBuildPerModelConfig(t *testing.T) {
	ctxLen := 4096
	flashAttn := true
	offloadKV := false
	evalBatch := 128
	numExperts := 2

	mgmtCfg := &config.LMStudioMgmtConfig{
		Enabled: true,
		ModelConfig: map[string]config.LMStudioModelCfg{
			"test-model": {
				ContextLength:  &ctxLen,
				FlashAttention: &flashAttn,
				OffloadKVCache: &offloadKV,
				EvalBatchSize:  &evalBatch,
				NumExperts:     &numExperts,
			},
		},
	}

	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, mgmtCfg)

	cfg := adapter.BuildPerModelConfig("test-model")
	if cfg == nil {
		t.Fatal("expected non-nil per-model config")
	}
	if cfg.ContextLength == nil || *cfg.ContextLength != 4096 {
		t.Fatalf("expected context_length 4096, got %v", cfg.ContextLength)
	}
	if cfg.FlashAttention == nil || !*cfg.FlashAttention {
		t.Fatal("expected flash_attention true")
	}
	if cfg.OffloadKVCache == nil || *cfg.OffloadKVCache {
		t.Fatal("expected offload_kv_cache false")
	}
	if cfg.EvalBatchSize == nil || *cfg.EvalBatchSize != 128 {
		t.Fatalf("expected eval_batch_size 128, got %v", cfg.EvalBatchSize)
	}
	if cfg.NumExperts == nil || *cfg.NumExperts != 2 {
		t.Fatalf("expected num_experts 2, got %v", cfg.NumExperts)
	}
}

func TestLMStudioBuildPerModelConfigReturnsNilForUnknownModel(t *testing.T) {
	mgmtCfg := &config.LMStudioMgmtConfig{
		Enabled:     true,
		ModelConfig: map[string]config.LMStudioModelCfg{},
	}

	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, mgmtCfg)

	cfg := adapter.BuildPerModelConfig("nonexistent-model")
	if cfg != nil {
		t.Fatal("expected nil for unknown model")
	}
}

func TestLMStudioBuildPerModelConfigReturnsNilWhenNoModelConfig(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})

	cfg := adapter.BuildPerModelConfig("test-model")
	if cfg != nil {
		t.Fatal("expected nil when no model config map exists")
	}
}

func TestLMStudioMgmtAPIPath(t *testing.T) {
	server := newLMStudioMgmtTestServer(t)
	defer server.Close()

	adapter := newLMStudioMgmtAdapter(t, server.URL, &config.LMStudioMgmtConfig{Enabled: true})

	// mgmtAPIPath should strip /v1 and use /api/v1 prefix.
	path := adapter.mgmtAPIPath("/models/load")
	expected := server.URL + "/api/v1/models/load"
	if path != expected {
		t.Fatalf("expected mgmt path %q, got %q", expected, path)
	}
}

func TestLMStudioModelManagerInterface(t *testing.T) {
	// Verify that *LMStudioAdapter implements the ModelManager interface.
	var mm ModelManager = &LMStudioAdapter{}
	_ = mm
}
