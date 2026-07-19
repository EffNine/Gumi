# Agentic Coding Memory Engine Specification

Version: 1.0  
Status: **Implemented** ✅ (Phase 1 + CLI shipped — Sprint 12, 2026-07-13)
Scope: Cross-model persistent memory for agentic coding in Gumi Runtime

---

# 1. Purpose

This document defines the **Agentic Coding Memory Engine** for Gumi.

The Memory Engine provides a persistent, shared memory layer that:

- Lives entirely in **system RAM + SQLite** — zero GPU VRAM consumption
- Is **shared across all models** — when the router swaps models, memory persists
- Is **injected into context** as compressed, relevant facts — not raw history
- Feeds the **Agentic Coding Router** with per-model performance observations
- Survives **session boundaries** — facts learned in one session are available in
  the next

The Memory Engine is not a vector database, not a RAG system, and not a model
fine-tuning layer. It is a pragmatic, lightweight key-value store optimized for
coding agent memory.

---

# 2. The Problem

## 2.1 Coding Agents Are Stateless

Coding agents (Cline, Continue, OpenHands) operate in a loop:

```text
User Request → Model → Tool Calls → Observe → Next Step
```

Each step is a fresh model call. The model sees only the current context window.
Everything before — past steps, past failures, past successes — must be stuffed
into the prompt or it's lost.

This causes three problems:

| Problem | Consequence |
|---------|-------------|
| **Context window waste** | Repetition of past steps burns precious tokens |
| **No cross-model memory** | Switching from small to large model loses all accumulated state |
| **No session persistence** | Yesterday's debugging insights are gone today |

## 2.2 The Insight

> *"The model doesn't need to see the entire conversation history. It just needs
> to see the relevant facts."*

A memory engine can compress hours of agent interaction into a few hundred
tokens of structured facts — and those facts survive model swaps, session
boundaries, and context compression.

## 2.3 Why This Is Different from Context Window

| | Context Window | Memory Engine |
|---|---|---|
| Storage | GPU VRAM (model KV cache) | System RAM + Disk (SQLite) |
| Persistence | Per-request | Cross-session |
| Shared across models | No | Yes |
| Token cost | Full history retokenized | Compressed facts only |
| Volatility | Cleared every request | Persistent until evicted |
| Capacity | Model limit (4K-128K tokens) | Unlimited (disk-backed) |

---

# 3. Architecture

## 3.1 Position in Runtime

```
Agent Step
    ↓
Memory Engine (NEW)
    ├── Store current observation
    ├── Retrieve relevant facts
    └── Inject into context as system message
    ↓
Agentic Coding Router
    ↓
Pipeline Engine
    ↓
Provider + Model
    ↓
(response flows back through Memory Engine to extract new facts)
```

## 3.2 Components

```
Memory Engine
├── Fact Store        ← Key-value pairs (e.g., "project_language: Go")
├── Episode Store     ← Compressed step histories with outcomes
├── Model Fit Store   ← Per-model performance per task type (feeds router)
├── Injection Engine  ← Selects relevant facts and injects into context
├── Eviction Policy   ← LRU + importance-based pruning
└── Storage Backend   ← SQLite (persistent) + in-memory cache (hot)
```

## 3.3 Storage: Zero VRAM

| Tier | Medium | Capacity | Latency | Volatile |
|------|--------|----------|---------|----------|
| L1 — Hot Cache | Go map (system RAM) | 1000 entries | < 1µs | Session-scoped |
| L2 — Warm Cache | SQLite in-memory | 100K entries | < 1ms | Session-scoped |
| L3 — Persistent | SQLite file (`~/.gumi/memory.db`) | Unlimited | < 5ms | Permanent |

**No GPU VRAM is touched at any tier.**

---

# 4. Memory Types

## 4.1 Facts (Key-Value)

Simple, structured pieces of knowledge extracted from the agent's interaction.

```json
{
  "id": "fact_abc123",
  "key": "project_language",
  "value": "Go",
  "source": "inferred_from_files",
  "confidence": 0.95,
  "created_at": "2026-07-13T10:00:00Z",
  "accessed_count": 12,
  "session_id": "session_001"
}
```

