package candidate

import (
	"fmt"
	"strings"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/policy"
)

// moeSplitWhitelist lists MoE model families for which llama.cpp expert
// tensor placement (`-ot exps=CPU`) is verified to behave correctly.
//
// Expert placement changes execution only — never weights, active expert
// count, or the computation graph — but hardware/driver behavior varies and
// some families require special handling. For architectures outside this
// whitelist Gumi will NOT auto-apply expert split; users can still apply it
// manually from the exported commands.
var moeSplitWhitelist = map[string]bool{
	"qwen2moe":  true,
	"qwen3moe":  true,
	"mixtral":   true,
	"deepseek2": true,
}

// SplitAllowed reports whether automatic expert placement is permitted for
// this model family AND expressible by the discovered backend.
func (g *Generator) SplitAllowed() bool {
	return g.Model.MoE != nil && g.Model.ExpertBytes > 0 &&
		moeSplitWhitelist[strings.ToLower(g.Model.Architecture)]
}

// VRAMBudget returns the conservative safe VRAM budget used by feasibility
// math (total across GPUs × safety factor). Exposed so the pipeline and the
// confidence scorer use the same number as planning.
func (g *Generator) VRAMBudget() uint64 {
	if !g.hasGPU {
		return 0
	}
	return uint64(float64(g.gpuTotal) * vramSafetyFactor)
}

// Generate produces at most five deterministic candidates. REFERENCE is
// always first; the remaining slots come from the evaluated heuristic
// policy (priority order), each builder producing at most one candidate.
// Configurations that converge are dropped with a recorded reason, never
// silently verified twice.
func (g *Generator) Generate() []Candidate {
	ref := g.reference()
	g.finalize(&ref)
	cands := []Candidate{ref}
	for _, slot := range g.plan.Slots {
		var c *Candidate
		switch slot {
		case policy.SlotExpertSplit:
			c = g.expertSplit()
		case policy.SlotContextGrowth:
			c = g.quality()
		case policy.SlotKVMemoryRung:
			c = g.balanced()
		case policy.SlotAggressiveBatch:
			c = g.speed()
		case policy.SlotHighContextQ8:
			c = g.highContext()
		default:
			c = nil
		}
		if c == nil {
			reason := g.builderNote
			g.builderNote = ""
			if reason == "" {
				reason = "inapplicable after feasibility arithmetic"
			}
			g.suppressSlot(slot, reason)
			continue
		}
		if dup := findDuplicateConfig(cands, c.Config); dup != "" {
			g.suppressSlot(slot, fmt.Sprintf("converges on the same configuration as %s", dup))
			continue
		}
		c.Slot = string(slot)
		g.finalize(c)
		cands = append(cands, *c)
	}
	return cands
}

func (g *Generator) suppressSlot(s policy.Slot, reason string) {
	g.plan.Suppressed = append(g.plan.Suppressed, policy.Suppression{Slot: s, Reason: reason})
}

func findDuplicateConfig(cands []Candidate, cfg backend.Config) string {
	for _, c := range cands {
		if c.Config == cfg {
			return c.ID
		}
	}
	return ""
}

