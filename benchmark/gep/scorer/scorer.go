// Package scorer implements the scoring engine for GEP test results.
package scorer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

// CheckFunc evaluates a model response against a single GEP constraint.
type CheckFunc func(response string, constraint types.GEPConstraint) CheckResult

// CheckResult holds the outcome of a single constraint check.
type CheckResult struct {
	Passed  bool
	Details string
}

// CheckRegistry maps check operator names to their implementations.
var CheckRegistry = map[string]CheckFunc{
	"eq":                    checkEQ,
	"gte":                   checkGTE,
	"lte":                   checkLTE,
	"valid":                 checkValid,
	"superset":              checkSuperset,
	"not_contains":          checkNotContains,
	"starts_with":           checkStartsWith,
	"ends_with":             checkEndsWith,
	"no_markdown":           checkNoMarkdown,
	"no_commas":             checkNoCommas,
	"self_consistency":      checkSelfConsistency,
	"json":                  checkJSONValid,
	"answer_match":          checkAnswerMatch,
	"numeric_correct":       checkNumericCorrect,
	"expected_answer_match": checkExpectedAnswerMatch,
	"answer_type":           checkAnswerType,
	"contains_expected":     checkContainsExpected,
}

// Scorer evaluates GEP test responses against constraints.
type Scorer struct {
	checks map[string]CheckFunc
}

// New creates a new GEP Scorer with the default check registry.
func New() *Scorer {
	reg := make(map[string]CheckFunc)
	for name, fn := range CheckRegistry {
		reg[name] = fn
	}
	return &Scorer{checks: reg}
}

// Score evaluates a single GEP test's response against its constraints.
func (s *Scorer) Score(test types.GEPTest, response string) types.GEPResult {
	result := types.GEPResult{
		TestID:    test.ID,
		SuiteID:   string(test.Type),
		Passed:    true,
		Subscores: make(map[string]float64),
	}

	if len(test.Constraints) == 0 {
		return result
	}

	var errors []string
	allPassed := true

	for _, constraint := range test.Constraints {
		checkFn, ok := s.checks[constraint.Operator]
		if !ok {
			result.Subscores[constraint.Field] = 0.0
			errors = append(errors, fmt.Sprintf("unknown operator %q for field %q", constraint.Operator, constraint.Field))
			allPassed = false
			continue
		}

		checkResult := checkFn(response, constraint)
		if checkResult.Passed {
			result.Subscores[constraint.Field] = 1.0
		} else {
			result.Subscores[constraint.Field] = 0.0
			errors = append(errors, fmt.Sprintf("%s: %s", constraint.Field, checkResult.Details))
			allPassed = false
		}
	}

	result.Passed = allPassed
	if len(errors) > 0 {
		result.Error = strings.Join(errors, "; ")
	}

	return result
}

// ScoreSelfConsistency returns 1.0 if all normalized responses are identical, 0.0 otherwise.
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

// ---- Check implementations ----

func checkEQ(response string, constraint types.GEPConstraint) CheckResult {
	switch v := constraint.Value.(type) {
	case bool:
		if !v {
			result := checkEQBooleanTrue(response, constraint.Field)
			return CheckResult{
				Passed:  !result.Passed,
				Details: fmt.Sprintf("expected false, %s", result.Details),
			}
		}
		return checkEQBooleanTrue(response, constraint.Field)

	case float64:
		n := extractNumber(response)
		if n == nil {
			return CheckResult{Passed: false, Details: fmt.Sprintf("expected number, got %q", response)}
		}
		passed := *n == v
		return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v, expected %v", *n, v)}

	case int:
		fv := float64(v)
		n := extractNumber(response)
		if n == nil {
			return CheckResult{Passed: false, Details: fmt.Sprintf("expected number, got %q", response)}
		}
		passed := *n == fv
		return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v, expected %v", *n, fv)}

	case string:
		trimmed := strings.TrimSpace(response)
		passed := strings.EqualFold(trimmed, v)
		return CheckResult{Passed: passed, Details: fmt.Sprintf("response %q %s %q", trimmed, condStr(passed, "==", "!="), v)}

	default:
		return CheckResult{Passed: false, Details: fmt.Sprintf("unsupported eq type %T", constraint.Value)}
	}
}

