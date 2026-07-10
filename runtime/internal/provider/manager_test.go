package provider

import (
	"context"
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/logger"
)

func TestManagerHealthCheckUnknownProvider(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	status, err := mgr.HealthCheck(context.Background(), "missing")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
	if status != StatusUnknown {
		t.Errorf("expected status unknown, got %s", status)
	}
}

func TestManagerResolveAutoNoProviders(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	res, perr := mgr.ResolveModel(context.Background(), "local:auto")
	if res != nil {
		t.Error("expected no resolution")
	}
	if perr.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, perr.Code)
	}
}

func TestManagerResolveExplicitProviderNotConfigured(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, perr := mgr.ResolveModel(context.Background(), "ollama:llama3")
	if perr.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, perr.Code)
	}
}

func TestManagerResolveMalformedModelID(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, perr := mgr.ResolveModel(context.Background(), "notamodeLID")
	if perr.Code != ProviderMisconfigured {
		t.Errorf("expected %s, got %s", ProviderMisconfigured, perr.Code)
	}
}

func TestManagerGenerateEmptyModelID(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, _, perr := mgr.Generate(context.Background(), api.ChatCompletionRequest{Model: ""})
	if perr.Code != ProviderMisconfigured {
		t.Errorf("expected %s, got %s", ProviderMisconfigured, perr.Code)
	}
}

func TestManagerListModelsOffline(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	models := mgr.ListModels(context.Background())
	if len(models) != 0 {
		t.Errorf("expected no models when offline, got %d", len(models))
	}
}