// reference is the conservative control every candidate is paired against.
//
// Selection policy: REFERENCE is the highest-confidence quality baseline that
// is feasible on the current hardware — same model, same backend, same
// workload, maximum capability priority (f16 KV cache, greedy decoding),
// stable execution (conservative context, 95% VRAM budget). It is never a
// random default; every relaxation below is recorded in ReferenceWhy.
func (g *Generator) reference() Candidate {
	ctx := g.clampContext(g.Profile.MinContext)
	kv := "f16"
	ngl := g.layersThatFit(ctx, kv, false)
	expertsCPU := false
	splitNote := ""
	if g.placementAllowed() {
		// Prefer whichever placement puts more of the model on GPU; expert
		// split is execution-only and whitelist-approved.
		if splitNgl := g.layersThatFit(ctx, kv, true); splitNgl > ngl {
			expertsCPU = true
			ngl = splitNgl
			splitNote = fmt.Sprintf(
				"Expert tensors placed in system RAM: raises GPU-resident layers from %d to %d (%s family is whitelist-approved for expert placement).",
				g.layersThatFit(ctx, kv, false), ngl, g.Model.Architecture)
		}
	} else if g.Model.MoE != nil && ngl < int(g.Model.LayerCount) {
		splitNote = fmt.Sprintf(
			"Expert split was NOT auto-applied: architecture %q is not in the verified MoE whitelist; "+
				"partial offload keeps execution conservative instead.", g.Model.Architecture)
	}

	offload := "full GPU offload"
	if ngl < int(g.Model.LayerCount) {
		offload = fmt.Sprintf("partial GPU offload (%d/%d layers) as required by memory safety",
			ngl, g.Model.LayerCount)
	}
	why := []string{
		fmt.Sprintf("memory safe: context capped at the workload minimum (%d tokens), f16 KV cache accounted exactly from GGUF geometry, planned within a 95%% VRAM budget", ctx),
		"highest quality settings: f16 KV precision, greedy decoding (temperature 0), fixed seed, flash attention",
		"stable execution: " + offload,
	}
	if splitNote != "" {
		why = append(why, splitNote)
	}
	why = append(why,
		"used for paired comparison: every candidate runs identical prompts and seeds against this configuration")

	c := Candidate{
		ID:   "reference",
		Name: "REFERENCE",
		Kind: KindReference,
		Rationale: "Conservative control selected by policy: highest-confidence quality baseline that is " +
			"feasible on this hardware. All candidates are capability-paired against this configuration.",
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: ctx, KVCacheType: kv,
			FlashAttention: true, Threads: g.threads,
			MMap: true, ExpertsOnCPU: expertsCPU,
			BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
		},
		ReferenceWhy: why,
	}
	if expertsCPU {
		c.Experimental = true
		c.ExperimentalNote = expPlacementNote
	}
	return c
}

// quality maximizes context and KV precision within budget (policy slot:
// context_growth).
func (g *Generator) quality() *Candidate {
	refCtx := g.clampContext(g.Profile.MinContext)
	// Grow toward the training limit while full f16 offload still fits.
	// Whitelisted expert split counts as a legitimate placement for reaching
	// larger f16 contexts — placement is execution-only.
	maxF16 := g.maxContextFor("f16", false)
	splitMode := false
	if g.placementAllowed() {
		if maxSplit := g.maxContextFor("f16", true); maxSplit > maxF16 {
			maxF16 = maxSplit
			splitMode = true
		}
	}
	target := refCtx
	if maxF16 > target {
		grown := refCtx
		for _, step := range []int{2, 4} {
			next := g.clampContext(refCtx * step)
			if next <= maxF16 && next > grown {
				grown = next
			}
		}
		target = grown
	}
	if target == refCtx {
		// No larger f16 window fits: emitting QUALITY would verify a
		// near-duplicate of REFERENCE and waste a slot (Phase 7 budget rule).
		g.builderNote = "context growth infeasible: no larger f16 window fits the safe budget at this geometry"
		return nil
	}
	ctx := target
	ngl := g.layersThatFit(ctx, "f16", splitMode)
	c := Candidate{
		ID:   "quality",
		Name: "QUALITY",
		Kind: KindQuality,
		Rationale: fmt.Sprintf(
			"Maximum fidelity: f16 KV precision with the largest context that fits (%d tokens), %d/%d layers on GPU.",
			ctx, ngl, g.Model.LayerCount),
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: ctx, KVCacheType: "f16",
			FlashAttention: true, Threads: g.threads,
			MMap: true, MLock: g.mlockSafe(), ExpertsOnCPU: splitMode,
			BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
		},
	}
	markExperimental(&c)
	return &c
}

