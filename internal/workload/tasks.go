package workload

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/internal/verify"
	fixtures "github.com/EffNine/gumi/internal/workload/agentic_coding/tests"
)

func agenticCoding() *Profile {
	p := &Profile{
		Name:        "agentic_coding",
		Description: "Coding agents: tool calls, long file context, structured output.",
		Objective:   "maximize usable context and prefill throughput at preserved long-context capability",
		HardConstraints: []string{
			"late-window retrieval must not regress vs REFERENCE (retrieval_end, kv_probe_deep)",
			"flash attention enabled (required for quantized KV correctness)",
			"capability gate parity vs REFERENCE on the full battery",
		},
		PreferredMetrics: []string{"prefill tok/s", "context headroom", "late-context retrieval rate"},
		MinContext:       16384,
		QualityPriority:  0.65,
		LatencyPriority:  0.35,
		PrefillBound:     true,
		DepthBound:       true,
		DecodeRetention:  0.75,
		PerfPromptTokens: 1536,
		PerfGenTokens:    160,
		SmokeTasks:       withChecks(smokeSuite()),
		CapabilityTasks:  agenticCapabilitySuite(),
	}
	for _, u := range fixtures.Unavailable() {
		p.Notes = append(p.Notes, "golden task skipped (toolchain missing): "+u)
	}
	return p
}

func chat() *Profile {
	return &Profile{
		Name:        "chat",
		Description: "Interactive chat: instruction following, reasoning, responsiveness.",
		Objective:   "maximize interactive decode responsiveness at preserved instruction quality",
		HardConstraints: []string{
			"smoke formatting/instruction checks must pass",
			"reasoning and instruction-following tasks must not regress vs REFERENCE",
			"context floor sufficient for multi-turn sessions",
		},
		PreferredMetrics: []string{"decode tok/s", "time-to-first-token proxy", "instruction pass rate"},
		MinContext:       4096,
		QualityPriority:  0.55,
		LatencyPriority:  0.45,
		DecodeBound:      true,
		DecodeRetention:  0.85,
		PerfPromptTokens: 256,
		PerfGenTokens:    128,
		SmokeTasks:       withChecks(smokeSuite()),
		CapabilityTasks:  chatCapabilitySuite(),
	}
}

func smokeSuite() []verify.Task {
	return []verify.Task{
		{
			ID: "smoke_echo", Category: "smoke", Tier: verify.TierSmoke, MaxTokens: 768,
			PromptText: "Reply with exactly this single line and nothing else:\nGUMI_SMOKE_OK",
		},
		{
			ID: "smoke_json", Category: "smoke", Tier: verify.TierSmoke, MaxTokens: 768,
			PromptText: `Return a JSON object with the key "status" set to "ok". Output only the JSON object.`,
		},
		{
			ID: "smoke_format", Category: "smoke", Tier: verify.TierSmoke, MaxTokens: 768,
			PromptText: "List exactly three colors, one per line, each line starting with \"- \". No other text.",
		},
	}
}

// taskChecks binds validators to static prompts by ID at startup.
var taskChecks = map[string]verify.CheckFunc{}

func init() {
	taskChecks["smoke_echo"] = verify.AllOf(verify.ContainsFold("GUMI_SMOKE_OK"))
	taskChecks["smoke_json"] = verify.ValidJSONWithKey("status")
	taskChecks["smoke_format"] = verify.BulletList(3)
	taskChecks["code_fizzbuzz"] = verify.AllOf(
		verify.ContainsFold("def fizzbuzz"),
		verify.ContainsFold("fizz"),
		verify.ContainsFold("buzz"),
	)
	taskChecks["math_mult"] = verify.NumericAnswer(47 * 83)
	taskChecks["math_speed"] = verify.NumericAnswer(80)
	taskChecks["reason_logic"] = verify.FinalWord("yes")
	taskChecks["instr_numbered"] = verify.NumberedList(5, "ITEM")
	taskChecks["instr_json_schema"] = schemaCheck
	// Phase 4 hardening fixtures.
	taskChecks["multi_file_reasoning"] = verify.NumericAnswer(30 * 4 / 2)
	taskChecks["subtle_bug_diagnosis"] = verify.FinalWord("net_price_bulk")
}

// WithChecks attaches bound validators to static-prompt tasks.
func withChecks(tasks []verify.Task) []verify.Task {
	out := make([]verify.Task, len(tasks))
	for i, t := range tasks {
		if c, ok := taskChecks[t.ID]; ok && t.PromptFn == nil && t.PromptText != "" {
			text := t.PromptText
			t.PromptFn = func(int) verify.BuiltPrompt {
				return verify.BuiltPrompt{Text: text, Check: c}
			}
		}
		out[i] = t
	}
	return out
}

