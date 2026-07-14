# Mega Prompt — Review Fixes 2026-07-13

> **One-shot implementation prompt.** Feed this to an coding agent (or run it
> yourself) to apply every fix from the two-axis review of commits
> `fcd61e3..HEAD`. Work packages WP1→WP7 must be executed in order; WP3 may run
> in parallel with WP1. After each WP, run `go build ./...` and `go test ./...`
> before moving on.

---

## Context

You are working in the **Gumi** repo (`/Users/afnanrudy/Github-Projects/Gumi`),
a Go runtime (`runtime/` module, `go.mod` at `runtime/go.mod`) that proxies LLM
providers with a pipeline, router, memory engine, guard, and gateway. A review
of today's commits found 9 Standards violations and 15 Spec gaps. The full plan
with acceptance criteria is in `docs/specs/21-review-fix-plan.md` — read it
first, then execute the work packages below.

**Ground rules**:
- Go 1.25 module path `github.com/gumi/gumi/runtime`.
- Follow the existing code style: `internal/` packages, lowercase exports,
  `fmt.Errorf("...: %w", err)` wrapping, context-first signatures.
- Do NOT introduce new dependencies. Use only what's in `go.mod`.
- Every new exported function needs a doc comment.
- Run `gofmt -w` and `go vet ./...` before committing.
- One commit per work package, message format:
  `fix(review): WP<n> <short title>`.
- Read the relevant spec section before each WP:
  - Spec 19: `docs/specs/19-agentic-coding-router-specification.md`
  - Spec 20: `docs/specs/20-memory-engine-specification.md`
  - Spec 04: `docs/specs/04-api-specification.md`
  - Spec 05: `docs/specs/05-configuration-specification.md`

---

## WP1 — Config wiring

**Goal**: every field in `RoutingConfig` and `MemoryConfig` is read by an
engine code path; every field in `RoutingExtensions` / `MemoryExtension`
(`runtime/internal/api/chat.go:39-57`) is honoured when present in a request.

### 1.1 Expand `RoutingConfig`

File: `runtime/internal/config/config.go`

Replace:
```go
type RoutingConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Mode    string `json:"mode" yaml:"mode"`
}
```
With:
```go
type RoutingConfig struct {
	Enabled    bool                 `json:"enabled" yaml:"enabled"`
	Mode       string               `json:"mode" yaml:"mode"`
	Classifier ClassifierConfig     `json:"classifier,omitempty" yaml:"classifier,omitempty"`
	CodingRules []CodingRuleOverride `json:"coding_rules,omitempty" yaml:"coding_rules,omitempty"`
}

type ClassifierConfig struct {
	EscalationThreshold EscalationThreshold `json:"escalation_threshold,omitempty" yaml:"escalation_threshold,omitempty"`
}

type EscalationThreshold struct {
	Retries      int `json:"retries,omitempty" yaml:"retries,omitempty"`
	Steps        int `json:"steps,omitempty" yaml:"steps,omitempty"`
	Repetitions  int `json:"repetitions,omitempty" yaml:"repetitions,omitempty"`
}

type CodingRuleOverride struct {
	Name        string `json:"name" yaml:"name"`
	Prefer      string `json:"prefer,omitempty" yaml:"prefer,omitempty"`
	MinCoding   string `json:"min_coding,omitempty" yaml:"min_coding,omitempty"`
	MinContext  int    `json:"min_context,omitempty" yaml:"min_context,omitempty"`
	MinReasoning string `json:"min_reasoning,omitempty" yaml:"min_reasoning,omitempty"`
	MaxSize     string `json:"max_size,omitempty" yaml:"max_size,omitempty"`
}
```

Add defaults in the existing `applyDefaults` (or wherever `Config` defaults are
set): if `Routing.Enabled` and `Classifier.EscalationThreshold` is zero-valued,
set `{Retries: 3, Steps: 6, Repetitions: 3}` (matching
`classifier.go` current defaults).

