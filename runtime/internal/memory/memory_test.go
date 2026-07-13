package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
)

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
	}
	eng, err := New(cfg, "")
	if err != nil {
		t.Fatalf("failed to create memory engine: %v", err)
	}
	return eng
}

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
	eng.db.Exec("UPDATE facts SET accessed_at = ? WHERE key = ?", old, "recent:key")
	eng.mu.Unlock()

	fact2 := MemoryFact{Key: "old:key", Value: "old value", Source: "test", Confidence: 0.8}
	if err := eng.StoreFact(fact2); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}
	// Manually set very old access time.
	eng.mu.Lock()
	eng.db.Exec("UPDATE facts SET accessed_at = ? WHERE key = ?", old, "old:key")
	eng.mu.Unlock()

	// Touch the recent fact to update its access time.
	eng.GetFact("recent:key")

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
	eng.db.Exec(
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

func TestMinObservationCountGating(t *testing.T) {
	eng := newTestEngine(t)
	defer eng.Close()

	// With MinObservationCount=2, a fact should not be stored until observed twice.
	key := "gated:key"
	obs := eng.ObservationCount(key)
	if obs != 0 {
		t.Fatalf("expected 0 observations initially, got %d", obs)
	}

	// First observation.
	count, err := eng.IncrementObservation(key)
	if err != nil {
		t.Fatalf("IncrementObservation failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Second observation.
	count, err = eng.IncrementObservation(key)
	if err != nil {
		t.Fatalf("IncrementObservation failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}

	// ObservationCount should return 2.
	obs = eng.ObservationCount(key)
	if obs != 2 {
		t.Fatalf("expected ObservationCount 2, got %d", obs)
	}
}
