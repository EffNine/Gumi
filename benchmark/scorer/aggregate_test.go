package scorer

import (
	"math"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestAggregate_SingleTestAllOnes(t *testing.T) {
	results := []benchmark.TestResult{
		{Subscores: map[string]float64{"accuracy": 1.0, "format": 1.0}},
	}
	ms := Aggregate(results, nil)
	if ms.Mean != 2.0 {
		t.Errorf("Aggregate with one test (2 subscores=1.0): Mean=%v, want 2.0", ms.Mean)
	}
	if ms.Std != 0 {
		t.Errorf("Aggregate with one test: Std=%v, want 0", ms.Std)
	}
	if ms.N != 1 {
		t.Errorf("Aggregate with one test: N=%v, want 1", ms.N)
	}
}

func TestAggregate_MultipleTestsVaryingScores(t *testing.T) {
	results := []benchmark.TestResult{
		{Subscores: map[string]float64{"acc": 1.0}},
		{Subscores: map[string]float64{"acc": 0.5}},
		{Subscores: map[string]float64{"acc": 0.0}},
	}
	ms := Aggregate(results, nil)
	// sum = 1.0 + 0.5 + 0.0 = 1.5; mean = 0.5
	if math.Abs(ms.Mean-0.5) > 1e-10 {
		t.Errorf("Aggregate: Mean=%v, want 0.5", ms.Mean)
	}
	if ms.N != 3 {
		t.Errorf("Aggregate: N=%v, want 3", ms.N)
	}
	// variance = (0.5)^2 + 0^2 + (-0.5)^2 / 3 = 0.25+0+0.25/3 = 0.1667; std = sqrt(1/6) ≈ 0.4082
	expectedStd := math.Sqrt(1.0 / 6.0)
	if math.Abs(ms.Std-expectedStd) > 1e-10 {
		t.Errorf("Aggregate: Std=%v, want %v", ms.Std, expectedStd)
	}
}

func TestAggregate_EmptyInput(t *testing.T) {
	ms := Aggregate(nil, nil)
	if ms.Mean != 0 || ms.Std != 0 || ms.N != 0 {
		t.Errorf("Aggregate(nil) = %+v, want zero MetricSet", ms)
	}
}

func TestAggregate_EmptyResults(t *testing.T) {
	ms := Aggregate([]benchmark.TestResult{}, nil)
	if ms.Mean != 0 || ms.Std != 0 || ms.N != 0 {
		t.Errorf("Aggregate(empty) = %+v, want zero MetricSet", ms)
	}
}

func TestAggregate_WithWeights(t *testing.T) {
	results := []benchmark.TestResult{
		{Subscores: map[string]float64{"a": 1.0, "b": 0.0}},
		{Subscores: map[string]float64{"a": 0.5, "b": 0.5}},
	}
	weights := map[string]float64{"a": 2.0, "b": 1.0}

	ms := Aggregate(results, weights)
	// Test 1: 1.0*2 + 0.0*1 = 2.0
	// Test 2: 0.5*2 + 0.5*1 = 1.5
	// Mean = (2.0 + 1.5)/2 = 1.75
	if math.Abs(ms.Mean-1.75) > 1e-10 {
		t.Errorf("Aggregate with weights: Mean=%v, want 1.75", ms.Mean)
	}
	if ms.N != 2 {
		t.Errorf("Aggregate with weights: N=%v, want 2", ms.N)
	}
}

func TestAggregate_PartialWeights(t *testing.T) {
	results := []benchmark.TestResult{
		{Subscores: map[string]float64{"a": 0.5, "b": 0.5, "c": 1.0}},
	}
	// Only "a" has a weight; "b" and "c" get weight 1.0.
	weights := map[string]float64{"a": 3.0}
	ms := Aggregate(results, weights)
	// 0.5*3 + 0.5*1 + 1.0*1 = 1.5 + 0.5 + 1.0 = 3.0
	if math.Abs(ms.Mean-3.0) > 1e-10 {
		t.Errorf("Aggregate with partial weights: Mean=%v, want 3.0", ms.Mean)
	}
}

func TestAggregate_NoSubscores(t *testing.T) {
	results := []benchmark.TestResult{
		{Subscores: map[string]float64{}},
		{Subscores: map[string]float64{}},
	}
	ms := Aggregate(results, nil)
	if ms.Mean != 0 || ms.N != 2 {
		t.Errorf("Aggregate with empty subscores: Mean=%v, N=%v; want 0, 2", ms.Mean, ms.N)
	}
}
