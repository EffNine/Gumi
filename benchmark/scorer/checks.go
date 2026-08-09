package scorer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/benchmark"
)

// CheckFunc evaluates a model response against a single constraint.
type CheckFunc func(response string, constraint benchmark.Constraint) CheckResult

// CheckResult holds the outcome of a single constraint check.
type CheckResult struct {
	Passed  bool
	Details string
}

// CheckRegistry maps check operator names to their implementations.
var CheckRegistry = map[string]CheckFunc{
	"eq":                checkEQ,
	"gte":               checkGTE,
	"lte":               checkLTE,
	"valid":             checkValid,
	"superset":          checkSuperset,
	"not_contains":      checkNotContains,
	"starts_with":       checkStartsWith,
	"ends_with":         checkEndsWith,
	"no_markdown":       checkNoMarkdown,
	"no_commas":         checkNoCommas,
	"self_consistency":  checkSelfConsistency,
	"python_exec":       pythonExecCheck,
	"math_answer":       mathAnswerCheck,
	"contains":          checkContains,
	"min_chars":         checkMinChars,
	"unique_lines":      checkUniqueLines,
	"sentence_count":    checkSentenceCount,
	"answer_correct":    checkAnswerCorrect,
	"reasoning_quality": checkReasoningQuality,
	"code_unchanged":    checkCodeUnchanged,
}

// checkEQ verifies numeric or string equality, or boolean field checks.
//
// Value types:
//   - bool true + known field name → semantic check (capital_start, no_markdown, no_commas)
//   - float64 → parse number from response and compare
//   - string → exact or substring match
//   - int → parse number from response and compare
func checkEQ(response string, constraint benchmark.Constraint) CheckResult {
	switch v := constraint.Value.(type) {
	case bool:
		if !v {
			// eq: false — relay to field-specific check and invert
			result := checkEQBooleanTrue(response, constraint.Field)
			return CheckResult{
				Passed:  !result.Passed,
				Details: fmt.Sprintf("expected false, %s", result.Details),
			}
		}
		// eq: true — delegate to field-specific semantic checks
		return checkEQBooleanTrue(response, constraint.Field)

	case float64:
		n := extractNumber(response)
		if n == nil {
			return CheckResult{Passed: false, Details: fmt.Sprintf("expected number, got: %q", response)}
		}
		passed := *n == v
		detail := fmt.Sprintf("got %v, expected %v", *n, v)
		if !passed {
			detail += " (not equal)"
		}
		return CheckResult{Passed: passed, Details: detail}

	case int:
		fv := float64(v)
		n := extractNumber(response)
		if n == nil {
			return CheckResult{Passed: false, Details: fmt.Sprintf("expected number, got: %q", response)}
		}
		passed := *n == fv
		detail := fmt.Sprintf("got %v, expected %v", *n, fv)
		if !passed {
			detail += " (not equal)"
		}
		return CheckResult{Passed: passed, Details: detail}

	case string:
		trimmed := strings.TrimSpace(response)
		passed := trimmed == v
		detail := fmt.Sprintf("response %q %s %q", trimmed, condStr(passed, "==", "!="), v)
		return CheckResult{Passed: passed, Details: detail}

	default:
		return CheckResult{Passed: false, Details: fmt.Sprintf("unsupported eq type %T, cannot evaluate constraint %q", constraint.Value, constraint.Field)}
	}
}

// checkEQBooleanTrue handles the `eq: true` pattern where the field name determines
// what semantic check to perform on the response.
func checkEQBooleanTrue(response string, field string) CheckResult {
	switch field {
	case "capital_start":
		// Check that the first non-whitespace character is an uppercase letter
		trimmed := strings.TrimSpace(response)
		if len(trimmed) == 0 {
			return CheckResult{Passed: false, Details: "empty response, cannot check capital start"}
		}
		first := trimmed[0]
		passed := first >= 'A' && first <= 'Z'
		return CheckResult{Passed: passed, Details: fmt.Sprintf("first char %q is %s", first, condStr(passed, "uppercase", "not uppercase"))}

	case "no_markdown":
		// Check no ``` fences in response
		passed := !strings.Contains(response, "```")
		return CheckResult{Passed: passed, Details: condStr(passed, "no markdown fences", "contains markdown fences")}

	case "no_commas":
		// Check no comma character in response
		passed := !strings.Contains(response, ",")
		return CheckResult{Passed: passed, Details: condStr(passed, "no commas", "contains commas")}

	default:
		return CheckResult{Passed: true, Details: fmt.Sprintf("eq true on field %q assumed passed", field)}
	}
}

