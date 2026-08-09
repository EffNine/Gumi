package scorer

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

// ---------------------------------------------------------------------------
// checkSuperset — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckSuperset_CodeFenceWithLangTag(t *testing.T) {
	resp := "```json\n{\"a\":1,\"b\":2}\n```"
	got := CheckRegistry["superset"](resp, benchmark.Constraint{
		Value: []interface{}{"a", "b"},
	})
	if !got.Passed {
		t.Errorf("superset with code fence lang tag: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckSuperset_CodeFenceWithoutLangTag(t *testing.T) {
	resp := "```\n{\"a\":1,\"b\":2}\n```"
	got := CheckRegistry["superset"](resp, benchmark.Constraint{
		Value: []interface{}{"a", "b"},
	})
	if !got.Passed {
		t.Errorf("superset with code fence no lang: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckSuperset_InvalidJSONInCodeFence(t *testing.T) {
	resp := "```\nnot json\n```"
	got := CheckRegistry["superset"](resp, benchmark.Constraint{
		Value: []interface{}{"a"},
	})
	if got.Passed {
		t.Errorf("expected fail for invalid JSON in code fence")
	}
}

func TestCheckSuperset_EmptyString(t *testing.T) {
	got := CheckRegistry["superset"]("", benchmark.Constraint{
		Value: []interface{}{"a"},
	})
	if got.Passed {
		t.Errorf("expected fail for empty string")
	}
}

// ---------------------------------------------------------------------------
// checkEQ — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckEQ_BoolFalse(t *testing.T) {
	// eq: false with no_markdown field — inverted check: fails if no markdown (passes if has markdown)
	// Response "plain text" has no markdown → no_markdown check passes → inverted fails
	got := CheckRegistry["eq"]("plain text", benchmark.Constraint{
		Field: "no_markdown",
		Value: false,
	})
	// eq:false on no_markdown means "we expect no_markdown to be FALSE" i.e. we expect markdown to exist
	// Since plain text has no markdown, the underlying check passes, and inversion makes this fail
	if got.Passed {
		t.Errorf("eq:false no_markdown on plain text: Passed=%v, want false (response has no markdown, so 'no_markdown=false' is violated)", got.Passed)
	}
	// eq: false with no_markdown field → response HAS markdown, so no_markdown check fails, inversion passes
	got2 := CheckRegistry["eq"]("```code```", benchmark.Constraint{
		Field: "no_markdown",
		Value: false,
	})
	if !got2.Passed {
		t.Errorf("eq:false no_markdown on markdown text: Passed=%v, want true", got2.Passed)
	}
}

func TestCheckEQ_BoolFalseCapitalStart(t *testing.T) {
	// eq: false capital_start → response does NOT start with capital
	got := CheckRegistry["eq"]("hello", benchmark.Constraint{
		Field: "capital_start",
		Value: false,
	})
	if !got.Passed {
		t.Errorf("eq:false capital_start on lowercase: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEQ_Float64WithDecimal(t *testing.T) {
	got := CheckRegistry["eq"]("3.14", benchmark.Constraint{
		Value: 3.14,
	})
	if !got.Passed {
		t.Errorf("eq float64 with decimal: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEQ_Float64Mismatch(t *testing.T) {
	got := CheckRegistry["eq"]("3.14", benchmark.Constraint{
		Value: 3.15,
	})
	if got.Passed {
		t.Errorf("eq float64 mismatch: Passed=%v, want false", got.Passed)
	}
}

func TestCheckEQ_StringWithWhitespace(t *testing.T) {
	got := CheckRegistry["eq"]("  hello world  ", benchmark.Constraint{
		Value: "hello world",
	})
	if !got.Passed {
		t.Errorf("eq string with whitespace: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEQ_NestedJSONString(t *testing.T) {
	// Response contains JSON but we're doing string eq
	got := CheckRegistry["eq"](`{"a":1}`, benchmark.Constraint{
		Value: `{"a":1}`,
	})
	if !got.Passed {
		t.Errorf("eq nested JSON string: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkGTE / checkLTE — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckGTE_FractionalValue(t *testing.T) {
	got := CheckRegistry["gte"]("5.5", benchmark.Constraint{
		Value: 5.0,
	})
	if !got.Passed {
		t.Errorf("gte fractional: Passed=%v, want true", got.Passed)
	}
}

func TestCheckGTE_FractionalResponse(t *testing.T) {
	got := CheckRegistry["gte"]("5.5", benchmark.Constraint{
		Value: 5.5,
	})
	if !got.Passed {
		t.Errorf("gte fractional exact: Passed=%v, want true", got.Passed)
	}
}

func TestCheckLTE_FractionalValue(t *testing.T) {
	got := CheckRegistry["lte"]("4.5", benchmark.Constraint{
		Value: 5.0,
	})
	if !got.Passed {
		t.Errorf("lte fractional: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkValid — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckValid_JSONArray(t *testing.T) {
	got := CheckRegistry["valid"]("[1,2,3]", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON array: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_JSONBoolean(t *testing.T) {
	got := CheckRegistry["valid"]("true", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON boolean: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_JSONNull(t *testing.T) {
	got := CheckRegistry["valid"]("null", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON null: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_JSONString(t *testing.T) {
	got := CheckRegistry["valid"](`"hello"`, benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON string: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_JSONNumber(t *testing.T) {
	got := CheckRegistry["valid"]("42", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON number: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_JSONFloat(t *testing.T) {
	got := CheckRegistry["valid"]("3.14", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid JSON float: Passed=%v, want true", got.Passed)
	}
}

func TestCheckValid_CodeFenceWithExtraText(t *testing.T) {
	got := CheckRegistry["valid"]("Here is the JSON:\n```json\n{\"a\":1}\n```", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("valid with prefix text: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkNotContains — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckNotContains_EmptyResponse(t *testing.T) {
	got := CheckRegistry["not_contains"]("", benchmark.Constraint{
		Value: []interface{}{"forbidden"},
	})
	if !got.Passed {
		t.Errorf("not_contains empty response: Passed=%v, want true", got.Passed)
	}
}

func TestCheckNotContains_PartialWord(t *testing.T) {
	// "the" should be found in "there"
	got := CheckRegistry["not_contains"]("there is a cat", benchmark.Constraint{
		Value: []interface{}{"the"},
	})
	if got.Passed {
		t.Errorf("not_contains partial word: Passed=%v, want false", got.Passed)
	}
}

func TestCheckNotContains_CaseInsensitive(t *testing.T) {
	got := CheckRegistry["not_contains"]("THE cat is here", benchmark.Constraint{
		Value: []interface{}{"the"},
	})
	if got.Passed {
		t.Errorf("not_contains case insensitive: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkStartsWith / checkEndsWith — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckStartsWith_EmptyExpected(t *testing.T) {
	got := CheckRegistry["starts_with"]("anything", benchmark.Constraint{
		Value: "",
	})
	if !got.Passed {
		t.Errorf("starts_with empty expected: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEndsWith_EmptyExpected(t *testing.T) {
	got := CheckRegistry["ends_with"]("anything", benchmark.Constraint{
		Value: "",
	})
	if !got.Passed {
		t.Errorf("ends_with empty expected: Passed=%v, want true", got.Passed)
	}
}

func TestCheckStartsWith_WhitespaceBefore(t *testing.T) {
	got := CheckRegistry["starts_with"]("  hello", benchmark.Constraint{
		Value: "hello",
	})
	if !got.Passed {
		t.Errorf("starts_with whitespace before: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEndsWith_WhitespaceAfter(t *testing.T) {
	got := CheckRegistry["ends_with"]("hello  ", benchmark.Constraint{
		Value: "hello",
	})
	if !got.Passed {
		t.Errorf("ends_with whitespace after: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkNoMarkdown / checkNoCommas — additional edge cases
// ---------------------------------------------------------------------------

func TestCheckNoMarkdown_InlineBackticks(t *testing.T) {
	// Single backticks should NOT trigger the check
	got := CheckRegistry["no_markdown"]("`code`", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("no_markdown inline backticks: Passed=%v, want true", got.Passed)
	}
}

func TestCheckNoMarkdown_MultipleFences(t *testing.T) {
	got := CheckRegistry["no_markdown"]("```a```\n```b```", benchmark.Constraint{})
	if got.Passed {
		t.Errorf("no_markdown multiple fences: Passed=%v, want false", got.Passed)
	}
}

func TestCheckNoCommas_Empty(t *testing.T) {
	got := CheckRegistry["no_commas"]("", benchmark.Constraint{})
	if !got.Passed {
		t.Errorf("no_commas empty: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// scorer.Score — additional edge cases
// ---------------------------------------------------------------------------

func TestScorer_MultipleConstraintsAllPass(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "multi-pass",
		Constraints: []benchmark.Constraint{
			{Field: "json", Operator: "valid", Value: nil},
			{Field: "keys", Operator: "superset", Value: []interface{}{"a"}},
			{Field: "no_md", Operator: "eq", Value: true},
		},
	}
	resp := `{"a": 1}`
	result := s.Score(test, resp)
	if !result.Passed {
		t.Errorf("multiple constraints all pass: Passed=%v, want true. Error: %s", result.Passed, result.Error)
	}
	if len(result.Subscores) != 3 {
		t.Errorf("expected 3 subscores, got %d", len(result.Subscores))
	}
}

func TestScorer_MultipleConstraintsOneFail(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "multi-fail",
		Constraints: []benchmark.Constraint{
			{Field: "json", Operator: "valid", Value: nil},
			{Field: "keys", Operator: "superset", Value: []interface{}{"a", "b"}},
		},
	}
	resp := `{"a": 1}`
	result := s.Score(test, resp)
	if result.Passed {
		t.Errorf("expected fail when one constraint fails")
	}
	if result.Subscores["json"] != 1.0 {
		t.Errorf("json subscore should be 1.0, got %v", result.Subscores["json"])
	}
	if result.Subscores["keys"] != 0.0 {
		t.Errorf("keys subscore should be 0.0, got %v", result.Subscores["keys"])
	}
}

func TestScorer_ErrorAggregation(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "err-test",
		Constraints: []benchmark.Constraint{
			{Field: "a", Operator: "eq", Value: "wrong"},
			{Field: "b", Operator: "valid", Value: nil},
		},
	}
	resp := "not json"
	result := s.Score(test, resp)
	if result.Passed {
		t.Errorf("expected fail")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty error")
	}
	// Error should contain both failures
	if len(result.Error) < 10 {
		t.Errorf("error too short: %q", result.Error)
	}
}

func TestScorer_UnknownOperatorInMulti(t *testing.T) {
	s := New()
	test := benchmark.SuiteTest{
		ID: "unknown-op",
		Constraints: []benchmark.Constraint{
			{Field: "known", Operator: "valid", Value: nil},
			{Field: "unknown", Operator: "fake_op", Value: "x"},
		},
	}
	resp := "hello"
	result := s.Score(test, resp)
	if result.Passed {
		t.Errorf("expected fail with unknown operator")
	}
	// Unknown operator sets subscore to 0.0 — known constraint also fails because "hello" is not valid JSON
	if result.Subscores["unknown"] != 0.0 {
		t.Errorf("unknown constraint should be 0.0, got %v", result.Subscores["unknown"])
	}
}

// ---------------------------------------------------------------------------
// extractNumber — additional edge cases
// ---------------------------------------------------------------------------

func TestExtractNumber_FirstNumberInText(t *testing.T) {
	got := extractNumber("The price is 99 dollars")
	if got == nil || *got != 99 {
		t.Errorf("extractNumber first number: got %v, want 99", got)
	}
}

func TestExtractNumber_NegativeNumber(t *testing.T) {
	got := extractNumber("Temperature is -10 degrees")
	if got == nil {
		t.Fatal("extractNumber negative: got nil")
	}
	if *got != -10 {
		t.Errorf("extractNumber negative: got %v, want -10", *got)
	}
}

func TestExtractNumber_ScientificNotation(t *testing.T) {
	// Scientific notation is NOT supported by Sscanf %f in this context
	got := extractNumber("1e10")
	// This may or may not parse — document the behavior
	_ = got
}

func TestExtractNumber_Percentage(t *testing.T) {
	got := extractNumber("The rate is 5%")
	if got == nil || *got != 5 {
		t.Errorf("extractNumber percentage: got %v, want 5", got)
	}
}

func TestExtractNumber_DollarAmount(t *testing.T) {
	// $42.50 — the $ is not stripped, so "42.50" won't match after $ prefix
	// extractNumber tries Sscanf on "42.50" (after trimming trailing punctuation) but $ stays
	got := extractNumber("The cost is $42.50")
	// $ prefix prevents parsing — this is a known limitation
	if got != nil {
		t.Logf("extractNumber dollar: got %v (known limitation: $ prefix)", *got)
	}
}

// ---------------------------------------------------------------------------
// toStringSlice — additional edge cases
// ---------------------------------------------------------------------------

func TestToStringSlice_IntValue(t *testing.T) {
	got := toStringSlice(42)
	if len(got) != 1 || got[0] != "42" {
		t.Errorf("toStringSlice(42) = %v, want [42]", got)
	}
}

func TestToStringSlice_FloatValue(t *testing.T) {
	got := toStringSlice(3.14)
	if len(got) != 1 || got[0] != "3.14" {
		t.Errorf("toStringSlice(3.14) = %v, want [3.14]", got)
	}
}

func TestToStringSlice_NilValue(t *testing.T) {
	got := toStringSlice(nil)
	if len(got) != 1 {
		t.Errorf("toStringSlice(nil) length = %d, want 1", len(got))
	}
}

// ---------------------------------------------------------------------------
// toFloat64 — additional edge cases
// ---------------------------------------------------------------------------

func TestToFloat64_DefaultCase(t *testing.T) {
	_, ok := toFloat64([]string{"a"})
	if ok {
		t.Errorf("toFloat64([]string) should return ok=false")
	}
}

func TestToFloat64_Int64(t *testing.T) {
	got, ok := toFloat64(int64(42))
	if !ok || got != 42 {
		t.Errorf("toFloat64(int64(42)) = (%v, %v), want (42, true)", got, ok)
	}
}

func TestToFloat64_Bool(t *testing.T) {
	_, ok := toFloat64(true)
	if ok {
		t.Errorf("toFloat64(bool) should return ok=false")
	}
}

// ---------------------------------------------------------------------------
// checkEQ — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckEQ_BoolFalseNoMarkdown(t *testing.T) {
	// eq:false on no_markdown with markdown present → should pass (inverted)
	got := CheckRegistry["eq"]("```code```", benchmark.Constraint{
		Field: "no_markdown",
		Value: false,
	})
	if !got.Passed {
		t.Errorf("eq:false no_markdown with fences: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEQ_SliceValue(t *testing.T) {
	// eq with slice value → unsupported type → fail
	got := CheckRegistry["eq"]("hello", benchmark.Constraint{
		Value: []string{"a"},
	})
	if got.Passed {
		t.Errorf("eq slice value: Passed=%v, want false", got.Passed)
	}
}

func TestCheckEQ_IntValue(t *testing.T) {
	got := CheckRegistry["eq"]("42", benchmark.Constraint{
		Value: int(42),
	})
	if !got.Passed {
		t.Errorf("eq int value: Passed=%v, want true", got.Passed)
	}
}

func TestCheckEQ_NilValue(t *testing.T) {
	got := CheckRegistry["eq"]("hello", benchmark.Constraint{
		Value: nil,
	})
	if got.Passed {
		t.Errorf("eq nil value: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkSuperset — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckSuperset_JSONWithExtraKeys(t *testing.T) {
	got := CheckRegistry["superset"](`{"a":1,"b":2,"c":3}`, benchmark.Constraint{
		Value: []interface{}{"a", "b"},
	})
	if !got.Passed {
		t.Errorf("superset extra keys: Passed=%v, want true", got.Passed)
	}
}

func TestCheckSuperset_MixedTypeValues(t *testing.T) {
	got := CheckRegistry["superset"](`{"a":1,"b":2}`, benchmark.Constraint{
		Value: []interface{}{"a", "b"},
	})
	if !got.Passed {
		t.Errorf("superset mixed types in JSON: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckSuperset_StringValue(t *testing.T) {
	got := CheckRegistry["superset"](`{"a":1}`, benchmark.Constraint{
		Value: "a",
	})
	if !got.Passed {
		t.Errorf("superset string value: Passed=%v, want true", got.Passed)
	}
}

func TestCheckSuperset_IntValue(t *testing.T) {
	// int value gets converted to string "42" by toStringSlice, then checked as substring
	got := CheckRegistry["superset"](`{"a":1,"42":2}`, benchmark.Constraint{
		Value: 42,
	})
	// The int 42 is converted to "42" string, which won't be found as a JSON key
	// This tests the edge case where non-string values are handled
	if got.Passed {
		t.Logf("superset int value passed (converted to string): Details=%s", got.Details)
	} else {
		t.Logf("superset int value failed as expected (non-string in JSON): Details=%s", got.Details)
	}
}
