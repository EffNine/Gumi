# 23 — Optimization Engine MVP — Preserved Evidence Engine (V1 Component)

Version: 1.0
Status: **Implemented (MVP) — preserved inside V1 auto-tuner**; extended by Phase 2 — see
`24-verification-confidence-phase2.md` for the reference selection policy,
confidence scoring, objective agentic-coding fixtures, MoE whitelist,
golden benchmark groups, and report rework.
Supersedes for product direction: `21-auto-fit-engine-specification.md`
(runtime-coupled fit planning; the runtime is now frozen legacy).

---

# 1. Purpose and positioning

Gumi pivots from "reliability runtime + dashboard" to a **local inference auto-tuner** (V1 product name: local inference auto-tuner; engineering name in this spec: *Local LLM Optimization Engine*):

> Find the best **verified** inference configuration for a given GGUF model
> on the user's hardware and workload.

Hard constraints (product, 2026-08-24):

- CLI first; optional TUI for visualization only.
- No dashboard, web UI, cloud service, account system, database-heavy
  architecture, model hosting, or chat interface.
- Simple scripts/pipeline, local-first, reproducible, engineering-tool style.
- Implementation language: Go (existing project language), zero external
  dependencies in the root module.

Gumi is not a launcher, benchmark, model server, dashboard, or coding agent. It is a local inference auto-tuner answering:
*"I have this model and this PC — what is the best verified way to run it
without sacrificing capability?"*

# 2. Pipeline

```
MODEL → Geometry Inspector → Hardware Probe → Candidate Generator
      → Backend Tester → Capability Verification → Profile Generator
      → Report Export
```

Primary UX (V1): `gumi tune <model.gguf> [--workload agentic_coding|chat] [--min-decode N]` (`optimize` is a documented alias). For the V1 scope and constraints see `26-gumi-v1-auto-tuner.md`.

# 3. Components (as implemented)

## 3.1 GGUF inspector (`internal/gguf`)

Dependency-free GGUF v2/v3 reader. Parses header, all metadata KVs (all
value types incl. arrays), and the tensor directory; never reads tensor data.

Derives: architecture, parameter count (exact tensor-element sum),
quantization label (general.file_type), layer count, hidden size,
attention/KV heads, head dim, training context, RoPE settings, MoE metadata
(total/active experts, expert FFN size), weight bytes and expert-tensor byte
share.

KV-cache arithmetic is exact:
`bytes/token = ceil(2 × layers × kv_heads × head_dim / block) × block_bytes`.
Qwen3-30B-A3B geometry = 96 KiB/token at f16 (tested).

## 3.2 Hardware prober (`internal/hardware`)

Linux-first detection via /proc/cpuinfo, /proc/meminfo, nvidia-smi
(name/VRAM/driver/compute-cap), rocm-smi, lspci fallback, statfs filesystem
identification with mmap capability, plus an opt-in streaming memory
bandwidth micro-benchmark. Non-linux platforms degrade gracefully.

**Never fabricates values:** unknown stays unknown. Parsers are pure
functions tested against fixtures.

## 3.3 Workload profiles (`internal/workload`)

Exactly two MVP profiles, code-defined (no DSL):

| profile | min ctx | quality/latency | suites |
|---|---|---|---|
| agentic_coding | 16384 | 0.65 / 0.35 | smoke(3) + coding×3, retrieval@ctx×0.85 (mid/end), numbered-list instruction |
| chat | 4096 | 0.55 / 0.45 | smoke(3) + logic, arithmetic, JSON-schema instruction, retrieval |

Tasks are deterministic: fixed seeds, temperature 0, validators as pure
functions (exact/contains/numeric/JSON/bullet-numbered format/retrieval
codes embedded in seeded haystacks).

## 3.4 Candidate generator (`internal/candidate`)

