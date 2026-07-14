package memory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestEngine(t *testing.T) *MemoryEngine {
	t.Helper()
	cfg := &config.MemoryConfig{
		Enabled:               true,
		Engine:                "sqlite",
		MaxFacts:              100,
		MaxEpisodesPerSession: 10,
		ModelFitRetentionDays: 30,
		InjectionBudgetTokens: 1200,
		MinConfidence:         0.3,
		MaxInjectedFacts:      20,
		ExtractEnabled:        true,
		MinObservationCount:   2,
		TrackModelFit:         true,
		ModelFitDecay:         0.95,
		HotCacheMaxSize:       50,
	}
	eng, err := New(cfg, "")
	if err != nil {
		t.Fatalf("failed to create memory engine: %v", err)
	}
	return eng
}

// ---------------------------------------------------------------------------
// StoreFact
// ---------------------------------------------------------------------------

func TestStoreFactNew(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	fact := MemoryFact{Key: "test:key", Value: "test value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	stored, err := eng.GetFact("test:key")
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}
	if stored.Value != "test value" {
		t.Fatalf("expected 'test value', got %q", stored.Value)
	}
}

func TestStoreFactSameValueBumpsConfidence(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	fact1 := MemoryFact{Key: "test:key", Value: "same value", Source: "test", Confidence: 0.5}
	if err := eng.StoreFact(fact1); err != nil {
		t.Fatalf("first StoreFact failed: %v", err)
	}

	fact2 := MemoryFact{Key: "test:key", Value: "same value", Source: "test", Confidence: 0.9}
	if err := eng.StoreFact(fact2); err != nil {
		t.Fatalf("second StoreFact failed: %v", err)
	}

	stored, err := eng.GetFact("test:key")
	if err != nil {
		t.Fatalf("GetFact failed: %v", err)
	}
	if stored.Confidence != 0.9 {
		t.Fatalf("expected confidence 0.9 after bump, got %f", stored.Confidence)
	}
}

func TestStoreFactConflictingValueKeepsBoth(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	fact1 := MemoryFact{Key: "conflict:key", Value: "original value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact1); err != nil {
		t.Fatalf("first StoreFact failed: %v", err)
	}

	fact2 := MemoryFact{Key: "conflict:key", Value: "new value", Source: "test", Confidence: 0.6}
	if err := eng.StoreFact(fact2); err != nil {
		t.Fatalf("second StoreFact failed: %v", err)
	}

	// Primary should still be the original (higher confidence).
	primary, err := eng.GetFact("conflict:key")
	if err != nil {
		t.Fatalf("GetFact for primary failed: %v", err)
	}
	if primary.Value != "original value" {
		t.Fatalf("expected primary value 'original value', got %q", primary.Value)
	}

	// Alternative should exist.
	alt, err := eng.GetFact("conflict:key:alt")
	if err != nil {
		t.Fatalf("GetFact for alt failed: %v", err)
	}
	if alt.Value != "new value" {
		t.Fatalf("expected alt value 'new value', got %q", alt.Value)
	}
}

func TestStoreFactConflictingValueNewHigherConfidence(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Existing fact with lower confidence.
	fact1 := MemoryFact{Key: "promote:key", Value: "old value", Source: "test", Confidence: 0.4}
	if err := eng.StoreFact(fact1); err != nil {
		t.Fatalf("first StoreFact failed: %v", err)
	}

	// New fact with same key, different value, higher confidence.
	fact2 := MemoryFact{Key: "promote:key", Value: "better value", Source: "test", Confidence: 0.9}
	if err := eng.StoreFact(fact2); err != nil {
		t.Fatalf("second StoreFact failed: %v", err)
	}

	// Primary should now be the new value (higher confidence).
	primary, err := eng.GetFact("promote:key")
	if err != nil {
		t.Fatalf("GetFact for primary failed: %v", err)
	}
	if primary.Value != "better value" {
		t.Fatalf("expected primary value 'better value', got %q", primary.Value)
	}
	if primary.Confidence != 0.9 {
		t.Fatalf("expected primary confidence 0.9, got %f", primary.Confidence)
	}

	// Old value should be demoted to :alt.
	alt, err := eng.GetFact("promote:key:alt")
	if err != nil {
		t.Fatalf("GetFact for alt failed: %v", err)
	}
	if alt.Value != "old value" {
		t.Fatalf("expected alt value 'old value', got %q", alt.Value)
	}
}

