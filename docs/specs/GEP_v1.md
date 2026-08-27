> **Historical — Pre-Pivot Architecture (Frozen)**
> This document describes the pre-pivot Gumi runtime / dashboard / benchmark architecture. It is **not** the current V1 product. The current product is the **local inference auto-tuner** specified in `26-gumi-v1-auto-tuner.md` (with `23-optimization-engine-mvp.md` · `24-verification-confidence-phase2.md` · `25-evidence-hardening.md` · `27-gumi-v1-release-audit.md`). This file is retained for provenance — do not extend it.

---

# Gumi Evaluation Protocol (GEP) v1 Specification

**Version:** 1.0.0  
**Status:** Active  
**Sprint:** 16A  
**Scope:** Provider-independent benchmark framework for evaluating local AI models

---

# 1. Purpose

The Gumi Evaluation Protocol (GEP) is a provider-agnostic benchmark framework designed to measure how well local AI models perform across five core capabilities:

1. **Instruction Following** — Can the model obey explicit formatting and content constraints?
2. **Structured Output** — Can the model produce valid, schema-compliant JSON?
3. **Consistency** — Does the model give the same answer when asked the same question differently?
4. **Context Retention** — Can the model remember information across multi-turn conversations?
5. **Latency** — How fast does the model respond under varying load?

GEP is **independent of runtime implementation**. It evaluates models directly against their providers (LM Studio, Ollama) without routing through the Gumi runtime, making it a pure model-quality benchmark.

---

# 2. Design Principles

| Principle | Rationale |
|-----------|-----------|
| **Provider-agnostic** | Same test suites work for LM Studio, Ollama, and any OpenAI-compatible server |
| **Independent of Gumi runtime** | Measures model capability, not Gumi's enhancement |
| **Progressive difficulty** | Easy → Medium → Hard tiers allow calibration across model sizes |
| **Multi-turn support** | Context retention tests use explicit turn sequences |
| **Self-consistency** | Same question, different phrasings — measures reliability |
| **Extensible suites** | New test categories added via YAML, no Go recompilation |
| **Baseline storage** | Historical runs stored for regression detection |
| **Dual report format** | JSON for machine parsing, Markdown for human review |

---

# 3. Architecture

## 3.1 System Context

```
┌─────────────────────────────────────────────────────────────────┐
│                     GEP Benchmark Framework                      │
│                                                                 │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐   │
│  │   Runner     │   │   Scorer     │   │   Reports        │   │
│  │  (execution) │──▶│  (validation)│──▶│  (JSON + MD)     │   │
│  └──────┬───────┘   └──────────────┘   └──────────────────┘   │
│         │                                                       │
│  ┌──────▼───────┐   ┌──────────────┐   ┌──────────────────┐   │
│  │   Suites     │   │  Baselines   │   │   Types          │   │
│  │  (YAML defs) │   │  (storage)   │   │  (data models)   │   │
│  └──────────────┘   └──────────────┘   └──────────────────┘   │
│                                                                 │
└────────────────────────┬────────────────────────────────────────┘
                         │
              ┌──────────┴──────────┐
              │   Provider Adapters  │
              │  (LM Studio, Ollama) │
              └──────────┬──────────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
     ┌─────────┐   ┌─────────┐   ┌─────────┐
     │LM Studio│   │ Ollama  │   │  (future)│
     │ :1234/v1│   │ :11434  │   │          │
     └─────────┘   └─────────┘   └─────────┘
```

## 3.2 Package Layout

```
benchmark/gep/
├── types/
│   ├── types.go          # Core data models (GEPTest, GEPReport, etc.)
│   └── types_test.go     # Type validation tests
├── providers/
│   ├── providers.go      # Provider interface + factory
│   ├── lmstudio.go       # LM Studio adapter (OpenAI-compatible)
│   ├── ollama.go         # Ollama adapter
│   └── providers_test.go # Provider tests
├── suites/
│   ├── instruction_following/
│   │   ├── easy.yaml     # 2-3 constraints
│   │   ├── medium.yaml   # 4-5 constraints
│   │   └── hard.yaml     # 6+ constraints
│   ├── structured_output/
│   │   ├── easy.yaml     # Simple JSON objects
│   │   ├── medium.yaml   # Nested structures
│   │   └── hard.yaml     # Schema-compliant output
│   ├── consistency/
│   │   ├── easy.yaml     # 4 variants, simple facts
│   │   ├── medium.yaml   # 4 variants, reasoning
│   │   └── hard.yaml     # 3 variants, complex logic
│   ├── context_retention/
│   │   ├── easy.yaml     # 2-3 turns, single fact
│   │   ├── medium.yaml   # 4-5 turns, multiple facts
│   │   └── hard.yaml     # 6+ turns, complex state
│   └── latency/
│       └── easy.yaml     # Latency measurements
├── scorer/
│   ├── scorer.go         # Constraint evaluation engine
│   └── scorer_test.go    # Scorer unit tests
├── runner/
│   ├── runner.go         # Test execution orchestration
│   └── runner_test.go    # Runner unit tests
├── baselines/
│   ├── baselines.go      # Baseline storage and comparison
│   └── baselines_test.go # Baseline tests
└── reports/
    ├── reports.go        # JSON and Markdown writers
    └── reports_test.go   # Report generation tests
```

