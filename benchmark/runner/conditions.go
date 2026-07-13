package runner

import (
	"github.com/novexa/novexa/benchmark"
)

// Condition identifiers for the different request modes.
const (
	ConditionDirect            Condition = "direct"
	ConditionNovexaDirect      Condition = "novexa-direct"
	ConditionNovexaLightweight Condition = "novexa-lightweight"
	ConditionNovexaStabilized  Condition = "novexa-stabilized"
	ConditionNovexaStructured  Condition = "novexa-structured"
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
//   - direct: raw model name, no novexa params
//   - novexa-direct: model prefixed with "lmstudio:" or "ollama:", no novexa params
//   - novexa-lightweight: model prefixed, novexa: {mode: lightweight}
//   - novexa-stabilized: model prefixed, novexa: {mode: stabilized}
//   - novexa-structured: model prefixed, novexa: {mode: structured}
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

	case ConditionNovexaDirect:
		base.Model = cm.providerPrefix() + cm.modelName

	case ConditionNovexaLightweight:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Novexa = &benchmark.NovexaConfig{Mode: "lightweight"}

	case ConditionNovexaStabilized:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Novexa = &benchmark.NovexaConfig{Mode: "stabilized"}

	case ConditionNovexaStructured:
		base.Model = cm.providerPrefix() + cm.modelName
		base.Novexa = &benchmark.NovexaConfig{Mode: "structured"}

	case ConditionFrontier:
		if cm.frontierModel != "" {
			base.Model = cm.frontierModel
		}
	}

	return base
}

// providerPrefix returns the provider-specific prefix for the model name
// when routing through the Novexa runtime.
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

