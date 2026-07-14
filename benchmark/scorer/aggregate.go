package scorer

import (
	"math"

	"github.com/EffNine/gumi/benchmark"
)

// Aggregate computes a MetricSet from a collection of test results, using the
// provided weights to compute a weighted score per result.
// This is a stub that aggregates only by count.
func Aggregate(results []benchmark.TestResult, weights map[string]float64) MetricSet {
	// TODO: implement proper weighted aggregation
	if len(results) == 0 {
		return MetricSet{}
	}

	var sum float64
	for _, r := range results {
		var s float64
		for check, score := range r.Subscores {
			if w, ok := weights[check]; ok {
				s += score * w
			} else {
				s += score
			}
		}
		sum += s
	}

	mean := sum / float64(len(results))

	var variance float64
	for _, r := range results {
		var s float64
		for check, score := range r.Subscores {
			if w, ok := weights[check]; ok {
				s += score * w
			} else {
				s += score
			}
		}
		variance += (s - mean) * (s - mean)
	}
	std := math.Sqrt(variance / float64(len(results)))

	return MetricSet{
		Mean: mean,
		Std:  std,
		N:    len(results),
	}
}

// MetricSet is a statistical summary of a group of scores.
type MetricSet struct {
	Mean float64
	Std  float64
	N    int
}
