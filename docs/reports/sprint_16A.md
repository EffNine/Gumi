# Sprint 16A Report: GEP Foundation

**Date:** 2026-08-07  
**Version:** v1.0.0-rc1  
**Sprint:** 16A  
**Status:** Complete

---

## Summary

Implemented the first version of the Gumi Evaluation Protocol (GEP) — a provider-independent benchmark framework for evaluating local AI models. GEP supports LM Studio and Ollama providers, defines 5 benchmark suites (Instruction Following, Structured Output, Consistency, Context Retention, Latency), and includes baseline storage with regression comparison. All code passes formatting, linting, and tests.

---

## Architecture Decision

GEP was designed as a **companion** to the existing `benchmark/` package, not a replacement. Key differences:

| Aspect | Existing Benchmark | GEP |
|--------|-------------------|-----|
| Routes through | Gumi runtime (127.0.0.1:8787) | Direct to provider |
| Measures | Gumi's enhancement effect | Pure model capability |
| Multi-turn | No | Yes (context retention suite) |
| Self-consistency | Basic | Full (4-5 variants per test) |
| Baselines | None | Stored at ~/.gumi/gep/baselines/ |
| Regression | None | Automatic comparison on each run |

---

## Files Created

### Core Framework

| File | Description |
|------|-------------|
| `benchmark/gep/types/types.go` | Core data models (GEPTest, GEPReport, GEPCapability, etc.) |
| `benchmark/gep/types/types_test.go` | Type validation tests |
| `benchmark/gep/providers/providers.go` | Provider interface + factory |
| `benchmark/gep/providers/lmstudio.go` | LM Studio adapter (OpenAI-compatible API) |
| `benchmark/gep/providers/ollama.go` | Ollama adapter (native /api/chat) |
| `benchmark/gep/providers/providers_test.go` | Provider factory tests |
| `benchmark/gep/scorer/scorer.go` | Constraint evaluation engine (16 operators) |
| `benchmark/gep/scorer/scorer_test.go` | Scorer unit tests |
| `benchmark/gep/runner/runner.go` | Test execution orchestration |
| `benchmark/gep/runner/runner_test.go` | Runner unit tests |
| `benchmark/gep/baselines/baselines.go` | Baseline storage and regression comparison |
| `benchmark/gep/baselines/baselines_test.go` | Baseline tests |
| `benchmark/gep/reports/reports.go` | JSON and Markdown report writers |

### Benchmark Suites (YAML)

| Suite | Files | Tests |
|-------|-------|-------|
| Instruction Following | `suites/instruction_following/{easy,medium,hard}.yaml` | 15 tests |
| Structured Output | `suites/structured_output/{easy,medium,hard}.yaml` | 15 tests |
| Consistency | `suites/consistency/{easy,medium,hard}.yaml` | 15 tests |
| Context Retention | `suites/context_retention/{easy,medium,hard}.yaml` | 15 tests |
| Latency | `suites/latency/easy.yaml` | 6 tests |

### Documentation

| File | Description |
|------|-------------|
| `docs/specs/GEP_v1.md` | Full GEP v1 specification (14 sections) |
| `docs/reports/sprint_16A.md` | This report |

---

## Constraint Operators Implemented

