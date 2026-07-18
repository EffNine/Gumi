package scorer

import (
	"math"
	"sort"

	"github.com/EffNine/gumi/benchmark"
)

// Aggregate computes a MetricSet from a collection of test results, using the
// provided weights to compute a weighted score per result.
// For each test, the per-check subscores are combined into a single 0..1 score
// via weighted averaging: sum(score * weight) / sum(weight).  Weights default
// to 1.0 for any check key not present in the weights map.
func Aggregate(results []benchmark.TestResult, weights map[string]float64) MetricSet {
	if len(results) == 0 {
		return MetricSet{}
	}

	// Compute a normalized weighted score for each test.
	scores := make([]float64, len(results))
	for i, r := range results {
		var scoreSum, weightSum float64
		for check, score := range r.Subscores {
			w := 1.0
			if weights != nil {
				if v, ok := weights[check]; ok {
					w = v
				}
			}
			scoreSum += score * w
			weightSum += w
		}
		if weightSum > 0 {
			scores[i] = scoreSum / weightSum
		}
		// else scores[i] stays 0.0
	}

	mean := meanOf(scores)
	std := stdOf(scores, mean)

	sorted := make([]float64, len(scores))
	copy(sorted, scores)
	sort.Float64s(sorted)

	return MetricSet{
		Mean:   mean,
		Std:    std,
		N:      len(results),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Median: percentile(sorted, 0.5),
		P25:    percentile(sorted, 0.25),
		P75:    percentile(sorted, 0.75),
	}
}

// MetricSet is a statistical summary of a group of scores.
type MetricSet struct {
	Mean   float64
	Std    float64
	N      int
	Min    float64
	Max    float64
	Median float64
	P25    float64
	P75    float64
}

// meanOf returns the arithmetic mean of xs.  Returns 0 for empty/nil slices.
func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}

// stdOf returns the population standard deviation of xs given its mean.
func stdOf(xs []float64, mean float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var variance float64
	for _, v := range xs {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(xs)))
}

// percentile returns the value at the given percentile (0..1) from a sorted
// slice using linear interpolation between the two nearest ranks.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	// R-type index (1-based), same as R's quantile(type=7) default.
	idx := p * float64(n-1)
	lo := int(math.Floor(idx))
	hi := lo + 1
	if hi >= n {
		hi = n - 1
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
