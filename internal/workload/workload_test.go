package workload

import (
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/verify"
	fixtures "github.com/EffNine/gumi/internal/workload/agentic_coding/tests"
)

func TestProfilesExistAndValid(t *testing.T) {
	for _, name := range Names() {
		p, err := Get(name)
		if err != nil {
			t.Fatalf("Get(%s): %v", name, err)
		}
		if p.MinContext < 2048 {
			t.Errorf("%s: min context %d too small", name, p.MinContext)
		}
		if p.QualityPriority+p.LatencyPriority <= 0.99 || p.QualityPriority+p.LatencyPriority >= 1.01 {
			t.Errorf("%s: priorities do not sum to 1 (%.2f)", name, p.QualityPriority+p.LatencyPriority)
		}
		if len(p.SmokeTasks) == 0 {
			t.Errorf("%s: no smoke tasks", name)
		}
		if len(p.CapabilityTasks) == 0 {
			t.Errorf("%s: no capability tasks", name)
		}
		if p.PerfPromptTokens <= 0 || p.PerfGenTokens <= 0 {
			t.Errorf("%s: perf probe sizes invalid", name)
		}
	}
	if _, err := Get("nonexistent"); err == nil {
		t.Error("unknown profile must error")
	}
}

func TestTaskIDsUniquePerSuite(t *testing.T) {
	for _, name := range Names() {
		p, _ := Get(name)
		seen := map[string]bool{}
		for _, task := range append(append([]verify.Task{}, p.SmokeTasks...), p.CapabilityTasks...) {
			if seen[task.ID] {
				t.Errorf("%s: duplicate task id %s", name, task.ID)
			}
			seen[task.ID] = true
		}
	}
}

func TestGoldenGroupsReferenceRealTasks(t *testing.T) {
	golden := Golden()
	if len(golden) != len(Names()) {
		t.Errorf("golden groups cover %d workloads, want %d", len(golden), len(Names()))
	}
	// Exec fixtures are present only when their toolchain exists locally.
	optional := map[string]bool{}
	for _, id := range fixtures.OptionalTaskIDs() {
		optional[id] = true
	}
	for _, name := range Names() {
		p, _ := Get(name)
		taskIDs := map[string]bool{}
		for _, task := range append(append([]verify.Task{}, p.SmokeTasks...), p.CapabilityTasks...) {
			taskIDs[task.ID] = true
		}
		groups, ok := golden[name]
		if !ok || len(groups) == 0 {
			t.Errorf("%s has no golden groups", name)
			continue
		}
		for _, g := range groups {
			if len(g.TaskIDs) == 0 {
				t.Errorf("%s/%s: empty group", name, g.Name)
			}
			for _, id := range g.TaskIDs {
				if taskIDs[id] {
					continue
				}
				if optional[id] {
					t.Logf("%s/%s: exec fixture %q skipped (toolchain missing)", name, g.Name, id)
					continue
				}
				t.Errorf("%s/%s references unknown task %q", name, g.Name, id)
			}
		}
	}
}

func TestAgenticSuiteIncludesObjectiveFixtures(t *testing.T) {
	p, _ := Get("agentic_coding")
	found := map[string]bool{}
	for _, task := range p.CapabilityTasks {
		switch task.ID {
		case "python_bug_fix", "rust_refactor", "repository_navigation":
			found[task.ID] = true
			built := task.Build(16384)
			if built.Check == nil || len(built.Text) == 0 {
				t.Errorf("%s not bound properly", task.ID)
			}
		}
	}
	// Exec fixtures may be skipped for missing toolchains, but the exact-match
	// navigation fixture has no toolchain dependency and must always exist.
	if !found["repository_navigation"] {
		t.Error("repository_navigation fixture missing from agentic suite")
	}
	for _, id := range []string{"python_bug_fix", "rust_refactor"} {
		if !found[id] {
			t.Logf("note: exec fixture %s skipped (toolchain missing on this machine)", id)
		}
	}
}

