package scorer

import (
	"math"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func eq(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["eq"](response, benchmark.Constraint{Value: value, Field: ""})
}

func gte(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["gte"](response, benchmark.Constraint{Value: value})
}

func lte(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["lte"](response, benchmark.Constraint{Value: value})
}

func valid(t *testing.T, response string) CheckResult {
	t.Helper()
	return CheckRegistry["valid"](response, benchmark.Constraint{})
}

func superset(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["superset"](response, benchmark.Constraint{Value: value})
}

func notContains(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["not_contains"](response, benchmark.Constraint{Value: value})
}

func startsWith(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["starts_with"](response, benchmark.Constraint{Value: value})
}

func endsWith(t *testing.T, response string, value interface{}) CheckResult {
	t.Helper()
	return CheckRegistry["ends_with"](response, benchmark.Constraint{Value: value})
}

func noMarkdown(t *testing.T, response string) CheckResult {
	t.Helper()
	return CheckRegistry["no_markdown"](response, benchmark.Constraint{})
}

func noCommas(t *testing.T, response string) CheckResult {
	t.Helper()
	return CheckRegistry["no_commas"](response, benchmark.Constraint{})
}

// ---------------------------------------------------------------------------
// checkEQ
// ---------------------------------------------------------------------------

func TestCheckEQ_Numeric(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"42" eq 42`, "42", 42, true},
		{`"42" eq 42 (int)`, "42", int(42), true},
		{`"not a number" eq 42 → fail`, "not a number", 42, false},
		{`"30" eq 50 → fail`, "30", 50, false},
		{`"" eq 42 → fail`, "", 42, false},
		{`"  50  " eq 50 → true`, "  50  ", 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eq(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkEQ(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

func TestCheckEQ_String(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"hello" eq "hello"`, "hello", "hello", true},
	{`"hello world" contains "hello" → false (strict eq)`, "hello world", "hello", false},
		{`"hi" eq "hello" → false`, "hi", "hello", false},
		{`"Paris" eq "paris" → false (case-sensitive)`, "Paris", "paris", false},
		{`exact match with trailing spaces`, "  hello  ", "hello", true},
		{`substring with case difference → false`, "Hello World", "world", false},
		{`"World" eq "Hello World" → false (strict eq)`, "Hello World", "World", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eq(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkEQ(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

func TestCheckEQ_BoolTrue(t *testing.T) {
	tests := []struct {
		name     string
		response string
		field    string
		wantPass bool
	}{
		{"capital_start uppercase first letter", "Hello", "capital_start", true},
		{"capital_start lowercase first letter", "hello", "capital_start", false},
		{"capital_start whitespace then uppercase", " Hello", "capital_start", true},
		{"capital_start empty", "", "capital_start", false},
		{"no_markdown plain text", "plain text", "no_markdown", true},
		{"no_markdown with fences", "```json\n{}```", "no_markdown", false},
		{"no_commas plain", "no commas here", "no_commas", true},
		{"no_commas with commas", "has, commas", "no_commas", false},
		{"unknown field defaults to true", "anything", "unknown_field", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckRegistry["eq"](tt.response, benchmark.Constraint{Field: tt.field, Value: true})
			if got.Passed != tt.wantPass {
				t.Errorf("checkEQ with field=%q, value=true: got Passed=%v, want %v. Details: %s",
					tt.field, got.Passed, tt.wantPass, got.Details)
			}
		})
	}
}

func TestCheckEQ_UnsupportedType(t *testing.T) {
	// Unsupported types should fail.
	got := CheckRegistry["eq"]("foo", benchmark.Constraint{Value: []string{"a", "b"}})
	if got.Passed {
		t.Errorf("unsupported type should fail, got Passed=%v", got.Passed)
	}
}

// ---------------------------------------------------------------------------
// checkGTE
// ---------------------------------------------------------------------------

func TestCheckGTE(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"100" gte 50`, "100", 50, true},
		{`"30" gte 50`, "30", 50, false},
		{`"50" gte 50`, "50", 50, true},
		{`"abc" gte 50 (no number)`, "abc", 50, false},
		{`" 99 " gte 50`, " 99 ", 50, true},
		{`"the answer is 42" gte 40`, "the answer is 42", 40, true},
		{`"the answer is 30" gte 40`, "the answer is 30", 40, false},
		{`value as string "50"`, "100", "50", true},
		{`unparsable value fails`, "100", "abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gte(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkGTE(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkLTE
// ---------------------------------------------------------------------------

func TestCheckLTE(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"30" lte 50`, "30", 50, true},
		{`"100" lte 50`, "100", 50, false},
		{`"50" lte 50`, "50", 50, true},
		{`"abc" lte 50 (no number)`, "abc", 50, false},
		{`"the answer is 30" lte 50`, "the answer is 30", 50, true},
		{`value as string "50"`, "30", "50", true},
		{`unparsable value fails`, "30", "abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lte(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkLTE(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkValid (JSON)
// ---------------------------------------------------------------------------

func TestCheckValid(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantPass bool
	}{
		{"valid JSON object", `{"a":1}`, true},
		{"valid JSON array", `[1,2,3]`, true},
		{"invalid JSON", "not json", false},
		{"valid JSON in code fence", "```json\n{\"a\":1}\n```", true},
		{"valid JSON in code fence without lang", "```\n{\"a\":1}\n```", true},
		{"empty code fence", "```\n```", false},
		{"empty string", "", false},
		{"valid JSON number", "42", true},
		{"valid JSON string", `"hello"`, true},
		{"whitespace then valid JSON", `  {"b":2}  `, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valid(t, tt.response)
			if got.Passed != tt.wantPass {
				t.Errorf("checkValid(%q) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkSuperset
// ---------------------------------------------------------------------------

func TestCheckSuperset(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{"all keys present", `{"a":1,"b":2}`, []interface{}{"a", "b"}, true},
		{"missing key", `{"a":1}`, []interface{}{"a", "b"}, false},
		{"not JSON", "not json", []interface{}{"a"}, false},
		{"empty keys list", `{"a":1}`, []interface{}{}, true},
		{"single key as string", `{"a":1}`, "a", true},
		{"extra keys allowed", `{"a":1,"b":2,"c":3}`, []interface{}{"a"}, true},
		{"keys from code fence", "```\n{\"a\":1,\"b\":2}\n```", []interface{}{"a", "b"}, true},
		{"empty response", "", []interface{}{"a"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := superset(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkSuperset(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkNotContains
// ---------------------------------------------------------------------------

func TestCheckNotContains(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"hello world" not_contains "goodbye"`, "hello world", "goodbye", true},
		{`"hello world" not_contains "world" → fail`, "hello world", "world", false},
		{`"Hello World" not_contains "hello" (case-insensitive)`, "Hello World", "hello", false},
		{`no forbidden words`, "clean text", []interface{}{"bad", "ugly"}, true},
		{`one forbidden word`, "this has bad stuff", []interface{}{"bad", "ugly"}, false},
		{`empty forbidden list`, "anything", []interface{}{}, true},
		{`empty response`, "", "something", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notContains(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkNotContains(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkStartsWith
// ---------------------------------------------------------------------------

func TestCheckStartsWith(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"Hello" starts_with "H"`, "Hello", "H", true},
		{`"hello" starts_with "H"`, "hello", "H", false},
		{`" hello" starts_with "hello" (whitespace trimmed → prefix matches)`, " hello", "hello", true},
		{`"Hello" starts_with "He"`, "Hello", "He", true},
		{`"Hello World" starts_with "Hello"`, "Hello World", "Hello", true},
		{`response with leading space trimmed then matches`, "  Hello", "Hello", true},
		{`empty expected prefix`, "anything", "", true},
		{`"Yes" starts_with "Yes"`, "Yes", "Yes", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startsWith(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkStartsWith(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkEndsWith
// ---------------------------------------------------------------------------

func TestCheckEndsWith(t *testing.T) {
	tests := []struct {
		name     string
		response string
		value    interface{}
		wantPass bool
	}{
		{`"complete" ends_with "complete"`, "complete", "complete", true},
		{`"end with complete" ends_with "complete"`, "end with complete", "complete", true},
		{`"not ending" ends_with "complete"`, "not ending", "complete", false},
		{`"hello world" ends_with "world"`, "hello world", "world", true},
		{`response with trailing space (trimmed then matches)`, "complete  ", "complete", true},
		{`empty expected suffix`, "anything", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := endsWith(t, tt.response, tt.value)
			if got.Passed != tt.wantPass {
				t.Errorf("checkEndsWith(%q, %v) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, tt.value, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkNoMarkdown
// ---------------------------------------------------------------------------

func TestCheckNoMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantPass bool
	}{
		{"plain text", "plain text", true},
		{"with code fence", "```json\n...\n```", false},
		{"with inline backticks", "`code`", true}, // single backticks OK
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := noMarkdown(t, tt.response)
			if got.Passed != tt.wantPass {
				t.Errorf("checkNoMarkdown(%q) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkNoCommas
// ---------------------------------------------------------------------------

func TestCheckNoCommas(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantPass bool
	}{
		{"no commas", "no commas here", true},
		{"has commas", "has, commas", false},
		{"many commas", "a,b,c,d", false},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := noCommas(t, tt.response)
			if got.Passed != tt.wantPass {
				t.Errorf("checkNoCommas(%q) = {Passed: %v, Details: %q}; want Passed=%v",
					tt.response, got.Passed, got.Details, tt.wantPass)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers: extractNumber, toFloat64, toStringSlice, condStr
// ---------------------------------------------------------------------------

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		input string
		want  *float64
	}{
		{"42", ptr(42.0)},
		{"3.14", ptr(3.14)},
		{"  -7  ", ptr(-7.0)},
		{"abc", nil},
		{"", nil},
		{"The answer is 42", ptr(42.0)},
		{"price: 19.99 dollars", ptr(19.99)},
		{"no digits here!", nil},
		{"   ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractNumber(tt.input)
			if tt.want == nil && got != nil {
				t.Errorf("extractNumber(%q) = %v, want nil", tt.input, *got)
			} else if tt.want != nil && got == nil {
				t.Errorf("extractNumber(%q) = nil, want %v", tt.input, *tt.want)
			} else if tt.want != nil && got != nil {
				if math.Abs(*got-*tt.want) > 1e-9 {
					t.Errorf("extractNumber(%q) = %v, want %v", tt.input, *got, *tt.want)
				}
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input interface{}
		want  float64
		ok    bool
	}{
		{42, 42, true},
		{3.14, 3.14, true},
		{"100", 100, true},
		{"abc", 0, false},
		{nil, 0, false},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.ok || (ok && math.Abs(got-tt.want) > 1e-9) {
				t.Errorf("toFloat64(%v) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{"string", "hello", []string{"hello"}},
		{"slice", []interface{}{"a", "b"}, []string{"a", "b"}},
		{"int", 42, []string{"42"}},
		{"float", 3.14, []string{"3.14"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toStringSlice(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("toStringSlice(%v) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("toStringSlice(%v) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestCondStr(t *testing.T) {
	if got := condStr(true, "yes", "no"); got != "yes" {
		t.Errorf("condStr(true) = %q, want %q", got, "yes")
	}
	if got := condStr(false, "yes", "no"); got != "no" {
		t.Errorf("condStr(false) = %q, want %q", got, "no")
	}
}

func ptr(v float64) *float64 {
	return &v
}
