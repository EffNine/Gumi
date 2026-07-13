# Novexa Benchmarks

Reproducible benchmarks comparing direct provider calls vs Novexa Runtime.

The unified benchmark framework is now the primary benchmarking tool.

## Quick Start

```bash
# 1. Start LM Studio with a model loaded
# 2. Start Novexa
./novexa start

# 3. Run the unified benchmark
novexa benchmark --model ornith-1.0-9b@q4_k_m --mode auto
```

## Specification

The design, architecture, data model, scoring methodology, and implementation
roadmap for the Novexa Benchmark Suite are defined in:

📄 [`docs/specs/22-benchmark-specification.md`](../docs/specs/22-benchmark-specification.md)

Key features of the new framework:
- **Single command**: `novexa benchmark --model <name>`
- **Calibrated difficulty tiers**: Easy / Medium / Hard / Frontier
- **Per-capability scoring**: JSON, instruction-following, tool-calling, reasoning, repetition
- **Statistical rigor**: Multiple attempts, Cohen's d effect sizes, confidence intervals
- **Degradation detection**: Measures whether Novexa ever makes correct output worse
- **Frontier baseline**: Optional comparison to GPT-4o / Claude / Fable 5

## Calibration Results

Calibrated against `ornith-1.0-9b@q4_k_m` via LM Studio:

| Tier | Target | Direct | Novexa | Status |
|------|--------|--------|--------|--------|
| Easy | 70-90% | 50% | **75%** | ✅ |
| Medium | 40-70% | 0% | **50%** | ✅ |
| Hard | 10-40% | 8% | **33%** | ✅ |

Novexa improved every capability with large effect sizes (Cohen's d > 0.8 for
JSON, tool-calling, and reasoning). Degradation rate was 16.7%.

## Model Recommendations

| Model | Why |
|---|---|
| `ornith-1.0-9b@q4_k_m` | Calibrated against this model. Good balance of speed + quality. |
| `qwen/qwen3.5-9b` | Strong coding, good instruction following |
| `qwen2.5-coder-7b-instruct` | Specialized coding model |

## Structure

```
benchmarks/
├── README.md           # this file
├── reports/            # final summary reports (committed to repo)
└── archive/            # historical raw results (for reference only)
```

Raw benchmark outputs are stored **outside** the repository at
`~/.novexa/benchmarks/` to keep the repo small.

## Legacy Benchmarks (Deprecated)

These scripts remain for reference but are superseded by the unified framework:

| Benchmark | Script | What it measures |
|---|---|---|
| **IFEval** | `scripts/benchmark-standard-scorecard.sh` | Instruction following accuracy |
| **Agentic Coding** | `scripts/benchmark-agentic-coding.sh` | Tool calling, JSON, multi-turn |
| **Local Model** | `scripts/benchmark-local-model.sh` | Basic reliability |
| **LM Studio Matrix** | `scripts/benchmark-lmstudio-matrix.sh` | Multi-model comparison |
| **Terminal-Bench** | `scripts/benchmark-terminal-bench.sh` | Agentic coding tasks (Docker) |