---

# 4. Data Model

## 4.1 Core Types

```go
// GEPTest is a single evaluation case.
type GEPTest struct {
    ID             string         `yaml:"id" json:"id"`
    Difficulty     DifficultyTier `yaml:"difficulty" json:"difficulty"`
    Description    string         `yaml:"description" json:"description"`
    Prompt         string         `yaml:"prompt" json:"prompt"`
    SystemPrompt   string         `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
    Type           string         `yaml:"type,omitempty" json:"type,omitempty"` // "self_consistency" or "multi_turn"
    Variants       []string       `yaml:"variants,omitempty" json:"variants,omitempty"`
    Turns          []Turn         `yaml:"turns,omitempty" json:"turns,omitempty"`
    ExpectedAnswer string         `yaml:"expected_answer,omitempty" json:"expected_answer,omitempty"`
    TimeoutSeconds int            `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
    MaxTokens      int            `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
    Constraints    []GEPConstraint `yaml:"constraints,omitempty" json:"constraints,omitempty"`
}

// GEPConstraint defines a single check.
type GEPConstraint struct {
    Field    string      `yaml:"field" json:"field"`
    Operator string      `yaml:"operator" json:"operator"`
    Value    interface{} `yaml:"value" json:"value"`
}
```

## 4.2 Suite Definition

```yaml
# suites/instruction_following/easy.yaml
suite:
  id: instruction-following
  type: instruction_following
  difficulty: easy
  description: "Test basic instruction following with 2-3 constraints"
  attempts_recommended: 3
  timeout_seconds: 60
  max_tokens: 512

tests:
  - id: inst-easy-01
    difficulty: easy
    description: "2-sentence response with forbidden word"
    prompt: |
      Explain what Go is in exactly 2 sentences. Do not use the word "programming".
    constraints:
      - field: sentence_count
        operator: eq
        value: 2
      - field: forbidden_words
        operator: not_contains
        value: ["programming"]
      - field: non_empty
        operator: eq
        value: true
```

## 4.3 Multi-Turn Test

```yaml
  - id: ctx-easy-01
    difficulty: easy
    description: "Remember a number across turns"
    prompt: |
      Turn 1: Remember this number: 42. Reply with just "OK".
      Turn 2: What number did I ask you to remember?
    type: multi_turn
    turns:
      - role: user
        content: "Remember this number: 42. Reply with just OK."
      - role: assistant
        content: "OK"
      - role: user
        content: "What number did I ask you to remember?"
    expected_answer: "42"
    constraints:
      - field: answer_match
        operator: eq
        value: "42"
```

## 4.4 Self-Consistency Test

```yaml
  - id: cons-easy-01
    difficulty: easy
    description: "Math fact consistency"
    prompt: "What is 12 * 12?"
    type: self_consistency
    variants:
      - "Calculate twelve times twelve."
      - "12 multiplied by 12 equals what?"
      - "Find the product of 12 and 12."
      - "Twelve squared is?"
    expected_answer: "144"
    constraints:
      - field: self_consistency
        operator: self_consistency
        value: []
      - field: numeric_correct
        operator: eq
        value: 144
```

---

# 5. Constraint Operators

| Operator | Field | Description | Example Value |
|----------|-------|-------------|---------------|
| `eq` | string | Case-insensitive string match | `"hello"` |
| `eq` | int/float | Numeric equality | `42` |
| `eq` | bool | Boolean check (delegates to field semantics) | `true` |
| `gte` | int/float | Greater than or equal | `100` |
| `lte` | int/float | Less than or equal | `500` |
| `valid` | — | JSON validity check | `null` |
| `superset` | string[] | All expected values present | `["sunlight", "energy"]` |
| `not_contains` | string[] | No forbidden values present | `["the", "and"]` |
| `starts_with` | string | Response starts with prefix | `"In"` |
| `ends_with` | string | Response ends with suffix | `"progress."` |
| `no_markdown` | bool | No ``` fences | `true` |
| `no_commas` | bool | No comma characters | `true` |
| `self_consistency` | string[] | All variants identical (normalized) | `["var1", "var2"]` |
| `answer_match` | string | Response contains expected answer | `"42"` |
| `numeric_correct` | number | First number in response matches | `144` |
| `expected_answer_match` | bool | Metadata flag for expected answer check | `true` |