// balanced is the memory-efficient rung: quantized KV at the workload
// context (policy slot: kv_memory_rung). Batches stay at the baseline so
// the contrast against REFERENCE isolates KV precision — one variable per
// candidate keeps gate outcomes attributable.
func (g *Generator) balanced() *Candidate {
	ctx := g.clampContext(g.Profile.MinContext)
	kv := "q4_0"
	switch {
	case !g.kvAllowed(kv) && g.kvAllowed("q8_0"):
		kv = "q8_0"
	case !g.kvAllowed(kv):
		g.builderNote = "quantized-KV rung suppressed: backend build lists no supported quantized cache types"
		return nil
	}
	expertsCPU := g.expertSplitPreferred()
	ngl := g.layersThatFit(ctx, kv, expertsCPU)
	if ngl == 0 && !expertsCPU && g.placementAllowed() {
		expertsCPU = true
		ngl = g.layersThatFit(ctx, kv, expertsCPU)
	}
	name := "BALANCED"
	rationale := "Memory-efficient rung: q4_0 KV cache (requires flash attention) at the workload context. " +
		"Any KV precision is presumed risky until the capability gate clears it. " +
		"Batches held at baseline so the contrast against REFERENCE isolates KV precision."
	if expertsCPU {
		rationale += " Expert tensors placed in system RAM."
	}
	c := Candidate{
		ID:        "balanced",
		Name:      name,
		Kind:      KindBalanced,
		Rationale: rationale,
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: ctx, KVCacheType: kv,
			FlashAttention: true, Threads: g.threads,
			MMap: true, ExpertsOnCPU: expertsCPU,
			BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
		},
	}
	markExperimental(&c)
	return &c
}

// speed is the aggressive prefill contrast (policy slot: aggressive_batch):
// large batch/ubatch on top of the memory-efficient rung — accepted only if
// the capability gate passes. Decode-bound workloads never spend a slot here;
// the policy declines the slot and this builder is not called.
func (g *Generator) speed() *Candidate {
	ctx := g.clampContext(g.Profile.MinContext)
	kv := "q4_0"
	switch {
	case !g.kvAllowed(kv) && g.kvAllowed("q8_0"):
		kv = "q8_0"
	case !g.kvAllowed(kv):
		g.builderNote = "speed line suppressed: backend build lists no supported quantized cache types"
		return nil
	}
	expertsCPU := g.expertSplitPreferred()
	ngl := g.layersThatFit(ctx, kv, expertsCPU)
	if ngl == 0 && !expertsCPU && g.placementAllowed() {
		expertsCPU = true
		ngl = g.layersThatFit(ctx, kv, expertsCPU)
	}
	c := Candidate{
		ID:   "speed",
		Name: "SPEED",
		Kind: KindSpeed,
		Rationale: "Aggressive prefill point: large batch/ubatch on top of the memory-efficient rung. " +
			"Rejected automatically if capability verification fails.",
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: ctx, KVCacheType: kv,
			FlashAttention: true, Threads: g.threads,
			MMap: true, ExpertsOnCPU: expertsCPU,
			BatchSize: 4096, UBatchSize: 2048, Seed: verificationSeed,
		},
	}
	markExperimental(&c)
	return &c
}

// highContext probes a doubled window on q8_0 KV (policy slot:
// high_context_q8). It exists only for depth-bound workloads and is framed
// as what it is: a candidate with known capability risk that must clear the
// paired gate like everything else. No precision is presumed good or bad.
func (g *Generator) highContext() *Candidate {
	ctxMin := g.clampContext(g.Profile.MinContext)
	hiCtx := g.clampContext(ctxMin * 2)
	if hiCtx <= ctxMin || !g.kvAllowed("q8_0") || g.maxContextFor("q8_0", false) < hiCtx {
		return nil
	}
	ngl := g.layersThatFit(hiCtx, "q8_0", false)
	if ngl <= 0 && g.hasGPU {
		return nil
	}
	c := Candidate{
		ID:   "highctx",
		Name: "HIGH-CONTEXT",
		Kind: KindQuality,
		Rationale: fmt.Sprintf(
			"Doubled context (%d tokens) on q8_0 KV cache: included deliberately as a capability-risk "+
				"candidate — narrower KV types have historically failed late-window recall on some stacks — "+
				"and kept only if paired verification holds.",
			hiCtx),
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: hiCtx, KVCacheType: "q8_0",
			FlashAttention: true, Threads: g.threads,
			MMap: true, BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
		},
	}
	return &c
}

