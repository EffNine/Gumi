# 26 — Gumi V1: The Auto-Tuner

**Status:** Implemented (V1)
**Supersedes:** none; extends 23–25 (the evidence engine is preserved, not replaced)
**Validation:** Phase 9 environment — Qwen3-30B-A3B Q4_K_M on RTX 5070 12GB;
second validation on Meta-Llama-3.1-8B-Instruct Q4_K_M (dense) on the same GPU.
**Language:** *local inference auto-tuner*, *hardware-aware inference tuning*, *measured / verified configuration*, *practical context frontier*, *capability gate*, *performance objective*. Avoid: *AI-powered optimization*, *guaranteed optimal*, *same intelligence*, *lossless* unless qualified by measured evidence.

---

## 1. V1 Product Contract

Gumi V1 is a **local inference auto-tuner** (CUDA / NVIDIA, single-GPU, GGUF + llama.cpp). The user gives
Gumi a model and (optionally) a workload:

```
gumi tune <model.gguf> [--workload agentic_coding|chat] [--min-decode N]
```

Gumi automatically experiments with inference configurations on the user's
actual machine, measures real performance, verifies capability, searches the
practical operating frontier, and returns **verified profiles**.

The user never needs to understand: GPU offload, `n_gpu_layers`, KV cache
precision, context size, batch size, ubatch size, expert placement, flash
attention, or any llama.cpp flag.

### Scope

| Dimension | V1 scope |
|---|---|
| Accelerator | CUDA / NVIDIA only, single GPU (any class: consumer, workstation, datacenter) |
| Model format | GGUF (parsed directly; no catalog) |
| Backend | llama.cpp (`llama-cli` subprocess) for actual tuning |
| Exports | llama.cpp CLI/server, LM Studio, Ollama (compatibility outputs — exports never claim settings the target cannot represent) |
| Tuning | Local hardware, workload-aware (`agentic_coding` / `chat`), capability-verified, context-frontier search |

Explicitly out of scope for V1 (and not claimed): ROCm, Metal, Vulkan, DirectML, CPU-only optimization, multi-GPU, multi-node, tensor parallelism, runtime scheduling / daemon / dashboard / web UI, sampler tuning, temperature optimization, continuous learning / online learning, automatic model quant selection, model downloading, cluster scheduling. The architecture stays extensible behind `backend.Runner`, but no other backend is implemented.

### Hard rules carried forward

- **Capability is absolute.** Performance ranks only candidates that pass the
  paired capability battery vs REFERENCE. Raw tok/s can never rescue a FAIL.
- **Safe-tuning boundary.** Never auto-change weights, quantization file,
  active expert count, RoPE scaling, system prompt, reasoning mode/budget, or
  sampling behavior (temperature/top-p/top-k/min-p fixed at greedy/seed 42).
- **Unknown stays unknown.** Hardware facts come from probing; nothing is
  fabricated.
- **KV arithmetic is exact** from GGUF geometry (`internal/gguf`).
- **Honest language.** Verification is described as **SCREENED / VERIFIED / RECOMMENDED / REJECTED / UNKNOWN** — never as proof of "same intelligence" or "lossless"; see §5 and `25-evidence-hardening.md`.

## 2. Tuning Loop

One tuning session runs this deterministic loop:

```
INSPECT → PROBE → DISCOVER BACKEND CAPABILITIES
  → GENERATE INITIAL CONFIGS (policy layer, unchanged)
  → STAGE A: MEASURE REFERENCE (warmup + repeated perf + full battery)
  → STAGE B: CONTEXT FRONTIER SWEEP (coarse doubling, perf probes)
  → STAGE C: BOUNDARY REFINEMENT (bisection between pass/fail)
  → STAGE D: VARIANT LINES (+ dominance pruning before batteries)
  → STAGE E: CAPABILITY-GATE THE FRONTIER (step down on regression)
  → FINAL VERIFICATION (re-test every recommendation)
  → VERIFIED PROFILES + REPORT + EXPORTS
```

The optimizer learns from measurements during ONE run. This is deterministic
experimental search, not machine learning.

Implementation map (single system — no second engine):

- `internal/optimize/pipeline.go` — orchestration, staging, gating, ranking
- `internal/optimize/tuner.go` — sweep/refinement/gating/confirmation machinery
- `internal/search` — pure strategy functions (ladder, refinement midpoint,
  objective evaluation, dominance, profile selection)
- `internal/candidate` / `internal/policy` — initial configs and slot policy
  (Phases 4–7 preserved as the seed of Stage D)
- `internal/backend` — llama.cpp runner, capability discovery, exports
- `internal/verify` / `internal/confidence` — gate, tiers, evidence semantics

## 3. Search Strategy

