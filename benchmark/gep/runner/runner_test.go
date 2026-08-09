package runner

import (
	"testing"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"model/name", "model-name"},
		{"model@tag", "model-tag"},
		{"model name", "model-name"},
		{"model:name", "model-name"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMeanOf(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	mean := meanOf(values)
	expected := 3.0
	if mean != expected {
		t.Errorf("meanOf(%v) = %f, want %f", values, mean, expected)
	}
}

func TestMeanOfEmpty(t *testing.T) {
	mean := meanOf([]float64{})
	if mean != 0 {
		t.Errorf("meanOf([]) = %f, want 0", mean)
	}
}

func TestStdOf(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean := meanOf(values)
	std := stdOf(values, mean)
	expected := 2.0
	if std != expected {
		t.Errorf("stdOf(%v) = %f, want %f", values, std, expected)
	}
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	p50 := percentile(values, 0.5)
	if p50 != 3.0 {
		t.Errorf("percentile(0.5) = %f, want 3.0", p50)
	}
	p25 := percentile(values, 0.25)
	if p25 != 2.0 {
		t.Errorf("percentile(0.25) = %f, want 2.0", p25)
	}
}

func TestComputeSummaryEmpty(t *testing.T) {
	summary := computeSummary([]types.GEPResult{}, nil)
	if summary.TotalTests != 0 {
		t.Errorf("expected 0 tests, got %d", summary.TotalTests)
	}
}

func TestComputeSummaryNonEmpty(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", Passed: true, Condition: types.ConditionGumiStabilized, Subscores: map[string]float64{"a": 1.0}, LatencyMs: 100},
		{TestID: "t2", Passed: true, Condition: types.ConditionGumiStabilized, Subscores: map[string]float64{"b": 1.0}, LatencyMs: 200},
		{TestID: "t3", Passed: false, Condition: types.ConditionGumiStabilized, Subscores: map[string]float64{"c": 0.0}, LatencyMs: 150},
	}
	summary := computeSummary(results, []types.GEPCondition{types.ConditionGumiStabilized})
	if summary.TotalTests != 3 {
		t.Errorf("expected 3 tests, got %d", summary.TotalTests)
	}
	if summary.PassedTests != 2 {
		t.Errorf("expected 2 passed, got %d", summary.PassedTests)
	}
	if summary.PassRate < 0.66 || summary.PassRate > 0.68 {
		t.Errorf("expected pass rate ~0.67, got %f", summary.PassRate)
	}
}

func TestAggregateCapabilities(t *testing.T) {
	results := []types.GEPResult{
		{TestID: "t1", SuiteID: "instruction_following", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t2", SuiteID: "instruction_following", Condition: types.ConditionDirect, Passed: false, Subscores: map[string]float64{"b": 0.0}},
		{TestID: "t3", SuiteID: "instruction_following", Condition: types.ConditionGumiStabilized, Passed: true, Subscores: map[string]float64{"a": 1.0}},
		{TestID: "t4", SuiteID: "structured_output", Condition: types.ConditionDirect, Passed: true, Subscores: map[string]float64{"c": 1.0}},
	}
	caps := aggregateCapabilities(results)
	if len(caps) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(caps))
	}
	if caps["instruction_following"].Direct.N != 2 {
		t.Errorf("expected 2 direct results for instruction_following, got %d", caps["instruction_following"].Direct.N)
	}
	if caps["instruction_following"].Gumi.N != 1 {
		t.Errorf("expected 1 gumi result for instruction_following, got %d", caps["instruction_following"].Gumi.N)
	}
	if caps["structured_output"].Direct.N != 1 {
		t.Errorf("expected 1 direct result for structured_output, got %d", caps["structured_output"].Direct.N)
	}
}
