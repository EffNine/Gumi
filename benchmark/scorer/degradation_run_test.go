package scorer

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

// ---------------------------------------------------------------------------
// RunDegradationChecks
// ---------------------------------------------------------------------------

func TestRunDegradationChecks_NoDegradation(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-01",
			Condition: "direct",
			Passed:    true,
			Output:    "Tokyo is the capital of Japan.",
			LatencyMs: 100,
		},
		{
			TestID:    "deg-01",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    "Tokyo is the capital of Japan.",
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-01": "degradation"}
	report := RunDegradationChecks(results, categories)
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions, got %d", report.OverRepairCount)
	}
	if report.TotalTests != 1 {
		t.Errorf("expected 1 total test, got %d", report.TotalTests)
	}
	if report.DegradationRate != 0 {
		t.Errorf("expected 0 degradation rate, got %v", report.DegradationRate)
	}
}

func TestRunDegradationChecks_SemanticDegradation(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-02",
			Condition: "direct",
			Passed:    true,
			Output:    "The value is 42.",
			LatencyMs: 100,
		},
		{
			TestID:    "deg-02",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    "The value is 99.",
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-02": "degradation"}
	report := RunDegradationChecks(results, categories)
	if report.OverRepairCount != 1 {
		t.Errorf("expected 1 corruption, got %d", report.OverRepairCount)
	}
	if report.TotalTests != 1 {
		t.Errorf("expected 1 total test, got %d", report.TotalTests)
	}
	if report.DegradationRate != 1.0 {
		t.Errorf("expected 1.0 degradation rate, got %v", report.DegradationRate)
	}
	if len(report.Corruptions) != 1 {
		t.Fatalf("expected 1 corruption record, got %d", len(report.Corruptions))
	}
	if report.Corruptions[0].Severity != "semantic" {
		t.Errorf("expected semantic severity, got %q", report.Corruptions[0].Severity)
	}
}

func TestRunDegradationChecks_CosmeticDegradation(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-03",
			Condition: "direct",
			Passed:    true,
			Output:    "Hello world",
			LatencyMs: 100,
		},
		{
			TestID:    "deg-03",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    "  hello   world  ",
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-03": "degradation"}
	report := RunDegradationChecks(results, categories)
	// Cosmetic changes are still counted as corruptions
	if report.OverRepairCount != 1 {
		t.Errorf("expected 1 corruption (cosmetic), got %d", report.OverRepairCount)
	}
	if report.Corruptions[0].Severity != "cosmetic" {
		t.Errorf("expected cosmetic severity, got %q", report.Corruptions[0].Severity)
	}
}

func TestRunDegradationChecks_DirectFailedSkipped(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-04",
			Condition: "direct",
			Passed:    false,
			Output:    "Wrong answer",
			LatencyMs: 100,
		},
		{
			TestID:    "deg-04",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    "Correct answer",
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-04": "degradation"}
	report := RunDegradationChecks(results, categories)
	// If direct failed, Gumi fixing it is NOT degradation
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions when direct failed, got %d", report.OverRepairCount)
	}
}

func TestRunDegradationChecks_NoDirectResult(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-05",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    "Some answer",
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-05": "degradation"}
	report := RunDegradationChecks(results, categories)
	if report.TotalTests != 0 {
		t.Errorf("expected 0 total tests (no direct), got %d", report.TotalTests)
	}
}

func TestRunDegradationChecks_NoGumiResult(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-no-gumi",
			Condition: "direct",
			Passed:    true,
			Output:    "Some answer",
			LatencyMs: 100,
		},
	}
	categories := map[string]string{"deg-no-gumi": "degradation"}
	report := RunDegradationChecks(results, categories)
	// totalTests counts all degradation tests with a direct result,
	// even if no gumi result exists for comparison.
	if report.TotalTests != 1 {
		t.Errorf("expected 1 total test (direct found), got %d", report.TotalTests)
	}
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions (no gumi to compare), got %d", report.OverRepairCount)
	}
}

func TestRunDegradationChecks_MultipleTests(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "deg-m1", Condition: "direct", Passed: true, Output: "A", LatencyMs: 100},
		{TestID: "deg-m1", Condition: "gumi-stabilized", Passed: true, Output: "A", LatencyMs: 150},
		{TestID: "deg-m2", Condition: "direct", Passed: true, Output: "B", LatencyMs: 100},
		{TestID: "deg-m2", Condition: "gumi-stabilized", Passed: true, Output: "C", LatencyMs: 150},
		{TestID: "deg-m3", Condition: "direct", Passed: true, Output: "D", LatencyMs: 100},
		{TestID: "deg-m3", Condition: "gumi-direct", Passed: true, Output: "D", LatencyMs: 120},
	}
	categories := map[string]string{"deg-m1": "degradation", "deg-m2": "degradation", "deg-m3": "degradation"}
	report := RunDegradationChecks(results, categories)
	if report.TotalTests != 3 {
		t.Errorf("expected 3 total tests, got %d", report.TotalTests)
	}
	// deg-m2 has semantic change (B→C), deg-m1 and deg-m3 are identical
	if report.OverRepairCount != 1 {
		t.Errorf("expected 1 corruption, got %d", report.OverRepairCount)
	}
	if report.DegradationRate != 1.0/3.0 {
		t.Errorf("expected degradation rate 0.333, got %v", report.DegradationRate)
	}
}

