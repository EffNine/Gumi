package router

import (
	"fmt"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
)

// ---------------------------------------------------------------------------
// PreferenceStrategy
// ---------------------------------------------------------------------------

// PreferenceStrategy controls how the rule engine selects among candidates.
type PreferenceStrategy string

const (
	PreferenceFastest        PreferenceStrategy = "fastest"
	PreferenceBestCoding     PreferenceStrategy = "best_coding"
	PreferenceBestCombo      PreferenceStrategy = "best_combo"
	PreferenceLargestContext PreferenceStrategy = "largest_context"
	PreferenceExplicit       PreferenceStrategy = "explicit"
)

// ---------------------------------------------------------------------------
// Rules
// ---------------------------------------------------------------------------

// CodingRule is a first-match routing rule evaluated in order.
type CodingRule struct {
	Name        string        `yaml:"name" json:"name"`
	When        RuleCondition `yaml:"when" json:"when"`
	RouteAction RuleAction    `yaml:"route" json:"route"`
}

// RuleCondition specifies when a rule matches.
type RuleCondition struct {
	Difficulty   []int    `yaml:"difficulty,omitempty" json:"difficulty,omitempty"`
	TaskType     []string `yaml:"task_type,omitempty" json:"task_type,omitempty"`
	HasTraceback *bool    `yaml:"has_traceback,omitempty" json:"has_traceback,omitempty"`
	MinFileCount *int     `yaml:"min_file_count,omitempty" json:"min_file_count,omitempty"`
	MaxFileCount *int     `yaml:"max_file_count,omitempty" json:"max_file_count,omitempty"`
	MinStep      *int     `yaml:"min_step,omitempty" json:"min_step,omitempty"`
	MaxStep      *int     `yaml:"max_step,omitempty" json:"max_step,omitempty"`
	MinRetries   *int     `yaml:"min_retries,omitempty" json:"min_retries,omitempty"`
}

// RuleAction specifies the routing outcome when a rule matches.
type RuleAction struct {
	Prefer       PreferenceStrategy `yaml:"prefer" json:"prefer"`
	MinCoding    string             `yaml:"min_coding,omitempty" json:"min_coding,omitempty"`
	MinContext   int                `yaml:"min_context,omitempty" json:"min_context,omitempty"`
	MinReasoning string             `yaml:"min_reasoning,omitempty" json:"min_reasoning,omitempty"`
	MaxSize      string             `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	Provider     string             `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model        string             `yaml:"model,omitempty" json:"model,omitempty"`
}

// ---------------------------------------------------------------------------
// RouteResult
// ---------------------------------------------------------------------------

// RouteResult is the output of the rule engine.
type RouteResult struct {
	MatchedRule  string                  `json:"matched_rule"`
	Provider     string                  `json:"provider"`
	Model        string                  `json:"model"`
	Strategy     PreferenceStrategy      `json:"strategy"`
	Reason       string                  `json:"reason"`
	Alternatives []AlternativeConsidered `json:"alternatives,omitempty"`
	FallbackUsed bool                    `json:"fallback_used"`
}

// AlternativeConsidered records a candidate that was considered but rejected.
type AlternativeConsidered struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Rejected string `json:"rejected"`
}

// ---------------------------------------------------------------------------
// CodingRuleEngine
// ---------------------------------------------------------------------------

// CodingRuleEngine evaluates routing rules first-match and applies the
// matching rule's action to select a model from the registry.
type CodingRuleEngine struct {
	rules     []CodingRule
	registry  *CodingModelRegistry
	memoryFit ModelFitLookup
}

// ModelFitLookup is the interface the router uses to query the memory engine
// for model performance data. Implemented by memory.MemoryEngine.
type ModelFitLookup interface {
	GetBestModelForRouter(difficulty int, taskType string) (modelID string, successRate float64, ok bool)
}

