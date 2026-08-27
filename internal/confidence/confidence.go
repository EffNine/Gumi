// Package confidence derives a deterministic HIGH/MEDIUM/LOW confidence
// rating for verified candidates.
//
// No ML, no learned weights: the score is a fixed rule set over measured
// evidence (gate verdicts, repeated-run stability, memory headroom, error
// events). Every level change is explainable from the recorded positives and
// negatives, which are surfaced verbatim in reports.
package confidence

import "fmt"

// Level is the reported confidence tier.
type Level string

const (
	High   Level = "HIGH"
	Medium Level = "MEDIUM"
	Low    Level = "LOW"
)

// Factors is the measured evidence input. Zero values mean "unknown" and are
// treated neutrally — Gumi never penalizes missing data, it just withholds
// the corresponding positive.
type Factors struct {
	GatePassed      bool    // paired capability gate verdict
	CapabilityRate  float64 // Tier 2 rate in [0,1] when HasCapability
	HasCapability   bool    // false in smoke-only runs
	SmokePassed     int
	SmokeTotal      int
	PerfRunsOK      int       // successful perf probes
	PerfRunsFailed  int       // failed perf probes
	DecodeTPS       []float64 // per-sample decode tok/s (len >= 2 enables latency stability)
	PeakVRAMBytes   uint64    // max observed peak VRAM; 0 = unknown
	VRAMBudgetBytes uint64    // safe planning budget; 0 = unknown
	OOMEvents       int
	Timeouts        int
	Experimental    bool // relies on experimental placement (e.g. MoE expert split)
}

// Assessment is the deterministic scoring output.
type Assessment struct {
	Level     Level
	Positives []string
	Negatives []string
}

// Thresholds and constants shaping the rule set.
const (
	latencyStableMax = 0.10 // relative decode spread considered stable
	latencyBadMin    = 0.25 // relative decode spread considered unstable
	headroomMinBytes = 512 << 20
)

// Assess applies the fixed rule set. Deterministic: identical factors always
// produce an identical assessment.
func Assess(f Factors) Assessment {
	a := Assessment{}

	fullSmoke := f.SmokeTotal > 0 && f.SmokePassed == f.SmokeTotal

	// ---- positives ----
	switch {
	case f.HasCapability && f.CapabilityRate >= 0.999:
		a.positive("capability verification passed (Tier 2)")
	case f.HasCapability:
		a.negative(fmt.Sprintf("Tier 2 incomplete (%.0f%%)", f.CapabilityRate*100))
	case fullSmoke:
		a.positive(fmt.Sprintf("smoke verification passed (%d/%d)", f.SmokePassed, f.SmokeTotal))
	}
	if fullSmoke && f.HasCapability {
		a.positive(fmt.Sprintf("smoke suite %d/%d", f.SmokePassed, f.SmokeTotal))
	}

	if f.PerfRunsFailed == 0 && f.PerfRunsOK > 0 {
		if f.PerfRunsOK > 1 {
			a.positive(fmt.Sprintf("%d/%d stable perf runs", f.PerfRunsOK, f.PerfRunsOK))
		} else {
			a.positive("perf run succeeded")
		}
	}
	if spread, ok := decodeSpread(f.DecodeTPS); ok {
		switch {
		case spread <= latencyStableMax:
			a.positive(fmt.Sprintf("stable decode latency (spread %.0f%%)", spread*100))
		case spread >= latencyBadMin:
			a.negative(fmt.Sprintf("unstable decode latency (spread %.0f%% across %d runs)",
				spread*100, len(f.DecodeTPS)))
		}
	}
	if f.PeakVRAMBytes > 0 && f.VRAMBudgetBytes > 0 {
		headroom := f.VRAMBudgetBytes - minU64(f.PeakVRAMBytes, f.VRAMBudgetBytes)
		gb := float64(headroom) / (1 << 30)
		switch {
		case headroom >= headroomMinBytes:
			a.positive(fmt.Sprintf("VRAM headroom %.1fGB", gb))
		default:
			a.negative(fmt.Sprintf("borderline VRAM (headroom %.2fGB)", gb))
		}
	}

	// ---- hard negatives ----
	if !f.GatePassed {
		a.negative("capability gate failed")
	}
	if f.OOMEvents > 0 {
		a.negative(fmt.Sprintf("%d out-of-memory event(s) during verification", f.OOMEvents))
	}
	if f.Timeouts > 0 {
		a.negative(fmt.Sprintf("%d run(s) timed out", f.Timeouts))
	}
	if f.PerfRunsFailed > 0 {
		a.negative(fmt.Sprintf("%d perf run(s) errored", f.PerfRunsFailed))
	}
	if f.Experimental {
		a.negative("experimental expert placement active")
	}

	// ---- level mapping (deterministic precedence) ----
	hardFail := !f.GatePassed || f.OOMEvents > 0 || f.Timeouts > 0 || f.PerfRunsFailed > 0 ||
		(f.HasCapability && f.CapabilityRate < 0.5)
	clean := len(a.Negatives) == 0
	switch {
	case hardFail:
		a.Level = Low
	case clean && f.PerfRunsOK >= 2 && (fullSmoke || f.HasCapability):
		a.Level = High
	default:
		a.Level = Medium
	}
	return a
}

func (a *Assessment) positive(s string) { a.Positives = append(a.Positives, s) }
func (a *Assessment) negative(s string) { a.Negatives = append(a.Negatives, s) }

// decodeSpread returns the relative range (max-min)/max of the samples.
// Returns ok=false unless at least two positive samples exist.
func decodeSpread(samples []float64) (float64, bool) {
	minV, maxV := 0.0, 0.0
	n := 0
	for _, v := range samples {
		if v <= 0 {
			continue
		}
		if n == 0 || v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
		n++
	}
	if n < 2 || maxV <= 0 {
		return 0, false
	}
	return (maxV - minV) / maxV, true
}

func minU64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
