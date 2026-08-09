# Scorer Audit Report — Sprint 16C

**Date:** 2026-08-08  
**Sprint:** 16C  
**Scope:** `benchmark/scorer/` and `benchmark/gep/scorer/`

---

## Executive Summary

The benchmark subsystem has **two scorer implementations**: the modern `benchmark/scorer` package (used by the current runner) and the legacy `benchmark/gep/scorer` package. This audit covers both.

**Overall health:**
- Primary scorer (`benchmark/scorer`): **77.7% coverage**, core logic sound with identified bugs
- GEP scorer (`benchmark/gep/scorer`): **40.5% coverage**, significant gaps
- Key bugs found: self-consistency normalization differs between versions, several operators have edge case issues, pass/fail thresholds need documentation

---

## 1. Instruction Scorer (checkEQ, checkGTE, checkLTE, checkStartsWith, checkEndsWith)

### Files
- `checks.go:44-371`
- `checks_test.go:64-211`

### Correctness
- **eq** (string): Uses strict equality after `TrimSpace`. Case-sensitive. **Correct.**
- **eq** (numeric): Parses first number found via `extractNumber`. **Correct.**
- **eq** (bool true): Delegates to semantic checks (`capital_start`, `no_markdown`, `no_commas`). **Correct.**
- **eq** (bool false): Inverts the semantic check. **Correct.**
- **gte/lte**: Uses `toFloat64` on constraint value, `extractNumber` on response. **Correct.**
- **starts_with/ends_with**: Trims response, checks prefix/suffix. **Correct.**

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| `extractNumber` fails on negative numbers in some contexts (e.g., "temperature: -5°C") | Low | `checks.go:346` |
| `extractNumber` stops at first number — may pick wrong number in multi-number responses | Medium | `checks.go:341-352` |
| `eq` with string value is case-sensitive (intended, but may surprise) | Low | `checks.go:84-87` |
| `eq` with `int` type is not handled in GEP scorer (`checkEQ` only handles `float64` and `string`) | Medium | `gep/scorer/scorer.go:121-157` |
| Unknown field in `eq: true` defaults to pass (line 119) — may hide bugs in test definitions | Low | `checks.go:119` |

### Weighting
- All constraints have equal weight (1.0) in `Aggregate`. Per-check weighting is supported but not used in any suite.

### Normalization
- `extractNumber` trims and scans fields — reasonable for simple numeric answers.
- String `eq` does NOT normalize whitespace beyond `TrimSpace` — fine for exact match constraints.

### Threshold Logic
- No threshold logic — boolean pass/fail per constraint. Aggregation is mean of per-test scores.

---

## 2. Structured Output Scorer (checkValid, checkSuperset)

### Files
- `checks.go:156-224`
- `checks_test.go:217-274`