No brute force. Budgets are bounded by construction:

1. **Deterministic initial candidates** — REFERENCE plus policy slots
   (expert split, quality growth, balanced KV rung, speed batch, high-context
   q8 probe). At most five generated variants plus human baselines.
2. **Measured results** drive everything after REFERENCE:
   - Frontier levels: warmup + `--perf-runs` perf probes each (no battery).
   - Variants: perf probes first; the capability battery runs **only** for
     configurations not dominated by already-cleared measurements.
3. **Boundary refinement** bisects between the highest passing and lowest
   failing level until the bracket is ≤ 2048 tokens or `--max-refine-steps`
   (default 4) probes are spent.
4. **Dominated configurations** are pruned from further budget (see §7).
5. **Final verification** re-tests every recommended profile with fresh perf
   rounds; new samples append to the existing evidence (means and half-ranges
   recomputed over all runs).

## 4. Performance Objective

There is **no universal tok/s floor** in Gumi. A floor exists only when
someone declares one:

- `--min-decode N` — an absolute user floor in decode tok/s. It gates the
  frontier AND profile eligibility exactly as stated.
- Otherwise the workload's declared practicality rule applies
  (`Profile.DecodeRetention`): a larger context counts as practical only
  while it retains the fraction of the BEST MEASURED decode on this machine:
  - `agentic_coding`: retain ≥ 75% (prefill+depth bound; tolerates decode
    loss in exchange for window),
  - `chat`: retain ≥ 85% (decode-bound; responsiveness dominates).
- Neither declared → objective = stable execution; ranking orders by
  workload utility alone.

Decisions use the conservative lower bound (mean − half-range of repeated
probes): noise can never promote a point past the bar. OOM or timeout during
probes fails regardless of throughput.

These rules are hardware-relative by construction: an H100 and a laptop GPU
are each judged against their own measured baseline. A datacenter card is not
"successful" because it crossed some consumer threshold.

Workload emphasis (unchanged contracts): `agentic_coding` weights prefill
efficiency, context sufficiency, late-window capability, stability;
`chat` weights decode throughput and latency. Ranking weights live in the
profiles (`QualityPriority` / `LatencyPriority`).

## 5. Capability Gate

Unchanged from Phases 2–8 and still the **final authority** (faster never beats more capable):

- REFERENCE is measured first (conservative control: f16 KV, greedy decoding,
  fixed seed, memory-safe placement). Its OOM-retry degrade behavior is kept.
- Every candidate that survives perf probing and dominance pruning runs the
  smoke tier (must pass fully) and the Tier-2 battery; `verify.Gate` rejects
  any rate regression beyond `--gate-slack` (default 0).
- The frontier point is a recommendation like any other: **the full battery
  must clear it**. On regression it steps down through measured passing
  levels (≤ 3 attempts), otherwise MAX PRACTICAL CONTEXT anchors at the
  workload minimum.

## 6. Context Frontier Algorithm

Objective: find the maximum practical context satisfying capability PASS ∧
memory-stable ∧ performance-objective PASS — and report it SEPARATELY from
the theoretical capacity.

```
line   := execution line maximizing deterministic reach among backend-
          supported (KV type × placement) options      # pure arithmetic
ladder := doublings of MinContext capped by min(TrainContext, reach(line)),
          plus the cap itself when ≥1.25× the last doubling
          (a 40960-token training ceiling earns its own probe)
for level in ladder:                                     # coarse sweep
    probe(level): warmup + repeated perf probes
    OOM/timeout        → hi=level; break                 # memory wall
    objective pass     → lo=level; continue
    objective fail     → hi=level; break                 # throughput wall
while hi>lo and steps < --max-refine-steps:              # bisection
    mid := round1024((lo+hi)/2); probe(mid); update lo/hi
frontier := lo                                           # practical boundary
run FULL battery at frontier; on gate failure step down
```

- Midpoints are rounded to 1024-token multiples; boundaries are NOT assumed
  to be powers of two (e.g. 80K, not just 64K).
- Early exit: if the user gave `--min-decode` and REFERENCE itself misses it,
  the sweep is skipped — growing context cannot recover throughput.
- Report carries both numbers: `TheoreticalMax` (exact memory arithmetic) vs
  `MaxPractical` (measured, capability-gated). They are never conflated.

Validated examples — **both on RTX 5070 12GB** (CUDA 13.2, driver 595.84, `llama-cli` v10360). Presented as **validation examples, not universal performance guarantees** — throughput varies by hardware, driver, llama.cpp build, and quant.

- **Llama-3.1-8B-Instruct Q4_K_M · workload `chat`** — maximum practical context **65,536 tokens**, capability verified. Theoretical capacity ≈ 96K on q4_0 KV; ladder capped by training context; all levels ~85 tok/s ≥ floor.

