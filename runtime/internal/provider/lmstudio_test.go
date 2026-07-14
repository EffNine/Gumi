package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

func newLMStudioTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			list := api.ModelsList{
				Object: "list",
				Data: []api.Model{
					{ID: "lmstudio-model", Object: "model", Created: time.Now().Unix(), OwnedBy: "lmstudio"},
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
				ID:      "chatcmpl-lmstudio-123",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []api.Choice{
					{
						Index:        0,
						Message:      api.Message{Role: "assistant", Content: "hello from lm studio"},
						FinishReason: "stop",
					},
				},
				Usage: api.Usage{PromptTokens: 1, CompletionTokens: 4, TotalTokens: 5},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func newLMStudioCaptureServer(t *testing.T, captured *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(captured); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := api.ChatCompletionResponse{
			ID:      "chatcmpl-lmstudio-capture",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "lmstudio-model",
			Choices: []api.Choice{
				{
					Index:        0,
					Message:      api.Message{Role: "assistant", Content: "ok"},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func newLMStudioAdapter(t *testing.T, serverURL string) *LMStudioAdapter {
	t.Helper()
	log := logger.New("error")
	adapter, err := NewLMStudioAdapter("lmstudio", config.ProviderSettings{URL: serverURL, TimeoutSeconds: 5}, log)
	if err != nil {
		t.Fatalf("failed to create lmstudio adapter: %v", err)
	}
	return adapter.(*LMStudioAdapter)
}

func TestLMStudioHealthCheck(t *testing.T) {
	server := newLMStudioTestServer(t)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	status, err := adapter.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}
	if status != StatusOK {
		t.Errorf("expected status ok, got %s", status)
	}
}

func TestLMStudioListModels(t *testing.T) {
	server := newLMStudioTestServer(t)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected list models error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "lmstudio-model" {
		t.Errorf("expected lmstudio-model, got %s", models[0].Name)
	}
}

func TestLMStudioGenerate(t *testing.T) {
	server := newLMStudioTestServer(t)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	resp, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "lmstudio-model",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	if resp.ID != "chatcmpl-lmstudio-123" {
		t.Errorf("unexpected id: %s", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "hello from lm studio" {
		t.Errorf("unexpected content: %v", resp.Choices[0].Message.Content)
	}
}

func TestLMStudioGenerateDisablesThinkingWithReasoningEffortNone(t *testing.T) {
	var captured map[string]interface{}
	server := newLMStudioCaptureServer(t, &captured)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	thinkingEnabled := false
	_, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "lmstudio-model",
		Messages: []api.Message{
			{Role: "user", Content: "What is 2+2?"},
		},
		Gumi: &api.GumiExtensions{
			Thinking: &api.ThinkingConfig{Enabled: &thinkingEnabled},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}

	if _, ok := captured["gumi"]; ok {
		t.Fatal("expected LM Studio payload to omit Gumi extension field")
	}
	if got := captured["reasoning_effort"]; got != "none" {
		t.Fatalf("expected reasoning_effort none, got %v", got)
	}
}

func TestLMStudioGenerateOmitsReasoningEffortWhenThinkingUnspecified(t *testing.T) {
	var captured map[string]interface{}
	server := newLMStudioCaptureServer(t, &captured)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	_, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "lmstudio-model",
		Messages: []api.Message{
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}

	if _, ok := captured["gumi"]; ok {
		t.Fatal("expected LM Studio payload to omit Gumi extension field")
	}
	if _, ok := captured["reasoning_effort"]; ok {
		t.Fatal("expected reasoning_effort to be omitted when thinking is unspecified")
	}
}

func TestLMStudioGenerateMapsJSONObjectResponseFormatToJSONSchema(t *testing.T) {
	var captured map[string]interface{}
	server := newLMStudioCaptureServer(t, &captured)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	_, err := adapter.Generate(context.Background(), api.ChatCompletionRequest{
		Model: "lmstudio-model",
		Messages: []api.Message{
			{Role: "user", Content: "return json"},
		},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}

	format, ok := captured["response_format"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected response_format object, got %v", captured["response_format"])
	}
	if got := format["type"]; got != "json_schema" {
		t.Fatalf("expected response_format.type json_schema, got %v", got)
	}
	if _, ok := format["json_schema"].(map[string]interface{}); !ok {
		t.Fatalf("expected response_format.json_schema object, got %v", format["json_schema"])
	}
}

func newLMStudioStreamingTestServer(t *testing.T) *httptest.Server {
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

		// First chunk
		chunk1 := `{"id":"chatcmpl-lmstudio-stream","object":"chat.completion.chunk","created":1234567890,"model":"lmstudio-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk1)
		flusher.Flush()

		// Second chunk
		chunk2 := `{"id":"chatcmpl-lmstudio-stream","object":"chat.completion.chunk","created":1234567890,"model":"lmstudio-model","choices":[{"index":0,"delta":{"role":"assistant","content":" world"},"finish_reason":null}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk2)
		flusher.Flush()

		// Final chunk with finish_reason
		chunk3 := `{"id":"chatcmpl-lmstudio-stream","object":"chat.completion.chunk","created":1234567890,"model":"lmstudio-model","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":"stop"}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk3)
		flusher.Flush()

		// Termination
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

func TestLMStudioGenerateStream(t *testing.T) {
	server := newLMStudioStreamingTestServer(t)
	defer server.Close()

	adapter := newLMStudioAdapter(t, server.URL)
	chunkCh, errCh, setupErr := adapter.GenerateStream(context.Background(), api.ChatCompletionRequest{
		Model: "lmstudio-model",
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

	// Check terminal error
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
