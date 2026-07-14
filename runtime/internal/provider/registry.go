package provider

import (
	"fmt"
	"strings"
	"sync"

	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

// Factory builds a ProviderAdapter from provider settings.
type Factory func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error)

// Registry maps provider keys to adapter factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a factory for the given provider key.
func (r *Registry) Register(name string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get looks up a factory by provider key.
func (r *Registry) Get(name string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	return f, ok
}

// modelIDPrefix returns the public model ID prefix for a provider key.
func modelIDPrefix(providerKey string) string {
	if providerKey == "openai_compatible_local" {
		return "openai-compatible"
	}
	return providerKey
}

// providerKeyFromModelID extracts the provider key from a model ID such as
// "ollama:llama3" or "openai-compatible:gpt4". It returns "" for the
// "local:auto" alias and for model IDs without a recognized provider prefix.
func providerKeyFromModelID(modelID string) string {
	if modelID == "local:auto" {
		return ""
	}
	parts := strings.SplitN(modelID, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	prefix := parts[0]
	if prefix == "local" {
		return ""
	}
	if prefix == "openai-compatible" {
		return "openai_compatible_local"
	}
	return prefix
}

// Build creates a Manager containing every enabled provider in the config.
func (r *Registry) Build(cfg *config.Config, log *logger.Logger) (*Manager, error) {
	adapters := make(map[string]ProviderAdapter)

	for key, settings := range cfg.Providers {
		if !settings.Enabled {
			log.Debug("provider disabled", "provider", key)
			continue
		}

		factory, ok := r.Get(key)
		if !ok {
			return nil, fmt.Errorf("unknown provider %q", key)
		}

		adapter, err := factory(key, settings, log)
		if err != nil {
			// Misconfigured providers are logged but do not block startup.
			log.Error("failed to build provider adapter", err, "provider", key)
			continue
		}

		adapters[key] = adapter
	}

	return NewManager(adapters, log), nil
}
