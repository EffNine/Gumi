# Benchmark Delta Report — Sprint 17R Live Validation

**Date:** 2026-08-09  
**Sprint:** 17R  
**Protocol:** GEP v2.0.0  
**Environment:** Linux Development PC, NVIDIA RTX 5070, Ollama 0.32.6

---

## Certified Baselines (Sprint 16D)

| Model | Overall Score | Pass Rate | Tests | Source |
|-------|--------------|-----------|-------|--------|
| Qwen3 8B | 0.57 | 8% | 26 | `baseline_v2_qwen.json` |
| Gemma 3 4B | 0.53 | 9% | 195 | `baseline_v2_gemma.json` |

---

## Live Benchmark Results

### Qwen3 8B — Easy Tier

| Condition | Overall | Pass Rate | Instr. | Struct. | Consist. | Latency |
|-----------|---------|-----------|--------|---------|----------|---------|
| Direct | 0.57 | 8% | 0.63 | 0.76 | 0.47 | 0.51 |
| Gumi-Stabilized | 0.51 | 4% | 0.67 | 0.63 | 0.47 | 0.47 |
| **Delta** | **-0.06** | **-4pp** | **+0.04** | **-0.13** | **0.00** | **-0.04** |

### Gemma 3 4B — Easy Tier

| Condition | Overall | Pass Rate | Instr. | Struct. | Consist. | Latency |
|-----------|---------|-----------|--------|---------|----------|---------|
| Direct | 0.57 | 8% | 0.63 | 0.72 | 0.60 | 0.47 |
| Gumi-Stabilized | 0.54 | 4% | 0.47 | 0.72 | 0.60 | 0.47 |
| **Delta** | **-0.03** | **-4pp** | **-0.16** | **0.00** | **0.00** | **0.00** |

### Gemma 3 4B — Medium Tier

| Condition | Overall | Pass Rate | Instr. | Struct. | Context | Consist. |
|-----------|---------|-----------|--------|---------|---------|----------|
| Direct | 0.54 | 15% | 0.55 | 0.66 | 0.60 | 0.40 |
| Gumi-Stabilized | 0.55 | 15% | 0.50 | 0.76 | 0.40 | 0.33 |
| **Delta** | **+0.01** | **0pp** | **-0.05** | **+0.10** | **-0.20** | **-0.07** |

---

## Delta Summary

| Model | Tier | Overall Delta | Instr Delta | JSON Delta | Pass Rate Delta | Latency |
|-------|------|--------------|-------------|------------|-----------------|---------|
| Qwen3 8B | Easy | -0.06 | +0.04 | -0.13 | -4pp | +~4000ms |
| Gemma 3 4B | Easy | -0.03 | -0.16 | 0.00 | -4pp | +~700ms |
| Gemma 3 4B | Medium | +0.01 | -0.05 | +0.10 | 0pp | +~400ms |

---

## Regression Analysis

### Critical Regressions (>2pp)
- **Qwen3 8B Structured Output**: -13pp (0.76 → 0.63)
- **Gemma 3 4B Instruction Following**: -16pp (0.63 → 0.47) easy, -5pp medium
- **Gemma 3 4B Context Retention**: -20pp (0.60 → 0.40) medium
- **Pass Rate**: -4pp for easy tier on both models

### Improvements
- **Qwen3 8B Instruction Following**: +4pp (0.63 → 0.67) easy
- **Gemma 3 4B Structured Output**: +10pp (0.66 → 0.76) medium
- **Gemma 3 4B Overall**: +1pp (0.54 → 0.55) medium

### Latency
- Gumi runtime adds significant latency overhead
- Qwen3 8B: ~4000ms per request (2.8s direct → ~7s via Gumi)
- Gemma 3 4B: ~700ms per request (0.4s direct → ~1.1s via Gumi)

---

## Merge Gate Assessment

| Criterion | Required | Qwen3 8B | Gemma 3 4B | Pass? |
|-----------|----------|----------|------------|-------|
| Overall delta > 0 | Positive | -0.06 | -0.03/+0.01 | FAIL/MIXED |
| Instruction delta > 0 | Positive | +0.04 | -0.16/-0.05 | FAIL |
| JSON delta > 0 | Positive | -0.13 | 0.00/+0.10 | FAIL/MIXED |
| Latency overhead < 20% | <20% | ~100% | ~100% | FAIL |
| Any capability regression < 2pp | <2pp | No (-13pp) | No (-20pp) | FAIL |

**Gate Result: FAIL**

---

## Recommendations

1. **REJECT Sprint 17 merge** — Multiple critical regressions detected
2. **Investigate instruction engine optimizations** — Priority ordering may be causing conflicts
3. **Review conflict detection logic** — May be too aggressive in removing guidance
4. **Analyze latency overhead** — Gumi runtime adds ~4s per request for Qwen3 8B
5. **Next iteration focus**:
   - Reduce hint block token usage
   - Improve JSON output handling
   - Investigate context retention regression
   - Profile latency bottleneck

---

## Live Benchmark Commands

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

## Validation

```
go test ./runtime/...  → 23 packages, ALL PASS
go test ./benchmark/... → 29 packages, ALL PASS
go vet                 → CLEAN
make build             → SUCCESS
```

---

**Report generated:** 2026-08-09  
**GEP Protocol:** v2.0.0  
**Baseline references:** `~/.gumi/gep/baselines/runtime/qwen3:8b/`, `~/.gumi/gep/baselines/runtime/gemma3:4b/`
