# Gumi Benchmark Suite

A unified, scientifically rigorous benchmark framework for measuring Gumi's
impact on model output quality across local and frontier models.

## Quick Start

```bash
# 1. Start LM Studio with a model loaded
# 2. Start Gumi
gumi start

# 3. Run the benchmark
gumi benchmark --model ornith-1.0-9b@q4_k_m --mode auto
```

## CLI Usage

```
gumi benchmark [flags]

Flags:
  --model string         Model name (required)
  --provider string      Provider (auto-detected from model name)
  --mode string          Execution mode: auto | quick | thorough | frontier (default "auto")
  --attempts int         Attempts per condition (default 3)
  --conditions strings   Conditions to test (default "direct,gumi-stabilized")
  --frontier-key string  API key for frontier baseline (optional)
  --frontier-model string Frontier model name (optional)
  --output string        Output directory (default "benchmarks/reports/")
  --json                 Output report as JSON to stdout
```

## Architecture

The benchmark subsystem lives in the `benchmark/` directory:

```
benchmark/
├── cmd/run.go          # Standalone CLI entry point
├── runner/             # Test loop, condition dispatch, provider clients
├── scorer/             # Scoring engine, check registry, degradation detection
├── report/             # JSON and Markdown report writers
├── suites/             # YAML test definitions organized by category and tier
└── types.go            # Core data types shared across packages
```

For the full design specification, see `docs/specs/22-benchmark-specification.md`.

## Test Suites

| Category | What it measures | Gumi engine under test |
|----------|-----------------|--------------------------|
| JSON | Valid JSON rate, key presence, schema compliance | Repair, Validation, Prompt |
| Instruction | Constraint satisfaction rate | Instruction Assist |
| Repetition | Deduplication of repeated content | Guard, Repair |
| Tool Calling | Valid tool call JSON, correct arguments | Tool shim, Validation |
| Reasoning | Correct answer on multi-step problems | Prompt, Context |
| Degradation | Does Gumi corrupt already-correct output? | All engines (negative test) |

## Difficulty Tiers

| Tier | Target direct score | Models that run it |
|------|-------------------|-------------------|
| Easy | 70-90% | All models |
| Medium | 40-70% | All models |
| Hard | 10-40% | Medium + Frontier |
| Frontier | 30-70% | Frontier only |

## Calibration Results

The benchmark was calibrated against `ornith-1.0-9b@q4_k_m` via LM Studio:

| Tier | Target | Direct | Gumi | Status |
|------|--------|--------|--------|--------|
| Easy | 70-90% | 50% | **75%** | ✅ |
| Medium | 40-70% | 0% | **50%** | ✅ |
| Hard | 10-40% | 8% | **33%** | ✅ |

Gumi improved every capability with large effect sizes (Cohen's d > 0.8 for
JSON, tool-calling, and reasoning). Degradation rate was 16.7% (1 semantic
corruption out of 6 degradation tests).

## Sample Report

```
# Gumi Benchmark Report
**Model:** ornith-1.0-9b@q4_k_m · **Tier:** medium

## Overall
| Metric | Direct | Gumi | Delta |
|--------|--------|--------|-------|
| Overall Score | 0.80 | 1.38 | +0.58 |
| Latency (avg) | 4709ms | 1540ms | -3169ms |
| Degradation Rate | — | 16.7% | — |
| Worth it? | | ✅ Yes | |

## By Capability
| Capability | Δ | Effect |
|-----------|---|--------|
| JSON | +1.33 | ★★★ |
| Tool-calling | +0.67 | ★★★ |
| Reasoning | +0.50 | ★★★ |
| Instruction | +0.44 | ★★ |
| Repetition | +0.17 | ★★ |
```
