// Package workload defines the two MVP workload profiles and their built-in
// verification task suites.
//
// A profile states what "good" means for a use case: minimum context,
// quality vs latency priority, performance probe sizes, and the deterministic
// capability tasks used to gate candidates.
package workload

import (
	"fmt"

	"github.com/EffNine/gumi/internal/verify"
)

// Profile describes a target workload.
//
// The contract is measured, not aspirational (Phase 6 sensitivity study,
// docs/experiments/04): Objective states what ranking optimizes;
// HardConstraints are conditions that must hold for any recommendation —
// they gate, they do not score.
type Profile struct {
	Name        string
	Description string

	// Objective is the primary optimization target used by ranking.
	Objective string
	// HardConstraints must hold for a candidate to be eligible at all.
	HardConstraints []string
	// PreferredMetrics lists the measurements this workload weights most
	// heavily (documentation + report emphasis; weights live in the fields
	// below).
	PreferredMetrics []string

	MinContext int // hard floor: candidates never plan below this

	QualityPriority float64 // weight in [0,1]
	LatencyPriority float64 // weight in [0,1]; both sum to 1

	// DecodeRetention declares this workload's practicality rule for the
	// context frontier: a larger context counts as practical only while it
	// retains this fraction of the BEST MEASURED decode throughput on the
	// current machine. The rule is relative to what the hardware actually
	// delivers, so it carries no assumption about GPU class or speed — a
	// datacenter card and a laptop GPU are each judged against their own
	// baseline. Users override it with an absolute floor via --min-decode.
	// 0 disables the relative rule entirely.
	DecodeRetention float64

	// Sensitivity classification (Phase 7 heuristic-policy input): which
	// physical resources this workload's experienced quality rides on.
	// Declared per profile from measured workload analysis
	// (docs/experiments/04 §3); plain fields, deliberately not a DSL. The
	// policy layer reads these to decide which axes deserve candidate slots;
	// it never overrides MinContext or the capability tasks.
	PrefillBound bool // prompt-processing throughput materially affects experience
	DecodeBound  bool // generation responsiveness is the dominant experience metric
	DepthBound   bool // late-window recall / long-session depth matter

	PerfPromptTokens int // approximate prompt length for perf runs
	PerfGenTokens    int // generation length for perf runs

	SmokeTasks      []verify.Task // Tier 1 — always run
	CapabilityTasks []verify.Task // Tier 2 — run unless tier=smoke

	Notes []string // transparency notes, e.g. golden tasks skipped for missing toolchains
}

// Names lists available workload profiles.
func Names() []string { return []string{"agentic_coding", "chat"} }

// Get resolves a profile by name.
func Get(name string) (*Profile, error) {
	switch name {
	case "agentic_coding":
		return agenticCoding(), nil
	case "chat":
		return chat(), nil
	default:
		return nil, fmt.Errorf("unknown workload profile %q (available: %v)", name, Names())
	}
}
