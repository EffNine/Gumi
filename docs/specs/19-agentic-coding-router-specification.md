# Agentic Coding Router Specification

Version: 1.0  
Status: Implemented (Phase 1)  
Scope: Intelligent coding-agent routing for Gumi Runtime

---

## Implementation Status

| Phase | Status | Description |
|-------|--------|-------------|
| **Phase 1** | ✅ **Complete** | Structural classifier + rule engine + model registry + telemetry + pipeline integration |
| **Phase 2** | ✅ **Shipped (Sprint 13)** | Agent-state awareness — escalation thresholds for retries, steps, and repetitions |
| **Phase 3** | ✅ **Complete** | Observation & self-tuning — rule/strategy adjustments, model boost/demote, epsilon-greedy exploration |

**Phase 1 files created:**
- `runtime/internal/router/classifier.go` — Structural `CodingTaskClassifier` (no AI inference)
- `runtime/internal/router/registry.go` — `CodingModelRegistry` indexes models by coding capability from YAML profiles
- `runtime/internal/router/engine.go` — `CodingRuleEngine` with 11 built-in first-match rules
- `runtime/internal/router/telemetry.go` — Routing decision recording

**Phase 1 integration points:**
- `runtime/internal/pipeline/context.go` — `CodingRoute` + `CodingTaskProfile` fields added
- `runtime/internal/pipeline/engine.go` — Router integrated into `resolveProviderAndProfile()`; routing activates when `routing.enabled: true` + agent mode
- `runtime/internal/config/config.go` — `RoutingConfig` section (opt-in, disabled by default)
- `runtime/internal/api/chat.go` — `RoutingExtensions` for per-request hints

**Phase 1 is opt-in** — disabled by default. Enable with `routing.enabled: true` in config. Routing only activates in agent mode. Falls through to default resolution if routing fails.

---

# 1. Purpose

This document defines the **Agentic Coding Router** for Gumi.

This is a specialized router — not a general-purpose model router. It exists
solely to help **coding agents** (Cline, Continue, Claude Code, OpenHands, etc.)
route code fixes and generation tasks to the right local model based on **coding
difficulty**.

The router answers one question:

> *"For this coding task, which of my local models will give the best
> quality-speed trade-off?"*

It does not route chat, creative writing, translation, or general Q&A. It is
purpose-built for agentic coding workflows.

---

# 2. Research Foundation

## 2.1 Coding Agents Need Model Selection

Coding agents (Cline, Continue, Claude Code, OpenHands) operate in a loop:

```text
User Request → Agent Plan → Tool Calls → Observe → Next Step
```

Each loop iteration is a model inference call. Using the wrong model hurts
either quality (too small) or speed (too large). The right model depends on the
**coding difficulty** of the current step.

Empirical observations from coding-agent users:

| Task | Wrong Model | Consequence |
|------|-------------|-------------|
| Typo fix (trivial) | 9B model | 5× slower than needed, wastes battery/tokens |
| Feature design (complex) | 1B model | Poor output, multiple retries, frustration |
| Multi-file refactor | 3B model | Loses context, produces broken code |
| Single-line bug fix | 9B reasoning model | Over-engineers, adds unnecessary complexity |

## 2.2 Related Work

- **Anthropic's "Building Effective Agents"** — The Routing pattern (route
  simple queries to smaller models) directly inspired this spec
- **Cline's model selection** — Users manually specify a model per-session; no
  automatic routing
- **Continue's model roles** — Allows different models for chat vs. edit vs.
  apply, but not adaptive per-task difficulty
- **OpenHands' AgentRuntime** — Supports model configuration per agent type,
  but not dynamic per-step routing

Gumi's position: **Automatic difficulty-based routing within a single coding
session** — no other local AI tool does this today.

---

# 3. Design Goals

1. **Coding-only focus** — No general-purpose classification. Only classifies
   coding task difficulty.

2. **Per-step routing** — The router can re-evaluate at each agent step, not
   just at session start.

3. **Transparent** — Every routing decision is visible in telemetry and the
   dashboard. The user can see *why* a model was chosen.

4. **Privacy-first** — All classification is structural. No code or prompts
   leave the machine.

5. **Zero overhead goal** — Classification must complete in < 20ms. Routing
   should be unnoticeable.

6. **Graceful fallback** — If routing fails or no suitable model is found,
   fall back to the user's configured default model.

