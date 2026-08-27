// Package search implements the deterministic experimental-search logic of
// the Gumi V1 auto-tuner.
//
// Everything here is a pure function of its inputs: same facts in, same plan
// out, forever. No measurement happens inside this package — the pipeline
// feeds observations in and acts on the decisions that come out. Keeping the
// strategy pure makes the tuner auditable and unit-testable without a GPU.
//
// The strategy is deliberately NOT brute force:
//
//	coarse doubling sweep  →  boundary refinement  →  dominance pruning
//	    →  capability gating  →  final verification
package search

import "sort"

// MinRefineGranularity is the smallest context step worth probing during
// boundary refinement. llama.cpp prefers context sizes divisible by small
// powers of two; 1024 tokens keeps every probe well-formed.
const MinRefineGranularity = 1024

// Ladder returns the coarse exploration levels above start: doublings,
// capped by maxCtx, PLUS the cap itself when it sits meaningfully (≥25%)
// above the last doubling — a training-context ceiling like 40960 deserves
// its own probe even though it is not a power of two. Levels ascend.
//
// Example: Ladder(16384, 131072) => [32768 65536 131072].
func Ladder(start, maxCtx int) []int {
	if start <= 0 || maxCtx < start {
		return nil
	}
	var out []int
	for ctx := start * 2; ctx <= maxCtx; ctx *= 2 {
		out = append(out, ctx)
	}
	if maxCtx > start {
		last := start
		if len(out) > 0 {
			last = out[len(out)-1]
		}
		if maxCtx*4 >= last*5 { // maxCtx >= 1.25 × last level
			out = append(out, maxCtx)
		}
	}
	return out
}

// Midpoint returns the next boundary-refinement probe between lo (highest
// passing context) and hi (lowest failing context), rounded down to a
// multiple of granularity. Returns 0 when the bracket is already tighter
// than the granularity — the boundary has been located.
//
// The midpoint is deliberately NOT assumed to be a power of two: real
// practical boundaries land wherever memory and throughput actually run out.
func Midpoint(lo, hi, granularity int) int {
	if granularity < MinRefineGranularity {
		granularity = MinRefineGranularity
	}
	if hi-lo <= granularity {
		return 0
	}
	mid := lo + (hi-lo)/2
	mid -= mid % MinRefineGranularity
	if mid <= lo {
		mid = lo + MinRefineGranularity
	}
	if mid >= hi {
		return 0
	}
	return mid
}

// Observation is one measured operating point (a configuration at a
// specific context size). It carries only what dominance, objectives, and
// profile selection need — derived conservatively from repeated samples.
type Observation struct {
	ID               string  // owning candidate id
	Context          int     // tokens
	DecodeMean       float64 // mean decode tok/s across repeats
	DecodeHalfRange  float64 // (max-min)/2 across repeats
	Prefill          float64 // mean prefill tok/s
	PrefillHalfRange float64 // (max-min)/2 across repeats
	CapRate          float64 // capability suite rate in [0,1]; negative when unmeasured
	PeakVRAM         uint64  // bytes; 0 = unknown
	PeakRAM          uint64  // bytes; 0 = unknown
	Stable           bool    // no OOM/timeout/failure events during probes
	KVQ              int     // KV precision rank (KVRank); execution-line identity
	Batch            int
	UBatch           int
	ExpertsCPU       bool
}

// KVRank orders KV precisions by fidelity for quality comparisons. Unknown
// types rank below every known one.
func KVRank(kv string) int {
	switch kv {
	case "f16":
		return 3
	case "q8_0":
		return 2
	case "q4_0":
		return 1
	default:
		return 0
	}
}

// DecodeLowerBound is the conservative per-observation decode estimate: the
// mean minus half the observed range. Frontier and floor decisions use this
// so a lucky fast run cannot push an unreliable point past the bar.
func (o Observation) DecodeLowerBound() float64 {
	lb := o.DecodeMean - o.DecodeHalfRange
	if lb < 0 {
		return 0
	}
	return lb
}

// PrefillLowerBound mirrors DecodeLowerBound for prefill throughput.
func (o Observation) PrefillLowerBound() float64 {
	lb := o.Prefill - o.PrefillHalfRange
	if lb < 0 {
		return 0
	}
	return lb
}

// DominatedBy reports whether observation a is dominated by observation b.
// b must be at least as good as a on EVERY axis and strictly better on at
// least one:
//
//	benefit axes:  decode (lower bound), prefill, capability rate
//	resource axes: context, peak VRAM, peak RAM — context IS memory here
//	               (KV scales linearly with window size), matching the
//	               product dominance rule: more memory for equal/worse
//	               performance is waste, and its evidence is recorded.
//
// Unmeasured values never participate: capability < 0 means "battery not
// run" and VRAM/RAM == 0 means "unknown". An unknown can neither win nor
// lose its axis, and a point with no strict advantage dominates nothing.
//
// Capability enters as an axis, but callers must still enforce the absolute
// rule separately: raw speed can never rescue a capability FAIL (the gate
// decides eligibility before dominance is consulted).
func DominatedBy(a, b Observation) bool {
	if !b.Stable {
		return false // an unstable point proves nothing about dominance
	}
	if b.DecodeLowerBound() < a.DecodeLowerBound() {
		return false
	}
	if b.PrefillLowerBound() < a.PrefillLowerBound() {
		return false
	}
	if b.Context > a.Context {
		return false // b spends more memory on window; that is a cost
	}
	if b.PeakVRAM > 0 && (a.PeakVRAM == 0 || b.PeakVRAM > a.PeakVRAM) {
		return false
	}
	if b.PeakRAM > 0 && (a.PeakRAM == 0 || b.PeakRAM > a.PeakRAM) {
		return false
	}
	if a.CapRate >= 0 && b.CapRate >= 0 && b.CapRate < a.CapRate {
		return false
	}
	// Strictness: b must beat a on at least one measurable axis by MORE
	// THAN MEASUREMENT NOISE on the noisy axes (decode/prefill ranges);
	// deterministic axes (context, VRAM, RAM, capability) compare exactly.
	// Without this guard, jitter would prune near-identical configurations
	// arbitrarily.
	noise := a.DecodeHalfRange + b.DecodeHalfRange
	strict := b.DecodeMean-a.DecodeMean > noise ||
		b.Prefill-a.Prefill > a.PrefillHalfRange+b.PrefillHalfRange ||
		b.Context < a.Context ||
		(b.PeakVRAM > 0 && a.PeakVRAM > 0 && b.PeakVRAM < a.PeakVRAM) ||
		(b.PeakRAM > 0 && a.PeakRAM > 0 && b.PeakRAM < a.PeakRAM) ||
		(a.CapRate >= 0 && b.CapRate > a.CapRate)
	return strict
}

// PruneDominated removes observations dominated by any surviving peer and
// reports them alongside who dominated them, so the pipeline can record why
// tuning budget was not spent. Input order is preserved for survivors.
func PruneDominated(obs []Observation) (survivors []Observation, pruned map[string]string) {
	pruned = map[string]string{}
	for i := range obs {
		a := obs[i]
		if pruned[a.ID] != "" {
			continue
		}
		for j := range obs {
			b := obs[j]
			if b.ID == a.ID || pruned[b.ID] != "" {
				continue
			}
			if DominatedBy(a, b) {
				pruned[a.ID] = b.ID
				break
			}
		}
	}
	for _, o := range obs {
		if pruned[o.ID] == "" {
			survivors = append(survivors, o)
		}
	}
	sort.SliceStable(survivors, func(i, j int) bool { return survivors[i].Context < survivors[j].Context })
	return survivors, pruned
}
