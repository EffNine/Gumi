package search

import "fmt"

// Stats summarizes repeated performance probes for one operating point.
// HalfRange is (max-min)/2 across successful repeats — the existing
// performance-stability semantics from the evidence-hardening phase.
type Stats struct {
	Mean      float64
	HalfRange float64
	RunsOK    int
	OOM       int
	Timeouts  int
}

// Objective is the workload/user performance target against which measured
// operating points are judged.
//
// V1 deliberately has NO universal tok/s requirement. A floor exists only
// when someone declares one:
//
//   - Floor > 0: an absolute decode floor (user --min-decode). Frontier and
//     eligibility respect it exactly as stated.
//   - otherwise Retention in (0,1): the workload's declared practicality —
//     a larger context must retain this fraction of the best measured decode
//     (Baseline) to count as practical. Declared per workload profile, never
//     per hardware generation.
//   - neither: the objective is stable execution; every stable point passes
//     and ranking orders by workload utility alone.
type Objective struct {
	Floor     float64 // absolute decode tok/s floor; 0 = unset
	Retention float64 // required fraction of Baseline decode; 0 = unset
	Baseline  float64 // best measured decode at the reference operating point
}

// EffectiveFloor resolves the concrete decode tok/s a point must retain.
func (o Objective) EffectiveFloor() float64 {
	switch {
	case o.Floor > 0:
		return o.Floor
	case o.Retention > 0 && o.Baseline > 0:
		return o.Retention * o.Baseline
	default:
		return 0
	}
}

// Describe renders the objective in human terms for reports and console UX.
func (o Objective) Describe() string {
	switch {
	case o.Floor > 0:
		return fmt.Sprintf("decode >= %.1f tok/s (user floor)", o.Floor)
	case o.Retention > 0 && o.Baseline > 0:
		return fmt.Sprintf("decode >= %.1f tok/s (%.0f%% of measured baseline %.1f)",
			o.EffectiveFloor(), o.Retention*100, o.Baseline)
	case o.Retention > 0:
		return fmt.Sprintf("decode within %.0f%% of best measured (baseline pending)", o.Retention*100)
	default:
		return "stable execution; rank by workload utility"
	}
}

// Evaluate judges repeated samples against the objective. Decisions use the
// conservative lower bound (mean − half-range): measurement noise can never
// promote a point past the bar. Zero observed variance is treated as an
// UNKNOWN noise floor, not evidence of separation, so single-run points are
// only judged on their raw mean when that is all that exists.
func (o Objective) Evaluate(s Stats) (bool, string) {
	if s.OOM > 0 {
		return false, "out-of-memory during probes"
	}
	if s.Timeouts > 0 {
		return false, "timeout during probes"
	}
	if s.RunsOK == 0 || s.Mean <= 0 {
		return false, "no usable performance data"
	}
	floor := o.EffectiveFloor()
	if floor <= 0 {
		return true, "stable execution"
	}
	lb := s.Mean - s.HalfRange
	if lb < 0 {
		lb = 0
	}
	if lb+1e-9 >= floor {
		return true, fmt.Sprintf("decode %.1f tok/s meets target %.1f", s.Mean, floor)
	}
	return false, fmt.Sprintf("decode %.1f tok/s below target %.1f (floor %s)",
		s.Mean, floor, fmt.Sprintf("%.1f", floor))
}
