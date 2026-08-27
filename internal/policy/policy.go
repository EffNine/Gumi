// Package policy turns facts into an explicit, deterministic candidate plan.
//
// This is the entire "intelligence" of candidate selection: a handful of
// if-statements over auditable inputs, evaluated fresh on every run. It
// never measures, never learns, never touches the network, and never
// declares a configuration capable or safe — those judgments belong to
// verification (internal/verify), which stays the final authority.
//
// Knowledge categories are kept separate (Phase 7, Task 1):
//
//	A hardware facts   probed machine state (never fabricated)
//	B model facts      GGUF geometry (exact where arithmetic applies)
//	C measured facts   prior controlled experiments; cited as motivation
//	                   for heuristics, never encoded as universal rules
//	D derived policy   the Decisions produced here, each labeled with its
//	                   source so reports can audit why a choice was made
//
// The pipeline position is:
//
//	FACTS -> DETERMINISTIC CONSTRAINTS -> THIS PACKAGE -> MEASURE ->
//	CAPABILITY GATE -> RANK -> REPORT
package policy

import (
	"fmt"
	"sort"
)

// Source labels where a decision's justification comes from.
type Source string

const (
	SourceHardwareFact     Source = "hardware_fact" // A
	SourceModelFact        Source = "model_fact"    // B
	SourceFormula          Source = "deterministic_formula"
	SourceWorkloadContract Source = "workload_contract"
	SourceHeuristic        Source = "heuristic" // D, motivated by C
)

// Impact is the expected leverage of an axis for one model+hardware+workload
// shape. It allocates attention and candidate slots; it is not a measured
// quantity and says nothing about capability.
type Impact string

const (
	ImpactForced Impact = "forced" // feasibility mechanism on this shape, not a tuning axis
	ImpactHigh   Impact = "high"
	ImpactMedium Impact = "medium"
	ImpactLow    Impact = "low"
	ImpactNone   Impact = "none" // no useful interior on this shape; spend no slots
)

// Axis names one tunable execution dimension.
type Axis string

const (
	AxisFlashAttention Axis = "flash_attention"
	AxisContext        Axis = "context"
	AxisKVMemory       Axis = "kv_memory"
	AxisBatch          Axis = "batch"
	AxisOffload        Axis = "offload"
	AxisPlacement      Axis = "expert_placement"
)

// Slot names a conditional candidate slot beyond REFERENCE.
type Slot string

const (
	SlotExpertSplit     Slot = "expert_split"     // MoE placement variant when it changes placement
	SlotContextGrowth   Slot = "context_growth"   // QUALITY: grow context while f16 fits
	SlotKVMemoryRung    Slot = "kv_memory_rung"   // BALANCED: quantized-KV line at floor context
	SlotAggressiveBatch Slot = "aggressive_batch" // SPEED: large batch/ubatch prefill contrast
	SlotHighContextQ8   Slot = "high_context_q8"  // doubled window on q8_0 (capability-risk candidate)
)

// Decision is one explicit policy output with its justification.
type Decision struct {
	Axis   Axis   `json:"axis"`
	Impact Impact `json:"impact"`
	Choice string `json:"choice"`
	Source Source `json:"source"`
	Why    string `json:"why,omitempty"`
}

// Suppression records a slot the policy declined to spend, with the reason.
// Declined slots are as much a decision as admitted ones.
type Suppression struct {
	Slot   Slot   `json:"slot"`
	Reason string `json:"reason"`
}

// Plan is the complete, deterministic output of one Evaluate call.
type Plan struct {
	Decisions  []Decision    `json:"decisions"`
	Slots      []Slot        `json:"slots"` // admitted, priority-ordered
	Suppressed []Suppression `json:"suppressed,omitempty"`
}

// Input carries the separated facts plus deterministic-constraint outcomes.
//
// Fields in the first three blocks are raw facts (categories A/B). The last
// block is computed by exact arithmetic elsewhere (candidate package) so all
// memory formulas live in exactly one place; policy interprets outcomes but
// never does VRAM arithmetic itself.
type Input struct {
	// Model facts (B).
	Architecture    string
	LayerCount      int64
	TrainContext    int64
	MoE             bool
	ExpertShare     float64           // expert bytes / weight bytes; 0 when dense
	KVBytesPerToken map[string]uint64 // exact per precision ("f16","q8_0","q4_0")

	// Hardware facts (A).
	HasGPU            bool
	VRAMBudgetBytes   uint64 // safe planning budget; 0 = unknown/CPU-only
	RAMAvailableBytes uint64 // 0 = unknown

	// Workload contract (declared sensitivity + hard floors).
	Workload     string
	MinContext   int
	PrefillBound bool
	DecodeBound  bool
	DepthBound   bool

	// Deterministic-constraint outcomes (computed from the facts above).
	FullOffloadFeasible   bool    // all layers on GPU at min ctx, f16 KV, within safe budget
	FitHeadroomFraction   float64 // free fraction of budget after full offload at min ctx; negative if over
	SplitChangesPlacement bool    // whitelisted MoE where expert split raises GPU-resident layers
}

