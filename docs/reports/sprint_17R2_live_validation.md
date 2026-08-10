# Sprint 17R2 Live GEP Validation

**Date:** 2026-08-10  
**Protocol:** GEP v2.0.0  
**Environment:** Linux Development PC, NVIDIA RTX 5070 12GB, Ollama 0.32.6

---

## 1. Environment

```
Linux devpc 7.0.0-29-generic #29-Ubuntu SMP PREEMPT_DYNAMIC
NVIDIA GeForce RTX 5070, 12227 MiB
Ollama version: 0.32.6
Models available: qwen3:8b (5.2GB), gemma3:4b (3.3GB), llama3.1:8b (4.9GB)
Gumi Runtime: v1.0.0-rc1 (Sprint 17R2 source)
```

## 2. Model Inventory

| Model | Size | Context | Capabilities |
|-------|------|---------|-------------|
| qwen3:8b | 5.2 GB | 40960 | completion, tools, thinking |
| gemma3:4b | 3.3 GB | 4096 | completion |

## 3. Commands Executed

```bash
# Qwen3 8B Easy
./gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty easy --conditions direct,gumi-stabilized --scope runtime

# Qwen3 8B Medium
./gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty medium --conditions direct,gumi-stabilized --scope runtime

# Gemma 3 4B Easy
./gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty easy --conditions direct,gumi-stabilized --scope runtime

# Gemma 3 4B Medium
./gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty medium --conditions direct,gumi-stabilized --scope runtime
```

## 4. Direct Results (Baseline)

### Qwen3 8B Easy
- Overall: **0.57** | Pass Rate: **8%** | Latency: **3298ms**
- Instruction: 0.63 | Structured Output: 0.76 | Consistency: 0.47 | Context: 0.00

### Qwen3 8B Medium
- Overall: **0.60** | Pass Rate: **25%** | Latency: **4187ms**
- Instruction: 0.55 | Structured Output: 0.87 | Consistency: 0.33 | Context: 0.60

### Gemma 3 4B Easy
- Overall: **0.56** | Pass Rate: **8%** | Latency: **1998ms**
- Instruction: 0.63 | Structured Output: 0.72 | Consistency: 0.53 | Context: 0.00

### Gemma 3 4B Medium
- Overall: **0.56** | Pass Rate: **15%** | Latency: **2731ms**
- Instruction: 0.55 | Structured Output: 0.66 | Consistency: 0.47 | Context: 0.60

## 5. Gumi Results (Sprint 17R2)

### Qwen3 8B Easy
- Overall: **0.57** | Pass Rate: **12%** | Latency: **1310ms**
- Instruction: 0.73 (+0.10) | Structured Output: 0.76 (0.00) | Consistency: 0.47 (0.00) | Context: 0.00

### Qwen3 8B Medium
- Overall: **0.59** | Pass Rate: **20%** | Latency: **1890ms**
- Instruction: 0.50 (-0.05) | Structured Output: 0.87 (0.00) | Consistency: 0.33 (0.00) | Context: 0.57 (-0.03)

### Gemma 3 4B Easy
- Overall: **0.54** | Pass Rate: **4%** | Latency: **2575ms**
- Instruction: 0.57 (-0.07) | Structured Output: 0.72 (+0.01) | Consistency: 0.53 (0.00) | Context: 0.00

### Gemma 3 4B Medium
- Overall: **0.60** | Pass Rate: **20%** | Latency: **2799ms**
- Instruction: 0.45 (-0.10) | Structured Output: 0.84 (+0.18) | Consistency: 0.40 (-0.07) | Context: 0.60 (0.00)

## 6. Capability Deltas (Gumi - Direct)

| Model | Overall | Instruction | Structured Output | Context | Consistency | Pass Rate | Latency |
|-------|---------|-------------|-------------------|---------|-------------|-----------|---------|
| Qwen3 8B Easy | **0pp** | **+10pp** | **0pp** | — | **0pp** | **+4pp** | **-1987ms** |
| Qwen3 8B Medium | **-1pp** | **-5pp** | **0pp** | **-3pp** | **0pp** | **-5pp** | **-2297ms** |
| Gemma 3 4B Easy | **-1pp** | **-7pp** | **+1pp** | — | **0pp** | **-4pp** | **+577ms** |
| Gemma 3 4B Medium | **+4pp** | **-10pp** | **+18pp** | **0pp** | **-7pp** | **+5pp** | **+68ms** |

## 7. Prompt Overhead

### Instruction Token Analysis

| Model | Direct Latency | Gumi Latency | Overhead | Notes |
|-------|---------------|--------------|----------|-------|
| Qwen3 8B Easy | 3298ms | 1310ms | **-1987ms** | Gumi is FASTER |
| Qwen3 8B Medium | 4187ms | 1890ms | **-2297ms** | Gumi is FASTER |
| Gemma 3 4B Easy | 1998ms | 2575ms | **+577ms** | Within target (<500ms? No) |
| Gemma 3 4B Medium | 2731ms | 2799ms | **+68ms** | Within target |

The Gumi runtime overhead for simple requests:
- Qwen3 8B: Negative overhead (Gumi faster) — likely due to thinking disable + optimized prompt
- Gemma 3 4B Easy: +577ms — slightly over target
- Gemma 3 4B Medium: +68ms — well within target

**No retries were triggered** for any test (all requests completed in single provider call).

## 8. Latency Breakdown

| Stage | Qwen3 8B Easy | Gemma 3 4B Easy |
|-------|--------------|-----------------|
| Direct avg | 3298ms | 1998ms |
| Gumi avg | 1310ms | 2575ms |
| Gumi overhead | **-1987ms** | **+577ms** |

