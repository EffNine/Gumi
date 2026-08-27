# Gumi — Local Inference Auto-Tuner

Gumi is a **local inference auto-tuner**: a CLI-first, local-first Go tool that
experiments with inference configurations on *your* CUDA machine. Give it a GGUF
model and a workload — Gumi inspects the model geometry, probes the hardware,
discovers what the installed `llama.cpp` backend actually supports, measures real
configurations, screens and verifies capability against its defined evidence
battery, and recommends the configurations supported by measured evidence. No cloud,
no database, no guessing. Gumi does not prove general model intelligence and does
not guarantee global optimality.

```bash
gumi tune qwen3-30b-a3b-q4_k_m.gguf --workload agentic_coding
# or just:
gumi tune qwen3-30b-a3b-q4_k_m.gguf
# with an explicit decode floor (tok/s):
gumi tune model.gguf --min-decode 25
```

## Why Gumi?

Local model users constantly guess:

- context length
- KV cache precision (`f16` / `q8_0` / `q4_0`)
- GPU offload (`-ngl`)
- expert placement (`-ot exps=CPU`)
- flash attention (`-fa`)
- batch / ubatch (`-b` / `-ub`)

Different models behave differently on different hardware. Wrong guesses make good
models look weak, waste VRAM, or silently degrade long-context recall.

Gumi replaces guesswork with measurement. It loads and measures real
configurations on your machine — real prefill/decode throughput, real VRAM/RAM
peaks, real capability checks — and only recommends what it verified. The core
philosophy:

> *Don't guess the best local inference settings. Measure them on the user's
> actual machine.*

## Quick Start

**Build:**

```bash
make build        # produces ./gumi  (Go 1.25+)
make test         # unit tests — no backend or models required
```

Requires `llama-cli` (llama.cpp, CUDA build recommended) on `PATH` or via
`--backend-bin` for real tuning. No model files are bundled.

**Tune:**

```bash
# V1 product command — default workload: agentic_coding
./gumi tune ./model.gguf --workload agentic_coding

# Minimum decode requirement (tok/s):
./gumi tune ./model.gguf --workload agentic_coding --min-decode 25

# Plan only — no llama.cpp needed (shows the planned search)
./gumi tune ./model.gguf --workload agentic_coding --dry-run
```

**Show result:**

Each run writes `reports/<model>-<workload>-<timestamp>/` containing:

- `report.md` — human-readable recommendation
- `report.json` — machine-readable evidence
- `candidates.json` — full candidate objects + every perf sample
- `hardware.json` — probed hardware snapshot

Example summary printed after tuning:

```
MAX PRACTICAL CONTEXT
  40960 tokens — decode 26.1 tok/s, prefill 3105.2 tok/s

RECOMMENDED — QUALITY (f16, 40960 ctx)
  Decode: 30.4 tok/s ± 0.3 (3 runs)  ·  Tier 2: 12/12 PASSED  ·  Confidence: HIGH
  Operationally tied with SPEED on capability; wins on KV fidelity.
```

**Export:**

```bash
# Render one saved candidate as a launch config for another tool
./gumi export --config reports/<run>/candidates.json --id balanced \
    --target llama.cpp --model model.gguf
# targets: llama.cpp | llama-server | lmstudio | ollama
```

## How It Works

```
MODEL ──▶ Inspect ──▶ Hardware Probe ──▶ Backend Capability Discovery ──▶
      (GGUF)       (GPU/CPU/RAM/disk)   (what does THIS build support?)

──▶ REFERENCE measurement (warmup + repeated perf + full battery)
──▶ Context frontier sweep (coarse doubling → bisection refinement)
──▶ Variant lines (+ dominance pruning) ──▶ Capability gate on everything
──▶ Final verification ──▶ QUALITY / BALANCED / SPEED / MAX-CONTEXT profiles
```

