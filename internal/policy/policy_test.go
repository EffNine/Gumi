package policy

import (
	"reflect"
	"testing"
)

const GiB = 1 << 30

// roomyDense is a dense model that fits a GPU comfortably at min ctx.
func roomyDense() Input {
	return Input{
		Architecture:        "llama",
		LayerCount:          32,
		TrainContext:        131072,
		KVBytesPerToken:     map[string]uint64{"f16": 128 << 10, "q8_0": 64 << 10, "q4_0": 32 << 10},
		HasGPU:              true,
		VRAMBudgetBytes:     11 * GiB,
		RAMAvailableBytes:   20 * GiB,
		Workload:            "agentic_coding",
		MinContext:          16384,
		PrefillBound:        true,
		DepthBound:          true,
		FullOffloadFeasible: true,
		FitHeadroomFraction: 0.45,
	}
}

func find(t *testing.T, p *Plan, a Axis) Decision {
	t.Helper()
	for _, d := range p.Decisions {
		if d.Axis == a {
			return d
		}
	}
	t.Fatalf("no decision for axis %s", a)
	return Decision{}
}

func hasSlot(p *Plan, s Slot) bool {
	for _, v := range p.Slots {
		if v == s {
			return true
		}
	}
	return false
}

func suppressionReason(p *Plan, s Slot) string {
	for _, v := range p.Suppressed {
		if v.Slot == s {
			return v.Reason
		}
	}
	return ""
}

func TestPrefillDepthBoundRoomyAdmitsBatchAndDepthSlots(t *testing.T) {
	p := Evaluate(roomyDense())

	if d := find(t, p, AxisBatch); d.Impact != ImpactHigh || !hasSlot(p, SlotAggressiveBatch) {
		t.Errorf("prefill-bound workload must prioritize batch: %+v slots=%v", d, p.Slots)
	}
	if d := find(t, p, AxisContext); d.Impact != ImpactHigh || !hasSlot(p, SlotContextGrowth) {
		t.Errorf("depth-bound workload must prioritize context growth: %+v", d)
	}
	if !hasSlot(p, SlotHighContextQ8) {
		t.Error("depth-bound workload with room to double should admit the q8 long-window probe")
	}
	if !hasSlot(p, SlotKVMemoryRung) {
		t.Error("kv rung should be admitted on roomy shapes as the standard ladder point")
	}
	if d := find(t, p, AxisOffload); d.Impact != ImpactNone {
		t.Errorf("comfortable full offload must not spend attention: %+v", d)
	}
	if d := find(t, p, AxisPlacement); d.Impact != ImpactNone {
		t.Errorf("dense model must skip placement: %+v", d)
	}
}

func TestDecodeBoundWorkloadDeclinesBatchSlot(t *testing.T) {
	in := roomyDense()
	in.Workload = "chat"
	in.MinContext = 4096
	in.PrefillBound = false
	in.DecodeBound = true
	in.DepthBound = false

	p := Evaluate(in)

	d := find(t, p, AxisBatch)
	if d.Impact != ImpactNone || hasSlot(p, SlotAggressiveBatch) {
		t.Errorf("decode-bound workload must decline batch variation: %+v", d)
	}
	if reason := suppressionReason(p, SlotAggressiveBatch); reason == "" {
		t.Error("declined slot must record a reason")
	}
	if hasSlot(p, SlotHighContextQ8) {
		t.Errorf("non-depth workload must decline the q8 long-window probe (reason=%q)", suppressionReason(p, SlotHighContextQ8))
	}
	if !hasSlot(p, SlotContextGrowth) {
		t.Error("moderate context growth stays available via the quality line")
	}
}

