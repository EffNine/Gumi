package baselines

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

func TestStoreSaveRuntimeScoped(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report := &types.GEPReport{
		RunID: "runtime-run-001",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.8},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {Gumi: types.GEPMetricSet{Mean: 0.9}},
		},
	}

	if err := store.Save(report); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was saved in runtime scope directory
	expectedPath := filepath.Join(tmpDir, string(types.ScopeRuntime), "test-model")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected baseline dir %s to exist", expectedPath)
	}
}

func TestStoreSaveModelScoped(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report := &types.GEPReport{
		RunID: "model-run-001",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeModel,
		},
		Summary: types.GEPSummary{OverallScore: 0.7},
	}

	if err := store.Save(report); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was saved in model scope directory
	expectedPath := filepath.Join(tmpDir, string(types.ScopeModel), "test-model")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected baseline dir %s to exist", expectedPath)
	}
}

func TestStoreLoadAcrossScopes(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Save a runtime baseline
	runtimeReport := &types.GEPReport{
		RunID: "runtime-001",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.8},
	}
	if err := store.Save(runtimeReport); err != nil {
		t.Fatalf("Save runtime failed: %v", err)
	}

	// Save a model baseline
	modelReport := &types.GEPReport{
		RunID: "model-001",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC().Add(-1 * time.Hour),
			Scope:     types.ScopeModel,
		},
		Summary: types.GEPSummary{OverallScore: 0.7},
	}
	if err := store.Save(modelReport); err != nil {
		t.Fatalf("Save model failed: %v", err)
	}

	// Load should find both
	baselines, err := store.Load("test-model")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(baselines) != 2 {
		t.Fatalf("expected 2 baselines, got %d", len(baselines))
	}
	// Should be sorted by timestamp (runtime first as it's newer)
	if baselines[0].RunID != "runtime-001" {
		t.Errorf("expected runtime-001 first, got %s", baselines[0].RunID)
	}
}

func TestStoreListModelsAcrossScopes(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create model dirs in both scopes
	os.MkdirAll(filepath.Join(tmpDir, string(types.ScopeRuntime), "model-a"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, string(types.ScopeRuntime), "model-b"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, string(types.ScopeModel), "model-c"), 0755)

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
	expected := []string{"model-a", "model-b", "model-c"}
	for i, exp := range expected {
		if models[i] != exp {
			t.Errorf("expected models[%d]=%s, got %s", i, exp, models[i])
		}
	}
}

func TestStoreCompareWithScope(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	baseline := &types.GEPReport{
		RunID: "baseline-001",
		Config: types.GEPRunConfig{
			Model:     "compare-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC().Add(-1 * time.Hour),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.8},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {Gumi: types.GEPMetricSet{Mean: 0.85}},
		},
	}
	if err := store.Save(baseline); err != nil {
		t.Fatalf("Save baseline failed: %v", err)
	}

	current := &types.GEPReport{
		RunID: "current-001",
		Config: types.GEPRunConfig{
			Model:     "compare-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.9},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {Gumi: types.GEPMetricSet{Mean: 0.95}},
		},
	}

	reg, err := store.Compare(current)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if reg == nil {
		t.Fatal("expected regression result, got nil")
	}
	if reg.Regression {
		t.Error("expected no regression for higher score")
	}
	if reg.Delta < 0.099 || reg.Delta > 0.101 {
		t.Errorf("expected delta approx 0.10, got %f", reg.Delta)
	}
}
