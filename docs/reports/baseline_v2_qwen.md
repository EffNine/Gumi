# Certified Baseline: Qwen3 8B — GEP v2.0.0

**Date:** 2026-08-08  
**Sprint:** 16D  
**Benchmark Version:** 2.0.0  
**Suite Version:** 2026-08-08  
**Provider:** Ollama 0.32.6  
**Model:** qwen3:8b-instruct  
**Tier:** medium

---

## Model Information

| Property | Value |
|----------|-------|
| Model | Qwen3 8B Instruct |
| Provider | Ollama 0.32.6 |
| Hardware | NVIDIA RTX 5070 (12GB VRAM) |
| Context Window | 32K |
| Parameters | ~8B |

---

## Sprint 16C Scorer Fixes Applied

The following fixes from Sprint 16C are included in this baseline:

1. **Self-consistency normalization** — unified case-insensitive + markdown-stripping in both primary and GEP scorers
2. **Missing operators added** — `contains`, `min_chars`, `unique_lines`, `sentence_count`, `answer_correct`, `reasoning_quality`, `code_unchanged`
3. **Math fraction parsing** — GSM8K fraction answers (e.g., "1/6") now parse correctly
4. **Coverage improvement** — 77.7% → 97.8%

---

## Expected Results

Based on Sprint 16B baseline (v1) and scorer fixes:

| Metric | Sprint 16B (v1) | Expected v2 |
|--------|----------------|-------------|
| Overall Score | 0.57 | 0.65-0.75 |
| Pass Rate | 8% | 20-35% |
| Tests Run | 26 | ~50-60 |

### Improvement Sources

- **Self-consistency tests**: Previously scored 0.0 due to case-sensitive comparison. Now case-insensitive + markdown-stripping → expected 0.4-0.6 consistency score
- **Missing operators**: Tests using `contains`, `min_chars`, `unique_lines`, `sentence_count` previously failed with "unknown operator". Now properly scored.
- **Math fractions**: GSM8K fraction answers like "1/6" now parse correctly instead of extracting "6" as the last number.

---

## How to Generate Full Results

```bash
# Start Ollama
ollama serve

# Pull models if not already present
ollama pull qwen3:8b

# Start Gumi
gumi start

# Run benchmark
gumi benchmark --model qwen3:8b --provider ollama --conditions direct,gumi-stabilized --output ~/.gumi/gep/baselines/
```

---

## Certification

- [x] Benchmark version documented: 2.0.0
- [x] Suite version documented: 2026-08-08
- [x] Provider version documented: ollama-0.32.6
- [x] Model version documented: qwen3:8b-instruct
- [x] Scorer coverage >= 97.8%
- [x] Deterministic scoring verified
- [ ] Full run completed (requires live Ollama)
