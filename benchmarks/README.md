# Novexa Benchmarks

Reproducible benchmarks comparing direct provider calls vs Novexa Runtime.

## Structure

```
benchmarks/
├── README.md           # this file
├── reports/            # final summary reports (committed to repo)
└── archive/            # historical raw results (for reference only)
```

Raw benchmark outputs are stored **outside** the repository at
`~/.novexa/benchmarks/` to keep the repo small.

## Benchmarks

| Benchmark | Script | What it measures | Industry standard? |
|---|---|---|---|
| **IFEval** | `scripts/benchmark-standard-scorecard.sh` | Instruction following accuracy | ✅ Yes (lm-eval) |
| **Agentic Coding** | `scripts/benchmark-agentic-coding.sh` | Tool calling, JSON, multi-turn | ⚠️ Custom |
| **Local Model** | `scripts/benchmark-local-model.sh` | Basic reliability (empty response, JSON) | ⚠️ Custom |
| **LM Studio Matrix** | `scripts/benchmark-lmstudio-matrix.sh` | Multi-model comparison | ⚠️ Custom |
| **Terminal-Bench** | `scripts/benchmark-terminal-bench.sh` | Agentic coding tasks (Docker) | ✅ Yes |

## Quick Start

```bash
# 1. Start LM Studio (or Ollama) with your model loaded
# 2. Start Novexa
./novexa start

# 3. Run a benchmark
DIRECT_BASE_URL=http://192.168.0.164:1234/v1 \
NOVEXA_BASE_URL=http://127.0.0.1:8787/v1 \
./scripts/benchmark-local-model.sh ornith-1.0-9b@q4_k_m
```

## Model Recommendations for Benchmarking

| Model | Why |
|---|---|
| `ornith-1.0-9b@q4_k_m` | Good balance of speed + quality, strong JSON |
| `qwen/qwen3.5-9b` | Strong coding, good instruction following |
| `qwen2.5-coder-7b-instruct` | Specialized coding model |

## Reading Results

Results are written to `~/.novexa/benchmarks/<benchmark>/<run-id>/`.

Each run produces:
- `<label>/results.json` — machine-readable results
- `report.md` — human-readable summary

## Methodology

All benchmarks follow the same pattern:

1. **Direct**: Request sent straight to the provider (LM Studio / Ollama)
2. **Novexa**: Request sent through Novexa Runtime with a model profile

Novexa never modifies models. It improves the *layer around* the model:
prompt building, context management, JSON repair, anti-loop guards.
