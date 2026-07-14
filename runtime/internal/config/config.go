// Package config provides Gumi runtime configuration loading.
//
// Config is loaded from the first available source:
//  1. YAML file at the given configPath (via --config flag)
//  2. ~/.gumi/gumi.yaml
//  3. ./gumi.yaml (project-local)
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
	Routing   RoutingConfig               `json:"routing" yaml:"routing"`
	Memory    MemoryConfig                `json:"memory" yaml:"memory"`
}

// DefaultHotCacheMaxSize is the default maximum number of entries in the hot
// cache (L1). This prevents unbounded memory growth from frequently accessed
// facts.
const DefaultHotCacheMaxSize = 500

// MemoryConfig controls the zero-VRAM memory engine for agentic coding.
type MemoryConfig struct {
	Enabled               bool    `json:"enabled" yaml:"enabled"`
	Engine                string  `json:"engine" yaml:"engine"`
	DBPath                string  `json:"db_path" yaml:"db_path"`
	MaxFacts              int     `json:"max_facts" yaml:"max_facts"`
	MaxEpisodesPerSession int     `json:"max_episodes_per_session" yaml:"max_episodes_per_session"`
	ModelFitRetentionDays int     `json:"model_fit_retention_days" yaml:"model_fit_retention_days"`
	InjectionBudgetTokens int     `json:"injection_budget_tokens" yaml:"injection_budget_tokens"`
	MinConfidence         float64 `json:"min_confidence" yaml:"min_confidence"`
	MaxInjectedFacts      int     `json:"max_injected_facts" yaml:"max_injected_facts"`
	ExtractEnabled        bool    `json:"extract_enabled" yaml:"extract_enabled"`
	MinObservationCount   int     `json:"min_observation_count" yaml:"min_observation_count"`
	TrackModelFit         bool    `json:"track_model_fit" yaml:"track_model_fit"`
	ModelFitDecay         float64 `json:"model_fit_decay" yaml:"model_fit_decay"`
	HotCacheMaxSize       int     `json:"hot_cache_max_size" yaml:"hot_cache_max_size"`
}

// RuntimeConfig controls the core API server behaviour.
type RuntimeConfig struct {
	Name        string      `json:"name" yaml:"name"`
	Mode        string      `json:"mode" yaml:"mode"`
	Host        string      `json:"host" yaml:"host"`
	Port        int         `json:"port" yaml:"port"`
	Environment string      `json:"environment" yaml:"environment"`
	LogLevel    string      `json:"log_level" yaml:"log_level"`
	Agent       AgentConfig `json:"agent" yaml:"agent"`
}

// RoutingConfig controls the Agentic Coding Router.
type RoutingConfig struct {
	Enabled     bool                 `json:"enabled" yaml:"enabled"`
	Mode        string               `json:"mode" yaml:"mode"`
	Classifier  ClassifierConfig     `json:"classifier,omitempty" yaml:"classifier,omitempty"`
	CodingRules []CodingRuleOverride `json:"coding_rules,omitempty" yaml:"coding_rules,omitempty"`
}

// ClassifierConfig controls the coding task classifier thresholds.
type ClassifierConfig struct {
	EscalationThreshold EscalationThreshold `json:"escalation_threshold,omitempty" yaml:"escalation_threshold,omitempty"`
}

// EscalationThreshold defines the thresholds for agent-state escalation.
type EscalationThreshold struct {
	Retries     int `json:"retries,omitempty" yaml:"retries,omitempty"`
	Steps       int `json:"steps,omitempty" yaml:"steps,omitempty"`
	Repetitions int `json:"repetitions,omitempty" yaml:"repetitions,omitempty"`
}

// CodingRuleOverride allows per-request overrides of default routing rules.
type CodingRuleOverride struct {
	Name         string `json:"name" yaml:"name"`
	Prefer       string `json:"prefer,omitempty" yaml:"prefer,omitempty"`
	MinCoding    string `json:"min_coding,omitempty" yaml:"min_coding,omitempty"`
	MinContext   int    `json:"min_context,omitempty" yaml:"min_context,omitempty"`
	MinReasoning string `json:"min_reasoning,omitempty" yaml:"min_reasoning,omitempty"`
	MaxSize      string `json:"max_size,omitempty" yaml:"max_size,omitempty"`
}

// AgentConfig controls the agent mode governance layer.
type AgentConfig struct {
	MaxSteps                   int     `yaml:"max_steps" json:"max_steps"`
	ToolCallTimeoutSeconds     int     `yaml:"tool_call_timeout_seconds" json:"tool_call_timeout_seconds"`
	ContextCompactionThreshold float64 `yaml:"context_compaction_threshold" json:"context_compaction_threshold"`
	LoopDetection              string  `yaml:"loop_detection" json:"loop_detection"`
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
	Enabled         bool                `json:"enabled" yaml:"enabled"`
	URL             string              `json:"url" yaml:"url"`
	DefaultModel    string              `json:"default_model" yaml:"default_model"`
	TimeoutSeconds  int                 `json:"timeout_seconds" yaml:"timeout_seconds"`
	ModelManagement *LMStudioMgmtConfig `json:"model_management,omitempty" yaml:"model_management,omitempty"`
}

