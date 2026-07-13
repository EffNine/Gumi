package router

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/profiles"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// fakeModelFitLookup returns a fixed model for testing.
type fakeModelFitLookup struct {
	modelID     string
	successRate float64
	ok          bool
}

func (f *fakeModelFitLookup) GetBestModelForRouter(difficulty int, taskType string) (string, float64, bool) {
	return f.modelID, f.successRate, f.ok
}

// newTestRegistry creates a registry with a few test profiles.
// The registry iterates profiles first, then all provider models per profile.
// The first profile claims all model names. We use separate providers so
// each profile gets its own entry.
func newTestRegistry() *CodingModelRegistry {
	profilesList := []*profiles.Profile{
		{
			ID:           "strong-model",
			Name:         "Strong Model",
			Size:         "8b",
			ContextLimit: 65536,
			Capabilities: profiles.Capabilities{
				Coding:      "strong",
				ToolCalling: "strong",
				Reasoning:   "strong",
			},
		},
		{
			ID:           "weak-model",
			Name:         "Weak Model",
			Size:         "2b",
			ContextLimit: 32000,
			Capabilities: profiles.Capabilities{
				Coding:      "weak",
				ToolCalling: "weak",
				Reasoning:   "weak",
			},
		},
	}
	// Use separate providers so each profile gets its own entry.
	providerModels := map[string][]string{
		"ollama":   {"strong-model:v1"},
		"lmstudio": {"weak-model:v1"},
	}
	return NewCodingModelRegistry(profilesList, providerModels)
}

