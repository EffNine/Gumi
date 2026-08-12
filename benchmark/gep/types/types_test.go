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

func TestPairResultsDeterministic(t *testing.T) {
	results := []GEPResult{
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionDirect, Passed: true, Subscores: map[string]float64{"score": 0.8}},
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"score": 0.9}},
		{SuiteID: "s1", TestID: "t2", Attempt: 1, Condition: ConditionDirect, Passed: false, Subscores: map[string]float64{"score": 0.3}},
		{SuiteID: "s1", TestID: "t2", Attempt: 1, Condition: ConditionGumiStabilized, Passed: false, Subscores: map[string]float64{"score": 0.4}},
		{SuiteID: "s1", TestID: "t1", Attempt: 2, Condition: ConditionDirect, Passed: true, Subscores: map[string]float64{"score": 0.7}},
		{SuiteID: "s1", TestID: "t1", Attempt: 2, Condition: ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"score": 0.85}},
	}

	pairs, err := PairResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}

	// Verify deterministic pairing by (SuiteID, TestID, Attempt).
	expected := []struct {
		suiteID string
		testID  string
		attempt int
	}{
		{"s1", "t1", 1},
		{"s1", "t2", 1},
		{"s1", "t1", 2},
	}
	for i, exp := range expected {
		if pairs[i].Key.SuiteID != exp.suiteID {
			t.Errorf("pair %d: expected suite %q, got %q", i, exp.suiteID, pairs[i].Key.SuiteID)
		}
		if pairs[i].Key.TestID != exp.testID {
			t.Errorf("pair %d: expected test %q, got %q", i, exp.testID, pairs[i].Key.TestID)
		}
		if pairs[i].Key.Attempt != exp.attempt {
			t.Errorf("pair %d: expected attempt %d, got %d", i, exp.attempt, pairs[i].Key.Attempt)
		}
	}

	// First pair should have both Direct and Gumi.
	if !pairs[0].DirectOK || !pairs[0].GumiOK {
		t.Error("expected pair 0 to have both Direct and Gumi")
	}
	if pairs[0].Direct.Passed != true {
		t.Error("expected direct passed=true for pair 0")
	}
	if pairs[0].Gumi.Passed != true {
		t.Error("expected gumi passed=true for pair 0")
	}
}

func TestPairResultsUnpairedCondition(t *testing.T) {
	// Only Direct results — Gumi should be nil.
	results := []GEPResult{
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionDirect, Passed: true},
	}
	pairs, err := PairResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if !pairs[0].DirectOK {
		t.Error("expected Direct to be present")
	}
	if pairs[0].GumiOK {
		t.Error("expected Gumi to be nil for unpaired result")
	}
}

func TestPairResultsDuplicateConditionErrors(t *testing.T) {
	// Duplicate results for the same (SuiteID, TestID, Attempt, Condition)
	// must return an explicit error, not silently overwrite.
	results := []GEPResult{
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionDirect, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionDirect, Passed: false, Subscores: map[string]float64{"a": 0.0}},
	}
	_, err := PairResults(results)
	if err == nil {
		t.Fatal("expected error for duplicate Direct result with same key")
	}
}

func TestPairResultsSameKeyDifferentConditionsOK(t *testing.T) {
	// Same key but different conditions is valid — Direct and Gumi should pair.
	results := []GEPResult{
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionDirect, Passed: true},
		{SuiteID: "s1", TestID: "t1", Attempt: 1, Condition: ConditionGumiStabilized, Passed: false},
	}
	pairs, err := PairResults(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if !pairs[0].DirectOK || !pairs[0].GumiOK {
		t.Error("expected both Direct and Gumi for same key with different conditions")
	}
}
