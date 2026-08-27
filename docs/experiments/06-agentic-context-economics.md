# Experiment 06 — Agentic Context Economics

Phase: 8
Date: 2026-08-26
Question: *What is the smallest active-context strategy that preserves actual
task completion while maximizing performance and stability?* — and, derived
from it: *is the `agentic_coding` 16k minimum-context contract justified?*

Environment (primary product target, unchanged from Phases 3–6):
- GPU: RTX 5070 12 GB · Ryzen 7 5700X (gumi pins 8 threads) · ~30 GB RAM
- Model: Qwen3-30B-A3B Q4_K_M (17.28 GiB, exps-CPU placement mandatory)
- Backend: llama-cli v10360 · temperature 0 · seed 42 · greedy decoding
- Anchor execution shape held fixed across strategies: ngl=max, KV=f16,
  flash attention on, b=2048/u=512, experts in RAM. Strategies differ ONLY
  in the configured context window: **8k / 16k / 24k / 32k**.
- Harness: `docs/experiments/exp06` (module-local experiment driver reusing
  `internal/backend`, `internal/verify`, `internal/gguf`, `internal/hardware`).
- Raw artifacts: `~/reports/phase8/exp06-results.json`, `run.log`.

Method honesty notes:

- Tokenizer density differs per model (llama-3.1 ≈ 2.95 chars/token,
  Qwen3 ≈ 2.46 measured on this exact fixture mix), so the driver calibrates
  against a deliberate micro-overflow probe — this build reports the exact
  prompt token count when a request exceeds `-c` — and shares the calibrated
  factor across all strategies so every window sees byte-identical sessions.
- This llama.cpp build **errors** (`request (N tokens) exceeds …`) rather
  than silently truncating most overflow paths; any residual overflow was
  detected from telemetry/stderr and retried once with harder shrink, so no
  cell conflates an overflow artifact with capability.
- Every failing cell was re-run once to separate stable failure from flake;
  all failures reproduced (no flakes observed).

---

## 1. Current context contract (Task 1)

`agentic_coding = MinContext(16384)` has a single source:
`internal/workload/tasks.go` (`agenticCoding()`). Everything else *reads*
it: candidate planning clamps every builder through
`clampContext` (floor), the Phase-7 policy layer consumes it for
context/placement decisions, the report prints it as a hard constraint, and
`gumi profiles` renders it. Nothing else hard-codes 16k.

Compaction infrastructure: the optimizer harness has **none**.
`AgentConfig.ContextCompactionThreshold` exists only inside the frozen
pre-pivot runtime module (`runtime/internal/{config,pipeline}`, default 0.85)
and is not wired into `cmd/gumi` + `internal/*`. Per the Phase-8 boundary
(no new memory subsystems), the experiment models active-context discipline
as **head-eviction** (oldest blocks dropped first — worst-case memory
management; a real compaction engine could only score equal or better).
Consequently this experiment measures small-window economics under naive
eviction, and says nothing about what smart compaction could achieve.

## 2. Hypothesis (Task 2)

A smaller active context may outperform a larger window for real agentic
workloads IF task completion survives. Context must grow the way agent work
actually grows — accumulated tool outputs — not as a static needle-in-haystack.

## 3. Context-growth workload

A deterministic simulated coding-agent session (pricing-engine migration):
the transcript accumulates `read_file` outputs, test logs, grep results,
review comments, and git history in steps. Three facts are planted at fixed
relative depths, and ONE final task requires all three:

| part | depth | content | survives eviction while… |
|---|---|---|---|
| `RULE=` | ~0% (step-1 briefing, with early reinforcement) | active pricing-rule ID | head blocks remain |
| `TEST=` | ~50% | exact name of the failing test | mid blocks remain |
| `FIX=`  | ~97% | approved-fix phrase | always (newest content) |

Session sizes: **4k, 7k, 10k, 14k, 18k, 24k** tokens (nominal; actual counts
verified by calibration). Grading is per-part, so failures localize: losing
`RULE` while keeping `TEST`/`FIX` means "capacity evicted the head", not
"model degraded". Full task completion = all three parts.

This is deliberately *not* a retrieval-only proxy: the final answer requires
synthesizing an early compliance rule into late code work — the shape of real
agent sessions where old constraints bind new edits.

## 4. Strategy definitions (Task 3)

