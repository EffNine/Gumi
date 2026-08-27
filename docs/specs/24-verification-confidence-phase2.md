# 24 — Verification & Confidence Phase 2 — Preserved Engine (V1 Component)

Version: 1.0
Status: **Implemented — preserved inside V1 auto-tuner**
Extends: `23-optimization-engine-mvp.md` (Optimization Engine MVP). See `26-gumi-v1-auto-tuner.md` for the current product contract.

Phase 2 goal: make Gumi results **trustworthy, reproducible, and useful**.
The product promise stays:

> Find the best *verified* inference configuration for this model, hardware,
> and workload — without silently sacrificing capability.

Everything below is deterministic. No ML, no learned weights, no LLM judge.

> **Note (V1):** Verification is empirical and bounded by the included test battery. Results are described as **SCREENED / VERIFIED / RECOMMENDED / REJECTED / UNKNOWN** — never as mathematical proof of "same intelligence" or "lossless". The reference policy, confidence rules, and fixture suites below are unchanged in V1.

---

# 1. Reference selection policy (was: implicit default)

The capability gate needs a baseline that is never a random default.
REFERENCE is now defined by an explicit policy:

```
REFERENCE = highest-confidence quality baseline that is feasible
            on the current hardware.
```

Properties (all enforced in `internal/candidate`, documented per-run):

- same model, same backend binary, same workload as every candidate;
- maximum capability priority: f16 KV cache, greedy decoding
  (temperature 0), fixed seed (42), flash attention;
- stable execution: context capped at the workload minimum, planned inside
  the 95% VRAM budget using exact GGUF KV arithmetic;
- expert placement only when memory safety requires it AND the model family
  is on the MoE whitelist (see §4);
- every relaxation is recorded in `ReferenceWhy[]` and rendered in the
  report under **REFERENCE CONFIGURATION / Why selected** with bullets for
  memory safety, quality settings, execution stability, and its role as the
  paired-comparison anchor.

If the reference OOMs at runtime, context halves once and retries (unchanged
MVP behavior); reference failure still fails the whole run loudly rather
than producing unpaired recommendations.

# 2. Confidence scoring (`internal/confidence`)

Every measured candidate receives a HIGH/MEDIUM/LOW confidence rating built
from fixed rules over measured evidence. Inputs (`confidence.Factors`) come
exclusively from pipeline measurements:

Positives (+):
- Tier 2 passed fully (or Tier 1 smoke when `--tier smoke`);
- N/N successful perf runs (N ≥ 2 required for the multi-run positive);
- stable decode latency: relative spread across perf repeats ≤ 10%;
- VRAM headroom ≥ 512 MiB against the safe planning budget.

Negatives (−):
- borderline VRAM (headroom < 512 MiB);
- any out-of-memory event;
- any timeout;
- errored perf runs;
- unstable decode latency (spread ≥ 25%);
- experimental expert placement active;
- incomplete Tier 2 rate.

Level mapping (deterministic precedence):

- **LOW** — gate failed, or any OOM/timeout/error event, or Tier 2 < 50%;
- **HIGH** — gate passed, zero negatives, ≥ 2 successful perf runs, and a
  completed verification tier;
- **MEDIUM** — everything else.

Unknown data is neutral: missing measurements withhold positives but never
fabricate penalties. Assessments are attached to candidates in
`candidates.json` (`confidence`) and rendered in `report.md`.

## 2.1 Repeated performance probes

Perf probing now runs `--perf-runs` times per candidate (default 3) and
records each sample (`Measurement.PerfSamples`). Reported t/s values are
means over successful samples; peak VRAM/RAM take the maximum across
samples (conservative). This is what makes latency-stability evidence
honest instead of asserted.

# 3. Objective agentic-coding verification

New fixture package: `internal/workload/agentic_coding/tests`.

| fixture | evaluation |
|---|---|
| `python_bug_fix` | model returns corrected `calculator.py`; evaluator injects it into a temp copy and runs `python3 test_calculator.py` — exit status decides |
| `rust_refactor` | model returns corrected `main.rs`; evaluator compiles `rustc --edition 2021 --test main.rs` and executes the harness |
| `repository_navigation` | multi-file mini repo; model names the file defining `MAX_RETRIES`; exact-match validation |

Rules:

- evaluation never depends on an LLM judge;
- executable fixtures require local toolchains (`python3`; `rustc` + `bash`);
  when missing they are excluded up front and listed in profile notes so
  gaps stay visible (`fixtures.Unavailable()`);
