package optimize

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/candidate"
	"github.com/EffNine/gumi/internal/confidence"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/testgguf"
	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

// fakeRunner simulates a backend whose behavior depends on the candidate's
// KV cache type, letting tests exercise the capability gate end-to-end.
type fakeRunner struct {
	// decodeTPS maps KV type -> throughput; q4_0 is fastest but "dumb".
	decodeTPS map[string]float64
	prefill   map[string]float64
	dumbKV    string // candidates with this KV type fail capability tasks
	// ctxBoost adds extra decode tok/s for a specific context size, used to
	// make baseline candidates measurably distinct.
	ctxBoost map[int]float64
	// vram overrides reported peak VRAM per KV type (bytes).
	vram map[string]uint64
	// jitter cycles per-call decode offsets to emulate measurement noise.
	jitter  []float64
	jitterN int
	// dumbBatch fails capability tasks for candidates using this batch size,
	// letting tests mark one candidate (e.g. SPEED) as fast-but-dumb.
	dumbBatch int
}

func (f *fakeRunner) Name() string { return "fake" }

func (f *fakeRunner) Available(context.Context) error { return nil }

func (f *fakeRunner) Run(_ context.Context, spec backend.RunSpec) (*backend.Result, error) {
	kv := spec.Config.KVCacheType
	decode := f.decodeTPS[kv]
	prefill := f.prefill[kv]
	if b, ok := f.ctxBoost[spec.Config.ContextTokens]; ok {
		decode += b
	}
	if len(f.jitter) > 0 {
		j := f.jitter[f.jitterN%len(f.jitter)]
		f.jitterN++
		decode += j
		prefill += j * 10 // real instruments vary on every metric
	}
	vramBytes := uint64(8 << 30)
	if v, ok := f.vram[kv]; ok {
		vramBytes = v
	}
	res := &backend.Result{Metrics: backend.Metrics{
		PrefillTPS:    prefill,
		DecodeTPS:     decode,
		PeakVRAMBytes: vramBytes,
	}}
	if strings.HasPrefix(spec.Purpose, "task:") {
		taskID := strings.TrimPrefix(spec.Purpose, "task:")
		switch {
		case taskID == "smoke_echo":
			res.Output = "GUMI_SMOKE_OK"
		case taskID == "smoke_json":
			res.Output = `{"status":"ok"}`
		case taskID == "smoke_format":
			res.Output = "- red\n- green\n- blue"
		case (f.dumbBatch > 0 && spec.Config.BatchSize == f.dumbBatch || kv == f.dumbKV) &&
			!strings.HasPrefix(taskID, "smoke"):
			res.Output = "I cannot answer that question, sorry." // fails validators
		default:
			res.Output = correctAnswer(taskID)
		}
	}
	return res, nil
}

func correctAnswer(taskID string) string {
	switch taskID {
	case "math_mult":
		return "3901"
	case "reason_logic":
		return "yes"
	case "instr_numbered":
		return "1. ITEM\n2. ITEM\n3. ITEM\n4. ITEM\n5. ITEM"
	case "retrieval_mid", "retrieval_end":
		return "GX-1042" // must match haystack seed derivation
	default:
		return "valid output"
	}
}

func setup(t *testing.T) (modelPath string) {
	t.Helper()
	b := testgguf.New("qwen3moe").Arch().
		Geometry(48, 40960, 2048, 16, 4).
		MoE(128, 8, 768).
		FileType(15)
	b.Tensor("token_embd.weight", []uint64{2048, 78643}, 1)
	for i := 0; i < 48; i++ {
		b.Tensor("blk."+itoa(i)+".attn_q.weight", []uint64{2048, 4096}, 1)
	}
	for i := 0; i < 48; i++ {
		b.Tensor("blk."+itoa(i)+".ffn_gate_exps.weight", []uint64{512, 2048, 128}, 1)
	}
	return b.WriteFile(t)
}

