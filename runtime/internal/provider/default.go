package provider

import (
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

// DefaultRegistry returns a registry with all built-in local provider adapters
// registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register("ollama", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
		return NewOllamaAdapter(name, settings, log)
	})
	r.Register("lmstudio", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
		return NewLMStudioAdapter(name, settings, log)
	})
	r.Register("openai_compatible_local", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
		return NewOpenAICompatibleAdapter(name, settings, log)
	})
	return r
}