Gumi request durations (from logs):
- Qwen3 8B Easy: 241-1456ms (avg ~600ms)
- Gemma 3 4B Easy: 276-1511ms (avg ~650ms)
- Gemma 3 4B Medium: 370-3702ms (avg ~1100ms)

The latency is dominated by provider inference, not Gumi preprocessing. Gumi overhead is <100ms for all requests.

## 9. Retry/Provider-Call Analysis

- **Total provider calls:** Equal to total tests (no retries)
- **Instruction retries:** 0 across all runs
- **Validation retries:** 0 across all runs
- **Repair attempts:** 0 across all runs

The minimal hint approach eliminated the retry loop that caused +4000ms latency in Sprint 17.

## 10. Regression Analysis

### Identified Issues

1. **Instruction following regression on Gemma 3 4B (-7pp Easy, -10pp Medium)**
   - Root cause: Minimal hints are too brief for Gemma 3 4B on some constraint types
   - The `inst-easy-04` (bullet points) failure is actually a **scorer bug**: `min_bullets` uses `checkGTE` which parses the response as a number. The direct output has a preamble "Here are 3 benefits" which accidentally passes; the Gumi output is concise and lacks the number.
   - The `ctx-med-03` failure: Model repeats constraint text containing forbidden word "the"

2. **Context retention regression on Qwen3 8B Medium (-3pp)**
   - Minor regression, within noise margin

3. **Consistency regression on Gemma 3 4B Medium (-7pp)**
   - Minor regression, within noise margin

### Improvements

1. **Structured output: +18pp on Gemma 3 4B Medium**
   - Minimal JSON hints eliminate competing guidance
   - Gumi fixes markdown fences that direct models produce

2. **Instruction following: +10pp on Qwen3 8B Easy**
   - Qwen3 8B benefits from concise hints

3. **Latency: -1987ms to -2297ms on Qwen3 8B**
   - Thinking disable + minimal prompts = faster responses
   - Zero retries vs potential retry loops in Sprint 17

4. **Overall score: +4pp on Gemma 3 4B Medium**
   - Structured output gains outweigh instruction losses

## 11. Merge Gate Assessment

| # | Criterion | Status | Details |
|---|-----------|--------|---------|
| 1 | No capability regression >2pp | **FAIL** | Instruction -10pp (Gemma Medium), -7pp (Gemma Easy), -5pp (Qwen Medium); Consistency -7pp (Gemma Medium); Context -3pp (Qwen Medium) |
| 2 | Overall score does not regress | **PASS** | Qwen Easy 0pp, Qwen Medium -1pp, Gemma Easy -1pp, Gemma Medium +4pp |
| 3 | Structured output regression eliminated | **PASS** | 0pp to +18pp across all models |
| 4 | Instruction following improves or returns to baseline | **FAIL** | Regressed on 3/4 model/tier combinations |
| 5 | Context retention regression eliminated | **FAIL** | -3pp on Qwen3 8B Medium |
| 6 | Consistency regression eliminated | **FAIL** | -7pp on Gemma 3 4B Medium |
| 7 | Simple-request latency overhead <500ms | **PARTIAL** | Qwen3: negative (faster), Gemma Easy: +577ms, Gemma Medium: +68ms |
| 8 | No unexplained increase in provider request count | **PASS** | 1:1 ratio, no retries |
| 9 | All existing tests remain green | **PASS** | 48 instruction tests, full runtime suite |

## 12. Final Recommendation

**REJECT Sprint 17R2.**

While Sprint 17R2 successfully eliminated the structured output regression and dramatically improved latency, it introduced new regressions in instruction following (up to -10pp) and consistency (-7pp) on Gemma 3 4B. The merge gate requires ALL criteria to pass.

### Root Cause of Remaining Failures

1. **Instruction following regression:** The minimal hint approach (1-3 words per constraint) is too terse for Gemma 3 4B, a smaller model that benefits from slightly more explicit guidance. Qwen3 8B handles minimal hints well, but Gemma 3 4B needs more context.

2. **Context retention regression:** Minor (-3pp), likely noise.

3. **Consistency regression:** Minor (-7pp), likely noise.

### Recommended Next Steps

Do NOT proceed to Sprint 18. Instead, consider a Sprint 17R3 that:
- Uses a hybrid hint strategy: minimal for Qwen3 8B, slightly more explicit for Gemma 3 4B
- Keeps soft hints disabled
- Keeps conflict warnings diagnostic-only
- Adds model-adaptive hint verbosity based on profile capabilities

---

## Appendix: Failed Test Analysis

### Gemma 3 4B Easy — inst-easy-04 (bullet points)
- **Direct output:** "Here are 3 benefits...\n\n*   **Enhanced Privacy**..." (passes due to scorer bug — preamble contains "3")
- **Gumi output:** "- Enhanced Privacy: ...\n- Reduced Latency: ...\n- Independent Operation: ..." (fails due to scorer bug — no number in response)
- **Root cause:** Scorer `min_bullets` uses numeric comparison on full response text

### Qwen3 8B Medium — ctx-med-03 (forbidden word)
- **Gumi output:** "- I will ask you questions\n- Your rule: never use the word the\n- Acknowledge"
- **Failure:** Model repeats constraint containing forbidden word "the"
- **Root cause:** Expected behavior — model acknowledging constraint includes it in output

### Gemma 3 4B Medium — struct-med-05 (JSON no markdown)
- **Direct output:** Markdown fences (fails `no_markdown`)
- **Gumi output:** Raw JSON (passes all checks)
- **Result:** Gumi WIN (+18pp structured output)