// Evaluate produces the plan for one model+hardware+workload shape. Pure
// function of its input: same input, same plan, forever.
func Evaluate(in Input) *Plan {
	p := &Plan{}
	p.add(flashAttention(in))
	p.add(context(in))
	p.add(kvMemory(in))
	p.add(batch(in))
	p.add(offload(in))
	p.add(placement(in))
	p.allocateSlots(in)
	sortDecisions(p.Decisions)
	return p
}

func (p *Plan) add(d Decision) { p.Decisions = append(p.Decisions, d) }

func (p *Plan) admit(s Slot)             { p.Slots = append(p.Slots, s) }
func (p *Plan) decline(s Slot, r string) { p.Suppressed = append(p.Suppressed, Suppression{s, r}) }

// flashAttention: prefer ON everywhere. Motivated by prior measurement and
// by the backend fact that quantized KV requires it; support itself is
// probed from the backend binary at verification time, so an unsupported
// build fails loudly instead of degrading silently.
func flashAttention(in Input) Decision {
	return Decision{
		Axis:   AxisFlashAttention,
		Impact: ImpactHigh,
		Choice: "enable flash attention on every candidate",
		Source: SourceHeuristic,
		Why:    "Flash attention reduces attention memory traffic; prior measurement found no compensating upside for OFF when supported (docs/experiments/04 §2). Quantized KV caches require it. Backend support is probed at verification time.",
	}
}

// context: grow toward feasibility bounds; depth-bound workloads value the
// headroom most because late-window recall converts it into capability.
func context(in Input) Decision {
	if in.TrainContext > 0 && in.TrainContext <= int64(in.MinContext) {
		return Decision{
			Axis: AxisContext, Impact: ImpactNone,
			Choice: fmt.Sprintf("hold at workload minimum (%d); training context leaves no growth room", in.MinContext),
			Source: SourceModelFact,
			Why:    fmt.Sprintf("model training context is %d", in.TrainContext),
		}
	}
	if in.DepthBound {
		return Decision{
			Axis: AxisContext, Impact: ImpactHigh,
			Choice: "grow toward the largest feasible window above the workload minimum",
			Source: SourceWorkloadContract,
			Why:    "depth-bound workload: late-window recall converts context headroom into capability",
		}
	}
	return Decision{
		Axis: AxisContext, Impact: ImpactMedium,
		Choice: "grow moderately via the quality line only",
		Source: SourceHeuristic,
		Why:    "longer sessions help but cost linear memory and a mild decode tax; depth recall is not the binding constraint here",
	}
}

// kvMemory: the quantized-KV rung is always worth one slot (it halves KV
// memory without touching execution behavior), but its expected impact
// scales with memory pressure. Capability of any KV type is decided solely
// by the gate — no precision is presumed good or bad.
func kvMemory(in Input) Decision {
	q4 := in.KVBytesPerToken["q4_0"]
	f16 := in.KVBytesPerToken["f16"]
	if q4 == 0 || q4 == f16 {
		return Decision{
			Axis: AxisKVMemory, Impact: ImpactNone,
			Choice: "no quantized-KV rung (geometry unknown or identical)",
			Source: SourceModelFact,
		}
	}
	if !in.HasGPU {
		return Decision{
			Axis: AxisKVMemory, Impact: ImpactMedium,
			Choice: "include quantized-KV rung (system-RAM savings)",
			Source: SourceHardwareFact,
			Why:    "no discrete GPU: KV cache resides in system RAM, so smaller KV still frees memory",
		}
	}
	if !in.FullOffloadFeasible {
		// need = budget × (1 − headroom) by definition of the headroom
		// fraction; headroom is negative when full offload exceeds budget.
		need := float64(in.VRAMBudgetBytes) * (1 - in.FitHeadroomFraction)
		return Decision{
			Axis: AxisKVMemory, Impact: ImpactForced,
			Choice: "quantized KV is capability-enabling at the workload minimum: f16 does not fit the safe budget",
			Source: SourceFormula,
			Why: fmt.Sprintf("full offload at %d ctx needs %.2f GiB vs safe budget %.2f GiB",
				in.MinContext, gib(need), gib(float64(in.VRAMBudgetBytes))),
		}
	}
	d := Decision{
		Axis:   AxisKVMemory,
		Impact: ImpactMedium,
		Choice: "include quantized-KV rung as the standard memory-efficient operating point",
		Source: SourceHeuristic,
		Why:    "halves KV memory per token; speed/capability effects are stack-specific and decided by measurement, not assumed",
	}
	if in.FitHeadroomFraction >= 0 && in.FitHeadroomFraction < 0.25 {
		d.Impact = ImpactHigh
		d.Why = fmt.Sprintf("tight fit (%.0f%% headroom after full offload at min ctx): the rung doubles as OOM insurance and context headroom", in.FitHeadroomFraction*100)
	}
	return d
}