### Correctness
- **valid**: Tries JSON parsing directly, then falls back to extracting from ``` code fences. **Correct.**
- **superset**: Parses JSON, checks key presence. Falls back to code fence extraction. **Correct.**

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| `valid` accepts any JSON (numbers, strings, arrays) — may not match "must be object" intent | Low | `checks.go:175` |
| Code fence extraction in `valid`/`superset` may grab wrong content if multiple fences exist | Medium | `checks.go:159-171` |
| `superset` with empty key list passes immediately (line 187) — may mask missing constraints | Low | `checks.go:185-188` |
| GEP `checkSuperset` does **contains** (substring) not JSON key check — different semantics than primary scorer | **High** | `gep/scorer/scorer.go:238-257` |

### Weighting
- Equal weight. No special handling.

### Threshold Logic
- None. Binary pass/fail.

---

## 3. Context Scorer (no dedicated implementation)

### Finding
**Context retention is NOT implemented in the primary `benchmark/scorer` package.** It exists only in the legacy `benchmark/gep/` subsystem as `SuiteContextRetention`.

### GEP Context Retention
- **Files:** `benchmark/gep/suites/context_retention/{easy,medium,hard}.yaml` (15 tests total)
- **Execution:** Multi-turn conversations via `type: multi_turn` in `gep/runner/runner.go:139-148`
- **Scoring:** Uses generic operators (`eq`, `not_contains`, `superset`) on the final turn's response
- **No dedicated scorer** — relies on answer-match constraints

### Issues Identified
| Issue | Severity | Location |
|-------|----------|----------|
| Context retention tests are in GEP only, not in primary benchmark | Medium | `suites/_manifest.yaml` — no `context_retention` category |
| GEP scorer `checkEQ` string comparison is case-insensitive (`strings.EqualFold`) | Low | `gep/scorer/scorer.go:152` |
| GEP scorer `checkAnswerMatch` uses substring containment (`strings.Contains`) | Medium | `gep/scorer/scorer.go:319-324` |
| Multi-turn context may exceed model context window for small models | High | Runner has no context window check |
| Hard tier tests have 6-7 turns — may be unrealistic for 4B models | Medium | `gep/suites/context_retention/hard.yaml` |

---

## 4. Consistency Scorer (checkSelfConsistency, ScoreSelfConsistency)

### Files
- `checks.go:289-325`
- `checks_test.go:515-590`

### Correctness
- **Primary scorer** (`benchmark/scorer`): Normalizes by `strings.Fields` (no case folding). Compares exact normalized strings. Returns 1.0 only if ALL responses are identical after normalization.
- **GEP scorer** (`benchmark/gep/scorer`): Normalizes by `strings.ToLower` + `strings.Fields`. Case-insensitive comparison.

### Critical Bug: Normalization Inconsistency
| Aspect | Primary Scorer | GEP Scorer |
|--------|---------------|------------|
| Case handling | **Case-sensitive** (`strings.Fields`) | **Case-insensitive** (`strings.ToLower`) |
| Result | "Uranium" vs "uranium" = **0.0** | "Uranium" vs "uranium" = **1.0** |

This inconsistency means the same test run through different scorers produces different results.

### Self-Consistency Pass Logic
- `checkSelfConsistency`: `passed := score == 1.0` — **must be 100% consistent**
- `ScoreSelfConsistency`: Returns 1.0 for 0 or 1 responses (vacuous truth)
- In runner (`runner.go:389-404`): `result.Passed = scored.Passed && firstErr == ""` — additional error check

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| All-identical responses score 1.0 even if all are wrong answers | Low (by design) | `checks.go:307-325` |
| Empty/nil response array returns 1.0 — may mask execution failures | Medium | `checks.go:308-309` |
| Whitespace-only differences are normalized away | Low (by design) | `checks.go:315` |
| No handling for markdown code fences in responses | Medium | `checks.go:315` |

### Weighting
- Self-consistency subscore is **0.0 or 1.0** only (no partial credit).
- In `runSelfConsistencyAttempt`, `expected_answer` subscore is also binary.

---

## 5. Latency Scorer (computeLatencyOverhead, OverallScore)

### Files
- `runner.go:480-506`
- `overall.go:61-80`

### Correctness
- **Latency overhead**: Average gumi latency minus average direct latency. Capped at 0 if negative. **Correct.**
- **Overall score**: Weighted sum of deltas minus latency penalty. `worthIt` if score > 0.05.

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| `latencyOverhead` is total average, not per-test — masks outlier behavior | Low | `runner.go:498-501` |
| Latency penalty is `min(overhead/1000, w.LatencyCost)` — scales linearly up to cap | Low | `overall.go:75` |
| No timeout-based penalty for tests that timed out | Medium | `runner.go:241-244` |
| Frontier tier weights are extremely skewed (Degradation: 0.50, most others: 0.05) | Low (design choice) | `overall.go:37-46` |

### Threshold Logic
- `worthIt`: Score > 0.05 (5% improvement threshold)
- No per-capability pass/fail thresholds
- Pass/fail is per-constraint (binary), not per-capability

---

## 6. Degradation Scorer (DegradationDetector)

### Files
- `degradation.go:30-234`
- `degradation_test.go`

### Correctness
- **Compare**: Returns empty record if identical. Classifies as "cosmetic" (whitespace) or "semantic" (numbers/keys changed). **Correct.**
- **RunDegradationChecks**: Groups by test ID, compares direct vs best-gumi result. Only flags if direct passed. **Correct.**

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| `stringSliceEqual` checks order — same keys in different order = "changed" | Low | `degradation.go:139-149` |
| JSON key detection regex `"([^"]+)"\s*:` may miss nested keys in some formats | Medium | `degradation.go:127` |
| Number extraction `\b\d+(?:\.\d+)?\b` doesn't match negative numbers | Low | `degradation.go:121` |
| `RunDegradationChecks` not tested (0% coverage) | High | `degradation.go:161-234` |

---

## 7. Math Scorer (mathAnswerCheck, extractMathAnswer)

### Files
- `math.go:14-79`
- **Coverage: 0%** — no tests exist

### Correctness
- Extracts answer via: (1) `####` marker, (2) last number in response, (3) generic `extractNumber`
- Compares parsed float to expected float

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| No test coverage for this entire file | **Critical** | `math.go` |
| `extractMathAnswer` takes LAST number — may pick wrong number in complex responses | Medium | `math.go:68` |
| Fraction answers (e.g., "1/6") are NOT handled — will parse as "1" then "6" | **High** | `math.go:67-75` |
| Negative answers not handled in regex `-\d+` but not in GSM8K format | Low | `math.go:58` |
| Number with commas like "1,000" parsed but "1,234.56" may fail | Medium | `math.go:58` |

---

## 8. Code Scorer (pythonExecCheck, extractPythonCode)

### Files
- `code.go:18-214`
- `code_test.go`

