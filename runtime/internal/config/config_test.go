package config

import (
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
	cfg, err := Load("/this/path/does/not/exist/novexa.yaml")
	if err != nil {
		t.Fatalf("Load returned unexpected error for missing file: %v", err)
	}
	if cfg.Runtime.Mode != "stabilized" {
		t.Errorf("expected safe default mode when file missing, got %q", cfg.Runtime.Mode)
	}
}