| Fact Key | Value Example | Source |
|----------|--------------|--------|
| `project_language` | `"Go"` | File extension scan |
| `build_system` | `"Makefile"` | File detection |
| `test_framework` | `"pytest"` | Import/require detection |
| `project_purpose` | `"CLI tool for data processing"` | README analysis |
| `user_preference_indentation` | `"tabs"` | Observed in files |
| `last_session_summary` | `"Was implementing auth module"` | Previous session |
| `common_error_pattern` | `"nil pointer in handler.go:42"` | Error trace analysis |
| `model_x_good_at` | `"complex refactors"` | Observed performance |

## 4.2 Episodes (Compressed Steps)

Compressed summaries of agent steps and their outcomes. These are how the agent
"remembers" what it tried and what happened.

```json
{
  "id": "episode_456",
  "session_id": "session_001",
  "step": 3,
  "task": "Implement user authentication handler",
  "difficulty": 3,
  "model_used": "ornith-1.0-9b-q4-km",
  "outcome": "success",
  "retries": 0,
  "latency_ms": 2340,
  "tokens_used": 4500,
  "key_facts_extracted": [
    "auth_approach: jwt",
    "auth_file: internal/auth/handler.go"
  ],
  "errors_encountered": [],
  "compressed_summary": "Implemented JWT auth handler with login/logout endpoints. Tests pass."
}
```

**Episode compression ratio:** A 10-turn conversation (15K tokens) compresses to
~500 tokens of episode summaries — a **30× compression**.

## 4.3 Model Fit (Router Feed)

This is the feedback loop to the Agentic Coding Router. It tracks which models
perform well for which types of coding tasks.

```json
{
  "model_id": "ornith-1.0-9b-q4-km",
  "difficulty_levels": {
    "2": {"attempts": 15, "successes": 14, "avg_latency_ms": 180, "avg_retries": 0.1},
    "3": {"attempts": 42, "successes": 38, "avg_latency_ms": 350, "avg_retries": 0.4},
    "4": {"attempts": 8, "successes": 3, "avg_latency_ms": 1200, "avg_retries": 2.1}
  },
  "task_types": {
    "fix": {"attempts": 25, "successes": 22, "repair_rate": 0.08},
    "feature": {"attempts": 30, "successes": 25, "repair_rate": 0.12},
    "refactor": {"attempts": 10, "successes": 7, "repair_rate": 0.20}
  },
  "last_updated": "2026-07-13T10:30:00Z"
}
```

This data feeds directly into the router's model selection algorithm:

```
Model Fit says: Ornith 9B has 38/42 successes at difficulty 3 → strong confidence
Model Fit says: Ornith 9B has 3/8 successes at difficulty 4 → weak, escalate
```

## 4.4 Memory Types Summary

| Type | Storage | Persistence | Purpose |
|------|---------|-------------|---------|
| Facts | SQLite + cache | Cross-session | Project knowledge, preferences |
| Episodes | SQLite + cache | Session (auto-summarized) | What happened, what worked |
| Model Fit | SQLite | Cross-session | Router feedback, performance history |
| Hot Cache | Go map | Session | Fast access to recent facts |

---

# 5. Memory Injection

The Memory Engine injects relevant facts into the model's context as a system
message prefix.

## 5.1 Injection Format

```text
[Memory: Session Context]
Project: Go CLI tool for data processing
Build system: Makefile
Test framework: pytest

[Memory: This Session]
Step 1: Fixed nil pointer in handler.go ✓
Step 2: Implemented JWT auth (3 retries) ⚠
Step 3: In progress — writing integration tests

[Memory: Model Fit - Current Router Decision]
Selected: ornith-1.0-9b (difficulty 3, task: test)
Reason: 38/42 success rate for test tasks at difficulty 3
```

## 5.2 Token Budget

Memory injection is allocated a fixed token budget (configurable, default 600
tokens — already reserved in the Context Engine as `ReservedMemoryTokens`).

