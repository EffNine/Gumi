// Package types defines the core data types for the Gumi Evaluation Protocol (GEP).
package types

import "time"

// ProtocolVersion is the current GEP protocol version.
const ProtocolVersion = "2.0.0"

// SuiteType identifies the category of a GEP benchmark suite.
type SuiteType string

const (
	SuiteInstructionFollowing SuiteType = "instruction_following"
	SuiteStructuredOutput     SuiteType = "structured_output"
	SuiteConsistency          SuiteType = "consistency"
	SuiteContextRetention     SuiteType = "context_retention"
	SuiteLatency              SuiteType = "latency"
)

// ProviderType identifies the inference provider.
type ProviderType string

const (
	ProviderLMStudio ProviderType = "lmstudio"
	ProviderOllama   ProviderType = "ollama"
)

// DifficultyTier represents the difficulty level of a GEP test.
type DifficultyTier string

const (
	TierEasy   DifficultyTier = "easy"
	TierMedium DifficultyTier = "medium"
	TierHard   DifficultyTier = "hard"
)

// GEPCondition identifies the execution condition for a GEP run.
type GEPCondition string

const (
	// ConditionDirect sends requests directly to the inference provider,
	// bypassing the Gumi runtime entirely. This measures raw model capability.
	ConditionDirect GEPCondition = "direct"
	// ConditionGumiStabilized sends requests through the Gumi runtime in
	// stabilized mode, exercising the full instruction-engine pipeline.
	ConditionGumiStabilized GEPCondition = "gumi-stabilized"
)

// GEPScope identifies whether a baseline/report measures model capability
// or runtime effectiveness. Model-scoped results are raw-provider baselines;
// runtime-scoped results compare direct vs gumi-stabilized.
type GEPScope string

const (
	ScopeModel     GEPScope = "model"
	ScopeRuntime   GEPScope = "runtime"
)

// A GEPTest is a single evaluation case within a GEP suite.
type GEPTest struct {
	ID             string          `yaml:"id" json:"id"`
	Difficulty     DifficultyTier  `yaml:"difficulty" json:"difficulty"`
	Description    string          `yaml:"description" json:"description"`
	Prompt         string          `yaml:"prompt" json:"prompt"`
	SystemPrompt   string          `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
	Type           string          `yaml:"type,omitempty" json:"type,omitempty"`
	Variants       []string        `yaml:"variants,omitempty" json:"variants,omitempty"`
	ExpectedAnswer string          `yaml:"expected_answer,omitempty" json:"expected_answer,omitempty"`
	Turns          []Turn          `yaml:"turns,omitempty" json:"turns,omitempty"`
	TimeoutSeconds int             `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	MaxTokens      int             `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	Constraints    []GEPConstraint `yaml:"constraints,omitempty" json:"constraints,omitempty"`
}

// Turn represents a single message in a multi-turn test.
type Turn struct {
	Role    string `yaml:"role" json:"role"`
	Content string `yaml:"content" json:"content"`
}

// GEPConstraint defines a single check that a model's response must pass.
type GEPConstraint struct {
	Field    string      `yaml:"field" json:"field"`
	Operator string      `yaml:"operator" json:"operator"`
	Value    interface{} `yaml:"value" json:"value"`
}

// GESuite is a collection of GEP tests in a single category.
type GESuite struct {
	ID                  string         `yaml:"id" json:"id"`
	Type                SuiteType      `yaml:"type" json:"type"`
	Difficulty          DifficultyTier `yaml:"difficulty" json:"difficulty"`
	Description         string         `yaml:"description" json:"description"`
	TargetDirectScore   string         `yaml:"target_direct_score,omitempty" json:"target_direct_score,omitempty"`
	ModelProfiles       []string       `yaml:"model_profiles,omitempty" json:"model_profiles,omitempty"`
	AttemptsRecommended int            `yaml:"attempts_recommended,omitempty" json:"attempts_recommended,omitempty"`
	TimeoutSeconds      int            `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	MaxTokens           int            `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	Tests               []GEPTest      `yaml:"tests" json:"tests"`
}

// GEPRunConfig is the configuration used for a GEP benchmark run.
type GEPRunConfig struct {
	Model       string         `json:"model"`
	Provider    ProviderType   `json:"provider"`
	ProviderURL string         `json:"provider_url"`
	APIKey      string         `json:"api_key,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Attempts    int            `json:"attempts"`
	SuiteID     string         `json:"suite_id,omitempty"`
	Difficulty  DifficultyTier `json:"difficulty,omitempty"`
	// Conditions lists which execution conditions to run (direct, gumi-stabilized).
	Conditions []GEPCondition `json:"conditions,omitempty"`
	// GumiURL is the Gumi runtime base URL used for gumi-stabilized condition.
	GumiURL string `json:"gumi_url,omitempty"`
	// GumiAPIKey is the bearer token for the Gumi runtime.
	GumiAPIKey string `json:"gumi_api_key,omitempty"`
	// Scope indicates whether this is a model-capability run or a runtime-effectiveness run.
	Scope GEPScope `json:"scope,omitempty"`
}

