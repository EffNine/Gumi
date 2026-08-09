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
//   - A fraction like "1/6" after "####" (GSM8K fraction answers)
//   - A fraction like "1/6" in the response text
//   - The last number in the response (fallback)
func extractMathAnswer(response string) *float64 {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return nil
	}

	// First try: look for #### marker (GSM8K convention)
	// Check for fraction first (e.g., "#### 1/6")
	re := regexp.MustCompile(`####\s*(-?\d+)\s*/\s*(-?\d+)`)
	if m := re.FindStringSubmatch(trimmed); len(m) > 2 {
		num, err1 := strconv.ParseFloat(m[1], 64)
		den, err2 := strconv.ParseFloat(m[2], 64)
		if err1 == nil && err2 == nil && den != 0 {
			result := num / den
			return &result
		}
	}
	// Then check for plain number (e.g., "#### 42")
	re = regexp.MustCompile(`####\s*([\d,]+(?:\.\d+)?)`)
	if m := re.FindStringSubmatch(trimmed); len(m) > 1 {
		cleaned := strings.ReplaceAll(m[1], ",", "")
		if n, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return &n
		}
	}

	// Second try: find fractions in the response text (e.g., "1/6")
	re = regexp.MustCompile(`(-?\d+(?:,\d{3})*(?:\.\d+)?)\s*/\s*(-?\d+(?:,\d{3})*(?:\.\d+)?)`)
	fracMatches := re.FindAllStringSubmatch(trimmed, -1)
	if len(fracMatches) > 0 {
		last := fracMatches[len(fracMatches)-1]
		if len(last) >= 3 {
			num, err1 := strconv.ParseFloat(strings.ReplaceAll(last[1], ",", ""), 64)
			den, err2 := strconv.ParseFloat(strings.ReplaceAll(last[2], ",", ""), 64)
			if err1 == nil && err2 == nil && den != 0 {
				result := num / den
				return &result
			}
		}
	}

	// Third try: find all numbers in the response, take the last one
	re = regexp.MustCompile(`-?\d+(?:,\d{3})*(?:\.\d+)?`)
	matches := re.FindAllString(trimmed, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		cleaned := strings.ReplaceAll(last, ",", "")
		if n, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return &n
		}
	}

	// Fourth try: use the generic extractNumber as fallback
	return extractNumber(trimmed)
}