1. **Inspect** the model geometry from GGUF (exact KV-cache arithmetic: 2 × layers × kv_heads × head_dim × bytes-per-elem, including ggml block sizes for quantized KV — e.g. Qwen3-30B-A3B = 96 KiB/token at f16).
2. **Probe** CUDA/NVIDIA hardware (`nvidia-smi` / `lspci` fallback, CPU topology, RAM, filesystem). Unknown stays unknown — never fabricated.
3. **Discover** backend capabilities by parsing `llama-cli --help` once (accepted KV types, flash-attention syntax, `-ot`, batch flags). Unsupported dimensions are suppressed upstream with recorded reasons.
4. **Generate** a small deterministic candidate set (REFERENCE + up to five policy slots; `internal/candidate` + `internal/policy`).
5. **Search** the practical context frontier: coarse doublings (`16K→32K→64K…`) capped by `min(TrainContext, reach)`, then bisection between the last passing and first failing level until ≤ 2048 tokens or `--max-refine-steps` (default 4). Midpoints are 1024-aligned.
6. **Measure** prefill/decode, VRAM/RAM peaks, stability; **verify** capability via paired battery vs REFERENCE.
7. **Reject** anything that degrades capability. **Refine** the best boundary. **Rank** verified configurations conservatively and report **operational ties** honestly.
8. **Export** verified configurations for `llama.cpp` / LM Studio / Ollama.

## What Gumi Tunes

Execution configuration only (never weights, never active expert count, never
RoPE scaling, never system prompts, never reasoning/sampling behavior):

- GPU offload (`-ngl`)
- Expert placement (`-ot exps=CPU` — MoE whitelist: `qwen2moe`/`qwen3moe`/`mixtral`/`deepseek2`; otherwise suppressed)
- Context length
- KV cache precision (`f16` / `q8_0` / `q4_0`, backend-gated)
- Flash attention (`-fa on|off|auto`, backend-gated)
- Batch size (`-b`) and ubatch size (`-ub`)
- Threads, `mmap` / `mlock`

Quantization of the model file itself is **recommend-only** — never applied
automatically.

## What Gumi Does NOT Tune

- Model quantization / weight changes
- Temperature, `top_p`, `top_k`, `min_p` or any sampler setting (verification is fixed at `temperature 0`, `seed 42`)
- Reasoning budget / thinking mode / system prompt
- Number of active experts (`NumExperts` is read from GGUF metadata; Gumi only moves *where* expert tensors reside)
- RoPE / context-extension scaling
- Any backend control not advertised by the current `llama-cli --help`

Gumi never auto-changes any of the above; if a knob is unsupported by the
installed backend build, it is suppressed with an explicit reason.

## What Gumi Is NOT

- An LLM runtime, model server, launcher, or wrapper
- A dashboard or web UI (the pre-pivot runtime/dashboard at `runtime/` + `dashboard/` is frozen legacy)
- A coding agent, sampler optimizer, or model quantizer
- An ML training / fine-tuning / continuous-learning system
- A general cluster scheduler or multi-node orchestrator

## The Core Trade-off: Faster != Better

Gumi does **not** simply maximize tok/s. It searches for the best configuration
**subject to** four constraints:

1. **Hardware feasibility** — fits in probed VRAM/RAM budgets (95% safety factor).
2. **Workload requirements** — meets the workload's `MinContext` and hard constraints.
3. **Performance objective** — satisfies the declared decode floor (see below).
4. **Capability preservation** — passes the paired capability gate vs REFERENCE.

A faster configuration can be **rejected** if capability verification fails.

```
FAST + capability FAIL  →  REJECTED
SLOWER + capability PASS  →  wins
```

This is the primary product differentiator and is exercised end-to-end in
`internal/optimize/pipeline_test.go` with a fake backend (`dumbKV: "q4_0"` at
99 tok/s loses to f16 at 30 tok/s).

## Maximum Practical Context

Do not confuse:

- **Theoretical context capacity** — what VRAM arithmetic says could fit
- **Model training context** — what the model was trained on
- **VRAM-derived possible context** — a budget estimate
- **Maximum practical verified context** — what Gumi actually measured and verified

