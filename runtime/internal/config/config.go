// Package config provides Novexa runtime configuration loading.
//
// In Sprint 1 this is intentionally a placeholder: it returns safe local
// defaults when no configuration file is provided and ignores any provided
// path. Full YAML config parsing will be added in a later sprint.
package config

import "os"

// Config is the top-level runtime configuration.
type Config struct {
	Runtime   RuntimeConfig               `json:"runtime" yaml:"runtime"`
	Dashboard DashboardConfig             `json:"dashboard" yaml:"dashboard"`
	Auth      AuthConfig                  `json:"auth" yaml:"auth"`
	Provider  ProviderConfig              `json:"provider" yaml:"provider"`
	Providers map[string]ProviderSettings `json:"providers" yaml:"providers"`
	Storage   StorageConfig               `json:"storage" yaml:"storage"`
	Telemetry TelemetryConfig             `json:"telemetry" yaml:"telemetry"`
}

// RuntimeConfig controls the core API server behaviour.
type RuntimeConfig struct {
	Name        string `json:"name" yaml:"name"`
	Mode        string `json:"mode" yaml:"mode"`
	Host        string `json:"host" yaml:"host"`
	Port        int    `json:"port" yaml:"port"`
	Environment string `json:"environment" yaml:"environment"`
	LogLevel    string `json:"log_level" yaml:"log_level"`
}

// DashboardConfig controls the local dashboard.
type DashboardConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Host    string `json:"host" yaml:"host"`
	Port    int    `json:"port" yaml:"port"`
}

// AuthConfig controls authentication mode.
type AuthConfig struct {
	Mode     string `json:"mode" yaml:"mode"`
	LocalKey string `json:"local_key" yaml:"local_key"`
}

// ProviderConfig selects the active provider.
type ProviderConfig struct {
	Default string `json:"default" yaml:"default"`
}

// ProviderSettings holds per-provider connection settings.
type ProviderSettings struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	URL            string `json:"url" yaml:"url"`
	DefaultModel   string `json:"default_model" yaml:"default_model"`
	TimeoutSeconds int    `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// StorageConfig controls local SQLite storage location and retention.
type StorageConfig struct {
	DBPath     string `json:"database_path" yaml:"database_path"`
	RetainDays int    `json:"retain_days" yaml:"retain_days"`
}

// TelemetryConfig controls local telemetry and logging.
type TelemetryConfig struct {
	Local        bool `json:"local" yaml:"local"`
	External     bool `json:"external" yaml:"external"`
	LogPrompts   bool `json:"log_prompts" yaml:"log_prompts"`
	LogResponses bool `json:"log_responses" yaml:"log_responses"`
	RetainDays   int  `json:"retain_days" yaml:"retain_days"`
}

// DefaultConfig returns the safe local defaults defined by the architecture.
func DefaultConfig() *Config {
	return &Config{
		Runtime: RuntimeConfig{
			Name:        "novexa",
			Mode:        "stabilized",
			Host:        "127.0.0.1",
			Port:        8787,
			Environment: "local",
			LogLevel:    "info",
		},
		Dashboard: DashboardConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8788,
		},
		Auth: AuthConfig{
			Mode:     "local",
			LocalKey: "novexa-local",
		},
		Provider: ProviderConfig{
			Default: "ollama",
		},
		Providers: map[string]ProviderSettings{
			"ollama": {
				Enabled:        true,
				URL:            "http://localhost:11434",
				DefaultModel:   "local:auto",
				TimeoutSeconds: 60,
			},
			"lmstudio": {
				Enabled:        true,
				URL:            "http://localhost:1234/v1",
				DefaultModel:   "local:auto",
				TimeoutSeconds: 60,
			},
			"openai_compatible_local": {
				Enabled:        true,
				URL:            "http://localhost:8000/v1",
				DefaultModel:   "local:auto",
				TimeoutSeconds: 60,
			},
		},
		Storage: StorageConfig{
			DBPath:     "",
			RetainDays: 14,
		},
		Telemetry: TelemetryConfig{
			Local:        true,
			External:     false,
			LogPrompts:   false,
			LogResponses: false,
			RetainDays:   14,
		},
	}
}

// Load returns the runtime configuration.
//
// If configPath is empty or the file does not exist, safe defaults are used.
// This placeholder does not yet parse YAML; that support will be added when
// the config loader is fully implemented.
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			// TODO: parse YAML configuration in a later sprint.
			_ = cfg
		}
	}

	return cfg, nil
}