// checkEQBooleanTrue handles the `eq: true` pattern where the field name determines
// what semantic check to perform on the response.
func checkEQBooleanTrue(response string, field string) CheckResult {
	switch field {
	case "capital_start":
		trimmed := strings.TrimSpace(response)
		if len(trimmed) == 0 {
			return CheckResult{Passed: false, Details: "empty response, cannot check capital start"}
		}
		first := trimmed[0]
		passed := first >= 'A' && first <= 'Z'
		return CheckResult{Passed: passed, Details: fmt.Sprintf("first char %q is %s", first, condStr(passed, "uppercase", "not uppercase"))}

	case "no_markdown":
		passed := !strings.Contains(response, "```")
		return CheckResult{Passed: passed, Details: condStr(passed, "no markdown fences", "contains markdown fences")}

	case "no_commas":
		passed := !strings.Contains(response, ",")
		return CheckResult{Passed: passed, Details: condStr(passed, "no commas", "contains commas")}

	default:
		return CheckResult{Passed: true, Details: fmt.Sprintf("eq true on field %q assumed passed", field)}
	}
}

func checkGTE(response string, constraint types.GEPConstraint) CheckResult {
	v, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse gte value %v", constraint.Value)}
	}
	n := extractNumber(response)
	if n == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("expected number >= %v, got %q", v, response)}
	}
	passed := *n >= v
	return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v >= %v: %v", *n, v, passed)}
}

func checkLTE(response string, constraint types.GEPConstraint) CheckResult {
	v, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse lte value %v", constraint.Value)}
	}
	n := extractNumber(response)
	if n == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("expected number <= %v, got %q", v, response)}
	}
	passed := *n <= v
	return CheckResult{Passed: passed, Details: fmt.Sprintf("got %v <= %v: %v", *n, v, passed)}
}

func checkValid(response string, _ types.GEPConstraint) CheckResult {
	return checkJSONValid(response, types.GEPConstraint{})
}

func checkJSONValid(response string, _ types.GEPConstraint) CheckResult {
	trimmed := strings.TrimSpace(response)
	if idx := strings.Index(trimmed, "```"); idx >= 0 {
		end := strings.LastIndex(trimmed, "```")
		if end > idx+3 {
			inner := strings.TrimSpace(trimmed[idx+3 : end])
			if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
				inner = strings.TrimSpace(inner[nl:])
			}
			var tmp interface{}
			if err := json.Unmarshal([]byte(inner), &tmp); err == nil {
				return CheckResult{Passed: true, Details: "valid JSON from code fence"}
			}
		}
	}
	var tmp interface{}
	if err := json.Unmarshal([]byte(trimmed), &tmp); err != nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("invalid JSON: %v", err)}
	}
	return CheckResult{Passed: true, Details: "valid JSON"}
}

func checkSuperset(response string, constraint types.GEPConstraint) CheckResult {
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
		return CheckResult{Passed: false, Details: fmt.Sprintf("missing values: %v", missing)}
	}
	return CheckResult{Passed: true, Details: fmt.Sprintf("all %d values present", len(expected))}
}

func checkNotContains(response string, constraint types.GEPConstraint) CheckResult {
	forbidden := toStringSlice(constraint.Value)
	if len(forbidden) == 0 {
		return CheckResult{Passed: true, Details: "no forbidden values"}
	}

	responseLower := strings.ToLower(response)
	var found []string
	for _, word := range forbidden {
		wordLower := strings.ToLower(word)
		if strings.Contains(responseLower, wordLower) {
			found = append(found, word)
		}
	}

	if len(found) > 0 {
		return CheckResult{Passed: false, Details: fmt.Sprintf("found forbidden: %v", found)}
	}
	return CheckResult{Passed: true, Details: "no forbidden values found"}
}

func checkStartsWith(response string, constraint types.GEPConstraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	trimmed := strings.TrimSpace(response)
	passed := strings.HasPrefix(trimmed, expected)
	return CheckResult{Passed: passed, Details: fmt.Sprintf("starts with %q: %v", expected, passed)}
}

