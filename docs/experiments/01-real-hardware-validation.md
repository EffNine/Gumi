# Experiment 01 — Real-Hardware Validation of the Gumi Thesis

Status: **EXECUTED — see results-01.md. Verdict: VALIDATED** (two full
repetitions on RTX 5070 + Qwen3-30B-A3B Q4_K_M; artifacts in ~/reports/exp01-run{1,2})
Phase: 3
Date: 2026-08-25
Target machine: this workstation (probed, not assumed)

---

## 1. Hypothesis under test

> Gumi can find a better verified inference configuration for a local model on
> the user's exact hardware, while preserving model capability.

Operationalized: on an RTX 5070 12 GB + Ryzen 7 5700X + 30 GB RAM, for a
27B/30B-class MoE GGUF and the `agentic_coding` workload, at least one Gumi
configuration achieves, against a competent human baseline:

- **capability parity** (Tier-2 rate ≥ baseline rate, paired prompts/seeds,
  temperature 0, identical model file/backend binary), AND
- **materially better practical performance** (≥ 20% decode tok/s) or
  **materially better feasibility** (baseline OOMs/timeout/unstable; Gumi
  config stable across repeats),

while never recommending a faster configuration that regresses capability.
"Better" therefore means *capability-dominant*, not merely faster.

## 2. Variables

Independent (the four configurations):

| ID | Config | Source |
|---|---|---|
| A `CURRENT-BASELINE` | competent-user manual config (§4) | human-specified via `--baseline`, measured identically |
| B `GUMI REFERENCE` | policy-selected quality anchor | candidate generator |
| C `GUMI BALANCED` | optimizer recommendation | candidate generator |
| D `GUMI SPEED` (optional) | aggressive throughput | included only if it passes the capability gate |

Controlled (identical across all configurations):

- one model file (bit-identical, sha256 recorded)
- one backend binary (`llama-cli`, version recorded)
- same prompts, seeds (42), temperature 0, task fixtures
- same hardware state as measurable (idle desktop, no other GPU clients;
  nvidia-smi utilization checked before each leg)
- same per-run timeout

Dependent (measured, never assumed):

- prefill tok/s, decode tok/s (mean over ≥3 perf probes; spread reported)
- latency: TTFT proxy = prompt eval time from perf line; decode ms/token
- peak VRAM (nvidia-smi polling delta), peak RAM (child RSS polling)
- OOM events, timeouts, errored runs
- Tier 1 smoke pass rate; Tier 2 capability pass rate split by group:
  coding (incl. `python_bug_fix`, `rust_refactor` exec fixtures),
  repository_navigation, instruction following, context retrieval at the
  configuration's own context window
- stability: decode spread across perf samples; run-to-run repeatability of
  the whole experiment (protocol supports N full repetitions)

Excluded from tuning in this experiment: sampler parameters (temperature/
top_p/top_k/min_p stay fixed at verification defaults). Execution knobs only.

## 3. Procedure

1. `gumi probe --json` → record hardware facts.
2. `gumi inspect <model.gguf> --json` → record geometry incl. MoE expert
   counts, expert bytes, KV bytes/token (exact math).
3. `gumi optimize <model.gguf> --workload agentic_coding --baseline <SPEC>`
   → runs all four configurations through the identical measurement +
   paired-gate pipeline; artifacts in `reports/<run>/`.
4. Repeat step 3 once more (second full repetition) to check run-to-run
   stability of decisions (winner must not flip; gate verdicts must match).
5. Collect `report.json` + `candidates.json`; render comparison (§7).

Total expected wall time: ~2–3 h for both repetitions on this class of GPU
(5 configs × [3 perf probes + 3 smoke + ~8 capability tasks] × 2 runs).

## 4. CURRENT-BASELINE definition (A)

A technically competent user with a 12 GB card running Qwen3-30B-A3B today
typically lands on one of:

- LM Studio default-ish: max GPU offload attempt, context 4096–8192,
  KV f16, flash attention on — often OOMs or silently splits layers;
- community guidance: partial offload ~28–33/48 layers with experts on CPU,
  context 8k–16k, q8_0 or q4_0 KV to fit.

The baseline actually used will be **recorded verbatim** in results, chosen
from that family, e.g.:

```
--baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128'
```

It participates in the pipeline like any candidate: same perf probes, same
suites, gated against GUMI REFERENCE. If the baseline wins honestly, that is
a valid (falsifying) outcome. Baseline spec keys are execution-only
(`ngl,c,kv,fa,b,ub,exps-cpu,mmap,mlock,t`); sampling is forced to the shared
verification values so the comparison stays paired.

## 5. Capability dominance rule (Task 4)

