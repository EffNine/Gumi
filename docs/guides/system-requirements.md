# System Requirements — V1 Local Inference Auto-Tuner

## Supported scope (V1)

| Dimension | V1 scope | Out of scope |
|---|---|---|
| Accelerator | CUDA / NVIDIA, **single GPU** (consumer, workstation, datacenter) | ROCm, Metal, Vulkan, DirectML, CPU-only, multi-GPU, multi-node, tensor parallelism |
| Model format | **GGUF** (parsed directly; no catalog) | Other formats |
| Backend | **llama.cpp** (`llama-cli` subprocess) for actual tuning | Other inference runtimes as verification backends |
| Exports | `llama.cpp` CLI/server, **LM Studio**, **Ollama** (compatibility outputs) | — |
| Tuning | Local hardware, **workload-aware** (`agentic_coding` / `chat`), **capability-verified**, **context-frontier search** | Remote/cluster scheduling, cloud inference |
| Workloads | `agentic_coding` (MinContext 16384, retain ≥75% decode), `chat` (MinContext 4096, retain ≥85% decode) | — |

Gumi is a **local inference auto-tuner**, not a runtime, model server, dashboard, web UI, coding agent, quantizer, or continuous-learning system.

## Minimum requirements

| Resource | Minimum | Notes |
|---|---|---|
| OS | Linux (primary target); macOS/Windows untested but not blocked | CUDA probing assumes `nvidia-smi`; process/RSS has platform files (linux-primary) |
| Accelerator | NVIDIA GPU with CUDA driver + `nvidia-smi` | Single-GPU verification; whatever `llama.cpp` does with visible devices is what gets measured |
| RAM | Model-dependent (GGUF file size + KV cache). Gumi itself adds negligible overhead | Planning uses probed VRAM/RAM × 95% safety factor + 4 GiB RAM headroom; measurement overrides planning |
| Disk | ~50 MB for Gumi binary; model files vary (2–40 GB+) | Reports are written to `--out` (default `reports/<model>-<workload>-<timestamp>/`) |
| Software | Go 1.25+ to build; `llama-cli` (CUDA build recommended) on `PATH` or `--backend-bin` for real tuning | No backend needed for `inspect` / `probe` / `profiles` / `--dry-run` |
| Model | Any GGUF with valid `general.file_type` and attention geometry | MoE expert placement auto-tuned only for whitelist: `qwen2moe` / `qwen3moe` / `mixtral` / `deepseek2` |

## What Gumi measures, not assumes

- **Never fabricates values.** Unknown stays unknown (`internal/hardware`). Parsers are pure functions tested against fixtures.
- No RTX-model, VRAM-size, generation, bandwidth, tensor-core, or throughput assumptions exist in `internal/*`; the same engine must serve a laptop RTX and an H100.
- KV-cache arithmetic is **exact** from GGUF geometry: `2 × layers × kv_heads × head_dim × bytes-per-elem` (ggml block sizes for quantized KV). Qwen3-30B-A3B geometry = 96 KiB/token at f16 — covered by tests.
- `gumi inspect` reads GGUF metadata directly — no manually maintained model catalog.

## Workload contracts

| Workload | `MinContext` | `DecodeRetention` | Perf probe shape | Priority |
|---|---|---|---|---|
| `agentic_coding` | 16384 | 0.75 (prefill+depth bound; tolerates decode loss for window) | 1536 prompt / 160 gen | quality 0.65 |
| `chat` | 4096 | 0.85 (decode-bound; responsiveness dominates) | 256 prompt / 128 gen | balanced 0.55/0.45 |

There is **no universal tok/s floor**. Floors come from `--min-decode` (absolute) or the workload's relative retention rule anchored on the **frozen REFERENCE baseline** (stable decode before exploration; best observed tracked separately as `best_observed_decode_tps` and never redefines the floor). See `26-gumi-v1-auto-tuner.md` §4 and `README.md` §Performance Objective.

## Validation provenance (not guarantees)

Both examples below are on **RTX 5070 12GB**, CUDA 13.2, driver 595.84, `llama-cli` v10360 — presented as **validation examples**, not universal performance claims:

- **Llama-3.1-8B-Instruct Q4_K_M** · `chat` · maximum practical context **65,536 tokens** · capability verified
- **Qwen3-30B-A3B Q4_K_M** · `agentic_coding` · maximum practical context **40,960 tokens** · capability verified · ~30 tok/s decode in the validated run

Theoretical vs practical context are reported separately (exact arithmetic vs measured, capability-gated). See `27-gumi-v1-release-audit.md` and `docs/experiments/` for full evidence.

## Network / services

- **Local-first, zero cloud dependencies.** No database, no account system, no external API required.
- No internet needed after installation (except to fetch models/llama.cpp by your own tooling).
- No ports are bound. Gumi runs as a CLI pipeline of short-lived subprocesses.

## When Gumi cannot run

- No CUDA device visible → falls back to CPU-only path where `MaxContextFor` derives from RAM (measured capacity, not claimed throughput).
- No GGUF geometry → refuses to plan (never silently estimates).
- No `llama-cli` → `inspect`/`probe`/`profiles`/`--dry-run` still work; `tune` without `--dry-run` returns an actionable error.
