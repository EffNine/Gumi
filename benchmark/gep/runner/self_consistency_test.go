package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/EffNine/gumi/benchmark/gep/providers"
	"github.com/EffNine/gumi/benchmark/gep/scorer"
	"github.com/EffNine/gumi/benchmark/gep/types"
)

// mockProvider is a test double that records which method was called.
type mockProvider struct {
	chatCalled    bool
	chatResponses []providers.ChatResponse
	chatErrors    []error
	index         int
}

func (m *mockProvider) ChatCompletion(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	m.chatCalled = true
	if m.index < len(m.chatResponses) {
		resp := m.chatResponses[m.index]
		var err error
		if m.index < len(m.chatErrors) {
			err = m.chatErrors[m.index]
		}
		m.index++
		return &resp, err
	}
	return &providers.ChatResponse{
		Choices: []providers.ChatChoice{
			{Message: providers.ChatMessage{Role: "assistant", Content: "mock response"}},
		},
	}, nil
}

func (m *mockProvider) Health(ctx context.Context) error { return nil }
func (m *mockProvider) Type() string                     { return "mock" }
func (m *mockProvider) Name() string                     { return "Mock" }

func TestSelfConsistencyDirectUsesProviderNotGumiRuntime(t *testing.T) {
	// Verify that ConditionDirect + self_consistency calls provider.ChatCompletion,
	// NOT callGumiRuntime.
	serverCalled := false
	gumiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "gumi response"}},
			},
		})
	}))
	defer gumiServer.Close()

	mock := &mockProvider{}
	mock.chatResponses = []providers.ChatResponse{
		// First call from runAttempt (the initial attempt before self-consistency branch)
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "answer A"}}}},
		// Second call from runSelfConsistencyAttempt (first prompt)
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "answer A"}}}},
		// Third call from runSelfConsistencyAttempt (second prompt)
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "answer A"}}}},
	}

	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
		GumiURL:     gumiServer.URL,
	}
	test := types.GEPTest{
		ID:       "sc-test",
		Type:     "self_consistency",
		Prompt:   "What is 2+2?",
		Variants: []string{"What is 2 plus 2?"},
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionDirect, test, 1, sc)

	// The mock provider must have been called.
	if !mock.chatCalled {
		t.Fatal("ConditionDirect self-consistency did not call provider.ChatCompletion")
	}
	// The Gumi runtime server must NOT have been called.
	if serverCalled {
		t.Fatal("ConditionDirect self-consistency incorrectly called Gumi runtime")
	}
	// Result should have valid score.
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.TestID != "sc-test" {
		t.Fatalf("expected test_id sc-test, got %s", result.TestID)
	}
}

func TestSelfConsistencyGumiStabilizedUsesGumiRuntime(t *testing.T) {
	// Verify that ConditionGumiStabilized + self_consistency calls callGumiRuntime.
	requestCount := 0
	gumiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "consistent answer"}},
			},
		})
	}))
	defer gumiServer.Close()

	mock := &mockProvider{}
	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
		GumiURL:     gumiServer.URL,
	}
	test := types.GEPTest{
		ID:       "sc-test-2",
		Type:     "self_consistency",
		Prompt:   "What is 2+2?",
		Variants: []string{"Calculate 2+2"},
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionGumiStabilized, test, 1, sc)

	// Gumi runtime should have been called for each prompt variant.
	if requestCount == 0 {
		t.Fatal("ConditionGumiStabilized self-consistency did not call Gumi runtime")
	}
	// The mock provider should NOT have been called.
	if mock.chatCalled {
		t.Fatal("ConditionGumiStabilized self-consistency incorrectly called provider.ChatCompletion")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestSelfConsistencyDirectProducesValidScores(t *testing.T) {
	// Both Direct and Gumi-stabilized self-consistency must produce valid scores.
	mock := &mockProvider{}
	mock.chatResponses = []providers.ChatResponse{
		// First call from runAttempt
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "4"}}}},
		// Second call from runSelfConsistencyAttempt (first prompt)
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "4"}}}},
		// Third call from runSelfConsistencyAttempt (second prompt)
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "4"}}}},
	}

	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
	}
	test := types.GEPTest{
		ID:             "sc-score-test",
		Type:           "self_consistency",
		Prompt:         "What is 2+2?",
		Variants:       []string{"Compute 2+2"},
		ExpectedAnswer: "4",
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionDirect, test, 1, sc)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.TestID != "sc-score-test" {
		t.Fatalf("expected test_id sc-score-test, got %s", result.TestID)
	}
	// Self-consistency score should be present.
	if _, ok := result.Subscores["self_consistency"]; !ok {
		t.Error("expected self_consistency subscore")
	}
	// Expected answer score should be present.
	if _, ok := result.Subscores["expected_answer"]; !ok {
		t.Error("expected expected_answer subscore")
	}
}

