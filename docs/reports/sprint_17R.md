# Sprint 17R Report — Live GEP Runtime Validation

**Date:** 2026-08-09  
**Sprint:** 17R  
**Status:** FAIL — Merge Gate Not Passed  
**Protocol:** GEP v2.0.0

---

## Primary Objective

Execute certified GEP v2.0.0 benchmark against the optimized Gumi runtime to validate Sprint 17 instruction engine improvements.

---

## Environment

| Property | Value |
|----------|-------|
| Machine | Linux devpc (Development PC) |
| OS | Ubuntu 24.04, kernel 7.0.0-29-generic |
| CPU | x86_64 |
| GPU | NVIDIA RTX 5070 (12GB VRAM) |
| Ollama | 0.32.6 |
| Models | qwen3:8b, gemma3:4b |
| Gumi Runtime | v0.2.0-alpha-35-g0bd2864-dirty |
| GEP Protocol | v2.0.0 |

---

## Validation Results

### Pre-Benchmark Validation

```
go test ./runtime/...    → 23 packages, ALL PASS
go test ./benchmark/...  → 29 packages, ALL PASS
go vet                   → CLEAN
make build               → SUCCESS
```

### Live Benchmark Execution

| Run | Model | Tier | Condition | Overall | Pass Rate | Tests |
|-----|-------|------|-----------|---------|-----------|-------|
| 1 | qwen3:8b | easy | direct | 0.57 | 8% | 26 |
| 2 | qwen3:8b | easy | gumi-stabilized | 0.51 | 4% | 26 |
| 3 | gemma3:4b | easy | direct | 0.57 | 8% | 26 |
| 4 | gemma3:4b | easy | gumi-stabilized | 0.54 | 4% | 26 |
| 5 | gemma3:4b | medium | direct | 0.54 | 15% | 20 |
| 6 | gemma3:4b | medium | gumi-stabilized | 0.55 | 15% | 20 |

---

## Delta Analysis

### Qwen3 8B — Easy Tier

| Capability | Direct | Gumi | Delta | Status |
|------------|--------|------|-------|--------|
| Instruction Following | 0.63 | 0.67 | +0.04 | ✅ |
| Structured Output | 0.76 | 0.63 | **-0.13** | ❌ |
| Consistency | 0.47 | 0.47 | 0.00 | ⚠️ |
| Latency | 0.51 | 0.47 | -0.04 | ⚠️ |
| **Overall** | **0.57** | **0.51** | **-0.06** | ❌ |
| **Pass Rate** | **8%** | **4%** | **-4pp** | ❌ |

### Gemma 3 4B — Easy Tier

| Capability | Direct | Gumi | Delta | Status |
|------------|--------|------|-------|--------|
| Instruction Following | 0.63 | 0.47 | **-0.16** | ❌ |
| Structured Output | 0.72 | 0.72 | 0.00 | ⚠️ |
| Consistency | 0.60 | 0.60 | 0.00 | ⚠️ |
| **Overall** | **0.57** | **0.54** | **-0.03** | ❌ |
| **Pass Rate** | **8%** | **4%** | **-4pp** | ❌ |

### Gemma 3 4B — Medium Tier

| Capability | Direct | Gumi | Delta | Status |
|------------|--------|------|-------|--------|
| Instruction Following | 0.55 | 0.50 | -0.05 | ⚠️ |
| Structured Output | 0.66 | 0.76 | +0.10 | ✅ |
| Context Retention | 0.60 | 0.40 | **-0.20** | ❌ |
| Consistency | 0.40 | 0.33 | -0.07 | ⚠️ |
| **Overall** | **0.54** | **0.55** | **+0.01** | ✅ |
| **Pass Rate** | **15%** | **15%** | **0pp** | ⚠️ |

---

## Merge Gate Assessment

