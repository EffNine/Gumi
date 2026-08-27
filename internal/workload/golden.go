// Golden benchmark suite.
//
// The golden set is Gumi's built-in regression benchmark: a small, fast,
// deterministic family of capability checks that must never silently degrade
// as Gumi itself evolves. Every optimization run executes these against both
// the reference and each candidate, so an improvement to planning/verification
// code cannot mask a quality regression in recommended configurations.
//
// Groups (kept deliberately small so Tier 2 completes on consumer hardware):
//
//	agentic_coding:
//	  - python_bug_fix          exec: fix a buggy module; real test run decides
//	  - rust_refactor           exec: fix failing #[test]; rustc+run decides
//	  - repository_navigation   exact: locate a definition across files
//	chat:
//	  - reasoning               logic + arithmetic word problem
//	  - instruction_following   JSON schema + numbered-list constraints
//	  - context_retrieval       seeded haystack needle recall
package workload

// GoldenGroup names one built-in benchmark family.
type GoldenGroup struct {
	Name    string   // group slug, e.g. "python_bug_fix"
	Eval    string   // evaluation style: "exec" | "exact" | "validator"
	TaskIDs []string // verify.Task IDs belonging to this group
}

// Golden returns the built-in benchmark groups per workload name.
func Golden() map[string][]GoldenGroup {
	return map[string][]GoldenGroup{
		"agentic_coding": {
			{Name: "python_bug_fix", Eval: "exec", TaskIDs: []string{"python_bug_fix"}},
			{Name: "rust_refactor", Eval: "exec", TaskIDs: []string{"rust_refactor"}},
			{Name: "repository_navigation", Eval: "exact", TaskIDs: []string{"repository_navigation"}},
			// Dedicated KV/context degradation probe: distractor codes fill
			// the window, the true tag sits at ~97% depth (Experiment 01
			// showed q8_0 KV losing exactly this recall pattern).
			{Name: "kv_degradation_probe", Eval: "validator",
				TaskIDs: []string{"kv_probe_deep", "retrieval_end"}},
			{Name: "multi_file_reasoning", Eval: "exact", TaskIDs: []string{"multi_file_reasoning"}},
			{Name: "bug_diagnosis", Eval: "exact", TaskIDs: []string{"subtle_bug_diagnosis"}},
			{Name: "context_retrieval", Eval: "validator", TaskIDs: []string{"retrieval_mid"}},
			{Name: "instruction_following", Eval: "validator",
				TaskIDs: []string{"instr_numbered", "late_instruction"}},
			{Name: "code_synthesis", Eval: "validator", TaskIDs: []string{"code_fizzbuzz", "math_mult"}},
		},
		"chat": {
			{Name: "reasoning", Eval: "validator", TaskIDs: []string{"reason_logic", "math_speed"}},
			{Name: "instruction_following", Eval: "validator", TaskIDs: []string{"instr_json_schema"}},
			{Name: "context_retrieval", Eval: "validator", TaskIDs: []string{"retrieval_mid"}},
		},
	}
}
