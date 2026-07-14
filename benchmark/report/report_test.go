package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestWriteJSON_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	r := &Report{
		RunResult: benchmark.RunResult{
			SchemaVersion: 1,
			RunID:         "test-run-001",
			Model:         "test-model",
			Provider:      "lmstudio",
			ModelTier:     "small",
			Config: benchmark.RunConfig{
				Attempts: 2,
			},
			Summary: benchmark.Summary{
				OverallScore: 0.85,
				WorthIt:      true,
			},
		},
	}

	if err := WriteJSON(r, path); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Verify file exists and is not empty
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("WriteJSON created empty file")
	}
}

func TestWriteJSON_Deserializable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	r := &Report{
		RunResult: benchmark.RunResult{
			SchemaVersion: 1,
			RunID:         "test-run-002",
			Model:         "qwen3.5-9b",
			Provider:      "lmstudio",
			ModelTier:     "medium",
			Config: benchmark.RunConfig{
				Attempts:   3,
				Conditions: []string{"direct", "gumi-stabilized"},
				Timestamp:  "2026-07-13T12:00:00Z",
			},
			Summary: benchmark.Summary{
				OverallScore:      0.72,
				LatencyOverheadMs: 150.5,
				DegradationRate:   0.05,
				WorthIt:           true,
			},
			Capabilities: map[string]benchmark.Capability{
				"json": {
					Direct:     benchmark.MetricSet{Mean: 0.65, Std: 0.12, N: 10},
					Gumi:     benchmark.MetricSet{Mean: 0.82, Std: 0.09, N: 10},
					Delta:      0.17,
					EffectSize: 0.62,
				},
			},
			PerTest: []benchmark.TestResult{
				{
					TestID:    "json-1",
					Condition: "direct",
					Attempt:   1,
					Passed:    true,
					Subscores: map[string]float64{"valid": 1.0},
					LatencyMs: 120.0,
				},
			},
			Degradation: benchmark.DegradationReport{
				OverRepairCount: 1,
				TotalTests:      20,
				DegradationRate: 0.05,
			},
		},
	}

	if err := WriteJSON(r, path); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Read back and deserialize
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}

	if decoded.RunResult.RunID != "test-run-002" {
		t.Errorf("RunID = %q, want %q", decoded.RunResult.RunID, "test-run-002")
	}
	if decoded.RunResult.Model != "qwen3.5-9b" {
		t.Errorf("Model = %q, want %q", decoded.RunResult.Model, "qwen3.5-9b")
	}
	if decoded.RunResult.Summary.OverallScore != 0.72 {
		t.Errorf("OverallScore = %v, want 0.72", decoded.RunResult.Summary.OverallScore)
	}
	if decoded.RunResult.Summary.WorthIt != true {
		t.Errorf("WorthIt = %v, want true", decoded.RunResult.Summary.WorthIt)
	}
	if len(decoded.RunResult.Capabilities) != 1 {
		t.Errorf("Capabilities count = %d, want 1", len(decoded.RunResult.Capabilities))
	}
	if len(decoded.RunResult.PerTest) != 1 {
		t.Errorf("PerTest count = %d, want 1", len(decoded.RunResult.PerTest))
	}
}

func TestWriteJSON_NilReport(t *testing.T) {
	err := WriteJSON(nil, "/tmp/nonexistent/report.json")
	if err == nil {
		t.Fatal("WriteJSON with nil report should return error")
	}
}

func TestWriteMarkdown_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")

	r := &Report{
		RunResult: benchmark.RunResult{
			SchemaVersion: 1,
			RunID:         "test-run-003",
			Model:         "test-model",
			Provider:      "ollama",
			ModelTier:     "small",
			Config: benchmark.RunConfig{
				Attempts: 1,
			},
			Summary: benchmark.Summary{
				OverallScore:      0.90,
				LatencyOverheadMs: 50.0,
				DegradationRate:   0.02,
				WorthIt:           true,
			},
			Capabilities: map[string]benchmark.Capability{
				"json": {
					Direct:     benchmark.MetricSet{Mean: 0.70, Std: 0.1, N: 5},
					Gumi:     benchmark.MetricSet{Mean: 0.85, Std: 0.08, N: 5},
					Delta:      0.15,
					EffectSize: 0.60,
				},
			},
			Degradation: benchmark.DegradationReport{
				OverRepairCount: 1,
				TotalTests:      20,
				DegradationRate: 0.05,
			},
			PerTest: []benchmark.TestResult{
				{
					TestID:    "json-1",
					Condition: "direct",
					Attempt:   1,
					Passed:    true,
					Subscores: map[string]float64{"valid": 1.0},
					LatencyMs: 100.0,
				},
			},
		},
	}

	if err := WriteMarkdown(r, path); err != nil {
		t.Fatalf("WriteMarkdown failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("WriteMarkdown created empty file")
	}
}

func TestWriteMarkdown_NilReport(t *testing.T) {
	err := WriteMarkdown(nil, "/tmp/nonexistent/report.md")
	if err == nil {
		t.Fatal("WriteMarkdown with nil report should return error")
	}
}

func TestWriteMarkdown_NoCorruptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")

	r := &Report{
		RunResult: benchmark.RunResult{
			SchemaVersion: 1,
			RunID:         "test-run-004",
			Model:         "test",
			Provider:      "test",
			ModelTier:     "medium",
			Config:        benchmark.RunConfig{Attempts: 1},
			Summary:       benchmark.Summary{OverallScore: 0.5, WorthIt: true},
			Degradation:   benchmark.DegradationReport{TotalTests: 0},
			PerTest:       []benchmark.TestResult{},
		},
	}

	if err := WriteMarkdown(r, path); err != nil {
		t.Fatalf("WriteMarkdown failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("WriteMarkdown produced empty output")
	}
}

func TestStoreArtifact(t *testing.T) {
	// Override homedir detection by setting HOME temporarily
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
	})
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	err := StoreArtifact("run-abc", "json-1", "direct", 1, "test output")
	if err != nil {
		t.Fatalf("StoreArtifact failed: %v", err)
	}

	path := filepath.Join(tmpHome, ".gumi", "benchmarks", "run-abc", "artifacts", "json-1-direct-1.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile artifact: %v", err)
	}
	if string(data) != "test output" {
		t.Errorf("artifact content = %q, want %q", string(data), "test output")
	}
}