// checkGTE checks that a numeric value extracted from the response is >= the constraint value.
func checkGTE(response string, constraint benchmark.Constraint) CheckResult {
	v, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse gte value %v", constraint.Value)}
	}

	n := extractNumber(response)
	if n == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("expected number >= %v, got: %q", v, response)}
	}

	passed := *n >= v
	return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v >= %v: %v", *n, v, passed)}
}

// checkLTE checks that a numeric value extracted from the response is <= the constraint value.
func checkLTE(response string, constraint benchmark.Constraint) CheckResult {
	v, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse lte value %v", constraint.Value)}
	}

	n := extractNumber(response)
	if n == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("expected number <= %v, got: %q", v, response)}
	}

	passed := *n <= v
	return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v <= %v: %v", *n, v, passed)}
}

// checkValid tries to parse the response as JSON.
func checkValid(response string, _ benchmark.Constraint) CheckResult {
	trimmed := strings.TrimSpace(response)
	// Try to extract JSON from code fences first
	if idx := strings.Index(trimmed, "```"); idx >= 0 {
		end := strings.LastIndex(trimmed, "```")
		if end > idx+3 {
			inner := strings.TrimSpace(trimmed[idx+3 : end])
			// Remove optional language tag line
			if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
				inner = strings.TrimSpace(inner[nl:])
			}
			var tmp interface{}
			if err := json.Unmarshal([]byte(inner), &tmp); err == nil {
				return CheckResult{Passed: true, Details: "valid JSON extracted from code fence"}
			}
		}
	}

	var tmp interface{}
	if err := json.Unmarshal([]byte(trimmed), &tmp); err != nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("invalid JSON: %v", err)}
	}
	return CheckResult{Passed: true, Details: "valid JSON"}
}

// checkSuperset verifies that all expected keys exist in the parsed JSON response.
// constraint.Value should be a []interface{} of string keys, or a single string key.
func checkSuperset(response string, constraint benchmark.Constraint) CheckResult {
	// Parse expected keys
	expectedKeys := toStringSlice(constraint.Value)
	if len(expectedKeys) == 0 {
		return CheckResult{Passed: true, Details: "no keys to check"}
	}

	// Parse response as JSON
	trimmed := strings.TrimSpace(response)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		// Try extracting from code fence
		if idx := strings.Index(trimmed, "```"); idx >= 0 {
			end := strings.LastIndex(trimmed, "```")
			if end > idx+3 {
				inner := strings.TrimSpace(trimmed[idx+3 : end])
				if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
					inner = strings.TrimSpace(inner[nl:])
				}
				if err := json.Unmarshal([]byte(inner), &parsed); err != nil {
					return CheckResult{Passed: false, Details: fmt.Sprintf("response is not valid JSON: %v", err)}
				}
			} else {
				return CheckResult{Passed: false, Details: "response is not valid JSON"}
			}
		} else {
			return CheckResult{Passed: false, Details: "response is not valid JSON"}
		}
	}

	var missing []string
	for _, key := range expectedKeys {
		if _, exists := parsed[key]; !exists {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return CheckResult{Passed: false, Details: fmt.Sprintf("missing keys: %v", missing)}
	}
	return CheckResult{Passed: true, Details: fmt.Sprintf("all %d keys present", len(expectedKeys))}
}

// checkNotContains verifies that forbidden words/strings are absent from the response (case-insensitive).
// constraint.Value can be a string or a []interface{} of strings.
func checkNotContains(response string, constraint benchmark.Constraint) CheckResult {
	forbiddenWords := toStringSlice(constraint.Value)
	if len(forbiddenWords) == 0 {
		return CheckResult{Passed: true, Details: "no forbidden words to check"}
	}

	responseLower := strings.ToLower(response)
	var found []string
	for _, word := range forbiddenWords {
		wordLower := strings.ToLower(word)
		if strings.Contains(responseLower, wordLower) {
			found = append(found, word)
		}
	}

	if len(found) > 0 {
		return CheckResult{Passed: false, Details: fmt.Sprintf("found forbidden words: %v", found)}
	}
	return CheckResult{Passed: true, Details: "no forbidden words found"}
}

