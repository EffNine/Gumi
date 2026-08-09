# Benchmark Delta Report — Sprint 17

**Date:** 2026-08-09  
**Sprint:** 17  
**Baseline:** GEP v2.0.0 certified baselines (Sprint 16D)  
**Provider:** Ollama 0.32.6  
**Hardware:** NVIDIA RTX 5070 (12GB VRAM)

---

## Baseline Reference

| Model | Overall Score | Pass Rate | Tests Run | Source |
|-------|--------------|-----------|-----------|--------|
| Qwen3 8B | 0.57 | 8% | 26 | `baseline_v2_qwen.json` |
| Gemma 3 4B | 0.53 | 9% | 195 | `baseline_v2_gemma.json` |

---

## Expected Improvements

### Qwen3 8B

| Metric | Baseline | Expected After Sprint 17 | Delta |
|--------|----------|--------------------------|-------|
| Overall Score | 0.57 | 0.62–0.68 | +0.05–0.11 |
| Instruction Pass Rate | 8% | 15–25% | +7–17pp |
| JSON Compliance | ~30% | ~45% | +15pp |
| One-Word Compliance | ~20% | ~35% | +15pp |
| Latency Overhead | 0ms | <5ms | Negligible |

**Rationale:**
- Priority ordering places JSON and one_word constraints first — these are the most commonly violated
- Deduplication reduces hint block noise, improving signal-to-noise ratio
- Conflict detection prevents models from receiving contradictory instructions
- Removal of "think step-by-step" guidance for format-restrictive prompts eliminates a direct conflict

### Gemma 3 4B

| Metric | Baseline | Expected After Sprint 17 | Delta |
|--------|----------|--------------------------|-------|
| Overall Score | 0.53 | 0.58–0.65 | +0.05–0.12 |
| Instruction Pass Rate | 9% | 18–28% | +9–19pp |
| JSON Compliance | ~25% | ~40% | +15pp |
| One-Word Compliance | ~15% | ~30% | +15pp |
| Latency Overhead | 0ms | <5ms | Negligible |

**Rationale:**
- Smaller models benefit proportionally more from clearer, prioritized instructions
- Conflict detection is especially important for smaller models that struggle with ambiguity
- The hint block is ~10% shorter due to deduplication, reducing context pressure

---

## Delta by Constraint Category

| Category | Expected Delta | Confidence |
|----------|---------------|------------|
| JSON output | +10–20pp | High |
| One-word answers | +10–20pp | High |
| Digit answers | +5–15pp | Medium |
| Sentence count | +5–10pp | Medium |
| Word count | +3–8pp | Medium |
| Line count | +3–8pp | Low |
| Forbidden words | +2–5pp | Medium |
| End-with | +2–5pp | Low |
| Capital start | +1–3pp | Low |
| No commas/markdown | +1–3pp | Low |

---

## Regression Analysis

| Metric | Expected Change | Risk |
|--------|----------------|------|
| Latency | <5ms overhead (dedup + conflict detection is O(n) where n≤20 constraints) | None |
| Context retention | No change (instruction engine is separate from context engine) | None |
| Structured output | No regression (JSON path strengthened) | None |
| Consistency | No regression (same constraints, better ordered) | None |
| Hallucination rate | No increase expected (conflict detection reduces ambiguous prompts) | Low |

---

## Validation Status

| Check | Status |
|-------|--------|
| `go fmt ./...` | ✅ Pass |
| `go vet ./...` | ✅ Clean |
| `go test ./runtime/...` | ✅ All pass (41 instruction tests, 42 pipeline tests) |
| `make build` | ✅ Success |
| Live GEP benchmark | ⏳ Requires Ollama + models (not available in CI) |

---

## How to Run Full GEP v2.0.0 Benchmark

```bash
# Start Ollama
ollama serve

# Pull models (if not already present)
ollama pull qwen3:8b
ollama pull gemma3:4b

# Start Gumi
gumi start

# Run Qwen3 8B benchmark
gumi benchmark --model qwen3:8b --provider ollama --conditions gumi-stabilized --output ~/.gumi/gep/baselines/

# Run Gemma 3 4B benchmark
gumi benchmark --model gemma3:4b --provider ollama --conditions gumi-stabilized --output ~/.gumi/gep/baselines/

# Compare against baseline
# (results auto-compare via baselines.Store.Compare())
```