func TestInfeasibleFullOffloadForcesKVAndPlacement(t *testing.T) {
	in := roomyDense()
	in.Architecture = "qwen3moe"
	in.MoE = true
	in.ExpertShare = 0.87
	in.FullOffloadFeasible = false
	in.FitHeadroomFraction = -0.6 // weights alone exceed budget by ~60%
	in.SplitChangesPlacement = true

	p := Evaluate(in)

	kv := find(t, p, AxisKVMemory)
	if kv.Impact != ImpactForced || kv.Source != SourceFormula {
		t.Errorf("infeasible f16 must force quantized KV via formula: %+v", kv)
	}
	pl := find(t, p, AxisPlacement)
	if pl.Impact != ImpactForced || !hasSlot(p, SlotExpertSplit) {
		t.Errorf("placement that changes feasibility must be forced and admitted: %+v", pl)
	}
	off := find(t, p, AxisOffload)
	if off.Impact != ImpactForced {
		t.Errorf("mandatory partial offload must be labeled forced: %+v", off)
	}
}

func TestUnwhitelistedMoENeverGetsPlacement(t *testing.T) {
	in := roomyDense()
	in.Architecture = "obscuremoe"
	in.MoE = true
	in.ExpertShare = 0.9
	in.FullOffloadFeasible = false
	in.FitHeadroomFraction = -0.5
	in.SplitChangesPlacement = false

	p := Evaluate(in)

	if hasSlot(p, SlotExpertSplit) {
		t.Error("placement slot must require verified family compatibility")
	}
	pl := find(t, p, AxisPlacement)
	if pl.Choice == "" || pl.Source != SourceModelFact && pl.Source != SourceFormula {
		t.Errorf("placement decision must exist and be sourced from facts: %+v", pl)
	}
}

func TestNoGrowthRoomHoldsContext(t *testing.T) {
	in := roomyDense()
	in.TrainContext = int64(in.MinContext)

	p := Evaluate(in)
	d := find(t, p, AxisContext)
	if d.Impact != ImpactNone || d.Source != SourceModelFact {
		t.Errorf("no growth room must hold context by model fact: %+v", d)
	}
	if hasSlot(p, SlotContextGrowth) || hasSlot(p, SlotHighContextQ8) {
		t.Error("no-growth shapes must not admit context slots")
	}
}

func TestTightFitRaisesKVImpact(t *testing.T) {
	in := roomyDense()
	in.FitHeadroomFraction = 0.10

	d := find(t, Evaluate(in), AxisKVMemory)
	if d.Impact != ImpactHigh {
		t.Errorf("tight fit must raise KV-memory impact: %+v", d)
	}
}

func TestEvaluateIsDeterministic(t *testing.T) {
	a := Evaluate(roomyDense())
	b := Evaluate(roomyDense())
	if !reflect.DeepEqual(a, b) {
		t.Error("policy must be a pure function of its input")
	}
	// Slots arrive in priority order; decisions are sorted deterministically.
	ranks := make([]int, 0, len(b.Decisions))
	for _, d := range b.Decisions {
		ranks = append(ranks, impactRank[d.Impact])
	}
	for i := 1; i < len(ranks); i++ {
		if ranks[i] < ranks[i-1] {
			t.Fatalf("decisions not priority-sorted: %v", b.Decisions)
		}
	}
}

func TestEveryDecisionCarriesSourceAndChoice(t *testing.T) {
	in := roomyDense()
	in.MoE = true
	in.ExpertShare = 0.9
	in.SplitChangesPlacement = true
	in.FullOffloadFeasible = false
	valid := map[Source]bool{
		SourceHardwareFact: true, SourceModelFact: true, SourceFormula: true,
		SourceWorkloadContract: true, SourceHeuristic: true,
	}
	for _, d := range Evaluate(in).Decisions {
		if !valid[d.Source] {
			t.Errorf("%s: unknown source %q", d.Axis, d.Source)
		}
		if d.Choice == "" {
			t.Errorf("%s: empty choice", d.Axis)
		}
	}
}

func TestTraceLinesRenderAllOutcomes(t *testing.T) {
	in := roomyDense()
	in.Workload = "chat"
	in.PrefillBound = false
	in.DecodeBound = true
	in.DepthBound = false

	lines := Evaluate(in).TraceLines()
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	for _, want := range []string{"flash_attention [high|heuristic]", "batch [none|workload_contract]", "slot declined: aggressive_batch"} {
		if !contains(joined, want) {
			t.Errorf("trace missing %q:\n%s", want, joined)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