// newRichTestRegistry creates a registry with more varied profiles for
// testing preference strategies and alternatives.
// Uses direct entry construction to avoid the registry's first-profile-wins
// dedup design.
func newRichTestRegistry() *CodingModelRegistry {
	return &CodingModelRegistry{
		entries: []CodingModelRegistryEntry{
			{
				ProfileID:      "tiny-model",
				Provider:       "provider-tiny",
				ModelName:      "tiny-model:v1",
				CodingStrength: CodingStrengthWeak,
				ToolCalling:    "basic",
				Reasoning:      "none",
				ContextLimit:   4096,
				SizeCategory:   SizeTiny,
			},
			{
				ProfileID:      "small-model",
				Provider:       "provider-small",
				ModelName:      "small-model:v1",
				CodingStrength: CodingStrengthMedium,
				ToolCalling:    "good",
				Reasoning:      "basic",
				ContextLimit:   8192,
				SizeCategory:   SizeSmall,
			},
			{
				ProfileID:      "large-model",
				Provider:       "provider-large",
				ModelName:      "large-model:v1",
				CodingStrength: CodingStrengthStrong,
				ToolCalling:    "excellent",
				Reasoning:      "strong",
				ContextLimit:   32768,
				SizeCategory:   SizeLarge,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// 1. Rule matching by difficulty
// ---------------------------------------------------------------------------

func TestRoute_MatchesByDifficulty_Trivial(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyTrivial,
		TaskType:   TaskFix,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "trivial-fix" {
		t.Fatalf("expected rule 'trivial-fix', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByDifficulty_Simple(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// simple-coding matches first (no task type filter, difficulty=simple)
	if result.MatchedRule != "simple-coding" {
		t.Fatalf("expected rule 'simple-coding', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByDifficulty_Moderate(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "moderate-feature" {
		t.Fatalf("expected rule 'moderate-feature', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByDifficulty_Complex(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// complex-coding matches first (difficulty=complex, task_type=feature)
	if result.MatchedRule != "complex-coding" {
		t.Fatalf("expected rule 'complex-coding', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByDifficulty_Novel(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyNovel,
		TaskType:   TaskPlan,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// complex-coding matches first (difficulty=novel, task_type=plan)
	if result.MatchedRule != "complex-coding" {
		t.Fatalf("expected rule 'complex-coding', got %q", result.MatchedRule)
	}
}

// ---------------------------------------------------------------------------
// 2. Rule matching by task type
// ---------------------------------------------------------------------------

func TestRoute_MatchesByTaskType_Test(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	// Use DifficultyModerate so simple-coding (difficulty=Simple only) doesn't match first.
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskTest,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "write-test" {
		t.Fatalf("expected rule 'write-test', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByTaskType_Review(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskReview,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "code-review" {
		t.Fatalf("expected rule 'code-review', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByTaskType_Docs(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskDocs,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "docs" {
		t.Fatalf("expected rule 'docs', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByTaskType_Refactor(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskRefactor,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "moderate-refactor" {
		t.Fatalf("expected rule 'moderate-refactor', got %q", result.MatchedRule)
	}
}

// ---------------------------------------------------------------------------
// 3. Rule matching by traceback
// ---------------------------------------------------------------------------

func TestRoute_MatchesByTraceback_True(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty:   DifficultyComplex,
		TaskType:     TaskFix,
		HasTraceback: true,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// complex-fix-with-trace matches first (difficulty=complex, has_traceback=true)
	if result.MatchedRule != "complex-fix-with-trace" {
		t.Fatalf("expected rule 'complex-fix-with-trace', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByTraceback_False(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty:   DifficultyComplex,
		TaskType:     TaskFix,
		HasTraceback: false,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// complex-fix-with-trace requires has_traceback=true, so it won't match.
	// complex-coding requires task_type in [feature, refactor, plan], so it won't match.
	// Falls through to complex-general.
	if result.MatchedRule != "complex-general" {
		t.Fatalf("expected rule 'complex-general', got %q", result.MatchedRule)
	}
}

// ---------------------------------------------------------------------------
// 4. Rule matching by file count
// ---------------------------------------------------------------------------

func TestRoute_MatchesByFileCount_MinFileCount(t *testing.T) {
	// Create a custom rule with MinFileCount to test the condition.
	rules := []CodingRule{
		{
			Name: "multi-file-task",
			When: RuleCondition{
				MinFileCount: intPtr(3),
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceBestCoding,
				MinCoding: "medium",
			},
		},
		{
			Name: "fallback",
			When: RuleCondition{},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(rules, reg, nil, nil)

	// Profile with 3 files should match multi-file-task.
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
		FileCount:  3,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "multi-file-task" {
		t.Fatalf("expected rule 'multi-file-task', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByFileCount_MaxFileCount(t *testing.T) {
	rules := []CodingRule{
		{
			Name: "single-file-task",
			When: RuleCondition{
				MaxFileCount: intPtr(1),
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
		{
			Name: "fallback",
			When: RuleCondition{},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(rules, reg, nil, nil)

	// Profile with 1 file should match single-file-task.
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
		FileCount:  1,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "single-file-task" {
		t.Fatalf("expected rule 'single-file-task', got %q", result.MatchedRule)
	}
}

func TestRoute_MatchesByFileCount_ExceedsMax(t *testing.T) {
	rules := []CodingRule{
		{
			Name: "single-file-task",
			When: RuleCondition{
				MaxFileCount: intPtr(1),
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
		{
			Name: "fallback",
			When: RuleCondition{},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(rules, reg, nil, nil)

	// Profile with 5 files should NOT match single-file-task, fall through to fallback.
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
		FileCount:  5,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "fallback" {
		t.Fatalf("expected rule 'fallback', got %q", result.MatchedRule)
	}
}

// ---------------------------------------------------------------------------
// 5. Explicit provider/model routing
// ---------------------------------------------------------------------------

func TestRoute_ExplicitProviderModel(t *testing.T) {
	rules := []CodingRule{
		{
			Name: "explicit-route",
			When: RuleCondition{
				Difficulty: []int{DifficultyModerate},
			},
			RouteAction: RuleAction{
				Provider: "ollama",
				Model:    "strong-model:v1",
			},
		},
	}
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Provider != "ollama" {
		t.Fatalf("expected provider 'ollama', got %q", result.Provider)
	}
	if result.Model != "strong-model:v1" {
		t.Fatalf("expected model 'strong-model:v1', got %q", result.Model)
	}
	if result.Strategy != PreferenceExplicit {
		t.Fatalf("expected strategy 'explicit', got %q", result.Strategy)
	}
}

func TestRoute_ExplicitProviderModel_NotAvailable_FallsBack(t *testing.T) {
	rules := []CodingRule{
		{
			Name: "explicit-route",
			When: RuleCondition{
				Difficulty: []int{DifficultyModerate},
			},
			RouteAction: RuleAction{
				Provider: "ollama",
				Model:    "nonexistent-model:v99",
			},
		},
	}
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Should fall through to selectFromRegistry since the explicit model is not available.
	if result.Provider == "" || result.Model == "" {
		t.Fatal("expected a model to be selected via registry fallback")
	}
}

func TestRoute_UserHintExplicit(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyTrivial,
		TaskType:   TaskFix,
	}
	hints := &api.RoutingExtensions{
		PreferredProvider: "lmstudio",
		PreferredModel:    "weak-model:v1",
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, hints)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.MatchedRule != "user_hint_explicit" {
		t.Fatalf("expected matched rule 'user_hint_explicit', got %q", result.MatchedRule)
	}
	if result.Provider != "lmstudio" {
		t.Fatalf("expected provider 'lmstudio', got %q", result.Provider)
	}
	if result.Model != "weak-model:v1" {
		t.Fatalf("expected model 'weak-model:v1', got %q", result.Model)
	}
	if result.Strategy != PreferenceExplicit {
		t.Fatalf("expected strategy 'explicit', got %q", result.Strategy)
	}
}

// ---------------------------------------------------------------------------
// 6. Preference-based routing
// ---------------------------------------------------------------------------

func TestRoute_PreferenceFastest(t *testing.T) {
	reg := newRichTestRegistry()
	rules := []CodingRule{
		{
			Name: "fastest-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultyTrivial},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyTrivial,
		TaskType:   TaskFix,
	}
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Fastest should prefer the smallest model (tiny-model:v1, size 1.5b).
	if result.Model != "tiny-model:v1" {
		t.Fatalf("expected fastest model 'tiny-model:v1', got %q", result.Model)
	}
}

func TestRoute_PreferenceBestCoding(t *testing.T) {
	reg := newRichTestRegistry()
	rules := []CodingRule{
		{
			Name: "best-coding-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultyComplex},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceBestCoding,
				MinCoding: "medium",
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Best coding should prefer the strongest coding model (large-model:v1, coding=strong).
	if result.Model != "large-model:v1" {
		t.Fatalf("expected best coding model 'large-model:v1', got %q", result.Model)
	}
}

func TestRoute_PreferenceBestCombo(t *testing.T) {
	reg := newRichTestRegistry()
	rules := []CodingRule{
		{
			Name: "best-combo-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultyComplex},
			},
			RouteAction: RuleAction{
				Prefer:       PreferenceBestCombo,
				MinCoding:    "medium",
				MinReasoning: "medium",
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Best combo should prefer the model with highest weighted score (large-model:v1).
	if result.Model != "large-model:v1" {
		t.Fatalf("expected best combo model 'large-model:v1', got %q", result.Model)
	}
}

// ---------------------------------------------------------------------------
// 7. Alternatives population
// ---------------------------------------------------------------------------

func TestRoute_Alternatives_RejectedCandidates(t *testing.T) {
	reg := newRichTestRegistry()
	rules := []CodingRule{
		{
			Name: "strict-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultyComplex},
			},
			RouteAction: RuleAction{
				Prefer:     PreferenceBestCoding,
				MinCoding:  "strong",
				MinContext: 16000,
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Only large-model:v1 meets min_coding=strong and min_context=16000.
	// tiny-model:v1 and small-model:v1 should appear in alternatives.
	if len(result.Alternatives) == 0 {
		t.Fatal("expected at least one alternative to be populated")
	}
	// Check that rejected alternatives have reasons.
	for _, alt := range result.Alternatives {
		if alt.Rejected == "" {
			t.Fatalf("expected alternative %s/%s to have a rejection reason", alt.Provider, alt.Model)
		}
	}
}

func TestRoute_Alternatives_NotAvailable(t *testing.T) {
	reg := newRichTestRegistry()
	rules := []CodingRule{
		{
			Name: "simple-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultySimple},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
	}
	// Only tiny-model:v1 is available; small-model:v1 and large-model:v1 are not.
	available := map[string]bool{
		"provider-tiny:tiny-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// The alternatives should include small-model:v1 and large-model:v1 as not_available.
	hasNotAvailable := false
	for _, alt := range result.Alternatives {
		if alt.Rejected == "not_available" {
			hasNotAvailable = true
			break
		}
	}
	if !hasNotAvailable {
		t.Fatal("expected at least one alternative with reason 'not_available'")
	}
}

// ---------------------------------------------------------------------------
// 8. Fallback relaxation
// ---------------------------------------------------------------------------

func TestRoute_FallbackRelaxation_NoModelMeetsMinRequirements(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	// Only the weak model is available, but complex-coding requires min_coding=strong.
	// FindBest has internal relaxation: when strict filter returns nothing, it
	// retries with no requirements. So a model is still selected.
	available := map[string]bool{"lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model == "" {
		t.Fatal("expected a model to be selected via internal relaxation")
	}
}

func TestRoute_FallbackRelaxation_NoModelsAtAll(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	// No models available at all.
	available := map[string]bool{}
	result := engine.Route(profile, available, nil)
	if result != nil {
		t.Fatal("expected nil when no models are available at all")
	}
}

// ---------------------------------------------------------------------------
// 9. Memory fit boost
// ---------------------------------------------------------------------------

func TestRoute_MemoryFitBoost_SelectsRecommendedModel(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, &fakeModelFitLookup{
		modelID:     "ollama:strong-model:v1",
		successRate: 0.85,
		ok:          true,
	})
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model != "strong-model:v1" {
		t.Fatalf("expected model 'strong-model:v1' from memory fit boost, got %q", result.Model)
	}
}

func TestRoute_MemoryFitBoost_LowSuccessRateIgnored(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, &fakeModelFitLookup{
		modelID:     "ollama:strong-model:v1",
		successRate: 0.5, // below 0.7 threshold
		ok:          true,
	})
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// Should still return a valid result, but the memory fit boost is not applied.
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

func TestRoute_MemoryFitBoost_NotOkIgnored(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, &fakeModelFitLookup{
		modelID:     "ollama:strong-model:v1",
		successRate: 0.0,
		ok:          false,
	})
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

func TestRoute_MemoryFitBoost_NilLookup(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil) // nil memoryFit
	profile := &CodingTaskProfile{
		Difficulty: DifficultyModerate,
		TaskType:   TaskFeature,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

// ---------------------------------------------------------------------------
// 10. Hint overrides (MinContext)
// ---------------------------------------------------------------------------

// newHintTestRegistry creates a registry with distinct context limits for
// testing MinContext hint overrides. Avoids the dedup bug in newTestRegistry
// where the first profile claims all model names.
func newHintTestRegistry() *CodingModelRegistry {
	return &CodingModelRegistry{
		entries: []CodingModelRegistryEntry{
			{
				ProfileID:      "strong-model",
				Provider:       "ollama",
				ModelName:      "strong-model:v1",
				CodingStrength: CodingStrengthStrong,
				ToolCalling:    "strong",
				Reasoning:      "strong",
				ContextLimit:   65536,
				SizeCategory:   SizeSmall,
			},
			{
				ProfileID:      "weak-model",
				Provider:       "lmstudio",
				ModelName:      "weak-model:v1",
				CodingStrength: CodingStrengthWeak,
				ToolCalling:    "weak",
				Reasoning:      "weak",
				ContextLimit:   32000,
				SizeCategory:   SizeTiny,
			},
		},
	}
}

func TestRoute_HintMinContext_OverridesRule(t *testing.T) {
	reg := newHintTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
	}
	// simple-fix rule has MinContext=4096. Hint raises it to 50000.
	hints := &api.RoutingExtensions{
		MinContext: 50000,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, hints)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// strong-model:v1 has 65536 context, weak-model:v1 has 32000.
	// With effectiveMinContext=50000, only strong-model:v1 qualifies.
	if result.Model != "strong-model:v1" {
		t.Fatalf("expected model 'strong-model:v1' (context 65536 >= 50000), got %q", result.Model)
	}
}

func TestRoute_HintMinContext_LowerThanRule_UsesRule(t *testing.T) {
	reg := newHintTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
	}
	// simple-fix rule has MinContext=4096. Hint is lower, so rule value is used.
	hints := &api.RoutingExtensions{
		MinContext: 1000,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, hints)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

func TestRoute_HintMinContext_NoRuleMinContext(t *testing.T) {
	reg := newHintTestRegistry()
	rules := []CodingRule{
		{
			Name: "no-min-context-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultySimple},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
				// No MinContext set.
			},
		},
	}
	engine := NewCodingRuleEngine(rules, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultySimple,
		TaskType:   TaskFix,
	}
	hints := &api.RoutingExtensions{
		MinContext: 50000,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, hints)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// effectiveMinContext = max(0, 50000) = 50000, so only strong-model:v1 qualifies.
	if result.Model != "strong-model:v1" {
		t.Fatalf("expected model 'strong-model:v1' (context 65536 >= 50000), got %q", result.Model)
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestRoute_NoMatchingRule_ReturnsNil(t *testing.T) {
	reg := newTestRegistry()
	// Empty rules list.
	engine := NewCodingRuleEngine([]CodingRule{}, reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyTrivial,
		TaskType:   TaskFix,
	}
	available := map[string]bool{"ollama:strong-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result != nil {
		t.Fatal("expected nil when no rules match")
	}
}

func TestRoute_DefaultRules_AllDifficultiesMatch(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}

	for _, d := range []int{DifficultyTrivial, DifficultySimple, DifficultyModerate, DifficultyComplex, DifficultyNovel} {
		profile := &CodingTaskProfile{
			Difficulty: d,
			TaskType:   TaskFeature,
		}
		result := engine.Route(profile, available, nil)
		if result == nil {
			t.Fatalf("expected non-nil result for difficulty %d", d)
		}
		if result.MatchedRule == "" {
			t.Fatalf("expected a matched rule for difficulty %d", d)
		}
	}
}

func TestRoute_OverrideMerging(t *testing.T) {
	// Use a custom rule without MaxSize to avoid size constraints.
	rules := []CodingRule{
		{
			Name: "test-rule",
			When: RuleCondition{
				Difficulty: []int{DifficultyTrivial},
			},
			RouteAction: RuleAction{
				Prefer:    PreferenceFastest,
				MinCoding: "weak",
			},
		},
	}
	reg := newTestRegistry()
	overrides := []config.CodingRuleOverride{
		{
			Name:      "test-rule",
			Prefer:    string(PreferenceBestCoding),
			MinCoding: "medium",
		},
	}
	engine := NewCodingRuleEngine(rules, reg, overrides, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyTrivial,
		TaskType:   TaskFix,
	}
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	// With override, test-rule now uses PreferenceBestCoding and min_coding=medium.
	// Only strong-model:v1 meets medium coding.
	if result.Model != "strong-model:v1" {
		t.Fatalf("expected model 'strong-model:v1' after override, got %q", result.Model)
	}
	if result.Strategy != PreferenceBestCoding {
		t.Fatalf("expected strategy 'best_coding' after override, got %q", result.Strategy)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func intPtr(i int) *int { return &i }