### Correctness
- **pythonExecCheck**: Extracts Python from response, builds test script, runs with timeout. **Correct.**
- **extractPythonCode**: Handles fenced and unfenced code. **Correct.**
- **buildPythonTestSource**: Avoids duplicate signatures. **Correct.**

### Edge Cases
| Issue | Severity | Location |
|-------|----------|----------|
| `stripImports` has 0% coverage — untested path | Medium | `code.go:177` |
| `pythonBinary` falls back to "python3" even if not found | Low | `code.go:194-201` |
| Timeout is per-test, not per-attempt — long-running tests may hang | Low | `code.go:65-70` |
| No handling for non-Python code responses (e.g., TypeScript) | Low | `code.go:100-130` |

---

## 9. CheckRegistry Completeness

### Primary Scorer (`benchmark/scorer/checks.go`)
```go
var CheckRegistry = map[string]CheckFunc{
    "eq":               checkEQ,
    "gte":              checkGTE,
    "lte":              checkLTE,
    "valid":            checkValid,
    "superset":         checkSuperset,
    "not_contains":     checkNotContains,
    "starts_with":      checkStartsWith,
    "ends_with":        checkEndsWith,
    "no_markdown":      checkNoMarkdown,
    "no_commas":        checkNoCommas,
    "self_consistency": checkSelfConsistency,
    "python_exec":      pythonExecCheck,
    "math_answer":      mathAnswerCheck,
}
```

### Missing Operators (referenced in suite YAMLs but not implemented)
| Operator | Referenced In | Status |
|----------|--------------|--------|
| `contains` | `degradation/semantic.yaml` (field: `contains`) | **Not in registry** — would fail |
| `sentence_count` | `instruction/frontier.yaml` | **Not in registry** — would fail |
| `min_chars` | Multiple frontier/easy suites | **Not in registry** — would fail |
| `unique_lines` | `repetition/medium.yaml` | **Not in registry** — would fail |
| `code_unchanged` | `degradation/semantic.yaml` | **Not in registry** — would fail |
| `answer_correct` | `reasoning/frontier.yaml` | **Not in registry** — would fail |
| `reasoning_quality` | `reasoning/frontier.yaml` | **Not in registry** — would fail |

**These missing operators cause tests to fail with "unknown operator" errors, contributing to the low pass rate.**

---

## 10. GEP Scorer Differences from Primary Scorer

| Aspect | Primary (`benchmark/scorer`) | GEP (`benchmark/gep/scorer`) |
|--------|-----------------------------|------------------------------|
| `eq` string comparison | Case-sensitive (`==`) | Case-insensitive (`EqualFold`) |
| `eq` supports `int` type | Yes | **No** — falls through to default |
| `superset` behavior | JSON key check | **Substring check** (different semantics) |
| `self_consistency` normalization | No case fold | Case-insensitive |
| `valid` vs `json` | Separate operators | `json` is alias for `valid` |
| Answer match operators | N/A | Has `answer_match`, `numeric_correct`, `expected_answer_match` |
| Constraint value types | `interface{}` | `types.GEPConstraint` |
| `extractNumber` | In `checks.go` | In `scorer.go` (duplicate) |
| `toFloat64` | In `checks.go` | In `scorer.go` (duplicate) |

---

## 11. Coverage Summary

| Package | Coverage | Missing |
|---------|----------|---------|
| `benchmark/scorer` | 77.7% | `math.go` (0%), `RunDegradationChecks` (0%), `stripImports` (0%) |
| `benchmark/gep/scorer` | 40.5% | Most operators untested |
| `benchmark/runner` | 27.7% | `Execute`, `runSingleAttempt`, `runSelfConsistencyAttempt` |
| `benchmark/gep/runner` | 24.4% | Suite loading, aggregation |

---

## 12. Findings Summary

### Critical
1. **Missing operators** in primary scorer registry: `contains`, `sentence_count`, `min_chars`, `unique_lines`, `code_unchanged`, `answer_correct`, `reasoning_quality` — all cause test failures
2. **Zero coverage on `math.go`** — math answer extraction is untested
3. **Zero coverage on `RunDegradationChecks`** — degradation detection is untested

### High
4. **Self-consistency normalization inconsistency** between primary and GEP scorers (case-sensitive vs case-insensitive)
5. **GEP `superset` does substring check** instead of JSON key check
6. **`stripImports` function has 0% coverage** — dead code path?

### Medium
7. `extractNumber` picks first number, may be wrong in multi-number responses
8. `extractMathAnswer` takes last number, may be wrong
9. Fraction answers in math not handled
10. Context retention not in primary benchmark (only GEP)
11. Empty response array in self-consistency returns 1.0 (vacuous truth)

### Low
12. Unknown field in `eq: true` defaults to pass
13. `eq` string comparison is case-sensitive (intentional but may surprise)
14. Latency overhead is averaged, not per-test