```go
// From context/engine.go:
const defaultReservedMemory = 1200  // ← already reserved, can be used
```

The Memory Engine selects the most relevant facts to fit this budget:

1. **Highest priority:** Model Fit info for current routing decision
2. **Medium priority:** This session's episode summaries (recent first)
3. **Lower priority:** Cross-session facts (project knowledge)
4. **Lowest priority:** Older episodes from previous sessions

## 5.3 Relevance Scoring

Facts are scored by `relevance × recency × confidence`:

```python
def score_fact(fact, current_request, agent_state):
    relevance = 0
    # Boost facts related to current task
    if fact.key in current_request.text:
        relevance += 0.5
    if fact.session_id == agent_state.session_id:
        relevance += 0.3  # same-session boost
    # Boost recently accessed facts
    recency = 1.0 / (1.0 + hours_since(fact.accessed_at))
    # Boost high-confidence facts
    confidence = fact.confidence
    return relevance * recency * confidence
```

---

# 6. Integration Points

## 6.1 Pipeline Engine

The Memory Engine is called at two points in the pipeline:

**Pre-generation (inject memory):**

```go
// In pipeline/engine.go — after resolveProviderAndProfile, before buildPrompt
func (e *Engine) prepareMemory(pc *Context) {
    if e.memory == nil || !e.cfg.Memory.Enabled {
        return
    }
    // Store the current step as an episode
    e.memory.StoreEpisode(pc)

    // Retrieve relevant facts for this request
    facts := e.memory.RetrieveRelevant(pc.IncomingRequest, pc)

    // Inject into context as a system message
    if len(facts) > 0 {
        memoryBlock := e.memory.FormatInjection(facts, pc)
        pc.InjectedMemory = memoryBlock
        pc.AddEvent("memory", "memory_injected", SeverityInfo,
            "injected memory into context",
            map[string]string{
                "fact_count": fmt.Sprintf("%d", len(facts)),
                "tokens":     fmt.Sprintf("%d", estimateTokens(memoryBlock)),
            },
        )
    }
}
```

**Post-generation (extract facts):**

```go
// In pipeline/engine.go — after callProviderGenerate, before agentPostProcess
func (e *Engine) extractMemory(pc *Context) {
    if e.memory == nil || !e.cfg.Memory.Enabled {
        return
    }
    if pc.ProviderResponse == nil {
        return
    }

    // Extract facts from the successful response
    facts := e.memory.ExtractFacts(pc.IncomingRequest, pc.ProviderResponse)

    // Update model fit with outcome
    e.memory.RecordOutcome(pc.SelectedModel, pc.CodingRoute, pc)

    // Store extracted facts
    for _, fact := range facts {
        e.memory.StoreFact(fact)
    }

    pc.AddEvent("memory", "facts_extracted", SeverityInfo,
        "extracted facts from response",
        map[string]string{
            "fact_count": fmt.Sprintf("%d", len(facts)),
        },
    )
}
```

## 6.2 Context Extensions

```go
// New field on Pipeline Context
type Context struct {
    // ...existing fields...

    // Memory engine state
    InjectedMemory string          `json:"injected_memory,omitempty"`
    MemoryFacts    []MemoryFact    `json:"memory_facts,omitempty"`
    MemoryEpisodes []MemoryEpisode `json:"memory_episodes,omitempty"`
}
```

## 6.3 Gateway / API

The memory engine exposes endpoints for the dashboard, CLI, and coding agents:

```text
GET  /v1/gumi/memory/facts       → list/search stored facts
POST /v1/gumi/memory/facts       → store a new fact (agent-driven)
GET  /v1/gumi/memory/model-fit    → model performance data
POST /v1/gumi/memory/clear        → clear memory (user-requested reset)
GET  /v1/gumi/memory/status       → memory engine status
```

