// Package benchmark defines the core data types for the Gumi Benchmark subsystem.
package benchmark

// RunResult is the top-level output of a benchmark run. It contains the full
// configuration, summary statistics, per-capability breakdowns, degradation
// analysis, per-test details, and an optional frontier model baseline.
type RunResult struct {
	SchemaVersion    int                  `json:"schema_version"`
	RunID            string               `json:"run_id"`
	Model            string               `json:"model"`
	Provider         string               `json:"provider"`
	ModelTier        string               `json:"model_tier"`
	Config           RunConfig            `json:"config"`
	Summary          Summary              `json:"summary"`
	Capabilities     map[string]Capability `json:"capabilities"`
	Degradation      DegradationReport    `json:"degradation"`
	PerTest          []TestResult         `json:"per_test"`
	FrontierBaseline *FrontierScores      `json:"frontier_baseline,omitempty"`
}

// RunConfig describes the configuration used for a benchmark run.
type RunConfig struct {
	Attempts   int      `json:"attempts"`
	Conditions []string `json:"conditions"`
	Tiers      []string `json:"tiers"`
	Timestamp  string   `json:"timestamp"`
}

// Summary holds the top-level aggregate results of a benchmark run.
type Summary struct {
	OverallScore         float64 `json:"overall_score"`
	LatencyOverheadMs    float64 `json:"latency_overhead_ms"`
	DegradationRate      float64 `json:"degradation_rate"`
	FrontierGapReduction float64 `json:"frontier_gap_reduction,omitempty"`
	WorthIt              bool    `json:"worth_it"`
}

// Capability holds per-capability benchmark results comparing direct and Gumi runs.
type Capability struct {
	Direct          MetricSet `json:"direct"`
	Gumi          MetricSet `json:"gumi"`
	Delta           float64   `json:"delta"`
	EffectSize      float64   `json:"effect_size"`
	FrontierCeiling float64   `json:"frontier_ceiling,omitempty"`
}

// MetricSet is a statistical summary of scores for a group of test runs.
type MetricSet struct {
	Mean float64 `json:"mean"`
	Std  float64 `json:"std"`
	N    int     `json:"n"`
}

// TestResult records the outcome of a single test attempt under a specific condition.
type TestResult struct {
	TestID    string             `json:"test_id"`
	Condition string             `json:"condition"`
	Attempt   int                `json:"attempt"`
	Passed    bool               `json:"passed"`
	Subscores map[string]float64 `json:"subscores"`
	LatencyMs float64            `json:"latency_ms"`
	Output    string             `json:"-"` // Not serialized, written to artifact file
	Error     string             `json:"error,omitempty"`
}

// SuiteTest defines a single test case within a benchmark suite.
type SuiteTest struct {
	ID              string       `yaml:"id"`
	Difficulty      string       `yaml:"difficulty"`
	Description     string       `yaml:"description"`
	Prompt          string       `yaml:"prompt,omitempty"`
	Type            string       `yaml:"type,omitempty"`
	Variants        []string     `yaml:"variants,omitempty"`
	ExpectedAnswer  string       `yaml:"expected_answer,omitempty"`
	Expected        interface{}  `yaml:"expected,omitempty"`
	TimeoutSeconds  int          `yaml:"timeout_seconds"`
	MaxTokens       int          `yaml:"max_tokens"`
	Constraints     []Constraint `yaml:"constraints,omitempty"`
}

// Constraint defines a single check that a model's response must pass.
type Constraint struct {
	Field    string      `yaml:"field"`
	Operator string      `yaml:"operator"`
	Value    interface{} `yaml:"value"`
}

// Suite is a collection of tests at a specific category and difficulty tier.
type Suite struct {
	ID                  string       `yaml:"id"`
	Category            string       `yaml:"category"`
	Tier                string       `yaml:"tier"`
	Description         string       `yaml:"description"`
	TargetDirectScore   string       `yaml:"target_direct_score"`
	ModelProfiles       []string     `yaml:"model_profiles"`
	AttemptsRecommended int          `yaml:"attempts_recommended"`
	Tests               []SuiteTest  `yaml:"tests"`
}

// DegradationReport records cases where Gumi over-repaired or altered correct output.
type DegradationReport struct {
	OverRepairCount int                `json:"over_repair_count"`
	TotalTests      int                `json:"total_tests"`
	DegradationRate float64            `json:"degradation_rate"`
	Corruptions     []CorruptionRecord `json:"corruptions,omitempty"`
	LatencyOverhead map[string]float64 `json:"latency_overhead_by_mode"`
}

// CorruptionRecord describes a single instance where Gumi changed correct output.
type CorruptionRecord struct {
	TestID   string `json:"test_id"`
	Original string `json:"original"`
	Repaired string `json:"repaired"`
	Severity string `json:"severity"`
}

// FrontierScores holds baseline scores from a frontier model for comparison.
type FrontierScores struct {
	Model  string             `json:"model"`
	Scores map[string]float64 `json:"scores"`
}

// ChatCompletionRequest represents a request to a chat completion API.
type ChatCompletionRequest struct {
	Model    string              `json:"model"`
	Messages []ChatMessage       `json:"messages"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Gumi    *GumiConfig      `json:"gumi,omitempty"`
}

// ChatMessage represents a single message in a chat completion conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents a response from a chat completion API.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a single completion choice returned by the API.
type Choice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

// Usage holds token usage statistics for a completion request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// GumiConfig holds Gumi-specific parameters for a request.
type GumiConfig struct {
	Mode string `json:"mode"`
}