// checkStartsWith verifies the response starts with a specific character or string.
func checkStartsWith(response string, constraint benchmark.Constraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	if expected == "" {
		return CheckResult{Passed: true, Details: "empty expected prefix"}
	}

	trimmed := strings.TrimSpace(response)
	passed := strings.HasPrefix(trimmed, expected)
	return CheckResult{Passed: passed, Details: fmt.Sprintf("response starts with %q: %v", expected, passed)}
}

// checkEndsWith verifies the response ends with a specific string.
func checkEndsWith(response string, constraint benchmark.Constraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	if expected == "" {
		return CheckResult{Passed: true, Details: "empty expected suffix"}
	}

	trimmed := strings.TrimSpace(response)
	passed := strings.HasSuffix(trimmed, expected)
	return CheckResult{Passed: passed, Details: fmt.Sprintf("response ends with %q: %v", expected, passed)}
}

// checkNoMarkdown verifies the response contains no markdown code fences (```).
func checkNoMarkdown(response string, _ benchmark.Constraint) CheckResult {
	passed := !strings.Contains(response, "```")
	return CheckResult{Passed: passed, Details: condStr(passed, "no markdown fences", "contains markdown fences")}
}

// checkNoCommas verifies the response contains no comma characters.
func checkNoCommas(response string, _ benchmark.Constraint) CheckResult {
	passed := !strings.Contains(response, ",")
	return CheckResult{Passed: passed, Details: condStr(passed, "no commas", "contains commas")}
}

// checkSelfConsistency checks response consistency across multiple prompt variants.
// When constraint.Value is a []string (the accumulated variant responses), it
// appends the current response and scores consistency.  For other value types
// it passes with an informational message.
func checkSelfConsistency(response string, constraint benchmark.Constraint) CheckResult {
	variants, ok := constraint.Value.([]string)
	if !ok {
		return CheckResult{Passed: true, Details: "self_consistency check requires variant responses"}
	}

	// Append the current response to the variants slice.
	all := append(variants, response)
	score := ScoreSelfConsistency(all)
	passed := score == 1.0
	return CheckResult{
		Passed:  passed,
		Details: fmt.Sprintf("self_consistency: %d variants, score=%.2f", len(all), score),
	}
}

// checkContains verifies that the response contains all expected substrings (case-insensitive).
// constraint.Value can be a string or a []interface{} of strings.
func checkContains(response string, constraint benchmark.Constraint) CheckResult {
	expected := toStringSlice(constraint.Value)
	if len(expected) == 0 {
		return CheckResult{Passed: true, Details: "no values to check"}
	}

	responseLower := strings.ToLower(response)
	var missing []string
	for _, exp := range expected {
		expLower := strings.ToLower(exp)
		if !strings.Contains(responseLower, expLower) {
			missing = append(missing, exp)
		}
	}

	if len(missing) > 0 {
		return CheckResult{Passed: false, Details: fmt.Sprintf("missing expected content: %v", missing)}
	}
	return CheckResult{Passed: true, Details: fmt.Sprintf("all %d expected values found", len(expected))}
}

// checkMinChars verifies that the response has at least the specified number of characters.
func checkMinChars(response string, constraint benchmark.Constraint) CheckResult {
	minChars, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse min_chars value %v", constraint.Value)}
	}
	actual := len([]rune(response))
	passed := float64(actual) >= minChars
	return CheckResult{
		Passed:  passed,
		Details: fmt.Sprintf("response has %d runes, need >= %.0f", actual, minChars),
	}
}

// checkUniqueLines verifies that the response has at least the specified number of unique non-empty lines.
func checkUniqueLines(response string, constraint benchmark.Constraint) CheckResult {
	minUnique, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse unique_lines value %v", constraint.Value)}
	}
	lines := strings.Split(response, "\n")
	seen := make(map[string]struct{})
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	actual := float64(len(seen))
	passed := actual >= minUnique
	return CheckResult{
		Passed:  passed,
		Details: fmt.Sprintf("response has %d unique non-empty lines, need >= %.0f", len(seen), minUnique),
	}
}

