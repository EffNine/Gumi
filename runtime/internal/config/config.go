// Package config provides Novexa runtime configuration loading.
//
// Config is loaded from the first available source:
//  1. YAML file at the given configPath (via --config flag)
//  2. ~/.novexa/novexa.yaml
//  3. ./novexa.yaml (project-local)
//  4. Safe defaults (no file needed)
//
// Environment variables override any YAML values (see applyEnvOverrides).
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

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
// Config is loaded from the first available source:
//  1. YAML file at the given configPath (via --config flag)
//  2. ~/.novexa/novexa.yaml
//  3. ./novexa.yaml (project-local)
//  4. Safe defaults
//
// Environment variables override any YAML values.
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from a YAML file.
	paths := configSearchPaths(configPath)
	for _, p := range paths {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
			break
		}
	}

	// Expand ~ in the database path since yaml.Unmarshal doesn't do it.
	cfg.Storage.DBPath = expandHome(cfg.Storage.DBPath)

	applyEnvOverrides(cfg)

	return cfg, nil
}

// configSearchPaths returns the list of YAML file paths to try, in priority
// order. The first one that exists is used.
func configSearchPaths(configPath string) []string {
	home, _ := os.UserHomeDir()
	paths := []string{configPath}
	if home != "" {
		paths = append(paths, filepath.Join(home, ".novexa", "novexa.yaml"))
	}
	paths = append(paths, "novexa.yaml")
	return paths
}

// expandHome replaces a leading ~/ with the user's home directory, since
// yaml.Unmarshal does not expand it.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("NOVEXA_PROVIDER_DEFAULT"); v != "" {
		cfg.Provider.Default = v
	}
	if v := os.Getenv("NOVEXA_OLLAMA_URL"); v != "" {
		updateProvider(cfg, "ollama", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("NOVEXA_LMSTUDIO_URL"); v != "" {
		updateProvider(cfg, "lmstudio", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("NOVEXA_OPENAI_COMPATIBLE_LOCAL_URL"); v != "" {
		updateProvider(cfg, "openai_compatible_local", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("NOVEXA_DEFAULT_MODEL"); v != "" {
		updateProvider(cfg, cfg.Provider.Default, func(s *ProviderSettings) { s.DefaultModel = v })
	}
	if v := os.Getenv("NOVEXA_PROVIDER_TIMEOUT_SECONDS"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil && seconds > 0 {
			for key := range cfg.Providers {
				updateProvider(cfg, key, func(s *ProviderSettings) { s.TimeoutSeconds = seconds })
			}
		}
	}
}

func updateProvider(cfg *Config, key string, update func(*ProviderSettings)) {
	settings, ok := cfg.Providers[key]
	if !ok {
		return
	}
	update(&settings)
	cfg.Providers[key] = settings
}
