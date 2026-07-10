package provider

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

func TestDefaultRegistryHasBuiltinProviders(t *testing.T) {
	r := DefaultRegistry()

	for _, name := range []string{"ollama", "lmstudio", "openai_compatible_local"} {
		if _, ok := r.Get(name); !ok {
			t.Errorf("expected provider %q to be registered", name)
		}
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	called := false
	r.Register("test", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
		called = true
		return nil, nil
	})

	factory, ok := r.Get("test")
	if !ok {
		t.Fatal("expected registered factory")
	}
	_, _ = factory("test", config.ProviderSettings{}, logger.New("error"))
	if !called {
		t.Error("expected factory to be called")
	}
}

func TestProviderKeyFromModelID(t *testing.T) {
	cases := []struct {
		modelID string
		want    string
	}{
		{"ollama:llama3", "ollama"},
		{"lmstudio:my-model", "lmstudio"},
		{"openai-compatible:gpt4", "openai_compatible_local"},
		{"local:auto", ""},
		{"no-prefix", ""},
	}

	for _, c := range cases {
		got := providerKeyFromModelID(c.modelID)
		if got != c.want {
			t.Errorf("providerKeyFromModelID(%q) = %q, want %q", c.modelID, got, c.want)
		}
	}
}

func TestModelIDPrefix(t *testing.T) {
	cases := []struct {
		providerKey string
		want        string
	}{
		{"ollama", "ollama"},
		{"lmstudio", "lmstudio"},
		{"openai_compatible_local", "openai-compatible"},
	}

	for _, c := range cases {
		got := modelIDPrefix(c.providerKey)
		if got != c.want {
			t.Errorf("modelIDPrefix(%q) = %q, want %q", c.providerKey, got, c.want)
		}
	}
}

func TestRegistryBuildWithDisabledProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["ollama"] = config.ProviderSettings{Enabled: false}

	r := DefaultRegistry()
	mgr, err := r.Build(cfg, logger.New("error"))
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	providers := mgr.ListProviders()
	for _, p := range providers {
		if p == "ollama" {
			t.Error("expected disabled ollama provider to be omitted")
		}
	}
}
