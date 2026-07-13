package router

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/profiles"
)

func TestFindBestFiltersByCodingStrength(t *testing.T) {
	reg := newTestRegistry()
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}

	// Should reject weak models when min is medium.
	best := reg.FindBest(PreferenceFastest, CodingStrengthMedium, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match with medium coding strength")
	}
	if best.CodingStrength < CodingStrengthMedium {
		t.Fatalf("expected coding strength >= medium, got %s", best.CodingStrength)
	}
}

func TestFindBestFiltersByContext(t *testing.T) {
	reg := newTestRegistry()
	available := map[string]bool{"ollama:strong-model:v1": true, "lmstudio:weak-model:v1": true}

	// strong-model:v1 has 65k context, weak-model:v1 has 32k.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 50000, "", "", available)
	if best == nil {
		t.Fatal("expected a match with 50k min context")
	}
	if best.ContextLimit < 50000 {
		t.Fatalf("expected context >= 50000, got %d", best.ContextLimit)
	}
	if best.ModelName != "strong-model:v1" {
		t.Fatalf("expected strong-model:v1 for high context, got %s", best.ModelName)
	}
}

func TestFindBestFiltersBySize(t *testing.T) {
	// Create a registry with a small model first so it claims all names.
	profilesList := []*profiles.Profile{
		{
			ID:           "small-model",
			Name:         "Small Model",
			Size:         "3b",
			ContextLimit: 32000,
			Capabilities: profiles.Capabilities{
				Coding:      "weak",
				ToolCalling: "weak",
				Reasoning:   "weak",
			},
		},
	}
	providerModels := map[string][]string{
		"ollama": {"small-model:v1"},
	}
	reg := NewCodingModelRegistry(profilesList, providerModels)
	available := map[string]bool{"ollama:small-model:v1": true}

	// Max size small should return the small model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", SizeSmall, available)
	if best == nil {
		t.Fatal("expected a match with small size limit")
	}
	if sizeRank(best.SizeCategory) > sizeRank(SizeSmall) {
		t.Fatalf("expected size <= small, got %s", best.SizeCategory)
	}
}

func TestFindBestReturnsNilWhenNoModelsAvailable(t *testing.T) {
	reg := newTestRegistry()
	available := map[string]bool{"ollama:nonexistent:99b": true}
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best != nil {
		t.Fatal("expected nil when no models match available set")
	}
}
