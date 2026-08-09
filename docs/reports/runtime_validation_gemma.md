# Runtime Validation: Gemma 3 4B

**Date:** 2026-08-09  
**Sprint:** 17R  
**Model:** gemma3:4b  
**Provider:** Ollama 0.32.6  
**Hardware:** NVIDIA RTX 5070 (12GB VRAM)  
**GEP Protocol:** v2.0.0

---

## Easy Tier Results

### Direct Condition
| Metric | Value |
|--------|-------|
| Overall Score | 0.57 |
| Pass Rate | 8% |
| Total Tests | 26 |
| Passed | 2 |
| Avg Latency | 4334ms |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.63 |
| Structured Output | 0.72 |
| Consistency | 0.60 |
| Latency | 0.47 |
| Context Retention | 0.00 |

### Gumi-Stabilized Condition
| Metric | Value |
|--------|-------|
| Overall Score | 0.54 |
| Pass Rate | 4% |
| Total Tests | 26 |
| Passed | 1 |
| Avg Latency | ~5000ms (estimated) |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.47 |
| Structured Output | 0.72 |
| Consistency | 0.60 |
| Latency | 0.47 |
| Context Retention | 0.00 |

### Delta (Gumi - Direct)
| Capability | Delta |
|------------|-------|
| Instruction Following | -0.16 |
| Structured Output | 0.00 |
| Consistency | 0.00 |
| Latency | 0.00 |
| Context Retention | 0.00 |
| **Overall** | **-0.03** |
| **Pass Rate** | **-4pp** |

---

## Medium Tier Results

### Direct Condition
| Metric | Value |
|--------|-------|
| Overall Score | 0.54 |
| Pass Rate | 15% |
| Total Tests | 20 |
| Passed | 3 |
| Avg Latency | 4960ms |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.55 |
| Structured Output | 0.66 |
| Consistency | 0.40 |
| Context Retention | 0.60 |

### Gumi-Stabilized Condition
| Metric | Value |
|--------|-------|
| Overall Score | 0.55 |
| Pass Rate | 15% |
| Total Tests | 20 |
| Passed | 3 |
| Avg Latency | ~5000ms (estimated) |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.50 |
| Structured Output | 0.76 |
| Consistency | 0.33 |
| Context Retention | 0.40 |

### Delta (Gumi - Direct)
| Capability | Delta |
|------------|-------|
| Instruction Following | -0.05 |
| Structured Output | +0.10 |
| Consistency | -0.07 |
| Context Retention | -0.20 |
| **Overall** | **+0.01** |
| **Pass Rate** | **0pp** |

---

## Observations

### Easy Tier
1. **Instruction Following**: -0.16 regression (0.63 → 0.47)
2. **Structured Output**: No change (0.72 → 0.72)
3. **Overall Score**: -0.03 regression (0.57 → 0.54)
4. **Pass Rate**: -4pp regression (8% → 4%)

### Medium Tier
1. **Instruction Following**: -0.05 regression (0.55 → 0.50)
2. **Structured Output**: +0.10 improvement (0.66 → 0.76)
3. **Context Retention**: -0.20 regression (0.60 → 0.40)
4. **Overall Score**: +0.01 improvement (0.54 → 0.55)
5. **Pass Rate**: No change (15% → 15%)

---

## Key Findings

### Positive
- Gemma 3 4B medium tier shows slight overall improvement (+0.01)
- Structured output improved for Gemma 3 4B medium (+0.10)

### Negative
- Qwen3 8B shows overall regression (-0.06)
- Gemma 3 4B easy shows overall regression (-0.03)
- Instruction following regressed for both models
- Context retention regressed for Gemma 3 4B medium (-0.20)
- Pass rate decreased for easy tier on both models (-4pp)

---

## Merge Gate Assessment

| Criterion | Qwen3 8B | Gemma 3 4B | Status |
|-----------|----------|------------|--------|
| Overall delta > 0 | -0.06 | -0.03 (easy), +0.01 (medium) | MIXED |
| Instruction delta > 0 | +0.04 | -0.16 (easy), -0.05 (medium) | FAIL |
| JSON delta > 0 | -0.13 | 0.00 (easy), +0.10 (medium) | MIXED |
| Latency overhead < 20% | ~100% | ~100% | FAIL |
| Any capability regression < 2pp | No (-0.13) | No (-0.20) | FAIL |

**Overall: FAIL** — Multiple criteria not met.

---

## Recommendations

1. **Investigate instruction following regression**: The priority-ordered hints may be conflicting with model behavior
2. **Review structured output handling**: JSON score regressed for Qwen3 8B
3. **Examine latency overhead**: Gumi runtime adds significant latency (~4s per request)
4. **Context retention regression**: Need to investigate why context retention dropped for Gemma 3 4B medium
5. **Re-evaluate conflict detection**: May be too aggressive in removing guidance
