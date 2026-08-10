# GEP 17R3 Validation Report

**Date:** 2026-08-10  
**Protocol:** GEP v2.0.0  
**Environment:** Linux Development PC, NVIDIA RTX 5070 12GB, Ollama 0.32.6

---

## Pre-Validation

```bash
go test ./runtime/...     → ALL PASS (54 instruction tests)
go vet ./runtime/...      → CLEAN
make build                → SUCCESS
```

---

## Benchmark Results

### Qwen3 8B Easy

| Metric | Direct | Gumi 17R2 | Gumi 17R3 | Delta vs 17R2 |
|--------|--------|-----------|-----------|---------------|
| Overall | 0.57 | 0.57 | TBD | TBD |
| Instruction | 0.63 | 0.73 | TBD | TBD |
| Structured Output | 0.76 | 0.76 | TBD | TBD |
| Consistency | 0.47 | 0.47 | TBD | TBD |
| Context | 0.00 | 0.00 | TBD | TBD |
| Pass Rate | 8% | 12% | TBD | TBD |
| Latency | 3298ms | 1310ms | TBD | TBD |

### Qwen3 8B Medium

| Metric | Direct | Gumi 17R2 | Gumi 17R3 | Delta vs 17R2 |
|--------|--------|-----------|-----------|---------------|
| Overall | 0.60 | 0.59 | TBD | TBD |
| Instruction | 0.55 | 0.50 | TBD | TBD |
| Structured Output | 0.87 | 0.87 | TBD | TBD |
| Consistency | 0.33 | 0.33 | TBD | TBD |
| Context | 0.60 | 0.57 | TBD | TBD |
| Pass Rate | 25% | 20% | TBD | TBD |
| Latency | 4187ms | 1890ms | TBD | TBD |

### Gemma 3 4B Easy

| Metric | Direct | Gumi 17R2 | Gumi 17R3 | Delta vs 17R2 |
|--------|--------|-----------|-----------|---------------|
| Overall | 0.56 | 0.54 | TBD | TBD |
| Instruction | 0.63 | 0.57 | TBD | TBD |
| Structured Output | 0.72 | 0.72 | TBD | TBD |
| Consistency | 0.53 | 0.53 | TBD | TBD |
| Context | 0.00 | 0.00 | TBD | TBD |
| Pass Rate | 8% | 4% | TBD | TBD |
| Latency | 1998ms | 2575ms | TBD | TBD |

### Gemma 3 4B Medium

| Metric | Direct | Gumi 17R2 | Gumi 17R3 | Delta vs 17R2 |
|--------|--------|-----------|-----------|---------------|
| Overall | 0.56 | 0.60 | TBD | TBD |
| Instruction | 0.55 | 0.45 | TBD | TBD |
| Structured Output | 0.66 | 0.84 | TBD | TBD |
| Consistency | 0.47 | 0.40 | TBD | TBD |
| Context | 0.60 | 0.60 | TBD | TBD |
| Pass Rate | 15% | 20% | TBD | TBD |
| Latency | 2731ms | 2799ms | TBD | TBD |

---

## Acceptance Criteria Assessment

| # | Criterion | Required | Qwen3 Easy | Qwen3 Medium | Gemma3 Easy | Gemma3 Medium | Result |
|---|-----------|----------|------------|--------------|-------------|---------------|--------|
| 1 | No capability regression >2pp | ≤2pp | TBD | TBD | TBD | TBD | Pending |
| 2 | Overall score does not regress | ≥0pp | TBD | TBD | TBD | TBD | Pending |
| 3 | Instruction regression on Gemma eliminated | ≥-2pp | — | — | TBD | TBD | Pending |
| 4 | Qwen Medium instruction regression eliminated | ≥-2pp | — | TBD | — | — | Pending |
| 5 | Context regression eliminated | ≥-2pp | — | TBD | — | — | Pending |
| 6 | Consistency regression eliminated | ≥-2pp | — | — | — | TBD | Pending |
| 7 | Structured output ≥ Sprint 17R2 | ≥0pp | TBD | TBD | TBD | TBD | Pending |
| 8 | Preprocessing overhead <100ms | <100ms | TBD | TBD | TBD | TBD | Pending |
| 9 | Runtime overhead <500ms | <500ms | TBD | TBD | TBD | TBD | Pending |
| 10 | No unexplained provider calls | 1:1 ratio | TBD | TBD | TBD | TBD | Pending |
| 11 | Hint tokens < Sprint 17 | <100 tokens | TBD | TBD | TBD | TBD | Pending |
| 12 | All tests pass | 54/54 | PASS | — | — | — | PASS |

---

## Three-State Comparison

| Model | Tier | Sprint 16D | Sprint 17R2 | Sprint 17R3 |
|-------|------|-----------|-------------|-------------|
| Qwen3 8B | Easy | TBD | 0.57 / +10pp Instr | TBD |
| Qwen3 8B | Medium | TBD | 0.59 / -5pp Instr | TBD |
| Gemma 3 4B | Easy | TBD | 0.54 / -7pp Instr | TBD |
| Gemma 3 4B | Medium | TBD | 0.60 / -10pp Instr, +18pp JSON | TBD |

---

## Notes

- GEP v2.0.0 not modified
- Benchmark suites not modified
- Scorer not modified
- Certified baselines not modified
- Regression thresholds not modified

---

## Recommendation

**PENDING GEP RUN RESULTS**

If all acceptance criteria pass: RECOMMEND MERGE  
If any acceptance criterion fails: REJECT Sprint 17R3. Document failure. Do not proceed to Sprint 18.