// GEPResult records the outcome of a single GEP test attempt.
type GEPResult struct {
	TestID     string             `json:"test_id"`
	SuiteID    string             `json:"suite_id"`
	Attempt    int                `json:"attempt"`
	Condition  GEPCondition       `json:"condition,omitempty"`
	Passed     bool               `json:"passed"`
	Subscores  map[string]float64 `json:"subscores"`
	LatencyMs  float64            `json:"latency_ms"`
	Output     string             `json:"output,omitempty"`
	Error      string             `json:"error,omitempty"`
	Request    string             `json:"request,omitempty"`
	ResponseID string             `json:"response_id,omitempty"`
	Model      string             `json:"model,omitempty"`
	Provider   ProviderType       `json:"provider,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
}

// GEPMetricSet is a statistical summary for a group of GEP results.
type GEPMetricSet struct {
	Mean   float64 `json:"mean"`
	Std    float64 `json:"std"`
	N      int     `json:"n"`
	Min    float64 `json:"min,omitempty"`
	Max    float64 `json:"max,omitempty"`
	Median float64 `json:"median,omitempty"`
	P25    float64 `json:"p25,omitempty"`
	P75    float64 `json:"p75,omitempty"`
}

// GEPCapability holds per-capability GEP results with condition dimension.
type GEPCapability struct {
	SuiteType  SuiteType    `json:"suite_type"`
	Direct     GEPMetricSet `json:"direct"`
	Gumi       GEPMetricSet `json:"gumi"`
	Delta      float64      `json:"delta"`
	PassRate   float64      `json:"pass_rate"`
	Desc       string       `json:"description"`
}

// GEPBaseline stores historical benchmark data for regression comparison.
type GEPBaseline struct {
	RunID        string                  `json:"run_id"`
	Model        string                  `json:"model"`
	Provider     ProviderType            `json:"provider"`
	Scope        GEPScope                `json:"scope"`
	Timestamp    time.Time               `json:"timestamp"`
	OverallScore float64                 `json:"overall_score"`
	Capabilities map[string]GEPCapability `json:"capabilities"`
	Config       GEPRunConfig            `json:"config"`
}

// GEPRegression is the comparison between a new run and a baseline.
type GEPRegression struct {
	BaselineRunID    string                     `json:"baseline_run_id"`
	BaselineScore    float64                    `json:"baseline_score"`
	CurrentScore     float64                    `json:"current_score"`
	Delta            float64                    `json:"delta"`
	Regression       bool                       `json:"regression"`
	CapabilityDeltas map[string]CapabilityDelta `json:"capability_deltas"`
}

// CapabilityDelta shows the score change for a single capability.
type CapabilityDelta struct {
	SuiteType  SuiteType `json:"suite_type"`
	Baseline   float64   `json:"baseline"`
	Current    float64   `json:"current"`
	Delta      float64   `json:"delta"`
	Regression bool      `json:"regression"`
}

// GEPReport is the top-level output of a GEP benchmark run.
type GEPReport struct {
	SchemaVersion   int                      `json:"schema_version"`
	RunID           string                   `json:"run_id"`
	ProtocolVersion string                   `json:"protocol_version"`
	Config          GEPRunConfig             `json:"config"`
	Summary         GEPSummary               `json:"summary"`
	Capabilities    map[string]GEPCapability `json:"capabilities"`
	PerTest         []GEPResult              `json:"per_test"`
	Regression      *GEPRegression           `json:"regression,omitempty"`
}

// GEPSummary holds the top-level aggregate results of a GEP run.
type GEPSummary struct {
	OverallScore       float64      `json:"overall_score"`
	DirectScore        float64      `json:"direct_score,omitempty"`
	GumiScore          float64      `json:"gumi_score,omitempty"`
	ScoreDelta         float64      `json:"score_delta,omitempty"`
	PassRate           float64      `json:"pass_rate"`
	DirectPassRate     float64      `json:"direct_pass_rate,omitempty"`
	GumiPassRate       float64      `json:"gumi_pass_rate,omitempty"`
	PassRateDelta      float64      `json:"pass_rate_delta,omitempty"`
	AvgLatencyMs       float64      `json:"avg_latency_ms"`
	DirectLatencyMs    float64      `json:"direct_latency_ms,omitempty"`
	GumiLatencyMs      float64      `json:"gumi_latency_ms,omitempty"`
	LatencyDeltaMs     float64      `json:"latency_delta_ms,omitempty"`
	TotalTests         int          `json:"total_tests"`
	PassedTests        int          `json:"passed_tests"`
	WorthIt            bool         `json:"worth_it"`
}