- suites are fixed before a run starts — availability never changes
  mid-flight, keeping paired comparisons fair;
- each evaluation is bounded by a 60 s timeout in a temp directory.

These replace the weakest synthetic task (string reversal) in the
agentic_coding suite. Golden group coverage below.

# 4. Explicit MoE handling

Expert tensor placement (`-ot exps=CPU`) affects execution only — Gumi never
changes weights, active expert count, or the computation graph. But hardware
and driver behavior varies, and some families need special handling.

Safeguards:

- **family whitelist**: automatic expert split is applied only to verified
  architectures (`qwen2moe`, `qwen3moe`, `mixtral`, `deepseek2`). Unknown
  families get partial offload instead, and no candidate carries
  `ExpertsOnCPU`;
- **explicit labeling**: any candidate relying on expert placement carries
  `experimental: true` plus a note stating exactly what changed and why it
  is considered experimental; reports mark such rows
  *(experimental)* and surface the note under the recommended block;
- EXPERT-SPLIT is generated only for whitelisted families.

Users can still apply expert split manually to non-whitelisted families via
exported llama.cpp commands — Gumi just will not choose it automatically.

# 5. Golden benchmark suite

The built-in verification tasks double as Gumi's regression benchmark
(`internal/workload/golden.go`, exposed via `gumi profiles`):

```
agentic_coding          chat
├─ python_bug_fix exec   ├─ reasoning
├─ rust_refactor  exec   ├─ instruction_following
├─ repository_navigation ├─ context_retrieval
│                 exact
├─ context_retrieval
├─ instruction_following
└─ code_synthesis
```

Kept deliberately small: retrieval haystacks cap at 24k tokens and suites
are fixed per workload, so a full Tier-2 pass completes in reasonable time
on consumer hardware. Any change to these tasks must be treated as a
capability-affecting change to Gumi itself.

# 6. Report quality

`report.md` now answers "what should I run?" first:

1. header (model/hardware/workload);
2. **REFERENCE CONFIGURATION** with the selection-policy rationale;
3. verified-candidates table (adds batch size and confidence columns,
   *(experimental)* markers, `planned` verdicts for dry runs);
4. **RECOMMENDED** block: configuration (context/KV/offload/batch/expert
   placement), verified performance, capability tier, confidence level with
   itemized evidence, and an **Alternatives** section describing each other
   gate-passer as a one-line tradeoff vs the winner (speed delta, context,
   KV precision, experimental flags);
5. exports (llama.cpp cli/server, LM Studio, Ollama) — unchanged.

Dry runs state plainly that performance/capability/confidence appear after
real verification instead of printing empty tiers.

# 7. CLI surface changes

- `gumi optimize --perf-runs N` (default 3): perf probe repetitions feeding
  stability evidence;
- `gumi optimize --baseline SPEC` (Phase 3): admits a human-provided
  execution-only configuration (`ngl,c,kv,fa,no-fa,b,ub,exps-cpu,gpu-exps,
  mmap,no-mmap,mlock,t`) as a `CURRENT-BASELINE` candidate that is planned,
  measured, capability-gated, ranked, and reported identically to generated
  candidates. Sampling keys are deliberately rejected by the spec parser —
  paired verification always forces temperature 0 and the shared seed.
  See `docs/experiments/01-real-hardware-validation.md` for the validation
  protocol this enables.
- `report.json`: new `reference` section, per-candidate `confidence`,
  `experimental`, batch fields;
- `candidates.json`: per-candidate `reference_why`, `experimental`,
  `measured.perf_samples`, run-event counters, `confidence`;
- `gumi profiles`: golden groups + notes (skipped toolchain fixtures).

# 8. Verification evidence

- `go vet ./internal/... ./cmd/...` clean; gofmt clean; zero external deps.
- `go test ./internal/... ./cmd/...` green, adding:
  - confidence rule-set unit tests (HIGH/MEDIUM/LOW paths, neutrality of
    unknown evidence, determinism),
  - reference-policy documentation test,
  - MoE whitelist gating test (whitelisted vs unknown architecture),
  - E2E report assertions (reference section, confidence levels including
    LOW for gate-failed candidates, alternatives, 3 perf samples),
  - golden-group/task consistency test,
  - fixture evaluator tests executing real python3/rustc when present
    (skipped otherwise).
