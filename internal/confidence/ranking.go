package confidence

import (
	"fmt"
	"sort"
)

// Ranking confidence is a DIFFERENT question from capability confidence:
// given two candidates that both passed the capability gate, how much
// measured evidence supports ordering them by performance?
//
// The rules are deterministic and intentionally simple (mean, min/max
// range, pairwise delta). No ML, no distributional assumptions.

// SampleSet holds one candidate's repeated perf-probe observations.
// A single sample cannot support ranking statements on its own.
type SampleSet struct {
	Decode  []float64 // tok/s per perf probe
	Prefill []float64 // tok/s per perf probe
}

// RankingThresholds shape the level decision. A "range" is max−min of a
// candidate's samples; the noise floor when comparing two candidates is the
// larger of their two ranges. Separation is |meanA − meanB|.
//
//	separation ≥ 2 × noise  → HIGH  (no overlap in measured ranges)
//	separation ≥ ½ × noise  → MEDIUM (means ordered, tails may touch)
//	otherwise              → LOW   (operationally indistinguishable)
const (
	highSepRatio = 2.0
	medSepRatio  = 0.5
)

// Ranking is the verdict for one ordered pair (faster-reported first).
type Ranking struct {
	Level             Level
	Indistinguishable bool   // true → do not claim an ordering at all
	Note              string // human-readable, evidence-citing explanation

	// Mean deltas (first minus second); kept for report rendering.
	DecodeDelta  float64
	PrefillDelta float64
}

// RankConfidence compares two gate-passing candidates' measured performance.
// `a` is the currently-preferred (higher-scored) candidate; `b` the runner-up.
// Deterministic: identical inputs always produce an identical Ranking.
func RankConfidence(a, b SampleSet) Ranking {
	r := Ranking{
		DecodeDelta:  mean(a.Decode) - mean(b.Decode),
		PrefillDelta: mean(a.Prefill) - mean(b.Prefill),
	}

	// Missing telemetry never fabricates confidence.
	if len(a.Decode) < 2 || len(b.Decode) < 2 || len(a.Prefill) < 2 || len(b.Prefill) < 2 {
		r.Level = Low
		r.Indistinguishable = true
		r.Note = fmt.Sprintf("insufficient repetitions for ranking (%d vs %d perf runs; need ≥2 each)",
			len(a.Decode), len(b.Decode))
		return r
	}

	decNoise := max(rangeOf(a.Decode), rangeOf(b.Decode))
	preNoise := max(rangeOf(a.Prefill), rangeOf(b.Prefill))
	decSep := abs(r.DecodeDelta)
	preSep := abs(r.PrefillDelta)

	decRatio := ratio(decSep, decNoise)
	preRatio := ratio(preSep, preNoise)
	worst := min(decRatio, preRatio)

	switch {
	// Either metric failing to separate (including zero-variance metrics,
	// whose noise floor is unknown) blocks a confident ordering.
	case decRatio < medSepRatio || preRatio < medSepRatio:
		r.Level = Low
		r.Indistinguishable = true
		r.Note = fmt.Sprintf(
			"performance operationally indistinguishable: decode %+.1f%%, prefill %+.1f%% (within %.1f%% observed spread)",
			pctDelta(r.DecodeDelta, mean(b.Decode)), pctDelta(r.PrefillDelta, mean(b.Prefill)),
			100*max(relRange(a.Decode), relRange(b.Decode)))
	case decRatio >= medSepRatio && preRatio >= medSepRatio &&
		sign(r.DecodeDelta) != sign(r.PrefillDelta):
		// Both metrics are individually distinguishable but disagree about
		// which candidate is faster — no defensible ordering exists.
		r.Level = Low
		r.Indistinguishable = true
		r.Note = "decode and prefill favor different candidates within measurement noise"
	case worst >= highSepRatio:
		r.Level = High
		r.Note = fmt.Sprintf(
			"no overlap in measured ranges: decode %+.1f%%, prefill %+.1f%% vs runner-up",
			pctDelta(r.DecodeDelta, mean(b.Decode)), pctDelta(r.PrefillDelta, mean(b.Prefill)))
	default:
		r.Level = Medium
		r.Note = fmt.Sprintf(
			"means ordered but measurement ranges touch: decode %+.1f%%, prefill %+.1f%%",
			pctDelta(r.DecodeDelta, mean(b.Decode)), pctDelta(r.PrefillDelta, mean(b.Prefill)))
	}
	return r
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func rangeOf(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	sorted := append([]float64(nil), v...)
	sort.Float64s(sorted)
	return sorted[len(sorted)-1] - sorted[0]
}

func relRange(v []float64) float64 {
	m := mean(v)
	if m <= 0 {
		return 0
	}
	return rangeOf(v) / m
}

func ratio(sep, noise float64) float64 {
	if noise <= 0 {
		// No observed variation ⇒ the noise floor is UNKNOWN, not zero.
		// A constant metric provides no evidence for or against separation;
		// claiming decisiveness from it would be fabrication.
		return 0
	}
	return sep / noise
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func min(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

func sign(f float64) int {
	switch {
	case f > 0:
		return 1
	case f < 0:
		return -1
	default:
		return 0
	}
}

func pctDelta(delta, base float64) float64 {
	if base <= 0 {
		return 0
	}
	return delta / base * 100
}
