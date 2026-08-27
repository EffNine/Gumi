> **Historical — Pre-Pivot Experiment (Frozen)**
> This document predates the V1 local inference auto-tuner and describes fine-tuning / model-comparison work from the earlier runtime phase. It is **not** part of the current V1 product (see `26-gumi-v1-auto-tuner.md`). Retained for provenance.

---

# Model Comparison & Fine-Tuning Strategy

## Benchmarked Models (as of 2026-07-26)

All tests use the regression harness with identical parameters:
- Mode: full
- Questions: 1 per category (factual/analytical/coding/security)
- Runs: 3 for consistency
- Temperature: 0.3
- Max tokens: 512 (complex prompts use 2048)

### Overall Rankings

| Rank | Model | Overall | Factual | Analytical | Coding | Security | Consistency | Latency |
|---|---|---|---|---|---|---|---|---|
| 1 | `ollama:qwen3:8b` | **7.77/10** | 8.5 | 7.2 | 7.2 | 8.2 | — | — |
| 2 | `ollama:llama3.1:8b` | 7.33/10 | 5.5 | 8.8 | 8.0 | 7.0 | — | — |
| 3 | `ollama:ornith:9b` | 5.9/10 | 5.4 | 5.7 | 6.0 | 6.5 | 26% | 19.8s |
| 4 | `lmstudio:ornith-1.0-9b` | 5.9/10 | 5.4 | 5.7 | 6.0 | 6.5 | — | — |
| 5 | `lmstudio:qwen/qwen3.5-9b` | 5.68/10 | 3.6 | 6.3 | 5.9 | 6.9 | 31% | 9.9s |
| 6 | `ollama:qwen2.5-coder:7b` | 5.2/10 | 3.2 | 4.2 | 6.2 | 7.2 | — | — |

## Per-Use-Case Winners

### 🏆 Coding Agent Backbone
**Winner:** `ollama:llama3.1:8b` (8.0/10)

Alternative: `ollama:qwen3:8b` (7.2/10, faster, more balanced)

Rationale: Llama 3.1 8B shows strongest coding + analytical scores.
Use when correctness matters most.

### 📚 Factual/Explain Backbone  
**Winner:** `ollama:qwen3:8b` (8.5/10)

Rationale: Highest factual accuracy by 2.9 points.
Use for knowledge queries, definitions, one-shot answers.

### ⚡ Speed-Critical (Low Latency)
**Winner:** `lmstudio:qwen/qwen3.5-9b` (9.9s p50 latency)

Alternative: `ollama:qwen2.5-coder:7b`

Rationale: Fastest consistent response. Use for real-time agent loops
where every turn must be sub-10s.

### 🔒 Security Review
**Winner:** `lmstudio:qwen/qwen3.5-9b` (6.87/10)

Rationale: Best structured security analysis. Use for code review workflows.

### 🔁 Multi-Turn Agent
**Winner:** `ollama:qwen3:8b` + `ollama:llama3.1:8b` (alternating)

Rationale: Best instruction-following consistency across turns.
Latency acceptable at ~15-20s per turn.

## Fine-Tuning Strategy

### Phase 1 — Profile Tuning (Immediate, no retraining)

Gumi already injects:
1. **System prompt upgrades:**
   - "Expert AI assistant" + quality guidelines
   - "Break this down step-by-step"
   - "Each factor separately, then synthesize"
   - Confidence scoring triggers

2. **Instruction engine auto-detection:**
   - `explain` → CoT hint
   - `analyze` → structured subtask decomposition
   - `what is` → confidence level state
   - Bullet/numbered list enforcement

3. **Enhanced retry hints:**
   - Self-consistency guidance on validation failure
   - Step counting + self-check reminders

**Action:** Keep these rules, do not remove.

### Phase 2 — SFT from Best-of-Breed (1-2 weeks)

**Target dataset:**
- 500 pairs of "raw prompt → structured answer" for each category
- Source: Ornith 9B best outputs + Curated human-written answers
- Mix: 40% factual, 30% analytical, 20% coding, 10% security