The `POST /v1/gumi/memory/facts` endpoint allows coding agents to write facts
directly (see Open Question #2, now resolved).

---

# 7. Fact Extraction Engine

The Memory Engine extracts structured facts from unstructured agent interactions
using structural patterns — no model inference needed.

## 7.1 Extraction Patterns (V1)

| Pattern | Extracts | Example |
|---------|----------|---------|
| `File: path/to/file.ext` | Project structure facts | `src/main.go exists` |
| `Error: ...` | Error pattern facts | `nil pointer in handler.go:42` |
| `Test: test_name` | Test existence facts | `TestAuthenticate exists` |
| `func Name` | Function facts | `func LoginHandler exists` |
| `import "pkg"` | Dependency facts | `import "github.com/gin-gonic/gin"` |

## 7.2 Confidence Scoring

| Signal | Confidence Boost |
|--------|-----------------|
| Fact observed 3+ times | +0.2 |
| Fact verified by test pass | +0.3 |
| Fact observed in current session | +0.1 |
| Fact contradicts known fact | -0.4 |
| Fact from model output (not user) | -0.1 |

## 7.3 Deduplication

When a new fact matches an existing fact's key, the engine updates the existing
fact rather than creating a duplicate:

- If confidence increases → update value, increase confidence
- If confidence decreases → keep original, log discrepancy
- If key is identical but value differs → keep both as "alternatives" with
  separate confidence scores

---

# 8. Configuration

## 8.1 Global Config (`gumi.yaml`)

```yaml
memory:
  enabled: false           # Opt-in in V1
  engine: sqlite           # sqlite | in_memory (for testing)

  # Memory limits
  max_facts: 10000         # Max stored facts before eviction
  max_episodes_per_session: 500
  model_fit_retention_days: 90

  # Injection
  injection_budget_tokens: 1200  # Tokens reserved for memory context
  min_confidence: 0.3            # Minimum confidence to inject a fact
  max_injected_facts: 20         # Maximum facts per injection

  # Extraction
  extract_enabled: true
  min_observation_count: 2       # Require 2+ observations to store a fact

  # Model fit (router feedback)
  track_model_fit: true          # Record model performance per task type
  model_fit_decay: 0.95          # Exponential decay for older observations
```

## 8.2 Per-Request Override

```json
{
  "model": "auto",
  "messages": [...],
  "gumi": {
    "memory": {
      "enable_injection": true,
      "max_injected_facts": 10,
      "reset_session": false
    }
  }
}
```

---

# 9. Telemetry & Observability

Memory events are recorded in the existing pipeline events table:

| Event | When | Metadata |
|-------|------|----------|
| `memory_injected` | Facts added to context | fact_count, tokens_used |
| `memory_facts_extracted` | Facts extracted from response | fact_count |
| `memory_model_fit_updated` | Model performance recorded | model, difficulty, outcome |
| `memory_eviction` | Fact evicted due to capacity | fact_key, reason |

The dashboard gains a **Memory** tab showing:

- Current session memory (what the model "remembers")
- Stored facts (project knowledge)
- Model fit data (performance per model per task type)
- Injection history (what was injected when)

---

# 10. Connection to Agentic Coding Router

The Memory Engine and Router form a **feedback loop**:

```
Router selects model based on difficulty
    ↓
Model processes step
    ↓
Memory Engine records outcome (success/fail, latency, retries)
    ↓
Model Fit store updated
    ↓
Router uses updated Model Fit for next selection
    ↓
(loop)
```

This means the system improves over time:

| After N sessions | Router Behavior |
|-----------------|-----------------|
| 0 | Rules-only — uses profile coding_strength |
| 10 | Starts preferring models with better observed success rates |
| 100 | Knows which models handle which task types best |
| 1000 | Can predict the optimal model for a task with high confidence |

The Memory Engine also helps the router detect **model unsuitability**:

- If a model has 3 consecutive failures at a difficulty level → router escalates
  preemptively next time
- If a model consistently handles a task type well → router prefers it even if
  another model has better general coding_strength

---

# 11. Implementation Plan

## Phase 1 — Fact Store + Injection (Shipped — Sprint 12) ✅

1. ✅ Create `runtime/internal/memory/` package
2. ✅ Implement `FactStore` (SQLite-backed key-value)
3. ✅ Implement `EpisodeStore` (session-scoped step history)
4. ✅ Implement `InjectionEngine` (select + format facts for context)
5. ✅ Implement `ModelFitStore` (per-model performance tracking)
6. ✅ Integrate into pipeline (prepareMemory + extractMemory)
7. ✅ Add memory config to schema (top-level `memory:` block)
8. ✅ Add memory API endpoints (GET facts, POST facts, GET model-fit, POST clear, GET status)
9. ✅ Add memory telemetry events

**Files created:**
- `runtime/internal/memory/memory.go` — `MemoryEngine` facade, FactStore, EpisodeStore
- `runtime/internal/memory/schema.go` — SQLite schema for memory tables

**Files modified:**
- `runtime/internal/pipeline/context.go` — add Memory fields
- `runtime/internal/pipeline/engine.go` — add prepareMemory, extractMemory calls
- `runtime/internal/config/config.go` — add top-level `memory` section
- `runtime/internal/gateway/memory.go` — add memory API routes
- `runtime/internal/cli/memory.go` — add `gumi memory` CLI commands

## Phase 2 — Dashboard Integration (Shipped — Sprint 12) ✅

1. ✅ CLI `gumi memory` command (status, facts, clear)
2. ✅ All CLI commands support `--json`
3. Dashboard Memory tab (pending UI work)

## Phase 3 — Adaptive Memory

1. Automatic fact confidence adjustment
2. Cross-session importance scoring
3. Memory compaction (merge related facts)
4. Router feedback loop optimization

---

# 12. Edge Cases & Failure Modes

| Scenario | Behavior |
|----------|----------|
| Memory DB corrupt | Delete and recreate, log warning |
| No disk space | Disable persistence, fall back to in-memory only |
| Too many facts | LRU eviction, oldest/lowest-confidence facts removed first |
| Model consistently fails | Model Fit records failures, router stops recommending |
| Fact injection exceeds budget | Truncate lowest-confidence facts first |
| User clears memory | Drop all rows, reset cache |
| First run (no memory) | Silent no-op — no memory to inject |
| Session resets mid-way | Episodes persist until session expires, then archived |

---

# 13. Relationship to Other Specs

| Spec | Relationship |
|------|-------------|
| [19-agentic-coding-router-specification.md](./19-agentic-coding-router-specification.md) | Memory feeds Model Fit data to router; router triggers memory extraction |
| [07-pipeline-specification.md](./07-pipeline-specification.md) | Memory integrates at pre/post generation hooks |
| [08-context-and-prompt-engine-specification.md](./08-context-and-prompt-engine-specification.md) | Memory injects into ReservedMemoryTokens budget |
| [13-storage-and-telemetry-specification.md](./13-storage-and-telemetry-specification.md) | Memory uses SQLite (separate DB or same) |
| [12-cli-and-dashboard-specification.md](./12-cli-and-dashboard-specification.md) | Dashboard gains Memory tab, CLI gains memory command |
| [14-implementation-roadmap.md](./14-implementation-roadmap.md) | Phased implementation plan |

---

# 14. Open Questions

1. **Separate DB or same SQLite file as telemetry?** Separate is cleaner —
   memory data has different retention/backup requirements. But same file is
   simpler for users. Decision: separate `~/.gumi/memory.db` for V1.

2. **Should coding agents (Cline, Continue) be able to write to memory via
   API?** ✅ **Resolved** — Yes. `POST /v1/gumi/memory/facts` is now shipped,
   allowing agents to store facts directly.

3. **How to handle conflicting facts from different sessions?** Use confidence
   scoring. If session A says "language: Python" (confidence 0.9) and session B
   says "language: Rust" (confidence 0.6), keep Python with higher confidence.
   Log the conflict.

4. **Should memory injection be visible to the user in the response?** By
   default, no — it's a system message. But a `gumi.debug: true` flag could
   echo the injected memory in the response for debugging.
