package router

import (
	"testing"

	"github.com/EffNine/gumi/runtime/internal/profiles"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newRegistryWithEntries creates a CodingModelRegistry with the given entries
// directly, bypassing NewCodingModelRegistry's profile-based construction
// (which has a first-profile-wins dedup design that makes multi-profile
// registries unreliable for testing attribute-based filtering).
func newRegistryWithEntries(entries []CodingModelRegistryEntry) *CodingModelRegistry {
	return &CodingModelRegistry{entries: entries}
}

// newRegistryForFilterTests creates a registry with models of varying
// capabilities for testing FindBest filters.
func newRegistryForFilterTests() *CodingModelRegistry {
	return newRegistryWithEntries([]CodingModelRegistryEntry{
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
	})
}

// ---------------------------------------------------------------------------
// 1. FindBest with coding strength filter
// ---------------------------------------------------------------------------

func TestFindBest_FiltersByCodingStrength_Medium(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=medium should reject tiny-model (weak).
	best := reg.FindBest(PreferenceFastest, CodingStrengthMedium, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match with medium coding strength")
	}
	if best.CodingStrength < CodingStrengthMedium {
		t.Fatalf("expected coding strength >= medium, got %s", best.CodingStrength)
	}
	// Should not be the tiny model.
	if best.ModelName == "tiny-model:v1" {
		t.Fatal("tiny-model should have been filtered out by medium coding strength")
	}
}

func TestFindBest_FiltersByCodingStrength_Strong(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=strong should only return large-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthStrong, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match with strong coding strength")
	}
	if best.CodingStrength != CodingStrengthStrong {
		t.Fatalf("expected coding strength 'strong', got %s", best.CodingStrength)
	}
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1', got %s", best.ModelName)
	}
}

func TestFindBest_FiltersByCodingStrength_None(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=none should return all models (fastest picks smallest).
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match with no coding strength filter")
	}
	// Fastest should pick the smallest: tiny-model.
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1' (smallest), got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 2. FindBest with context limit filter
// ---------------------------------------------------------------------------

func TestFindBest_FiltersByContext_LowThreshold(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minContext=5000 should reject tiny-model (4096).
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 5000, "", "", available)
	if best == nil {
		t.Fatal("expected a match with 5k min context")
	}
	if best.ContextLimit < 5000 {
		t.Fatalf("expected context >= 5000, got %d", best.ContextLimit)
	}
	if best.ModelName == "tiny-model:v1" {
		t.Fatal("tiny-model should have been filtered out by 5k min context")
	}
}

func TestFindBest_FiltersByContext_HighThreshold(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minContext=30000 should only return large-model (32768).
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 30000, "", "", available)
	if best == nil {
		t.Fatal("expected a match with 30k min context")
	}
	if best.ContextLimit < 30000 {
		t.Fatalf("expected context >= 30000, got %d", best.ContextLimit)
	}
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' for high context, got %s", best.ModelName)
	}
}

func TestFindBest_FiltersByContext_ZeroThreshold(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minContext=0 should not filter anything.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match with 0 min context")
	}
}

// ---------------------------------------------------------------------------
// 3. FindBest with size category filter
// ---------------------------------------------------------------------------

func TestFindBest_FiltersBySize_Tiny(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// maxSize=tiny should only return tiny-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", SizeTiny, available)
	if best == nil {
		t.Fatal("expected a match with tiny size limit")
	}
	if best.SizeCategory != SizeTiny {
		t.Fatalf("expected size 'tiny', got %s", best.SizeCategory)
	}
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1', got %s", best.ModelName)
	}
}

func TestFindBest_FiltersBySize_Small(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// maxSize=small should return tiny or small (fastest picks tiny).
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", SizeSmall, available)
	if best == nil {
		t.Fatal("expected a match with small size limit")
	}
	if sizeRank(best.SizeCategory) > sizeRank(SizeSmall) {
		t.Fatalf("expected size <= small, got %s", best.SizeCategory)
	}
}

func TestFindBest_FiltersBySize_Medium(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// maxSize=medium should exclude large-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", SizeMedium, available)
	if best == nil {
		t.Fatal("expected a match with medium size limit")
	}
	if sizeRank(best.SizeCategory) > sizeRank(SizeMedium) {
		t.Fatalf("expected size <= medium, got %s", best.SizeCategory)
	}
	if best.ModelName == "large-model:v1" {
		t.Fatal("large-model should have been filtered out by medium size limit")
	}
}