| Operator | Description |
|----------|-------------|
| `eq` | String equality (case-insensitive), numeric equality, boolean field checks |
| `gte` | Numeric >= check |
| `lte` | Numeric <= check |
| `valid` | JSON validity (with code-fence extraction) |
| `superset` | All expected values present in response |
| `not_contains` | No forbidden values present |
| `starts_with` | Response starts with prefix |
| `ends_with` | Response ends with suffix |
| `no_markdown` | No ``` fences |
| `no_commas` | No comma characters |
| `self_consistency` | All variants produce identical normalized output |
| `answer_match` | Response contains expected answer |
| `numeric_correct` | First number in response matches expected |
| `expected_answer_match` | Metadata flag for expected answer validation |
| `answer_type` | Response is yes/no type |
| `contains_expected` | Response contains any of expected values |

---

## Provider Support

### LM Studio
- **Endpoint:** `POST {baseURL}/chat/completions`
- **Auth:** Optional Bearer token
- **Request format:** Standard OpenAI Chat Completions API

### Ollama
- **Endpoint:** `POST {baseURL}/api/chat`
- **Auth:** None (local-only)
- **Request format:** Ollama native API (messages array, options for num_predict/temperature)

---

## Test Results

```
ok  github.com/EffNine/gumi/benchmark/gep/baselines   0.423s
ok  github.com/EffNine/gumi/benchmark/gep/providers    0.358s
ok  github.com/EffNine/gumi/benchmark/gep/runner       0.358s
ok  github.com/EffNine/gumi/benchmark/gep/scorer       0.484s
ok  github.com/EffNine/gumi/benchmark/gep/types        0.336s
```

All existing benchmark and runtime tests remain passing:
- `benchmark/...`: 11 packages, all pass
- `runtime/...`: 21 packages, all pass

---

## Validation Results

| Check | Result |
|-------|--------|
| `go vet ./gep/...` | ✅ Clean |
| `go test ./gep/...` | ✅ All pass (4 packages + types) |
| `go test ./...` (benchmark) | ✅ All pass (11 packages) |
| `go test ./...` (runtime) | ✅ All pass (21 packages) |
| `go vet ./...` (runtime) | ✅ Clean |
| `go work sync` | ✅ Clean |

---

## What Was NOT Done (Out of Scope)

- **Runtime integration** — GEP runs independently, not through the Gumi runtime API
- **Dashboard UI** — No dashboard changes for GEP (future sprint)
- **CLI command** — No `gumi gep` subcommand (uses Go run directly for now)
- **Additional providers** — Only LM Studio and Ollama (vLLM, llama.cpp are future)
- **Sweeping optimization** — No runtime behavior changes

---

## Usage Example

```bash
# Run GEP benchmark against LM Studio
cd benchmark && go run ./cmd/run-gep.go \
  --model qwen2.5-coder-7b-instruct \
  --provider lmstudio \
  --provider-url http://localhost:1234/v1 \
  --attempts 3 \
  --output-dir benchmarks/gep/reports

# Run only instruction following, easy tier
go run ./cmd/run-gep.go \
  --model qwen2.5-coder-7b-instruct \
  --provider ollama \
  --provider-url http://localhost:11434 \
  --suite instruction-following \
  --difficulty easy \
  --attempts 3
```

Reports are written to `benchmarks/gep/reports/<run-id>.json` and `<run-id>.md`.

---

## Architecture Impact

**None.** GEP is a completely new subsystem under `benchmark/gep/`. No existing types, interfaces, or public APIs were modified. The Gumi runtime is untouched.

---

## Remaining Work

| Item | Priority | Notes |
|------|----------|-------|
| CLI command `gumi gep` | P2 | Shell wrapper for Go runner |
| Dashboard GEP view | P2 | Trend charts for baseline comparison |
| CI integration | P2 | Automated runs on PR |
| Additional providers | P3 | vLLM, llama.cpp, LocalAI |
| Custom suite import | P3 | `--suite path/to/custom.yaml` |
| Hard tier completion | P3 | More hard tests for each suite |

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| YAML suite files too large | Low | Low | Split by category+tier; total <100 tests |
| Provider URL format differences | Medium | Low | Adapter handles normalization; tested against both |
| Self-consistency false negatives | Medium | Medium | Normalization is aggressive; future: Levenshtein tolerance |
| Baseline storage bloat | Low | Low | One file per run; <1KB per file |

---

## Recommendation

Sprint 16A is complete. GEP v1 foundation is implemented, tested, and documented. The framework is ready for real-world benchmark runs against local models.

**Next Sprint Recommendation:** Sprint 16B — GEP CLI Integration & Dashboard View

---

**Report Generated:** 2026-08-07
