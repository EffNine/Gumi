package scorer

import (
	"testing"

	"github.com/novexa/novexa/benchmark"
)

func TestScore_AllPass(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "test1",
		Constraints: []benchmark.Constraint{
			{Field: "valid_json", Operator: "valid", Value: nil},
			{Field: "has_key", Operator: "superset", Value: []interface{}{"a"}},
		},
	}
	response := `{"a": 1}`
	result := s.Score(test, response)
	if !result.Passed {
		t.Errorf("Score: Passed=%v, want true. Error: %s", result.Passed, result.Error)
	}
	if len(result.Subscores) != 2 {
		t.Errorf("Score: expected 2 subscores, got %d", len(result.Subscores))
	}
	for field, score := range result.Subscores {
		if score != 1.0 {
			t.Errorf("Score: subscore[%q]=%v, want 1.0", field, score)
		}
	}
}

func TestScore_OneFails(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "test2",
		Constraints: []benchmark.Constraint{
			{Field: "valid_json", Operator: "valid", Value: nil},
			{Field: "has_key", Operator: "superset", Value: []interface{}{"a", "b"}},
		},
	}
	response := `{"a": 1}` // missing key "b"
	result := s.Score(test, response)
	if result.Passed {
		t.Errorf("Score: Passed=%v, want false (missing key 'b')", result.Passed)
	}
	if result.Subscores["valid_json"] != 1.0 {
		t.Errorf("Score: valid_json subscore=%v, want 1.0", result.Subscores["valid_json"])
	}
	if result.Subscores["has_key"] != 0.0 {
		t.Errorf("Score: has_key subscore=%v, want 0.0", result.Subscores["has_key"])
	}
}

func TestScore_EmptyConstraints(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID:          "test3",
		Constraints: nil,
	}
	result := s.Score(test, "any response")
	if !result.Passed {
		t.Errorf("Score with no constraints: Passed=%v, want true", result.Passed)
	}
	if len(result.Subscores) != 0 {
		t.Errorf("Score with no constraints: expected 0 subscores, got %d", len(result.Subscores))
	}
}

func TestScore_UnknownOperator(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "test4",
		Constraints: []benchmark.Constraint{
			{Field: "custom", Operator: "unknown_op", Value: "x"},
		},
	}
	result := s.Score(test, "response")
	if result.Passed {
		t.Errorf("Score with unknown operator: Passed=%v, want false", result.Passed)
	}
	if result.Subscores["custom"] != 0.0 {
		t.Errorf("Score with unknown operator: subscore=%v, want 0.0", result.Subscores["custom"])
	}
}

func TestScore_AllOperators(t *testing.T) {
	s := New()
	// Text-based operators: use a response that passes all
	test := benchmark.SuiteTest{
		ID: "test5",
		Constraints: []benchmark.Constraint{
			{Field: "eq_check", Operator: "eq", Value: 42},
			{Field: "gte_check", Operator: "gte", Value: 10},
			{Field: "lte_check", Operator: "lte", Value: 100},
			{Field: "starts_with_check", Operator: "starts_with", Value: "The"},
			{Field: "ends_with_check", Operator: "ends_with", Value: "42"},
			{Field: "no_markdown_check", Operator: "no_markdown", Value: nil},
			{Field: "no_commas_check", Operator: "no_commas", Value: nil},
			{Field: "not_contains_check", Operator: "not_contains", Value: "bad"},
		},
	}
	response := "The answer is 42"
	result := s.Score(test, response)
	if !result.Passed {
		t.Errorf("Score with text operators: Passed=%v, want true. Error: %s", result.Passed, result.Error)
	}
	if len(result.Subscores) != 8 {
		t.Errorf("Score: expected 8 subscores, got %d", len(result.Subscores))
	}
}

func TestScore_JSONOperators(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "test6",
		Constraints: []benchmark.Constraint{
			{Field: "valid_check", Operator: "valid", Value: nil},
			{Field: "superset_check", Operator: "superset", Value: []interface{}{"answer"}},
		},
	}
	response := `{"answer": 42}`
	result := s.Score(test, response)
	if !result.Passed {
		t.Errorf("Score with JSON operators: Passed=%v, want true. Error: %s", result.Passed, result.Error)
	}
	if len(result.Subscores) != 2 {
		t.Errorf("Score: expected 2 subscores, got %d", len(result.Subscores))
	}
	for field, score := range result.Subscores {
		if score != 1.0 {
			t.Errorf("Score: subscore[%q]=%v, want 1.0", field, score)
		}
	}
}

