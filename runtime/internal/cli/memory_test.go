package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/memory"
)

func TestMemoryConfig(t *testing.T) {
	cfg := MemoryConfig()
	if cfg == nil {
		t.Fatal("expected non-nil MemoryConfig")
	}
	// Should return a default config with zero values.
	if cfg.Enabled {
		t.Fatal("expected Enabled=false by default")
	}
}

func TestResolveMemoryDBPath(t *testing.T) {
	path := resolveMemoryDBPath()
	if path == "" {
		t.Fatal("expected non-empty memory DB path")
	}
	if !strings.HasSuffix(path, "/.novexa/memory.db") {
		t.Fatalf("expected path ending in '/.novexa/memory.db', got %q", path)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long string that should be truncated", 20, "this is a long st..."},
		{"", 5, ""},
		{"a", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// captureStdout runs f and returns everything written to stdout.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr runs f and returns everything written to stderr.
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestRunMemoryStatus(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	output := captureStdout(func() {
		runMemoryStatus(eng, nil)
	})

	if !strings.Contains(output, "Memory Engine Status") {
		t.Fatal("expected 'Memory Engine Status' in output")
	}
	if !strings.Contains(output, "Facts stored: 0") {
		t.Fatal("expected 'Facts stored: 0' in output")
	}
	if !strings.Contains(output, "Model fit entries: 0") {
		t.Fatal("expected 'Model fit entries: 0' in output")
	}
}

func TestRunMemoryStatusWithData(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	// Store a fact.
	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "test:key",
		Value:      "test value",
		Source:     "test",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	// Record a model fit entry.
	if err := eng.RecordOutcome("test-model", 3, "feature", true, 1000, 1); err != nil {
		t.Fatalf("RecordOutcome failed: %v", err)
	}

	output := captureStdout(func() {
		runMemoryStatus(eng, nil)
	})

	if !strings.Contains(output, "Facts stored: 1") {
		t.Fatalf("expected 'Facts stored: 1' in output, got: %s", output)
	}
	if !strings.Contains(output, "Model fit entries: 1") {
		t.Fatalf("expected 'Model fit entries: 1' in output, got: %s", output)
	}
}

func TestRunMemoryStatusJSON(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	output := captureStdout(func() {
		runMemoryStatus(eng, []string{"--json"})
	})

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if facts, ok := parsed["facts_count"].(float64); !ok || facts != 0 {
		t.Fatalf("expected facts_count=0, got %v", facts)
	}
	if fit, ok := parsed["model_fit_entries"].(float64); !ok || fit != 0 {
		t.Fatalf("expected model_fit_entries=0, got %v", fit)
	}
}

func TestRunMemoryFacts(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	// Store a fact.
	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "project:language",
		Value:      "Go",
		Source:     "test",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	output := captureStdout(func() {
		runMemoryFacts(eng, nil)
	})

	if !strings.Contains(output, "Memory Facts") {
		t.Fatal("expected 'Memory Facts' in output")
	}
	if !strings.Contains(output, "project:language") {
		t.Fatal("expected 'project:language' in output")
	}
	if !strings.Contains(output, "1 facts shown") {
		t.Fatal("expected '1 facts shown' in output")
	}
}

func TestRunMemoryFactsEmpty(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	output := captureStdout(func() {
		runMemoryFacts(eng, nil)
	})

	if !strings.Contains(output, "No facts stored.") {
		t.Fatalf("expected 'No facts stored.' in output, got: %s", output)
	}
}

func TestRunMemoryFactsJSON(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "test:json",
		Value:      "json value",
		Source:     "test",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	output := captureStdout(func() {
		runMemoryFacts(eng, []string{"--json"})
	})

	var facts []memory.MemoryFact
	if err := json.Unmarshal([]byte(output), &facts); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Key != "test:json" {
		t.Fatalf("expected key 'test:json', got %q", facts[0].Key)
	}
}

func TestRunMemoryFactsSearch(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "project:framework",
		Value:      "React",
		Source:     "test",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}
	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "project:language",
		Value:      "TypeScript",
		Source:     "test",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	output := captureStdout(func() {
		runMemoryFacts(eng, []string{"framework"})
	})

	if !strings.Contains(output, "project:framework") {
		t.Fatal("expected 'project:framework' in search output")
	}
	if strings.Contains(output, "project:language") {
		t.Fatal("did not expect 'project:language' in search output")
	}
}

func TestRunMemoryClearWithForce(t *testing.T) {
	eng := newTestMemoryEngine(t)
	defer eng.Close()

	// Store a fact.
	if err := eng.StoreFact(memory.MemoryFact{
		Key:        "test:clear",
		Value:      "to be cleared",
		Source:     "test",
		Confidence: 0.8,
	}); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	output := captureStdout(func() {
		runMemoryClear(eng, []string{"--force"})
	})

	if !strings.Contains(output, "Memory cleared.") {
		t.Fatalf("expected 'Memory cleared.' in output, got: %s", output)
	}

	// Verify facts are gone.
	facts, err := eng.ListFacts(100)
	if err != nil {
		t.Fatalf("ListFacts failed: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("expected 0 facts after clear, got %d", len(facts))
	}
}

func TestRunMemoryClearWithoutForce(t *testing.T) {
	t.Skip("os.Exit(1) cannot be tested with stderr capture; requires subprocess refactor")
}

// newTestMemoryEngine creates a memory engine for CLI testing.
func newTestMemoryEngine(t *testing.T) *memory.MemoryEngine {
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
	eng, err := memory.New(cfg, "")
	if err != nil {
		t.Fatalf("failed to create memory engine: %v", err)
	}
	return eng
}