- **Qwen3-30B-A3B Q4_K_M · workload `agentic_coding`** — maximum practical context **40,960 tokens**, capability verified, approximately **30 tok/s decode** in the validated run (e.g. CTX-40K q4_0 at 30.4 tok/s, REFERENCE f16 at 30.5 tok/s; QUALITY f16 at 27.7 tok/s — all 12/12 100% capability where noted; see `27-gumi-v1-release-audit.md` §4.2 for full profile table). The frontier was found via coarse doubling + bisection refinement, then capability-gated, dominance-pruned, and re-tested.

Theoretical vs practical remain separate: `TheoreticalMax` is exact memory arithmetic; `MaxPractical` is the largest **measured and capability-gated** context that also satisfied the performance objective — never conflated.

## 7. Dominated Configurations

A candidate A is dominated by a measured, capability-CLEARED point B when:

- benefit axes — B is at least as good: decode lower-bound, prefill
  lower-bound, capability rate;
- resource axes — B costs no more: **context**, peak VRAM, peak RAM
  (context IS memory: KV scales linearly with window);
- strictness — B beats A somewhere by more than combined measurement noise.

Rules:

- Only gate-PASSED points grant domination rights. Fast-but-dumb lines can
  never prune anything, and unmeasured axes neither win nor lose.
- Dominated variants skip the capability battery; the row records
  `dominated_by` and status PROBED. Rejections remain recorded either way —
  every tested configuration appears in the report.

## 8. Hardware Abstraction

Every hardware-dependent value comes from probing (`internal/hardware`):
GPU name/VRAM via nvidia-smi, RAM via /proc or sysinfo, threads from CPU
topology, storage mmap-capability. No RTX-model, VRAM-size, generation,
bandwidth, tensor-core, or throughput assumptions exist anywhere in
`internal/`. Planning uses probed totals × a generic 0.95 safety factor;
measurement overrides planning wherever they disagree.

Validated across model shapes on one CUDA GPU (Qwen3-30B-A3B MoE with expert
split; Llama-3.1-8B dense without). The same code path handles both; nothing
keys off model family except the audited MoE placement whitelist
(`qwen2moe/qwen3moe/mixtral/deepseek2`).

## 9. Backend Capability Discovery

At startup Gumi probes `llama-cli --help` once (`internal/backend`):

- parses accepted cache types (`-ctk/--cache-type-k allowed values:`),
- detects flash-attention syntax (`on|off|auto` values vs bare flag),
- detects `-ngl`, `-b/-ub`, mmap/mlock, `-ot` override-tensor,
  `--single-turn`.

Unsupported dimensions are **suppressed**: planning skips them (generator is
capability-aware; the exploration line never selects them), affected
candidates are excluded up front with recorded reasons, and the report lists
each suppression under BACKEND CAPABILITIES + Limitations. One missing
optional feature never fails the session. Defense-in-depth:
`validateAgainstCaps` refuses to RUN a config demanding an unsupported
evidence-critical flag rather than silently measuring something else, and
the legacy unknown-argument retry chain remains the final arbiter when
discovery produced nothing usable.

## 10. Failure Handling

- Subprocess isolation per run, per-run timeout, process-group kill,
  VRAM/RSS sampling, stderr classification into OOM / timeout / other.
- OOM during the frontier sweep marks the wall and refinement continues
  below it; OOM in a variant records a REJECTED row; the session continues.
- Non-classified backend errors yield UNKNOWN rows (insufficient evidence,
  never fabricated verdicts).
- Reference failure aborts the run (nothing is rankable without the anchor);
  partial artifacts are written first.
- **TARGET NOT ACHIEVED** is a first-class outcome: when no verified
  configuration satisfies the declared objective, Gumi reports it plainly,
  names the best verified configuration, writes all artifacts, and exits
  non-zero. It does not fake a winner.

## 11. Profile Generation

From verified, objective-satisfying candidates (REFERENCE always eligible as
anchor), `search.SelectProfiles` deterministically assigns:

| Label | Rule (tie-break order) |
|---|---|
| MAX CONTEXT | largest context (less VRAM → faster) |
| SPEED | fastest decode lower bound (less VRAM → smaller ctx) |
| QUALITY | capability rate → KV fidelity → context |
| BALANCED | best workload utility among unlabeled candidates |

Labels may collapse onto one configuration; overlapping repetition ranges at
equal capability are reported as **operationally tied** — distinctions are
never manufactured. Each recommendation is re-tested before reporting.

## 12. CLI UX

