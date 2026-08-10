# GEP Regression Analysis — Sprint 17R2

**Date:** 2026-08-10  
**Sprint:** 17R2  
**Protocol:** GEP v2.0.0

---

## Certified Baseline (Sprint 17R)

GEP v2.0.0 live validation on Development PC (Linux, NVIDIA RTX 5070, Ollama):

### Qwen3 8B

| Metric | Delta | Status |
|--------|-------|--------|
| Overall | **-6pp** | FAIL |
| Instruction | +4pp | PASS |
| Structured Output / JSON | **-13pp** | FAIL |
| Pass Rate | **-4pp** | FAIL |
| Latency | **+4000ms** | FAIL |

### Gemma 3 4B Easy

| Metric | Delta | Status |
|--------|-------|--------|
| Overall | **-3pp** | FAIL |
| Instruction | **-16pp** | FAIL |
| JSON | 0pp | PASS |
| Pass Rate | **-4pp** | FAIL |

### Gemma 3 4B Medium

| Metric | Delta | Status |
|--------|-------|--------|
| Overall | +1pp | PASS |
| Instruction | -5pp | FAIL |
| JSON | +10pp | PASS |
| Context | **-20pp** | FAIL |
| Consistency | **-7pp** | FAIL |
| Pass Rate | 0pp | PASS |

**Merge gate: FAILED**

---

## Root Cause Analysis

### JSON Regression (-13pp on Qwen3 8B)

**Cause:** Sprint 17's hint block appended generic formatting guidance alongside JSON constraints.

The old hint block for a JSON request looked like:
```
CRITICAL: Follow ALL of these rules exactly:
1. Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.
2. Your response must contain exactly 3 sentence(s). No more, no less.
...

Additional guidance:
- Break this question down step-by-step...

Before responding, verify each rule above is satisfied.
```

The "Additional guidance" soft hint and "verify each rule" footer created **competing instructions**. Models prioritized the verbose guidance over the JSON format constraint, producing prose responses instead of JSON.

**Fix in 17R2:** Soft hints are no longer injected. The hint block for JSON is now just:
```
return valid JSON only
```

### Context Retention Regression (-20pp on Gemma 3 4B Medium)

**Cause:** Verbose hint blocks consumed context budget, leaving less room for conversation history.

A typical Sprint 17 hint block added 150-300 tokens to the system prompt. For models with limited context windows (Gemma 3 4B has ~8K context), this represented 2-4% of total context per request — enough to trigger early context compaction.

**Fix in 17R2:** 60-80% token reduction in hint blocks frees context budget.

### Consistency Regression (-7pp on Gemma 3 4B Medium)

**Cause:** Soft hints were injected inconsistently. Factual questions got confidence hints; complex questions got step-by-step hints. This created variable behavior across similar prompts.

**Fix in 17R2:** Soft hints are completely disabled by default. All prompts receive the same minimal treatment based only on hard constraints.

### Instruction Regression on Gemma 3 4B (-16pp Easy)

**Cause:** The verbose hint block may have confused Gemma 3 4B, a smaller model, more than Qwen3 8B. The "think step-by-step" soft hint directly conflicted with format-restrictive constraints.

**Fix in 17R2:** No soft hints injected. Format-restrictive constraints get only their minimal hint.

### Latency Regression (+4000ms on Qwen3 8B)

**Cause:** Token inflation from verbose hints increased prefill time. Additionally, confusing hints may have triggered validation failures and retries.

**Fix in 17R2:** 85% token reduction should significantly decrease preprocessing latency.

---

## Sprint 17R2 Fix Validation

### Changes Made

| Change | Expected Impact |
|--------|----------------|
| Minimal hint blocks (85% token reduction) | ↓ Latency, ↑ Context retention |
| Soft hints disabled by default | ↑ Consistency, ↑ JSON (no competing guidance) |
| Conflict warnings diagnostic-only | ↑ Instruction (no model confusion) |
| Simplified retry hints | ↓ Retry failures, ↓ Latency |
| Telemetry instrumentation | Measurable overhead tracking |

### Expected GEP Results

| Model | Expected Overall | Expected JSON | Expected Context | Expected Consistency | Expected Latency |
|-------|-----------------|---------------|------------------|---------------------|-----------------|
| Qwen3 8B | ~0pp to +2pp | ~0pp to +5pp | ~+5pp | ~+3pp | <-500ms overhead |
| Gemma 3 4B Easy | ~0pp | 0pp | ~+5pp | ~+3pp | <-500ms overhead |
| Gemma 3 4B Medium | ~+1pp to +3pp | ~+5pp | ~+10pp | ~+5pp | <-500ms overhead |

---

## Acceptance Criteria

Sprint passes only if:

1. **No capability regression >2pp** — All deltas must be within ±2pp of baseline
2. **Overall score does not regress** — Must be ≥ Sprint 17 baseline
3. **Structured output regression eliminated** — JSON delta must be ≥ -2pp
4. **Instruction following improves or returns to baseline** — Delta ≥ -2pp
5. **Context retention regression eliminated** — Delta ≥ -2pp
6. **Consistency regression eliminated** — Delta ≥ -2pp
7. **Simple-request latency overhead <500ms** — Where no retry occurs
8. **All existing tests pass** — 48 instruction tests, full runtime suite

---

## Next Steps

1. Run GEP v2.0.0 on Linux Development PC (RTX 5070)
2. Compare results against certified baselines
3. If all criteria pass: **RECOMMEND MERGE**
4. If any criterion fails: **Document remaining failure and stop** (do not proceed to Sprint 18)

---

## Validation Commands

```bash
# Build
make build

# Run tests
cd runtime && go test ./... && go vet ./... && go fmt ./...

# Run GEP benchmark (on Linux Dev PC)
go run ./benchmark/gep/run_benchmark.go \
  --model qwen3:8b \
  --provider ollama \
  --provider-url http://localhost:11434 \
  --gumi-url http://localhost:8787 \
  --difficulty easy \
  --attempts 3
```