// batch: vary only when prompt processing dominates the experience.
func batch(in Input) Decision {
	switch {
	case in.PrefillBound && !in.DecodeBound:
		return Decision{
			Axis: AxisBatch, Impact: ImpactHigh,
			Choice: "dedicated large-batch contrast candidate",
			Source: SourceWorkloadContract,
			Why:    "prefill-bound workload: prompt-processing throughput rides on batch/ubatch (prior measurement saw large spreads; magnitude is stack-specific)",
		}
	case in.DecodeBound && !in.PrefillBound:
		return Decision{
			Axis: AxisBatch, Impact: ImpactNone,
			Choice: "hold batches at the baseline on every candidate; spend no slot",
			Source: SourceWorkloadContract,
			Why:    "decode-bound workload: batch sensitivity measured noise-level on the reference stack; short prompts make it irrelevant to experience",
		}
	default:
		return Decision{
			Axis: AxisBatch, Impact: ImpactMedium,
			Choice: "include batch contrast until this workload is classified",
			Source: SourceHeuristic,
			Why:    "unclassified sensitivity keeps conservative coverage",
		}
	}
}

// offload: planning already maximizes GPU residency per candidate by
// arithmetic, so the axis earns a slot only where an interior exists.
func offload(in Input) Decision {
	if !in.HasGPU {
		return Decision{
			Axis: AxisOffload, Impact: ImpactNone,
			Choice: "no offload axis (no discrete GPU)",
			Source: SourceHardwareFact,
		}
	}
	if !in.FullOffloadFeasible {
		return Decision{
			Axis: AxisOffload, Impact: ImpactForced,
			Choice: "maximal feasible offload chosen by arithmetic for every candidate",
			Source: SourceFormula,
			Why:    "weights exceed what fits with minimum-context KV; partial offload is mandatory, and interior levels are not explored — they trade strictly worse decode without identified upside in prior measurement (docs/experiments/04 §2)",
		}
	}
	if in.FitHeadroomFraction < 0.15 {
		return Decision{
			Axis: AxisOffload, Impact: ImpactLow,
			Choice: "keep full offload; memory-efficient rungs hedge OOM risk",
			Source: SourceFormula,
			Why:    fmt.Sprintf("marginal fit (%.0f%% headroom): verification OOM is plausible; no separate variant needed since rungs already reduce pressure", in.FitHeadroomFraction*100),
		}
	}
	return Decision{
		Axis: AxisOffload, Impact: ImpactNone,
		Choice: "full offload comfortable; no slot spent",
		Source: SourceFormula,
		Why:    "reducing offload strictly reduces GPU-resident compute; there is no upside direction to explore",
	}
}

// placement: expert-tensor placement is derived from memory geometry and
// verified family compatibility — never from a hardware name.
func placement(in Input) Decision {
	if !in.MoE {
		return Decision{
			Axis: AxisPlacement, Impact: ImpactNone,
			Choice: "no expert tensors (dense model)",
			Source: SourceModelFact,
		}
	}
	if in.SplitChangesPlacement {
		return Decision{
			Axis: AxisPlacement, Impact: ImpactForced,
			Choice: "place expert tensors in system RAM where it raises GPU-resident layers",
			Source: SourceFormula,
			Why: fmt.Sprintf("MoE geometry: experts are %.0f%% of weights; placement is whitelist-approved for %q and changes the feasible layer count",
				in.ExpertShare*100, in.Architecture),
		}
	}
	return Decision{
		Axis: AxisPlacement, Impact: ImpactNone,
		Choice: "keep default placement (fits, or family not verified for placement)",
		Source: SourceModelFact,
		Why:    "placement is applied only where geometry shows a need AND the family is whitelist-approved; users can apply it manually from exports otherwise",
	}
}

