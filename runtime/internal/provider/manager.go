package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

// healthTTL is how long a cached health status is considered fresh.
const healthTTL = 30 * time.Second

// providerPreferenceOrder is the order used by local:auto when selecting a
// provider. Ollama is preferred first, then LM Studio, then OpenAI-compatible.
var providerPreferenceOrder = []string{"ollama", "lmstudio", "openai_compatible_local"}

// TelemetryWriter receives provider health observations. It is defined in the
// provider package so Manager can record health without creating an import cycle.
type TelemetryWriter interface {
	RecordProviderHealth(ctx context.Context, provider string, status ProviderStatus, latency time.Duration, err ProviderError)
}

// Manager owns the configured provider adapters and coordinates health checks,
// model discovery, and generation.
type Manager struct {
	adapters map[string]ProviderAdapter
	log      *logger.Logger

	mu          sync.RWMutex
	healthCache map[string]healthEntry

	// Telemetry receives health check results. It may be nil.
	Telemetry TelemetryWriter
}

type healthEntry struct {
	status    ProviderStatus
	timestamp time.Time
}

// NewManager creates a manager from a map of adapters keyed by provider key.
func NewManager(adapters map[string]ProviderAdapter, log *logger.Logger) *Manager {
	return &Manager{
		adapters:    adapters,
		log:         log,
		healthCache: make(map[string]healthEntry),
	}
}

// ListProviders returns the keys of all configured adapters in preference order.
func (m *Manager) ListProviders() []string {
	keys := make([]string, 0, len(m.adapters))
	for _, key := range providerPreferenceOrder {
		if _, ok := m.adapters[key]; ok {
			keys = append(keys, key)
		}
	}
	return keys
}

// HealthCheck returns the current status of a provider, using a cached value
// when fresh. Every fresh health check result is forwarded to telemetry.
func (m *Manager) HealthCheck(ctx context.Context, name string) (ProviderStatus, error) {
	adapter, ok := m.adapters[name]
	if !ok {
		return StatusUnknown, fmt.Errorf("provider %q is not configured", name)
	}

	m.mu.RLock()
	entry, ok := m.healthCache[name]
	m.mu.RUnlock()

	if ok && time.Since(entry.timestamp) < healthTTL {
		return entry.status, nil
	}

	start := time.Now()
	status, err := adapter.HealthCheck(ctx)
	latency := time.Since(start)

	m.mu.Lock()
	m.healthCache[name] = healthEntry{status: status, timestamp: time.Now()}
	m.mu.Unlock()

	var perr ProviderError
	if err != nil {
		perr = adapter.NormalizeError(err)
	}
	if m.Telemetry != nil {
		m.Telemetry.RecordProviderHealth(ctx, name, status, latency, perr)
	}

	return status, err
}

// ListModels returns models from all configured providers that are currently
// reachable. Errors from individual providers are logged but do not fail the
// overall call.
func (m *Manager) ListModels(ctx context.Context) []api.Model {
	var result []api.Model

	for key, adapter := range m.adapters {
		status, err := m.HealthCheck(ctx, key)
		if err != nil {
			m.log.Error("health check failed while listing models", err, "provider", key)
			continue
		}
		if status != StatusOK && status != StatusDegraded {
			m.log.Debug("skipping offline provider during model listing", "provider", key, "status", status)
			continue
		}

		models, err := adapter.ListModels(ctx)
		if err != nil {
			m.log.Error("model discovery failed", err, "provider", key)
			continue
		}

		prefix := modelIDPrefix(key)
		for _, info := range models {
			id := fmt.Sprintf("%s:%s", prefix, info.Name)
			created := info.CreatedAt.Unix()
			if created < 0 {
				created = 0
			}
			result = append(result, api.Model{
				ID:      id,
				Object:  "model",
				Created: created,
				OwnedBy: prefix,
			})
		}
	}

	return result
}

// ModelResolution describes a selected provider and model.
type ModelResolution struct {
	ProviderKey string
	ModelName   string
	Adapter     ProviderAdapter
}

