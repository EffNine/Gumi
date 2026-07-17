# llama3.2:3b — Before/After (Ollama, CPU)

Date: 2026-07-17  
Model: `llama3.2:3b` via Ollama  
Changes: profile tune + digit/one-word instruction assist

## Headline

| Metric | Before | After |
|--------|--------|-------|
| Total passed | 20 / 39 | **25 / 39** |
| Exact instruction following | 14 / 24 | **18 / 24** |
| D-GumiStabilized | 8 / 9 | **9 / 9** |
| E-GumiStructured JSON | 3 / 3 | **3 / 3** |
| C concise exact | 0 / 3 | **3 / 3** |
| Valid JSON (all JSON prompts) | 6 / 15 | 7 / 15 |

## Mode detail

| Mode | Before | After |
|------|--------|-------|
| A Direct | concise 0/3, factual 3/3, JSON 0/3 | same (provider-only control) |
| B Gumi Direct | concise 0/3, factual 3/3, JSON 0/3 | same (no repair/retry path) |
| C Lightweight | concise 0/3, factual 3/3, JSON 0/3 | concise **3/3**, factual 3/3, JSON 1/3 |
| D Stabilized | 8/9 (1 spelled-number miss) | **9/9** |
| E Structured | 3/3 | **3/3** |

Stabilized and structured remain the recommended modes for reliability.
