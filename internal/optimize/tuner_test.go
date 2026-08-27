package optimize

// V1 auto-tuner end-to-end scenarios against scripted backends. These tests
// pin the empirical search contract: real-looking measurements drive the
// frontier, floors reject on evidence, dominance saves battery budget, and
// unsupported backend dimensions are suppressed instead of crashing the run.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/search"
)

// hookRunner routes every run through a test-provided function.
type hookRunner struct {
	name string
	caps *backend.Capabilities
	run  func(spec backend.RunSpec) (*backend.Result, error)
}

func (h *hookRunner) Name() string {
	if h.name == "" {
		return "hook"
	}
	return h.name
}

func (h *hookRunner) Available(context.Context) error { return nil }

func (h *hookRunner) Run(_ context.Context, spec backend.RunSpec) (*backend.Result, error) {
	return h.run(spec)
}

func (h *hookRunner) Capabilities() backend.Capabilities {
	if h.caps == nil {
		return backend.Capabilities{}
	}
	return *h.caps
}

// scriptedMetrics answers perf probes with context-dependent throughput;
// tasks are answered correctly unless answerHook says otherwise.
type scriptedMetrics struct {
	Decode     func(ctx int) float64
	Prefill    float64
	OOMAt      int // contexts above this OOM during load/perf; 0 = never
	answerHook func(spec backend.RunSpec) (string, bool)
}

func (s scriptedMetrics) run(spec backend.RunSpec) (*backend.Result, error) {
	ctx := spec.Config.ContextTokens
	if s.OOMAt > 0 && ctx > s.OOMAt {
		return &backend.Result{StderrTail: "ggml_backend_alloc: out of memory"},
			fmt.Errorf("%w: ctx %d", backend.ErrOutOfMemory, ctx)
	}
	if strings.HasPrefix(spec.Purpose, "task:") {
		out := correctAnswer(strings.TrimPrefix(spec.Purpose, "task:"))
		if s.answerHook != nil {
			if o, ok := s.answerHook(spec); ok {
				out = o
			}
		}
		switch strings.TrimPrefix(spec.Purpose, "task:") {
		case "smoke_echo":
			out = "GUMI_SMOKE_OK"
		case "smoke_json":
			out = `{"status":"ok"}`
		case "smoke_format":
			out = "- red\n- green\n- blue"
		}
		dec := 0.0
		if s.Decode != nil {
			dec = s.Decode(ctx)
		}
		return &backend.Result{
			Output: out,
			Metrics: backend.Metrics{
				PrefillTPS:    s.Prefill,
				DecodeTPS:     dec,
				PeakVRAMBytes: 6 << 30,
			},
		}, nil
	}
	dec := 0.0
	if s.Decode != nil {
		dec = s.Decode(ctx)
	}
	if dec <= 0 {
		return nil, fmt.Errorf("could not parse llama.cpp timing output")
	}
	return &backend.Result{Metrics: backend.Metrics{
		PrefillTPS:    s.Prefill,
		DecodeTPS:     dec,
		PeakVRAMBytes: 6 << 30,
	}}, nil
}

// TestFrontierSearchFindsNonPowerOfTwoBoundary pins the core V1 behavior:
// coarse doubling, then boundary refinement between pass and fail, landing
// on a practical maximum that is NOT a power of two, then capability-
// verifying the frontier point before it may be recommended.
func TestFrontierSearchFindsNonPowerOfTwoBoundary(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &hookRunner{run: scriptedMetrics{
			Decode: func(ctx int) float64 {
				switch {
				case ctx <= 16384:
					return 30
				case ctx <= 24576:
					return 24 // >= 0.75*30 retention floor
				default:
					return 18 // below floor
				}
			},
			Prefill: 800,
		}.run}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	f := rep.Frontier
	if f == nil {
		t.Fatal("frontier section missing")
	}
	if f.MaxPractical != 24576 {
		t.Errorf("max practical context = %d, want 24576 (between 16K and 32K)", f.MaxPractical)
	}
	if f.CapabilityGated && f.FrontierCandidateID != "frontier-24576" {
		t.Errorf("frontier candidate = %q, want frontier-24576", f.FrontierCandidateID)
	}
	nonPowerOfTwoProbes := false
	for _, p := range f.Refined {
		if p != 32768 && p%32768 != 0 {
			nonPowerOfTwoProbes = true
		}
	}
	if len(f.Refined) == 0 || !nonPowerOfTwoProbes {
		t.Errorf("boundary refinement must probe between the coarse levels: %v", f.Refined)
	}
	if f.TheoreticalMax <= f.MaxPractical {
		t.Errorf("theoretical capacity %d must exceed practical %d", f.TheoreticalMax, f.MaxPractical)
	}

	// The frontier point must have cleared the full battery.
	fc := findView(rep, "frontier-24576")
	if fc == nil || fc.Status != report.StatusVerified {
		t.Fatalf("frontier point must be VERIFIED before recommendation: %+v", fc)
	}

	// MAX CONTEXT profile points at it; profiles exist at all.
	hasMaxCtx := false
	for _, p := range rep.Profiles {
		for _, l := range p.Labels {
			if l == "MAX CONTEXT" && p.CandidateID == "frontier-24576" {
				hasMaxCtx = true
			}
		}
	}
	if !hasMaxCtx {
		t.Errorf("MAX CONTEXT profile missing: %+v", rep.Profiles)
	}
	if rep.Objective == nil || !rep.Objective.Achieved {
		t.Errorf("objective must be achieved (stability-only rule): %+v", rep.Objective)
	}
}