// ResolveModel picks a provider and model name for a Gumi model ID.
// - "local:auto" selects the first online provider/model.
// - "ollama:llama3" selects the ollama adapter and model "llama3".
func (m *Manager) ResolveModel(ctx context.Context, modelID string) (*ModelResolution, ProviderError) {
	if modelID == "" {
		return nil, ProviderError{
			Code:       ProviderMisconfigured,
			Message:    "model ID is empty",
			Suggestion: "Provide a model ID such as 'local:auto' or 'ollama:llama3'.",
		}
	}

	if modelID == "local:auto" {
		return m.resolveAuto(ctx)
	}

	key := providerKeyFromModelID(modelID)
	if key == "" {
		return nil, ProviderError{
			Code:       ProviderMisconfigured,
			Message:    fmt.Sprintf("model ID %q has no recognized provider prefix", modelID),
			Suggestion: "Use 'local:auto', 'ollama:<model>', 'lmstudio:<model>', or 'openai-compatible:<model>'.",
		}
	}

	adapter, ok := m.adapters[key]
	if !ok {
		return nil, ProviderError{
			Code:       ProviderUnavailable,
			Message:    fmt.Sprintf("provider %q is not configured", key),
			Suggestion: fmt.Sprintf("Enable %s in the Gumi configuration or use 'local:auto'.", modelIDPrefix(key)),
		}
	}

	parts := strings.SplitN(modelID, ":", 2)
	modelName := parts[1]

	status, err := m.HealthCheck(ctx, key)
	if err != nil {
		return nil, adapter.NormalizeError(err)
	}
	if status != StatusOK && status != StatusDegraded {
		return nil, ProviderError{
			Code:       ProviderUnavailable,
			Message:    fmt.Sprintf("provider %s is %s", modelIDPrefix(key), status),
			Suggestion: fmt.Sprintf("Start %s or choose 'local:auto'.", modelIDPrefix(key)),
		}
	}

	return &ModelResolution{ProviderKey: key, ModelName: modelName, Adapter: adapter}, ProviderError{}
}

// resolveAuto returns the first reachable provider and one of its models.
func (m *Manager) resolveAuto(ctx context.Context) (*ModelResolution, ProviderError) {
	keys := m.ListProviders()

	for _, key := range keys {
		adapter := m.adapters[key]
		status, err := m.HealthCheck(ctx, key)
		if err != nil {
			m.log.Debug("auto-select skipped provider", "provider", key, "error", err)
			continue
		}
		if status != StatusOK && status != StatusDegraded {
			continue
		}

		models, err := adapter.ListModels(ctx)
		if err != nil {
			m.log.Error("model discovery failed during auto-select", err, "provider", key)
			continue
		}
		if len(models) == 0 {
			continue
		}

		return &ModelResolution{
			ProviderKey: key,
			ModelName:   models[0].Name,
			Adapter:     adapter,
		}, ProviderError{}
	}

	return nil, ProviderError{
		Code:       ProviderUnavailable,
		Message:    "no local provider is currently available for 'local:auto'",
		Suggestion: "Start Ollama, LM Studio, or an OpenAI-compatible local server, then retry.",
	}
}

// Adapter returns the configured adapter for a provider key, if any.
func (m *Manager) Adapter(key string) (ProviderAdapter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	adapter, ok := m.adapters[key]
	return adapter, ok
}

// Generate delegates a chat completion to the provider selected by model ID.
// It returns the provider response, the provider key that served it, and any
// normalized error.
func (m *Manager) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, string, ProviderError) {
	resolution, perr := m.ResolveModel(ctx, req.Model)
	if perr.Code != "" {
		return nil, "", perr
	}

	// Replace the requested model ID with the resolved provider model name so
	// the adapter can address the correct model on the provider.
	req.Model = resolution.ModelName

	resp, err := resolution.Adapter.Generate(ctx, req)
	if err != nil {
		var pe ProviderError
		if errors.As(err, &pe) {
			return nil, resolution.ProviderKey, pe
		}
		return nil, resolution.ProviderKey, resolution.Adapter.NormalizeError(err)
	}

	return resp, resolution.ProviderKey, ProviderError{}
}

// GenerateStream delegates a streaming chat completion to the provider selected
// by model ID. It returns the chunk channel, the provider key that served it,
// and any synchronous setup error.
func (m *Manager) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.ChatCompletionChunk, string, ProviderError) {
	resolution, perr := m.ResolveModel(ctx, req.Model)
	if perr.Code != "" {
		return nil, "", perr
	}

	// Replace the requested model ID with the resolved provider model name.
	req.Model = resolution.ModelName

	chunkCh, errCh, setupErr := resolution.Adapter.GenerateStream(ctx, req)
	if setupErr != nil {
		var pe ProviderError
		if errors.As(setupErr, &pe) {
			return nil, resolution.ProviderKey, pe
		}
		return nil, resolution.ProviderKey, resolution.Adapter.NormalizeError(setupErr)
	}

	// Wrap the chunk channel to also drain the error channel on completion.
	// This allows the pipeline to read from a single channel and get the
	// terminal error via a separate mechanism.
	wrappedCh := make(chan api.ChatCompletionChunk, 64)
	go func() {
		defer close(wrappedCh)
		for chunk := range chunkCh {
			wrappedCh <- chunk
		}
		// Drain the error channel
		<-errCh
	}()

	return wrappedCh, resolution.ProviderKey, ProviderError{}
}