func TestSmokeEchoValidatorBound(t *testing.T) {
	p, _ := Get("chat")
	for _, task := range p.SmokeTasks {
		built := task.Build(4096)
		if built.Check == nil {
			t.Fatalf("task %s has no check bound", task.ID)
		}
		switch task.ID {
		case "smoke_echo":
			if err := built.Check("GUMI_SMOKE_OK"); err != nil {
				t.Errorf("echo check rejects valid output: %v", err)
			}
			if err := built.Check("something else entirely"); err == nil {
				t.Error("echo check accepts garbage")
			}
		case "smoke_json":
			if err := built.Check(`{"status":"ok"}`); err != nil {
				t.Errorf("json check rejects valid: %v", err)
			}
		case "smoke_format":
			if err := built.Check("- red\n- green\n- blue"); err != nil {
				t.Errorf("format check rejects valid: %v", err)
			}
		}
	}
}

func TestRetrievalScalesWithContext(t *testing.T) {
	p, _ := Get("agentic_coding")
	var retrieval *verify.Task
	for i := range p.CapabilityTasks {
		if p.CapabilityTasks[i].ID == "retrieval_mid" {
			retrieval = &p.CapabilityTasks[i]
		}
	}
	if retrieval == nil {
		t.Fatal("no retrieval task")
	}
	small := retrieval.Build(4096)
	large := retrieval.Build(32768)
	if len(small.Text) >= len(large.Text) {
		t.Error("haystack must scale with context window")
	}
	if !strings.Contains(large.Text, "access code") {
		t.Error("retrieval prompt missing question")
	}
}

func TestCodingValidators(t *testing.T) {
	p, _ := Get("agentic_coding")
	checks := map[string]struct{ good, bad string }{
		"code_fizzbuzz": {"def fizzbuzz(n):\n    # fizz buzz logic\n    return []", "def solve(n): pass"},
		"math_mult":     {"3901", "1234"},
		"instr_numbered": {
			"1. ITEM a\n2. ITEM b\n3. ITEM c\n4. ITEM d\n5. ITEM e",
			"1. stuff\n2. stuff",
		},
	}
	found := map[string]bool{}
	for _, task := range p.CapabilityTasks {
		built := task.Build(16384)
		if tc, ok := checks[task.ID]; ok {
			found[task.ID] = true
			if err := built.Check(tc.good); err != nil {
				t.Errorf("%s rejects valid output: %v", task.ID, err)
			}
			if err := built.Check(tc.bad); err == nil {
				t.Errorf("%s accepts invalid output", task.ID)
			}
		}
	}
	for id := range checks {
		if !found[id] {
			t.Errorf("task %s missing from agentic suite", id)
		}
	}
}

// Phase 6: the profile contract must be explicit and code-defined.
func TestProfileContractFields(t *testing.T) {
	for _, name := range Names() {
		p, _ := Get(name)
		if strings.TrimSpace(p.Objective) == "" {
			t.Errorf("%s: empty Objective", name)
		}
		if len(p.HardConstraints) == 0 {
			t.Errorf("%s: no HardConstraints", name)
		}
		if len(p.PreferredMetrics) == 0 {
			t.Errorf("%s: no PreferredMetrics", name)
		}
	}
	// Workload-specific weighting is measured, not assumed (Phase 6): the
	// two profiles must not declare identical objectives.
	a, _ := Get("agentic_coding")
	c, _ := Get("chat")
	if a.Objective == c.Objective {
		t.Error("workload objectives must differ")
	}
}

// Sensitivity classification feeds the Phase 7 heuristic policy; it must be
// declared and the two profiles must classify differently (otherwise the
// policy would have nothing workload-specific to act on).
func TestProfileSensitivityDeclared(t *testing.T) {
	a, _ := Get("agentic_coding")
	c, _ := Get("chat")
	if !a.PrefillBound || !a.DepthBound {
		t.Errorf("agentic_coding must be prefill- and depth-bound, got %+v", a)
	}
	if a.DecodeBound {
		t.Error("agentic_coding must not be decode-bound")
	}
	if !c.DecodeBound {
		t.Error("chat must be decode-bound")
	}
	if c.PrefillBound || c.DepthBound {
		t.Errorf("chat must not be prefill/depth-bound, got %+v", c)
	}
}