func TestSelfConsistencyGumiProducesValidScores(t *testing.T) {
	gumiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "42"}},
			},
		})
	}))
	defer gumiServer.Close()

	mock := &mockProvider{}
	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
		GumiURL:     gumiServer.URL,
	}
	test := types.GEPTest{
		ID:             "sc-score-gumi",
		Type:           "self_consistency",
		Prompt:         "What is 6*7?",
		Variants:       []string{"Multiply 6 by 7"},
		ExpectedAnswer: "42",
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionGumiStabilized, test, 1, sc)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.TestID != "sc-score-gumi" {
		t.Fatalf("expected test_id sc-score-gumi, got %s", result.TestID)
	}
	if _, ok := result.Subscores["self_consistency"]; !ok {
		t.Error("expected self_consistency subscore")
	}
}

func TestNonSelfConsistencyUnchanged(t *testing.T) {
	// Non-self-consistency tests should use the normal routing.
	mock := &mockProvider{}
	mock.chatResponses = []providers.ChatResponse{
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "hello"}}}},
	}

	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
	}
	test := types.GEPTest{
		ID:     "normal-test",
		Type:   "instruction_following",
		Prompt: "Say hello",
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionDirect, test, 1, sc)

	if !mock.chatCalled {
		t.Fatal("normal test did not call provider.ChatCompletion")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestSelfConsistencyRoutingWithOnlyDirect(t *testing.T) {
	// When only ConditionDirect is requested, self-consistency should use provider.
	mock := &mockProvider{}
	mock.chatResponses = []providers.ChatResponse{
		{Choices: []providers.ChatChoice{{Message: providers.ChatMessage{Role: "assistant", Content: "ok"}}}},
	}

	gumiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Gumi runtime should not be called for ConditionDirect")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "x"}},
			},
		})
	}))
	defer gumiServer.Close()

	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
		GumiURL:     gumiServer.URL,
		Conditions:  []types.GEPCondition{types.ConditionDirect},
	}
	test := types.GEPTest{
		ID:       "sc-only-direct",
		Type:     "self_consistency",
		Prompt:   "Test",
		Variants: []string{"Var1"},
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionDirect, test, 1, sc)

	if !mock.chatCalled {
		t.Fatal("ConditionDirect self-consistency must call provider")
	}
	if result.Error != "" && !strings.Contains(result.Error, "no prompt variants") {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}

func TestSelfConsistencyRoutingWithOnlyGumi(t *testing.T) {
	// When only ConditionGumiStabilized is requested, self-consistency should use Gumi.
	gumiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer gumiServer.Close()

	mock := &mockProvider{}
	cfg := RunConfig{
		Model:       "test-model",
		Provider:    types.ProviderLMStudio,
		ProviderURL: "http://example.com",
		GumiURL:     gumiServer.URL,
		Conditions:  []types.GEPCondition{types.ConditionGumiStabilized},
	}
	test := types.GEPTest{
		ID:       "sc-only-gumi",
		Type:     "self_consistency",
		Prompt:   "Test",
		Variants: []string{"Var1"},
	}
	sc := scorer.New()

	result := runAttempt(context.Background(), mock, cfg, types.ConditionGumiStabilized, test, 1, sc)

	if mock.chatCalled {
		t.Fatal("ConditionGumiStabilized self-consistency must not call provider")
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
}
