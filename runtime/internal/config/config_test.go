package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Runtime.Mode != "stabilized" {
		t.Errorf("expected runtime mode 'stabilized', got %q", cfg.Runtime.Mode)
	}
	if cfg.Runtime.Port != 8787 {
		t.Errorf("expected runtime port 8787, got %d", cfg.Runtime.Port)
	}
	if cfg.Runtime.Host != "127.0.0.1" {
		t.Errorf("expected runtime host 127.0.0.1, got %q", cfg.Runtime.Host)
	}
	if cfg.Runtime.LogLevel != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Runtime.LogLevel)
	}
	if !cfg.Dashboard.Enabled {
		t.Error("expected dashboard to be enabled by default")
	}
	if cfg.Provider.Default != "ollama" {
		t.Errorf("expected default provider 'ollama', got %q", cfg.Provider.Default)
	}
}

func TestLoadWithEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if cfg.Runtime.Port != 8787 {
		t.Errorf("expected default port 8787, got %d", cfg.Runtime.Port)
	}
}

func TestLoadWithMissingFile(t *testing.T) {
	cfg, err := Load("/this/path/does/not/exist/gumi.yaml")
	if err != nil {
		t.Fatalf("Load returned unexpected error for missing file: %v", err)
	}
	if cfg.Runtime.Mode != "stabilized" {
		t.Errorf("expected safe default mode when file missing, got %q", cfg.Runtime.Mode)
	}
}

func TestLoadAppliesProviderEnvOverrides(t *testing.T) {
	t.Setenv("GUMI_PROVIDER_DEFAULT", "lmstudio")
	t.Setenv("GUMI_LMSTUDIO_URL", "http://192.168.0.164:1234/v1")
	t.Setenv("GUMI_DEFAULT_MODEL", "qwen/qwen3.5-9b")
	t.Setenv("GUMI_PROVIDER_TIMEOUT_SECONDS", "120")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.Provider.Default != "lmstudio" {
		t.Fatalf("expected default provider lmstudio, got %q", cfg.Provider.Default)
	}
	lmstudio := cfg.Providers["lmstudio"]
	if lmstudio.URL != "http://192.168.0.164:1234/v1" {
		t.Fatalf("expected LM Studio URL override, got %q", lmstudio.URL)
	}
	if lmstudio.DefaultModel != "qwen/qwen3.5-9b" {
		t.Fatalf("expected default model override, got %q", lmstudio.DefaultModel)
	}
	if lmstudio.TimeoutSeconds != 120 {
		t.Fatalf("expected timeout override 120, got %d", lmstudio.TimeoutSeconds)
	}
}

func TestLoadFromYAMLFile(t *testing.T) {
	// Create a temp YAML file matching the example.yaml format.
	yamlContent := []byte(`
runtime:
  mode: lightweight
  port: 8790
  log_level: debug

provider:
  default: lmstudio

providers:
  lmstudio:
    enabled: true
    url: http://192.168.0.164:1234/v1
    default_model: ornith-1.0-9b@q4_k_m
    timeout_seconds: 120
`)
	yamlPath := filepath.Join(t.TempDir(), "gumi.yaml")
	if err := os.WriteFile(yamlPath, yamlContent, 0644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load from yaml returned error: %v", err)
	}

	if cfg.Runtime.Mode != "lightweight" {
		t.Errorf("expected runtime mode 'lightweight' from yaml, got %q", cfg.Runtime.Mode)
	}
	if cfg.Runtime.Port != 8790 {
		t.Errorf("expected runtime port 8790 from yaml, got %d", cfg.Runtime.Port)
	}
	if cfg.Runtime.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from yaml, got %q", cfg.Runtime.LogLevel)
	}
	if cfg.Provider.Default != "lmstudio" {
		t.Errorf("expected provider default 'lmstudio' from yaml, got %q", cfg.Provider.Default)
	}
	lmstudio := cfg.Providers["lmstudio"]
	if lmstudio.URL != "http://192.168.0.164:1234/v1" {
		t.Errorf("expected lmstudio URL from yaml, got %q", lmstudio.URL)
	}
	if lmstudio.DefaultModel != "ornith-1.0-9b@q4_k_m" {
		t.Errorf("expected lmstudio model from yaml, got %q", lmstudio.DefaultModel)
	}
	if lmstudio.TimeoutSeconds != 120 {
		t.Errorf("expected lmstudio timeout 120 from yaml, got %d", lmstudio.TimeoutSeconds)
	}
}

func TestSaveModelsPreservesUnrelatedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gumi.yaml")

	initial := []byte(`runtime:
  mode: lightweight
provider:
  default: ollama
`)
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	models := []ModelRegistryEntry{
		{Alias: "my-alias", Provider: "ollama", ModelID: "llama3", Enabled: true},
	}
	if err := SaveModels(path, models); err != nil {
		t.Fatalf("SaveModels returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Models) != 1 || cfg.Models[0].Alias != "my-alias" {
		t.Fatalf("expected persisted model alias my-alias, got %+v", cfg.Models)
	}
	if cfg.Runtime.Mode != "lightweight" {
		t.Fatalf("expected runtime.mode preserved, got %q", cfg.Runtime.Mode)
	}
	if cfg.Provider.Default != "ollama" {
		t.Fatalf("expected provider.default preserved, got %q", cfg.Provider.Default)
	}
}

func TestLoadYAMLWithHomeDirExpansion(t *testing.T) {
	// The config should expand ~/ in database_path.
	yamlContent := []byte(`
storage:
  database_path: ~/.gumi/gumi.db
`)
	yamlPath := filepath.Join(t.TempDir(), "gumi.yaml")
	if err := os.WriteFile(yamlPath, yamlContent, 0644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load from yaml returned error: %v", err)
	}

	if cfg.Storage.DBPath == "~/.gumi/gumi.db" {
		t.Error("expected database_path ~ to be expanded to home directory, but it was left as-is")
	}
	if !strings.Contains(cfg.Storage.DBPath, ".gumi/gumi.db") {
		t.Errorf("expected database_path to contain '.gumi/gumi.db', got %q", cfg.Storage.DBPath)
	}
}
