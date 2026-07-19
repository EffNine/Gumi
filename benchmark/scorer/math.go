package scorer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/EffNine/gumi/benchmark"
)

// mathAnswerCheck evaluates a math word problem response by extracting the
// final numeric answer and comparing it to the expected value.
func mathAnswerCheck(response string, constraint benchmark.Constraint) CheckResult {
	params, ok := constraint.Value.(map[string]interface{})
	if !ok {
		return CheckResult{
			Passed:  false,
			Details: fmt.Sprintf("math_answer constraint value must be a map, got %T", constraint.Value),
		}
	}

	expectedStr, ok := params["answer"].(string)
	if !ok || expectedStr == "" {
		return CheckResult{Passed: false, Details: "math_answer missing expected answer"}
	}

	expected, err := strconv.ParseFloat(expectedStr, 64)
	if err != nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("math_answer invalid expected answer %q: %v", expectedStr, err)}
	}

	got := extractMathAnswer(response)
	if got == nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("no numeric answer found in response: %q", truncate(response, 200))}
	}

	passed := *got == expected
	detail := fmt.Sprintf("got %v, expected %v", *got, expected)
	if !passed {
		detail += " (not equal)"
	}
	return CheckResult{Passed: passed, Details: detail}
}

// extractMathAnswer extracts the final numeric answer from a model response.
// It looks for:
//   - The number after "####" (GSM8K convention)
//   - The last number in the response (fallback)
//   - A number on the last line (fallback)
func extractMathAnswer(response string) *float64 {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return nil
	}

	// First try: look for #### marker (GSM8K convention)
	re := regexp.MustCompile(`####\s*([\d,]+(?:\.\d+)?)`)
	if m := re.FindStringSubmatch(trimmed); len(m) > 1 {
		cleaned := strings.ReplaceAll(m[1], ",", "")
		if n, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return &n
		}
	}

	// Second try: find all numbers in the response, take the last one
	re = regexp.MustCompile(`-?\d+(?:,\d{3})*(?:\.\d+)?`)
	matches := re.FindAllString(trimmed, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		cleaned := strings.ReplaceAll(last, ",", "")
		if n, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return &n
		}
	}

	// Third try: use the generic extractNumber as fallback
	return extractNumber(trimmed)
}
