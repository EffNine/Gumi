package scorer

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestDegradationDetector_Identical(t *testing.T) {
	d := NewDegradationDetector()
	rec := d.Compare("hello world", "hello world", benchmark.SuiteTest{ID: "test1"})
	if rec.Severity != "" {
		t.Errorf("identical strings: severity=%q, want empty", rec.Severity)
	}
}

func TestDegradationDetector_WhitespaceOnly(t *testing.T) {
	d := NewDegradationDetector()
	rec := d.Compare("hello world", "  hello   world  ", benchmark.SuiteTest{ID: "test2"})
	if rec.Severity != "cosmetic" {
		t.Errorf("whitespace only: severity=%q, want 'cosmetic'", rec.Severity)
	}
}

func TestDegradationDetector_NumberChange(t *testing.T) {
	d := NewDegradationDetector()
	rec := d.Compare("The value is 42", "The value is 99", benchmark.SuiteTest{ID: "test3"})
	if rec.Severity != "semantic" {
		t.Errorf("number change: severity=%q, want 'semantic'. Detail: %s", rec.Severity, rec.Detail)
	}
}

func TestDegradationDetector_JSONKeyChange(t *testing.T) {
	d := NewDegradationDetector()
	// Actual JSON key name change (not just value change)
	original := `{"name": "Alice"}`
	repaired := `{"full_name": "Alice"}`
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test4"})
	if rec.Severity != "semantic" {
		t.Errorf("JSON key change: severity=%q, want 'semantic'. Detail: %s", rec.Severity, rec.Detail)
	}
}

func TestDegradationDetector_FormattingChange(t *testing.T) {
	d := NewDegradationDetector()
	original := "Hello\nWorld"
	repaired := "Hello World"
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test5"})
	if rec.Severity != "cosmetic" {
		t.Errorf("formatting change: severity=%q, want 'cosmetic'. Detail: %s", rec.Severity, rec.Detail)
	}
}

func TestDegradationDetector_ValueChangeCosmetic(t *testing.T) {
	d := NewDegradationDetector()
	// Value changes (non-number, non-key) are cosmetic in current implementation
	original := `{"active": true}`
	repaired := `{"active": false}`
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test6"})
	if rec.Severity != "cosmetic" {
		t.Errorf("value change (no keys/numbers): severity=%q, want 'cosmetic'. Detail: %s", rec.Severity, rec.Detail)
	}
}

func TestDegradationDetector_Truncation(t *testing.T) {
	d := NewDegradationDetector()
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	original := string(long)
	repaired := string(long) + " extra"
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test7"})
	if rec.Severity != "cosmetic" {
		t.Errorf("long string change: severity=%q, want 'cosmetic'", rec.Severity)
	}
	// Check that the records are truncated to 200 chars
	if len(rec.Original) > 203 {
		t.Errorf("original should be truncated to ≤200 chars, got %d", len(rec.Original))
	}
}

func TestDegradationDetector_NewlineNormalization(t *testing.T) {
	d := NewDegradationDetector()
	// Different newline styles should be cosmetic
	original := "line1\nline2"
	repaired := "line1\r\nline2"
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test8"})
	if rec.Severity != "cosmetic" {
		t.Errorf("newline normalization: severity=%q, want 'cosmetic'. Detail: %s", rec.Severity, rec.Detail)
	}
}

func TestDegradationDetector_MultipleNumberChanges(t *testing.T) {
	d := NewDegradationDetector()
	original := "values: 10, 20, 30"
	repaired := "values: 11, 21, 31"
	rec := d.Compare(original, repaired, benchmark.SuiteTest{ID: "test9"})
	if rec.Severity != "semantic" {
		t.Errorf("multiple number changes: severity=%q, want 'semantic'", rec.Severity)
	}
}

func TestDegradationDetector_EmptyStrings(t *testing.T) {
	d := NewDegradationDetector()
	rec := d.Compare("", "", benchmark.SuiteTest{ID: "test10"})
	if rec.Severity != "" {
		t.Errorf("empty strings: severity=%q, want empty", rec.Severity)
	}

	// Empty vs non-empty with no semantic content is cosmetic
	rec2 := d.Compare("", "hello", benchmark.SuiteTest{ID: "test11"})
	if rec2.Severity != "cosmetic" {
		t.Errorf("empty vs non-empty (no numbers/keys): severity=%q, want 'cosmetic'", rec2.Severity)
	}

	// Empty vs number-containing should be semantic
	rec3 := d.Compare("", "value is 42", benchmark.SuiteTest{ID: "test12"})
	if rec3.Severity != "semantic" {
		t.Errorf("empty vs number: severity=%q, want 'semantic'", rec3.Severity)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello   world", "hello world"},
		{"  leading and trailing  ", "leading and trailing"},
		{"nochange", "nochange"},
		{"multi\nline\ttext", "multi line text"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := normalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("normalizeWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractNumbers(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"abc 42 def 3.14", []string{"42", "3.14"}},
		{"no numbers", nil},
		{"", nil},
		{"100 200 300", []string{"100", "200", "300"}},
		{"version 2.0.1", []string{"2.0", "1"}}, // "0" consumed by "2.0"
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := extractNumbers(tt.input)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("extractNumbers(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractJSONKeys(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{`{"a": 1, "b": 2}`, []string{"a", "b"}},
		{`{"nested": {"inner": 3}}`, []string{"nested", "inner"}},
		{"no json here", nil},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := extractJSONKeys(tt.input)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("extractJSONKeys(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "b"}, []string{"b", "a"}, false},
		{nil, nil, true},
		{[]string{}, []string{}, true},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := stringSliceEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("stringSliceEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly long", 12, "exactly long"},
		{"this is too long for the limit", 20, "this is too long for "},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if len(got) > tt.maxLen+3 {
				t.Errorf("truncate(%q, %d) length = %d, want ≤ %d", tt.input, tt.maxLen, len(got), tt.maxLen+3)
			}
			if len(tt.input) <= tt.maxLen && got != tt.input {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.input)
			}
		})
	}
}
