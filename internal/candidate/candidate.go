// Package candidate generates the small, deterministic set of inference
// configurations Gumi verifies for a model + hardware + workload triple.
//
// This is deliberately not AutoML: at most five candidates, produced by
// explicit arithmetic over model geometry (KV cache bytes are exactly
// computable) and hardware budgets. Candidate selection is shaped by the
// heuristic policy layer (internal/policy), which allocates slots to the
// axes with demonstrated potential impact for this shape; feasibility math
// here stays authoritative, and real numbers come from measurement later.
package candidate

import (
	"fmt"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/confidence"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/policy"
	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

// Kind classifies a candidate.
type Kind string

const (
	KindReference Kind = "reference"
	KindQuality   Kind = "quality"
	KindBalanced  Kind = "balanced"
	KindSpeed     Kind = "speed"
	// KindBaseline marks a human-provided configuration admitted through
	// `--baseline` so it is measured and capability-gated exactly like
	// generated candidates. It exists for validation/comparison runs.
	KindBaseline Kind = "baseline"
)

// Candidate is one configuration under test.
type Candidate struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Kind      Kind           `json:"kind"`
	Rationale string         `json:"rationale"`
	Config    backend.Config `json:"config"`
	// Slot records which policy slot shaped this candidate (empty for
	// REFERENCE and human baselines) — decision-trace provenance.
	Slot             string `json:"slot,omitempty"`
	PredictedVRAM    uint64 `json:"predicted_vram_bytes,omitempty"`
	PredictedRAM     uint64 `json:"predicted_ram_bytes,omitempty"`
	Feasible         bool   `json:"feasible"`
	InfeasibleReason string `json:"infeasible_reason,omitempty"`

	// Experimental marks configs relying on placement behavior (currently
	// MoE expert tensor placement) whose interaction with hardware varies
	// across driver/backend versions. Experimental candidates are never
	// silently recommended without the label surfacing in reports.
	Experimental     bool     `json:"experimental,omitempty"`
	ExperimentalNote string   `json:"experimental_note,omitempty"`
	ReferenceWhy     []string `json:"reference_why,omitempty"` // REFERENCE only: selection policy

	// ProbeOnly marks exploratory operating points measured during the
	// frontier sweep/refinement (performance + stability evidence only).
	// They inform the search but carry no capability verdict of their own;
	// a probe point is only promoted to a recommendation after the full
	// battery clears it.
	ProbeOnly bool `json:"probe_only,omitempty"`

	// DominatedBy names the measured configuration that dominates this one
	// (pipeline-owned); dominated candidates are not granted further budget.
	DominatedBy string `json:"dominated_by,omitempty"`

	// Filled during verification (pipeline-owned).
	Measured   *Measurement           `json:"measured,omitempty"`
	Gate       *GateResult            `json:"gate,omitempty"`
	Score      float64                `json:"score,omitempty"`
	Confidence *confidence.Assessment `json:"confidence,omitempty"`
}

const expPlacementNote = "Experimental: expert placement changed (-ot exps=CPU). " +
	"This affects execution only — weights, active expert count, and the computation " +
	"graph are never modified — but hardware/driver behavior varies and some model " +
	"families require special handling."

// Measurement holds verified performance and capability for a candidate.
type Measurement struct {
	PrefillTPS float64             `json:"prefill_tps"`
	DecodeTPS  float64             `json:"decode_tps"`
	PeakVRAM   uint64              `json:"peak_vram_bytes,omitempty"`
	PeakRAM    uint64              `json:"peak_ram_bytes,omitempty"`
	Smoke      *verify.SuiteResult `json:"smoke,omitempty"`
	Capability *verify.SuiteResult `json:"capability,omitempty"`
	Error      string              `json:"error,omitempty"`

	// Reproducibility evidence (pipeline-owned, from repeated perf probes).
	PerfSamples []PerfSample `json:"perf_samples,omitempty"`
	RunsOK      int          `json:"runs_ok,omitempty"`
	RunsFailed  int          `json:"runs_failed,omitempty"`
	OOMEvents   int          `json:"oom_events,omitempty"`
	Timeouts    int          `json:"timeouts,omitempty"`

	// ObjectiveMet records the frontier-objective verdict for probe-only
	// operating points (pipeline-owned). False with a non-empty note is a
	// rejection reason; both zero/empty means "not judged".
	ObjectiveMet  *bool  `json:"objective_met,omitempty"`
	ObjectiveNote string `json:"objective_note,omitempty"`
}