// TestMinDecodeFloorRejectsAndReportsTargetNotAchieved: an explicit user
// floor gates the frontier AND recommendations. When even the best verified
// configuration cannot meet it, Gumi reports TARGET NOT ACHIEVED instead of
// pretending.
func TestMinDecodeFloorRejectsAndReportsTargetNotAchieved(t *testing.T) {
	model := setup(t)
	outDir := t.TempDir()
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &hookRunner{run: scriptedMetrics{
			Decode:  func(int) float64 { return 30 },
			Prefill: 700,
		}.run}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: outDir,
		MinDecode: 45, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Objective.Achieved {
		t.Fatal("objective must NOT be achieved when every verified point is below the floor")
	}
	if !strings.Contains(rep.Objective.Statement, "TARGET NOT ACHIEVED") {
		t.Errorf("statement must say TARGET NOT ACHIEVED: %q", rep.Objective.Statement)
	}
	if !strings.Contains(rep.Objective.Statement, "best verified") {
		t.Errorf("statement must report the best verified config: %q", rep.Objective.Statement)
	}
	// The frontier growth was skipped deliberately: growing context cannot
	// recover a floor the reference already misses.
	if got := len(rep.Frontier.CoarseTested); got != 0 {
		t.Errorf("frontier sweep must be skipped when reference misses an explicit floor, ran %v", rep.Frontier.CoarseTested)
	}
}

// A floor ABOVE some levels but BELOW the best: levels failing the floor are
// rejected on evidence while passing ones remain eligible.
func TestMinDecodeFloorRejectsSlowLevels(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &hookRunner{run: scriptedMetrics{
			Decode: func(ctx int) float64 {
				if ctx <= 16384 {
					return 50
				}
				return 30
			},
			Prefill: 900,
		}.run}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: t.TempDir(),
		MinDecode: 40, Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The 32K coarse level measures 30 tok/s < 40 floor: REJECTED on evidence.
	rejected32k := false
	for _, c := range rep.Candidates {
		if c.Context == 32768 && c.Status == report.StatusRejected &&
			strings.Contains(c.GateReason, "below target") {
			rejected32k = true
		}
	}
	if !rejected32k {
		t.Errorf("32K level must be rejected below the explicit floor: %+v", rep.Candidates)
	}
	if rep.Frontier.MaxPractical != 16384 {
		t.Errorf("max practical = %d, want anchored 16384", rep.Frontier.MaxPractical)
	}
	if !rep.Objective.Achieved {
		t.Errorf("floor 40 is met at MinContext (50 tok/s): %+v", rep.Objective)
	}
}