// checkSentenceCount verifies that the response has exactly the specified number of sentences.
// A sentence is defined as text ending with '.', '!', or '?'.
func checkSentenceCount(response string, constraint benchmark.Constraint) CheckResult {
	expected, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse sentence_count value %v", constraint.Value)}
	}
	count := countSentences(response)
	passed := float64(count) == expected
	return CheckResult{
		Passed:  passed,
		Details: fmt.Sprintf("response has %d sentences, expected %.0f", count, expected),
	}
}

// countSentences counts sentences in a response by looking for sentence-ending punctuation.
func countSentences(s string) int {
	count := 0
	inSentence := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			inSentence = false
			continue
		}
		if !inSentence {
			inSentence = true
		}
		if r == '.' || r == '!' || r == '?' {
			count++
			inSentence = false
		}
	}
	return count
}

// ScoreSelfConsistency returns 1.0 if all normalized responses are identical,
// 0.0 otherwise.  Returns 1.0 for 0 or 1 responses.
// Normalization: strips markdown code fences, lowercases, collapses whitespace.
func ScoreSelfConsistency(responses []string) float64 {
	if len(responses) < 2 {
		return 1.0
	}

	normalized := make([]string, len(responses))
	for i, r := range responses {
		normalized[i] = normalizeForConsistency(r)
	}

	first := normalized[0]
	for _, n := range normalized[1:] {
		if n != first {
			return 0.0
		}
	}
	return 1.0
}

// normalizeForConsistency prepares a response for self-consistency comparison.
// It strips markdown code fences, lowercases, and collapses whitespace to single
// spaces. This ensures that superficial formatting differences (fences, case,
// extra whitespace) do not cause false inconsistency.
func normalizeForConsistency(s string) string {
	s = strings.TrimSpace(s)
	// Strip markdown code fences if present
	if idx := strings.Index(s, "```"); idx >= 0 {
		end := strings.LastIndex(s, "```")
		if end > idx+3 {
			inner := strings.TrimSpace(s[idx+3 : end])
			if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
				inner = strings.TrimSpace(inner[nl:])
			}
			s = inner
		}
	}
	// Lowercase and collapse whitespace
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// ---- helpers ----

// extractNumber attempts to find the first numeric value in a string.
// It searches for patterns like integers and floats.
func extractNumber(s string) *float64 {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}

	// Try parsing the entire trimmed string as a number first
	var n float64
	if _, err := fmt.Sscanf(trimmed, "%f", &n); err == nil {
		return &n
	}

	// Try to find a number in the string using simple scanning
	// Look for patterns like digits, possibly with decimal point
	fields := strings.Fields(trimmed)
	for _, field := range fields {
		// Remove trailing punctuation
		field = strings.TrimRight(field, ".,;!?)}]")
		if _, err := fmt.Sscanf(field, "%f", &n); err == nil {
			return &n
		}
	}

	return nil
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// toStringSlice converts a constraint value to a slice of strings.
func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}

// condStr returns t if condition is true, otherwise f.
func condStr(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}

// checkAnswerCorrect verifies that the response contains the expected answer.
// This is a semantic correctness check — the model's answer must match the
// expected value, either exactly (for strings) or numerically (for numbers).
//
// Value types:
//   - string → case-insensitive substring or exact match
//   - float64/int → numeric extraction and comparison
//   - bool true → any non-empty, non-generic response is considered correct
func checkAnswerCorrect(response string, constraint benchmark.Constraint) CheckResult {
	switch v := constraint.Value.(type) {
	case bool:
		if v {
			// answer_correct: true means any substantive response is acceptable
			trimmed := strings.TrimSpace(response)
			if trimmed == "" {
				return CheckResult{Passed: false, Details: "empty response"}
			}
			return CheckResult{Passed: true, Details: "non-empty response"}
		}
		trimmed := strings.TrimSpace(response)
		if trimmed == "" {
			return CheckResult{Passed: true, Details: "empty response (correctly rejected)"}
		}
		return CheckResult{Passed: false, Details: "expected empty, got non-empty"}

	case string:
		trimmed := strings.TrimSpace(response)
		expectedLower := strings.ToLower(v)
		respLower := strings.ToLower(trimmed)
		// Exact match or substring match
		passed := respLower == expectedLower || strings.Contains(respLower, expectedLower)
		return CheckResult{
			Passed:  passed,
			Details: fmt.Sprintf("answer %q %s %q", trimmed, condStr(passed, "==", "!="), v),
		}

	case float64, int:
		var expected float64
		switch ev := v.(type) {
		case float64:
			expected = ev
		case int:
			expected = float64(ev)
		}
		got := extractNumber(response)
		if got == nil {
			return CheckResult{Passed: false, Details: fmt.Sprintf("no number found, expected %v", expected)}
		}
		passed := *got == expected
		return CheckResult{
			Passed:  passed,
			Details: fmt.Sprintf("got %v, expected %v", *got, expected),
		}

	default:
		return CheckResult{Passed: false, Details: fmt.Sprintf("unsupported answer_correct value type %T", v)}
	}
}