---

# 4. Coding Task Classifier

The classifier examines the agent's current step and produces a **Coding
Difficulty Profile**.

## 4.1 Classification Dimensions

### Difficulty

This is the primary routing signal:

| Level | Label | Examples | Characteristics |
|-------|-------|----------|-----------------|
| 1 | `trivial` | Typo fix, rename variable, formatting | Single line, no logic change |
| 2 | `simple` | Add parameter, fix import, add comment | One function, obvious change |
| 3 | `moderate` | Implement function, add error handling, write test | Single file, new logic |
| 4 | `complex` | Refactor module, implement feature, fix bug across files | Multiple files, subtle logic |
| 5 | `novel` | New algorithm, architecture design, unfamiliar domain | Requires planning, high uncertainty |

### Task Type (within coding)

| Type | Description |
|------|-------------|
| `fix` | Bug fix, error correction, test failure repair |
| `refactor` | Restructure without changing behavior |
| `feature` | New functionality, implement from spec |
| `test` | Write or modify tests |
| `review` | Code review, explain code |
| `docs` | Documentation, comments |
| `search` | Find code, grep, navigate codebase |
| `plan` | Design, architecture, approach |

### Signals Used by the Classifier

The classifier uses **only structural signals** — no model inference:

| Signal | Source | Difficulty Indicator |
|--------|--------|---------------------|
| **Message length** | `len(messages[-1].content)` | Short → trivial/simple. Long → complex/novel |
| **File count mentioned** | Parse for file paths (`path/to/file`) | 1 file → simple. 3+ files → complex |
| **Error trace present** | Contains stack trace or error message | Usually indicates `fix` task |
| **Tool call count** | Agent tool call history | Many retries → complex or stuck |
| **Step number** | `pc.StepCount` | Early → simpler. Late → complex (stuck) |
| **Retry count** | `pc.Retry.Attempt` | High retries → task may be too hard for current model |
| **Keywords** | Request contains words like "refactor", "design", "architecture", "implement" | Indicates task type and complexity |
| **Code block size** | ``` blocks in the request | Large blocks → complex change |
| **Search/grep presence** | Request includes search commands | Usually simple (finding code) |

## 4.2 Classification Algorithm (V1)

```python
def classify_coding_task(request, agent_state):
    """
    Returns a CodingDifficultyProfile.
    No model inference — purely structural heuristics.
    """
    text = request.messages[-1].content
    has_traceback = contains_stack_trace(text)
    file_count = count_file_paths(text)
    step = agent_state.step_count
    retries = agent_state.retry.attempt
    has_tools = len(request.tools) > 0
    code_block_size = max_code_block_size(text)
    keywords = extract_coding_keywords(text)

    # Determine task type
    if has_traceback or "error" in text.lower() or "bug" in text.lower():
        task_type = "fix"
    elif any(k in text.lower() for k in ["refactor", "restructure", "rename", "extract"]):
        task_type = "refactor"
    elif any(k in text.lower() for k in ["implement", "create", "add", "write", "build"]):
        task_type = "feature"
    elif any(k in text.lower() for k in ["test", "assert", "mock"]):
        task_type = "test"
    elif any(k in text.lower() for k in ["review", "explain", "what does"]):
        task_type = "review"
    elif any(k in text.lower() for k in ["search", "find", "where is", "grep"]):
        task_type = "search"
    elif any(k in text.lower() for k in ["design", "architecture", "plan", "approach"]):
        task_type = "plan"
    else:
        task_type = "feature"  # default for coding tasks

    # Determine difficulty
    # Baseline from text length
    text_len = len(text)

    if text_len < 50 and file_count <= 1 and not has_traceback:
        difficulty = 1  # trivial
    elif text_len < 200 and file_count <= 1 and step <= 1:
        difficulty = 2  # simple
    elif file_count <= 2 and step <= 3 and retries <= 1:
        difficulty = 3  # moderate
    elif file_count >= 3 or step > 5 or retries > 2:
        difficulty = 4  # complex
    else:
        difficulty = 3  # moderate default

    # Escalate if keywords suggest complexity
    if task_type in ("plan", "review") and file_count >= 2:
        difficulty = max(difficulty, 4)
    if code_block_size > 100:  # large code changes
        difficulty = max(difficulty, 4)
    if has_traceback and file_count >= 2:
        difficulty = max(difficulty, 4)

    # Downgrade if it's clearly simple
    if task_type == "search":
        difficulty = 1
    if task_type == "fix" and file_count == 1 and text_len < 100:
        difficulty = min(difficulty, 2)

    return CodingDifficultyProfile(
        difficulty=difficulty,
        task_type=task_type,
        file_count=file_count,
        has_traceback=has_traceback,
        step=step,
        retries=retries,
    )