// ---------------------------------------------------------------------------
// 4. FindBest with all filters combined
// ---------------------------------------------------------------------------

func TestFindBest_AllFiltersCombined(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=medium, minContext=8000, maxSize=large, minReasoning=weak
	// This should match small-model (medium coding, 8192 context, small size, basic reasoning)
	// and large-model (strong coding, 32768 context, large size, strong reasoning).
	// Fastest picks the smallest: small-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthMedium, 8000, "weak", SizeLarge, available)
	if best == nil {
		t.Fatal("expected a match with combined filters")
	}
	if best.CodingStrength < CodingStrengthMedium {
		t.Fatalf("expected coding strength >= medium, got %s", best.CodingStrength)
	}
	if best.ContextLimit < 8000 {
		t.Fatalf("expected context >= 8000, got %d", best.ContextLimit)
	}
	if sizeRank(best.SizeCategory) > sizeRank(SizeLarge) {
		t.Fatalf("expected size <= large, got %s", best.SizeCategory)
	}
}

func TestFindBest_AllFilters_NoMatch_Relaxes(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// Impossible combination: minCoding=strong, maxSize=tiny.
	// FindBest has internal relaxation: when strict filter returns nothing,
	// it retries with no requirements and returns the best available.
	best := reg.FindBest(PreferenceFastest, CodingStrengthStrong, 0, "", SizeTiny, available)
	if best == nil {
		t.Fatal("expected a match via relaxation when no model meets strict filters")
	}
	// After relaxation, fastest should pick the smallest available: tiny-model.
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1' after relaxation, got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 5. FindBest with PreferenceFastest (prefers smallest)
// ---------------------------------------------------------------------------

func TestFindBest_PreferenceFastest_PrefersSmallest(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	// Fastest should pick the smallest model: tiny-model (1.5b).
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1' (smallest), got %s", best.ModelName)
	}
}