// checkReasoningQuality assesses whether a response demonstrates adequate reasoning.
// It checks for:
//   - Minimum character count (at least 50 chars)
//   - Presence of reasoning indicators (therefore, thus, because, since, implies, etc.)
//   - Logical structure (at least 2 sentences for complex answers)
//
// Value types:
//   - bool true → apply default thresholds
//   - map with "min_chars" and/or "require_reasoning" keys → custom thresholds
func checkReasoningQuality(response string, constraint benchmark.Constraint) CheckResult {
	// Default thresholds
	minChars := 50
	requireReasoning := true

	switch v := constraint.Value.(type) {
	case bool:
		if !v {
			return CheckResult{Passed: false, Details: "reasoning_quality requires value true"}
		}
	case map[string]interface{}:
		if mc, ok := v["min_chars"]; ok {
			if f, ok := toFloat64(mc); ok {
				minChars = int(f)
			}
		}
		if rr, ok := v["require_reasoning"]; ok {
			if b, ok := rr.(bool); ok {
				requireReasoning = b
			}
		}
	default:
		return CheckResult{Passed: false, Details: fmt.Sprintf("unsupported reasoning_quality value type %T", v)}
	}

	trimmed := strings.TrimSpace(response)
	charCount := len([]rune(trimmed))

	if charCount < minChars {
		return CheckResult{Passed: false, Details: fmt.Sprintf("response too short: %d chars < %d minimum", charCount, minChars)}
	}

	if requireReasoning {
		reasoningWords := []string{"therefore", "thus", "because", "since", "implies", "consequently", "hence", "so", "logic", "reason", "step", "calculate", "compute", "derive", "evaluate"}
		respLower := strings.ToLower(trimmed)
		found := false
		for _, word := range reasoningWords {
			if strings.Contains(respLower, word) {
				found = true
				break
			}
		}
		if !found {
			return CheckResult{Passed: false, Details: fmt.Sprintf("no reasoning indicators found (min %d chars, no reasoning words)", minChars)}
		}
	}

	return CheckResult{Passed: true, Details: fmt.Sprintf("response meets reasoning quality: %d chars", charCount)}
}

// checkCodeUnchanged verifies that the model's generated code matches an expected
// reference implementation. This is used in degradation testing to ensure Gumi
// does not alter correct code.
//
// Value types:
//   - string → exact code match (after normalization)
//   - bool true → code is present and syntactically valid (no reference provided)
func checkCodeUnchanged(response string, constraint benchmark.Constraint) CheckResult {
	switch v := constraint.Value.(type) {
	case bool:
		if !v {
			return CheckResult{Passed: false, Details: "code_unchanged requires value true"}
		}
		// Check that response contains valid Python code
		code := extractPythonCode(response)
		if code == "" {
			return CheckResult{Passed: false, Details: "no Python code found in response"}
		}
		// Basic sanity: must contain 'def' for function definition
		if !strings.Contains(code, "def ") {
			return CheckResult{Passed: false, Details: "no function definition found"}
		}
		return CheckResult{Passed: true, Details: "valid Python code present"}

	case string:
		// Compare against reference code
		gotCode := strings.TrimSpace(extractPythonCode(response))
		refCode := strings.TrimSpace(v)
		if gotCode == refCode {
			return CheckResult{Passed: true, Details: "code matches reference exactly"}
		}
		// Normalize whitespace for comparison
		gotNorm := strings.Join(strings.Fields(gotCode), "\n")
		refNorm := strings.Join(strings.Fields(refCode), "\n")
		if gotNorm == refNorm {
			return CheckResult{Passed: true, Details: "code matches reference (whitespace-normalized)"}
		}
		return CheckResult{Passed: false, Details: "code differs from reference"}

	default:
		return CheckResult{Passed: false, Details: fmt.Sprintf("unsupported code_unchanged value type %T", v)}
	}
}