```

## 4.3 Output: CodingDifficultyProfile

```json
{
  "difficulty": 3,
  "difficulty_label": "moderate",
  "task_type": "fix",
  "file_count": 1,
  "has_traceback": true,
  "step": 2,
  "retries": 0,
  "classification_latency_ms": 3
}
```

---

# 5. Model Registry (Coding-Focused)

The Model Registry maintains a view of available models with their **coding
capabilities**, not general capabilities.

## 5.1 Source: Model Profiles

Each model profile (`profiles/*.yaml`) already declares coding capability:

```yaml
# Example from ornith-1.0-9b-q4-km.yaml
capabilities:
  coding: strong        # ← primary routing signal
  tool_calling: weak    # ← important for agent routing
  reasoning: strong     # ← useful for complex bugs
  context_limit: 32768  # ← critical for multi-file tasks
```

## 5.2 Registry Entry

```json
{
  "id": "ornith-1.0-9b-q4-km",
  "provider": "ollama",
  "coding_strength": "strong",
  "tool_calling": "weak",
  "reasoning": "strong",
  "context_limit": 32768,
  "size_category": "medium",     # tiny < 3B, small < 7B, medium < 15B, large >= 15B
  "observed_latency_p50_ms": 180,
  "observed_repair_rate": 0.03,  # how often Gumi must repair its output
  "observed_success_rate": 0.97  # how often validation passes
}
```

## 5.3 Coding Difficulty → Model Fit

| Difficulty | Recommended Coding Strength | Recommended Size | Example Models |
|------------|---------------------------|------------------|----------------|
| 1 — trivial | any | tiny (< 3B) | Gemma 3 1B, Qwen 2.5 Coder 1.5B |
| 2 — simple | weak+ | small (3-7B) | Qwen 2.5 Coder 7B, Llama 3.2 3B |
| 3 — moderate | medium+ | medium (7-9B) | Ornith 9B, DeepSeek Coder 6.7B |
| 4 — complex | strong | medium-large (7B+) | Ornith 9B, Qwen 3 8B, DeepSeek R1 8B |
| 5 — novel | strong | large (8B+) | DeepSeek R1 8B, Qwen 3 8B, Llama 3.1 8B |

---

# 6. Routing Rules (Coding-Specific)

Rules are evaluated **in order**, first-match wins.

## 6.1 Built-in Default Rules

```yaml
routing:
  enabled: true

  coding_rules:
    # Trivial fix → fastest model, any coding strength works
    - name: trivial-fix
      when:
        difficulty: 1
      route:
        prefer: fastest
        min_coding: weak
        max_size: small

    # Simple fix → small model, no reasoning needed
    - name: simple-fix
      when:
        difficulty: 2
        task_type: fix
      route:
        prefer: fastest
        min_coding: weak
        min_context: 4096

    # Test writing → moderate coding, decent context
    - name: write-test
      when:
        difficulty: [2, 3]
        task_type: test
      route:
        prefer: best_coding
        min_coding: medium
        min_context: 8192

    # Moderate feature → good coding model
    - name: moderate-feature
      when:
        difficulty: 3
        task_type: feature
      route:
        prefer: best_coding
        min_coding: medium
        min_context: 8192

    # Complex fix with traceback → coding + reasoning
    - name: complex-fix-with-trace
      when:
        difficulty: [4, 5]
        has_traceback: true
      route:
        prefer: best_combo
        min_coding: strong
        min_reasoning: medium
        min_context: 16384

    # Complex feature/refactor → strongest coding model
    - name: complex-coding
      when:
        difficulty: [4, 5]
        task_type: [feature, refactor, plan]
      route:
        prefer: best_coding
        min_coding: strong
        min_context: 16384

    # Review → mild model, just needs to read
    - name: code-review
      when:
        task_type: review
      route:
        prefer: fastest
        min_coding: weak
        min_context: 8192

    # Default fallback
    - name: default
      route:
        provider: ${default_provider}
        model: ${default_model}
```

## 6.2 Preference Strategies

| Strategy | Behavior |
|----------|----------|
| `fastest` | Model with lowest observed latency meeting all minimums |
| `best_coding` | Model with highest `coding: strong/medium` rating |
| `best_combo` | Weighted score: coding×0.5 + reasoning×0.3 + tool_calling×0.2 |
| `largest_context` | Model with largest context window |
| `explicit` | Exact provider+model (no selection) |

## 6.3 Agent-State-Aware Routing

The router considers the agent's loop state to detect when re-routing is needed:

| Agent Signal | Routing Action |
|-------------|----------------|
| `StepCount > 5` and `RetryCount > 2` | Escalate difficulty → route to stronger model |
| `ToolCallHistory` shows repeating pattern | Possible loop → route to different model (change of "mind") |
| `ContextCompactionCount > 0` | Context is strained → prefer model with larger context |
| Step is a `read`/`search` command | Downgrade difficulty → route to faster model |

This is what makes it an **agentic coding router** rather than a static router —
it responds to the agent's runtime state.

---

# 7. Integration with Gumi Agent Mode

## 7.1 Entry Point

The router integrates at the existing `resolveProviderAndProfile` step, which
is called once per agent loop iteration.

```go
// engine.go — resolveProviderAndProfile (actual implementation)
func (e *Engine) resolveProviderAndProfile(ctx context.Context, pc *Context) Result {
    // Agentic Coding Router: when enabled + agent mode, classify task
    // and select a model based on routing rules.
    if e.cfg.Routing.Enabled && pc.RuntimeMode == ModeAgent && e.codingRouter != nil {
        // Gather request hints if provided.
        var routingHints *api.RoutingExtensions
        if pc.IncomingRequest.Gumi != nil {
            routingHints = pc.IncomingRequest.Gumi.Routing
        }

        // Classify the coding task using structural heuristics.
        codingProfile := e.codingClassifier.Classify(
            pc.NormalizedRequest.Messages,
            pc.StepCount,
            pc.Retry.Attempt,
        )

        // Build available model set and run rule engine.
        availableModels := e.buildAvailableModelSet()
        result := e.codingRouter.Route(codingProfile, availableModels, routingHints)

        if result != nil {
            pc.SelectedProvider = result.Provider
            pc.SelectedModel = result.Model
            pc.CodingRoute = &CodingRoute{
                Profile: &CodingTaskProfile{...},
                SelectedModel:   result.Provider + "/" + result.Model,
                Preference:      string(result.Strategy),
                Reason:          result.Reason,
                EvaluationCount: pc.StepCount,
            }

            // Resolve model profile for the routed model.
            match := e.profileResolver.Resolve(result.Provider, result.Model)
            pc.ModelProfile = match.Profile

            pc.AddEvent("routing", "coding_route_selected", SeverityInfo,
                "agentic coding routing selected model", map[string]string{
                    "difficulty":     fmt.Sprintf("%d", codingProfile.Difficulty),
                    "difficulty_label": codingProfile.DifficultyLabel,
                    "task_type":      string(codingProfile.TaskType),
                    "provider":       result.Provider,
                    "model":          result.Model,
                    "matched_rule":   result.MatchedRule,
                    "strategy":       string(result.Strategy),
                    "fallback_used":  fmt.Sprintf("%t", result.FallbackUsed),
                })

            return Result{}
        }

        // Router returned nil — fall through to default resolution.
        pc.AddEvent("routing", "coding_route_fallback", SeverityWarning,
            "router returned nil; falling back to default resolution", nil)
    }

    // Default provider+profile resolution (unchanged for non-routed requests).
    // ...
}
```

## 7.2 Context Extensions

Two new types are added to `Pipeline Context`:

```go
// CodingTaskProfile captures the result of the structural classifier.
type CodingTaskProfile struct {
    Difficulty     int       `json:"difficulty"`
    DifficultyLabel string   `json:"difficulty_label"` // "trivial"|"simple"|"moderate"|"complex"|"novel"
    TaskType       string    `json:"task_type"`         // "fix"|"refactor"|"feature"|"test"|"review"|"docs"|"search"|"plan"
    FileCount      int       `json:"file_count"`
    HasTraceback   bool      `json:"has_traceback"`
    StepCount      int       `json:"step_count"`
}

// CodingRoute records the routing decision for this step.
type CodingRoute struct {
    Profile        *CodingTaskProfile `json:"profile,omitempty"`
    SelectedModel  string             `json:"selected_model"`  // "provider/model"
    Preference     string             `json:"preference"`      // routing strategy used
    Reason         string             `json:"reason"`          // human-readable explanation
    EvaluationCount int               `json:"evaluation_count"`// agent step count at routing time
}

// In Context struct:
CodingRoute *CodingRoute `json:"coding_route,omitempty"`
```

## 7.3 Re-Routing Within a Session

The router can change its mind mid-session:

- **Step 1**: "fix typo in greeting.go" → difficulty 1 → routes to Gemma 3 1B
- **Step 3**: "now implement the payment handler across 3 files" → difficulty 4 → routes to Ornith 9B
- **Step 7**: stuck in a loop, 4 retries → escalates to DeepSeek R1 8B

Each step independently classifies the current request. The router has no
memory of previous steps except what's in the agent state (`StepCount`,
`ToolCallHistory`, `Retry`).

---

# 8. Configuration

## 8.1 Global Config (`gumi.yaml`)

```yaml
routing:
  enabled: false             # Opt-in in V1
  mode: agentic_coding       # The only mode in V1

  # Classifier tuning
  classifier:
    method: structural       # structural only in V1
    escalation_threshold:    # When to bump difficulty
      retries: 3
      steps: 6
      repetitions: 3         # Same tool call repeated

  # Override default rules
  coding_rules:
    - name: custom-trivial
      when:
        difficulty: 1
      route:
        provider: ollama
        model: gemma3:1b
```

## 8.2 Per-Request Override

Coding agents can influence routing via the `Gumi` extension field:

```json
{
  "model": "auto",
  "messages": [...],
  "gumi": {
    "routing": {
      "hint_difficulty": 4,          // Override classifier
      "hint_task_type": "refactor",  // Override classifier
      "preferred_providers": ["ollama"],
      "min_context": 32768
    }
  }
}
```

---

# 9. Telemetry & Observability

Every routing decision is recorded:

```json
{
  "request_id": "req_abc123",
  "coding_route": {
    "request": {
      "text_length": 342,
      "file_count": 2,
      "has_traceback": true
    },
    "profile": {
      "difficulty": 4,
      "task_type": "fix",
      "step": 3,
      "retries": 1
    },
    "decision": {
      "matched_rule": "complex-fix-with-trace",
      "selected_provider": "ollama",
      "selected_model": "deepseek-r1-8b",
      "strategy": "best_combo",
      "alternatives": [
        {"provider": "ollama", "model": "ornith-9b", "rejected": "no_reasoning"},
        {"provider": "lmstudio", "model": "qwen3-8b", "rejected": "slower_latency"}
      ]
    },
    "classification_latency_ms": 4,
    "fallback_used": false
  }
}
```

The dashboard will display:

- **Current route** — which model is handling the current step
- **Difficulty timeline** — how difficulty changed across agent steps
- **Re-routing events** — when and why the router switched models
- **Model utilization** — how many requests each model handled

---

# 10. Implementation Plan

## Phase 1 — Structural Classifier + Rules (V1) ✅

1. ✅ Implement `CodingTaskClassifier` (structural heuristics only)
2. ✅ Implement `CodingModelRegistry` (from profiles)
3. ✅ Implement `CodingRuleEngine` (first-match, difficulty-based)
4. ✅ Integrate into agent mode at `resolveProviderAndProfile`
5. ✅ Add `CodingRoute` + `CodingTaskProfile` to Pipeline Context
6. ✅ Record routing telemetry
7. ✅ Add `routing` config section to schema

**Files created:**
- `runtime/internal/router/classifier.go` — CodingTaskClassifier
- `runtime/internal/router/registry.go` — CodingModelRegistry
- `runtime/internal/router/engine.go` — CodingRuleEngine
- `runtime/internal/router/telemetry.go` — routing telemetry

**Files modified:**
- `runtime/internal/pipeline/context.go` — add CodingRoute + CodingTaskProfile fields
- `runtime/internal/pipeline/engine.go` — modify resolveProviderAndProfile; add buildAvailableModelSet helper
- `runtime/internal/config/config.go` — add RoutingConfig section
- `runtime/internal/api/chat.go` — add RoutingExtensions struct

## Phase 2 — Agent-State Awareness (Shipped — Sprint 13) ✅

1. ✅ Use `StepCount`, `Retry`, `ToolCallHistory` for re-routing
2. ✅ Implement escalation thresholds (`retries`, `steps`, `repetitions`)
3. ✅ Repetitions escalation implemented — detects repeating tool-call patterns
   (same function name + arguments) via `applyEscalation` in
   `runtime/internal/router/classifier.go`. When the same tool call signature
   repeats ≥ `escalation_threshold.repetitions` times (default 3), difficulty
   escalates to `complex`.
4. ✅ Add reroute events and dashboard display

## Phase 3 — Observation & Self-Tuning (Complete)

Phase 3 closes the loop between observed model performance and routing decisions.

### What is implemented

- **Per-bucket outcome tracking** via the existing `model_fit` table in the memory engine (`RecordOutcome`).
- **Rich fit queries** (`GetFitStats`, `GetModelFit`, `TotalAttempts`) so the router can see all models for a `(difficulty, task_type)` bucket, not just the single best model.
- **Self-tuner** (`runtime/internal/router/selftuner.go`) that runs periodic tuning passes and maintains an in-memory overlay of adjustments:
  - Raises `MinCoding` for a rule when observed success rate falls below `min_success_rate`.
  - Bumps `MinContext` when models show strain signals (high retries / low success).
  - Flips `Prefer` strategy when an alternative would have won more often.
  - Promotes models with success rate ≥ `promote_threshold` and demotes models with success rate ≤ `demote_threshold`.
  - Epsilon-greedy exploration: with probability ε (decaying as observations grow), picks a different eligible candidate to gather data.
- **Config section** `routing.self_tuning` with knobs for attempts thresholds, success thresholds, boost/demote weights, exploration rate, decay, warmup, and tuning interval. Disabled by default (opt-in).
- **Pipeline wiring** in `runtime/internal/pipeline/engine.go`: the self-tuner observes every routing outcome after `RecordOutcome` and triggers `Tune()` every `min_outcomes_between_tunes` outcomes once warmup is satisfied.
- **Telemetry events** emitted on each tuning pass, per-adjustment, and on exploration.
- **Dashboard API** `GET /v1/gumi/self-tuning` returns the current `SelfTuningSnapshot` including rule overrides, boosts/demotes, adjustments, and config.

### Files added / changed

- Added: `runtime/internal/router/selftuner.go`, `runtime/internal/router/selftuner_test.go`
- Modified: `runtime/internal/router/engine.go`, `runtime/internal/memory/memory.go`, `runtime/internal/config/config.go`, `runtime/internal/pipeline/engine.go`, `runtime/internal/gateway/routes.go`, `runtime/internal/gateway/memory.go`

---

# 11. Edge Cases & Failure Modes

| Scenario | Behavior |
|----------|----------|
| Only one model available | Router selects that model, logs "no alternative" |
| No model meets `min_coding` | Log warning, use most capable available |
| Classifier uncertain (ambiguous request) | Default to difficulty 3 (moderate) |
| Routing disabled | Passthrough — no change from current behavior |
| Explicit model name in request | Bypass routing entirely |
| All models offline | Fall back to default, emit critical telemetry |
| Agent loop detected | Escalate to strongest model (different model = different behavior) |

---

# 12. Relationship to Existing Gumi Features

| Feature | Relationship |
|---------|-------------|
| **Model Profiles** | Profiles declare `coding: strong/medium/weak` — the router reads this |
| **Agent Mode** | Router integrates at `resolveProviderAndProfile`, called each agent step |
| **Guard Engine** | Guard detects loops → router can escalate to stronger model |
| **Validation Engine** | Repair rate feeds into model scoring |
| **Telemetry** | Every routing decision is recorded |
| **Dashboard** | Shows current route, difficulty timeline, re-routing events |
| **Thinking Policy** | Router can disable thinking for simple tasks, enable for complex |

---

# 13. References

- Anthropic. "Building Effective Agents." Dec 2024.
  https://www.anthropic.com/engineering/building-effective-agents
- Gumi Agent Mode. `runtime/internal/pipeline/engine.go`
- Gumi Model Profiles. `runtime/internal/profiles/profile.go`
- Gumi Pipeline Context. `runtime/internal/pipeline/context.go`
