//go:build integration

package runner

import (
	"testing"
)

func TestRun_Integration(t *testing.T) {
	cfg := Config{
		Model:    "test-model",
		Attempts: 1,
	}
	report, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.RunResult.Model != "test-model" {
		t.Errorf("expected model test-model, got %s", report.RunResult.Model)
	}
	if report.RunResult.ModelTier != "medium" {
		t.Errorf("expected tier medium, got %s", report.RunResult.ModelTier)
	}
	t.Logf("Report: model=%s tier=%s runID=%s scores=%+v",
		report.RunResult.Model,
		report.RunResult.ModelTier,
		report.RunResult.RunID,
		report.RunResult.Summary,
	)
}

func TestRun_DefaultsAttempts_Integration(t *testing.T) {
	cfg := Config{
		Model: "test-model",
	}
	report, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.RunResult.Config.Attempts != 3 {
		t.Errorf("default attempts = %d, want 3", report.RunResult.Config.Attempts)
	}
}

func TestConvertDegradationReport_Integration(t *testing.T) {
	cfg := Config{
		Model:    "test-model",
		Attempts: 1,
	}
	report, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.RunResult.Degradation.OverRepairCount < 0 {
		t.Errorf("OverRepairCount should be >= 0, got %d", report.RunResult.Degradation.OverRepairCount)
	}
}
