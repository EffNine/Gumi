package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/provider"
)

type fakeAdapter struct {
	name         string
	models       []provider.ModelInfo
	response     *api.ChatCompletionResponse
	err          error
	status       provider.ProviderStatus
	seenModel    string
	seenMessages []api.Message
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Type() string { return "fake" }

func (f *fakeAdapter) HealthCheck(ctx context.Context) (provider.ProviderStatus, error) {
	if f.status == "" {
		return provider.StatusOK, nil
	}
	return f.status, nil
}

func (f *fakeAdapter) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	if len(f.models) == 0 {
		return []provider.ModelInfo{{Name: "fake-model", Provider: f.name, CreatedAt: time.Now()}}, nil
	}
	return f.models, nil
}

func (f *fakeAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	f.seenModel = req.Model
	f.seenMessages = append([]api.Message(nil), req.Messages...)
	if f.err != nil {
		return nil, f.err
	}
	if f.response != nil {
		return f.response, nil
	}
	return &api.ChatCompletionResponse{
		ID:      "chatcmpl_fake",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.Choice{{
			Index: 0,
			Message: api.Message{
				Role:    "assistant",
				Content: "hello from fake provider",
			},
			FinishReason: "stop",
		}},
		Usage: api.Usage{},
	}, nil
}

func (f *fakeAdapter) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (f *fakeAdapter) NormalizeError(err error) provider.ProviderError {
	if pe, ok := err.(provider.ProviderError); ok {
		return pe
	}
	return provider.ProviderError{Code: provider.ProviderUnknownError, Message: err.Error()}
}

func testEngine(adapter provider.ProviderAdapter, cfg *config.Config) *Engine {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	mgr := provider.NewManager(map[string]provider.ProviderAdapter{"ollama": adapter}, logger.New("error"))
	return New(cfg, mgr, logger.New("error"))
}

func TestRunChatCompletionDirectMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeDirect)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_direct", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeDirect {
		t.Fatalf("expected direct mode, got %s", result.Context.RuntimeMode)
	}
	if adapter.seenModel != "fake-model" {
		t.Fatalf("expected provider model fake-model, got %s", adapter.seenModel)
	}
	assertEvent(t, result.Context, "direct_mode_selected")
	assertEvent(t, result.Context, "provider_selected")
	assertEvent(t, result.Context, "pipeline_completed")
}

func TestRunChatCompletionStabilizedBuildsContextAndPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeStabilized)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_stable", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	assertEvent(t, result.Context, "context_prepared")
	assertEvent(t, result.Context, "prompt_built")
	if result.Context.ContextPackage == nil {
		t.Fatal("expected context package")
	}
	if result.Context.PromptPackage == nil {
		t.Fatal("expected prompt package")
	}
	if len(adapter.seenMessages) == 0 || adapter.seenMessages[0].Role != "system" {
		t.Fatalf("expected provider request to include built system prompt, got %#v", adapter.seenMessages)
	}
}

func TestRunChatCompletionStructuredModeFromResponseFormat(t *testing.T) {
	engine := testEngine(&fakeAdapter{name: "ollama"}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_structured", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return JSON",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeStructured {
		t.Fatalf("expected structured mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "structured_mode_skeleton")
	assertEvent(t, result.Context, "structured_prompt_applied")
}

func TestRunChatCompletionStreamingUnsupported(t *testing.T) {
	engine := testEngine(&fakeAdapter{name: "ollama"}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_stream", api.ChatCompletionRequest{
		Model:  "local:auto",
		Stream: true,
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != provider.StreamingUnsupported {
		t.Fatalf("expected streaming unsupported, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestRunChatCompletionProviderFailure(t *testing.T) {
	engine := testEngine(&fakeAdapter{
		name: "ollama",
		err: provider.ProviderError{
			Code:       provider.ProviderUnavailable,
			Message:    "provider is down",
			Suggestion: "start provider",
		},
	}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_fail", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != provider.ProviderUnavailable {
		t.Fatalf("expected provider unavailable, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestRunChatCompletionIncludesMetadataWhenRequested(t *testing.T) {
	engine := testEngine(&fakeAdapter{name: "ollama"}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_meta", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
		Novexa: &api.NovexaExtensions{
			Telemetry: &api.TelemetryExtension{IncludeMetadata: true},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Response.Novexa == nil {
		t.Fatal("expected Novexa metadata")
	}
	if result.Response.Novexa.RequestID != "req_meta" {
		t.Fatalf("expected request ID req_meta, got %s", result.Response.Novexa.RequestID)
	}
	if result.Response.Novexa.Provider != "ollama" {
		t.Fatalf("expected provider ollama, got %s", result.Response.Novexa.Provider)
	}
}

func assertEvent(t *testing.T, ctx *Context, event string) {
	t.Helper()
	for _, item := range ctx.Events {
		if item.Event == event {
			return
		}
	}
	t.Fatalf("expected event %q in %#v", event, ctx.Events)
}
