# Instruction Overhead Analysis — Sprint 17R2

**Date:** 2026-08-10  
**Sprint:** 17R2

---

## Methodology

Instruction overhead is measured as the ratio of injected instruction tokens to original prompt tokens:

```
instruction_overhead_ratio = instruction_hint_tokens / original_prompt_tokens
```

This metric is tracked per-request via telemetry (`instruction_hint_tokens` field) and can be queried from the `requests` table.

---

## Sprint 17 Overhead (Baseline)

The Sprint 17 hint block format produced consistently high overhead:

| Prompt Type | Original Tokens | Hint Block Tokens | Overhead Ratio |
|------------|----------------|-------------------|----------------|
| Simple question | ~20 | ~180 | **900%** |
| Single constraint | ~30 | ~120 | **400%** |
| Multi-constraint | ~50 | ~250 | **500%** |
| JSON request | ~40 | ~200 | **500%** |
| Complex reasoning | ~60 | ~220 | **367%** |

**Problem:** Even simple prompts received the full verbose hint block treatment, creating enormous token inflation.

---

## Sprint 17R2 Overhead (Current)

The redesigned minimal hint block produces dramatically lower overhead:

| Prompt Type | Original Tokens | Hint Block Tokens | Overhead Ratio |
|------------|----------------|-------------------|----------------|
| Simple question | ~20 | 0 | **0%** |
| Single constraint | ~30 | ~8 | **27%** |
| Multi-constraint | ~50 | ~25 | **50%** |
| JSON request | ~40 | ~5 | **13%** |
| Complex reasoning | ~60 | 0 | **0%** |

**Improvement:** 60-95% reduction in instruction overhead across all prompt types.

---

## Bounded Policy

The new design uses a **deterministic, bounded policy**:

1. **No hard constraints → 0 tokens injected**
   - Simple questions, factual queries, open-ended prompts produce zero instruction overhead
   - This eliminates the worst-case 900% overhead from Sprint 17

2. **Single hard constraint → ~5-10 tokens**
   - Minimal hint: "exactly 2 sentences", "return valid JSON only", "one word"
   - Maximum ~10 tokens per constraint

3. **Multiple hard constraints → ~5-10 tokens each**
   - Capped at ~300 tokens total for extreme cases
   - No headers, footers, or section separators

4. **Soft hints → 0 tokens injected**
   - `complex_reasoning` and `factual_confidence` detected but never injected
   - Eliminates unpredictable overhead on factual/complex prompts

---

## Overhead by Benchmark Suite

### Instruction Following (Easy)

| Test | Original | Sprint 17 Hint | 17R2 Hint | Savings |
|------|----------|---------------|-----------|---------|
| 2-sentence + forbidden word | ~35 | ~180 | ~15 | 92% |
| End with specific word | ~25 | ~160 | ~8 | 95% |
| Single word answer | ~20 | ~170 | ~5 | 97% |
| Bullet list | ~30 | ~160 | ~10 | 94% |
| Yes/no answer | ~20 | ~150 | 0 | 100% |

### Structured Output (Easy)

| Test | Original | Sprint 17 Hint | 17R2 Hint | Savings |
|------|----------|---------------|-----------|---------|
| Simple JSON object | ~40 | ~200 | ~5 | 98% |
| JSON array | ~35 | ~190 | ~5 | 97% |
| Nested JSON | ~50 | ~210 | ~5 | 98% |
| Tabular JSON | ~60 | ~220 | ~5 | 98% |

### Context Retention Impact

With 60-80% token reduction in instruction overhead:
- More context budget available for conversation history
- Reduced likelihood of context compaction triggers
- Direct improvement to context retention scores

---

## Latency Correlation

The ~4000ms latency regression in Sprint 17 (Qwen3 8B) correlates with:

1. **Prompt token inflation:** 3-5x larger prompts → longer preprocessing
2. **Potential extra retries:** Confusing conflict warnings may have triggered validation failures → retries
3. **Model confusion:** Verbose hints may have caused models to "reason about the wrapper" instead of answering

Sprint 17R2's minimal hints should:
1. Reduce prompt size by 60-80% → faster preprocessing
2. Reduce validation failures → fewer retries
3. Reduce model confusion → cleaner outputs

---

## Telemetry Query

To analyze instruction overhead from stored telemetry:

```sql
SELECT
    request_id,
    prompt_tokens,
    instruction_hint_tokens,
    CASE WHEN prompt_tokens > 0
         THEN instruction_hint_tokens * 100.0 / prompt_tokens
         ELSE 0 END AS overhead_pct,
    instruction_hard_constraint_count,
    instruction_retry_count
FROM requests
WHERE instruction_hint_tokens > 0
ORDER BY created_at DESC
LIMIT 100;
```

---

## Recommendations

1. **Monitor `instruction_hint_tokens`** in production to catch regressions
2. **Set alert threshold** at >100 tokens for simple prompts
3. **Track overhead ratio** by runtime mode (stabilized vs lightweight)
4. **Correlate overhead with latency** to validate the 4000ms improvement hypothesis
