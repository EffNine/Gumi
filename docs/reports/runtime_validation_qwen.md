# Runtime Validation: Qwen3 8B

**Date:** 2026-08-09  
**Sprint:** 17R  
**Model:** qwen3:8b  
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
| Avg Latency | 2840ms |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.63 |
| Structured Output | 0.76 |
| Consistency | 0.47 |
| Latency | 0.51 |
| Context Retention | 0.00 |

### Gumi-Stabilized Condition
| Metric | Value |
|--------|-------|
| Overall Score | 0.51 |
| Pass Rate | 4% |
| Total Tests | 26 |
| Passed | 1 |
| Avg Latency | ~7000ms (estimated from individual requests) |

| Capability | Score |
|------------|-------|
| Instruction Following | 0.67 |
| Structured Output | 0.63 |
| Consistency | 0.47 |
| Latency | 0.47 |
| Context Retention | 0.00 |

### Delta (Gumi - Direct)
| Capability | Delta |
|------------|-------|
| Instruction Following | +0.04 |
| Structured Output | -0.13 |
| Consistency | 0.00 |
| Latency | -0.04 |
| Context Retention | 0.00 |
| **Overall** | **-0.06** |
| **Pass Rate** | **-4pp** |

---

## Observations

1. **Instruction Following**: +0.04 improvement with Gumi runtime (0.63 → 0.67)
2. **Structured Output**: -0.13 regression with Gumi runtime (0.76 → 0.63)
3. **Overall Score**: -0.06 regression (0.57 → 0.51)
4. **Pass Rate**: -4pp regression (8% → 4%)
5. **Latency**: Gumi adds ~4000ms overhead per request (7s vs 3s direct)

---

## Notes

- Combined direct+gumi-stabilized runs experienced hangs; single-condition runs completed successfully
- The gumi-stabilized-only run shows the runtime is functional but introduces latency overhead
- Instruction following improved slightly but structured output regressed