### 1.2 Thread config into the classifier

File: `runtime/internal/router/classifier.go`

- `NewCodingTaskClassifier` — accept `ClassifierConfig` (or the full
  `EscalationThreshold`). Replace the hardcoded `EscalationRetries: 3`,
  `EscalationSteps: 6`, `EscalationRepetitions: 3` with values from config.
- `Classify` signature — add `hints *api.RoutingExtensions` parameter. If
  non-nil:
  - `HintDifficulty > 0` → seed `difficulty = hints.HintDifficulty` before
    refinement.
  - `HintTaskType != ""` → override the classified `TaskType`.
  - `MinContext > 0` → store on the profile so `selectFromRegistry` can use it
    as a floor.
- `applyEscalation` — add a `repetitions int` parameter and a
  `conversation []api.Message` parameter; detect repeating tool-call patterns
  (same function name + args appearing ≥ `EscalationRepetitions` times) and bump
  difficulty by 1. See spec 19 §6.3.

Update the call site in `runtime/internal/pipeline/engine.go`
`resolveProviderAndProfile` to pass `routingHints` into `Classify` (currently
`routingHints` is extracted but only passed to `Route`).

### 1.3 Honour per-request memory overrides

File: `runtime/internal/pipeline/engine.go`, function `prepareMemory` (~line
1217).

At the top of `prepareMemory`, after the nil guard, read
`pc.IncomingRequest.Gumi.Memory`:
```go
var memExt *api.MemoryExtension
if pc.IncomingRequest.Gumi != nil {
	memExt = pc.IncomingRequest.Gumi.Memory
}
if memExt != nil && memExt.EnableInjection != nil && !*memExt.EnableInjection {
	return // injection disabled per-request
}
if memExt != nil && memExt.ResetSession != nil && *memExt.ResetSession {
	if err := e.memoryEngine.ClearSession(pc.SessionID); err != nil {
		e.log.Info("failed to clear session memory", "error", err)
	}
}
maxFacts := e.cfg.Memory.MaxInjectedFacts
if memExt != nil && memExt.MaxInjectedFacts != nil {
	maxFacts = *memExt.MaxInjectedFacts
}
```
Then use `maxFacts` in the `SelectRelevantFacts` call. Add `ClearSession` to
`memory/memory.go` if it doesn't exist (it should delete episodes + session row
for the given `sessionID`, but keep facts).

### 1.4 Thread config into the router rule engine

File: `runtime/internal/router/engine.go`

- `NewCodingRuleEngine` — accept `[]CodingRuleOverride`. If non-empty, merge
  over `DefaultCodingRules()` (overrides replace by `Name`).
- Wire in `pipeline/engine.go` where `NewCodingRuleEngine` is called.

### 1.5 Example config

File: `gumi.example.yaml` — add commented blocks:
```yaml
# routing:
#   enabled: false
#   mode: agent           # agent | always
#   classifier:
#     escalation_threshold:
#       retries: 3
#       steps: 6
#       repetitions: 3
#   coding_rules: []
# memory:
#   enabled: false
#   engine: sqlite
#   db_path: ~/.gumi/memory.db
#   max_facts: 10000
#   max_episodes_per_session: 50
#   model_fit_retention_days: 30
#   injection_budget_tokens: 1200
#   min_confidence: 0.6
#   max_injected_facts: 20
#   extract_enabled: true
#   min_observation_count: 2
#   track_model_fit: true
#   model_fit_decay: 0.95
```

**Verify**: `go build ./...` passes. Write a `config_test.go` case that loads a
YAML with all new fields and asserts they parse.

---

## WP2 — Memory engine correctness

File: `runtime/internal/memory/memory.go`. Read spec 20 §5.2, §5.3, §7.3 first.

### 2.1 Fix `FormatInjection` order (spec 20 §5.2)

Current order: facts → episodes → model fit.
Required order: (1) Model Fit, (2) episodes, (3) facts, (4) older episodes.