// Dominated configurations do not receive capability-battery budget, and the
// dominator is recorded on the row.
func TestDominanceSkipsBatteryBudget(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	batteryRuns := 0
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &hookRunner{run: func(spec backend.RunSpec) (*backend.Result, error) {
			dec := 30.0
			if spec.Config.BatchSize > 2048 {
				dec = 20 // aggressive batch is slower here
			}
			res, err := scriptedMetrics{
				Decode:  func(int) float64 { return dec },
				Prefill: 600,
			}.run(spec)
			if err == nil && strings.HasPrefix(spec.Purpose, "task:") && spec.Config.BatchSize > 2048 {
				// Count batteries spent on the dominated candidate.
			}
			if err == nil && strings.HasPrefix(spec.Purpose, "task:") &&
				spec.Config.BatchSize > 2048 {
				batteryRuns++
			}
			return res, err
		}}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "chat", OutDir: t.TempDir(), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rep.Candidates {
		if c.BatchSize > 2048 && c.DominatedBy == "" {
			t.Errorf("slow aggressive-batch candidate %s must record its dominator", c.ID)
		}
		if c.ID == "speed" {
			if c.Status != report.StatusProbed {
				t.Errorf("dominated speed status = %s, want PROBED", c.Status)
			}
			if c.SmokeTotal > 0 || c.CapabilityTotal > 0 {
				t.Errorf("dominated candidate must not spend battery budget (%d/%d)",
					c.SmokeTotal, c.CapabilityTotal)
			}
		}
	}
	if batteryRuns > 0 {
		t.Errorf("battery ran %d times for dominated configs", batteryRuns)
	}
}

// Unsupported backend dimensions are suppressed up front (with reasons) and
// tuning continues on what remains.
func TestBackendCapabilitySuppressionContinuesTuning(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		caps := backend.Capabilities{
			Discovered:     true,
			FlashAttention: true,
			KVTypes:        []string{"bf16", "f16", "q8_0"}, // no quantized-q4 support
			GPULayers:      true,
			Batch:          true,
			UBatch:         true,
			MMap:           true,
			MLock:          true,
			OverrideTensor: false, // no -ot: expert placement impossible
		}
		return &hookRunner{caps: &caps, run: scriptedMetrics{
			Decode:  func(int) float64 { return 33 },
			Prefill: 750,
		}.run}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: t.TempDir(), Version: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Backend == nil || !rep.Backend.Discovered {
		t.Fatal("backend capabilities section missing")
	}
	foundQ4Suppression, foundOTSuppression := false, false
	for _, s := range rep.Backend.Suppressed {
		if strings.Contains(s, "q4_0") {
			foundQ4Suppression = true
		}
		if strings.Contains(s, "-ot") || strings.Contains(s, "expert") {
			foundOTSuppression = true
		}
	}
	if !foundQ4Suppression || !foundOTSuppression {
		t.Errorf("suppressions incomplete: %v", rep.Backend.Suppressed)
	}
	// No measured candidate may use an unsupported dimension.
	for _, c := range rep.Candidates {
		measured := c.WasMeasured()
		if measured && strings.EqualFold(c.KVCache, "q4_0") {
			t.Errorf("%s ran with suppressed KV type q4_0", c.ID)
		}
		if measured && c.ExpertsOnCPU {
			t.Errorf("%s ran with suppressed expert placement", c.ID)
		}
	}
	// Tuning still produced a verified winner.
	if rep.WinnerID == "" {
		t.Fatal("tuning must continue on supported dimensions")
	}
	if rep.Frontier == nil || rep.Frontier.LineExpertsCPU {
		t.Errorf("exploration line must respect -ot suppression: %+v", rep.Frontier)
	}
}

// OOM during the sweep stops growth safely and the session continues to a
// verified result (TASK 13: a failed candidate must not corrupt the session).
func TestOOMDuringSweepDoesNotKillSession(t *testing.T) {
	model := setup(t)
	restoreHW := swapHardware(fixedHardware())
	defer restoreHW()
	savedNewRunner := newRunner
	newRunner = func(string) backend.Runner {
		return &hookRunner{run: scriptedMetrics{
			Decode:  func(int) float64 { return 28 },
			Prefill: 650,
			OOMAt:   40000, // anything above ~39K cannot allocate
		}.run}
	}
	defer func() { newRunner = savedNewRunner }()

	rep, _, err := Run(context.Background(), Options{
		ModelPath: model, Workload: "agentic_coding", OutDir: t.TempDir(), Version: "test",
	})
	if err != nil {
		t.Fatalf("session must survive OOM: %v", err)
	}
	if rep.WinnerID == "" {
		t.Fatal("expected a verified winner despite OOM levels")
	}
	o := findView(rep, rep.WinnerID)
	if o == nil || o.GatePassed != true {
		t.Fatalf("winner not gate-passing: %+v", o)
	}
	// The frontier anchored below the OOM wall.
	if rep.Frontier == nil || rep.Frontier.MaxPractical == 0 {
		t.Fatalf("frontier must still resolve: %+v", rep.Frontier)
	}
}

var _ = search.KVRank