// expertSplit is the MoE placement variant (policy slot: expert_split).
func (g *Generator) expertSplit() *Candidate {
	if !g.placementAllowed() || !g.hasGPU || g.Model.MoE == nil || g.Model.ExpertBytes == 0 {
		return nil
	}
	ctxMin := g.clampContext(g.Profile.MinContext)
	splitNgl := g.layersThatFit(ctxMin, "f16", true)
	if splitNgl <= 0 {
		return nil
	}
	c := Candidate{
		ID:   "split",
		Name: "EXPERT-SPLIT",
		Kind: KindBalanced,
		Rationale: fmt.Sprintf(
			"MoE placement: expert weights in system RAM frees %.1f GB of VRAM for layers+KV.",
			float64(g.Model.ExpertBytes)/(1<<30)),
		Config: backend.Config{
			GPULayers: splitNgl, ContextTokens: ctxMin, KVCacheType: "f16",
			FlashAttention: true, Threads: g.threads,
			MMap: true, ExpertsOnCPU: true,
			BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
		},
	}
	markExperimental(&c)
	return &c
}

// FrontierCandidate builds one exploratory operating point on the context
// frontier sweep line. The pipeline owns the sweep; this constructor only
// packages a config with the shared verification contract (seed, greedy
// decoding, flash attention) and runs the same feasibility arithmetic as
// every other candidate.
func (g *Generator) FrontierCandidate(id, name string, ctx int, kv string, expertsCPU bool) Candidate {
	ngl := g.layersThatFit(ctx, kv, expertsCPU)
	c := Candidate{
		ID:   id,
		Name: name,
		Kind: KindQuality,
		Slot: "context_frontier",
		Rationale: fmt.Sprintf(
			"Context frontier probe at %d tokens on %s KV%s: measured to locate the practical boundary.",
			ctx, kv, placementSuffix(expertsCPU)),
		Config: backend.Config{
			GPULayers: ngl, ContextTokens: ctx, KVCacheType: kv,
			FlashAttention: true, Threads: g.threads,
			MMap: true, ExpertsOnCPU: expertsCPU,
			BatchSize: 2048, UBatchSize: 512, Seed: verificationSeed,
			Temperature: 0,
		},
		ProbeOnly: true,
	}
	g.finalize(&c)
	return c
}

func placementSuffix(expertsCPU bool) string {
	if expertsCPU {
		return " with expert tensors in system RAM"
	}
	return ""
}

// BaselineCandidate converts a human-provided configuration into a candidate
// that receives identical treatment (feasibility math, measurement, paired
// gating, ranking). Determinism knobs are forced to the shared verification
// values — baselines differ only in execution placement, never sampling.
func (g *Generator) BaselineCandidate(id, name string, cfg backend.Config) Candidate {
	c := Candidate{
		ID:   id,
		Name: name,
		Kind: KindBaseline,
		Rationale: "Human-provided current configuration (competent-user baseline), " +
			"measured and capability-gated identically to generated candidates.",
		Config: cfg,
	}
	if c.Config.Threads <= 0 {
		c.Config.Threads = g.threads
	}
	// Paired-comparison contract: identical sampling across every candidate,
	// human-provided or generated. Temperature 0 makes the seed inert for
	// greedy decoding, but the recorded configuration must not diverge.
	c.Config.Seed = verificationSeed
	c.Config.Temperature = 0
	markExperimental(&c)
	g.finalize(&c)
	return c
}

// markExperimental flags candidates that rely on expert tensor placement so
// reports can label them explicitly instead of silently recommending them.
func markExperimental(c *Candidate) {
	if c.Config.ExpertsOnCPU {
		c.Experimental = true
		c.ExperimentalNote = expPlacementNote
	}
}

