// Package types contains GEP type tests.
package types

import (
	"testing"
	"time"
)

func TestGEPResultTimestampZeroValue(t *testing.T) {
	r := GEPResult{}
	// time.Time zero value should be valid
	_ = r.Timestamp.IsZero()
}

func TestGEPResultTimestampSet(t *testing.T) {
	now := time.Now()
	r := GEPResult{Timestamp: now}
	if !r.Timestamp.Equal(now) {
		t.Errorf("expected %v, got %v", now, r.Timestamp)
	}
}

func TestGEPReportEmpty(t *testing.T) {
	r := GEPReport{}
	if r.SchemaVersion != 0 {
		t.Errorf("expected SchemaVersion 0, got %d", r.SchemaVersion)
	}
	if r.ProtocolVersion != "" {
		t.Errorf("expected empty ProtocolVersion, got %s", r.ProtocolVersion)
	}
}

func TestGESuiteDefaults(t *testing.T) {
	s := GESuite{
		ID:         "test-suite",
		Type:       SuiteInstructionFollowing,
		Difficulty: TierEasy,
	}
	if s.ID != "test-suite" {
		t.Errorf("expected ID test-suite, got %s", s.ID)
	}
	if s.Type != SuiteInstructionFollowing {
		t.Errorf("expected type instruction_following, got %s", s.Type)
	}
	if s.AttemptsRecommended == 0 {
		t.Log("AttemptsRecommended defaults to 0 (reasonable)")
	}
}

func TestProviderTypeConstants(t *testing.T) {
	if ProviderLMStudio != "lmstudio" {
		t.Errorf("expected lmstudio, got %s", ProviderLMStudio)
	}
	if ProviderOllama != "ollama" {
		t.Errorf("expected ollama, got %s", ProviderOllama)
	}
}

func TestDifficultyTierConstants(t *testing.T) {
	if TierEasy != "easy" {
		t.Errorf("expected easy, got %s", TierEasy)
	}
	if TierMedium != "medium" {
		t.Errorf("expected medium, got %s", TierMedium)
	}
	if TierHard != "hard" {
		t.Errorf("expected hard, got %s", TierHard)
	}
}

func TestGEPBaseline(t *testing.T) {
	b := GEPBaseline{
		RunID:        "test-run-001",
		Model:        "test-model",
		Provider:     ProviderLMStudio,
		Scope:        ScopeRuntime,
		OverallScore: 0.85,
		Capabilities: map[string]GEPCapability{
			"instruction_following": {SuiteType: SuiteInstructionFollowing, Gumi: GEPMetricSet{Mean: 0.9, N: 10}},
		},
	}
	if b.RunID != "test-run-001" {
		t.Errorf("unexpected RunID: %s", b.RunID)
	}
	if b.OverallScore != 0.85 {
		t.Errorf("unexpected OverallScore: %f", b.OverallScore)
	}
}

func TestGEPRegressionBasic(t *testing.T) {
	r := GEPRegression{
		BaselineRunID: "baseline-001",
		BaselineScore: 0.80,
		CurrentScore:  0.75,
		Delta:         -0.05,
		Regression:    true,
	}
	if !r.Regression {
		t.Error("expected regression=true for negative delta")
	}
	if r.Delta != -0.05 {
		t.Errorf("unexpected delta: %f", r.Delta)
	}
}