Deterministic, ≤5 candidates: `REFERENCE`, `QUALITY`, `BALANCED`, `SPEED`,
plus one conditional (`EXPERT-SPLIT` for MoE models stranding experts, or
`HIGH-CONTEXT`). Feasibility from exact KV math + conservative compute
overhead estimates against VRAM/RAM budgets (95% VRAM safety factor, 4 GiB
RAM headroom). Infeasible candidates are kept and labeled with reasons.

Safe auto-tuning surface: GPU offload (-ngl), expert placement
(`-ot exps=CPU`), context length, KV precision (f16/q8_0/q4_0), flash
attention, batch/ubatch, threads, mmap, mlock. Recommend-only: quantization.
Never touched: weights, active experts, RoPE scaling, system prompt,
sampling behavior.

## 3.5 Backend (`internal/backend`)

MVP verification backend: llama.cpp `llama-cli` subprocess.

- Version-drift handling: one-time `--help` probe selects flash-attention
  syntax (`-fa on|off` vs legacy bare `-fa`) and `-no-cnv` support; unknown-
  argument failures retry legacy forms once.
- Timing parsing supports current and historical perf line formats.
- OOM classification maps stderr markers to typed errors.
- Peak VRAM sampled via nvidia-smi polling (delta over baseline); peak RAM
  via child-process RSS polling. Sampling failures leave metrics unknown.
- Process groups + SIGKILL on timeout.
- Exports are static renders of any config: llama.cpp cli/server commands,
  LM Studio load-settings JSON (with honest notes where the API lacks knobs),
  Ollama Modelfile.

Priority backlog: LM Studio API backend, then Ollama export round-trip.

## 3.6 Verification engine (`internal/verify`)

Performance probe per candidate: profile-sized filler prompt (prefill) +
fixed generation (decode), parsed t/s, TTFT, peak memory.

Capability tiers:

- Tier 1 Smoke (always): output validity, instruction following, formatting.
- Tier 2 Capability (default): coding, reasoning, long-context retrieval,
  structured instructions.
- Tier 3 Deep evaluation: explicitly not implemented in MVP.

**The gate** (`verify.Gate`): a candidate is rejected unless smoke passes
fully and its Tier-2 rate ≥ reference rate − slack (default slack 0).
Reference OOM degrades context by half and retries once. A faster config
that regresses capability is always rejected — this is the core
differentiator and is exercised end-to-end with a fake backend in
`internal/optimize/pipeline_test.go`.

Ranking among gate passers:
`score = Q·capability_rate + L·(0.7·norm(decode t/s) + 0.3·norm(prefill t/s))`,
deterministic tie-breaks by generation order.

## 3.7 Report & artifacts (`internal/report`, `internal/optimize`)

Per run (default `reports/<model>-<workload>-<timestamp>/`): `report.md`
(human), `report.json` (machine), `candidates.json` (full configs +
measurements + gate verdicts), `hardware.json`. Winner section includes
verified performance, capability tier result, and all four exports.
`--dry-run` produces plans without any backend.

# 4. Reproducibility contract

Same model file + same machine state + same flags ⇒ same candidate set,
same prompts/seeds, same gate decisions, same winner (up to measurement
noise, which never flips gate outcomes without slack). All randomness is
seeded; no wall-clock input enters planning decisions.

# 5. Out of scope (MVP)

Dashboard/TUI beyond text output, LM Studio/Ollama API backends, Tier 3 deep
evaluation, multi-GPU tensor splitting, speculative decoding, sampler tuning,
cloud anything.

# 6. Verification evidence

- `go vet ./internal/... ./cmd/...` clean; gofmt clean; zero external deps.
- `go test ./internal/... ./cmd/...` green, including:
  - exact KV math vs spec example (96 KiB/token),
  - deterministic ≤5 candidate generation across hardware shapes,
  - timing parsers (new+old llama.cpp formats), OOM classification,
  - validator suite positives/negatives, haystack determinism,
  - paired-gate rejection of a fast-but-dumb candidate (E2E fake backend),
  - CLI end-to-end through the real dispatcher (inspect/probe/profiles/
    optimize --dry-run) with synthetic GGUF fixtures.