// PerfSample is one performance probe observation.
type PerfSample struct {
	PrefillTPS float64 `json:"prefill_tps"`
	DecodeTPS  float64 `json:"decode_tps"`
	PeakVRAM   uint64  `json:"peak_vram_bytes,omitempty"`
}

// GateResult records the paired verification verdict.
type GateResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

// SplitContext returns the context of a candidate formatted for console UX
// (e.g. "64K"), falling back to the raw token count below 1024.
func (c Candidate) ContextLabel() string {
	ctx := c.Config.ContextTokens
	if ctx >= 1024 && ctx%1024 == 0 {
		return fmt.Sprintf("%dK", ctx/1024)
	}
	return fmt.Sprintf("%d", ctx)
}

// Budget constants shaping feasibility math. They are conservative by design;
// measured data is authoritative afterwards.
const (
	vramSafetyFactor = 0.95
	ramHeadroomBytes = uint64(4) << 30
	computeBaseBytes = uint64(384) << 20
	perTokenOverhead = 24 << 10 // activations/logits heuristic per context token
	minContextFloor  = 2048
)

// Generator produces candidates deterministically.
type Generator struct {
	Model    *gguf.ModelInfo
	Hardware *hardware.Info
	Profile  *workload.Profile

	// plan is the evaluated heuristic policy (Phase 7): explicit, sourced
	// decisions plus admitted/declared-declined candidate slots. Facts and
	// formulas feed it; it never bypasses feasibility math or the gate.
	plan *policy.Plan

	// builderNote carries a slot-specific decline reason from a builder that
	// returned nil; consumed immediately by Generate.
	builderNote string

	gpuTotal uint64
	gpuFree  uint64
	hasGPU   bool
	threads  int

	// Backend capabilities discovered at runtime (optional). When set,
	// planning never produces configurations the installed build cannot
	// express: unsupported KV types and expert placement are suppressed
	// upstream instead of failing mid-verification.
	capsSet bool
	caps    backend.Capabilities
}

// ApplyBackendCaps records probed backend capabilities so planning respects
// them. Call before Generate for real runs; unset means permissive planning
// (dry-run previews).
func (g *Generator) ApplyBackendCaps(caps backend.Capabilities) {
	g.caps = caps
	g.capsSet = true
}

// kvAllowed reports whether planning may use this KV precision.
func (g *Generator) kvAllowed(kv string) bool {
	return !g.capsSet || g.caps.SupportedKV(kv)
}

// placementAllowed reports whether planning may move expert tensors to CPU.
func (g *Generator) placementAllowed() bool {
	if !g.SplitAllowed() {
		return false
	}
	return !g.capsSet || !g.caps.Discovered || g.caps.OverrideTensor
}

// NewGenerator validates inputs and returns a ready generator.
func NewGenerator(m *gguf.ModelInfo, hw *hardware.Info, p *workload.Profile) (*Generator, error) {
	if m == nil || hw == nil || p == nil {
		return nil, fmt.Errorf("model, hardware, and profile are required")
	}
	if m.LayerCount <= 0 || m.HeadDim <= 0 {
		return nil, fmt.Errorf("model geometry incomplete (layers=%d head_dim=%d); cannot plan",
			m.LayerCount, m.HeadDim)
	}
	g := &Generator{Model: m, Hardware: hw, Profile: p}
	if t := hw.TotalVRAMBytes(); t > 0 {
		g.hasGPU = true
		g.gpuTotal = t
		g.gpuFree = hw.FreeVRAMBytes()
		if g.gpuFree == 0 {
			g.gpuFree = t // free memory unknown; assume idle card rather than skip GPU
		}
	}
	g.threads = hw.CPU.Threads()
	if g.threads <= 0 {
		g.threads = 1
	}
	g.plan = policy.Evaluate(g.policyInput())
	return g, nil
}