// allocateSlots admits conditional candidate slots in priority order and
// records why declined slots were declined. REFERENCE is implicit and
// always first; the generator enforces the total <= 5 budget.
func (p *Plan) allocateSlots(in Input) {
	batchD := p.find(AxisBatch)
	ctxD := p.find(AxisContext)
	kvD := p.find(AxisKVMemory)

	// 1. Expert split: enabling mechanism when geometry says it matters.
	if in.SplitChangesPlacement {
		p.admit(SlotExpertSplit)
	} else if in.MoE {
		p.decline(SlotExpertSplit, "expert placement does not change the feasible layer count on this shape")
	}

	// 2. Context growth (QUALITY line).
	switch {
	case ctxD == nil:
		p.decline(SlotContextGrowth, "no context decision")
	case ctxD.Impact == ImpactNone:
		p.decline(SlotContextGrowth, ctxD.Choice)
	default:
		p.admit(SlotContextGrowth)
	}

	// 3. KV-memory rung (BALANCED line).
	switch {
	case kvD == nil || kvD.Impact == ImpactNone:
		reason := "quantized KV unavailable or pointless for this geometry"
		if kvD != nil {
			reason = kvD.Choice
		}
		p.decline(SlotKVMemoryRung, reason)
	default:
		p.admit(SlotKVMemoryRung)
	}

	// 4. Aggressive batch (SPEED).
	switch {
	case batchD == nil:
		p.decline(SlotAggressiveBatch, "no batch decision")
	case batchD.Impact == ImpactNone:
		p.decline(SlotAggressiveBatch, batchD.Choice)
	default:
		p.admit(SlotAggressiveBatch)
	}

	// 5. Doubled window on q8_0 — explicitly a capability-risk candidate:
	// included because the workload values depth, verified like any other.
	q8 := in.KVBytesPerToken["q8_0"]
	switch {
	case !in.DepthBound:
		p.decline(SlotHighContextQ8, "workload is not depth-bound; a narrower-KV long-window probe buys little")
	case in.TrainContext > 0 && int64(in.MinContext)*2 > in.TrainContext:
		p.decline(SlotHighContextQ8, "training context cannot host a doubled window")
	case q8 == 0:
		p.decline(SlotHighContextQ8, "q8_0 KV geometry unknown")
	default:
		p.admit(SlotHighContextQ8)
	}
}

func (p *Plan) find(a Axis) *Decision {
	for i := range p.Decisions {
		if p.Decisions[i].Axis == a {
			return &p.Decisions[i]
		}
	}
	return nil
}

var impactRank = map[Impact]int{
	ImpactForced: 0, ImpactHigh: 1, ImpactMedium: 2, ImpactLow: 3, ImpactNone: 4,
}

var axisOrder = map[Axis]int{
	AxisFlashAttention: 0, AxisKVMemory: 1, AxisContext: 2,
	AxisBatch: 3, AxisOffload: 4, AxisPlacement: 5,
}

func sortDecisions(ds []Decision) {
	sort.SliceStable(ds, func(i, j int) bool {
		ri, rj := impactRank[ds[i].Impact], impactRank[ds[j].Impact]
		if ri != rj {
			return ri < rj
		}
		return axisOrder[ds[i].Axis] < axisOrder[ds[j].Axis]
	})
}

// TraceLines renders the human-readable decision trace for reports.
func (p *Plan) TraceLines() []string {
	out := make([]string, 0, len(p.Decisions)+len(p.Slots)+len(p.Suppressed))
	for _, d := range p.Decisions {
		line := fmt.Sprintf("%s [%s|%s]: %s", d.Axis, d.Impact, d.Source, d.Choice)
		if d.Why != "" {
			line += " — " + d.Why
		}
		out = append(out, line)
	}
	for _, s := range p.Slots {
		out = append(out, fmt.Sprintf("slot admitted: %s", s))
	}
	for _, s := range p.Suppressed {
		out = append(out, fmt.Sprintf("slot declined: %s (%s)", s.Slot, s.Reason))
	}
	return out
}

func gib(f float64) float64 { return f / (1 << 30) }
