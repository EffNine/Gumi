package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// LMStudioModelCfg holds per-model overrides for LM Studio load configuration.
// Duplicated from config.LMStudioModelCfg to avoid import cycle.
type LMStudioModelCfg struct {
	ContextLength  *int  `json:"context_length,omitempty"`
	FlashAttention *bool `json:"flash_attention,omitempty"`
	OffloadKVCache *bool `json:"offload_kv_cache_to_gpu,omitempty"`
	EvalBatchSize  *int  `json:"eval_batch_size,omitempty"`
	NumExperts     *int  `json:"num_experts,omitempty"`
}

// ModelManager is an optional interface that provider adapters can implement
// to support model lifecycle management (load, unload, list available models).
type ModelManager interface {
	// LoadModel loads a model into memory with optional configuration.
	LoadModel(ctx context.Context, modelID string, cfg *LMStudioModelCfg) (*LMStudioLoadResponse, error)

	// UnloadModel unloads a model from memory by instance ID.
	UnloadModel(ctx context.Context, instanceID string) error

	// ListAvailableModels returns models available on disk.
	ListAvailableModels(ctx context.Context) ([]LMStudioModelEntry, error)

	// LoadedModelID returns the instance ID of the currently loaded model.
	LoadedModelID() string

	// BuildPerModelConfig returns per-model config overrides from the
	// adapter's management configuration, or nil if none exist.
	BuildPerModelConfig(modelID string) *LMStudioModelCfg
}

// mgmtAPIPath returns the full URL for a v1 REST API endpoint suffix.
// The adapter's baseURL typically ends with /v1 (OpenAI-compatible), so the
// management API lives at /api/v1/...  (one level up → /api/v1).
//
// Examples:
//
//	baseURL = "http://localhost:1234/v1"
//	mgmtAPIPath("/models/load") → "http://localhost:1234/api/v1/models/load"
func (l *LMStudioAdapter) mgmtAPIPath(suffix string) string {
	base := strings.TrimSuffix(l.baseURL, "/v1")
	base = strings.TrimSuffix(base, "/")
	return base + "/api/v1" + suffix
}

// ────────────────────────────────────────────────────────────────────────────
// Types
// ────────────────────────────────────────────────────────────────────────────

// LMStudioLoadRequest is the POST /api/v1/models/load body.
type LMStudioLoadRequest struct {
	Model          string `json:"model"`
	ContextLength  *int   `json:"context_length,omitempty"`
	FlashAttention *bool  `json:"flash_attention,omitempty"`
	OffloadKVCache *bool  `json:"offload_kv_cache_to_gpu,omitempty"`
	EvalBatchSize  *int   `json:"eval_batch_size,omitempty"`
	NumExperts     *int   `json:"num_experts,omitempty"`
	EchoLoadConfig bool   `json:"echo_load_config,omitempty"`
}

// LMStudioLoadResponse is the response from POST /api/v1/models/load.
type LMStudioLoadResponse struct {
	Type            string                `json:"type"`
	InstanceID      string                `json:"instance_id"`
	LoadTimeSeconds float64               `json:"load_time_seconds"`
	Status          string                `json:"status"`
	LoadConfig      *LMStudioLoadedConfig `json:"load_config,omitempty"`
}

// LMStudioLoadedConfig echoes the final config applied to the loaded model.
type LMStudioLoadedConfig struct {
	ContextLength  *int  `json:"context_length,omitempty"`
	FlashAttention *bool `json:"flash_attention,omitempty"`
	OffloadKVCache *bool `json:"offload_kv_cache_to_gpu,omitempty"`
	EvalBatchSize  *int  `json:"eval_batch_size,omitempty"`
	NumExperts     *int  `json:"num_experts,omitempty"`
}

// LMStudioUnloadRequest is the POST /api/v1/models/unload body.
type LMStudioUnloadRequest struct {
	InstanceID string `json:"instance_id"`
}

// LMStudioUnloadResponse is the response from POST /api/v1/models/unload.
type LMStudioUnloadResponse struct {
	InstanceID string `json:"instance_id"`
}