| Criterion | Required | Qwen3 8B | Gemma 3 4B | Result |
|-----------|----------|----------|------------|--------|
| Overall delta > 0 | Positive | -0.06 | -0.03/+0.01 | **FAIL** |
| Instruction delta > 0 | Positive | +0.04 | -0.16/-0.05 | **FAIL** |
| JSON delta > 0 | Positive | -0.13 | 0.00/+0.10 | **FAIL** |
| Latency overhead < 20% | <20% | ~100% | ~100% | **FAIL** |
| Any capability regression < 2pp | <2pp | No (-13pp) | No (-20pp) | **FAIL** |

### Gate Result: **FAIL**

---

## Key Findings

### Regressions
1. **Structured Output (Qwen3 8B)**: -13pp regression (0.76 → 0.63)
2. **Instruction Following (Gemma 3 4B)**: -16pp regression easy, -5pp medium
3. **Context Retention (Gemma 3 4B)**: -20pp regression medium
4. **Pass Rate**: -4pp regression easy tier on both models
5. **Latency**: Gumi runtime adds ~4s overhead per request

### Improvements
1. **Instruction Following (Qwen3 8B)**: +4pp improvement (0.63 → 0.67)
2. **Structured Output (Gemma 3 4B medium)**: +10pp improvement (0.66 → 0.76)
3. **Overall (Gemma 3 4B medium)**: +1pp improvement (0.54 → 0.55)

---

## Root Cause Analysis

### Likely Causes of Regression

1. **Priority-ordered hints may confuse models**: Placing JSON first in hint block may interfere with non-JSON tasks
2. **Conflict detection too aggressive**: Removing "think step-by-step" guidance may harm complex reasoning
3. **Hint block verbosity**: Additional conflict warnings and grouping increase token usage
4. **Runtime overhead**: Gumi pipeline adds ~4s per request, affecting latency-sensitive tests

### Specific Issues

- **Structured output regression**: The JSON priority promotion may cause models to over-index on JSON format even when not requested
- **Instruction following regression**: Conflict detection may be removing useful guidance
- **Context retention regression**: Additional system prompt text may consume context budget

---

## Recommendations

### Immediate
1. **REJECT Sprint 17 merge** — Merge gate not passed
2. **Revert instruction engine optimizations** — Regressions exceed acceptable thresholds
3. **Profile latency** — Identify bottleneck in Gumi pipeline

### Next Iteration
1. **Reduce hint block size** — Target <50 tokens for hint injection
2. **Make priority ordering conditional** — Only reorder for explicit format constraints
3. **Softene conflict detection** — Warn but don't remove guidance
4. **Add latency budget** — Cap Gumi overhead at <500ms
5. **Re-test with smaller hint blocks** — Validate before re-submitting

---

## Deliverables

| File | Status |
|------|--------|
| `docs/reports/runtime_validation_qwen.md` | ✅ |
| `docs/reports/runtime_validation_gemma.md` | ✅ |
| `docs/reports/runtime_delta.md` | ✅ |
| `docs/reports/sprint_17R.md` | ✅ This file |
| `~/.gumi/gep/reports/gep-qwen3-8b-*.json` | ✅ 6 reports |
| `~/.gumi/gep/reports/gep-gemma3-4b-*.json` | ✅ 4 reports |

---

## Live Benchmark Commands (for reference)

```bash
# Qwen3 8B Easy Direct
gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 --attempts 1 --difficulty easy --conditions direct --scope runtime

# Qwen3 8B Easy Gumi-Stabilized
gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local --attempts 1 --difficulty easy --conditions gumi-stabilized --scope runtime

# Gemma 3 4B Easy Direct
gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 --attempts 1 --difficulty easy --conditions direct --scope runtime

# Gemma 3 4B Easy Gumi-Stabilized
gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local --attempts 1 --difficulty easy --conditions gumi-stabilized --scope runtime

# Gemma 3 4B Medium Direct
gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 --attempts 1 --difficulty medium --conditions direct --scope runtime

# Gemma 3 4B Medium Gumi-Stabilized
gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local --attempts 1 --difficulty medium --conditions gumi-stabilized --scope runtime
```

---

**Sprint 17R Complete — FAIL**

Sprint 17 changes are REJECTED pending next iteration addressing regressions.
