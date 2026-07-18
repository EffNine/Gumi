package provider

import (
	"context"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
)

// fakeStreamingAdapter implements ProviderAdapter with streaming support for tests.
type fakeStreamingAdapter struct {
	name     string
	chunks   []api.ChatCompletionChunk
	setupErr error
}

func (f *fakeStreamingAdapter) Name() string { return f.name }
func (f *fakeStreamingAdapter) Type() string { return "fake-stream" }
func (f *fakeStreamingAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
	return StatusOK, nil
}
func (f *fakeStreamingAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{{Name: "fake-stream-model", Provider: f.name}}, nil
}
func (f *fakeStreamingAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	return &api.ChatCompletionResponse{ID: "chatcmpl-fake", Object: "chat.completion", Created: time.Now().Unix(), Model: req.Model, Choices: []api.Choice{{Index: 0, Message: api.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}}}, nil
}
func (f *fakeStreamingAdapter) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.ChatCompletionChunk, <-chan error, error) {
	if f.setupErr != nil {
		return nil, nil, f.setupErr
	}
	chunkCh := make(chan api.ChatCompletionChunk, len(f.chunks))
	errCh := make(chan error, 1)
	go func() {
		defer close(chunkCh)
		defer close(errCh)
		for _, c := range f.chunks {
			chunkCh <- c
		}
		errCh <- nil
	}()
	return chunkCh, errCh, nil
}
func (f *fakeStreamingAdapter) Capabilities() Capabilities {
	return Capabilities{Streaming: true}
}
func strPtr(s string) *string { return &s }

func (f *fakeStreamingAdapter) NormalizeError(err error) ProviderError {
	if pe, ok := err.(ProviderError); ok {
		return pe
	}
	return ProviderError{Code: ProviderUnknownError, Message: err.Error()}
}

func TestManagerResolveRegistryAlias(t *testing.T) {
	adapter := &fakeStreamingAdapter{name: "ollama"}
	mgr := NewManager(map[string]ProviderAdapter{"ollama": adapter}, logger.New("error"))
	mgr.SetRegistry([]config.ModelRegistryEntry{
		{Alias: "my-alias", Provider: "ollama", ModelID: "fake-stream-model", Enabled: true},
	})

	res, perr := mgr.ResolveModel(context.Background(), "my-alias")
	if perr.Code != "" {
		t.Fatalf("unexpected error resolving alias: %v", perr)
	}
	if res.ProviderKey != "ollama" {
		t.Fatalf("expected provider ollama, got %s", res.ProviderKey)
	}
	if res.ModelName != "fake-stream-model" {
		t.Fatalf("expected model fake-stream-model, got %s", res.ModelName)
	}
}

func TestManagerResolveRegistryDefault(t *testing.T) {
	adapter := &fakeStreamingAdapter{name: "ollama"}
	mgr := NewManager(map[string]ProviderAdapter{"ollama": adapter}, logger.New("error"))
	mgr.SetRegistry([]config.ModelRegistryEntry{
		{Alias: "default-model", Provider: "ollama", ModelID: "fake-stream-model", Enabled: true, Default: true},
	})

	res, perr := mgr.ResolveModel(context.Background(), "")
	if perr.Code != "" {
		t.Fatalf("unexpected error resolving default: %v", perr)
	}
	if res.ModelName != "fake-stream-model" {
		t.Fatalf("expected default model fake-stream-model, got %s", res.ModelName)
	}
}

func TestManagerHealthCheckUnknownProvider(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	status, err := mgr.HealthCheck(context.Background(), "missing")
	if err == nil {
		t.Error("expected error for unknown provider")
	}
	if status != StatusUnknown {
		t.Errorf("expected status unknown, got %s", status)
	}
}

func TestManagerResolveAutoNoProviders(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	res, perr := mgr.ResolveModel(context.Background(), "local:auto")
	if res != nil {
		t.Error("expected no resolution")
	}
	if perr.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, perr.Code)
	}
}

func TestManagerResolveExplicitProviderNotConfigured(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, perr := mgr.ResolveModel(context.Background(), "ollama:llama3")
	if perr.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, perr.Code)
	}
}

func TestManagerResolveMalformedModelID(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, perr := mgr.ResolveModel(context.Background(), "notamodeLID")
	if perr.Code != ProviderMisconfigured {
		t.Errorf("expected %s, got %s", ProviderMisconfigured, perr.Code)
	}
}

func TestManagerGenerateEmptyModelID(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, _, perr := mgr.Generate(context.Background(), api.ChatCompletionRequest{Model: ""})
	if perr.Code != ProviderMisconfigured {
		t.Errorf("expected %s, got %s", ProviderMisconfigured, perr.Code)
	}
}

func TestManagerListModelsOffline(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	models := mgr.ListModels(context.Background())
	if len(models) != 0 {
		t.Errorf("expected no models when offline, got %d", len(models))
	}
}

func TestManagerGenerateStream(t *testing.T) {
	adapter := &fakeStreamingAdapter{
		name: "ollama",
		chunks: []api.ChatCompletionChunk{
			{ID: "chunk1", Object: "chat.completion.chunk", Created: 123, Model: "fake-stream-model", Choices: []api.ChunkChoice{{Index: 0, Delta: api.Message{Role: "assistant", Content: "Hello"}}}},
			{ID: "chunk2", Object: "chat.completion.chunk", Created: 123, Model: "fake-stream-model", Choices: []api.ChunkChoice{{Index: 0, Delta: api.Message{Role: "assistant", Content: " world"}, FinishReason: strPtr("stop")}}},
		},
	}
	mgr := NewManager(map[string]ProviderAdapter{"ollama": adapter}, logger.New("error"))

	chunkCh, providerKey, perr := mgr.GenerateStream(context.Background(), api.ChatCompletionRequest{
		Model:    "ollama:fake-stream-model",
		Messages: []api.Message{{Role: "user", Content: "hi"}},
	})
	if perr.Code != "" {
		t.Fatalf("unexpected error: %v", perr)
	}
	if providerKey != "ollama" {
		t.Fatalf("expected provider key ollama, got %s", providerKey)
	}

	var chunks []api.ChatCompletionChunk
	for chunk := range chunkCh {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Choices[0].Delta.Content != "Hello" {
		t.Fatalf("expected 'Hello', got %q", chunks[0].Choices[0].Delta.Content)
	}
}

func TestManagerGenerateStreamSetupError(t *testing.T) {
	adapter := &fakeStreamingAdapter{
		name:     "ollama",
		setupErr: ProviderError{Code: StreamingUnsupported, Message: "streaming not supported"},
	}
	mgr := NewManager(map[string]ProviderAdapter{"ollama": adapter}, logger.New("error"))

	_, _, perr := mgr.GenerateStream(context.Background(), api.ChatCompletionRequest{
		Model:    "ollama:fake-stream-model",
		Messages: []api.Message{{Role: "user", Content: "hi"}},
	})
	if perr.Code != StreamingUnsupported {
		t.Fatalf("expected StreamingUnsupported, got %s", perr.Code)
	}
}

func TestManagerGenerateStreamEmptyModelID(t *testing.T) {
	mgr := NewManager(map[string]ProviderAdapter{}, logger.New("error"))
	_, _, perr := mgr.GenerateStream(context.Background(), api.ChatCompletionRequest{Model: ""})
	if perr.Code != ProviderMisconfigured {
		t.Errorf("expected %s, got %s", ProviderMisconfigured, perr.Code)
	}
}
