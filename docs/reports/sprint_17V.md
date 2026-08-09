# Sprint 17V Report — Runtime-Aware GEP Validation

**Date:** 2026-08-09  
**Sprint:** 17V  
**Status:** Complete  
**Protocol:** GEP v2.0.0

---

## Primary Objective

Convert GEP into the single official benchmark for runtime changes by comparing each GEP test under two conditions: `direct` (raw provider) and `gumi-stabilized` (through Gumi runtime). This enables measurement of the instruction engine's actual impact on model performance.

---

## Changes Implemented

### 1. GEP Protocol Upgrade to v2.0.0

**File:** `benchmark/gep/types/types.go`

- Upgraded `ProtocolVersion` from `"1.0.0"` to `"2.0.0"`
- Upgraded `SchemaVersion` to `2`
- Added new types for condition dimension:
  - `GEPCondition` — identifies `direct` vs `gumi-stabilized` execution
  - `GEPScope` — distinguishes `model` (raw capability) vs `runtime` (effectiveness) baselines

### 2. Condition Dimension in Types

**File:** `benchmark/gep/types/types.go`

```go
type GEPCondition string
const (
    ConditionDirect           GEPCondition = "direct"
    ConditionGumiStabilized   GEPCondition = "gumi-stabilized"
)

type GEPScope string
const (
    ScopeModel     GEPScope = "model"
    ScopeRuntime   GEPScope = "runtime"
)
```

### 3. Extended Run Configuration

**File:** `benchmark/gep/types/types.go`

Added to `GEPRunConfig`:
- `Conditions []GEPCondition` — which execution paths to run
- `GumiURL string` — Gumi runtime base URL
- `GumiAPIKey string` — bearer token for Gumi runtime
- `Scope GEPScope` — baseline scope

### 4. Extended Result with Condition

**File:** `benchmark/gep/types/types.go`

Added to `GEPResult`:
- `Condition GEPCondition` — which condition this result belongs to

### 5. Per-Condition Capability Metrics

**File:** `benchmark/gep/types/types.go`

`GEPCapability` now tracks both conditions:
```go
type GEPCapability struct {
    SuiteType  SuiteType
    Direct     GEPMetricSet  // direct condition metrics
    Gumi       GEPMetricSet  // gumi-stabilized metrics
    Delta      float64       // gumi - direct
    PassRate   float64       // pass rate delta
    Desc       string
}
```

### 6. Extended Summary with Deltas

**File:** `benchmark/gep/types/types.go`

`GEPSummary` now includes:
- `DirectScore`, `GumiScore`, `ScoreDelta`
- `DirectPassRate`, `GumiPassRate`, `PassRateDelta`
- `DirectLatencyMs`, `GumiLatencyMs`, `LatencyDeltaMs`

### 7. Scoped Baseline Storage

**File:** `benchmark/gep/baselines/baselines.go`

- Baselines now stored in `~/.gumi/gep/baselines/{scope}/{model}/`
- `Load()` searches across all scopes for backward compatibility
- `ListModels()` returns models from all scopes
- Legacy unscoped baselines remain accessible

### 8. Dual-Condition Runner

**File:** `benchmark/gep/runner/runner.go`

- `runAttempt()` now accepts `GEPCondition` parameter
- When `ConditionGumiStabilized` and `GumiURL` is set, calls `callGumiRuntime()`
- `callGumiRuntime()` sends requests through Gumi's `/v1/chat/completions` API
- `aggregateCapabilities()` groups by suite AND condition
- `computeSummary()` computes per-condition metrics and deltas

### 9. Updated Reports

**File:** `benchmark/gep/reports/reports.go`

Markdown reports now show:
- Per-condition scores and deltas in summary table
- Per-capability tables with direct/gumi columns and delta
- GEP protocol version in footer

### 10. New CLI Command: `gumi gep run`

**File:** `runtime/internal/cli/gep.go`

```
gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 [flags]
```

Flags:
- `--model` (required): Model name
- `--provider` (required): Provider type (ollama, lmstudio)
- `--provider-url` (required): Direct provider API URL
- `--gumi-url`: Gumi runtime URL for gumi-stabilized condition
- `--gumi-api-key`: Gumi runtime API key
- `--attempts`: Number of attempts per test (default: 3)
- `--suite`: Suite ID to run
- `--difficulty`: Difficulty tier (default: easy)
- `--conditions`: Comma-separated conditions (default: direct,gumi-stabilized)
- `--output`: Output directory
- `--scope`: Baseline scope (model or runtime, default: runtime)
- `--json`: Machine-readable output

---

## Files Modified

| File | Lines Changed | Description |
|------|--------------|-------------|
| `benchmark/gep/types/types.go` | +80 | New condition/scope types, extended structs |
| `benchmark/gep/runner/runner.go` | +120 | Dual-condition execution, runtime API calls |
| `benchmark/gep/baselines/baselines.go` | +50 | Scoped storage, backward-compatible load |
| `benchmark/gep/reports/reports.go` | +40 | Condition-aware report output |
| `benchmark/gep/run_benchmark.go` | +5 | Updated capabilities print |
| `runtime/internal/cli/gep.go` | +180 | New CLI command |
| `runtime/internal/cli/cli.go` | +5 | Route to new command |
| Test files | +200 | Comprehensive new tests |

