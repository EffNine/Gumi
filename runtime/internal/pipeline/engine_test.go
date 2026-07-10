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
	responses    []*api.ChatCompletionResponse
	err          error
	status       provider.ProviderStatus
	seenModel    string
	seenMessages []api.Message
	seenReq      api.ChatCompletionRequest
	callCount    int
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
	f.callCount++
	f.seenModel = req.Model
	f.seenMessages = append([]api.Message(nil), req.Messages...)
	f.seenReq = req
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) > 0 {
		idx := f.callCount - 1
		if idx >= len(f.responses) {
			idx = len(f.responses) - 1
		}
		return f.responses[idx], nil
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
	engine := testEngine(&fakeAdapter{name: "ollama", response: response(`{"ok":true}`)}, config.DefaultConfig())

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
	assertEvent(t, result.Context, "structured_output_guard_enabled")
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

func TestRunChatCompletionGuardBlocksEmptyPrompt(t *testing.T) {
	engine := testEngine(&fakeAdapter{name: "ollama"}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_empty", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "   ",
		}},
	})

	if result.Error.Code != provider.EmptyPrompt {
		t.Fatalf("expected empty prompt, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "guard_completed")
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestRunChatCompletionRepairsFencedJSON(t *testing.T) {
	engine := testEngine(&fakeAdapter{
		name:     "ollama",
		response: response("```json\n{\"ok\":true}\n```"),
	}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_json_repair", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return JSON",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
		Novexa: &api.NovexaExtensions{
			Telemetry: &api.TelemetryExtension{IncludeMetadata: true},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	content, _ := result.Response.Choices[0].Message.Content.(string)
	if content != "{\"ok\":true}" {
		t.Fatalf("expected repaired JSON, got %q", content)
	}
	if !result.Context.RepairApplied {
		t.Fatal("expected repair applied")
	}
	if result.Response.Novexa == nil || !result.Response.Novexa.RepairApplied {
		t.Fatal("expected Novexa metadata repair_applied=true")
	}
	assertEvent(t, result.Context, "repair_completed")
	assertEvent(t, result.Context, "validation_completed_after_repair")
}

func TestRunChatCompletionRetriesInvalidStructuredOutput(t *testing.T) {
	engine := testEngine(&fakeAdapter{
		name: "ollama",
		responses: []*api.ChatCompletionResponse{
			response("not json"),
			response("{\"ok\":true}"),
		},
	}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_retry", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return JSON",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Context.Retry.Attempt != 2 {
		t.Fatalf("expected second attempt, got %d", result.Context.Retry.Attempt)
	}
	assertEvent(t, result.Context, "retry_requested")
	assertEvent(t, result.Context, "validation_completed_after_retry")
}

func TestRunChatCompletionRepairsRepeatedLines(t *testing.T) {
	engine := testEngine(&fakeAdapter{
		name:     "ollama",
		response: response("same\nsame\nsame\nsame"),
	}, config.DefaultConfig())

	result := engine.RunChatCompletion(context.Background(), "req_repeat", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Say same twice",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	content, _ := result.Response.Choices[0].Message.Content.(string)
	if content != "same\nsame" {
		t.Fatalf("expected repeated output cleaned, got %q", content)
	}
	if !result.Context.RepairApplied {
		t.Fatal("expected repair applied")
	}
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

func TestPipelineAppliesProfileDefaultsToProviderRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3:8b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_qwen", api.ChatCompletionRequest{
		Model: "ollama:qwen3:8b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.ModelProfile == nil || result.Context.ModelProfile.ID != "qwen3-8b" {
		t.Fatalf("expected qwen3-8b profile, got %v", result.Context.ModelProfile)
	}
	assertEvent(t, result.Context, "model_profile_applied")
	assertEvent(t, result.Context, "profile_defaults_applied")
	if adapter.seenReq.Temperature == nil {
		t.Fatal("expected profile temperature default to be applied")
	}
	if got := float64(*adapter.seenReq.Temperature); got < 0.39 || got > 0.41 {
		t.Fatalf("expected temperature ~0.4, got %v", got)
	}
	if adapter.seenReq.TopP == nil {
		t.Fatal("expected profile top_p default to be applied")
	}
	if got := float64(*adapter.seenReq.TopP); got < 0.89 || got > 0.91 {
		t.Fatalf("expected top_p ~0.9, got %v", got)
	}
	if adapter.seenReq.MaxTokens == nil || *adapter.seenReq.MaxTokens != 4096 {
		t.Fatalf("expected max_tokens 4096, got %v", adapter.seenReq.MaxTokens)
	}
}

func TestPipelineEmitsProfileFallbackForUnknownModel(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "unknown-model"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_unknown", api.ChatCompletionRequest{
		Model: "ollama:unknown-model",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.ModelProfile == nil || result.Context.ModelProfile.ID != "generic-local" {
		t.Fatalf("expected generic-local fallback, got %v", result.Context.ModelProfile)
	}
	assertEvent(t, result.Context, "model_profile_fallback")
}

func TestPipelineAppliesProfileThinkingDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_think", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.ModelProfile == nil || result.Context.ModelProfile.ID != "qwen3.5-2b" {
		t.Fatalf("expected qwen3.5-2b profile, got %v", result.Context.ModelProfile)
	}
	// Profile defaults thinking to false; request should have it set.
	if result.Context.NormalizedRequest.Novexa == nil || result.Context.NormalizedRequest.Novexa.Thinking == nil || result.Context.NormalizedRequest.Novexa.Thinking.Enabled == nil {
		t.Fatal("expected thinking to be resolved from profile default")
	}
	if *result.Context.NormalizedRequest.Novexa.Thinking.Enabled {
		t.Fatal("expected thinking to be false from profile default")
	}
	assertEvent(t, result.Context, "profile_defaults_applied")
}

func TestPipelineRequestThinkingOverridesProfileDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	trueVal := true
	result := engine.RunChatCompletion(context.Background(), "req_think_override", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
		Novexa: &api.NovexaExtensions{
			Thinking: &api.ThinkingConfig{Enabled: &trueVal},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.NormalizedRequest.Novexa == nil || result.Context.NormalizedRequest.Novexa.Thinking == nil || result.Context.NormalizedRequest.Novexa.Thinking.Enabled == nil {
		t.Fatal("expected thinking to be set")
	}
	if !*result.Context.NormalizedRequest.Novexa.Thinking.Enabled {
		t.Fatal("expected thinking to be true from request override")
	}
}

func TestPipelineThinkingTelemetryRecorded(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_think_tele", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.ThinkingTelemetry == nil {
		t.Fatal("expected thinking telemetry")
	}
	if result.Context.ThinkingTelemetry.ThinkingEnabled != "false" {
		t.Fatalf("expected thinking_enabled 'false', got %q", result.Context.ThinkingTelemetry.ThinkingEnabled)
	}
}

func TestPipelineNoThinkingDefaultForGenericProfile(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "unknown-model"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_no_think", api.ChatCompletionRequest{
		Model: "ollama:unknown-model",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	// Generic profile has no thinking default; request should not have thinking set.
	if result.Context.NormalizedRequest.Novexa != nil && result.Context.NormalizedRequest.Novexa.Thinking != nil && result.Context.NormalizedRequest.Novexa.Thinking.Enabled != nil {
		t.Fatal("expected no thinking default for generic profile")
	}
}
func TestPipelineAppliesProfilePromptInstructions(t *testing.T) {
	cfg := config.DefaultConfig()
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3:8b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_prompt", api.ChatCompletionRequest{
		Model: "ollama:qwen3:8b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.PromptPackage == nil {
		t.Fatal("expected prompt package")
	}
	if len(result.Context.PromptPackage.ModelProfileInstructions) == 0 {
		t.Fatalf("expected profile instructions in prompt package, got %v", result.Context.PromptPackage.ModelProfileInstructions)
	}
	assertEvent(t, result.Context, "profile_prompt_applied")
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

func response(content string) *api.ChatCompletionResponse {
	return &api.ChatCompletionResponse{
		ID:      "chatcmpl_fake",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "fake-model",
		Choices: []api.Choice{{
			Index:        0,
			Message:      api.Message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
	}
}
