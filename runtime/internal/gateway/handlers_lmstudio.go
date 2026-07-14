package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/provider"
	"gopkg.in/yaml.v3"
)

// ────────────────────────────────────────────────────────────────────────────
// LM Studio Model Management Handlers
// ────────────────────────────────────────────────────────────────────────────

// lmStudioProviderAdapter returns the LM Studio adapter if it's configured and implements ModelManager.
func (s *Server) lmStudioProviderAdapter() (provider.ModelManager, error) {
	adapter, ok := s.manager.Adapter("lmstudio")
	if !ok {
		return nil, fmt.Errorf("lmstudio provider is not configured")
	}
	mm, ok := adapter.(provider.ModelManager)
	if !ok {
		return nil, fmt.Errorf("lmstudio adapter does not support model management")
	}
	return mm, nil
}

// handleLMStudioModels returns models available on disk from LM Studio.
func (s *Server) handleLMStudioModels(w http.ResponseWriter, r *http.Request) {
	mm, err := s.lmStudioProviderAdapter()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRuntimeError("PROVIDER_UNAVAILABLE", err.Error(), requestIDFromContext(r.Context())))
		return
	}

	models, err := mm.ListAvailableModels(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, api.NewRuntimeError("LMSTUDIO_ERROR", "failed to list models: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	if models == nil {
		models = []provider.LMStudioModelEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": models,
	})
}