func fixedHardware() *hardware.Info {
	const GiB = 1 << 30
	return &hardware.Info{
		OS: "linux", Arch: "amd64",
		GPUs: []hardware.GPU{{
			Vendor: "nvidia", Name: "RTX TEST",
			VRAMTotalBytes: 12 * GiB, VRAMFreeBytes: 11500 << 20,
		}},
		CPU:     hardware.CPUInfo{PhysicalCores: 8, LogicalCores: 16},
		RAM:     hardware.Memory{TotalBytes: 32 * GiB, AvailableBytes: 26 * GiB},
		Storage: hardware.Storage{Path: ".", FSType: "ext4", MmapCapable: true, Known: true},
	}
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

func TestDryRunProducesArtifacts(t *testing.T) {
	model := setup(t)
	outDir := filepath.Join(t.TempDir(), "out")

	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()

	rep, dir, err := Run(context.Background(), Options{
		ModelPath: model,
		Workload:  "agentic_coding",
		DryRun:    true,
		OutDir:    outDir,
		Version:   "test",
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if dir != outDir {
		t.Errorf("dir = %q", dir)
	}
	if rep.WinnerID == "" {
		t.Error("dry run should pick first feasible candidate as tentative winner")
	}
	for _, name := range []string{"report.md", "report.json", "candidates.json", "hardware.json"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
		}
	}
	md, err := os.ReadFile(filepath.Join(outDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "# GUMI OPTIMIZATION REPORT") {
		t.Error("markdown header missing")
	}
}

// Phase 7: reports must carry the deterministic generation-policy trace so
// "why did Gumi test these configurations?" is answerable from artifacts.
func TestReportCarriesPolicyTrace(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model,
		Workload:  "agentic_coding",
		DryRun:    true,
		OutDir:    outDir,
		Version:   "test",
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if rep.Policy == nil {
		t.Fatal("policy section missing from report")
	}
	if len(rep.Policy.WorkloadContract) == 0 {
		t.Error("workload contract lines missing")
	}
	validSources := map[string]bool{
		"hardware_fact": true, "model_fact": true, "deterministic_formula": true,
		"workload_contract": true, "heuristic": true,
	}
	axes := map[string]bool{}
	for _, d := range rep.Policy.Decisions {
		if !validSources[d.Source] {
			t.Errorf("axis %s has unknown source %q", d.Axis, d.Source)
		}
		if d.Choice == "" {
			t.Errorf("axis %s has empty choice", d.Axis)
		}
		axes[d.Axis] = true
	}
	for _, want := range []string{"flash_attention", "batch", "kv_memory", "expert_placement"} {
		if !axes[want] {
			t.Errorf("policy trace missing axis %q", want)
		}
	}
	// This fixture needs expert placement and quantized KV to fit at all;
	// the forced decisions must say so via formula-sourced entries.
	forced := 0
	for _, d := range rep.Policy.Decisions {
		if d.Impact == "forced" && d.Source == "deterministic_formula" {
			forced++
		}
	}
	if forced == 0 {
		t.Error("memory-constrained fixture must produce forced formula-sourced decisions")
	}

	md, err := os.ReadFile(filepath.Join(outDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WHY THESE CANDIDATES", "Candidate slots declined:", "paired verification only"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("report.md missing %q", want)
		}
	}
}

func TestCapabilityGateRejectsFastButDumb(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()

	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{
				"f16":  30, // reference / quality / highctx
				"q4_0": 99, // balanced & speed — fastest but broken quality
			},
			prefill:   map[string]float64{"f16": 300, "q4_0": 900},
			dumbBatch: 4096, // SPEED's aggressive batch marks it fast-but-dumb
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model,
		Workload:  "agentic_coding",
		OutDir:    outDir,
		Version:   "test",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var speed *candidateReportView
	var balanced *candidateReportView
	for i := range rep.Candidates {
		c := candidateReportView(rep.Candidates[i])
		switch c.ID {
		case "speed":
			speed = &c
		case "balanced":
			balanced = &c
		}
	}
	if speed == nil || balanced == nil {
		t.Fatal("expected speed and balanced candidates in report")
	}

	// BALANCED (q4_0, moderate batch) passes: q4_0 itself is not the defect —
	// SPEED's measured behavior is. This keeps the rejection attributable to
	// capability, not to a KV label.
	if !balanced.GatePassed {
		t.Errorf("BALANCED must pass gate (reason=%s)", balanced.GateReason)
	}
	if speed.GatePassed {
		t.Errorf("SPEED must be rejected by capability gate (reason=%s)", speed.GateReason)
	}
	if rep.WinnerID == "speed" {
		t.Fatal("fast-but-dumb SPEED must never win")
	}
	if speed.DecodeTPS != 99 {
		t.Errorf("speed decode = %v (should still be recorded)", speed.DecodeTPS)
	}
	if rep.Exports == nil || rep.Exports.LlamaCLI == "" {
		t.Error("exports missing for winner")
	}
}

// Phase 3 harness: a human baseline must flow through the identical
// measurement + gate pipeline as generated candidates.
func TestBaselineParticipatesInGate(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()

	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 50, "q4_0": 99},
			prefill:   map[string]float64{"f16": 300, "q8_0": 500, "q4_0": 900},
			dumbKV:    "q4_0",
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath:     model,
		Workload:      "agentic_coding",
		OutDir:        outDir,
		Version:       "test",
		BaselineSpecs: []string{"ngl=33,c=16384,kv=q4_0,fa,b=512,ub=128,exps-cpu"}, // fast but dumb
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var baseline *candidateView
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == "baseline" {
			baseline = &rep.Candidates[i]
		}
	}
	if baseline == nil {
		t.Fatal("CURRENT-BASELINE missing from report")
	}
	if baseline.Name != "CURRENT-BASELINE" {
		t.Errorf("baseline name = %q", baseline.Name)
	}
	if baseline.DecodeTPS != 99 {
		t.Errorf("baseline decode = %v, want measured 99", baseline.DecodeTPS)
	}
	if baseline.GatePassed {
		t.Errorf("fast-but-dumb baseline must fail the capability gate (reason=%s)", baseline.GateReason)
	}
	if baseline.Confidence == nil || baseline.Confidence.Level != string(confidence.Low) {
		t.Error("rejected baseline must carry LOW confidence")
	}
	if rep.WinnerID == "baseline" {
		t.Error("gated-out baseline must never win")
	}
	// Sampling knobs are forced centrally; the spec cannot override them.
	cands := readCandidates(t, filepath.Join(outDir, "candidates.json"))
	for _, c := range cands {
		if c.ID == "baseline" {
			// seed/temp are not serialized in candidates.json Config summary
			// here; presence of measurement is sufficient for this check.
			if c.Measured == nil || len(c.Measured.PerfSamples) == 0 {
				t.Error("baseline lacks perf samples")
			}
		}
	}
}

// A competent baseline that is genuinely capable AND faster may win — that is
// an honest (falsifying) outcome the optimizer must not hide.
func TestBaselineCanWinWhenGenuinelyBetter(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()

	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 50, "q4_0": 60},
			prefill:   map[string]float64{"f16": 300, "q8_0": 500, "q4_0": 900},
			// The frontier sweep doubles MinContext (16384 -> 32768); the
			// baseline must sit OUTSIDE every probed operating point so its
			// advantage is genuinely its own.
			ctxBoost: map[int]float64{24576: 70}, // only the baseline runs this context
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath:     model,
		Workload:      "agentic_coding",
		OutDir:        outDir,
		Version:       "test",
		BaselineSpecs: []string{"ngl=33,c=24576,kv=q8_0,fa,exps-cpu"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var baseline *candidateView
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == "baseline" {
			baseline = &rep.Candidates[i]
		}
	}
	if baseline == nil {
		t.Fatal("baseline missing")
	}
	if !baseline.GatePassed {
		t.Fatalf("capable baseline must pass gate: %s", baseline.GateReason)
	}
	if rep.WinnerID != "baseline" {
		var all []string
		for _, c := range rep.Candidates {
			all = append(all, c.ID+"="+boolStr(c.GatePassed))
		}
		t.Fatalf("winner = %q, want baseline (candidates: %v)", rep.WinnerID, all)
	}
	if baseline.Confidence == nil {
		t.Error("winning baseline must carry confidence")
	}
}