Reorder the three blocks in `FormatInjection` so the output is:
```
[Memory]
--- Model Performance ---      ← (1) fitData
--- This Session ---           ← (2) episodeSummary
--- Project Knowledge ---      ← (3) facts
```
Keep the token-budget logic identical; just change the section sequence.

### 2.2 Fix `StoreFact` deduplication (spec 20 §7.3)

Current behaviour: if new confidence > existing → overwrite (old value lost);
if lower → store as `:alt`. Spec says: **if key identical but value differs →
keep both as alternatives.**

Replace the update-vs-alt branch in `StoreFact` with:
```go
// Existing fact with same key.
if fact.Value == existingValue {
	// Same value — just bump confidence if higher.
	if fact.Confidence > existingConfidence {
		_, err = m.db.Exec(`UPDATE facts SET confidence=?, updated_at=? WHERE id=?`,
			fact.Confidence, fact.UpdatedAt, existingID)
	}
	return err
}

// Value differs — keep both. Ensure the higher-confidence one is primary.
if fact.Confidence > existingConfidence {
	// Demote existing to :alt, insert new as primary.
	_, err = m.db.Exec(`UPDATE facts SET key = key || ':alt' WHERE id = ?`, existingID)
	if err != nil {
		return fmt.Errorf("demote existing fact: %w", err)
	}
	_, err = m.db.Exec(
		`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fact.ID, fact.Key, fact.Value, fact.Source, fact.Confidence,
		fact.SessionID, fact.CreatedAt, fact.UpdatedAt, fact.UpdatedAt, 1, fact.TTLSeconds)
} else {
	// New is lower confidence — store as :alt.
	fact.Key = fact.Key + ":alt"
	_, err = m.db.Exec(
		`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fact.ID, fact.Key, fact.Value, fact.Source, fact.Confidence,
		fact.SessionID, fact.CreatedAt, fact.UpdatedAt, fact.UpdatedAt, 1, fact.TTLSeconds)
}
// Log the conflict (WP4 adds the telemetry hook; for now use a package-level
// log or store an :alt event fact).
return err
```
Remove the `fact.Confidence = fact.Confidence * 0.8` penalty line — the spec
doesn't mention penalizing.

### 2.3 Add recency scoring to `SelectRelevantFacts` (spec 20 §5.3)

In the scoring loop, after the confidence and access-frequency boosts, add:
```go
// Recency factor: recency = 1.0 / (1.0 + hours_since(accessed_at))
if f.AccessedAt != "" {
	accessed, perr := time.Parse(time.RFC3339, f.AccessedAt)
	if perr == nil {
		hours := time.Since(accessed).Hours()
		if hours < 0 {
			hours = 0
		}
		recency := 1.0 / (1.0 + hours)
		score *= 0.5 + 0.5*recency // blend so recency doesn't dominate
	}
}
```
Ensure `time` is imported.

**Verify**: `go test ./internal/memory/` (after WP6 adds tests; for now just
`go build`).

---

## WP3 — Guard error classification

### 3.1 Define `GuardError`

File: `runtime/internal/guard/engine.go`

Add:
```go
type GuardErrorCode string

const (
	StepLimitExceeded   GuardErrorCode = "AGENT_STEP_LIMIT_EXCEEDED"
	ToolCallLoop        GuardErrorCode = "AGENT_TOOL_CALL_LOOP"
	InvalidToolCall     GuardErrorCode = "AGENT_INVALID_TOOL_CALL"
)

type GuardError struct {
	Code       GuardErrorCode
	Message    string
	Suggestion string
}

func (e GuardError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
```

Change `CheckAgent` to return `GuardError` instead of `provider.ProviderError`
in the agent-specific failure paths. Keep `Check` (non-agent) returning
`provider.ProviderError` for the existing non-agent codes (`EmptyPrompt`, etc.).

### 3.2 Remove agent codes from provider

File: `runtime/internal/provider/errors.go` — delete the three `AGENT_*`
constants. Remove the three `case` branches in `gateway/handlers.go` that
switch on them.

### 3.3 Handle `GuardError` in the gateway

File: `runtime/internal/gateway/handlers.go` — in the error-writing switch
(near line 310), add a type assertion:
```go
var gErr guard.GuardError
if errors.As(err, &gErr) {
	status := http.StatusTooManyRequests
	switch gErr.Code {
	case guard.ToolCallLoop:
		status = http.StatusConflict
	case guard.InvalidToolCall:
		status = http.StatusUnprocessableEntity
	}
	errResp := api.ErrorResponse{
		Error: api.APIError{
			Code:       string(gErr.Code),
			Message:    gErr.Message,
			Type:       "guard_error",
			Engine:     "guard",
			Retryable:  false,
			Suggestion: gErr.Suggestion,
			RequestID:  reqID,
		},
	}
	s.writeError(w, status, errResp)
	return
}
```
Import `github.com/gumi/gumi/runtime/internal/guard` and `errors`.

### 3.4 Update guard tests

File: `runtime/internal/guard/engine_test.go` — change:
- `out.Error.Code != provider.AGENT_STEP_LIMIT_EXCEEDED` →
  `out.Error.Code != guard.StepLimitExceeded`
- `out.Error.Code != provider.AGENT_TOOL_CALL_LOOP` →
  `out.Error.Code != guard.ToolCallLoop`
Remove the `provider` import if no longer used.

**Verify**: `go build ./...` + `go test ./internal/guard/`.

---

## WP4 — Router feedback loop + telemetry + memory events

### 4.1 Model Fit → Router

File: `runtime/internal/router/engine.go`

Add an optional `memoryEngine` field to `CodingRuleEngine`:
```go
type ModelFitLookup interface {
	GetBestModel(difficulty int, taskType string) (modelID string, successRate float64, ok bool)
}
```
Add `SetMemoryFit(lookup ModelFitLookup)` method. In `selectFromRegistry`,
after `FindBest` returns `best`, if `e.memoryFit != nil`, call
`GetBestModel(p.Difficulty, string(p.TaskType))`; if a model is returned and
it's in `availableModels` and has `successRate > 0.7`, prefer it over `best`.
Record the rationale in `RouteResult.Reason`.

Wire the memory engine into the router in `pipeline/engine.go` construction.

### 4.2 Populate `Alternatives`

File: `runtime/internal/router/engine.go`, `selectFromRegistry`.

Before returning the winning `RouteResult`, iterate all registry entries that
were considered but rejected and append `AlternativeConsidered` entries:
```go
type AlternativeConsidered struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Reason    string `json:"reason"`   // "below_min_context", "coding_strength_too_low", "not_available", "size_exceeds_max"
}
```
This type already exists in `telemetry.go` — reuse it. You may need to extend
`registry.FindBest` to return the rejected list, or add a `ListCandidates`
method to the registry that returns all profiles matching the action's
preferences minus the winner.

### 4.3 Emit `RoutingTelemetry`

File: `runtime/internal/pipeline/engine.go`, `resolveProviderAndProfile`,
after the `coding_route_selected` event:
```go
rt := router.NewRoutingTelemetry(pc.RequestID, pc.StepCount, codingProfile, result, hasTools)
pc.AddEvent("routing", "routing_telemetry", SeverityInfo, "structured routing telemetry", rt.ToMetadata())
```
Set `hasTools` by scanning `pc.NormalizedRequest.Messages` for `ToolCalls`.

### 4.4 `repetitions` escalation

Already done in WP1.2 (`applyEscalation` gets the repetitions param + message
history). Verify it fires: add a unit test in WP6.

### 4.5 `memory_eviction` event + retention

File: `runtime/internal/memory/memory.go`

Add a field to `MemoryEngine`:
```go
TelemetryHook func(event string, metadata map[string]string)
```
Set it from `pipeline/engine.go` after construction:
```go
e.memoryEngine.TelemetryHook = func(ev string, md map[string]string) {
	e.log.Info("memory_event", "event", ev, "metadata", md)
}
```
In `GarbageCollectExpired`:
- For LRU eviction, before deleting each over-limit fact, if
  `TelemetryHook != nil`, call it with `"memory_eviction"` and
  `{"fact_key": ..., "reason": "lru_max_facts"}`. You'll need to SELECT the
  to-be-deleted IDs+keys first, then delete.
- Add retention for `model_fit`:
  ```go
  retainDays := m.cfg.ModelFitRetentionDays
  if retainDays <= 0 { retainDays = 30 }
  m.db.Exec(`DELETE FROM model_fit WHERE datetime('now') > datetime(updated_at, '+' || ? || ' days')`, retainDays)
  ```

### 4.6 `MinObservationCount` gating

File: `runtime/internal/pipeline/engine.go`, `extractMemory` (~line 1285).

Before `StoreFact`, count prior observations:
```go
if e.cfg.Memory.MinObservationCount > 1 {
	obs := e.memoryEngine.ObservationCount(fact.Key)
	if obs+1 < e.cfg.Memory.MinObservationCount {
		e.memoryEngine.IncrementObservation(fact.Key) // new method
		continue // don't store yet
	}
}
e.memoryEngine.StoreFact(fact)
```
Add `ObservationCount(key string) int` and
`IncrementObservation(key string) error` to `memory/memory.go` using a new
`fact_observations` table (or a counter column on `facts`; a separate table is
cleaner since the fact may not be stored yet).

**Verify**: `go build ./...`. Full tests in WP6.

---

## WP5 — Error format + missing API route

### 5.1 Memory gateway structured errors

File: `runtime/internal/gateway/memory.go`

Delete the local `writeJSONError` function. Replace every call:
```go
writeJSONError(w, http.StatusInternalServerError, "failed to list facts: "+err.Error())
```
with:
```go
s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_ERROR", "failed to list facts: "+err.Error(), requestIDFromContext(r.Context())))
```
Use `api.NewRuntimeError` (same as `handlers.go`). Import
`github.com/gumi/gumi/runtime/internal/api`. Use distinct codes:
`MEMORY_LIST_ERROR`, `MEMORY_CLEAR_ERROR`, `MEMORY_FACT_CREATE_ERROR`,
`MEMORY_MODEL_FIT_ERROR`, `MEMORY_STATUS_ERROR`.

### 5.2 `POST /v1/gumi/memory/facts`

File: `runtime/internal/gateway/routes.go` — add:
```go
mux.HandleFunc("POST /v1/gumi/memory/facts", s.withAuthMiddleware(s.handleMemoryCreateFact))
```

File: `runtime/internal/gateway/memory.go` — add handler:
```go
func (s *Server) handleMemoryCreateFact(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		s.writeError(w, http.StatusServiceUnavailable, api.NewRuntimeError("MEMORY_DISABLED", "memory engine is not enabled", requestIDFromContext(r.Context())))
		return
	}
	var req struct {
		Key        string  `json:"key"`
		Value      string  `json:"value"`
		Source     string  `json:"source,omitempty"`
		Confidence float64 `json:"confidence,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}
	if req.Key == "" || req.Value == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "key and value are required", requestIDFromContext(r.Context())))
		return
	}
	if req.Confidence == 0 { req.Confidence = 0.7 }
	fact := memory.MemoryFact{Key: req.Key, Value: req.Value, Source: req.Source, Confidence: req.Confidence}
	if err := mem.StoreFact(fact); err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_FACT_CREATE_ERROR", "failed to store fact: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "ok", "fact": fact})
}
```

**Verify**: `go build ./...` + manual `curl -X POST` test.

---

## WP6 — Tests

Add test files (package `<pkgname>_test` using `testing` + `net/http/httptest`
where needed). Use an in-memory SQLite (`:memory:`) for memory tests.

### 6.1 `router/classifier_test.go`
- `TestClassifyDifficultyFromTraceback` — message with Python traceback → difficulty ≥ 4.
- `TestClassifyHintOverrides` — `HintDifficulty: 5` seeds difficulty; `HintTaskType: "refactor"` overrides.
- `TestApplyEscalationRetries` — retry count ≥ threshold bumps difficulty.
- `TestApplyEscalationSteps` — step count ≥ threshold bumps difficulty.
- `TestApplyEscalationRepetitions` — 3 identical tool calls → difficulty bump.

### 6.2 `router/engine_test.go`
- `TestRouteSimpleTask` — easy profile → small model.
- `TestRouteFallbackRelaxation` — no model meets min → relaxed fallback, `FallbackUsed=true`.
- `TestRouteAlternativesPopulated` — `RouteResult.Alternatives` non-empty with rejection reasons.
- `TestRouteModelFitBoost` — mock `ModelFitLookup` returns high-success model → preferred.

### 6.3 `router/registry_test.go`
- `TestFindBestFiltersByCodingStrength` — rejects below-min coding.
- `TestFindBestFiltersByContext` — rejects below-min context.
- `TestFindBestFiltersBySize` — rejects oversized.

### 6.4 `memory/memory_test.go`
- `TestStoreFactNew` — inserts.
- `TestStoreFactSameValueBumpsConfidence` — same key+value, higher confidence → update.
- `TestStoreFactConflictingValueKeepsBoth` — same key, different value → primary + `:alt`, no value lost.
- `TestFormatInjectionOrder` — output has Model Performance before This Session before Project Knowledge.
- `TestSelectRelevantFactsRecency` — recently-accessed fact scores higher than old one with same access_count.
- `TestGarbageCollectExpiredEvictsLRU` — over `max_facts` → oldest evicted.
- `TestGarbageCollectExpiredModelFitRetention` — old `model_fit` rows deleted.
- `TestMinObservationCountGating` — fact not stored until observed N times.

### 6.5 `provider/lmstudio_mgmt_test.go`
- Use `httptest.NewServer` to mock LM Studio REST API.
- `TestListModels` — returns parsed model list.
- `TestLoadModel` — POST to `/v1/models/load/{name}`, asserts params.
- `TestUnloadModel` — DELETE to `/v1/models/unload/{name}`.

### 6.6 `gateway/memory_test.go`
- `TestGetFacts` — 200, structured JSON.
- `TestGetFactsMemoryDisabled` — 200 with `enabled: false`.
- `TestPostFacts` — 201, fact stored.
- `TestPostFactsMissingFields` — 400, structured error (not `{"error":"..."}`).
- `TestClearMemory` — 200.
- `TestMemoryErrorShape` — assert response has `code`, `message`, `type`, `engine`, `request_id`.

### 6.7 `cli/lmstudio_test.go` + `cli/memory_test.go`
- Test flag parsing + output rendering to a `bytes.Buffer`.

**Verify**: `go test ./...` all green. `go vet ./...` clean.

---

## WP7 — Spec documentation

Update every spec that's now out of sync. Use precise section anchors.

1. `docs/specs/04-api-specification.md`
   - §11.1 — replace "Agent mode is reserved for future use." with a description
     of agent mode + the `Gumi` extension object (`routing`, `memory`,
     `context`, `validation`, `telemetry`).
   - §6 — add to the endpoint catalogue:
     `GET /v1/gumi/memory/facts`, `POST /v1/gumi/memory/facts`,
     `GET /v1/gumi/memory/model-fit`, `POST /v1/gumi/memory/clear`,
     `GET /v1/gumi/memory/status`.
   - §15.2 — add `guard_error` to the error `type` enum; note
     `AGENT_*` codes now come from the guard engine.

2. `docs/specs/05-configuration-specification.md`
   - §7.3 — replace "Agent mode is reserved for future versions." with the
     `runtime.agent` config block (`max_steps`,
     `tool_call_timeout_seconds`, `context_compaction_threshold`,
     `loop_detection`).
   - §12.6 — replace the `engines.memory` two-field stub with the full
     top-level `memory:` block matching `MemoryConfig` in `config.go`.
   - Add a new §12.7 `routing:` block with `enabled`, `mode`, `classifier`,
     `coding_rules`.

3. `docs/specs/06-provider-adapter-specification.md`
   - Add §22 "LM Studio Model Management" — `model_management` config block,
     `GET /v1/models`, `POST /v1/models/load/{name}`,
     `DELETE /v1/models/unload/{name}`, per-model overrides
     (`context_length`, `flash_attention`, `offload_kv_cache_to_gpu`,
     `eval_batch_size`, `num_experts`), `auto_unload` behaviour.

4. `docs/specs/12-cli-and-dashboard-specification.md`
   - §5 — add `gumi lmstudio` subcommand (`list`, `load`, `unload`, `info`)
     and `gumi memory` subcommand (`facts`, `search`, `clear`, `status`,
     `model-fit`) to the V1 command set.

5. `docs/specs/19-agentic-coding-router-specification.md`
   - Mark Sprint 13 status as **Shipped**.
   - §6.3 — note `repetitions` escalation now implemented.
   - §8.1 — document the full `routing.classifier.escalation_threshold` and
     `routing.coding_rules` config shape (matching WP1).
   - §8.2 — note per-request `routing` overrides are now honoured.

6. `docs/specs/20-memory-engine-specification.md`
   - Mark Phase 1 as **Shipped**; move `gumi memory` CLI to Phase 1.
   - §5.2 — confirm injection order is Model Fit → episodes → facts.
   - §5.3 — confirm recency formula is wired.
   - §7.3 — confirm `:alt` alternative behaviour is wired.
   - §8.1 — document `min_observation_count` + `model_fit_retention_days` as
     implemented.
   - §9 — document `memory_eviction` event as implemented.
   - §10 — document Model Fit → Router feedback loop as implemented.
   - §14 Open Question 2 — mark resolved: `POST /v1/gumi/memory/facts` added.

7. `CHANGELOG.md` — add a new "Fixed (Review 2026-07-13)" section listing:
   - Memory injection priority order corrected.
   - Fact deduplication now preserves alternatives.
   - Recency scoring added to fact selection.
   - Guard errors reclassified to `guard_error` type.
   - Memory gateway errors now use structured error format.
   - Per-request `gumi.memory` and `gumi.routing` overrides honoured.
   - Router config (`classifier`, `coding_rules`) now wired.
   - Model Fit → Router feedback loop connected.
   - `alternatives` and structured `routing_telemetry` now emitted.
   - `repetitions` escalation implemented.
   - `memory_eviction` telemetry event emitted.
   - `MinObservationCount` and `ModelFitRetentionDays` honoured.
   - `POST /v1/gumi/memory/facts` endpoint added.
   - Tests added for `router`, `memory`, `provider/lmstudio_mgmt`,
     `gateway/memory`, `cli/{lmstudio,memory}`.
   - Specs 04/05/06/12/19/20 updated to reflect shipped behaviour.

**Verify**: `grep -rn "reserved for future" docs/specs/` returns nothing for
agent mode or memory. Every shipped behaviour has a spec entry.

---

## Final checklist

- [ ] `gofmt -w .` clean
- [ ] `go vet ./...` clean
- [ ] `go build ./...` passes
- [ ] `go test ./...` all green
- [ ] No spec says "reserved" for a shipped feature
- [ ] No `writeJSONError` with bare-string errors remains
- [ ] No `provider.AGENT_*` constants remain
- [ ] `gumi.example.yaml` shows all new config fields
- [ ] CHANGELOG updated
- [ ] Commit per WP: `fix(review): WP1 config wiring`, `fix(review): WP2 memory
      correctness`, etc.

When all checkboxes pass, the review fix sprint is complete.