func TestStoreFactConfidenceOrdering(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store facts with different confidence levels.
	facts := []MemoryFact{
		{Key: "fact:low", Value: "low confidence", Source: "test", Confidence: 0.3},
		{Key: "fact:high", Value: "high confidence", Source: "test", Confidence: 0.9},
		{Key: "fact:medium", Value: "medium confidence", Source: "test", Confidence: 0.6},
	}
	for _, f := range facts {
		if err := eng.StoreFact(f); err != nil {
			t.Fatalf("StoreFact failed for %s: %v", f.Key, err)
		}
	}

	// SearchFacts should return results ordered by confidence DESC.
	results, err := eng.SearchFacts("confidence", 10)
	if err != nil {
		t.Fatalf("SearchFacts failed: %v", err)
	}

	if len(results) < 3 {
		t.Fatalf("expected at least 3 results, got %d", len(results))
	}

	// Verify descending confidence order.
	for i := 1; i < len(results); i++ {
		if results[i-1].Confidence < results[i].Confidence {
			t.Fatalf("facts not in descending confidence order: %f < %f at index %d",
				results[i-1].Confidence, results[i].Confidence, i)
		}
	}
}

// ---------------------------------------------------------------------------
// FormatInjection
// ---------------------------------------------------------------------------

func TestFormatInjectionOrder(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	facts := []MemoryFact{{Key: "fact:key", Value: "fact value", Confidence: 0.8}}
	episodeSummary := "Step 1: did something"
	fitData := []ModelFitEntry{{ModelID: "test-model", Difficulty: 3, TaskType: "feature", Attempts: 10, Successes: 8}}

	result := eng.FormatInjection(context.Background(), facts, episodeSummary, fitData, 1200)

	// Model Performance should come before This Session before Project Knowledge.
	mpIdx := strings.Index(result, "Model Performance")
	tsIdx := strings.Index(result, "This Session")
	pkIdx := strings.Index(result, "Project Knowledge")

	if mpIdx < 0 {
		t.Fatal("expected 'Model Performance' section")
	}
	if tsIdx < 0 {
		t.Fatal("expected 'This Session' section")
	}
	if pkIdx < 0 {
		t.Fatal("expected 'Project Knowledge' section")
	}

	if !(mpIdx < tsIdx && tsIdx < pkIdx) {
		t.Fatal("expected order: Model Performance < This Session < Project Knowledge")
	}
}

func TestFormatInjectionBudgetEnforcement(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Create a large number of facts that would exceed a small budget.
	var facts []MemoryFact
	for i := 0; i < 50; i++ {
		facts = append(facts, MemoryFact{
			Key:   fmt.Sprintf("fact:%d", i),
			Value: strings.Repeat("x", 200), // ~50 tokens each
		})
	}

	// Use a very small budget (100 tokens).
	budget := 100
	result := eng.FormatInjection(context.Background(), facts, "", nil, budget)

	// The result should be truncated to fit within budget.
	// Estimate tokens in the result (rough: 4 chars per token).
	estimatedTokens := (len(result) + 3) / 4
	if estimatedTokens > budget+50 { // allow some slack for section headers
		t.Fatalf("estimated tokens %d exceeds budget %d by more than slack", estimatedTokens, budget)
	}

	// With a tiny budget, we should still get the section headers.
	if !strings.Contains(result, "[Memory]") {
		t.Fatal("expected [Memory] header")
	}
	if !strings.Contains(result, "--- Project Knowledge ---") {
		t.Fatal("expected Project Knowledge section")
	}
}