func schemaCheck(out string) error {
	// Tolerate reasoning prose: prefer fenced or trailing JSON objects before
	// giving up. The last balanced-looking span is tried first because
	// thinking-style models place their answer at the end.
	m, err := decodeJSONObject(out)
	if err != nil {
		s := out
		i := strings.LastIndex(s, "{")
		j := strings.LastIndex(s, "}")
		if i >= 0 && j > i {
			var m2 map[string]any
			if err2 := json.Unmarshal([]byte(s[i:j+1]), &m2); err2 == nil {
				m, err = m2, nil
			}
		}
	}
	if err != nil {
		return err
	}
	name, okName := m["name"]
	items, okItems := m["items"]
	if !okName || !okItems {
		return fmt.Errorf(`JSON must contain "name" (string) and "items" (array of 3)`)
	}
	if _, ok := name.(string); !ok {
		return fmt.Errorf(`"name" must be a string`)
	}
	arr, ok := items.([]any)
	if !ok || len(arr) != 3 {
		return fmt.Errorf(`"items" must be an array of exactly 3 elements`)
	}
	return nil
}

func agenticCapabilitySuite() []verify.Task {
	static := withChecks([]verify.Task{
		{
			ID: "code_fizzbuzz", Category: "code_synthesis", Tier: verify.TierCapability, MaxTokens: 3072,
			PromptText: "Write a Python function fizzbuzz(n) that returns the FizzBuzz sequence from 1 to n as a list of strings. Include the function definition.",
		},
		{
			ID: "retrieval_mid", Category: "context_retrieval", Tier: verify.TierCapability, MaxTokens: 512,
			PromptFn: func(ctx int) verify.BuiltPrompt {
				return verify.BuildHaystack(haystackTokens(ctx), 0.5)
			},
		},
		{
			ID: "retrieval_end", Category: "context_retrieval", Tier: verify.TierCapability, MaxTokens: 512,
			PromptFn: func(ctx int) verify.BuiltPrompt {
				return verify.BuildHaystack(haystackTokens(ctx), 0.95)
			},
		},
		{
			ID: "instr_numbered", Category: "instruction_following", Tier: verify.TierCapability, MaxTokens: 1024,
			PromptText: `Output exactly 5 numbered lines. Line 1 starts with "1." and line 5 starts with "5.". Every line must contain the word ITEM.`,
		},
		{
			ID: "math_mult", Category: "code_synthesis", Tier: verify.TierCapability, MaxTokens: 2048,
			PromptText: "Compute 47*83. Reply with only the number.",
		},
	})
	// Phase 4 hardening fixtures: KV degradation probe, multi-file reasoning,
	// subtle bug diagnosis, late instruction following.
	static = append(static, kvDegradationProbe(), multiFileReasoning(),
		subtleBugDiagnosis(), lateInstructionFollowing())
	// Objective repository fixtures (test execution, no LLM judge). Tasks
	// whose toolchain is missing locally are excluded up front and noted in
	// the profile.
	return append(static, fixtures.Tasks()...)
}

// kvDegradationProbe is a golden long-context fixture purpose-built to catch
// KV-cache/context degradation.
//
// Why it exists: Experiment 01 showed q8_0 KV losing recall for information
// at the very END of a large context while decode throughput looked healthy
// — exactly the failure mode a throughput-only optimizer would miss.
//
// Failure mode detected: loss of exact recall for late-context tokens under
// quantized/evicting KV configurations.
//
// Construction: a seeded distractor field of plausible access codes fills
// ~85% of the configured window; the TRUE code appears once, at ~97% depth.
// Echoing cannot pass (many codes appear in-context); only precise
// late-context recall produces the expected value.
func kvDegradationProbe() verify.Task {
	return verify.Task{
		ID: "kv_probe_deep", Category: "context_retrieval", Tier: verify.TierCapability, MaxTokens: 512,
		PromptFn: func(ctx int) verify.BuiltPrompt {
			return verify.BuildCodeHaystack(haystackTokens(ctx), 0.97)
		},
	}
}