The ranking already encodes it (`score = Q·capability_rate + L·speed_norm`),
and the gate hard-rejects any configuration whose Tier-2 rate falls below
REFERENCE minus slack (default 0) even if it is fastest. The experiment
reports explicitly show, for every rejected-but-faster configuration, its
tok/s next to its failed tasks — the "30 tok/s but dumber loses to 20 tok/s"
case must be visible in the artifact table, not asserted in prose.

## 6. MoE behavior audit (Task 5)

From `candidates.json` + `gumi inspect`, report per configuration:
total experts, active experts (must be identical across all rows — read from
model metadata, never tuned), GPU-resident layers, expert placement flag,
context, KV type, batch/ubatch, flash attention, mmap/mlock, threads.

Expert-split validity check (static): `-ot exps=CPU` moves the `*_exps`
tensors only; active-expert count is a metadata property Gumi never emits a
flag for (grep audit of backend flag renderer required in the report).
Dynamic confirmation: llama.cpp load log shows expert tensors on CPU while
non-expert layers on GPU, and prompt-eval works — recorded from stderr tail.

## 7. Comparison report format (Task 8)

One markdown artifact per experiment (`docs/experiments/results-01.md`),
containing MODEL / HARDWARE / BACKEND sections, then per configuration:
config line, prefill, decode, peak VRAM, peak RAM, Tier 1, Tier 2, coding
pass rate, reasoning, retrieval, stability, confidence level. Verdict block
at the end, strictly one of:

- **VALIDATED** — C (or D) dominates A on the hypothesis's terms;
- **PARTIALLY VALIDATED** — improvement shown on some dimensions,
  insufficient evidence for the full claim (e.g., single repetition,
  small capability margins);
- **FALSIFIED** — A matches or beats every Gumi configuration, or Gumi
  cannot keep its own gate honest on real hardware.

No verdict without real numbers. No numbers without artifacts.

## 8. Candidate-space analysis (Task 6)

After measurements, answer explicitly:
1. Did Gumi beat A meaningfully? (quantified deltas)
2. Did candidates converge to similar performance? (spread table)
3. Were faster-but-worse configs rejected? (gate reasons)
4. Did expert placement matter materially? (BALANCED±split deltas)
5. Did KV/context choices matter materially? (QUALITY/BALANCED/SPEED deltas)
6. Is a safe execution knob missing? — only if the data shows a systematic
   loss attributable to a knob Gumi does not control (candidates: tensor
   override granularity beyond exps=CPU, ubatch-specific latency effects,
   KV cache reuse/defrag flags, swa/full attention split if the arch uses
   it). Each proposal must cite the measured gap it would close.
7. What actually constrains the optimization problem on this hardware?

## 9. Required artifacts — current blocker

| Artifact | Status |
|---|---|
| RTX 5070 12 GB, driver 595.84 | ✅ present (`nvidia-smi` verified: 12227 MiB total) |
| Ryzen 7 5700X (16 threads), 30 GB RAM | ✅ present (/proc/cpuinfo, /proc/meminfo) |
| 261 GB free disk | ✅ sufficient for model + reports |
| `llama-cli` (CUDA) | ❌ absent — see remediation options below |
| 27B/30B-class MoE GGUF | ❌ absent from disk (searched home, /mnt, /media, LM Studio & Ollama caches) |

**Backend gap detail:** a CUDA llama.cpp runtime exists
(`~/.unsloth/llama.cpp/build/bin/llama-server`, version 10360 / b90e6a913,
with `libggml-cuda.so`) but Gumi's runner requires `llama-cli`, which is not
included in that prebuilt set. Building it locally from the same tree fails
at CMake's CUDA compiler identification: **nvcc 13.1 vs glibc ≥ 2.42**
`rsqrt` exception-specification conflict in
`crt/math_functions.h` (reproduces with gcc-15 and gcc-13 as CUDA host
compiler). Known toolchain bug, fixed upstream in newer CUDA releases.
Viable remediations, in order of preference:

1. Install/upgrade to CUDA ≥ 13.2 toolkit, then build `llama-cli`
   (`cmake -DGGML_CUDA=ON -DCMAKE_CUDA_ARCHITECTURES=120`);
2. Build inside a container with an older glibc
   (`nvidia/cuda:13.1-devel-ubuntu22.04`; host docker already has the
   `nvidia` runtime registered) and run via `--gpus all`;
3. Patch `/usr/local/cuda/include/crt/math_functions.h` (root-owned;
   requires explicit authorization) to align the two declarations;
4. Provide any recent official `llama-cli` release binary with CUDA support
   matching sm_120.

**Model artifact required (exact):**

