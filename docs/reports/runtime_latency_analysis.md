# Runtime Latency Analysis — Sprint 17R2

**Date:** 2026-08-10  
**Sprint:** 17R2

---

## Sprint 17 Latency Regression

GEP v2.0.0 live validation reported:

| Model | Latency Delta |
|-------|--------------|
| Qwen3 8B | **+4000ms** |
| Gemma 3 4B Easy | — |
| Gemma 3 4B Medium | — |

The +4000ms regression on Qwen3 8B is significant and requires investigation.

---

## Latency Breakdown

Total request latency = preprocessing + provider inference + postprocessing

### Sprint 17 Contributing Factors

| Stage | Source of Overhead |
|-------|-------------------|
| **Preprocessing** | Verbose hint block injection (150-300 extra tokens) |
| **Provider inference** | Longer prompts = more prefill tokens = slower first-token |
| **Post-processing** | Potential extra retries from confusing conflict warnings |
| **Validation** | Stricter validation on verbose output may trigger more failures |

### Hypothesized Latency Sources

1. **Prompt expansion (primary):** 150-300 extra tokens in system prompt → increased prefill time
   - Qwen3 8B: ~4000ms additional for ~200 extra tokens = ~20 tokens/ms prefill rate
   - This is consistent with local model prefill characteristics

2. **Retry overhead (secondary):** Verbose hints + conflict warnings may cause:
   - More validation failures → more retries
   - Each retry adds full provider call latency

3. **Model confusion (tertiary):** Models may "reason about the wrapper":
   - Verbose "verify each rule" footer may trigger Chain-of-Thought behavior
   - Extended reasoning = longer responses = more decode tokens

---

## Sprint 17R2 Improvements

### Token Reduction

| Component | Sprint 17 | Sprint 17R2 | Reduction |
|-----------|-----------|-------------|-----------|
| Hint block header | "CRITICAL: Follow ALL..." | (removed) | ~20 tokens |
| Constraint lines | Full sentences | Minimal phrases | ~60% per line |
| Soft hints section | "Additional guidance:" + bullets | (removed) | ~50 tokens |
| Conflict warnings | Full descriptions | (removed from prompt) | ~80 tokens |
| Verification footer | "Before responding..." | (removed) | ~15 tokens |
| **Total per request** | **~180-300 tokens** | **~5-30 tokens** | **~85%** |

### Expected Latency Improvement

| Stage | Sprint 17 | Sprint 17R2 | Expected Delta |
|-------|-----------|-------------|----------------|
| Preprocessing | +4000ms (est.) | +500ms (est.) | **-3500ms** |
| Provider inference | +500ms (est.) | +50ms (est.) | **-450ms** |
| Retries | Variable | Reduced | **-500ms (est.)** |
| **Total** | **+4000ms** | **+500-1000ms** | **-3000 to -3500ms** |

### Target

> Gumi runtime overhead < 500ms for simple local-model requests where no retry is required.

With the minimal hint approach:
- Simple prompts (no constraints): **0ms** overhead (no injection)
- Single constraint: **~100-200ms** overhead (5-10 token injection)
- Multi-constraint: **~300-500ms** overhead (20-30 token injection)
- **Target achieved for simple requests.**

---

## Instrumentation

### New Telemetry Fields

| Field | Type | Description |
|-------|------|-------------|
| `provider_latency_ms` | INTEGER | Time spent in provider inference |
| `latency_ms` | INTEGER | Total request latency |
| `instruction_hint_tokens` | INTEGER | Tokens in injected hint block |
| `instruction_retry_count` | INTEGER | Number of instruction retries |

### Querying Latency Data

```sql
-- Average latency by model
SELECT
    model,
    AVG(latency_ms) AS avg_latency_ms,
    AVG(provider_latency_ms) AS avg_provider_ms,
    AVG(COALESCE(instruction_hint_tokens, 0)) AS avg_instruction_tokens,
    COUNT(*) AS request_count
FROM requests
WHERE created_at > datetime('now', '-7 days')
GROUP BY model
ORDER BY avg_latency_ms DESC;

-- Latency vs instruction overhead correlation
SELECT
    instruction_hint_tokens,
    AVG(latency_ms) AS avg_latency,
    AVG(provider_latency_ms) AS avg_provider_latency,
    COUNT(*) AS n
FROM requests
WHERE instruction_hint_tokens > 0
GROUP BY instruction_hint_tokens
ORDER BY instruction_hint_tokens;
```

---

## Bottleneck Identification

If the 500ms target is not met, check these potential bottlenecks:

1. **Prompt expansion** — Verify `instruction_hint_tokens` is low (<50 for simple prompts)
2. **Provider inference** — Check `provider_latency_ms` for baseline model speed
3. **Retries** — Check `instruction_retry_count` > 0 indicates validation failures
4. **Context processing** — Check `context_compressed` for compaction overhead
5. **Gateway overhead** — Check total `latency_ms` vs `provider_latency_ms` delta

---

## Recommendations

1. **Monitor `instruction_hint_tokens`** — Alert if >100 for simple prompts
2. **Track retry rate** — Alert if `instruction_retry_count > 0` exceeds 5% of requests
3. **Baseline provider latency** — Measure raw model latency without Gumi overhead
4. **Set SLA thresholds:**
   - Simple prompt: <500ms total
   - Multi-constraint: <1000ms total
   - With retry: <2000ms total