### Boolean `eq` Field Semantics

When `operator: eq` with `value: true` is used, the field name determines the check:

| Field | Check Performed |
|-------|-----------------|
| `no_markdown` | Response contains no ``` fences |
| `no_commas` | Response contains no comma characters |
| `capital_start` | First non-whitespace character is uppercase |

---

# 6. Provider Adapters

## 6.1 Interface

```go
type Provider interface {
    ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Health(ctx context.Context) error
    Type() string
    Name() string
}
```

## 6.2 LM Studio Adapter

- **URL:** `http://localhost:1234/v1` (OpenAI-compatible)
- **Endpoint:** `POST /chat/completions`
- **Auth:** Optional Bearer token
- **Request format:** Standard OpenAI Chat Completion API

```go
provider, err := providers.NewProvider("lmstudio", "http://localhost:1234/v1", "")
```

## 6.3 Ollama Adapter

- **URL:** `http://localhost:11434`
- **Endpoint:** `POST /api/chat`
- **Auth:** None (local only)
- **Request format:** Ollama native API (messages array, options for num_predict/temperature)

```go
provider, err := providers.NewProvider("ollama", "http://localhost:11434", "")
```

---

# 7. Benchmark Execution

## 7.1 Configuration

```go
type RunConfig struct {
    Model       string
    Provider    types.ProviderType
    ProviderURL string
    APIKey      string
    Attempts    int
    SuiteID     string        // Optional: run only this suite
    Difficulty  DifficultyTier // Optional: filter by difficulty
    OutputDir   string        // Defaults to benchmarks/gep/reports
}
```

## 7.2 Execution Flow

```
1. Resolve provider adapter (LM Studio / Ollama)
2. Load suites from benchmark/gep/suites/
3. For each suite:
   For each test:
     For each attempt (1..N):
       a. Build request (handle multi_turn or single_prompt)
       b. Call provider.ChatCompletion()
       c. Score response against constraints
       d. Record GEPResult
4. Aggregate results by suite type
5. Compute summary statistics
6. Write JSON + Markdown reports
7. Store baseline for future regression comparison
```

## 7.3 Example Usage

```go
import "github.com/EffNine/gumi/benchmark/gep/runner"

cfg := runner.RunConfig{
    Model:       "qwen2.5-coder-7b-instruct",
    Provider:    "lmstudio",
    ProviderURL: "http://localhost:1234/v1",
    Attempts:    3,
    OutputDir:   "benchmarks/gep/reports",
}

report, err := runner.Run(cfg)
```

---

# 8. Suite Catalog

## 8.1 Instruction Following (5-15 tests per tier)

Tests the model's ability to obey explicit formatting and content constraints.

| Tier | Constraints | Example Checks |
|------|-------------|----------------|
| Easy | 2-3 | sentence_count, forbidden_words, ends_with |
| Medium | 4-5 | json+keys, no_markdown, min_chars, capital_start |
| Hard | 6+ | nested JSON, alphabetical constraints, math with format |

**Sample tests:**
- `inst-easy-01`: 2-sentence response, no "programming"
- `inst-med-03`: JSON with name="Alice", age 25-35, no markdown
- `inst-hard-02`: OpenAPI-like schema with type enforcement

## 8.2 Structured Output (5-10 tests per tier)

Tests JSON generation quality with increasing schema complexity.

| Tier | Focus | Example |
|------|-------|---------|
| Easy | Flat objects, arrays | `{"name": "Test", "age": 42}` |
| Medium | Nested objects, type constraints | Library catalog, employee records |
| Hard | Schema compliance, conditional structure | OpenAPI specs, paginated APIs |

## 8.3 Consistency (5 tests per tier)

Self-consistency: same question, different phrasings, expect same answer.

| Tier | Type | Example |
|------|------|---------|
| Easy | Math facts, simple facts | "What is 12*12?" |
| Medium | Multi-step math, temporal reasoning | "If today is Wednesday, what in 72h?" |
| Hard | Complex reasoning, probability | "Probability of rolling 6 in two dice" |

## 8.4 Context Retention (5 tests per tier)

Multi-turn conversations requiring the model to remember earlier information.

| Tier | Turns | Example |
|------|-------|---------|
| Easy | 2-3 turns | "Remember 42. What was it?" |
| Medium | 4-5 turns | Remember multiple facts across 4 turns |
| Hard | 6+ turns | Shopping list, state changes, rules |