- Primary: **Qwen3-30B-A3B**, single-file GGUF, quant **Q4_K_M**
  (~18–19 GB), architecture key `qwen3moe` (MoE-whitelisted; 128 experts,
  8 active — the exact geometry Gumi's KV math is tested against:
  96 KiB/token f16). Instruct or Instruct-2507 variant preferred for the
  agentic_coding workload; Thinking variant acceptable but slower per task.
- Acceptable alternates: any other quant ≥ Q4_K_M of the same family
  (e.g. Q5_K_M ~21 GB — tighter fit, still feasible with expert split),
  or Qwen3-30B-A3B-Thinking for reasoning-heavy emphasis.
- Not acceptable for this experiment: dense models (Qwen3-32B), non-
  whitelisted MoE families (gpt-oss) — they change what is being tested.

Per instructions, Gumi will not download models autonomously. When the file
is placed locally (user's choice of source) and one backend remediation is
applied, the experiment proceeds with:

```bash
gumi probe --bandwidth --json > reports/hw.json
gumi inspect <model.gguf> --json
gumi optimize <model.gguf> --workload agentic_coding \
    --baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu' \
    --out reports/exp01-run1
gumi optimize <model.gguf> --workload agentic_coding \
    --baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu' \
    --out reports/exp01-run2   # repetition for stability
```

(The baseline spec above mirrors community guidance for this class of
model on 12 GB cards: partial offload with experts in system RAM, q8_0 KV,
modest batch. It is measured — not assumed — and may honestly win.)

## 10. Harness support

The three-way comparison requires measuring a *human* config through the
same pipeline. Implemented for this phase: `gumi optimize --baseline SPEC`
adds a `CURRENT-BASELINE` candidate that is planned, measured, capability-
gated, ranked, and reported exactly like generated candidates (see
`internal/backend/configspec.go`, `internal/optimize`). No new subsystem;
no changes to sampling behavior.

## 11. Real-hardware integration findings (pre-experiment, 2026-08-25)

Bringing the experiment up on the actual RTX 5070 exposed defects that
static testing had missed. All fixed before measurement; listed here
because they are themselves evidence of what "trustworthy" requires:

1. **MoE under-offload planning defect** (`internal/candidate`):
   `layersThatFit` charged full-weight per-layer cost in expert-split mode
   and double-counted the non-expert block, planning 22/48 GPU layers where
   48/48 fit (Qwen3-30B-A3B non-expert weights ≈ 0.93 GB). Reference policy
   also used an arbitrary `< LayerCount/2` split trigger instead of
   placement math; QUALITY never considered expert split (f16 candidates
   collapsed to CPU-only). Fixed: marginal-cost model per mode, split chosen
   when it increases GPU-resident layers (whitelisted families only),
   duplicate EXPERT-SPLIT suppressed when it converges with REFERENCE.
2. **llama.cpp b10360 compatibility** (`internal/backend`): new compact perf
   line `[ Prompt: x t/s | Generation: y t/s ]` printed on stdout (was not
   parsed → "no timing data"); `-no-cnv` accepted but conversation loop
   still spins on stdin EOF for chat-template models — `--single-turn`
   exits cleanly and is now preferred when advertised; timings parsed from
   combined stdout+stderr.
3. **Validator honesty against prompt echo**: recent builds echo the whole
   `-p` prompt to stdout; cleaning only a 200-char tail left retrieval
   haystacks in the output, letting any config pass by echoing the needle.
   `CleanOutput` now strips the full echoed prompt; retrieval grading uses
   last-code-match, logic answers use final-word match (thinking-tolerant,
   still judge-free).
4. **Thinking-model token budgets**: Qwen3-30B-A3B reasons before answering;
   original 24–96-token budgets truncated answers spuriously. Budgets
   raised across all suites (paired impact only — identical for every
   configuration).
5. **Test hygiene**: three pipeline tests restored injected hardware
   immediately (`swapHardware(...)()`), silently probing the host machine.
   A GPU-less container caught it; swaps now properly deferred.
6. **stdout capture integrity**: b10360 writes everything to stdout —
   `Loading model…` spinner with backspace animation frames, a UTF-8 init
   banner, and the perf summary after generation. Raw capture polluted
   validators (retrieval "failed" on spinner text). CleanOutput now strips
   control chars/spinner/banner and truncates at the perf summary.
7. **Thinking-model budget truncation**: Qwen3-30B-A3B emits explicit
   `[Start thinking] … [End thinking]` blocks and exhausted the original
   token budgets before answering, producing empty-but-honest failures for
   every config. Thinking spans are now stripped before validation and
   budgets raised so answers are reachable; identical budgets across all
   configurations keep comparisons paired.
