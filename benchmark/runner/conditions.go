package runner

import (
	"github.com/EffNine/gumi/benchmark"
)

// Condition identifiers for the different request modes.
const (
	ConditionDirect            Condition = "direct"
	ConditionGumiDirect      Condition = "gumi-direct"
	ConditionGumiLightweight Condition = "gumi-lightweight"
	ConditionGumiStabilized  Condition = "gumi-stabilized"
	ConditionGumiStructured  Condition = "gumi-structured"
	ConditionFrontier          Condition = "frontier"
)

// Condition represents a request mode for a benchmark test.
type Condition string

// ParseConditions converts a slice of condition name strings to Condition values.
// Unknown names are silently included as-is for forward compatibility.
func ParseConditions(conditions []string) []Condition {
	conds := make([]Condition, 0, len(conditions))
	for _, c := range conditions {
		conds = append(conds, Condition(c))
	}
	return conds
}

// ConditionManager builds provider request payloads based on the selected condition mode.
type ConditionManager struct {
	modelName     string
	provider      string
	frontierModel string
	frontierKey   string
}

// NewConditionManager creates a new ConditionManager with the given model and provider info.
func NewConditionManager(modelName, provider, frontierModel, frontierKey string) *ConditionManager {
	return &ConditionManager{
		modelName:     modelName,
		provider:      provider,
		frontierModel: frontierModel,
		frontierKey:   frontierKey,
	}
}

// BuildRequest constructs a ChatCompletionRequest for the given condition and test.
//   - direct: raw model name, no gumi params
//   - gumi-direct: model prefixed with "lmstudio:" or "ollama:", no gumi params
//   - gumi-lightweight: model prefixed, gumi: {mode: lightweight}
//   - gumi-stabilized: model prefixed, gumi: {mode: stabilized}
//   - gumi-structured: model prefixed, gumi: {mode: structured}
//   - frontier: uses frontierModel from config
func (cm *ConditionManager) BuildRequest(cond Condition, test benchmark.SuiteTest) benchmark.ChatCompletionRequest {
	base := benchmark.ChatCompletionRequest{
		Model:       cm.modelName,
		Messages:    []benchmark.ChatMessage{{Role: "user", Content: test.Prompt}},
		MaxTokens:   test.MaxTokens,
		Temperature: 0.3,
	}

	switch cond {
	case ConditionDirect:
		// Use raw model name as-is

	case ConditionGumiDirect:
		base.Model = cm.providerPrefix() + cm.modelName

	case ConditionGumiLightweight:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Gumi = &benchmark.GumiConfig{Mode: "lightweight"}

	case ConditionGumiStabilized:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Gumi = &benchmark.GumiConfig{Mode: "stabilized"}

	case ConditionGumiStructured:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Gumi = &benchmark.GumiConfig{Mode: "structured"}

	case ConditionFrontier:
		if cm.frontierModel != "" {
			base.Model = cm.frontierModel
		}
	}

	return base
}

// providerPrefix returns the provider-specific prefix for the model name
// when routing through the Gumi runtime.
func (cm *ConditionManager) providerPrefix() string {
	switch cm.provider {
	case "lmstudio":
		return "lmstudio:"
	case "ollama":
		return "ollama:"
	default:
		return "lmstudio:"
	}
}

