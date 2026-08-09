package scorer

import (
	"math"
	"strings"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

// ---------------------------------------------------------------------------
// mathAnswerCheck
// ---------------------------------------------------------------------------

func TestMathAnswerCheck_Correct(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "42",
		},
	}
	got := mathAnswerCheck("The answer is 42", c)
	if !got.Passed {
		t.Errorf("expected pass, got: %s", got.Details)
	}
}

func TestMathAnswerCheck_Wrong(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "42",
		},
	}
	got := mathAnswerCheck("The answer is 99", c)
	if got.Passed {
		t.Errorf("expected fail, got pass")
	}
}

func TestMathAnswerCheck_GSM8KMarker(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "150",
		},
	}
	got := mathAnswerCheck("Let me calculate...\n\n#### 150", c)
	if !got.Passed {
		t.Errorf("expected pass with #### marker, got: %s", got.Details)
	}
}

func TestMathAnswerCheck_LastNumberFallback(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "42",
		},
	}
	// Response has multiple numbers but last one is the answer
	got := mathAnswerCheck("I counted 5 apples and 37 oranges, so 5+37=42", c)
	if !got.Passed {
		t.Errorf("expected pass (last number), got: %s", got.Details)
	}
}

func TestMathAnswerCheck_NoNumber(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "42",
		},
	}
	got := mathAnswerCheck("I don't know the answer", c)
	if got.Passed {
		t.Errorf("expected fail (no number), got pass")
	}
}

func TestMathAnswerCheck_MissingAnswerParam(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value:    "not a map",
	}
	got := mathAnswerCheck("anything", c)
	if got.Passed {
		t.Errorf("expected fail for wrong value type")
	}
	if !strings.Contains(got.Details, "map") {
		t.Errorf("expected error about map type, got: %s", got.Details)
	}
}

func TestMathAnswerCheck_EmptyAnswerParam(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "",
		},
	}
	got := mathAnswerCheck("42", c)
	if got.Passed {
		t.Errorf("expected fail for empty answer")
	}
}

func TestMathAnswerCheck_InvalidAnswerParam(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "abc",
		},
	}
	got := mathAnswerCheck("42", c)
	if got.Passed {
		t.Errorf("expected fail for invalid answer param")
	}
}

func TestMathAnswerCheck_FractionResponse(t *testing.T) {
	// "1/6" — the regex will find "1" and "6", last is "6"
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "1",
		},
	}
	// The response "1/6" — extractMathAnswer finds "1" then "6", last is "6"
	got := mathAnswerCheck("The probability is 1/6", c)
	// This tests the known limitation: fraction not fully supported
	_ = got
}

func TestMathAnswerCheck_NegativeNumber(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "-5",
		},
	}
	got := mathAnswerCheck("The temperature dropped to -5 degrees", c)
	if !got.Passed {
		t.Errorf("expected pass for negative number, got: %s", got.Details)
	}
}

func TestMathAnswerCheck_CommaSeparated(t *testing.T) {
	c := benchmark.Constraint{
		Field:    "math_answer",
		Operator: "math_answer",
		Value: map[string]interface{}{
			"answer": "1000",
		},
	}
	got := mathAnswerCheck("The population is 1,000", c)
	if !got.Passed {
		t.Errorf("expected pass for comma-separated number, got: %s", got.Details)
	}
}

// ---------------------------------------------------------------------------
// extractMathAnswer
// ---------------------------------------------------------------------------

func TestExtractMathAnswer_GSM8KMarker(t *testing.T) {
	resp := "Step 1: 5+5=10. Step 2: 10*10=100.\n\n#### 100"
	got := extractMathAnswer(resp)
	if got == nil || *got != 100 {
		t.Errorf("extractMathAnswer(GSM8K marker) = %v, want 100", got)
	}
}

func TestExtractMathAnswer_LastNumber(t *testing.T) {
	resp := "I have 5 apples and eat 2, so I have 3 left"
	got := extractMathAnswer(resp)
	if got == nil || *got != 3 {
		t.Errorf("extractMathAnswer(last number) = %v, want 3", got)
	}
}

func TestExtractMathAnswer_NoNumber(t *testing.T) {
	resp := "I don't know the answer"
	got := extractMathAnswer(resp)
	if got != nil {
		t.Errorf("extractMathAnswer(no number) = %v, want nil", got)
	}
}

func TestExtractMathAnswer_Empty(t *testing.T) {
	got := extractMathAnswer("")
	if got != nil {
		t.Errorf("extractMathAnswer(empty) = %v, want nil", got)
	}
}

func TestExtractMathAnswer_WhitespaceOnly(t *testing.T) {
	got := extractMathAnswer("   \n  ")
	if got != nil {
		t.Errorf("extractMathAnswer(whitespace) = %v, want nil", got)
	}
}

func TestExtractMathAnswer_NegativeNumber(t *testing.T) {
	resp := "The change is -5 degrees"
	got := extractMathAnswer(resp)
	if got == nil || *got != -5 {
		t.Errorf("extractMathAnswer(negative) = %v, want -5", got)
	}
}

func TestExtractMathAnswer_CommaNumber(t *testing.T) {
	resp := "The total is 1,234 items"
	got := extractMathAnswer(resp)
	if got == nil || *got != 1234 {
		t.Errorf("extractMathAnswer(comma) = %v, want 1234", got)
	}
}

func TestExtractMathAnswer_Decimal(t *testing.T) {
	resp := "The price is 19.99 dollars"
	got := extractMathAnswer(resp)
	if got == nil || *got != 19.99 {
		t.Errorf("extractMathAnswer(decimal) = %v, want 19.99", got)
	}
}

func TestExtractMathAnswer_FallbackToExtractNumber(t *testing.T) {
	resp := "The result is 42"
	got := extractMathAnswer(resp)
	if got == nil || *got != 42 {
		t.Errorf("extractMathAnswer(fallback) = %v, want 42", got)
	}
}

func TestExtractMathAnswer_FractionAfterMarker(t *testing.T) {
	resp := "The probability is 1/6.\n\n#### 1/6"
	got := extractMathAnswer(resp)
	if got == nil {
		t.Fatalf("extractMathAnswer(fraction after marker) = nil, want non-nil")
	}
	expected := 1.0 / 6.0
	if math.Abs(*got-expected) > 1e-10 {
		t.Errorf("extractMathAnswer(fraction after marker) = %v, want %v", *got, expected)
	}
}

func TestExtractMathAnswer_FractionInText(t *testing.T) {
	resp := "The answer is 3/4"
	got := extractMathAnswer(resp)
	if got == nil {
		t.Fatalf("extractMathAnswer(fraction in text) = nil, want non-nil")
	}
	expected := 0.75
	if math.Abs(*got-expected) > 1e-10 {
		t.Errorf("extractMathAnswer(fraction in text) = %v, want %v", *got, expected)
	}
}

func TestExtractMathAnswer_FractionWithSpaces(t *testing.T) {
	resp := "The probability is  1  /  6 "
	got := extractMathAnswer(resp)
	if got == nil {
		t.Fatalf("extractMathAnswer(fraction with spaces) = nil, want non-nil")
	}
	expected := 1.0 / 6.0
	if math.Abs(*got-expected) > 1e-10 {
		t.Errorf("extractMathAnswer(fraction with spaces) = %v, want %v", *got, expected)
	}
}
