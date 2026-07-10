// Package config provides Novexa runtime configuration loading.
//
// In Sprint 1 this is intentionally a placeholder: it returns safe local
// defaults when no configuration file is provided and ignores any provided
// path. Full YAML config parsing will be added in a later sprint.
package config

import "os"

// Config is the top-level runtime configuration.
type Config struct {
	Runtime   RuntimeConfig
	Dashboard DashboardConfig
	Auth      AuthConfig
	Provider  ProviderConfig
	Providers map[string]ProviderSettings
	Storage   StorageConfig
	Telemetry TelemetryConfig
}

// RuntimeConfig controls the core API server behaviour.
type RuntimeConfig struct {
	Name        string
	Mode        string
	Host        string
	Port        int
	Environment string
	LogLevel    string
}

// DashboardConfig controls the local dashboard.
type DashboardConfig struct {
	Enabled bool
	Host    string
	Port    int
}

// AuthConfig controls authentication mode.
type AuthConfig struct {
	Mode     string
	LocalKey string
}

// ProviderConfig selects the active provider.
type ProviderConfig struct {
	Default string
}

// ProviderSettings holds per-provider connection settings.
type ProviderSettings struct {
	Enabled       bool
	URL           string
	DefaultModel  string
	TimeoutSeconds int
}

// StorageConfig controls local SQLite storage location and retention.
type StorageConfig struct {
	DBPath     string
	RetainDays int
}

// TelemetryConfig controls local telemetry and logging.
type TelemetryConfig struct {
	Local         bool
	External      bool
	LogPrompts    bool
	LogResponses  bool
	RetainDays    int
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