// finalize computes predictions and feasibility verdicts.
func (g *Generator) finalize(c *Candidate) {
	kvPerTok := g.KVBytesPerToken(c.Config.KVCacheType)
	ctx := uint64(c.Config.ContextTokens)

	gpuWeights, cpuWeights := g.weightsOnGPU(c.Config.ExpertsOnCPU)
	kvTotal := kvPerTok * ctx
	overhead := g.ComputeOverhead(c.Config.ContextTokens)

	if g.hasGPU {
		totalLayers := uint64(g.Model.LayerCount)
		layers := c.Config.GPULayers
		if layers < 0 {
			layers = 0
		}
		if totalLayers > 0 && uint64(layers) > totalLayers {
			layers = int(totalLayers)
		}
		frac := 0.0
		if totalLayers > 0 {
			frac = float64(layers) / float64(totalLayers)
		}
		gpuResident := uint64(float64(gpuWeights)*frac) + kvTotal + overhead
		c.PredictedVRAM = gpuResident
		c.PredictedRAM = cpuWeights + uint64(float64(gpuWeights)*(1-frac))

		budget := uint64(float64(g.gpuTotal) * vramSafetyFactor)
		if gpuResident > budget {
			c.Feasible = false
			c.InfeasibleReason = fmt.Sprintf(
				"predicted VRAM %.2f GB exceeds safe budget %.2f GB",
				gb(gpuResident), gb(budget))
			return
		}
		// With mmap (the default for every generated config), weights are
		// file-backed and paged on demand — predicted RSS is an upper bound
		// the kernel never requires. Rejecting here vetoed workloads on
		// machines where desktop usage dipped below the bound, despite the
		// first run proving this exact configuration executes (RTX 5070
		// validation surfaced this). mmap-less configs truly need that RAM.
		if !c.Config.MMap && g.Hardware.RAM.AvailableBytes > 0 {
			ramNeed := c.PredictedRAM + ramHeadroomBytes
			if ramNeed > g.Hardware.RAM.AvailableBytes {
				c.Feasible = false
				c.InfeasibleReason = fmt.Sprintf(
					"predicted RAM %.2f GB + headroom exceeds available %.2f GB",
					gb(c.PredictedRAM), gb(g.Hardware.RAM.AvailableBytes))
				return
			}
		}
		c.Feasible = true
		return
	}

	// CPU-only path. Note: maxContextInRAM shapes candidate sizing; a
	// planning-level infeasible flag here would hide the same mmap issue
	// (file-backed GGUF). Treat it as a bearing, not a hard window —
	// measurement decides. Keep the hard check only for the integer in
	// maximize-context sizing, not as a planning veto.
	c.PredictedVRAM = 0
	c.PredictedRAM = gpuWeights + cpuWeights + kvTotal + overhead
	if g.Hardware.RAM.TotalBytes == 0 {
		c.Feasible = true // cannot verify; let measurement decide
		return
	}
	if !c.Config.MMap && g.Hardware.RAM.AvailableBytes > 0 {
		avail := g.Hardware.RAM.AvailableBytes
		if c.PredictedRAM+ramHeadroomBytes > avail {
			c.Feasible = false
			c.InfeasibleReason = fmt.Sprintf(
				"predicted RAM %.2f GB exceeds available %.2f GB",
				gb(c.PredictedRAM), gb(avail))
			return
		}
	}
	c.Feasible = true
}

// expertSplitPreferred returns true for whitelisted MoE models whose expert
// block is a significant share of weights on constrained GPUs.
func (g *Generator) expertSplitPreferred() bool {
	if !g.placementAllowed() || !g.hasGPU {
		return false
	}
	share := float64(g.Model.ExpertBytes) / float64(g.Model.WeightBytes)
	fitsFully := g.maxContextFor("q8_0", false) >= g.Profile.MinContext
	return share > 0.3 && !fitsFully
}

func (g *Generator) mlockSafe() bool {
	avail := g.Hardware.RAM.AvailableBytes
	if avail == 0 {
		return false
	}
	return avail > g.Model.WeightBytes+(uint64(2)<<30)
}

func gb(b uint64) float64 { return float64(b) / (1 << 30) }