**Total new code:** ~700 lines  
**Total tests added:** 23

---

## Validation

### Unit Tests

```
go test ./benchmark/gep/...  → ALL PASS (31 tests)
go test ./runtime/...        → ALL PASS (23 packages)
go vet ./benchmark/...       → CLEAN
go vet ./runtime/...         → CLEAN
go fmt ./benchmark/...       → APPLIED
go fmt ./runtime/...         → APPLIED
make build                   → SUCCESS
```

### New Test Coverage

| Test | Purpose |
|------|---------|
| `TestDefaultConditions` | Verifies direct + gumi-stabilized defaults |
| `TestComputeSummaryWithBothConditions` | Validates per-condition score computation |
| `TestComputeSummaryOnlyDirect` | Handles single-condition runs |
| `TestComputeSummaryOnlyGumi` | Handles single-condition runs |
| `TestAggregateCapabilitiesWithConditions` | Validates per-condition metric aggregation |
| `TestGEPConditionConstants` | Verifies condition string values |
| `TestGEPScopeConstants` | Verifies scope string values |
| `TestProtocolVersion` | Verifies v2.0.0 |
| `TestStoreSaveRuntimeScoped` | Validates runtime scope storage |
| `TestStoreSaveModelScoped` | Validates model scope storage |
| `TestStoreLoadAcrossScopes` | Validates cross-scope loading |
| `TestStoreListModelsAcrossScopes` | Validates cross-scope model listing |
| `TestStoreCompareWithScope` | Validates scoped comparison |

---

## Backward Compatibility

| Aspect | Status |
|--------|--------|
| Existing `gumi benchmark` command | ✅ Unchanged |
| Legacy baseline files (unscoped) | ✅ Still loadable |
| Existing test suites (YAML) | ✅ No changes required |
| Protocol version bump | ✅ Schema version 2, backward-compatible JSON |

---

## Live Benchmark Instructions

### Prerequisites

1. Start Ollama:
   ```bash
   ollama serve
   ```

2. Pull models (if not present):
   ```bash
   ollama pull qwen3:8b
   ollama pull gemma3:4b
   ```

3. Start Gumi runtime:
   ```bash
   gumi start
   ```

### Run Qwen3 8B Benchmark

```bash
gumi gep run \
  --model qwen3:8b \
  --provider ollama \
  --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 \
  --gumi-api-key gumi-local \
  --attempts 3 \
  --difficulty easy,medium
```

### Run Gemma 3 4B Benchmark

```bash
gumi gep run \
  --model gemma3:4b \
  --provider ollama \
  --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 \
  --gumi-api-key gumi-local \
  --attempts 3 \
  --difficulty easy,medium
```

### Expected Output Format

```
=== Results ===
Overall Score: 0.65
Pass Rate: 45.0%
Total Tests: 20 (Passed: 9)
Run ID: gep-qwen3-8b-20260809T120000Z
Timestamp: 2026-08-09T12:00:00Z

--- Condition Breakdown ---
Direct Score: 0.55 | Gumi Score: 0.65 | Delta: +0.10
Direct Pass Rate: 35.0% | Gumi Pass Rate: 45.0% | Delta: +10.0pp
Direct Latency: 9800ms | Gumi Latency: 10200ms | Delta: +400ms

--- Capabilities ---
  instruction_following: direct=0.50 gumi=0.65 delta=+0.15 pass_rate_delta=+15.0pp
  structured_output: direct=0.60 gumi=0.70 delta=+0.10 pass_rate_delta=+10.0pp

Worth it: true
```

---

## Merge Gate Criteria

Sprint 17 changes are MERGE-GATED until live GEP v2.0.0 results demonstrate:

1. **Positive overall runtime delta** for both Qwen3 8B and Gemma 3 4B
2. **No critical capability regression** >2 percentage points in:
   - Instruction following
   - Structured output (JSON)
   - Consistency
   - Context retention
3. **Latency overhead reported** — no unmeasured latency claims accepted

### Acceptance Threshold

| Metric | Required |
|--------|----------|
| Overall score delta | > 0 (positive) |
| Instruction following delta | > 0 (positive) |
| JSON compliance delta | > 0 (positive) |
| Latency overhead | < 20% |
| Any capability regression | < 2pp |

---

## Deliverables

| File | Status |
|------|--------|
| `docs/reports/sprint_17.md` | ✅ Updated (pending live validation) |
| `docs/reports/instruction_engine_review.md` | ✅ |
| `docs/reports/instruction_optimization.md` | ✅ |
| `docs/reports/benchmark_delta.md` | ✅ Updated |
| `docs/reports/sprint_17V.md` | ✅ This file |

---

## Assumptions

1. Existing Sprint 16D model-capability baselines remain valid for raw model measurement
2. No runtime HTTP API, provider abstraction, context engine, plugin system, or session manager changes required
3. The current worktree may contain unrelated changes; this sprint isolates its own edits

---

**Sprint 17V Complete — awaiting live benchmark validation**