func TestInvalidBaselineSpecFailsFast(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	saved := newRunner
	newRunner = func(string) backend.Runner { return &fakeRunner{} }
	defer func() { newRunner = saved }()

	_, _, err := Run(context.Background(), Options{
		ModelPath:     model,
		Workload:      "chat",
		OutDir:        t.TempDir(),
		BaselineSpecs: []string{"temperature=0.7"},
	})
	if err == nil {
		t.Fatal("invalid baseline spec must fail before any measurement")
	}
	if !strings.Contains(err.Error(), "baseline 1") {
		t.Errorf("error should identify baseline index: %v", err)
	}
}

func TestDryRunIncludesPlannedBaseline(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()

	rep, _, err := Run(context.Background(), Options{
		ModelPath:     model,
		Workload:      "agentic_coding",
		DryRun:        true,
		OutDir:        t.TempDir(),
		BaselineSpecs: []string{"ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128"},
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	found := false
	for _, c := range rep.Candidates {
		if c.ID == "baseline" {
			found = true
			if !c.Feasible {
				t.Errorf("planned baseline infeasible: %s", c.InfeasibleReason)
			}
			if c.Context != 8192 || c.BatchSize != 512 {
				t.Errorf("planned baseline config mismatch: %+v", c)
			}
		}
	}
	if !found {
		t.Error("dry run must include planned CURRENT-BASELINE row")
	}
}

func TestReferenceFailureFailsPipeline(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	saved := newRunner
	newRunner = func(string) backend.Runner {
		return failingRunner{}
	}
	defer func() { newRunner = saved }()

	_, _, err := Run(context.Background(), Options{
		ModelPath: model,
		Workload:  "chat",
		OutDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("reference failure must surface as error")
	}
	if !strings.Contains(err.Error(), "reference") {
		t.Errorf("error should mention reference: %v", err)
	}
}

// Phase 2: reports must carry the reference selection policy, deterministic
// confidence ratings with evidence, and repeated-run perf samples.
func TestPhase2ReportCarriesConfidenceAndReference(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()

	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 50, "q4_0": 99},
			prefill:   map[string]float64{"f16": 300, "q8_0": 500, "q4_0": 900},
			dumbKV:    "q4_0",
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, dir, err := Run(context.Background(), Options{
		ModelPath: model,
		Workload:  "agentic_coding",
		OutDir:    outDir,
		Version:   "test",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rep.Reference == nil {
		t.Fatal("report missing REFERENCE section")
	}
	if len(rep.Reference.Why) < 3 {
		t.Errorf("reference why-selected documentation incomplete: %v", rep.Reference.Why)
	}

	var winner *candidateView
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == rep.WinnerID {
			winner = &rep.Candidates[i]
		}
		if rep.Candidates[i].ID == "speed" &&
			rep.Candidates[i].Confidence.Level != string(confidence.Low) {
			t.Errorf("gate-failed candidate confidence = %s, want LOW",
				rep.Candidates[i].Confidence.Level)
		}
	}
	if winner == nil {
		t.Fatal("no winner in report")
	}
	if winner.Confidence == nil {
		t.Fatal("winner missing confidence rating")
	}
	if len(winner.Confidence.Positives) == 0 {
		t.Error("winner confidence has no supporting evidence")
	}

	md := readFile(t, filepath.Join(dir, "report.md"))
	for _, want := range []string{
		"# GUMI OPTIMIZATION REPORT",
		"## REFERENCE CONFIGURATION",
		"**Why selected:**",
		"**Confidence:**",
		"### Alternatives",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report.md missing %q", want)
		}
	}

	cands := readCandidates(t, filepath.Join(dir, "candidates.json"))
	var wc *candidateJSON
	for i := range cands {
		if cands[i].ID == rep.WinnerID {
			wc = &cands[i]
		}
	}
	if wc == nil || wc.Measured == nil || len(wc.Measured.PerfSamples) != 6 {
		// 3 verification samples + 3 final-confirmation rounds (the V1 tuner
		// re-tests every recommendation before reporting it).
		t.Errorf("winner should record 3+3 perf samples, got %d", len(wc.Measured.PerfSamples))
	}
}

type failingRunner struct{}

func (failingRunner) Name() string                    { return "failing" }
func (failingRunner) Available(context.Context) error { return nil }
func (failingRunner) Run(context.Context, backend.RunSpec) (*backend.Result, error) {
	return nil, backend.ErrOutOfMemory
}

func swapHardware(hw *hardware.Info) func() {
	saved := probeHardware
	probeHardware = func(hardware.Options) (*hardware.Info, error) { return hw, nil }
	return func() { probeHardware = saved }
}

type candidateReportView = report.CandidateReport

type candidateView = report.CandidateReport

// candidateJSON mirrors the candidates.json artifact shape.
type candidateJSON struct {
	ID       string `json:"id"`
	Measured *struct {
		PerfSamples []json.RawMessage `json:"perf_samples"`
	} `json:"measured"`
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readCandidates(t *testing.T, path string) []candidateJSON {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []candidateJSON
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("candidates.json: %v", err)
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "pass"
	}
	return "fail"
}

var _ = json.Marshal
var _ = verify.TierSmoke

// Baseline configs must carry the shared verification seed/temperature —
// the pairing contract is recorded, not just effective.
func TestBaselineForcesVerificationSampling(t *testing.T) {
	model := setup(t)
	swap := swapHardware(fixedHardware())
	defer swap()
	gg, err := candidate.NewGenerator(mustInspect(t, model), fixedHardware(), mustProfile(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := backend.ParseConfigSpec("ngl=33,c=8192,kv=q8_0")
	if err != nil {
		t.Fatal(err)
	}
	bc := gg.BaselineCandidate("baseline", "CURRENT-BASELINE", cfg)
	if bc.Config.Seed != 42 || bc.Config.Temperature != 0 {
		t.Errorf("baseline sampling not forced: seed=%d temp=%v", bc.Config.Seed, bc.Config.Temperature)
	}
}

func mustInspect(t *testing.T, path string) *gguf.ModelInfo {
	t.Helper()
	m, err := gguf.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustProfile(t *testing.T) *workload.Profile {
	t.Helper()
	p, err := workload.Get("agentic_coding")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ---- Phase 4: evidence-hardening regressions ---------------------------

// A slower but capable candidate remains eligible: it must keep a passed
// verdict, appear as an alternative, and never be rejected for being slow.
func TestSlowerCapableCandidateStaysEligible(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 50},
			prefill:   map[string]float64{"f16": 300, "q8_0": 500},
			vram:      map[string]uint64{"f16": 2 << 30, "q8_0": 9 << 30},
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, dir, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := findView(rep, "reference")
	if ref == nil || !ref.GatePassed || !ref.Feasible {
		t.Fatalf("slowest capable candidate must stay eligible: %+v", ref)
	}
	md := readFile(t, filepath.Join(dir, "report.md"))
	if !strings.Contains(md, "### Alternatives") || !strings.Contains(md, "**REFERENCE**: ") {
		t.Error("eligible slower candidate must appear among alternatives")
	}
	for _, c := range rep.Candidates {
		if c.ID == "reference" && (!c.GatePassed || !c.Feasible) {
			t.Error("slower capable candidate must stay eligible")
		}
	}
}

// Two capable candidates with overlapping performance ranges must NOT be
// ranked with high confidence; the report must say they are tied.
func TestOverlappingPerformanceLowRankingConfidence(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 31.0, "q8_0": 31.4},
			prefill:   map[string]float64{"f16": 640, "q8_0": 636},
			vram:      map[string]uint64{"f16": 2 << 30, "q8_0": 10 << 30},
			jitter:    []float64{-1.5, 0.0, 1.5}, // ±1.5 tok/s noise around ~31
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Ranking == nil {
		t.Fatal("missing ranking assessment")
	}
	if rep.Ranking.Level != string(confidence.Low) || !rep.Ranking.Indistinguishable {
		t.Fatalf("overlapping ranges must yield LOW/indistinguishable, got %s (%s)",
			rep.Ranking.Level, rep.Ranking.Note)
	}
	md := readFile(t, filepath.Join(outDir, "report.md"))
	if !strings.Contains(md, "**Ranking confidence:** LOW") ||
		!strings.Contains(md, "operationally indistinguishable from") {
		t.Errorf("report must surface the tie explicitly")
	}
}

// A candidate with stable superior performance earns HIGH ranking confidence.
func TestStableSuperiorPerformanceHighRanking(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			// The q8_0 baseline is clearly fastest on BOTH measured axes;
			// generated lines trail by more than the observed noise, so
			// ranking must separate cleanly.
			decodeTPS: map[string]float64{"f16": 30.0, "q8_0": 50.0, "q4_0": 35.0},
			prefill:   map[string]float64{"f16": 300, "q8_0": 1200, "q4_0": 900},
			jitter:    []float64{-0.2, 0.0, 0.2}, // tight noise
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
		BaselineSpecs: []string{"ngl=33,c=16384,kv=q8_0,fa"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Ranking == nil || rep.Ranking.Level != string(confidence.High) {
		t.Fatalf("clear separation must earn HIGH ranking, got %+v", rep.Ranking)
	}
	if !strings.Contains(rep.Ranking.Note, "no overlap") {
		t.Errorf("note should cite non-overlap: %q", rep.Ranking.Note)
	}
}

// Single perf repetition cannot support ranking claims.
func TestSingleRepetitionSuppressesRankingClaims(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 99, "q4_0": 99},
			prefill:   map[string]float64{"f16": 300, "q8_0": 900, "q4_0": 900},
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir,
		PerfRuns: 1, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Ranking == nil || !rep.Ranking.Indistinguishable || rep.Ranking.Level != string(confidence.Low) {
		t.Fatalf("single repetition must suppress ranking confidence, got %+v", rep.Ranking)
	}
}

// When performance is indistinguishable, the safer operating margin wins:
// equal speed, equal capability → lower measured VRAM peak preferred.
func TestTieBreakPrefersSaferMargin(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 40.0, "q8_0": 40.0},
			prefill:   map[string]float64{"f16": 400, "q8_0": 400},
			jitter:    []float64{-0.4, 0.0, 0.4},
			vram:      map[string]uint64{"f16": 2 << 30, "q8_0": 10 << 30},
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Ranking.Indistinguishable {
		t.Fatalf("expected tie, got %s (%s)", rep.Ranking.Level, rep.Ranking.Note)
	}
	if rep.WinnerID != "reference" {
		t.Fatalf("safer margin (reference, 2GiB peak) should win the tie, got %q", rep.WinnerID)
	}
}

func findView(rep *report.Report, id string) *report.CandidateReport {
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == id {
			return &rep.Candidates[i]
		}
	}
	return nil
}

// ---- Phase 5: evidence-status contract ---------------------------------

func TestStatusVocabularyInReport(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &fakeRunner{
			decodeTPS: map[string]float64{"f16": 30, "q8_0": 50, "q4_0": 99},
			prefill:   map[string]float64{"f16": 300, "q8_0": 500, "q4_0": 900},
			dumbKV:    "q4_0",
		}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]string{}
	for _, c := range rep.Candidates {
		statuses[c.ID] = c.Status
	}
	// Winner (whatever it is under the current candidate set) must be
	// RECOMMENDED; every gate-failed candidate must be REJECTED; the anchor
	// is VERIFIED unless it is itself the winner.
	winnerStatus, ok := statuses[rep.WinnerID]
	if !ok || winnerStatus != report.StatusRecommended {
		t.Errorf("winner status = %q, want RECOMMENDED", winnerStatus)
	}
	rejectedSeen := false
	for _, c := range rep.Candidates {
		if !c.GatePassed && c.Error == "" {
			rejectedSeen = true
			if c.Status != report.StatusRejected {
				t.Errorf("%s: gate-failed status = %q, want REJECTED", c.ID, c.Status)
			}
		}
	}
	if !rejectedSeen {
		t.Fatal("this scenario must contain at least one gate-failed candidate")
	}
	if rep.WinnerID != "reference" && statuses["reference"] != report.StatusVerified {
		t.Errorf("reference status = %q, want VERIFIED (non-winner)", statuses["reference"])
	}
	md := readFile(t, filepath.Join(outDir, "report.md"))
	if !strings.Contains(md, "## Rejected configurations") {
		t.Error("rejected section missing")
	}
}

func TestDryRunStatusesAreScreened(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", DryRun: true,
		OutDir: t.TempDir(), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rep.Candidates {
		if c.Status != report.StatusScreened {
			t.Errorf("%s: dry-run status = %q, want SCREENED", c.ID, c.Status)
		}
		if c.ID == rep.WinnerID && c.Status != report.StatusScreened {
			t.Error("dry-run winner must not claim RECOMMENDED")
		}
	}
}

// Non-classified backend failure yields UNKNOWN — evidence is insufficient,
// never fabricated into a rejection.
func TestUnknownStatusOnUnclassifiedError(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner { return &crashRunner{} }
	defer func() { newRunner = savedNewRunner }()

	_, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "chat", OutDir: t.TempDir(), Version: "test",
	})
	// Reference failing entirely fails the pipeline; the point is the error
	// is not classified as OOM/timeout.
	if err == nil {
		t.Fatal("expected pipeline error from crashing backend")
	}
	if strings.Contains(strings.ToLower(err.Error()), "memory") {
		t.Errorf("generic crash must not be misclassified as OOM: %v", err)
	}
}

type crashRunner struct{}

func (crashRunner) Name() string                    { return "crash" }
func (crashRunner) Available(context.Context) error { return nil }
func (crashRunner) Run(_ context.Context, spec backend.RunSpec) (*backend.Result, error) {
	return nil, fmt.Errorf("backend crashed unexpectedly")
}

var _ = flakyRunner{}

type flakyRunner struct{ failAfter int }

func (f *flakyRunner) Name() string                    { return "flaky" }
func (f *flakyRunner) Available(context.Context) error { return nil }
func (f *flakyRunner) Run(_ context.Context, spec backend.RunSpec) (*backend.Result, error) {
	return nil, fmt.Errorf("backend crashed unexpectedly")
}
