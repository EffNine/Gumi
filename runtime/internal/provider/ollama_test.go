package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

func newOllamaTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			resp := map[string]interface{}{
				"models": []map[string]interface{}{
					{"name": "llama3", "modified_at": time.Now().UTC().Format(time.RFC3339)},
					{"name": "phi3", "modified_at": time.Now().UTC().Format(time.RFC3339)},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/chat":
			var req ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			content := "hello from ollama"
			thinking := ""
			// If think is explicitly false, return normal content.
			// If think is nil or true, simulate thinking response.
			if req.Think == nil || *req.Think {
				thinking = "I need to think about this..."
			}
			resp := ollamaChatResponse{
				Model: req.Model,
				Message: ollamaMessage{
					Role:     "assistant",
					Content:  content,
					Thinking: thinking,
				},
				Done: true,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func newOllamaAdapter(t *testing.T, serverURL string) *OllamaAdapter {
	t.Helper()
	log := logger.New("error")
	adapter, err := NewOllamaAdapter("ollama", config.ProviderSettings{URL: serverURL, TimeoutSeconds: 5}, log)
	if err != nil {
		t.Fatalf("failed to create ollama adapter: %v", err)
	}
	return adapter.(*OllamaAdapter)
}

func TestOllamaHealthCheck(t *testing.T) {
	server := newOllamaTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	status, err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}
	if status != StatusOK {
		t.Errorf("expected status ok, got %s", status)
	}
}

func TestOllamaListModels(t *testing.T) {
	server := newOllamaTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected list models error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Name != "llama3" {
		t.Errorf("expected llama3, got %s", models[0].Name)
	}
}

func TestOllamaGenerate(t *testing.T) {
	server := newOllamaTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	resp, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "llama3",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected assistant role, got %s", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].Message.Content != "hello from ollama" {
		t.Errorf("unexpected content: %v", resp.Choices[0].Message.Content)
	}
}

func TestOllamaNormalizeErrorOffline(t *testing.T) {
	adapter := newOllamaAdapter(t, "http://localhost:1")
	err := adapter.NormalizeError(ProviderError{Code: ProviderUnavailable, Message: "offline"})
	if err.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, err.Code)
	}
}

func TestOllamaGenerateWithThinkFalse(t *testing.T) {
	server := newOllamaTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	falseVal := false
	resp, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "llama3",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
		Novexa: &api.NovexaExtensions{
			Thinking: &api.ThinkingConfig{Enabled: &falseVal},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	if content != "hello from ollama" {
		t.Errorf("expected content 'hello from ollama', got %q", content)
	}
}

func TestOllamaGenerateOmitsThinkWhenUnspecified(t *testing.T) {
	server := newOllamaTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	resp, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "llama3",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	if content != "hello from ollama" {
		t.Errorf("expected content 'hello from ollama', got %q", content)
	}
}

func TestOllamaGenerateEmptyContentWithThinkingReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := ollamaChatResponse{
				Model: "test-model",
				Message: ollamaMessage{
					Role:     "assistant",
					Content:  "",
					Thinking: "I am thinking very hard about this...",
				},
				Done: true,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	}))
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	_, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "test-model",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty content with thinking")
	}
	var pe ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Code != ValidationFailed {
		t.Errorf("expected ValidationFailed, got %s", pe.Code)
	}
	if pe.Suggestion == "" {
		t.Error("expected a suggestion in the error")
	}
}

func newOllamaStreamingTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		// Ollama sends full accumulated content, not deltas
		line1 := `{"model":"ollama-model","message":{"role":"assistant","content":"Hel"},"done":false}`
		_, _ = fmt.Fprintln(w, line1)
		flusher.Flush()

		line2 := `{"model":"ollama-model","message":{"role":"assistant","content":"Hello"},"done":false}`
		_, _ = fmt.Fprintln(w, line2)
		flusher.Flush()

		line3 := `{"model":"ollama-model","message":{"role":"assistant","content":"Hello world"},"done":false}`
		_, _ = fmt.Fprintln(w, line3)
		flusher.Flush()

		line4 := `{"model":"ollama-model","message":{"role":"assistant","content":"Hello world"},"done":true,"prompt_eval_count":5,"eval_count":3}`
		_, _ = fmt.Fprintln(w, line4)
		flusher.Flush()
	}))
}

func TestOllamaGenerateStream(t *testing.T) {
	server := newOllamaStreamingTestServer(t)
	defer server.Close()

	adapter := newOllamaAdapter(t, server.URL)
	chunkCh, errCh, setupErr := adapter.GenerateStream(context.Background(), api.ChatCompletionRequest{
		Model: "ollama-model",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if setupErr != nil {
		t.Fatalf("unexpected setup error: %v", setupErr)
	}

	var chunks []api.ChatCompletionChunk
	for chunk := range chunkCh {
		chunks = append(chunks, chunk)
	}

	termErr := <-errCh
	if termErr != nil {
		t.Fatalf("unexpected terminal error: %v", termErr)
	}

	// Ollama sends full content, we diff: "Hel" -> "Hello" -> "Hello world"
	// So deltas should be: "Hel", "lo", " world"
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Content != "Hel" {
		t.Fatalf("expected first delta 'Hel', got %q", chunks[0].Choices[0].Delta.Content)
	}
	if chunks[1].Choices[0].Delta.Content != "lo" {
		t.Fatalf("expected second delta 'lo', got %q", chunks[1].Choices[0].Delta.Content)
	}
	if chunks[2].Choices[0].Delta.Content != " world" {
		t.Fatalf("expected third delta ' world', got %q", chunks[2].Choices[0].Delta.Content)
	}
	// Final chunk (done:true) should have empty delta and finish_reason
	if chunks[3].Choices[0].Delta.Content != "" {
		t.Fatalf("expected final chunk empty delta, got %q", chunks[3].Choices[0].Delta.Content)
	}
	if chunks[3].Choices[0].FinishReason == nil || *chunks[3].Choices[0].FinishReason != "stop" {
		t.Fatalf("expected final chunk finish_reason 'stop', got %v", chunks[3].Choices[0].FinishReason)
	}
}