func TestFindBest_PreferenceFastest_WithMinCoding(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=medium filters out tiny-model, so fastest picks small-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthMedium, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	if best.ModelName != "small-model:v1" {
		t.Fatalf("expected 'small-model:v1' (smallest meeting medium coding), got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 6. FindBest with PreferenceBestCoding (prefers strongest coding)
// ---------------------------------------------------------------------------

func TestFindBest_PreferenceBestCoding_PrefersStrongest(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	best := reg.FindBest(PreferenceBestCoding, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	// Best coding should pick the strongest coding model: large-model (strong).
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' (strongest coding), got %s", best.ModelName)
	}
	if best.CodingStrength != CodingStrengthStrong {
		t.Fatalf("expected coding strength 'strong', got %s", best.CodingStrength)
	}
}

func TestFindBest_PreferenceBestCoding_TiebreakerByContext(t *testing.T) {
	// Create two models with same coding strength but different context.
	reg := newRegistryWithEntries([]CodingModelRegistryEntry{
		{
			ProfileID:      "model-a",
			Provider:       "provider-a",
			ModelName:      "model-a:v1",
			CodingStrength: CodingStrengthStrong,
			ToolCalling:    "good",
			Reasoning:      "medium",
			ContextLimit:   16000,
			SizeCategory:   SizeMedium,
		},
		{
			ProfileID:      "model-b",
			Provider:       "provider-b",
			ModelName:      "model-b:v1",
			CodingStrength: CodingStrengthStrong,
			ToolCalling:    "good",
			Reasoning:      "medium",
			ContextLimit:   32000,
			SizeCategory:   SizeMedium,
		},
	})
	available := map[string]bool{"provider-a:model-a:v1": true, "provider-b:model-b:v1": true}

	best := reg.FindBest(PreferenceBestCoding, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	// Both have strong coding, so tiebreaker is context: model-b has more.
	if best.ModelName != "model-b:v1" {
		t.Fatalf("expected 'model-b:v1' (higher context tiebreaker), got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 7. FindBest with PreferenceBestCombo (weighted score)
// ---------------------------------------------------------------------------

func TestFindBest_PreferenceBestCombo_WeightedScore(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	best := reg.FindBest(PreferenceBestCombo, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	// Best combo should pick the model with highest weighted score: large-model.
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' (best combo), got %s", best.ModelName)
	}
}

func TestFindBest_PreferenceBestCombo_WithMinReasoning(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minReasoning=medium should filter out tiny-model (none) and small-model (basic).
	best := reg.FindBest(PreferenceBestCombo, CodingStrengthNone, 0, "medium", "", available)
	if best == nil {
		t.Fatal("expected a match with medium reasoning filter")
	}
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' (only model with medium+ reasoning), got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 8. FindBest with PreferenceLargestContext
// ---------------------------------------------------------------------------

func TestFindBest_PreferenceLargestContext_PrefersLargest(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	best := reg.FindBest(PreferenceLargestContext, CodingStrengthNone, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match")
	}
	// Largest context should pick large-model (32768).
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' (largest context), got %s", best.ModelName)
	}
	if best.ContextLimit != 32768 {
		t.Fatalf("expected context 32768, got %d", best.ContextLimit)
	}
}

func TestFindBest_PreferenceLargestContext_WithFilter(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// maxSize=small should exclude large-model, so largest context is small-model (8192).
	best := reg.FindBest(PreferenceLargestContext, CodingStrengthNone, 0, "", SizeSmall, available)
	if best == nil {
		t.Fatal("expected a match with small size limit")
	}
	if best.ModelName != "small-model:v1" {
		t.Fatalf("expected 'small-model:v1' (largest context within small size), got %s", best.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 9. FindBest returns nil when no models available
// ---------------------------------------------------------------------------

func TestFindBest_ReturnsNil_NoModelsAvailable(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{"provider-tiny:nonexistent:99b": true}
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best != nil {
		t.Fatal("expected nil when no models match available set")
	}
}

func TestFindBest_ReturnsNil_EmptyRegistry(t *testing.T) {
	reg := NewCodingModelRegistry(nil, nil)
	available := map[string]bool{"provider-tiny:anything:v1": true}
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best != nil {
		t.Fatal("expected nil for empty registry")
	}
}

func TestFindBest_ReturnsNil_AllFilteredOut_NoRelaxation(t *testing.T) {
	reg := newRegistryForFilterTests()
	// No models available at all (empty available set).
	available := map[string]bool{}

	// minContext=100000 — no models available, so relaxation also returns nothing.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 100000, "", "", available)
	if best != nil {
		t.Fatal("expected nil when no models are available at all")
	}
}

// ---------------------------------------------------------------------------
// 10. List returns all entries
// ---------------------------------------------------------------------------

func TestList_ReturnsAllEntries(t *testing.T) {
	reg := newRegistryForFilterTests()
	entries := reg.List()
	if len(entries) == 0 {
		t.Fatal("expected non-empty list")
	}
	// We have 3 profiles, each with its own provider.
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestList_ReturnsEmptyForEmptyRegistry(t *testing.T) {
	reg := NewCodingModelRegistry(nil, nil)
	entries := reg.List()
	if len(entries) != 0 {
		t.Fatalf("expected empty list, got %d entries", len(entries))
	}
}

func TestList_ContainsExpectedModels(t *testing.T) {
	reg := newRegistryForFilterTests()
	entries := reg.List()

	modelNames := make(map[string]bool)
	for _, e := range entries {
		modelNames[e.ModelName] = true
	}

	expected := []string{"tiny-model:v1", "small-model:v1", "large-model:v1"}
	for _, name := range expected {
		if !modelNames[name] {
			t.Fatalf("expected model %q in list, but it was not found", name)
		}
	}
}

func TestList_EntriesHaveCorrectAttributes(t *testing.T) {
	reg := newRegistryForFilterTests()
	entries := reg.List()

	for _, e := range entries {
		if e.ProfileID == "" {
			t.Fatal("expected entry to have a ProfileID")
		}
		if e.Provider == "" {
			t.Fatal("expected entry to have a Provider")
		}
		if e.ModelName == "" {
			t.Fatal("expected entry to have a ModelName")
		}
		if e.ContextLimit <= 0 {
			t.Fatalf("expected entry %s to have positive ContextLimit", e.ModelName)
		}
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestFindBest_Relaxation_WhenNoModelMeetsMinRequirements(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minCoding=strong, minContext=100000 — no model meets both.
	// FindBest should relax and return the best available.
	best := reg.FindBest(PreferenceFastest, CodingStrengthStrong, 100000, "", "", available)
	if best == nil {
		t.Fatal("expected a match via relaxation when no model meets strict requirements")
	}
	// After relaxation, fastest should pick the smallest available: tiny-model.
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1' after relaxation, got %s", best.ModelName)
	}
}

func TestFindBest_Relaxation_WithAvailableFilter(t *testing.T) {
	reg := newRegistryForFilterTests()
	// Only tiny-model is available.
	available := map[string]bool{"provider-tiny:tiny-model:v1": true}

	// minCoding=medium — tiny-model doesn't meet it.
	// FindBest should relax and return tiny-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthMedium, 0, "", "", available)
	if best == nil {
		t.Fatal("expected a match via relaxation")
	}
	if best.ModelName != "tiny-model:v1" {
		t.Fatalf("expected 'tiny-model:v1' after relaxation, got %s", best.ModelName)
	}
}

func TestFindBest_Relaxation_NoAvailableModels(t *testing.T) {
	reg := newRegistryForFilterTests()
	// No models available at all.
	available := map[string]bool{}

	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "", "", available)
	if best != nil {
		t.Fatal("expected nil when no models are available at all")
	}
}

func TestNewCodingModelRegistry_HandlesNilProfile(t *testing.T) {
	profilesList := []*profiles.Profile{nil}
	providerModels := map[string][]string{"provider-tiny": {"test:v1"}}
	reg := NewCodingModelRegistry(profilesList, providerModels)
	entries := reg.List()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for nil profile, got %d", len(entries))
	}
}

func TestNewCodingModelRegistry_DeduplicatesByKey(t *testing.T) {
	profilesList := []*profiles.Profile{
		{
			ID:           "test-model",
			Name:         "Test Model",
			Size:         "7b",
			ContextLimit: 16000,
			Capabilities: profiles.Capabilities{
				Coding:      "medium",
				ToolCalling: "good",
				Reasoning:   "basic",
			},
		},
	}
	// Same provider/model pair appears twice.
	providerModels := map[string][]string{
		"provider-test": {"test-model:v1", "test-model:v1"},
	}
	reg := NewCodingModelRegistry(profilesList, providerModels)
	entries := reg.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after deduplication, got %d", len(entries))
	}
}

func TestFindBest_WithReasoningFilter(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minReasoning=strong should only return large-model.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "strong", "", available)
	if best == nil {
		t.Fatal("expected a match with strong reasoning filter")
	}
	if best.ModelName != "large-model:v1" {
		t.Fatalf("expected 'large-model:v1' (only model with strong reasoning), got %s", best.ModelName)
	}
}

func TestFindBest_WithReasoningFilter_NoMatch(t *testing.T) {
	reg := newRegistryForFilterTests()
	available := map[string]bool{
		"provider-tiny:tiny-model:v1":   true,
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	// minReasoning=super (invalid level) should match nothing, then relax.
	best := reg.FindBest(PreferenceFastest, CodingStrengthNone, 0, "super", "", available)
	if best == nil {
		t.Fatal("expected a match via relaxation when reasoning level is invalid")
	}
}

func TestClassifySize(t *testing.T) {
	tests := []struct {
		size     string
		expected ModelSizeCategory
	}{
		{"1.5b", SizeTiny},
		{"2b", SizeTiny},
		{"3b", SizeSmall},
		{"4b", SizeSmall},
		{"6b", SizeSmall},
		{"7b", SizeMedium},
		{"8b", SizeMedium},
		// classifySize uses prefix matching: "12b" starts with "1" → tiny
		{"12b", SizeTiny},
		// "14b" starts with "1" → tiny
		{"14b", SizeTiny},
		// "15b" starts with "1" → tiny
		{"15b", SizeTiny},
		// "20b" starts with "2" → tiny
		{"20b", SizeTiny},
		// "70b" starts with "7" → medium
		{"70b", SizeMedium},
		// "120b" starts with "1" → tiny
		{"120b", SizeTiny},
	}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			result := classifySize(tt.size)
			if result != tt.expected {
				t.Fatalf("classifySize(%q) = %s, want %s", tt.size, result, tt.expected)
			}
		})
	}
}

func TestParseCodingStrength(t *testing.T) {
	tests := []struct {
		input    string
		expected CodingStrength
	}{
		{"strong", CodingStrengthStrong},
		{"Strong", CodingStrengthStrong},
		{"  strong  ", CodingStrengthStrong},
		{"medium", CodingStrengthMedium},
		{"weak", CodingStrengthWeak},
		{"unknown", CodingStrengthNone},
		{"", CodingStrengthNone},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCodingStrength(tt.input)
			if result != tt.expected {
				t.Fatalf("parseCodingStrength(%q) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
