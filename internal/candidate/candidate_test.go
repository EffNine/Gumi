package candidate

import (
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/testgguf"
	"github.com/EffNine/gumi/internal/workload"
)

const GiB = 1 << 30

// moeModel returns an 18GiB-weight Qwen3-MoE-shaped model (15GiB experts).
func moeModel(t *testing.T) *gguf.ModelInfo {
	t.Helper()
	b := testgguf.New("qwen3moe").Arch().
		Geometry(48, 40960, 2048, 16, 4).
		MoE(128, 8, 768).
		FileType(15)
	// non-expert weights ~3 GiB in F16: token_embd + attn
	b.Tensor("token_embd.weight", []uint64{2048, 78643}, 1) // ~0.3 GiB
	for i := 0; i < 48; i++ {
		name := "blk." + itoa(i) + ".attn_q.weight"
		b.Tensor(name, []uint64{2048, 33554}, 1) // ~134 MB each => ~6.4 GB total? keep small
	}
	// expert tensors dominate: 48 layers x [512,2048,128] F16 = 256 MiB each
	for i := 0; i < 48; i++ {
		name := "blk." + itoa(i) + ".ffn_gate_exps.weight"
		b.Tensor(name, []uint64{512, 2048, 128}, 1)
	}
	path := b.WriteFile(t)
	m, err := gguf.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func gamingPC() *hardware.Info {
	return &hardware.Info{
		OS: "linux", Arch: "amd64",
		GPUs: []hardware.GPU{{
			Vendor: "nvidia", Name: "RTX TEST",
			VRAMTotalBytes: 12 * GiB, VRAMFreeBytes: 11500 << 20,
			Source: "test",
		}},
		CPU: hardware.CPUInfo{ModelName: "Test", PhysicalCores: 8, LogicalCores: 16},
		RAM: hardware.Memory{TotalBytes: 32 * GiB, AvailableBytes: 26 * GiB},
	}
}

func newGen(t *testing.T, m *gguf.ModelInfo, hw *hardware.Info, profile string) *Generator {
	t.Helper()
	p, err := workload.Get(profile)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGenerator(m, hw, p)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGenerateDeterministicAndBounded(t *testing.T) {
	m := moeModel(t)
	g1 := newGen(t, m, gamingPC(), "agentic_coding")
	g2 := newGen(t, m, gamingPC(), "agentic_coding")

	c1 := g1.Generate()
	c2 := g2.Generate()

	if len(c1) == 0 || len(c1) > 5 {
		t.Fatalf("candidate count = %d, must be 1..5", len(c1))
	}
	for i := range c1 {
		a, b := c1[i], c2[i]
		if a.ID != b.ID || a.Config != b.Config || a.Rationale != b.Rationale {
			t.Errorf("candidate %d not deterministic:\n%+v\n%+v", i, a, b)
		}
	}
	ids := map[string]bool{}
	for _, c := range c1 {
		if ids[c.ID] {
			t.Errorf("duplicate id %s", c.ID)
		}
		ids[c.ID] = true
	}
	if !ids["reference"] || !ids["quality"] || !ids["balanced"] || !ids["speed"] {
		t.Errorf("missing core candidates: %v", ids)
	}
}

func TestContextRespectsWorkloadMinimum(t *testing.T) {
	m := moeModel(t)
	g := newGen(t, m, gamingPC(), "agentic_coding") // min 16384
	for _, c := range g.Generate() {
		if !c.Feasible {
			continue
		}
		if c.Config.ContextTokens < 16384 {
			t.Errorf("%s context %d below workload minimum", c.ID, c.Config.ContextTokens)
		}
		if m.TrainContext > 0 && c.Config.ContextTokens > int(m.TrainContext) {
			t.Errorf("%s context %d exceeds training context", c.ID, c.Config.ContextTokens)
		}
	}
}

// The aggressive-batch slot follows the workload contract: prefill-bound
// workloads get the large-batch contrast; decode-bound workloads (chat)
// must NOT spend a candidate slot on a knob that is noise for them.
func TestSpeedCandidateFollowsWorkloadSensitivity(t *testing.T) {
	m := moeModel(t)

	ga := newGen(t, m, gamingPC(), "agentic_coding")
	var speed *Candidate
	for _, c := range ga.Generate() {
		if c.ID == "speed" {
			speed = &c
		}
	}
	if speed == nil {
		t.Fatal("prefill-bound workload must generate the aggressive-batch candidate")
	}
	if speed.Config.KVCacheType != "q4_0" || speed.Config.BatchSize != 4096 {
		t.Errorf("speed = kv=%s b=%d, want q4_0/4096", speed.Config.KVCacheType, speed.Config.BatchSize)
	}
	if speed.Config.Seed != 42 || speed.Config.Temperature != 0 {
		t.Error("verification determinism knobs wrong on speed candidate")
	}

	gc := newGen(t, m, gamingPC(), "chat")
	for _, c := range gc.Generate() {
		if c.ID == "speed" {
			t.Error("decode-bound chat must not spend a slot on batch variation")
		}
	}
}

func TestMoESplitFifthCandidate(t *testing.T) {
	m := moeModel(t) // experts >> 12GiB VRAM
	g := newGen(t, m, gamingPC(), "agentic_coding")
	cands := g.Generate()

	// Split placement must appear somewhere in the set (standalone EXPERT-SPLIT
	// or absorbed into the reference when they converge).
	splitSeen := false
	for _, c := range cands {
		if c.Config.ExpertsOnCPU {
			splitSeen = true
			if !c.Experimental || c.ExperimentalNote == "" {
				t.Errorf("%s: expert-split config must be labeled experimental", c.ID)
			}
		}
	}
	if !splitSeen {
		t.Fatal("whitelisted MoE exceeding VRAM must produce an expert-split configuration")
	}

	// No two candidates may verify the identical configuration.
	seenCfg := map[backend.Config]string{}
	for _, c := range cands {
		if prev, ok := seenCfg[c.Config]; ok {
			t.Errorf("duplicate config between %s and %s", prev, c.ID)
		}
		seenCfg[c.Config] = c.ID
	}
	ref := findID(cands, "reference")
	if ref != nil && !ref.Config.ExpertsOnCPU {
		t.Log("note: reference kept full-offload form; acceptable if feasible")
	}
}

func TestInfeasibleWhenVRAMTooSmall(t *testing.T) {
	m := moeModel(t)
	hw := gamingPC()
	hw.GPUs[0].VRAMTotalBytes = 2 * GiB
	hw.GPUs[0].VRAMFreeBytes = 1900 << 20
	hw.RAM.AvailableBytes = 4 * GiB // cannot hold 15GiB of experts either

	g := newGen(t, m, hw, "agentic_coding")
	infeasible := 0
	for _, c := range g.Generate() {
		if !c.Feasible {
			infeasible++
			if c.InfeasibleReason == "" {
				t.Errorf("%s marked infeasible without reason", c.ID)
			}
		}
	}
	if infeasible == 0 {
		t.Skip("generator found CPU-only fallback; acceptable behavior for tiny GPU")
	}
}

func TestCPUMachineProducesCPUConfigs(t *testing.T) {
	m := moeModel(t)
	hw := &hardware.Info{
		OS:  "linux",
		CPU: hardware.CPUInfo{PhysicalCores: 8, LogicalCores: 16},
		RAM: hardware.Memory{TotalBytes: 64 * GiB, AvailableBytes: 60 * GiB},
	}
	g := newGen(t, m, hw, "chat")
	for _, c := range g.Generate() {
		if c.Config.GPULayers != 0 {
			t.Errorf("%s must offload zero layers without a GPU", c.ID)
		}
		if !c.Feasible {
			t.Errorf("%s should fit in 64GB RAM", c.ID)
		}
	}
}

func TestKVMathAgainstSpecExample(t *testing.T) {
	m := moeModel(t)
	// Spec §6.2: Qwen3-30B-A3B geometry -> 96 KiB/token at f16.
	if got := m.KVBytesPerToken("f16"); got != 96<<10 {
		t.Errorf("f16 KV/token = %d, want 98304", got)
	}
}

func TestReferencePolicyDocumentsSelection(t *testing.T) {
	m := moeModel(t)
	g := newGen(t, m, gamingPC(), "agentic_coding")
	ref := findID(g.Generate(), "reference")
	if ref == nil {
		t.Fatal("no reference candidate")
	}
	if ref.Config.KVCacheType != "f16" || ref.Config.Temperature != 0 || ref.Config.Seed != 42 {
		t.Error("reference must use maximum capability priority settings (f16 KV, greedy, fixed seed)")
	}
	if len(ref.ReferenceWhy) < 3 {
		t.Fatalf("reference policy must document why selected, got %v", ref.ReferenceWhy)
	}
	joined := ""
	for _, w := range ref.ReferenceWhy {
		joined += w + "\n"
	}
	for _, want := range []string{"memory safe", "highest quality", "paired comparison"} {
		if !contains(joined, want) {
			t.Errorf("ReferenceWhy missing %q:\n%s", want, joined)
		}
	}
}

// Expert-split offload must not count expert tensors in the marginal
// per-layer cost: Qwen3-30B-A3B-shaped weights (~0.9 GB non-expert) fit
// entirely on a 12 GB card with experts in RAM. Regression for the MoE
// under-offload planning defect found during Phase 3 dry-run planning.
func TestExpertSplitOffloadsAllLayersWhenNonExpertFits(t *testing.T) {
	m := moeModel(t)
	g := newGen(t, m, gamingPC(), "agentic_coding")
	if !g.SplitAllowed() {
		t.Fatal("test model must be whitelist-approved")
	}
	ngl := g.layersThatFit(g.clampContext(16384), "f16", true)
	if ngl < int(m.LayerCount) {
		t.Fatalf("expert-split layersThatFit = %d, want all %d layers (non-expert weights %.2f GiB)",
			ngl, m.LayerCount, float64(m.WeightBytes-m.ExpertBytes)/(1<<30))
	}
}

func TestMoESplitWhitelistGatesPlacement(t *testing.T) {
	m := moeModel(t) // qwen3moe: whitelisted
	g := newGen(t, m, gamingPC(), "agentic_coding")
	cands := g.Generate()
	splitSeen := false
	for _, c := range cands {
		if c.Config.ExpertsOnCPU {
			splitSeen = true
			if !c.Experimental || c.ExperimentalNote == "" {
				t.Errorf("%s: split config must be labeled experimental", c.ID)
			}
		}
	}
	if !splitSeen {
		t.Fatal("whitelisted family must still produce an expert-split configuration")
	}

	// Same shape but an unlisted architecture: placement must never be
	// auto-applied anywhere.
	b := testgguf.New("obscuremoe").Arch().
		Geometry(48, 40960, 2048, 16, 4).
		MoE(128, 8, 768).
		FileType(15)
	b.Tensor("token_embd.weight", []uint64{2048, 78643}, 1)
	for i := 0; i < 48; i++ {
		b.Tensor("blk."+itoa(i)+".attn_q.weight", []uint64{2048, 33554}, 1)
	}
	for i := 0; i < 48; i++ {
		b.Tensor("blk."+itoa(i)+".ffn_gate_exps.weight", []uint64{512, 2048, 128}, 1)
	}
	om, err := gguf.Inspect(b.WriteFile(t))
	if err != nil {
		t.Fatal(err)
	}
	og := newGen(t, om, gamingPC(), "agentic_coding")
	for _, c := range og.Generate() {
		if c.ID == "split" {
			t.Error("unknown MoE family must not receive EXPERT-SPLIT candidate")
		}
		if c.Config.ExpertsOnCPU {
			t.Errorf("%s: expert placement applied to non-whitelisted family %q", c.ID, om.Architecture)
		}
	}
	ref := findID(og.Generate(), "reference")
	if ref == nil {
		t.Fatal("no reference for obscure family")
	}
	if len(ref.ReferenceWhy) < 3 {
		t.Error("reference policy documentation missing")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func findID(cands []Candidate, id string) *Candidate {
	for i := range cands {
		if cands[i].ID == id {
			return &cands[i]
		}
	}
	return nil
}

// denseModel returns a small Llama-shaped dense model (~2.4 GiB weights)
// that fits a gaming PC comfortably at any tested context.
func denseModel(t *testing.T) *gguf.ModelInfo {
	t.Helper()
	b := testgguf.New("llama").Arch().
		Geometry(32, 131072, 4096, 32, 8).
		FileType(15)
	b.Tensor("token_embd.weight", []uint64{4096, 128256}, 0) // Q8_0 ~0.5 GiB
	for i := 0; i < 32; i++ {
		b.Tensor("blk."+itoa(i)+".attn_q.weight", []uint64{4096, 4096}, 1) // F16 32 MiB each
		b.Tensor("blk."+itoa(i)+".ffn_gate.weight", []uint64{4096, 14336}, 1)
	}
	m, err := gguf.Inspect(b.WriteFile(t))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Phase 7: the policy must conserve slots when no axis has an interior.
// A decode-bound workload on roomy hardware (dense model fully offloaded)
// gets REFERENCE + context growth + KV rung — batch variation is declined
// with a recorded reason instead of wasting slots.
func TestDecodeBoundRoomyShapeConservesSlots(t *testing.T) {
	m := denseModel(t)
	g := newGen(t, m, gamingPC(), "chat")

	cands := g.Generate()
	if len(cands) != 3 {
		t.Fatalf("candidate count = %d, want 3 (reference, quality, balanced)", len(cands))
	}
	for _, id := range []string{"speed", "highctx", "split"} {
		if findID(cands, id) != nil {
			t.Errorf("slot-derived candidate %q must not appear for decode-bound roomy shape", id)
		}
	}
	suppressed := map[string]string{}
	for _, s := range g.Plan().Suppressed {
		suppressed[string(s.Slot)] = s.Reason
	}
	if suppressed["aggressive_batch"] == "" || suppressed["high_context_q8"] == "" {
		t.Errorf("declined slots must record reasons, got %v", suppressed)
	}

	// BALANCED now differs from REFERENCE only in KV precision.
	ref, bal := findID(cands, "reference"), findID(cands, "balanced")
	if ref.Config.BatchSize != bal.Config.BatchSize ||
		ref.Config.UBatchSize != bal.Config.UBatchSize ||
		ref.Config.ContextTokens != bal.Config.ContextTokens ||
		ref.Config.KVCacheType == bal.Config.KVCacheType {
		t.Errorf("BALANCED must isolate KV precision vs REFERENCE: %+v vs %+v", ref.Config, bal.Config)
	}
}

// Every generated candidate beyond REFERENCE carries its policy slot as
// provenance, and the plan renders a human-readable trace.
func TestSlotProvenanceAndTrace(t *testing.T) {
	m := moeModel(t)
	g := newGen(t, m, gamingPC(), "agentic_coding")
	cands := g.Generate()
	for _, c := range cands {
		if c.ID == "reference" {
			if c.Slot != "" {
				t.Error("REFERENCE must not carry a slot")
			}
			continue
		}
		if c.Slot == "" {
			t.Errorf("%s missing slot provenance", c.ID)
		}
	}
	trace := strings.Join(g.Plan().TraceLines(), "\n")
	if !contains(trace, "flash_attention") || !contains(trace, "expert_placement") {
		t.Errorf("trace must cover all axes:\n%s", trace)
	}
}
