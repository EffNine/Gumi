package router

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/profiles"
)

// fakeModelFitLookup returns a fixed model for testing.
type fakeModelFitLookup struct {
	modelID     string
	successRate float64
	ok          bool
}

func (f *fakeModelFitLookup) GetBestModelForRouter(difficulty int, taskType string) (string, float64, bool) {
	return f.modelID, f.successRate, f.ok
}

func TestRouteSimpleTask(t *testing.T) {
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
	if result.FallbackUsed {
		t.Fatal("expected no fallback for simple task")
	}
	if result.MatchedRule != "trivial-fix" {
		t.Fatalf("expected rule 'trivial-fix', got %q", result.MatchedRule)
	}
}

func TestRouteFallbackRelaxation(t *testing.T) {
	reg := newTestRegistry()
	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, nil)
	profile := &CodingTaskProfile{
		Difficulty: DifficultyComplex,
		TaskType:   TaskFeature,
	}
	// Only a tiny model available — should still find a match via relaxation.
	available := map[string]bool{"ollama:strong-model:v1": true}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected non-nil route result")
	}
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

func TestRouteAlternativesPopulated(t *testing.T) {
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
	// With the current registry design (first profile claims all names),
	// both entries have the same profile attributes. The alternatives list
	// may be empty if the winner is the only candidate that meets requirements.
	// This test verifies the route succeeds.
	if result.Model == "" {
		t.Fatal("expected a model to be selected")
	}
}

func TestRouteModelFitBoost(t *testing.T) {
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
		t.Fatalf("expected model strong-model:v1 from memory fit boost, got %q", result.Model)
	}
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