// LMStudioModelEntry is one entry from GET /api/v1/models.
type LMStudioModelEntry struct {
	Model string `json:"model"`
	Type  string `json:"type,omitempty"`
	Path  string `json:"path,omitempty"`
	Size  string `json:"size,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// LoadModel
// ────────────────────────────────────────────────────────────────────────────

// LoadModel sends POST /api/v1/models/load to LM Studio. If config is nil,
// default values from the adapter's management config are used.
func (l *LMStudioAdapter) LoadModel(ctx context.Context, modelID string, cfg *LMStudioModelCfg) (*LMStudioLoadResponse, error) {
	url := l.mgmtAPIPath("/models/load")

	req := l.buildLoadRequest(modelID, cfg)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: marshal load request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: create load request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: load request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: read load response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(respBody)
		if len(preview) > 240 {
			preview = preview[:240] + "..."
		}
		return nil, fmt.Errorf("lmstudio mgmt: load returned status %d: %s", resp.StatusCode, preview)
	}

	var loadResp LMStudioLoadResponse
	if err := json.Unmarshal(respBody, &loadResp); err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: decode load response: %w", err)
	}

	l.loadedInstanceID = loadResp.InstanceID
	return &loadResp, nil
}

// buildLoadRequest assembles a load request using the best available config:
// per-model override → management defaults → zero values (let LM Studio decide).
func (l *LMStudioAdapter) buildLoadRequest(modelID string, cfg *LMStudioModelCfg) LMStudioLoadRequest {
	req := LMStudioLoadRequest{
		Model:          modelID,
		EchoLoadConfig: true,
	}

	mc := l.mgmtConfig
	if mc == nil || !mc.Enabled {
		// No management config — just send the model ID.
		return req
	}

	// Start with defaults from management config.
	if mc.DefaultContextLength > 0 {
		v := mc.DefaultContextLength
		req.ContextLength = &v
	}
	if mc.DefaultFlashAttention != nil {
		req.FlashAttention = mc.DefaultFlashAttention
	}
	if mc.DefaultOffloadKV != nil {
		req.OffloadKVCache = mc.DefaultOffloadKV
	}
	if mc.DefaultEvalBatchSize > 0 {
		v := mc.DefaultEvalBatchSize
		req.EvalBatchSize = &v
	}

	// If we have a per-model config, overlay it.
	if cfg != nil {
		if cfg.ContextLength != nil {
			req.ContextLength = cfg.ContextLength
		}
		if cfg.FlashAttention != nil {
			req.FlashAttention = cfg.FlashAttention
		}
		if cfg.OffloadKVCache != nil {
			req.OffloadKVCache = cfg.OffloadKVCache
		}
		if cfg.EvalBatchSize != nil {
			req.EvalBatchSize = cfg.EvalBatchSize
		}
		if cfg.NumExperts != nil {
			req.NumExperts = cfg.NumExperts
		}
	}

	// Also check the per-model overrides from the config file map.
	if mc.ModelConfig != nil {
		if override, ok := mc.ModelConfig[modelID]; ok {
			if override.ContextLength != nil {
				req.ContextLength = override.ContextLength
			}
			if override.FlashAttention != nil {
				req.FlashAttention = override.FlashAttention
			}
			if override.OffloadKVCache != nil {
				req.OffloadKVCache = override.OffloadKVCache
			}
			if override.EvalBatchSize != nil {
				req.EvalBatchSize = override.EvalBatchSize
			}
			if override.NumExperts != nil {
				req.NumExperts = override.NumExperts
			}
		}
	}

	return req
}

// ────────────────────────────────────────────────────────────────────────────
// UnloadModel
// ────────────────────────────────────────────────────────────────────────────

// UnloadModel sends POST /api/v1/models/unload to LM Studio.
func (l *LMStudioAdapter) UnloadModel(ctx context.Context, instanceID string) error {
	url := l.mgmtAPIPath("/models/unload")

	body, err := json.Marshal(LMStudioUnloadRequest{InstanceID: instanceID})
	if err != nil {
		return fmt.Errorf("lmstudio mgmt: marshal unload request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("lmstudio mgmt: create unload request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("lmstudio mgmt: unload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("lmstudio mgmt: read unload response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(respBody)
		if len(preview) > 240 {
			preview = preview[:240] + "..."
		}
		return fmt.Errorf("lmstudio mgmt: unload returned status %d: %s", resp.StatusCode, preview)
	}

	var unloadResp LMStudioUnloadResponse
	if err := json.Unmarshal(respBody, &unloadResp); err != nil {
		return fmt.Errorf("lmstudio mgmt: decode unload response: %w", err)
	}

	if l.loadedInstanceID == instanceID {
		l.loadedInstanceID = ""
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// ListAvailableModels
// ────────────────────────────────────────────────────────────────────────────

// ListAvailableModels calls GET /api/v1/models to list models available on
// disk (not just the currently loaded one). Returns the raw entries.
func (l *LMStudioAdapter) ListAvailableModels(ctx context.Context) ([]LMStudioModelEntry, error) {
	url := l.mgmtAPIPath("/models")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: create list request: %w", err)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: list request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: read list response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(respBody)
		if len(preview) > 240 {
			preview = preview[:240] + "..."
		}
		return nil, fmt.Errorf("lmstudio mgmt: list returned status %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Data []LMStudioModelEntry `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("lmstudio mgmt: decode list response: %w", err)
	}
	return result.Data, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Helper: buildPerModelConfig
// ────────────────────────────────────────────────────────────────────────────

// BuildPerModelConfig looks up a per-model config override from the adapter's
// management config and converts it to the local LMStudioModelCfg type.
// Returns nil if no override exists.
func (l *LMStudioAdapter) BuildPerModelConfig(modelID string) *LMStudioModelCfg {
	if l.mgmtConfig == nil || l.mgmtConfig.ModelConfig == nil {
		return nil
	}
	override, ok := l.mgmtConfig.ModelConfig[modelID]
	if !ok {
		return nil
	}
	// Convert from config.LMStudioModelCfg to local LMStudioModelCfg.
	// Both structs have identical fields so this is a field-by-field copy.
	cfg := &LMStudioModelCfg{
		ContextLength:  override.ContextLength,
		FlashAttention: override.FlashAttention,
		OffloadKVCache: override.OffloadKVCache,
		EvalBatchSize:  override.EvalBatchSize,
		NumExperts:     override.NumExperts,
	}
	return cfg
}

// ensure *LMStudioAdapter implements the optional ModelManager interface.
var _ ModelManager = (*LMStudioAdapter)(nil)