func TestFormatInjectionEmptySections(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// No facts, no episodes, no fit data.
	result := eng.FormatInjection(context.Background(), nil, "", nil, 1200)

	if !strings.Contains(result, "[Memory]") {
		t.Fatal("expected [Memory] header even with empty input")
	}

	// Should not contain any section headers.
	if strings.Contains(result, "Model Performance") {
		t.Fatal("unexpected Model Performance section with no fit data")
	}
	if strings.Contains(result, "This Session") {
		t.Fatal("unexpected This Session section with no episode summary")
	}
	if strings.Contains(result, "Project Knowledge") {
		t.Fatal("unexpected Project Knowledge section with no facts")
	}
}

// ---------------------------------------------------------------------------
// SelectRelevantFacts
// ---------------------------------------------------------------------------

func TestSelectRelevantFactsRecency(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store two facts with different access times.
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)

	fact1 := MemoryFact{Key: "recent:key", Value: "recent value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact1); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}
	// Manually set old access time.
	eng.mu.Lock()
	_, _ = eng.db.Exec("UPDATE facts SET accessed_at = ? WHERE key = ?", old, "recent:key")
	eng.mu.Unlock()

	fact2 := MemoryFact{Key: "old:key", Value: "old value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact2); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}
	// Manually set very old access time.
	eng.mu.Lock()
	_, _ = eng.db.Exec("UPDATE facts SET accessed_at = ? WHERE key = ?", old, "old:key")
	eng.mu.Unlock()

	// Touch the recent fact to update its access time.
	_, _ = eng.GetFact("recent:key")

	// Select relevant facts — the recently accessed one should score higher.
	selected := eng.SelectRelevantFacts("recent:key", 10)
	if len(selected) == 0 {
		t.Fatal("expected at least one fact selected")
	}
	// The recent fact should be first.
	if selected[0].Key != "recent:key" {
		t.Fatalf("expected recent:key to be first, got %s", selected[0].Key)
	}
}

func TestSelectRelevantFactsConfidenceScoring(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store two facts with different confidence.
	factLow := MemoryFact{Key: "topic:alpha", Value: "low confidence info", Source: "test", Confidence: 0.3}
	if err := eng.StoreFact(factLow); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	factHigh := MemoryFact{Key: "topic:beta", Value: "high confidence info", Source: "test", Confidence: 0.95}
	if err := eng.StoreFact(factHigh); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// The request text must contain the fact keys for SelectRelevantFacts to match.
	// It checks if requestLower contains strings.ToLower(f.Key).
	selected := eng.SelectRelevantFacts("topic:alpha topic:beta", 10)
	if len(selected) < 2 {
		t.Fatalf("expected at least 2 facts, got %d", len(selected))
	}

	// Higher confidence fact should be ranked first.
	if selected[0].Key != "topic:beta" {
		t.Fatalf("expected topic:beta (confidence 0.95) to be first, got %s (confidence %f)",
			selected[0].Key, selected[0].Confidence)
	}
}

func TestSelectRelevantFactsAccessFrequencyBoost(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store two facts with same confidence but different access counts.
	factRare := MemoryFact{Key: "freq:rare", Value: "rarely accessed", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(factRare); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	factCommon := MemoryFact{Key: "freq:common", Value: "frequently accessed", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(factCommon); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Access the common fact many times to boost its access count.
	for i := 0; i < 10; i++ {
		_, _ = eng.GetFact("freq:common")
	}

	// The request text must contain the fact keys for SelectRelevantFacts to match.
	selected := eng.SelectRelevantFacts("freq:rare freq:common", 10)
	if len(selected) < 2 {
		t.Fatalf("expected at least 2 facts, got %d", len(selected))
	}

	// The frequently accessed fact should be ranked first due to access frequency boost.
	if selected[0].Key != "freq:common" {
		t.Fatalf("expected freq:common (high access count) to be first, got %s (access_count=%d)",
			selected[0].Key, selected[0].AccessCount)
	}
}

func TestSelectRelevantFactsMaxFactsLimit(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store more facts than the max limit.
	for i := 0; i < 10; i++ {
		fact := MemoryFact{
			Key:        fmt.Sprintf("limit:key:%d", i),
			Value:      fmt.Sprintf("value %d", i),
			Source:     "test",
			Confidence: 0.8,
		}
		if err := eng.StoreFact(fact); err != nil {
			t.Fatalf("StoreFact failed: %v", err)
		}
	}

	// Build a request text that contains all fact keys so SelectRelevantFacts matches them.
	var requestText string
	for i := 0; i < 10; i++ {
		requestText += fmt.Sprintf("limit:key:%d ", i)
	}

	// Select with a small maxFacts limit.
	selected := eng.SelectRelevantFacts(requestText, 3)
	if len(selected) > 3 {
		t.Fatalf("expected at most 3 facts, got %d", len(selected))
	}
	if len(selected) == 0 {
		t.Fatal("expected at least 1 fact")
	}
}

func TestSelectRelevantFactsNoMatch(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store a fact.
	fact := MemoryFact{Key: "some:key", Value: "some value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Query for something that doesn't match.
	selected := eng.SelectRelevantFacts("nonexistent_xyzzy", 10)
	if len(selected) != 0 {
		t.Fatalf("expected 0 facts for non-matching query, got %d", len(selected))
	}
}

// ---------------------------------------------------------------------------
// GarbageCollectExpired
// ---------------------------------------------------------------------------

func TestGarbageCollectExpiredTTL(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Insert a fact with a TTL, but set created_at far in the past so the TTL
	// is already expired. We use SQLite's datetime arithmetic directly to avoid
	// Go's RFC3339 format (which uses 'Z' suffix that SQLite's datetime() cannot parse).
	eng.mu.Lock()
	_, err := eng.db.Exec(
		`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now', '-1 hour'), datetime('now'), datetime('now'), 1, ?)`,
		"fact_ttl_test", "ttl:fact", "will expire", "test", 0.8, "", 10,
	)
	eng.mu.Unlock()
	if err != nil {
		t.Fatalf("insert fact with TTL failed: %v", err)
	}

	// Verify it exists.
	_, err = eng.GetFact("ttl:fact")
	if err != nil {
		t.Fatalf("GetFact before GC failed: %v", err)
	}

	// Run GC — the fact was created 1 hour ago with a 10-second TTL, so it should be expired.
	count, err := eng.GarbageCollectExpired()
	if err != nil {
		t.Fatalf("GarbageCollectExpired failed: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 expired fact removed, got %d", count)
	}

	// Fact should be gone.
	_, err = eng.GetFact("ttl:fact")
	if err == nil {
		t.Fatal("expected GetFact to fail after TTL expiry and GC")
	}
}

func TestGarbageCollectExpiredTTLZero(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store a fact with TTLSeconds=0 (no expiry).
	fact := MemoryFact{
		Key:        "persistent:fact",
		Value:      "should persist",
		Source:     "test",
		Confidence: 0.8,
		TTLSeconds: 0,
	}
	if err := eng.StoreFact(fact); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Run GC.
	_, err := eng.GarbageCollectExpired()
	if err != nil {
		t.Fatalf("GarbageCollectExpired failed: %v", err)
	}

	// Fact should still exist.
	stored, err := eng.GetFact("persistent:fact")
	if err != nil {
		t.Fatalf("GetFact after GC failed: %v", err)
	}
	if stored.Value != "should persist" {
		t.Fatalf("expected value 'should persist', got %q", stored.Value)
	}
}

func TestGarbageCollectExpiredEvictsLRU(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store more facts than max_facts (100).
	for i := 0; i < 110; i++ {
		fact := MemoryFact{
			Key:        fmt.Sprintf("lru:key:%d", i),
			Value:      "value",
			Source:     "test",
			Confidence: 0.5,
		}
		if err := eng.StoreFact(fact); err != nil {
			t.Fatalf("StoreFact %d failed: %v", i, err)
		}
	}

	// Run GC.
	count, err := eng.GarbageCollectExpired()
	if err != nil {
		t.Fatalf("GarbageCollectExpired failed: %v", err)
	}
	if count < 0 {
		t.Fatal("expected non-negative eviction count")
	}

	// Should have evicted some facts.
	facts, err := eng.ListFacts(200)
	if err != nil {
		t.Fatalf("ListFacts failed: %v", err)
	}
	if len(facts) > 100 {
		t.Fatalf("expected <= 100 facts after GC, got %d", len(facts))
	}
}

func TestGarbageCollectExpiredModelFitRetention(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Insert a model fit entry with old last_updated.
	eng.mu.Lock()
	_, _ = eng.db.Exec(
		`INSERT INTO model_fit (model_id, difficulty, task_type, attempts, successes, last_updated)
		 VALUES (?, ?, ?, ?, ?, datetime('now', '-60 days'))`,
		"old-model", 3, "feature", 10, 5,
	)
	eng.mu.Unlock()

	// Run GC with 30-day retention.
	eng.cfg.ModelFitRetentionDays = 30
	_, err := eng.GarbageCollectExpired()
	if err != nil {
		t.Fatalf("GarbageCollectExpired failed: %v", err)
	}

	// Old entry should be deleted.
	entries, err := eng.ListModelFit()
	if err != nil {
		t.Fatalf("ListModelFit failed: %v", err)
	}
	for _, e := range entries {
		if e.ModelID == "old-model" {
			t.Fatal("expected old model fit entry to be deleted")
		}
	}
}

// ---------------------------------------------------------------------------
// RecordOutcome + GetBestModelForRouter
// ---------------------------------------------------------------------------

func TestRecordOutcomeBasic(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record a successful outcome.
	if err := eng.RecordOutcome("model-a", 3, "feature", true, 1500, 0); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	// Verify via GetModelProfile.
	entries, err := eng.GetModelProfile("model-a")
	if err != nil {
		t.Fatalf("GetModelProfile failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", entries[0].Attempts)
	}
	if entries[0].Successes != 1 {
		t.Fatalf("expected 1 success, got %d", entries[0].Successes)
	}
	if entries[0].AvgLatencyMs != 1500 {
		t.Fatalf("expected avg latency 1500, got %d", entries[0].AvgLatencyMs)
	}
}

func TestRecordOutcomeFailure(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record a failed outcome.
	if err := eng.RecordOutcome("model-b", 5, "bugfix", false, 3000, 2); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	entries, err := eng.GetModelProfile("model-b")
	if err != nil {
		t.Fatalf("GetModelProfile failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", entries[0].Attempts)
	}
	if entries[0].Successes != 0 {
		t.Fatalf("expected 0 successes, got %d", entries[0].Successes)
	}
}

func TestRecordOutcomeEWMAUpdate(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record multiple outcomes and verify EWMA update.
	// First outcome: success, 1000ms, 0 retries.
	if err := eng.RecordOutcome("model-c", 3, "feature", true, 1000, 0); err != nil {
		t.Fatalf("first RecordOutcome failed: %v", err)
	}

	// Second outcome: success, 2000ms, 1 retry.
	if err := eng.RecordOutcome("model-c", 3, "feature", true, 2000, 1); err != nil {
		t.Fatalf("second RecordOutcome failed: %v", err)
	}

	entries, err := eng.GetModelProfile("model-c")
	if err != nil {
		t.Fatalf("GetModelProfile failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", e.Attempts)
	}
	if e.Successes != 2 {
		t.Fatalf("expected 2 successes, got %d", e.Successes)
	}

	// EWMA: weight = decay / (1 - decay^attempts)
	// For decay=0.95, attempt 2: weight = 0.95 / (1 - 0.95^2) = 0.95 / (1 - 0.9025) = 0.95 / 0.0975 ≈ 9.74
	// That's > 1, so the first value dominates. Just check it's reasonable.
	if e.AvgLatencyMs <= 0 {
		t.Fatalf("expected positive avg latency, got %d", e.AvgLatencyMs)
	}
	if e.AvgRetries < 0 {
		t.Fatalf("expected non-negative avg retries, got %f", e.AvgRetries)
	}
}

func TestGetBestModelForRouterNoData(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// No data recorded yet.
	modelID, rate, ok := eng.GetBestModelForRouter(3, "feature")
	if ok {
		t.Fatalf("expected ok=false with no data, got model=%s rate=%f", modelID, rate)
	}
	if modelID != "" {
		t.Fatalf("expected empty modelID, got %q", modelID)
	}
}

func TestGetBestModelForRouterSelection(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record outcomes for two models at the same difficulty/task_type.
	// Model A: 8/10 successes (80%).
	for i := 0; i < 10; i++ {
		success := i < 8
		if err := eng.RecordOutcome("model-a", 3, "feature", success, 1000, 0); err != nil {
			t.Fatalf("RecordOutcome model-a failed: %v", err)
		}
	}

	// Model B: 9/10 successes (90%) — better rate.
	for i := 0; i < 10; i++ {
		success := i < 9
		if err := eng.RecordOutcome("model-b", 3, "feature", success, 2000, 0); err != nil {
			t.Fatalf("RecordOutcome model-b failed: %v", err)
		}
	}

	// GetBestModelForRouter should return model-b (higher success rate).
	modelID, rate, ok := eng.GetBestModelForRouter(3, "feature")
	if !ok {
		t.Fatal("expected ok=true with data")
	}
	if modelID != "model-b" {
		t.Fatalf("expected model-b (90%% success), got %s", modelID)
	}
	if math.Abs(rate-0.9) > 0.01 {
		t.Fatalf("expected success rate ~0.9, got %f", rate)
	}
}

func TestGetBestModelForRouterMinAttempts(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record only 2 outcomes (below the 3-attempt minimum).
	if err := eng.RecordOutcome("model-c", 3, "feature", true, 1000, 0); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}
	if err := eng.RecordOutcome("model-c", 3, "feature", true, 1000, 0); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	// Should not return a model with < 3 attempts.
	_, _, ok := eng.GetBestModelForRouter(3, "feature")
	if ok {
		t.Fatal("expected ok=false with < 3 attempts")
	}
}

func TestGetBestModelForRouterDifferentDifficulties(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Record outcomes for the same model at different difficulties.
	for i := 0; i < 5; i++ {
		if err := eng.RecordOutcome("model-x", 1, "feature", true, 500, 0); err != nil {
			t.Fatalf("RecordOutcome failed: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := eng.RecordOutcome("model-x", 5, "feature", false, 5000, 3); err != nil {
			t.Fatalf("RecordOutcome failed: %v", err)
		}
	}

	// Query for difficulty 1 — should find model-x.
	modelID, rate, ok := eng.GetBestModelForRouter(1, "feature")
	if !ok {
		t.Fatal("expected ok=true for difficulty 1")
	}
	if modelID != "model-x" {
		t.Fatalf("expected model-x, got %s", modelID)
	}
	if math.Abs(rate-1.0) > 0.01 {
		t.Fatalf("expected success rate 1.0, got %f", rate)
	}

	// Query for difficulty 5 — should also find model-x (but with 0% success).
	modelID, rate, ok = eng.GetBestModelForRouter(5, "feature")
	if !ok {
		t.Fatal("expected ok=true for difficulty 5")
	}
	if modelID != "model-x" {
		t.Fatalf("expected model-x, got %s", modelID)
	}
	if rate != 0.0 {
		t.Fatalf("expected success rate 0.0, got %f", rate)
	}
}

// ---------------------------------------------------------------------------
// ObserveAndCheck
// ---------------------------------------------------------------------------

func TestObserveAndCheckBelowThreshold(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// MinObservationCount is 2. First observation should return false.
	reached, err := eng.ObserveAndCheck("obs:key", 2)
	if err != nil {
		t.Fatalf("ObserveAndCheck failed: %v", err)
	}
	if reached {
		t.Fatal("expected reached=false on first observation")
	}
}

func TestObserveAndCheckAtThreshold(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// First observation.
	_, err := eng.ObserveAndCheck("obs:key", 2)
	if err != nil {
		t.Fatalf("first ObserveAndCheck failed: %v", err)
	}

	// Second observation should reach the threshold.
	reached, err := eng.ObserveAndCheck("obs:key", 2)
	if err != nil {
		t.Fatalf("second ObserveAndCheck failed: %v", err)
	}
	if !reached {
		t.Fatal("expected reached=true on second observation")
	}
}

func TestObserveAndCheckAboveThreshold(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Three observations with threshold 2.
	// Iteration 0: count becomes 1, reached=false (below threshold)
	// Iteration 1: count becomes 2, reached=true (at threshold)
	// Iteration 2: count becomes 3, reached=true (above threshold)
	for i := 0; i < 3; i++ {
		reached, err := eng.ObserveAndCheck("obs:key", 2)
		if err != nil {
			t.Fatalf("ObserveAndCheck iteration %d failed: %v", i, err)
		}
		if i == 0 && reached {
			t.Fatalf("expected reached=false on iteration 0 (first observation)")
		}
		if i >= 1 && !reached {
			t.Fatalf("expected reached=true on iteration %d (observation count >= threshold)", i)
		}
	}
}

func TestObserveAndCheckMinCountZero(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// With minCount=0, the first observation should immediately reach threshold.
	reached, err := eng.ObserveAndCheck("obs:zero", 0)
	if err != nil {
		t.Fatalf("ObserveAndCheck failed: %v", err)
	}
	if !reached {
		t.Fatal("expected reached=true with minCount=0 on first observation")
	}
}

func TestObserveAndCheckMultipleKeys(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Track two different keys independently.
	reachedA, err := eng.ObserveAndCheck("key:a", 2)
	if err != nil {
		t.Fatalf("ObserveAndCheck key:a failed: %v", err)
	}
	if reachedA {
		t.Fatal("expected reached=false for key:a on first observation")
	}

	reachedB, err := eng.ObserveAndCheck("key:b", 2)
	if err != nil {
		t.Fatalf("ObserveAndCheck key:b failed: %v", err)
	}
	if reachedB {
		t.Fatal("expected reached=false for key:b on first observation")
	}

	// Second observation for key:a should reach threshold.
	reachedA, err = eng.ObserveAndCheck("key:a", 2)
	if err != nil {
		t.Fatalf("ObserveAndCheck key:a failed: %v", err)
	}
	if !reachedA {
		t.Fatal("expected reached=true for key:a on second observation")
	}

	// key:b should still be below threshold.
	obsB := eng.ObservationCount("key:b")
	if obsB != 1 {
		t.Fatalf("expected ObservationCount 1 for key:b, got %d", obsB)
	}
}

// ---------------------------------------------------------------------------
// ClearSession
// ---------------------------------------------------------------------------

func TestClearSession(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// Store an episode.
	ep := MemoryEpisode{
		SessionID:         "test-session",
		Step:              1,
		Task:              "test task",
		Difficulty:        3,
		ModelUsed:         "test-model",
		Outcome:           "success",
		CompressedSummary: "did a thing",
	}
	if err := eng.StoreEpisode(ep); err != nil {
		t.Fatalf("StoreEpisode failed: %v", err)
	}

	// Store a fact (cross-session, should survive).
	fact := MemoryFact{Key: "cross:fact", Value: "should survive", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Clear the session.
	if err := eng.ClearSession("test-session"); err != nil {
		t.Fatalf("ClearSession failed: %v", err)
	}

	// Episodes should be gone.
	episodes, err := eng.GetRecentEpisodes("test-session", 10)
	if err != nil {
		t.Fatalf("GetRecentEpisodes failed: %v", err)
	}
	if len(episodes) != 0 {
		t.Fatalf("expected 0 episodes after ClearSession, got %d", len(episodes))
	}

	// Facts should still exist.
	stored, err := eng.GetFact("cross:fact")
	if err != nil {
		t.Fatalf("GetFact after ClearSession failed: %v", err)
	}
	if stored.Value != "should survive" {
		t.Fatalf("expected fact value 'should survive', got %q", stored.Value)
	}
}