func TestRunDegradationChecks_PreferredGumiCondition(t *testing.T) {
	// When multiple gumi conditions exist, should prefer stabilized
	results := []benchmark.TestResult{
		{TestID: "deg-p1", Condition: "direct", Passed: true, Output: "X", LatencyMs: 100},
		{TestID: "deg-p1", Condition: "gumi-direct", Passed: true, Output: "Y", LatencyMs: 120},
		{TestID: "deg-p1", Condition: "gumi-stabilized", Passed: true, Output: "X", LatencyMs: 200},
	}
	categories := map[string]string{"deg-p1": "degradation"}
	report := RunDegradationChecks(results, categories)
	// Should use gumi-stabilized (X) not gumi-direct (Y), so no degradation
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions (prefer stabilized), got %d", report.OverRepairCount)
	}
}

func TestRunDegradationChecks_LatencyOverhead(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "deg-l1", Condition: "direct", Passed: true, Output: "A", LatencyMs: 100},
		{TestID: "deg-l1", Condition: "gumi-stabilized", Passed: true, Output: "A", LatencyMs: 250},
	}
	categories := map[string]string{"deg-l1": "degradation"}
	report := RunDegradationChecks(results, categories)
	overhead := report.LatencyOverhead["deg-l1"]
	if overhead != 150 {
		t.Errorf("expected latency overhead 150ms, got %v", overhead)
	}
}

func TestRunDegradationChecks_NegativeLatencyOverhead(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "deg-l2", Condition: "direct", Passed: true, Output: "A", LatencyMs: 300},
		{TestID: "deg-l2", Condition: "gumi-stabilized", Passed: true, Output: "A", LatencyMs: 200},
	}
	categories := map[string]string{"deg-l2": "degradation"}
	report := RunDegradationChecks(results, categories)
	overhead := report.LatencyOverhead["deg-l2"]
	if overhead < 0 {
		t.Errorf("expected non-negative latency overhead, got %v", overhead)
	}
}

func TestRunDegradationChecks_SkipsNonDegradation(t *testing.T) {
	results := []benchmark.TestResult{
		{TestID: "json-01", Condition: "direct", Passed: true, Output: "{}", LatencyMs: 100},
		{TestID: "json-01", Condition: "gumi-stabilized", Passed: true, Output: "{}", LatencyMs: 150},
	}
	categories := map[string]string{"json-01": "json"}
	report := RunDegradationChecks(results, categories)
	if report.TotalTests != 0 {
		t.Errorf("expected 0 tests (not degradation category), got %d", report.TotalTests)
	}
}

func TestRunDegradationChecks_EmptyResults(t *testing.T) {
	report := RunDegradationChecks(nil, nil)
	if report.TotalTests != 0 {
		t.Errorf("expected 0 total tests, got %d", report.TotalTests)
	}
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions, got %d", report.OverRepairCount)
	}
}

func TestRunDegradationChecks_JSONKeyChange(t *testing.T) {
	results := []benchmark.TestResult{
		{
			TestID:    "deg-j1",
			Condition: "direct",
			Passed:    true,
			Output:    `{"name": "Alice"}`,
			LatencyMs: 100,
		},
		{
			TestID:    "deg-j1",
			Condition: "gumi-stabilized",
			Passed:    true,
			Output:    `{"full_name": "Alice"}`,
			LatencyMs: 150,
		},
	}
	categories := map[string]string{"deg-j1": "degradation"}
	report := RunDegradationChecks(results, categories)
	if report.OverRepairCount != 1 {
		t.Errorf("expected 1 corruption (JSON key change), got %d", report.OverRepairCount)
	}
	if report.Corruptions[0].Severity != "semantic" {
		t.Errorf("expected semantic severity for JSON key change, got %q", report.Corruptions[0].Severity)
	}
}

func TestRunDegradationChecks_PartialMatch(t *testing.T) {
	// Test with only some degradation tests having gumi results
	results := []benchmark.TestResult{
		{TestID: "deg-p2", Condition: "direct", Passed: true, Output: "A", LatencyMs: 100},
		// No gumi result for deg-p2
		{TestID: "deg-p3", Condition: "direct", Passed: true, Output: "B", LatencyMs: 100},
		{TestID: "deg-p3", Condition: "gumi-stabilized", Passed: true, Output: "B", LatencyMs: 150},
	}
	categories := map[string]string{"deg-p2": "degradation", "deg-p3": "degradation"}
	report := RunDegradationChecks(results, categories)
	// deg-p2 has direct but no gumi → totalTests=1 (skipped), deg-p3 has both → totalTests=2
	if report.TotalTests != 2 {
		t.Errorf("expected 2 total tests (one skipped), got %d", report.TotalTests)
	}
	// Only deg-p3 can be compared, and outputs are identical
	if report.OverRepairCount != 0 {
		t.Errorf("expected 0 corruptions, got %d", report.OverRepairCount)
	}
}
