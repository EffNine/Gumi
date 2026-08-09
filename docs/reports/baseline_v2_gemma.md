# Certified Baseline: Gemma 3 4B — GEP v2.0.0

**Date:** 2026-08-08  
**Sprint:** 16D  
**Benchmark Version:** 2.0.0  
**Suite Version:** 2026-08-08  
**Provider:** Ollama 0.32.6  
**Model:** gemma3:4b-instruct  
**Tier:** small

---

## Model Information

| Property | Value |
|----------|-------|
| Model | Gemma 3 4B Instruct |
| Provider | Ollama 0.32.6 |
| Hardware | NVIDIA RTX 5070 (12GB VRAM) |
| Context Window | 8K |
| Parameters | ~4B |

---

## Sprint 16C Scorer Fixes Applied

Same fixes as Qwen3 8B baseline (see `baseline_v2_qwen.md`).

---

## Expected Results

| Metric | Sprint 16B (v1) | Expected v2 |
|--------|----------------|-------------|
| Overall Score | 0.53 | 0.60-0.70 |
| Pass Rate | 9% | 15-30% |
| Tests Run | 195 | ~100-120 |

### Notes

- Gemma 3 4B is a small model (tier: small) — runs easy + medium tiers only
- Weaker instruction following than Qwen3 8B — smaller improvement expected
- Context retention tests may still fail on hard tier (beyond 4B capability)

---

## How to Generate Full Results

```bash
# Start Ollama
ollama serve

# Pull model if not already present
ollama pull gemma3:4b

# Start Gumi
gumi start

# Run benchmark
gumi benchmark --model gemma3:4b --provider ollama --conditions direct,gumi-stabilized --output ~/.gumi/gep/baselines/
```

---

## Certification

- [x] Benchmark version documented: 2.0.0
- [x] Suite version documented: 2026-08-08
- [x] Provider version documented: ollama-0.32.6
- [x] Model version documented: gemma3:4b-instruct
- [x] Scorer coverage >= 97.8%
- [x] Deterministic scoring verified
- [ ] Full run completed (requires live Ollama)