// multiFileReasoning requires connecting values defined in different files of
// an inline mini-repository; distractor constants make single-file shortcuts
// fail. The computed result never appears verbatim in the prompt, so exact
// numeric match is echo-proof.
func multiFileReasoning() verify.Task {
	prompt := "Mini-repository:\n\n" +
		"--- config.py ---\n" +
		"TIMEOUT_SECONDS = 30\n" +
		"LOG_LEVEL = \"info\"\n\n" +
		"--- agent/worker.py ---\n" +
		"# Backoff policy: total wait per failure cycle equals\n" +
		"# TIMEOUT_SECONDS * RETRY_BACKOFF / 2\n" +
		"RETRY_BACKOFF = 4\n\n" +
		"--- deploy.sh ---\n" +
		"SCALE=3\n\n" +
		"Using the formula documented in agent/worker.py together with the value it references from config.py: " +
		"what is the total wait per failure cycle in seconds? Reply with only the number."
	return verify.Task{
		ID: "multi_file_reasoning", Category: "repository_reasoning", Tier: verify.TierCapability, MaxTokens: 2048,
		PromptText: prompt,
	}
}

// subtleBugDiagnosis presents two similar functions plus failing-test output;
// the defect is in the less obvious candidate. The model must NAME the
// defective function — objective exact-match, no editing required.
func subtleBugDiagnosis() verify.Task {
	prompt := "The module below has failing tests.\n\n" +
		"```python\n" +
		"def net_price_standard(price):\n" +
		"    return price * 0.9\n\n" +
		"def net_price_bulk(price):\n" +
		"    # bulk orders: apply discount, then subtract flat processing fee\n" +
		"    return price * 0.9 - 5\n" +
		"```\n\n" +
		"Failing test output:\n" +
		"```\n" +
		"assert net_price_bulk(200) == 175  # got 185.0\n" +
		"assert net_price_standard(200) == 180  # ok\n" +
		"```\n\n" +
		"The intended behavior for bulk orders is: subtract the 5-unit processing fee FIRST, then apply the 10% discount. " +
		"Which function contains the defect? Reply with only the function name."
	return verify.Task{
		ID: "subtle_bug_diagnosis", Category: "bug_diagnosis", Tier: verify.TierCapability, MaxTokens: 2048,
		PromptText: prompt,
	}
}

// lateInstructionFollowing buries a structural constraint after ~60% seeded
// filler. Early filler includes bullet-formatted lines to establish a
// formatting habit the late instruction must override; grading is positional
// (trailing lines) so long-prompt echo residue cannot cause false failures.
// Recalibrated in Phase 5 after every configuration failed the v1 fixture:
// see docs/specs/25-evidence-hardening.md §8.
func lateInstructionFollowing() verify.Task {
	return verify.Task{
		ID: "late_instruction", Category: "instruction_following", Tier: verify.TierCapability, MaxTokens: 512,
		PromptFn: func(ctx int) verify.BuiltPrompt {
			return verify.BuildLateInstructionV2(haystackTokens(ctx)*6/10, []string{"gamma", "beta", "alpha"})
		},
	}
}

func chatCapabilitySuite() []verify.Task {
	return withChecks([]verify.Task{
		{
			ID: "reason_logic", Category: "reasoning", Tier: verify.TierCapability, MaxTokens: 2048,
			PromptText: "All bloops are razzies. All razzies are lazzies. Are all bloops definitely lazzies? Answer yes or no.",
		},
		{
			ID: "math_speed", Category: "reasoning", Tier: verify.TierCapability, MaxTokens: 2048,
			PromptText: "A train travels 60 km in 45 minutes. What is its average speed in km/h? Answer with only the number.",
		},
		{
			ID: "instr_json_schema", Category: "instruction_following", Tier: verify.TierCapability, MaxTokens: 1024,
			PromptText: `Return only a JSON object with keys "name" (a string) and "items" (an array of exactly 3 strings).`,
		},
		{
			ID: "retrieval_mid", Category: "context_retrieval", Tier: verify.TierCapability, MaxTokens: 32,
			PromptFn: func(ctx int) verify.BuiltPrompt {
				return verify.BuildHaystack(haystackTokens(ctx), 0.5)
			},
		},
	})
}

// haystackTokens sizes retrieval documents to ~85% of the candidate's context
// window so the task exercises long-context recall without overflowing.
func haystackTokens(ctx int) int {
	t := ctx * 85 / 100
	switch {
	case t > 24000:
		t = 24000 // keep verification time bounded
	case t < 512:
		t = 512
	}
	return t
}

// decodeJSONObject is shared JSON parsing tolerant of fenced output.
func decodeJSONObject(out string) (map[string]any, error) {
	s := strings.TrimSpace(out)
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		i := strings.Index(s, "{")
		j := strings.LastIndex(s, "}")
		if i < 0 || j <= i {
			return nil, fmt.Errorf("output is not valid JSON")
		}
		if err := json.Unmarshal([]byte(s[i:j+1]), &m); err != nil {
			return nil, fmt.Errorf("output is not valid JSON")
		}
	}
	return m, nil
}