## 8.5 Latency (5-6 tests)

Measures response time across varying prompt lengths and output types.

| Test | Prompt Length | Expected Latency |
|------|--------------|------------------|
| lat-short-01 | 3 words | < 5s |
| lat-short-02 | 10 words, JSON | < 5s |
| lat-medium-01 | 80 words | < 10s |
| lat-medium-02 | 80 words, JSON | < 15s |
| lat-long-01 | 200+ words | < 20s |
| lat-long-02 | Long context, multi-turn | < 30s |

---

# 9. Baseline Storage & Regression

## 9.1 Storage Location

Baselines are stored at `~/.gumi/gep/baselines/<model>/<run-id>-<timestamp>.json`.

## 9.2 Baseline Format

```json
{
  "run_id": "gep-qwen2.5-coder-7b-20260807T120000Z",
  "model": "qwen2.5-coder-7b-instruct",
  "provider": "lmstudio",
  "timestamp": "2026-08-07T12:00:00Z",
  "overall_score": 0.85,
  "capabilities": {
    "instruction_following": {"mean": 0.9, "std": 0.05, "n": 15},
    "structured_output": {"mean": 0.88, "std": 0.04, "n": 10}
  },
  "config": { ... }
}
```

## 9.3 Regression Detection

When a new run completes, it is automatically compared against the latest baseline for the same model:

```go
regression, err := store.Compare(newReport)
// regression.Regression == true if current < baseline
// regression.CapabilityDeltas shows per-suite changes
```

---

# 10. Report Generation

## 10.1 JSON Report

Written to `<output_dir>/<run-id>.json`. Contains:
- Full run configuration
- Summary statistics (overall score, pass rate, avg latency)
- Per-capability metrics (mean, std, N, min, max, median, P25, P75)
- Per-test results with subscores and latency
- Optional regression comparison

## 10.2 Markdown Report

Written to `<output_dir>/<run-id>.md`. Contains:
- Header with model, provider, run ID, timestamp
- Summary table (overall score, pass rate, latency, worth it?)
- Capabilities table with mean ± std
- Per-test results table
- Footer with protocol version

---

# 11. CLI Usage

## 11.1 Running GEP Benchmarks

```bash
# Full benchmark across all suites
GUMI_URL=http://127.0.0.1:8787/v1 \
GUMI_PROVIDER=lmstudio \
GUMI_PROVIDER_URL=http://localhost:1234/v1 \
go run ./benchmark/cmd/run-gep.go --model qwen2.5-coder-7b-instruct --attempts 3

# Run only instruction following suite
go run ./benchmark/cmd/run-gep.go --model qwen2.5-coder-7b-instruct --suite instruction-following

# Run only easy tier
go run ./benchmark/cmd/run-gep.go --model qwen2.5-coder-7b-instruct --difficulty easy
```

## 11.2 Managing Baselines

```bash
# List stored baselines
go run ./benchmark/cmd/gep-cli.go baselines list

# Compare latest run against baseline
go run ./benchmark/cmd/gep-cli.go baselines compare --model qwen2.5-coder-7b-instruct
```

---

# 12. Testing

```bash
# Run all GEP tests
cd benchmark && go test ./gep/...

# Run specific package
go test ./gep/scorer/...
go test ./gep/baselines/...

# Run with coverage
go test -cover ./gep/...
```

---

# 13. Future Work

| Feature | Priority | Description |
|---------|----------|-------------|
| Additional providers | P2 | vLLM, llama.cpp server, LocalAI |
| Dashboard integration | P2 | Visual trend charts in dashboard |
| Custom suite import | P3 | `--suite path/to/custom.yaml` |
| Adversarial prompts | P3 | LLM-generated edge cases |
| Cross-model comparison | P3 | Heatmap of GEP scores across models |
| CI integration | P2 | Automated GEP runs on PR |

---

# 14. Relationship to Existing Benchmark

GEP complements (does not replace) the existing `benchmark/` package:

| Aspect | Existing Benchmark | GEP |
|--------|-------------------|-----|
| Provider | Through Gumi runtime | Direct to LM Studio/Ollama |
| Focus | Gumi's enhancement effect | Pure model capability |
| Suites | Coding, Math, JSON, Instruction, etc. | Instruction, Structured Output, Consistency, Context, Latency |
| Baselines | No storage | Full baseline + regression |
| Multi-turn | No | Yes (context retention) |
| Self-consistency | Partial | Full (4-5 variants) |

Both use the same YAML suite format and can share test definitions.

---

**End of Specification**
