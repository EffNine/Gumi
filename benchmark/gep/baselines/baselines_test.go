package baselines

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	if store.path != tmpDir {
		t.Errorf("expected path %s, got %s", tmpDir, store.path)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report := &types.GEPReport{
		SchemaVersion:   2,
		RunID:           "test-run-001",
		ProtocolVersion: "2.0.0",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "lmstudio",
			Timestamp: time.Now().UTC(),
			Attempts:  3,
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{
			OverallScore: 0.85,
			PassRate:     0.9,
			TotalTests:   10,
			PassedTests:  9,
		},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {
				SuiteType: types.SuiteInstructionFollowing,
				Gumi:      types.GEPMetricSet{Mean: 0.9, N: 5},
			},
		},
	}

	if err := store.Save(report); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	baselines, err := store.Load("test-model")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(baselines) != 1 {
		t.Fatalf("expected 1 baseline, got %d", len(baselines))
	}
	if baselines[0].RunID != "test-run-001" {
		t.Errorf("expected run ID test-run-001, got %s", baselines[0].RunID)
	}
	if baselines[0].OverallScore != 0.85 {
		t.Errorf("expected score 0.85, got %f", baselines[0].OverallScore)
	}
}

func TestStoreGetLatest(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report1 := &types.GEPReport{
		RunID: "run-001",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC().Add(-2 * time.Hour),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.7},
	}
	report2 := &types.GEPReport{
		RunID: "run-002",
		Config: types.GEPRunConfig{
			Model:     "test-model",
			Provider:  "ollama",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.8},
	}

	store.Save(report1)
	store.Save(report2)

	latest, err := store.GetLatest("test-model")
	if err != nil {
		t.Fatalf("GetLatest failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest baseline, got nil")
	}
	if latest.RunID != "run-002" {
		t.Errorf("expected run-002, got %s", latest.RunID)
	}
}

func TestStoreGetLatestNone(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	latest, err := store.GetLatest("nonexistent-model")
	if err != nil {
		t.Fatalf("GetLatest failed: %v", err)
	}
	if latest != nil {
		t.Error("expected nil for nonexistent model")
	}
}

func TestStoreCompare(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	baseline := &types.GEPReport{
		RunID: "baseline-001",
		Config: types.GEPRunConfig{
			Model:     "compare-model",
			Provider:  "lmstudio",
			Timestamp: time.Now().UTC().Add(-1 * time.Hour),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.8},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {
				Gumi: types.GEPMetricSet{Mean: 0.85},
			},
		},
	}
	store.Save(baseline)

	current := &types.GEPReport{
		RunID: "current-001",
		Config: types.GEPRunConfig{
			Model:     "compare-model",
			Provider:  "lmstudio",
			Timestamp: time.Now().UTC(),
			Scope:     types.ScopeRuntime,
		},
		Summary: types.GEPSummary{OverallScore: 0.75},
		Capabilities: map[string]types.GEPCapability{
			"instruction_following": {
				Gumi: types.GEPMetricSet{Mean: 0.7},
			},
		},
	}

	reg, err := store.Compare(current)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if reg == nil {
		t.Fatal("expected regression result, got nil")
	}
	if !reg.Regression {
		t.Error("expected regression=true for lower score")
	}
	if reg.Delta < -0.051 || reg.Delta > -0.049 {
		t.Errorf("expected delta approx -0.05, got %f", reg.Delta)
	}
}

func TestStoreCompareNoBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report := &types.GEPReport{
		Config: types.GEPRunConfig{Model: "new-model"},
	}

	reg, err := store.Compare(report)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if reg != nil {
		t.Error("expected nil regression when no baseline exists")
	}
}

func TestStoreListModels(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Create some model directories
	os.MkdirAll(filepath.Join(tmpDir, "runtime", "model-a"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "runtime", "model-b"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "runtime", "model-c"), 0755)

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}
	if models[0] != "model-a" || models[2] != "model-c" {
		t.Error("expected models to be sorted")
	}
}

func TestStoreListModelsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	models, err := store.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}