// DefaultCodingRules returns the built-in default routing rules for coding agents.
func DefaultCodingRules() []CodingRule {
	return []CodingRule{
		{
			Name: "trivial-fix",
			When: RuleCondition{
				Difficulty: []int{DifficultyTrivial},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
				MaxSize:   "small",
			},
		},
		{
			Name: "simple-fix",
			When: RuleCondition{
				Difficulty: []int{DifficultySimple},
				TaskType:   []string{"fix"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceFastest,
				MinCoding:  "weak",
				MinContext: 4096,
			},
		},
		{
			Name: "simple-coding",
			When: RuleCondition{
				Difficulty: []int{DifficultySimple},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
		{
			Name: "write-test",
			When: RuleCondition{
				Difficulty: []int{DifficultySimple, DifficultyModerate},
				TaskType:   []string{"test"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCoding,
				MinCoding:  "medium",
				MinContext: 8192,
			},
		},
		{
			Name: "moderate-feature",
			When: RuleCondition{
				Difficulty: []int{DifficultyModerate},
				TaskType:   []string{"feature"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCoding,
				MinCoding:  "medium",
				MinContext: 8192,
			},
		},
		{
			Name: "moderate-refactor",
			When: RuleCondition{
				Difficulty: []int{DifficultyModerate},
				TaskType:   []string{"refactor"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCoding,
				MinCoding:  "medium",
				MinContext: 8192,
			},
		},
		{
			Name: "complex-fix-with-trace",
			When: RuleCondition{
				Difficulty:   []int{DifficultyComplex, DifficultyNovel},
				HasTraceback: boolPtr(true),
			},
			RouteAction: RuleAction{
				Prefer:       PreferenceBestCombo,
				MinCoding:    "strong",
				MinReasoning: "medium",
				MinContext:   16384,
			},
		},
		{
			Name: "complex-coding",
			When: RuleCondition{
				Difficulty: []int{DifficultyComplex, DifficultyNovel},
				TaskType:   []string{"feature", "refactor", "plan"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCoding,
				MinCoding:  "strong",
				MinContext: 16384,
			},
		},
		{
			Name: "code-review",
			When: RuleCondition{
				TaskType: []string{"review"},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceFastest,
				MinCoding:  "weak",
				MinContext: 8192,
			},
		},
		{
			Name: "docs",
			When: RuleCondition{
				TaskType: []string{"docs"},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
		{
			Name: "complex-general",
			When: RuleCondition{
				Difficulty: []int{DifficultyComplex, DifficultyNovel},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCombo,
				MinCoding:  "strong",
				MinContext: 16384,
			},
		},
	}
}

// NewCodingRuleEngine creates a rule engine with the given rules and registry.
// If overrides is non-empty, it merges them over DefaultCodingRules() (overrides
// replace by Name). memoryFit is an optional ModelFitLookup for preferring
// models with a proven success rate; pass nil to disable.
func NewCodingRuleEngine(rules []CodingRule, registry *CodingModelRegistry, overrides []config.CodingRuleOverride, memoryFit ModelFitLookup) *CodingRuleEngine {
	// Apply overrides: replace rules by name.
	if len(overrides) > 0 {
		merged := make([]CodingRule, len(rules))
		copy(merged, rules)
		for i, rule := range merged {
			for _, ov := range overrides {
				if ov.Name == rule.Name {
					if ov.Prefer != "" {
						merged[i].RouteAction.Prefer = PreferenceStrategy(ov.Prefer)
					}
					if ov.MinCoding != "" {
						merged[i].RouteAction.MinCoding = ov.MinCoding
					}
					if ov.MinContext > 0 {
						merged[i].RouteAction.MinContext = ov.MinContext
					}
					if ov.MinReasoning != "" {
						merged[i].RouteAction.MinReasoning = ov.MinReasoning
					}
					if ov.MaxSize != "" {
						merged[i].RouteAction.MaxSize = ov.MaxSize
					}
				}
			}
		}
		rules = merged
	}
	return &CodingRuleEngine{
		rules:     rules,
		registry:  registry,
		memoryFit: memoryFit,
	}
}

// Route evaluates rules in order and returns the first match. Returns nil if
// no rule matches (should not happen with the default fallback rule).
func (e *CodingRuleEngine) Route(
	profile *CodingTaskProfile,
	availableModels map[string]bool,
	hints *api.RoutingExtensions,
) *RouteResult {
	// If hints specify an explicit provider/model, use it directly.
	if hints != nil && hints.PreferredProvider != "" && hints.PreferredModel != "" {
		return &RouteResult{
			MatchedRule: "user_hint_explicit",
			Provider:    hints.PreferredProvider,
			Model:       hints.PreferredModel,
			Strategy:    PreferenceExplicit,
			Reason:      "user-provided explicit model hint",
		}
	}

	for _, rule := range e.rules {
		if !e.matchCondition(rule.When, profile) {
			continue
		}

		// Determine effective MinContext: rule's value floored by hint.
		effectiveMinContext := rule.RouteAction.MinContext
		if hints != nil && hints.MinContext > effectiveMinContext {
			effectiveMinContext = hints.MinContext
		}

		// Explicit route: fixed provider/model.
		if rule.RouteAction.Provider != "" && rule.RouteAction.Model != "" {
			key := rule.RouteAction.Provider + ":" + rule.RouteAction.Model
			if availableModels[key] {
				return &RouteResult{
					MatchedRule: rule.Name,
					Provider:    rule.RouteAction.Provider,
					Model:       rule.RouteAction.Model,
					Strategy:    PreferenceExplicit,
					Reason:      fmt.Sprintf("rule %q matched with explicit model", rule.Name),
				}
			}
			// Fall through to registry-based selection.
			return e.selectFromRegistry(rule, profile, availableModels, effectiveMinContext)
		}

		// Preference-based route.
		return e.selectFromRegistry(rule, profile, availableModels, effectiveMinContext)
	}

	return nil
}

// matchCondition checks if a rule's condition matches the task profile.
func (e *CodingRuleEngine) matchCondition(cond RuleCondition, p *CodingTaskProfile) bool {
	// Difficulty check.
	if len(cond.Difficulty) > 0 {
		if !intInSlice(p.Difficulty, cond.Difficulty) {
			return false
		}
	}

	// Task type check.
	if len(cond.TaskType) > 0 {
		if !stringInSlice(string(p.TaskType), cond.TaskType) {
			return false
		}
	}

	// Traceback check.
	if cond.HasTraceback != nil {
		if p.HasTraceback != *cond.HasTraceback {
			return false
		}
	}

	// File count check.
	if cond.MinFileCount != nil && p.FileCount < *cond.MinFileCount {
		return false
	}
	if cond.MaxFileCount != nil && p.FileCount > *cond.MaxFileCount {
		return false
	}

	// Step count check.
	if cond.MinStep != nil && p.Step < *cond.MinStep {
		return false
	}
	if cond.MaxStep != nil && p.Step > *cond.MaxStep {
		return false
	}

	// Retries check.
	if cond.MinRetries != nil && p.Retries < *cond.MinRetries {
		return false
	}

	return true
}

// selectFromRegistry selects the best model from the registry based on the
// rule's action requirements. effectiveMinContext overrides the rule's
// MinContext when higher (used for per-request hints).
func (e *CodingRuleEngine) selectFromRegistry(rule CodingRule, p *CodingTaskProfile, availableModels map[string]bool, effectiveMinContext int) *RouteResult {
	action := rule.RouteAction

	// Collect all candidates for the alternatives list.
	allCandidates := e.registry.List()
	var alternatives []AlternativeConsidered

	best := e.registry.FindBest(
		action.Prefer,
		parseCodingStrength(action.MinCoding),
		effectiveMinContext,
		action.MinReasoning,
		ModelSizeCategory(action.MaxSize),
		availableModels,
	)

	if best != nil {
		// Check if memory engine has a better model for this task.
		if e.memoryFit != nil {
			if memModelID, memRate, memOk := e.memoryFit.GetBestModelForRouter(p.Difficulty, string(p.TaskType)); memOk && memRate > 0.7 {
				// Check if the memory-recommended model is in availableModels
				// and meets the rule's minimum coding strength requirement.
				for _, cand := range allCandidates {
					key := cand.Provider + ":" + cand.ModelName
					if key == memModelID && availableModels[key] &&
						cand.CodingStrength >= parseCodingStrength(action.MinCoding) {
						// Prefer the memory-recommended model.
						best = &cand
						break
					}
				}
			}
		}

		// Populate alternatives from rejected candidates.
		for _, cand := range allCandidates {
			key := cand.Provider + ":" + cand.ModelName
			if key == best.Provider+":"+best.ModelName {
				continue
			}
			if !availableModels[key] {
				alternatives = append(alternatives, AlternativeConsidered{
					Provider: cand.Provider,
					Model:    cand.ModelName,
					Rejected: "not_available",
				})
				continue
			}
			if cand.CodingStrength < parseCodingStrength(action.MinCoding) {
				alternatives = append(alternatives, AlternativeConsidered{
					Provider: cand.Provider,
					Model:    cand.ModelName,
					Rejected: "coding_strength_too_low",
				})
				continue
			}
			if effectiveMinContext > 0 && cand.ContextLimit < effectiveMinContext {
				alternatives = append(alternatives, AlternativeConsidered{
					Provider: cand.Provider,
					Model:    cand.ModelName,
					Rejected: "below_min_context",
				})
				continue
			}
			if action.MaxSize != "" && sizeRank(cand.SizeCategory) > sizeRank(ModelSizeCategory(action.MaxSize)) {
				alternatives = append(alternatives, AlternativeConsidered{
					Provider: cand.Provider,
					Model:    cand.ModelName,
					Rejected: "size_exceeds_max",
				})
				continue
			}
		}

		return &RouteResult{
			MatchedRule:  rule.Name,
			Provider:     best.Provider,
			Model:        best.ModelName,
			Strategy:     action.Prefer,
			Reason:       fmt.Sprintf("rule %q selected %s (coding:%s, context:%d)", rule.Name, best.ProfileID, best.CodingStrength, best.ContextLimit),
			Alternatives: alternatives,
		}
	}

	// Relax all constraints and try again.
	best = e.registry.FindBest(
		action.Prefer,
		CodingStrengthNone,
		0, "", "",
		availableModels,
	)

	if best != nil {
		return &RouteResult{
			MatchedRule:  rule.Name,
			Provider:     best.Provider,
			Model:        best.ModelName,
			Strategy:     action.Prefer,
			Reason:       fmt.Sprintf("rule %q matched but no model met min requirements; relaxed to best available: %s", rule.Name, best.Describe()),
			FallbackUsed: true,
			Alternatives: alternatives,
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intInSlice(n int, slice []int) bool {
	for _, v := range slice {
		if v == n {
			return true
		}
	}
	return false
}

func stringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func boolPtr(b bool) *bool { return &b }