Gumi reports `TheoreticalMax` (exact memory arithmetic for the reach-maximizing
KV × placement line) **separately** from `MaxPractical` (measured,
stability-gated, capability-gated). The frontier point runs the **full battery**;
on gate regression it steps down through measured passing levels (≤ 3 attempts).
If no level clears, `MaxPractical` anchors at the workload minimum. The result
is:

> *The largest context Gumi could verify on this hardware / model / workload
> while satisfying the configured performance and capability constraints.*

Theoretical arithmetic alone is never sufficient.

## Performance Objective

Two modes (mutually exclusive), both evaluated against a **frozen** baseline:

**`--min-decode N`** — absolute minimum acceptable decode throughput (tok/s).
Gates the frontier **and** profile eligibility exactly as stated. When set, the
frontier sweep is skipped entirely if the REFERENCE itself misses it (growing
context cannot recover throughput).

**`DecodeRetention` (workload-relative)** — minimum acceptable decode throughput
*relative* to the **frozen REFERENCE baseline** measured before frontier
exploration. `EffectiveFloor()` = `Retention × ReferenceBaseline`; the floor is
fixed for the entire run. A later faster candidate is tracked as best-observed
stable decode and may be recommended, but it **never redefines** the gate floor.
This makes the objective path/order-independent and hardware-relative by
construction: an H100 and a laptop are each judged against their own REFERENCE.

| Workload | `MinContext` | `DecodeRetention` | Meaning |
|---|---|---|---|
| `agentic_coding` | 16384 | 0.75 | A larger window is practical while it retains ≥ 75% of the frozen reference baseline decode (prefill+depth bound; tolerates some decode loss for window). |
| `chat` | 4096 | 0.85 | A larger window is practical while it retains ≥ 85% of the frozen reference baseline decode (decode-bound; responsiveness dominates). |

Neither declared → ranking orders by workload utility alone (stable execution).

Decisions use the **conservative lower bound** `mean − half-range` of repeated
probes (`--perf-runs`, default 3). Noise can never promote a point past the bar.
OOM or timeout during probes fails unconditionally. Performance alone **never**
overrides the capability gate.

## Capability Verification

Gumi uses **reference-based, paired evaluation** against a REFERENCE
configuration selected by explicit policy: the highest-confidence quality
baseline that is feasible on your hardware — same model, same backend binary,
same prompts, same `seed 42`, `temperature 0`, greedy decoding, f16 KV where
possible.

```
REFERENCE = highest-confidence quality baseline feasible on current hardware
```

The report documents exactly *why* it was chosen (`REFERENCE CONFIGURATION /
Why selected`: memory safety, quality settings, stability, role as the paired
anchor).

The gate (`internal/verify.Gate`, `internal/optimize`):

- requires Tier 1 smoke to pass fully;
- requires Tier 2 rate ≥ reference rate − `slack` (default `0`);
- reference OOM degrades context by half once and retries; reference failure aborts the run (nothing rankable without the anchor).

**Honest wording** (never "proof" or "lossless"):

- **SCREENED** — probes collected (`PROBED`)
- **VERIFIED** — gate `PASSED` and re-tested
- **RECOMMENDED** — among verified, best per profile rule
- **REJECTED** — gate `FAILED` with reason (`capability regression: 9/12 < 10/12`)
- **UNKNOWN** — insufficient evidence (crash/unclassified error)

Verification is empirical and bounded by the included test battery (see
`gumi profiles`). It screens for capability *regression* vs REFERENCE — it does
not prove "same intelligence" mathematically.

## Profiles

Gumi produces four **evidence-backed** profile labels from verified,
objective-satisfying candidates (`internal/search.SelectProfiles`):

| Label | Rule (deterministic tie-break) |
|---|---|
| **MAX CONTEXT** | Largest passing context (less VRAM → faster) |
| **SPEED** | Fastest decode lower bound (less VRAM → smaller ctx) |
| **QUALITY** | Highest capability rate → highest KV fidelity → largest context |
| **BALANCED** | Best workload utility among unlabeled; when exhausted, shares the utility-best labeled candidate |