func checkEndsWith(response string, constraint types.GEPConstraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	trimmed := strings.TrimSpace(response)
	passed := strings.HasSuffix(trimmed, expected)
	return CheckResult{Passed: passed, Details: fmt.Sprintf("ends with %q: %v", expected, passed)}
}

func checkNoMarkdown(response string, _ types.GEPConstraint) CheckResult {
	passed := !strings.Contains(response, "```")
	return CheckResult{Passed: passed, Details: condStr(passed, "no markdown fences", "contains markdown fences")}
}

func checkNoCommas(response string, _ types.GEPConstraint) CheckResult {
	passed := !strings.Contains(response, ",")
	return CheckResult{Passed: passed, Details: condStr(passed, "no commas", "contains commas")}
}

func checkSelfConsistency(response string, constraint types.GEPConstraint) CheckResult {
	variants, ok := constraint.Value.([]string)
	if !ok {
		return CheckResult{Passed: true, Details: "self_consistency requires variant responses"}
	}

	all := append(variants, response)
	score := ScoreSelfConsistency(all)
	passed := score == 1.0
	return CheckResult{
		Passed:  passed,
		Details: fmt.Sprintf("self_consistency: %d responses, score=%.2f", len(all), score),
	}
}

func checkAnswerMatch(response string, constraint types.GEPConstraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	trimmed := strings.TrimSpace(strings.ToLower(response))
	expectedLower := strings.ToLower(expected)
	passed := strings.Contains(trimmed, expectedLower) || expectedLower == trimmed
	return CheckResult{Passed: passed, Details: fmt.Sprintf("answer match %q: %v", expected, passed)}
}

func checkNumericCorrect(response string, constraint types.GEPConstraint) CheckResult {
	v, ok := toFloat64(constraint.Value)
	if !ok {
		return CheckResult{Passed: false, Details: fmt.Sprintf("cannot parse numeric value %v", constraint.Value)}
	}
	n := extractNumber(response)
	if n == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("no number found in response %q", response)}
	}
	passed := *n == v
	return CheckResult{Passed: passed, Details: fmt.Sprintf("numeric answer %v == %v: %v", *n, v, passed)}
}

func checkExpectedAnswerMatch(response string, constraint types.GEPConstraint) CheckResult {
	expected := fmt.Sprintf("%v", constraint.Value)
	if expected == "true" {
		return CheckResult{Passed: true, Details: "expected_answer_match: true (metadata check)"}
	}
	return CheckResult{Passed: false, Details: fmt.Sprintf("expected_answer_match: %v", expected)}
}

func checkAnswerType(response string, constraint types.GEPConstraint) CheckResult {
	reqType := fmt.Sprintf("%v", constraint.Value)
	lower := strings.ToLower(strings.TrimSpace(response))
	switch reqType {
	case "yes_no":
		passed := strings.Contains(lower, "yes") || strings.Contains(lower, "no")
		return CheckResult{Passed: passed, Details: fmt.Sprintf("yes/no answer: %v", passed)}
	default:
		return CheckResult{Passed: true, Details: fmt.Sprintf("answer type %q: assumed passed", reqType)}
	}
}

func checkContainsExpected(response string, constraint types.GEPConstraint) CheckResult {
	expected := toStringSlice(constraint.Value)
	responseLower := strings.ToLower(response)
	var found []string
	for _, exp := range expected {
		if strings.Contains(responseLower, strings.ToLower(exp)) {
			found = append(found, exp)
		}
	}
	if len(found) == 0 {
		return CheckResult{Passed: false, Details: fmt.Sprintf("none of %v found", expected)}
	}
	return CheckResult{Passed: true, Details: fmt.Sprintf("found: %v", found)}
}

// ---- Helpers ----

func extractNumber(s string) *float64 {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	var n float64
	if _, err := fmt.Sscanf(trimmed, "%f", &n); err == nil {
		return &n
	}
	fields := strings.Fields(trimmed)
	for _, field := range fields {
		field = strings.TrimRight(field, ".,;!?)}]")
		if _, err := fmt.Sscanf(field, "%f", &n); err == nil {
			return &n
		}
	}
	return nil
}

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

func condStr(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}