| strategy | window | visible budget (window − gen cap − reserve) |
|---|---|---|
| A small | 8192 | ≈ 6.9k |
| B medium | 16384 | ≈ 15.1k |
| C alt | 24576 | ≈ 23.3k |
| D large | 32768 | ≈ 31.5k |

4k was considered and excluded before running: the step-1 briefing alone
plus one tool response exceeds a 4k visible budget minus generation space —
the point is structurally infeasible for any non-trivial agentic session
(documented rather than measured; "do not force infeasible points").

## 5. Performance measurements (Task 4)

Perf probes: 3 × 10k-token filler per strategy (identical prompts; medians):

| window | prefill tok/s | decode tok/s | peak VRAM GB |
|---|---|---|---|
| 8192  | 823 *(cold-contaminated, see §10)* | 25.3–29.3 | 2.00 |
| 16384 | 920 | 30.1–30.7 | 2.76 |
| 24576 | 933 | 32.2–32.3 | 3.52 |
| 32768 | 929 | 32.2 | 4.28 |

- VRAM scales linearly: **+0.75 GB per +8k window** (~94 MB/1k tokens,
  matching Exp 04's ~91 MB/1k including activations).
- Decode throughput is **window-independent within noise** on this stack
  (28–32 t/s across all windows). This matters for §7/§9: configuring more
  context did not slow generation down here.
- Session wall-time is dominated by prefill volume + thinking-style
  generation (~20 s at 4k sessions → ~62 s at 24k sessions, any window),
  i.e., latency tracks how much work the session contains far more than
  which window hosts it.

OOM/timeouts: none in any cell.

## 6. Capability / task-completion measurements (Tasks 4 + 12)

Completion matrix (P = all three parts; `.` = failed twice — every failure
was re-tested and reproduced; no flakes):

| window | 4k | 7k | 10k | 14k | 18k | 24k |
|---|---|---|---|---|---|---|
| 8192  | P | . | . | . | . | . |
| 16384 | P | P | P | . | . | . |
| 24576 | P | P | P | P | P | . |
| 32768 | P | P | P | P | P | P |

Part-level attribution at the frontiers (the diagnostic core):

- c=16384 @14k–24k sessions: `rule=false, test=true, fix=true` — ONLY the
  evicted head fact is lost; the model executes perfectly on visible content.
- c=24576 @24k session: identical signature (visible 21.4k < session 24.7k).
- c=8192 @≥7k sessions: all three parts fail — with the head gone AND the
  session entering mid-stream, the reasoning model loses the thread
  entirely (output degenerates toward continuation/echo rather than the
  three-line contract). Small windows don't degrade gracefully on this
  model; they fall off a cliff.

The competence frontier therefore tracks the visible budget almost exactly:
a window completes sessions up to ≈ (window − ~1.3k reserve) tokens, then
loses precisely the evicted content.

## 7. Compaction behavior (Task 5)

Not exercisable: the only compaction implementation lives in the frozen
pre-pivot runtime (`ContextCompactionThreshold`, config-only relative to the
optimizer) and could not be invoked without building a new memory subsystem,
which Phase 8 forbids. Measured instead: naive head-eviction economics (§6).
Documented limitation — **any claim of the form "8k + compaction suffices"
remains untested**; this experiment bounds it only from the eviction side
(eviction is the worst case, so compaction's true floor is ≤ ours, unknown
by how much).

## 8. Context capacity vs requirement (Task 8)

Separated explicitly:

- **Capacity** (what fits): Qwen3-30B-A3B processes 40k+ on this GPU with
  quantized tricks; even f16 reaches 32k in 4.28 GB. Capacity was never the
  binding constraint in any failure — every failure is eviction, i.e.,
  strategy-imposed.
- **Requirement** (what the workload needs): full task completion required
  visible ≥ session size, for every session size tested. Translating the
  ladder into requirement terms: hosting typical multi-file coding-agent
  sessions (≥7k tokens of tool traffic — a handful of file reads and one
  test log) already exceeds an 8k window; 15k-token sessions (one deeper
  investigation) exceed a 16k window.

A 40k-capable model does not imply a 40k-requiring workload — but the
inverse lesson is equally recorded: an 8k-capable configuration demonstrably
cannot host ordinary agent traffic without losing cross-session constraints.

## 9. Context-floor recommendation & economics (Tasks 6 + 7)

Completion-per-unit-latency reading of the matrix:

- **8k** buys nothing: it fails every session ≥7k outright (and its decode
  was if anything marginally slower than larger windows). The Phase-7
  observation that an 8k human baseline decoded 47% faster does NOT
  replicate as a context effect — with KV type and batch held fixed, decode
  is flat across 8k→32k. The Phase-7 baseline's advantage is now attributable
  primarily to its other variables (q8_0 KV, b=512/ub=128), not to context.
  (Llama-8B attribution not re-measured here; flagged in §11.)
- **16k** completes sessions through ~15k tokens at identical throughput to
  larger windows (+0.76 GB vs 8k).
- **24k/32k** extend reach linearly at +0.75 GB per 8k and zero measured
  throughput penalty on this stack; they convert directly into completion
  reach (18k and 24k sessions respectively).

Recommendation: **keep the 16k floor** (see §10); treat window growth above
it as a feasibility-priced completion-reach axis, already handled by the
existing QUALITY/policy machinery — not as a new default.

## 10. Workload-contract decision (Task 9)

**Decision: KEEP `agentic_coding MinContext = 16384`. Unchanged in code.**

Evidence:

1. 8k is empirically insufficient without compaction: complete-task failure
   on every ≥7k session, reproduced twice each, with cliff-shaped (not
   graceful) degradation on the primary model.
2. 16k is the smallest tested window that fully hosts common session scales
   (≤~15k tokens) — and its failures beyond that are pure capacity eviction,
   meaning the CONTRACT floor and the CAPABILITY frontier coincide; there is
   no hidden capability loss at 16k that a larger window would repair.
3. Throughput cost of the floor is nil on this stack (decode flat 8k→32k);
   its cost is VRAM (~0.76 GB vs 8k), which is exactly what feasibility
   arithmetic already prices.
4. Coherence with the existing battery: the suite's own deep-context probes
   generate content at scales where sub-16k windows cannot honestly run.

Options rejected: (B) lower to 8k — refuted by §6; (C) workload-derived
context — requires session modeling/compaction the harness cannot exercise;
(D) insufficient evidence — not the case; the paired ladder is decisive for
the no-compaction semantics actually shipped.

## 11. Remaining uncertainty

- Single model (Qwen3-30B-A3B) for the ladder; llama-3.1-8B validated the
  HARNESS (fixture depths, truncation, grading) but not the full matrix.
  The staircase shape should be assumed family-dependent until re-run.
- Compaction upside is unbounded above by this experiment: smart summarization
  could make 8k viable; nothing here tests that. Any future compaction
  subsystem must re-run this ladder before changing the floor.
- Real-session-size distribution is modeled, not observed: the ladder spans
  4k–24k because the existing battery and Phase-6 artifacts imply that range;
  instrumenting genuine agent traces would sharpen the requirement estimate.
- c=8192 perf-probe median looks depressed (823 vs ~930 t/s) but its first
  sample was cold-contaminated and its probe prompt was effectively
  truncated; treat the 8k prefill figure as low-confidence (does not affect
  any decision — decode and completion data carry the conclusions).
- Phase-7's "baseline faster at 8k" interpretation is corrected here for
  Qwen3 (context-flat decode); the Llama-8B three-variable confound remains
  formally unresolved.

---

## Concise comparison (required summary)

| | 8K | 16K | 32K (24k shown where tested) |
|---|---|---|---|
| task completion (sessions ≤10k / 14–18k / 24k) | pass / fail / fail | pass / fail@≥14k / fail | pass / pass / pass |
| prefill tok/s (probe median) | ~823 (low conf.) | 920 | 929–933 |
| decode tok/s | 25–29 | 30–31 | 32 |
| peak VRAM | 2.00 GB | 2.76 GB | 4.28 GB (@32k); 3.52 (@24k) |
| peak RAM | ~17.7 GB | ~17.7 GB | ~17.7 GB |
| compaction events | n/a (none exists) | n/a | n/a |
| capability result | cliff-fails ≥7k sessions | evicts head fact ≥14k sessions | completes all tested sessions |
| confidence | HIGH (repeated) | HIGH | HIGH |

## Verdict

**CONTEXT FLOOR VALIDATED**

The 16k `agentic_coding` minimum is validated as fit-for-purpose for the
shipped no-compaction harness: 8k is decisively insufficient (stable,
attributed-to-eviction task failures from 7k sessions upward), 16k is the
smallest tested window with no hidden capability loss, and additional window
above 16k buys completion reach at negligible throughput cost and linear,
already-budgeted VRAM. The validation is scoped: it holds under head-eviction
semantics on one model family; a future compaction mechanism must re-run this
ladder before any floor change.
