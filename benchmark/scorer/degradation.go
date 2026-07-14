package scorer

import (
	"regexp"
	"strings"

	"github.com/EffNine/gumi/benchmark"
)

// CorruptionRecord describes a single instance where Gumi changed correct output.
type CorruptionRecord struct {
	TestID   string
	Original string
	Repaired string
	Severity string
	Detail   string
}

// DegradationReport summarizes over-repair across all degradation tests.
type DegradationReport struct {
	OverRepairCount int
	TotalTests      int
	DegradationRate float64
	Corruptions     []CorruptionRecord
	LatencyOverhead map[string]float64
}

// DegradationDetector compares output before and after Gumi processing
// to detect over-repair and semantic corruption.
type DegradationDetector struct {
	semanticPatterns []*regexp.Regexp
}

// NewDegradationDetector creates a new DegradationDetector with default semantic patterns.
func NewDegradationDetector() *DegradationDetector {
	return &DegradationDetector{
		semanticPatterns: []*regexp.Regexp{
			regexp.MustCompile(`\d+\.\d+`),  // decimal numbers
			regexp.MustCompile(`"[^"]+":`),   // JSON keys
			regexp.MustCompile(`\b(?:true|false|null)\b`), // JSON literals
		},
	}
}

// Compare evaluates whether the repaired output differs from the original in
// a way that indicates over-repair or corruption.
//
// It returns an empty CorruptionRecord (Severity: "") if there is no change.
// Otherwise it classifies the change as "cosmetic" (whitespace/formatting only)
// or "semantic" (numbers, key names, logical content changed).
func (d *DegradationDetector) Compare(original, repaired string, test benchmark.SuiteTest) CorruptionRecord {
	if original == repaired {
		return CorruptionRecord{Severity: "", TestID: test.ID}
	}

	// Normalize whitespace for cosmetic comparison
	origNorm := normalizeWhitespace(original)
	repNorm := normalizeWhitespace(repaired)

	if origNorm == repNorm {
		return CorruptionRecord{
			TestID:   test.ID,
			Original: truncate(original, 200),
			Repaired: truncate(repaired, 200),
			Severity: "cosmetic",
			Detail:   "whitespace-only change",
		}
	}

	// Check for semantic changes
	semanticDiffs := d.detectSemanticChanges(original, repaired)
	if len(semanticDiffs) > 0 {
		return CorruptionRecord{
			TestID:   test.ID,
			Original: truncate(original, 200),
			Repaired: truncate(repaired, 200),
			Severity: "semantic",
			Detail:   strings.Join(semanticDiffs, "; "),
		}
	}

	return CorruptionRecord{
		TestID:   test.ID,
		Original: truncate(original, 200),
		Repaired: truncate(repaired, 200),
		Severity: "cosmetic",
		Detail:   "formatting change without semantic impact",
	}
}

// detectSemanticChanges looks for differences in numbers, JSON keys, and other
// semantically meaningful content between original and repaired strings.
func (d *DegradationDetector) detectSemanticChanges(original, repaired string) []string {
	var changes []string

	// Extract and compare numbers
	origNums := extractNumbers(original)
	repNums := extractNumbers(repaired)
	if !stringSliceEqual(origNums, repNums) {
		changes = append(changes, "numbers changed")
	}

	// Extract and compare JSON keys
	origKeys := extractJSONKeys(original)
	repKeys := extractJSONKeys(repaired)
	if !stringSliceEqual(origKeys, repKeys) {
		changes = append(changes, "JSON keys changed")
	}

	return changes
}

// normalizeWhitespace collapses all whitespace to single spaces and trims.
func normalizeWhitespace(s string) string {
	space := regexp.MustCompile(`\s+`)
	return space.ReplaceAllString(strings.TrimSpace(s), " ")
}

// extractNumbers finds all numeric values (integers and decimals) in a string.
func extractNumbers(s string) []string {
	re := regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	return re.FindAllString(s, -1)
}

// extractJSONKeys finds all JSON-like key strings in the format "key":
func extractJSONKeys(s string) []string {
	re := regexp.MustCompile(`"([^"]+)"\s*:`)
	matches := re.FindAllStringSubmatch(s, -1)
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// stringSliceEqual checks if two string slices contain the same elements in order.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// truncate truncates a string to the given length, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RunDegradationChecks runs the degradation detector on all tests in the
// "degradation" category, comparing direct (original) vs gumi (repaired) outputs.
func RunDegradationChecks(results []benchmark.TestResult, testCategories map[string]string) DegradationReport {
	detector := NewDegradationDetector()

	// Group results by test ID
	testResults := make(map[string]map[string]benchmark.TestResult) // testID → condition → result
	for _, r := range results {
		cat := testCategories[r.TestID]
		if cat != "degradation" {
			continue
		}
		if testResults[r.TestID] == nil {
			testResults[r.TestID] = make(map[string]benchmark.TestResult)
		}
		testResults[r.TestID][r.Condition] = r
	}

	var corruptions []CorruptionRecord
	var totalTests int
	latencyOverhead := make(map[string]float64)

	for testID, condResults := range testResults {
		directResult, hasDirect := condResults[string(ConditionDirect)]
		if !hasDirect {
			continue
		}
		totalTests++

		// Find the best gumi result for comparison (prefer stabilized, fall back to any gumi-*)
		var gumiResult benchmark.TestResult
		var foundGumi bool
		for _, preferred := range []string{"gumi-stabilized", "gumi-lightweight", "gumi-structured", "gumi-direct"} {
			if r, ok := condResults[preferred]; ok {
				gumiResult = r
				foundGumi = true
				break
			}
		}
		if !foundGumi {
			continue
		}

		// Only flag as degradation if the DIRECT result was correct (passed).
		// If the direct result was wrong and Gumi changed it, that's improvement, not corruption.
		if !directResult.Passed {
			continue
		}

		// Compute latency overhead for this test
		latencyOverhead[testID] = gumiResult.LatencyMs - directResult.LatencyMs
		if latencyOverhead[testID] < 0 {
			latencyOverhead[testID] = 0
		}

		// Compare outputs
		rec := detector.Compare(directResult.Output, gumiResult.Output, benchmark.SuiteTest{ID: testID})
		if rec.Severity != "" {
			corruptions = append(corruptions, rec)
		}
	}

	overRepairCount := len(corruptions)
	degradationRate := 0.0
	if totalTests > 0 {
		degradationRate = float64(overRepairCount) / float64(totalTests)
	}

	return DegradationReport{
		OverRepairCount: overRepairCount,
		TotalTests:      totalTests,
		DegradationRate: degradationRate,
		Corruptions:     corruptions,
		LatencyOverhead: latencyOverhead,
	}
}

// Condition identifiers (duplicated here to avoid import cycle with runner package).
const (
	ConditionDirect            = "direct"
	ConditionGumiDirect      = "gumi-direct"
	ConditionGumiLightweight = "gumi-lightweight"
	ConditionGumiStabilized  = "gumi-stabilized"
	ConditionGumiStructured  = "gumi-structured"
	ConditionFrontier          = "frontier"
)