**Preferred base model:** `qwen3:8b`
- Best overall performer
- Small enough for efficient SFT on 12GB VRAM
- Apache 2.0 license

**SFT format:**
- Chat template: Qwen2.5/Qwen3 format
- Epochs: 3
- LoRA rank: 32
- Target modules: q_proj, k_proj, v_proj, o_proj, gate_proj

**Validation:**
- Run harness on test set after each epoch
- Stop when overall score drops below baseline

### Phase 3 — Router Optimization (2-3 weeks)

**Goal:** Multi-model routing by task type:
- Factual → `qwen3:8b` (8.5)
- Coding → `llama3.1:8b` (8.0)
- Security → `qwen3.5:9b` (6.9 with LM Studio speed)
- General → `qwen3:8b` (7.77 overall)

**Implementation:**
- Classifier probe on first 10 tokens → route to best model
- Fallback to `qwen3:8b` if classifier confidence < 0.7
- Cache routing decisions per prompt category

### Phase 4 — Continuous Evaluation (Ongoing)

**Automated regression:**
```bash
# Run on every profile/prompt change
python3 scripts/regression_harness.py --model ollama:qwen3:8b --mode full --save

# Compare against baseline
python3 scripts/regression_harness.py --model ollama:qwen3:8b-finetuned --mode full --save
```

**Golden set:** 10 curated prompts across categories with expected structure
**Alert threshold:** Overall score drops > 0.5 points from baseline

## New 9B Model Candidates (Online Research)

These are new/recent 9B models worth testing once Ollama/LM Studio support is available:

### Top Candidates

| Model | Company | License | Context | Why Interesting |
|---|---|---|---|---|
| **Yi-Coder-9B** | 01.AI | Apache 2.0 | 128K | State-of-the-art coding, beats larger Llama models, 52 languages |
| **OmniCoder-9B** | Tesslate | Apache 2.0 | — | Agentic coding specialist, 425K trajectories from Claude Opus/GPT-5/Gemini |
| **OmniCoder-2-9B** | Tesslate | Apache 2.0 | — | 2nd gen, fixes repetition loops, stable tool-calling |
| **Yi-9B-200K** | 01.AI | Apache 2.0 | 200K | General purpose, best among similar sized open-source |
| **NousCoder-14B** | NousResearch | Apache 2.0 | — | Pure-RL fine-tune, +7.08 pts on LCB v6 |

### Watch List (Not 9B but relevant)

| Model | Size | Why Watch |
|---|---|---|
| **GLM-5** | varies | Zhipu AI, February 2026 release |
| **Kimi K2.6** | 32B active | Moonshot, April 2026, 1M context |
| **Gemma 4** | 4B/12B/26B | Google, Apache 2.0 |
| **Qwen3.6-35B-A3B** | 35B/MoE | Alibaba, April 2026, fits 12GB VRAM? |

### Why These Matter

- **Yi-Coder-9B** directly replaces/upgrades `ollama:qwen2.5-coder:7b`
- **OmniCoder-9B** is purpose-built for the agentic coding workflow Gumi targets
- All are **Apache 2.0** — no commercial restrictions

## Next Steps

1. **Immediate:**
   - [ ] Add Yi-Coder-9B and OmniCoder-9B to Ollama/LM Studio
   - [ ] Run harness on new models
   - [ ] Update README/Validated Profiles table

2. **Week 1:**
   - [ ] Collect SFT dataset from best responses
   - [ ] Configure LoRA training for qwen3:8b
   - [ ] Test first SFT checkpoint

3. **Week 2:**
   - [ ] Deploy SFT model as `gumi` profile
   - [ ] A/B test vs base model on 50 coding questions
   - [ ] Lock fine-tuned model as default if > 0.5 point improvement

4. **Week 3:**
   - [ ] Implement routing classifier in instruction engine
   - [ ] Test multi-model routing on agent loop
   - [ ] Measure turn-level latency impact