// LMStudioMgmtConfig controls LM Studio v1 REST API model management.
// Only applies to the "lmstudio" provider. Other providers ignore it.
type LMStudioMgmtConfig struct {
	Enabled               bool                        `json:"enabled" yaml:"enabled"`
	DefaultContextLength  int                         `json:"default_context_length" yaml:"default_context_length"`
	DefaultFlashAttention *bool                       `json:"default_flash_attention,omitempty" yaml:"default_flash_attention,omitempty"`
	DefaultOffloadKV      *bool                       `json:"default_offload_kv_cache,omitempty" yaml:"default_offload_kv_cache,omitempty"`
	DefaultEvalBatchSize  int                         `json:"default_eval_batch_size,omitempty" yaml:"default_eval_batch_size,omitempty"`
	AutoUnload            bool                        `json:"auto_unload" yaml:"auto_unload"`
	ModelConfig           map[string]LMStudioModelCfg `json:"model_config,omitempty" yaml:"model_config,omitempty"`
}

// LMStudioModelCfg is per-model overrides for LM Studio load configuration.
type LMStudioModelCfg struct {
	ContextLength  *int  `json:"context_length,omitempty" yaml:"context_length,omitempty"`
	FlashAttention *bool `json:"flash_attention,omitempty" yaml:"flash_attention,omitempty"`
	OffloadKVCache *bool `json:"offload_kv_cache_to_gpu,omitempty" yaml:"offload_kv_cache_to_gpu,omitempty"`
	EvalBatchSize  *int  `json:"eval_batch_size,omitempty" yaml:"eval_batch_size,omitempty"`
	NumExperts     *int  `json:"num_experts,omitempty" yaml:"num_experts,omitempty"`
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
			Name:        "gumi",
			Mode:        "stabilized",
			Host:        "127.0.0.1",
			Port:        8787,
			Environment: "local",
			LogLevel:    "info",
			Agent: AgentConfig{
				MaxSteps:                   30,
				ToolCallTimeoutSeconds:     120,
				ContextCompactionThreshold: 0.85,
				LoopDetection:              "strict",
			},
		},
		Dashboard: DashboardConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8788,
		},
		Auth: AuthConfig{
			Mode:     "local",
			LocalKey: "gumi-local",
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
		Routing: RoutingConfig{
			Enabled: false, // Opt-in in V1
			Mode:    "agentic_coding",
			Classifier: ClassifierConfig{
				EscalationThreshold: EscalationThreshold{
					Retries:     3,
					Steps:       6,
					Repetitions: 3,
				},
			},
		},
		Memory: MemoryConfig{
			Enabled:               false, // Opt-in in V1
			Engine:                "sqlite",
			DBPath:                "",
			MaxFacts:              10000,
			MaxEpisodesPerSession: 500,
			ModelFitRetentionDays: 90,
			InjectionBudgetTokens: 1200,
			MinConfidence:         0.3,
			MaxInjectedFacts:      20,
			ExtractEnabled:        true,
			MinObservationCount:   2,
			TrackModelFit:         true,
			ModelFitDecay:         0.95,
		},
	}
}

// Load returns the runtime configuration.
//
// Config is loaded from the first available source:
//  1. YAML file at the given configPath (via --config flag)
//  2. ~/.gumi/gumi.yaml
//  3. ./gumi.yaml (project-local)
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
		paths = append(paths, filepath.Join(home, ".gumi", "gumi.yaml"))
	}
	paths = append(paths, "gumi.yaml")
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
	if v := os.Getenv("GUMI_PROVIDER_DEFAULT"); v != "" {
		cfg.Provider.Default = v
	}
	if v := os.Getenv("GUMI_OLLAMA_URL"); v != "" {
		updateProvider(cfg, "ollama", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("GUMI_LMSTUDIO_URL"); v != "" {
		updateProvider(cfg, "lmstudio", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("GUMI_OPENAI_COMPATIBLE_LOCAL_URL"); v != "" {
		updateProvider(cfg, "openai_compatible_local", func(s *ProviderSettings) { s.URL = v })
	}
	if v := os.Getenv("GUMI_DEFAULT_MODEL"); v != "" {
		updateProvider(cfg, cfg.Provider.Default, func(s *ProviderSettings) { s.DefaultModel = v })
	}
	if v := os.Getenv("GUMI_PROVIDER_TIMEOUT_SECONDS"); v != "" {
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