```
$ gumi tune qwen3-30b-a3b-q4_k_m.gguf --workload agentic_coding

GUMI AUTO-TUNER
Model: …  Hardware: …  Workload: …

Discovering backend capabilities...
Measuring REFERENCE configuration...
Searching context frontier...
  [PASS] 32K q4_0 — decode 28.9 tok/s meets target 24.6
  [REJECT] …
Testing configuration variants...
Verifying frontier capability...
Final verification...

MAX PRACTICAL CONTEXT
  40K tokens — decode 26.1 tok/s, prefill 3105.2 tok/s

QUALITY / BALANCED / SPEED / MAX CONTEXT blocks…

RECOMMENDED …
Full report and machine-readable results: reports/<…>/
```

Flags: `--workload` (default `agentic_coding`), `--min-decode`,
`--tier smoke|capability`, `--perf-runs`, `--max-refine-steps`,
`--timeout`, `--gate-slack`, `--baseline`, `--backend-bin`, `--dry-run`,
`--out`. `gumi optimize` remains a documented alias.

Exit codes: `0` success · `1` TARGET NOT ACHIEVED or no verified winner ·
`2` usage errors.

## 13. JSON Schema (result artifacts)

`report.json` (single machine-readable result) contains:

```jsonc
{
  "generated_at": "…", "gumi_version": "…", "workload": "agentic_coding",
  "model": { "path", "architecture", "params", "quant", "layers",
             "train_context", "file_size_gb", "moe" },
  "hardware": { "gpus[]", "ram_total_gb", "ram_available_gb", "cpu_model",
                "threads", "filesystem", "bandwidth_gbps" },
  "backend_capabilities": { "backend", "discovered", "flash_attention",
                "kv_cache_types_supported[]", "expert_placement_supported",
                "suppressed[]" },
  "objective": { "user_floor_tps", "workload_retention",
                "baseline_decode_tps", "effective_floor_tps",
                "achieved", "statement" },
  "reference": { "context_tokens", "kv_cache", "gpu_layers", "why[]" },
  "policy": { "decisions[]{axis,impact,source,choice}", "admitted_slots[]",
              "declined_slots[]" },
  "candidates": [ { "id", "name", "status", "context_tokens", "kv_cache",
      "gpu_layers", "batch_size", "ubatch_size", "experts_on_cpu",
      "probe_only", "dominated_by", "feasible", "prefill_tps", "decode_tps",
      "peak_vram_gb", "perf_runs", "decode_half_range", "smoke_passed",
      "capability_passed", "capability_rate", "gate_passed", "gate_reason",
      "error", "score", "confidence{level,positives[],negatives[]}" } ],
  "ranking": { "level", "indistinguishable", "note", "winner_id",
               "runner_up_id" },
  "frontier": { "line_kv", "line_experts_cpu", "theoretical_max_context",
               "max_practical_context", "capability_gated",
               "boundary_reason", "coarse_levels_tested[]",
               "refinement_probes[]" },
  "profiles": [ { "labels[]", "candidate_id", "context_tokens", "kv_cache",
               "decode_tps", "prefill_tps", "peak_vram_gb", "capability_rate",
               "confidence", "tied_with[]" } ],
  "winner_id": "…", "limitations[]", "exports{llama_cli,llama_server,…}"
}
```

Auxiliary artifacts: `candidates.json` (full candidate objects incl. every
perf sample), `hardware.json`, `report.md`.

## 14. Known Limitations

1. **Single-GPU, single-backend verification.** Multi-GPU tensor/pipeline
   split is out of scope; whatever llama.cpp does with visible devices is
   what gets measured.
2. **Exploration line breadth.** The frontier sweeps ONE line (reach-
   maximizing KV × placement). Other KV types are evaluated as point
   variants at their planned contexts, not swept. Justified: reach defines
   the frontier; speed roles come from variants.
3. **Refinement granularity/steps.** Boundary precision defaults to
   2048 tokens within 4 bisection steps; tighter bounds cost more probes.
4. **Capability cost.** The Tier-2 battery is the dominant cost; perf-only
   probes keep the sweep cheap but mean intermediate levels carry no
   capability verdict unless promoted (status PROBED).
5. **Flat-throughput ties.** On GPUs where decode barely moves with context,
   profiles legitimately collapse onto near-identical operating points and
   are reported as tied rather than ranked.
6. **Warmup depth.** One discarded generation absorbs file-cache/allocator
   effects; deeper warmup (e.g. KV pre-fill) is future work.
7. **Windows/macOS.** Process management and RSS sampling have platform
   files; CUDA probing assumes nvidia-smi. Untested platforms stay untested.
8. **Second-GPU-class validation.** V1 validated two MODEL shapes on one GPU
   (RTX 5070). An A100/H100-class validation pass remains desirable.