If configurations are **operationally tied**, Gumi says so. Ties are detected
when repetition ranges overlap at equal capability (and ranking confidence is
`LOW` / `indistinguishable`). The report renders ties in both Markdown and JSON
(`profiles[].tied_with`, `ranking.{level,indistinguishable,note}`); winner
selection under ties falls back to the **safer operating margin**: higher
capability → more VRAM headroom → fewer errors. Labels may collapse — all four
can point at the same underlying configuration when evidence does not justify
different configs. Gumi never invents distinctions.

## Supported Scope

| Dimension | V1 scope |
|---|---|
| Accelerator | CUDA / NVIDIA only, single GPU (any class: consumer, workstation, datacenter) |
| Model format | GGUF (parsed directly; no catalog) |
| Backend | `llama.cpp` (`llama-cli` subprocess) for actual tuning |
| Exports | `llama.cpp` CLI/server, LM Studio, Ollama (compatibility outputs) |
| Local hardware | Workload-aware, capability-verified, context-frontier search |
| Workloads | `agentic_coding`, `chat` |

Explicitly **not** in scope for V1 (and not claimed): ROCm, Metal, Vulkan,
DirectML, CPU-only optimization, multi-GPU, multi-node, tensor parallelism,
runtime scheduling, daemon, continuous learning, automatic quant selection,
model downloading. The backend stays extensible behind `backend.Runner`, but
only `llama.cpp` is implemented.

## CLI

```
gumi tune <model.gguf> [--workload agentic_coding|chat] [--min-decode N] [flags]
gumi inspect <model.gguf> [--json]
gumi probe [--model path] [--bandwidth] [--json]
gumi profiles [--json]
gumi export --config candidates.json --id <id> --target llama.cpp|lmstudio|ollama [--model path]
gumi version
```

Common `gumi tune` flags: `--tier smoke|capability` (default `capability`),
`--perf-runs` (default 3), `--max-refine-steps` (default 4), `--timeout`
minutes (default 10), `--gate-slack` (default 0), `--baseline
'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128'` (human config admitted as a gated
`CURRENT-BASELINE` candidate), `--backend-bin /path/to/llama-cli`, `--out DIR`,
`--dry-run` (plan without a backend). `gumi optimize` is a documented alias.

Full help: `gumi tune --help` (or `gumi tune -h`). Dry run: `gumi tune
model.gguf --workload agentic_coding --dry-run`.

Exit codes: `0` success · `1` `TARGET NOT ACHIEVED` or no verified winner ·
`2` usage errors.

## Validation — Real Hardware Examples

> Presented as **validation examples**, not universal performance guarantees.
> Throughput varies by hardware, driver, llama.cpp build, and model quant. Do
> not treat these as promised tok/s.

Both on **RTX 5070 12 GB**, CUDA 13.2, driver 595.84, `llama-cli` v10360:

**Llama-3.1-8B-Instruct Q4_K_M — workload `chat`**

- Maximum practical context: **65,536 tokens** — capability verified
- Theoretical capacity ≈ 96K on q4_0 KV; ladder capped by training context; all levels ~85 tok/s ≥ floor

**Qwen3-30B-A3B Q4_K_M — workload `agentic_coding`**

- Maximum practical context: **40,960 tokens** — capability verified
- Approximately **30 tok/s decode** in the validated run (e.g. `CTX-40K` q4_0 at 30.4 tok/s, REFERENCE f16 at 30.5 tok/s; `QUALITY` f16 at 27.7 tok/s — all `12/12 (100%)` capability where noted)
- Full ladder/bisection discovery, capability-gated frontier, dominance-pruned variants, re-tested profiles

For broader validation (three model families, two replications, MoE placement
audit, sensitivity ladder), see `docs/experiments/` and `docs/specs/25-evidence-hardening.md`.

## Limitations

From `docs/specs/27-gumi-v1-release-audit.md` (V1 Release Audit — verdict **V1
READY WITH DOCUMENTED LIMITATIONS**):

