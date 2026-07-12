package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

func newOpenAICompatibleTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			list := api.ModelsList{
				Object: "list",
				Data: []api.Model{
					{ID: "local-model", Object: "model", Created: time.Now().Unix(), OwnedBy: "local"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
		case "/v1/chat/completions":
			var req api.ChatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resp := api.ChatCompletionResponse{
				ID:      "chatcmpl-local-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []api.Choice{
					{
						Index:        0,
						Message:      api.Message{Role: "assistant", Content: "hello from local openai-compatible server"},
						FinishReason: "stop",
					},
				},
				Usage: api.Usage{PromptTokens: 1, CompletionTokens: 5, TotalTokens: 6},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func newOpenAICompatibleAdapter(t *testing.T, serverURL string) *OpenAICompatibleAdapter {
	t.Helper()
	log := logger.New("error")
	adapter, err := NewOpenAICompatibleAdapter("openai_compatible_local", config.ProviderSettings{URL: serverURL, TimeoutSeconds: 5}, log)
	if err != nil {
		t.Fatalf("failed to create openai-compatible adapter: %v", err)
	}
	return adapter.(*OpenAICompatibleAdapter)
}

func TestOpenAICompatibleHealthCheck(t *testing.T) {
	server := newOpenAICompatibleTestServer(t)
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(t, server.URL)
	status, err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}
	if status != StatusOK {
		t.Errorf("expected status ok, got %s", status)
	}
}

func TestOpenAICompatibleListModels(t *testing.T) {
	server := newOpenAICompatibleTestServer(t)
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(t, server.URL)
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected list models error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "local-model" {
		t.Errorf("expected local-model, got %s", models[0].Name)
	}
}

func TestOpenAICompatibleGenerate(t *testing.T) {
	server := newOpenAICompatibleTestServer(t)
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(t, server.URL)
	resp, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "local-model",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if resp.ID != "chatcmpl-local-123" {
		t.Errorf("unexpected id: %s", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "hello from local openai-compatible server" {
		t.Errorf("unexpected content: %v", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 6 {
		t.Errorf("expected 6 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAICompatiblePathWithoutV1Suffix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.ModelsList{Object: "list", Data: []api.Model{}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(t, server.URL)
	_, err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("expected /v1/models path to be used: %v", err)
	}
}

func newOpenAICompatibleStreamingTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		chunk1 := `{"id":"chatcmpl-local-stream","object":"chat.completion.chunk","created":1234567890,"model":"local-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk1)
		flusher.Flush()

		chunk2 := `{"id":"chatcmpl-local-stream","object":"chat.completion.chunk","created":1234567890,"model":"local-model","choices":[{"index":0,"delta":{"role":"assistant","content":" world"},"finish_reason":null}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk2)
		flusher.Flush()

		chunk3 := `{"id":"chatcmpl-local-stream","object":"chat.completion.chunk","created":1234567890,"model":"local-model","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":"stop"}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk3)
		flusher.Flush()

		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

func TestOpenAICompatibleGenerateStream(t *testing.T) {
	server := newOpenAICompatibleStreamingTestServer(t)
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(t, server.URL)
	chunkCh, errCh, setupErr := adapter.GenerateStream(context.Background(), api.ChatCompletionRequest{
		Model: "local-model",
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

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Content != "Hello" {
		t.Fatalf("expected first delta 'Hello', got %q", chunks[0].Choices[0].Delta.Content)
	}
	if chunks[1].Choices[0].Delta.Content != " world" {
		t.Fatalf("expected second delta ' world', got %q", chunks[1].Choices[0].Delta.Content)
	}
	if chunks[2].Choices[0].FinishReason == nil || *chunks[2].Choices[0].FinishReason != "stop" {
		t.Fatalf("expected final chunk finish_reason 'stop', got %v", chunks[2].Choices[0].FinishReason)
	}
}