// ---------------------------------------------------------------------------
// AggregateCapabilities
// ---------------------------------------------------------------------------

func TestAggregateCapabilities_GroupsCorrectly(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "json-1", Condition: "direct", Subscores: map[string]float64{"valid": 1.0}},
		{TestID: "json-2", Condition: "direct", Subscores: map[string]float64{"valid": 0.5}},
		{TestID: "json-1", Condition: "novexa-stabilized", Subscores: map[string]float64{"valid": 1.0}},
		{TestID: "instruction-1", Condition: "direct", Subscores: map[string]float64{"follow": 0.8}},
	}
	categories := map[string]string{
		"json-1":          "json",
		"json-2":          "json",
		"instruction-1":   "instruction",
	}
	caps := AggregateCapabilities(results, categories)
	if len(caps) != 2 {
		t.Errorf("AggregateCapabilities: expected 2 capabilities, got %d", len(caps))
	}
	// Check json capability
	jsonCap, ok := caps["json"]
	if !ok {
		t.Fatal("AggregateCapabilities: missing 'json' capability")
	}
	if jsonCap.Direct.N != 2 {
		t.Errorf("json direct N=%d, want 2", jsonCap.Direct.N)
	}
	if jsonCap.Novexa.N != 1 {
		t.Errorf("json novexa N=%d, want 1", jsonCap.Novexa.N)
	}
	// Delta should be positive (novexa mean = 1.0, direct mean = 0.75)
	if jsonCap.Delta <= 0 {
		t.Errorf("json delta=%v, want > 0", jsonCap.Delta)
	}
}

func TestAggregateCapabilities_SkipsDegradation(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "deg-1", Condition: "direct", Subscores: map[string]float64{"a": 1.0}},
		{TestID: "deg-1", Condition: "novexa-stabilized", Subscores: map[string]float64{"a": 1.0}},
	}
	categories := map[string]string{
		"deg-1": "degradation",
	}
	caps := AggregateCapabilities(results, categories)
	if len(caps) != 0 {
		t.Errorf("AggregateCapabilities with only degradation: expected 0 caps, got %d", len(caps))
	}
}

func TestAggregateCapabilities_EmptyResults(t *testing.T) {
	caps := AggregateCapabilities(nil, nil)
	if caps == nil {
		t.Errorf("AggregateCapabilities(nil) should return empty map, not nil")
	}
}

func TestAggregateCapabilities_FrontierIsNovexa(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "json-1", Condition: "direct", Subscores: map[string]float64{"a": 0.5}},
		{TestID: "json-1", Condition: "frontier", Subscores: map[string]float64{"a": 1.0}},
	}
	categories := map[string]string{
		"json-1": "json",
	}
	caps := AggregateCapabilities(results, categories)
	jsonCap, ok := caps["json"]
	if !ok {
		t.Fatal("AggregateCapabilities: missing 'json' capability")
	}
	if jsonCap.Novexa.N != 0 {
		t.Errorf("json novexa N=%d, want 0 (frontier no longer grouped as novexa)", jsonCap.Novexa.N)
	}
}

func TestAggregateCapabilities_NoDirectResults(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "json-1", Condition: "novexa-stabilized", Subscores: map[string]float64{"a": 1.0}},
	}
	categories := map[string]string{
		"json-1": "json",
	}
	caps := AggregateCapabilities(results, categories)
	jsonCap, ok := caps["json"]
	if !ok {
		t.Fatal("AggregateCapabilities: missing 'json' capability")
	}
	if jsonCap.Direct.N != 0 {
		t.Errorf("json direct N=%d, want 0 (no direct results)", jsonCap.Direct.N)
	}
	if !isZeroMetricSet(jsonCap.Direct) {
		t.Errorf("json direct should be zero MetricSet, got %+v", jsonCap.Direct)
	}
}

func isZeroMetricSet(ms MetricSet) bool {
	return ms.Mean == 0 && ms.Std == 0 && ms.N == 0
}
