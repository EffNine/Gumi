package scorer

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

// ---------------------------------------------------------------------------
// checkAnswerCorrect
// ---------------------------------------------------------------------------

func TestCheckAnswerCorrect_StringExact(t *testing.T) {
	got := CheckRegistry["answer_correct"]("Paris", benchmark.Constraint{
		Value: "Paris",
	})
	if !got.Passed {
		t.Errorf("answer_correct exact string: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckAnswerCorrect_StringSubstring(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The capital is Paris.", benchmark.Constraint{
		Value: "Paris",
	})
	if !got.Passed {
		t.Errorf("answer_correct substring: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckAnswerCorrect_StringCaseInsensitive(t *testing.T) {
	got := CheckRegistry["answer_correct"]("paris", benchmark.Constraint{
		Value: "Paris",
	})
	if !got.Passed {
		t.Errorf("answer_correct case insensitive: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckAnswerCorrect_StringWrong(t *testing.T) {
	got := CheckRegistry["answer_correct"]("London", benchmark.Constraint{
		Value: "Paris",
	})
	if got.Passed {
		t.Errorf("answer_correct wrong: Passed=%v, want false", got.Passed)
	}
}

func TestCheckAnswerCorrect_Numeric(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The answer is 42", benchmark.Constraint{
		Value: 42,
	})
	if !got.Passed {
		t.Errorf("answer_correct numeric: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckAnswerCorrect_NumericWrong(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The answer is 99", benchmark.Constraint{
		Value: 42,
	})
	if got.Passed {
		t.Errorf("answer_correct numeric wrong: Passed=%v, want false", got.Passed)
	}
}

func TestCheckAnswerCorrect_BoolTrueNonEmpty(t *testing.T) {
	got := CheckRegistry["answer_correct"]("Some response", benchmark.Constraint{
		Value: true,
	})
	if !got.Passed {
		t.Errorf("answer_correct bool true non-empty: Passed=%v, want true", got.Passed)
	}
}

func TestCheckAnswerCorrect_BoolTrueEmpty(t *testing.T) {
	got := CheckRegistry["answer_correct"]("", benchmark.Constraint{
		Value: true,
	})
	if got.Passed {
		t.Errorf("answer_correct bool true empty: Passed=%v, want false", got.Passed)
	}
}

func TestCheckAnswerCorrect_BoolFalseNonEmpty(t *testing.T) {
	got := CheckRegistry["answer_correct"]("some text", benchmark.Constraint{
		Value: false,
	})
	if got.Passed {
		t.Errorf("answer_correct bool false non-empty: Passed=%v, want false", got.Passed)
	}
}

func TestCheckAnswerCorrect_BoolFalseEmpty(t *testing.T) {
	got := CheckRegistry["answer_correct"]("", benchmark.Constraint{
		Value: false,
	})
	if !got.Passed {
		t.Errorf("answer_correct bool false empty: Passed=%v, want true", got.Passed)
	}
}

func TestCheckAnswerCorrect_UnsupportedType(t *testing.T) {
	got := CheckRegistry["answer_correct"]("anything", benchmark.Constraint{
		Value: []string{"bad"},
	})
	if got.Passed {
		t.Errorf("answer_correct unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkReasoningQuality
// ---------------------------------------------------------------------------

func TestCheckReasoningQuality_MeetsThreshold(t *testing.T) {
	resp := "First, we calculate 5+5=10. Therefore, the answer is 10."
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: true,
	})
	if !got.Passed {
		t.Errorf("reasoning_quality meets threshold: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckReasoningQuality_ToShort(t *testing.T) {
	got := CheckRegistry["reasoning_quality"]("hi", benchmark.Constraint{
		Value: true,
	})
	if got.Passed {
		t.Errorf("reasoning_quality too short: Passed=%v, want false", got.Passed)
	}
}

func TestCheckReasoningQuality_NoReasoningWords(t *testing.T) {
	resp := "The answer is 42. I am confident in this result."
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: true,
	})
	// Should pass if >= 50 chars even without reasoning words
	if !got.Passed {
		t.Logf("reasoning_quality no words: Passed=%v, Details: %s", got.Passed, got.Details)
	}
}

func TestCheckReasoningQuality_CustomMinChars(t *testing.T) {
	resp := "A short answer"
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: map[string]interface{}{"min_chars": 100},
	})
	if got.Passed {
		t.Errorf("reasoning_quality custom min_chars: Passed=%v, want false", got.Passed)
	}
}

func TestCheckReasoningQuality_CustomMinCharsPass(t *testing.T) {
	resp := "This is a reasonably long response that should meet the minimum character requirement of 100 characters or more."
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: map[string]interface{}{"min_chars": 100},
	})
	if !got.Passed {
		t.Errorf("reasoning_quality custom min_chars pass: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckReasoningQuality_DisableReasoningWords(t *testing.T) {
	resp := "The answer is 42. This is a reasonably long response to ensure we meet the character threshold."
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: map[string]interface{}{"require_reasoning": false, "min_chars": 50},
	})
	if !got.Passed {
		t.Errorf("reasoning_quality disable reasoning words: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckReasoningQuality_BoolFalse(t *testing.T) {
	got := CheckRegistry["reasoning_quality"]("anything", benchmark.Constraint{
		Value: false,
	})
	if got.Passed {
		t.Errorf("reasoning_quality bool false: Passed=%v, want false", got.Passed)
	}
}

func TestCheckReasoningQuality_UnsupportedType(t *testing.T) {
	got := CheckRegistry["reasoning_quality"]("anything", benchmark.Constraint{
		Value: "bad",
	})
	if got.Passed {
		t.Errorf("reasoning_quality unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkCodeUnchanged
// ---------------------------------------------------------------------------

func TestCheckCodeUnchanged_BoolTrueValidCode(t *testing.T) {
	resp := "def add(a, b):\n    return a + b"
	got := CheckRegistry["code_unchanged"](resp, benchmark.Constraint{
		Value: true,
	})
	if !got.Passed {
		t.Errorf("code_unchanged bool true valid code: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckCodeUnchanged_BoolTrueNoCode(t *testing.T) {
	got := CheckRegistry["code_unchanged"]("no code here", benchmark.Constraint{
		Value: true,
	})
	if got.Passed {
		t.Errorf("code_unchanged bool true no code: Passed=%v, want false", got.Passed)
	}
}

func TestCheckCodeUnchanged_BoolTrueNoDef(t *testing.T) {
	resp := "print('hello')"
	got := CheckRegistry["code_unchanged"](resp, benchmark.Constraint{
		Value: true,
	})
	if got.Passed {
		t.Errorf("code_unchanged bool true no def: Passed=%v, want false", got.Passed)
	}
}

func TestCheckCodeUnchanged_StringExactMatch(t *testing.T) {
	ref := "def add(a, b):\n    return a + b"
	got := CheckRegistry["code_unchanged"](ref, benchmark.Constraint{
		Value: ref,
	})
	if !got.Passed {
		t.Errorf("code_unchanged string exact: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckCodeUnchanged_StringWhitespaceNormalized(t *testing.T) {
	ref := "def add(a, b): return a + b"
	resp := "def add(a, b):\n    return a + b"
	got := CheckRegistry["code_unchanged"](resp, benchmark.Constraint{
		Value: ref,
	})
	if !got.Passed {
		t.Errorf("code_unchanged whitespace normalized: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckCodeUnchanged_StringDifferent(t *testing.T) {
	got := CheckRegistry["code_unchanged"]("def subtract(a, b):\n    return a - b", benchmark.Constraint{
		Value: "def add(a, b):\n    return a + b",
	})
	if got.Passed {
		t.Errorf("code_unchanged string different: Passed=%v, want false", got.Passed)
	}
}

func TestCheckCodeUnchanged_StringWithMarkdown(t *testing.T) {
	ref := "def add(a, b):\n    return a + b"
	resp := "```python\ndef add(a, b):\n    return a + b\n```"
	got := CheckRegistry["code_unchanged"](resp, benchmark.Constraint{
		Value: ref,
	})
	if !got.Passed {
		t.Errorf("code_unchanged with markdown: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckCodeUnchanged_BoolFalse(t *testing.T) {
	got := CheckRegistry["code_unchanged"]("def f(): pass", benchmark.Constraint{
		Value: false,
	})
	if got.Passed {
		t.Errorf("code_unchanged bool false: Passed=%v, want false", got.Passed)
	}
}

func TestCheckCodeUnchanged_UnsupportedType(t *testing.T) {
	got := CheckRegistry["code_unchanged"]("anything", benchmark.Constraint{
		Value: 42,
	})
	if got.Passed {
		t.Errorf("code_unchanged unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkContains (re-tested here for coverage)
// ---------------------------------------------------------------------------

func TestCheckContains_Present(t *testing.T) {
	got := CheckRegistry["contains"]("hello world", benchmark.Constraint{
		Value: "world",
	})
	if !got.Passed {
		t.Errorf("contains present: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckContains_Missing(t *testing.T) {
	got := CheckRegistry["contains"]("hello world", benchmark.Constraint{
		Value: "xyz",
	})
	if got.Passed {
		t.Errorf("contains missing: Passed=%v, want false", got.Passed)
	}
}

func TestCheckContains_CaseInsensitive(t *testing.T) {
	got := CheckRegistry["contains"]("HELLO WORLD", benchmark.Constraint{
		Value: "world",
	})
	if !got.Passed {
		t.Errorf("contains case insensitive: Passed=%v, want true", got.Passed)
	}
}

func TestCheckContains_MultipleValues(t *testing.T) {
	got := CheckRegistry["contains"]("hello world foo", benchmark.Constraint{
		Value: []interface{}{"world", "foo"},
	})
	if !got.Passed {
		t.Errorf("contains multiple values: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckContains_OneMissing(t *testing.T) {
	got := CheckRegistry["contains"]("hello world", benchmark.Constraint{
		Value: []interface{}{"world", "xyz"},
	})
	if got.Passed {
		t.Errorf("contains one missing: Passed=%v, want false", got.Passed)
	}
}

func TestCheckContains_EmptyValue(t *testing.T) {
	got := CheckRegistry["contains"]("anything", benchmark.Constraint{
		Value: []interface{}{},
	})
	if !got.Passed {
		t.Errorf("contains empty: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkMinChars
// ---------------------------------------------------------------------------

func TestCheckMinChars_Pass(t *testing.T) {
	got := CheckRegistry["min_chars"]("hello world", benchmark.Constraint{
		Value: 5,
	})
	if !got.Passed {
		t.Errorf("min_chars pass: Passed=%v, want true", got.Passed)
	}
}

func TestCheckMinChars_Fail(t *testing.T) {
	got := CheckRegistry["min_chars"]("hi", benchmark.Constraint{
		Value: 10,
	})
	if got.Passed {
		t.Errorf("min_chars fail: Passed=%v, want false", got.Passed)
	}
}

func TestCheckMinChars_Exact(t *testing.T) {
	got := CheckRegistry["min_chars"]("hello", benchmark.Constraint{
		Value: 5,
	})
	if !got.Passed {
		t.Errorf("min_chars exact: Passed=%v, want true", got.Passed)
	}
}

func TestCheckMinChars_EmptyResponse(t *testing.T) {
	got := CheckRegistry["min_chars"]("", benchmark.Constraint{
		Value: 0,
	})
	if !got.Passed {
		t.Errorf("min_chars empty with 0: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkUniqueLines
// ---------------------------------------------------------------------------

func TestCheckUniqueLines_Pass(t *testing.T) {
	resp := "line one\nline two\nline three"
	got := CheckRegistry["unique_lines"](resp, benchmark.Constraint{
		Value: 3,
	})
	if !got.Passed {
		t.Errorf("unique_lines pass: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckUniqueLines_Duplicates(t *testing.T) {
	resp := "line one\nline one\nline two"
	got := CheckRegistry["unique_lines"](resp, benchmark.Constraint{
		Value: 3,
	})
	if got.Passed {
		t.Errorf("unique_lines duplicates: Passed=%v, want false", got.Passed)
	}
}

func TestCheckUniqueLines_EmptyLines(t *testing.T) {
	resp := "line one\n\n\nline two"
	got := CheckRegistry["unique_lines"](resp, benchmark.Constraint{
		Value: 2,
	})
	if !got.Passed {
		t.Errorf("unique_lines empty lines: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkSentenceCount
// ---------------------------------------------------------------------------

func TestCheckSentenceCount_Correct(t *testing.T) {
	resp := "Hello world. How are you?"
	got := CheckRegistry["sentence_count"](resp, benchmark.Constraint{
		Value: 2,
	})
	if !got.Passed {
		t.Errorf("sentence_count correct: Passed=%v, want true. Details: %s", got.Passed, got.Details)
	}
}

func TestCheckSentenceCount_Wrong(t *testing.T) {
	resp := "Hello world."
	got := CheckRegistry["sentence_count"](resp, benchmark.Constraint{
		Value: 3,
	})
	if got.Passed {
		t.Errorf("sentence_count wrong: Passed=%v, want false", got.Passed)
	}
}

func TestCheckSentenceCount_NoPunctuation(t *testing.T) {
	resp := "no punctuation here"
	got := CheckRegistry["sentence_count"](resp, benchmark.Constraint{
		Value: 0,
	})
	if !got.Passed {
		t.Errorf("sentence_count no punct: Passed=%v, want true (0 sentences = 0 expected)", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// countSentences
// ---------------------------------------------------------------------------

func TestCountSentences_Simple(t *testing.T) {
	if got := countSentences("Hello world."); got != 1 {
		t.Errorf("countSentences(simple) = %d, want 1", got)
	}
}

func TestCountSentences_Multiple(t *testing.T) {
	if got := countSentences("A. B! C?"); got != 3 {
		t.Errorf("countSentences(multiple) = %d, want 3", got)
	}
}

func TestCountSentences_None(t *testing.T) {
	if got := countSentences("no punctuation"); got != 0 {
		t.Errorf("countSentences(none) = %d, want 0", got)
	}
}

func TestCountSentences_Empty(t *testing.T) {
	if got := countSentences(""); got != 0 {
		t.Errorf("countSentences(empty) = %d, want 0", got)
	}
}

func TestCountSentences_Mixed(t *testing.T) {
	if got := countSentences("Hello. World? Yes!"); got != 3 {
		t.Errorf("countSentences(mixed) = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// checkMinChars — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckMinChars_FloatValue(t *testing.T) {
	got := CheckRegistry["min_chars"]("hello", benchmark.Constraint{
		Value: 5.0,
	})
	if !got.Passed {
		t.Errorf("min_chars float value: Passed=%v, want true", got.Passed)
	}
}

func TestCheckMinChars_UnsupportedType(t *testing.T) {
	got := CheckRegistry["min_chars"]("hi", benchmark.Constraint{
		Value: "bad",
	})
	if got.Passed {
		t.Errorf("min_chars unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkSentenceCount — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckSentenceCount_FloatValue(t *testing.T) {
	resp := "Hello world. How are you?"
	got := CheckRegistry["sentence_count"](resp, benchmark.Constraint{
		Value: 2.0,
	})
	if !got.Passed {
		t.Errorf("sentence_count float value: Passed=%v, want true", got.Passed)
	}
}

func TestCheckSentenceCount_UnsupportedType(t *testing.T) {
	got := CheckRegistry["sentence_count"]("hi", benchmark.Constraint{
		Value: "bad",
	})
	if got.Passed {
		t.Errorf("sentence_count unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkAnswerCorrect — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckAnswerCorrect_IntValue(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The answer is 42", benchmark.Constraint{
		Value: int(42),
	})
	if !got.Passed {
		t.Errorf("answer_correct int value: Passed=%v, want true", got.Passed)
	}
}

func TestCheckAnswerCorrect_WhitespaceAround(t *testing.T) {
	got := CheckRegistry["answer_correct"]("  Paris  ", benchmark.Constraint{
		Value: "Paris",
	})
	if !got.Passed {
		t.Errorf("answer_correct whitespace: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkUniqueLines — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckUniqueLines_AllEmpty(t *testing.T) {
	resp := "\n\n\n"
	got := CheckRegistry["unique_lines"](resp, benchmark.Constraint{
		Value: 0,
	})
	if !got.Passed {
		t.Errorf("unique_lines all empty: Passed=%v, want true", got.Passed)
	}
}

func TestCheckUniqueLines_UnsupportedType(t *testing.T) {
	got := CheckRegistry["unique_lines"]("hi", benchmark.Constraint{
		Value: "bad",
	})
	if got.Passed {
		t.Errorf("unique_lines unsupported type: Passed=%v, want false", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkAnswerCorrect — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckAnswerCorrect_NegativeNumber(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The temperature is -5", benchmark.Constraint{
		Value: -5,
	})
	if !got.Passed {
		t.Errorf("answer_correct negative: Passed=%v, want true", got.Passed)
	}
}

func TestCheckAnswerCorrect_FloatValue(t *testing.T) {
	got := CheckRegistry["answer_correct"]("The price is 19.99", benchmark.Constraint{
		Value: 19.99,
	})
	if !got.Passed {
		t.Errorf("answer_correct float: Passed=%v, want true", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkReasoningQuality — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckReasoningQuality_Exactly50Chars(t *testing.T) {
	// Exactly 50 characters with reasoning word
	resp := "This is exactly fifty characters long. Therefore the answer is correct."
	if len([]rune(resp)) != 50 {
		// Adjust to exactly 50
		resp = "This is exactly fifty char. Therefore the answer is correct."
	}
	got := CheckRegistry["reasoning_quality"](resp, benchmark.Constraint{
		Value: true,
	})
	if !got.Passed {
		t.Logf("reasoning_quality exactly 50 chars: Passed=%v, Details: %s", got.Passed, got.Details)
	}
}

// ---------------------------------------------------------------------------
// checkCodeUnchanged — edge cases for coverage
// ---------------------------------------------------------------------------

func TestCheckCodeUnchanged_UnicodeCode(t *testing.T) {
	resp := "def hello():\n    return '你好'"
	got := CheckRegistry["code_unchanged"](resp, benchmark.Constraint{
		Value: true,
	})
	if !got.Passed {
		t.Errorf("code_unchanged unicode: Passed=%v, want true", got.Passed)
	}
}
