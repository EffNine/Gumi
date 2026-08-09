// Package scorer contains GEP scorer tests.
package scorer

import (
	"testing"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

func TestCheckEQString(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-01",
		Constraints: []types.GEPConstraint{
			{Field: "answer", Operator: "eq", Value: "hello"},
		},
	}, "hello")
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Error)
	}
	if result.Subscores["answer"] != 1.0 {
		t.Errorf("expected subscore 1.0, got %f", result.Subscores["answer"])
	}
}

func TestCheckEQStringFail(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-02",
		Constraints: []types.GEPConstraint{
			{Field: "answer", Operator: "eq", Value: "hello"},
		},
	}, "wrong")
	if result.Passed {
		t.Error("expected fail, got pass")
	}
}

func TestCheckJSONValid(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-03",
		Constraints: []types.GEPConstraint{
			{Field: "json", Operator: "valid", Value: nil},
		},
	}, `{"name": "test", "value": 42}`)
	if !result.Passed {
		t.Errorf("expected valid JSON, got fail: %s", result.Error)
	}
}

func TestCheckJSONInvalid(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-04",
		Constraints: []types.GEPConstraint{
			{Field: "json", Operator: "valid", Value: nil},
		},
	}, `{"name": "test", invalid}`)
	if result.Passed {
		t.Error("expected invalid JSON to fail")
	}
}

func TestCheckNoMarkdown(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-05",
		Constraints: []types.GEPConstraint{
			{Field: "no_markdown", Operator: "eq", Value: true},
		},
	}, "no markdown here")
	if !result.Passed {
		t.Errorf("expected no markdown to pass, got fail: %s", result.Error)
	}
}

func TestCheckNoMarkdownFail(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-06",
		Constraints: []types.GEPConstraint{
			{Field: "no_markdown", Operator: "eq", Value: true},
		},
	}, "here is ```code```")
	if result.Passed {
		t.Error("expected markdown to fail")
	}
}

func TestCheckNotContains(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-07",
		Constraints: []types.GEPConstraint{
			{Field: "forbidden", Operator: "not_contains", Value: []interface{}{"bad", "word"}},
		},
	}, "this is a good response")
	if !result.Passed {
		t.Errorf("expected no forbidden words, got fail: %s", result.Error)
	}
}

func TestCheckNotContainsFail(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-08",
		Constraints: []types.GEPConstraint{
			{Field: "forbidden", Operator: "not_contains", Value: []interface{}{"bad"}},
		},
	}, "this has a BAD word")
	if result.Passed {
		t.Error("expected forbidden word to fail")
	}
}

func TestCheckNumericCorrect(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-09",
		Constraints: []types.GEPConstraint{
			{Field: "answer", Operator: "eq", Value: 42},
		},
	}, "The answer is 42")
	if !result.Passed {
		t.Errorf("expected numeric match, got fail: %s", result.Error)
	}
}

func TestScoreSelfConsistency(t *testing.T) {
	responses := []string{"42", "42", "42"}
	score := ScoreSelfConsistency(responses)
	if score != 1.0 {
		t.Errorf("expected consistency score 1.0, got %f", score)
	}
}

func TestScoreSelfConsistencyFail(t *testing.T) {
	responses := []string{"42", "43", "42"}
	score := ScoreSelfConsistency(responses)
	if score == 1.0 {
		t.Error("expected inconsistent responses to score 0")
	}
}

func TestCheckEndsWith(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-10",
		Constraints: []types.GEPConstraint{
			{Field: "ends", Operator: "ends_with", Value: "Paris."},
		},
	}, "The capital is Paris.")
	if !result.Passed {
		t.Errorf("expected end match, got fail: %s", result.Error)
	}
}

func TestCheckEndsWithFail(t *testing.T) {
	s := New()
	result := s.Score(types.GEPTest{
		ID: "test-11",
		Constraints: []types.GEPConstraint{
			{Field: "ends", Operator: "ends_with", Value: "Paris."},
		},
	}, "The capital is London.")
	if result.Passed {
		t.Error("expected end mismatch to fail")
	}
}
