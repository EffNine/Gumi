# Phase 5 — Capability Fixture Audit

Scope: every agentic_coding capability task, audited for what it tests,
what failure modes it detects, evaluator objectivity, and measured
discriminating power (Experiment 02 battery + generalization runs).

SIGNAL > DIFFICULTY: a task all configurations pass is weak evidence; a task
all fail is equally weak. Discriminating tasks are the payload.

---

## Execution / repository fixtures (`internal/workload/agentic_coding/tests`)

| Task | Capability tested | Failure modes detected | Evaluator | Can degraded inference fail it? | Strong model passes reliably? | Discrimination observed |
|---|---|---|---|---|---|---|
| `python_bug_fix` | code synthesis + instruction adherence (full corrected file, no prose) | broken code generation, format-instruction blindness | real test execution (`python3 test_calculator.py` exit status) after fence extraction | yes — syntax errors/wrong logic fail the suite | yes (Qwen3-30B passed once echo handling was fixed) | passed by every verified config so far; guards gross regressions |
| `rust_refactor` | same as above under a compiler + test harness | type/logic errors, markdown contamination of code | `rustc --test` compile+run | yes — compile errors are objective failures | yes | uniform pass so far; compiler makes false-passes implausible |
| `repository_navigation` | multi-file reading, exact answer selection | retrieval across files, hallucinated paths | exact path match (suffix-tolerant) | yes (wrong file names fail) | yes | uniform pass since answer-key fix |
| `multi_file_reasoning` | cross-file value tracing + arithmetic | single-file shortcut reasoning | exact numeric match; result never appears verbatim in prompt | plausible but unproven | yes | uniform pass in Experiment 02 — discriminating power UNKNOWN |
| `subtle_bug_diagnosis` | defect localization over plausible alternatives | superficial "looks-like" answers | final-word exact function-name match | plausible (distractor name is the tempting answer) | yes | uniform pass in Experiment 02 — discriminating power UNKNOWN |

Audit conclusions:
- All five evaluators are objective (compiler / test-runner / exact match).
  No LLM judge anywhere.
- The three always-run fixtures form a *floor*: they catch gross capability
  collapse and pipeline regressions (they did exactly that during Phase 3
  harness debugging), but current evidence shows them insufficient to
  separate strong configurations. That is acceptable for a floor; not
  sufficient alone.

## Workload-level tasks

| Task | Status | Notes |
|---|---|---|
| `retrieval_mid` (50% depth) | KEEP | mid-window recall; part of the retrieval group with the probe below |
| `retrieval_end` (95% depth) | KEEP | discriminated BALANCED(q8_0) in Experiments 01/02 — proven detector |
| `kv_probe_deep` (97% depth, distractor codes) | KEEP — primary KV detector | Phase 5 battery: every non-f16-40k config picked the SAME wrong distractor; f16@40k passed. Hardest objective late-context probe in the suite |
| `code_fizzbuzz`, `math_mult` | KEEP | cheap sanity synthesis checks; floor-level signal |
| `instr_numbered` | KEEP | structural formatting; floor-level |
| `late_instruction` | RECALIBRATED (v2) | v1 failed uniformly: long-prompt echo residue broke ExactFold grading AND models treated the plain-text constraint as document continuation. v2 adds fenced override block, early-formatting habit to override, and positional trailing-line grading (`verify.EndsWithLines`). Validation run pending — if strong configs still cannot pass, it will be REMOVED from scoring per the SIGNAL>DIFFICULTY rule |

## Suite totals after recalibration

agentic_coding: 3 smoke + 13 capability (12 executable everywhere + rust
when toolchain present). Within the 10–20 target. No further additions
planned; discrimination now comes from depth (probe/diagnosis) rather than
task count.