// handleLMStudioLoadModel loads a model on LM Studio.
func (s *Server) handleLMStudioLoadModel(w http.ResponseWriter, r *http.Request) {
	mm, err := s.lmStudioProviderAdapter()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRuntimeError("PROVIDER_UNAVAILABLE", err.Error(), requestIDFromContext(r.Context())))
		return
	}

	var req struct {
		ModelID string                     `json:"model_id"`
		Config  *provider.LMStudioModelCfg `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}
	if req.ModelID == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "model_id is required", requestIDFromContext(r.Context())))
		return
	}

	// Build per-model config from management config if not provided
	cfg := req.Config
	if cfg == nil {
		cfg = mm.BuildPerModelConfig(req.ModelID)
	}

	result, err := mm.LoadModel(r.Context(), req.ModelID, cfg)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, api.NewRuntimeError("LMSTUDIO_ERROR", "failed to load model: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleLMStudioUnloadModel unloads a model from LM Studio.
func (s *Server) handleLMStudioUnloadModel(w http.ResponseWriter, r *http.Request) {
	mm, err := s.lmStudioProviderAdapter()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRuntimeError("PROVIDER_UNAVAILABLE", err.Error(), requestIDFromContext(r.Context())))
		return
	}

	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}
	if req.InstanceID == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "instance_id is required", requestIDFromContext(r.Context())))
		return
	}

	if err := mm.UnloadModel(r.Context(), req.InstanceID); err != nil {
		s.writeError(w, http.StatusBadGateway, api.NewRuntimeError("LMSTUDIO_ERROR", "failed to unload model: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "instance_id": req.InstanceID})
}

// handleLMStudioLoaded returns the currently loaded model on LM Studio.
func (s *Server) handleLMStudioLoaded(w http.ResponseWriter, r *http.Request) {
	mm, err := s.lmStudioProviderAdapter()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRuntimeError("PROVIDER_UNAVAILABLE", err.Error(), requestIDFromContext(r.Context())))
		return
	}

	loadedID := mm.LoadedModelID()

	writeJSON(w, http.StatusOK, map[string]any{
		"loaded_instance_id": loadedID,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Config Save Handler
// ────────────────────────────────────────────────────────────────────────────

// configSaveRequest is the payload for POST /v1/gumi/config.
type configSaveRequest struct {
	ProviderDefault string `json:"provider_default,omitempty"`
	Providers       []struct {
		Name         string `json:"name"`
		Enabled      *bool  `json:"enabled,omitempty"`
		URL          string `json:"url,omitempty"`
		DefaultModel string `json:"default_model,omitempty"`
		TimeoutSecs  int    `json:"timeout_seconds,omitempty"`
	} `json:"providers,omitempty"`
	Runtime struct {
		Mode     string `json:"mode,omitempty"`
		LogLevel string `json:"log_level,omitempty"`
	} `json:"runtime,omitempty"`
	Telemetry struct {
		LogPrompts   *bool `json:"log_prompts,omitempty"`
		LogResponses *bool `json:"log_responses,omitempty"`
	} `json:"telemetry,omitempty"`
}

// handleConfigSave saves runtime configuration updates to ~/.gumi/gumi.yaml.
func (s *Server) handleConfigSave(w http.ResponseWriter, r *http.Request) {
	var req configSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}

	// Determine config path
	home, err := os.UserHomeDir()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "cannot determine home directory", requestIDFromContext(r.Context())))
		return
	}
	configDir := filepath.Join(home, ".gumi")
	configPath := filepath.Join(configDir, "gumi.yaml")

	// Load existing config or start fresh
	existingData := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = yaml.Unmarshal(data, &existingData)
	}

	// Apply provider default
	if req.ProviderDefault != "" {
		existingData["provider"] = map[string]any{
			"default": req.ProviderDefault,
		}
	}

	// Apply provider overrides
	if len(req.Providers) > 0 {
		providersMap := map[string]any{}
		if p, ok := existingData["providers"]; ok {
			if pm, ok := p.(map[string]any); ok {
				providersMap = pm
			}
		}
		for _, p := range req.Providers {
			entry := map[string]any{}
			if existing, ok := providersMap[p.Name]; ok {
				if em, ok := existing.(map[string]any); ok {
					for k, v := range em {
						entry[k] = v
					}
				}
			}
			if p.Enabled != nil {
				entry["enabled"] = *p.Enabled
			}
			if p.URL != "" {
				entry["url"] = p.URL
			}
			if p.DefaultModel != "" {
				entry["default_model"] = p.DefaultModel
			}
			if p.TimeoutSecs > 0 {
				entry["timeout_seconds"] = p.TimeoutSecs
			}
			providersMap[p.Name] = entry
		}
		existingData["providers"] = providersMap
	}

	// Apply runtime mode
	if req.Runtime.Mode != "" || req.Runtime.LogLevel != "" {
		runtimeMap := map[string]any{}
		if r, ok := existingData["runtime"]; ok {
			if rm, ok := r.(map[string]any); ok {
				for k, v := range rm {
					runtimeMap[k] = v
				}
			}
		}
		if req.Runtime.Mode != "" {
			runtimeMap["mode"] = req.Runtime.Mode
		}
		if req.Runtime.LogLevel != "" {
			runtimeMap["log_level"] = req.Runtime.LogLevel
		}
		existingData["runtime"] = runtimeMap
	}

	// Apply telemetry
	if req.Telemetry.LogPrompts != nil || req.Telemetry.LogResponses != nil {
		telemetryMap := map[string]any{}
		if t, ok := existingData["telemetry"]; ok {
			if tm, ok := t.(map[string]any); ok {
				for k, v := range tm {
					telemetryMap[k] = v
				}
			}
		}
		if req.Telemetry.LogPrompts != nil {
			telemetryMap["log_prompts"] = *req.Telemetry.LogPrompts
		}
		if req.Telemetry.LogResponses != nil {
			telemetryMap["log_responses"] = *req.Telemetry.LogResponses
		}
		existingData["telemetry"] = telemetryMap
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "cannot create config directory: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	// Marshal and write
	out, err := yaml.Marshal(existingData)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to marshal config: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to write config: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	s.log.Info("config saved via dashboard", "path", configPath)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"path":             configPath,
		"restart_required": true,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Log Streaming Handler
// ────────────────────────────────────────────────────────────────────────────

// logSubscriber receives log lines.
type logSubscriber struct {
	ch   chan string
	done chan struct{}
}

var (
	logSubsMu      sync.Mutex
	logSubscribers []*logSubscriber
)

// LogLine is broadcast to all dashboard subscribers.
func BroadcastLogLine(line string) {
	logSubsMu.Lock()
	defer logSubsMu.Unlock()
	for _, sub := range logSubscribers {
		select {
		case sub.ch <- line:
		default:
			// Drop if subscriber is slow
		}
	}
}

// handleLogStream serves runtime logs as an SSE stream.
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("STREAMING_NOT_SUPPORTED", "streaming not supported", requestIDFromContext(r.Context())))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := &logSubscriber{
		ch:   make(chan string, 64),
		done: make(chan struct{}),
	}

	logSubsMu.Lock()
	logSubscribers = append(logSubscribers, sub)
	logSubsMu.Unlock()

	// Remove subscriber on disconnect
	defer func() {
		logSubsMu.Lock()
		for i, s := range logSubscribers {
			if s == sub {
				logSubscribers = append(logSubscribers[:i], logSubscribers[i+1:]...)
				break
			}
		}
		logSubsMu.Unlock()
	}()

	ctx := r.Context()
	// Send initial connected event
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case line := <-sub.ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

// logWriter creates a wrapper that broadcasts log lines to dashboard subscribers.
type dashboardLogWriter struct {
	next http.HandlerFunc
}

func (w *dashboardLogWriter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	// Wrap the response writer to capture log output? No, we use a different approach.
	// Instead, we let the gateway's logger broadcast via BroadcastLogLine.
	w.next(rw, r)
}

// initLogBroadcast patches the logger to broadcast to dashboard subscribers.
// This is called once during server startup.
func init() {
	// We override the logger after server creation via a hook.
	// For now, the gateway logger will call BroadcastLogLine manually.
}

// LogLine represents a single log entry for streaming.
type LogLine struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Fields    string `json:"fields,omitempty"`
}

// FormatLogLine formats a structured log entry for SSE streaming.
func FormatLogLine(timestamp, level, msg string, pairs ...any) string {
	fields := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		key := fmt.Sprintf("%v", pairs[i])
		val := fmt.Sprintf("%v", pairs[i+1])
		fields += fmt.Sprintf(" %s=%s", key, val)
	}
	line := LogLine{
		Timestamp: timestamp,
		Level:     level,
		Message:   msg,
		Fields:    fields,
	}
	data, _ := json.Marshal(line)
	return string(data)
}

// BroadcastLog broadcasts a structured log entry to all dashboard subscribers.
func BroadcastLog(timestamp, level, msg string, pairs ...any) {
	line := FormatLogLine(timestamp, level, msg, pairs...)
	BroadcastLogLine(line)
}