// Plan exposes the evaluated heuristic policy for reporting and tests.
func (g *Generator) Plan() *policy.Plan { return g.plan }

// policyInput assembles the separated facts plus deterministic-constraint
// outcomes for the policy layer. All VRAM arithmetic stays in this package;
// policy interprets outcomes but never re-derives memory math.
func (g *Generator) policyInput() policy.Input {
	in := policy.Input{
		Architecture: g.Model.Architecture,
		LayerCount:   g.Model.LayerCount,
		TrainContext: g.Model.TrainContext,
		MoE:          g.Model.MoE != nil,
		KVBytesPerToken: map[string]uint64{
			"f16":  g.KVBytesPerToken("f16"),
			"q8_0": g.KVBytesPerToken("q8_0"),
			"q4_0": g.KVBytesPerToken("q4_0"),
		},
		HasGPU:            g.hasGPU,
		VRAMBudgetBytes:   g.VRAMBudget(),
		RAMAvailableBytes: g.Hardware.RAM.AvailableBytes,
		Workload:          g.Profile.Name,
		MinContext:        g.Profile.MinContext,
		PrefillBound:      g.Profile.PrefillBound,
		DecodeBound:       g.Profile.DecodeBound,
		DepthBound:        g.Profile.DepthBound,
	}
	if total := g.Model.WeightBytes; total > 0 && g.Model.ExpertBytes > 0 {
		in.ExpertShare = float64(g.Model.ExpertBytes) / float64(total)
	}
	if g.hasGPU {
		ctx := g.clampContext(g.Profile.MinContext)
		in.FullOffloadFeasible = g.layersThatFit(ctx, "f16", false) == int(g.Model.LayerCount)
		need := float64(g.fullOffloadNeed(ctx))
		budgetF := float64(g.gpuTotal) * vramSafetyFactor
		in.FitHeadroomFraction = (budgetF - need) / budgetF
		in.SplitChangesPlacement = g.SplitAllowed() &&
			g.layersThatFit(ctx, "f16", true) > g.layersThatFit(ctx, "f16", false)
	}
	return in
}

// fullOffloadNeed is the exact predicted GPU-resident footprint when every
// layer sits on the device at f16 KV.
func (g *Generator) fullOffloadNeed(ctx int) uint64 {
	w, _ := g.weightsOnGPU(false)
	return w + g.KVBytesPerToken("f16")*uint64(ctx) + g.ComputeOverhead(ctx)
}

// KVBytesPerToken returns exact f16 KV cache bytes per token for the model.
func (g *Generator) KVBytesPerToken(kv string) uint64 { return g.Model.KVBytesPerToken(kv) }

// ComputeOverhead estimates activation/logit working set for a context size.
func (g *Generator) ComputeOverhead(ctx int) uint64 {
	return computeBaseBytes + uint64(ctx)*perTokenOverhead
}

// weightsOnGPU splits weight residency given expert placement.
func (g *Generator) weightsOnGPU(expertsCPU bool) (gpuWeights, cpuWeights uint64) {
	expertBytes := g.Model.ExpertBytes
	total := g.Model.WeightBytes
	if total == 0 {
		total = g.Model.FileSize // fall back to file size when tensors unknown
	}
	if expertsCPU && expertBytes > 0 {
		return total - expertBytes, expertBytes
	}
	return total, 0
}

// maxContextFor computes the largest context that fits the VRAM budget with
// full offload at a given KV precision. Returns 0 if even tiny ctx fails.
func (g *Generator) maxContextFor(kvType string, expertsCPU bool) int {
	if !g.hasGPU {
		return g.maxContextInRAM(kvType)
	}
	kvPerTok := g.KVBytesPerToken(kvType)
	if kvPerTok == 0 {
		return 0
	}
	gpuWeights, _ := g.weightsOnGPU(expertsCPU)
	budgetF := float64(g.gpuTotal)*vramSafetyFactor - float64(gpuWeights) - float64(computeBaseBytes)
	if budgetF <= 0 {
		return 0
	}
	tokBudget := int(budgetF / float64(kvPerTok+perTokenOverhead))
	if tokBudget < minContextFloor {
		return 0
	}
	return tokBudget
}

