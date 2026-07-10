package provider

import (
	"context"
	"encoding/json"
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
			resp := ollamaChatResponse{
				Model: req.Model,
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "hello from ollama",
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