1. Single-GPU, single-backend verification. Multi-GPU split is out of scope.
2. Frontier sweeps **one** line (reach-maximizing KV × placement); other KV types are evaluated as point variants, not swept.
3. Boundary precision defaults to 2048 tokens within 4 bisection steps; tighter bounds cost more probes.
4. Capability cost dominates — Tier-2 battery is the dominant cost; intermediate levels carry no capability verdict unless promoted (`PROBED`).
5. Flat-throughput ties: on GPUs where decode barely moves with context, profiles legitimately collapse and are reported as tied.
6. One discarded generation warms up file-cache/allocator; deeper warmup (KV pre-fill) is future work.
7. Windows/macOS: process management and RSS sampling have platform files; CUDA probing assumes `nvidia-smi`. Untested platforms stay untested.
8. Second-GPU-class validation: V1 validated two model shapes on one GPU class (RTX 5070). An A100/H100-class pass remains desirable but is not a blocker.
9. Frozen reference baseline: the workload threshold is anchored on the **frozen REFERENCE baseline** (stable decode at the conservative control, measured before exploration); the best observed stable decode is reported separately as `best_observed_decode_tps` and never redefines the floor (path/order-independent by construction).
10. `lspci` fallback: without `nvidia-smi`, vendor entries may appear without VRAM; duplicate suppression prevents corruption when VRAM data exists (fixed in audit).

See §14 of the release audit for the full text.

## Repository Layout

```
cmd/gumi/          CLI entrypoint (commands: tune/inspect/probe/profiles/export)
internal/gguf/     GGUF metadata parser + geometry derivation (dependency-free)
internal/hardware/ Hardware prober (Linux-first; parsers are pure fixtures)
internal/workload/ Workload profiles, golden benchmark groups, verification suites
                   └─ agentic_coding/tests/  repository fixtures + exec evaluator
internal/candidate/ Deterministic candidate generator + reference policy + feasibility math
internal/backend/  llama.cpp subprocess runner, capability discovery, exports
internal/search/   Pure tuning strategy: frontier ladder, bisection, objective, dominance, profile selection
internal/verify/   Perf probing, capability suites, paired gate logic
internal/confidence/ Deterministic HIGH/MEDIUM/LOW + ranking confidence
internal/optimize/ Tuner orchestration (staged search loop)
internal/report/   Markdown + JSON report rendering
runtime/ dashboard/ benchmark/ …   Legacy pre-pivot components (frozen; own Go modules)
```

The pre-pivot Gumi (OpenAI-compatible reliability runtime + dashboard, specs
`00`–`22` + `GEP_v1`) is frozen in place under its own modules while migration
decisions are made. The current V1 product is the auto-tuner specified in
`docs/specs/26-gumi-v1-auto-tuner.md` (with evidence engine preserved from
`23`–`25`). Historical specs note this at the top.

## Development

```bash
make build        # ./gumi
make test         # go test ./internal/... ./cmd/...
make vet          # go vet ./internal/... ./cmd/...
make fmt          # gofmt -w cmd internal
```

`go.work` includes `.`, `./runtime`, `./benchmark`. **Root** make/CI targets use
`./internal/... ./cmd/...` (not `./...`), because `dashboard/node_modules`
contains stray vendored Go packages.

Real optimization runs need `llama-cli` (llama.cpp) on `PATH` or via
`--backend-bin`; none is bundled. `gumi inspect` reads GGUF metadata directly —
no manually maintained model catalog. KV-cache arithmetic is exact from GGUF
geometry.

Flag drift across llama.cpp versions is handled by a `--help` probe plus retry
chain (`internal/backend`).

## Product Language

Preferred (used throughout current docs): **local inference auto-tuner**,
**hardware-aware inference tuning**, **measured configuration**, **verified
configuration**, **practical context frontier**, **capability gate**,
**performance objective**.

Avoided for V1: *AI-powered optimization*, *intelligent scheduler*, *guaranteed
optimal*, *same intelligence*, *lossless*, *best possible configuration* — unless
explicitly qualified by measured evidence. Verification is **screened / verified
/ recommended / rejected / unknown**, not "proved".

## License

See [LICENSE](LICENSE).