// MaxContextFor is the exported planning bound: the largest context the
// deterministic memory arithmetic predicts for a KV precision + placement.
// The tuner uses it to cap exploration; measurement stays authoritative.
func (g *Generator) MaxContextFor(kvType string, expertsCPU bool) int {
	return g.maxContextFor(kvType, expertsCPU)
}

// HasGPU reports whether a discrete GPU was probed.
func (g *Generator) HasGPU() bool { return g.hasGPU }

// Threads returns the probed CPU thread count used for execution.
func (g *Generator) Threads() int { return g.threads }

// maxContextInRAM is the CPU-only fallback path.
func (g *Generator) maxContextInRAM(kvType string) int {
	kvPerTok := g.KVBytesPerToken(kvType)
	if kvPerTok == 0 || g.Hardware.RAM.AvailableBytes == 0 {
		return 0
	}
	avail := int64(g.Hardware.RAM.AvailableBytes) - int64(ramHeadroomBytes) -
		int64(g.Model.WeightBytes) - int64(computeBaseBytes)
	if avail <= 0 {
		return 0
	}
	n := int(avail / int64(kvPerTok+perTokenOverhead))
	if n < minContextFloor {
		return 0
	}
	return n
}

// layersThatFit computes how many layers fit on GPU given KV + overhead.
//
// In expert-split mode the marginal cost of one more layer excludes expert
// tensors: the -ot exps=CPU override keeps them in system RAM regardless of
// -ngl, so per-layer cost must use the non-expert weight share. Counting
// experts per layer would systematically under-offload MoE models
// (Qwen3-30B-A3B: ~0.9 GB non-expert total vs ~390 MB/layer full-weight).
func (g *Generator) layersThatFit(ctx int, kvType string, expertsCPU bool) int {
	if !g.hasGPU {
		return 0
	}
	gpuWeightsFull, _ := g.weightsOnGPU(false)
	perLayerWeights := gpuWeightsFull
	if expertsCPU {
		nonExpertTotal, _ := g.weightsOnGPU(true)
		if nonExpertTotal < gpuWeightsFull {
			perLayerWeights = nonExpertTotal
		}
	}
	perLayer := perLayerWeights / uint64(g.Model.LayerCount)
	kvBytes := g.KVBytesPerToken(kvType) * uint64(ctx)
	overhead := g.ComputeOverhead(ctx)

	budget := float64(g.gpuTotal) * vramSafetyFactor
	fixed := float64(kvBytes) + float64(overhead)
	// With -ngl N, the GPU holds N/total of the per-mode layer weights plus
	// KV + activations. Solve for the largest N within the safe budget;
	// weights left over land on CPU (or stay in RAM under expert split).
	affordable := budget - fixed
	if affordable <= 0 && !expertsCPU {
		// Try harder: without expert placement knobs we can still shrink? No —
		// report zero; caller degrades context instead.
		return 0
	}
	if affordable < 0 {
		return 0
	}
	if perLayer == 0 {
		return int(g.Model.LayerCount)
	}
	n := int(affordable / float64(perLayer))
	if n > int(g.Model.LayerCount) {
		n = int(g.Model.LayerCount)
	}
	if n < 0 {
		n = 0
	}
	return n
}

// clampContext clamps desired context into [floor, model training limit].
func (g *Generator) clampContext(desired int) int {
	ctx := desired
	if ctx < g.Profile.MinContext {
		ctx = g.Profile.MinContext
	}
	if ctx < minContextFloor {
		ctx = minContextFloor
	}
	if g.Model.TrainContext > 0 && ctx > int(g.Model.TrainContext) {
		ctx = int(g.Model.TrainContext)
	}
	return ctx
}

func baseConfig() backend.Config {
	return backend.Config{
		KVCacheType: "f16",
		MMap:        true,
		Seed:        verificationSeed,
		Temperature: 0,
		BatchSize:   2048,
		UBatchSize:  512,
	}
}

// verificationSeed keeps paired runs reproducible across all candidates.
const verificationSeed = int64(42)
