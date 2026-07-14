package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
	"github.com/EffNine/gumi/runtime/internal/provider"
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

func (f *fakeAdapter) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.ChatCompletionChunk, <-chan error, error) {
	f.callCount++
	f.seenModel = req.Model
	f.seenMessages = append([]api.Message(nil), req.Messages...)
	f.seenReq = req
	if f.err != nil {
		return nil, nil, f.err
	}
	chunkCh := make(chan api.ChatCompletionChunk, 10)
	errCh := make(chan error, 1)
	go func() {
		defer close(chunkCh)
		defer close(errCh)
		chunkCh <- api.ChatCompletionChunk{
			ID:      "chatcmpl_fake_stream",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []api.ChunkChoice{{
				Index: 0,
				Delta: api.Message{Role: "assistant", Content: "hello"},
			}},
		}
		chunkCh <- api.ChatCompletionChunk{
			ID:      "chatcmpl_fake_stream",
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []api.ChunkChoice{{
				Index:        0,
				Delta:        api.Message{Role: "assistant", Content: " from fake"},
				FinishReason: strPtr("stop"),
			}},
		}
		errCh <- nil
	}()
	return chunkCh, errCh, nil
}

func (f *fakeAdapter) Capabilities() provider.Capabilities {
	return provider.Capabilities{Streaming: true}
}

func strPtr(s string) *string { return &s }

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

func TestRunChatCompletionStreamDirect(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeDirect)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan StreamResult, 1)
	go func() {
		done <- engine.RunChatCompletionStream(context.Background(), "req_stream_direct", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
	}()
	// Drain chunks
	for range chunkCh {
	}
	result := <-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeDirect {
		t.Fatalf("expected direct mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "streaming_mode_selected")
	assertEvent(t, result.Context, "direct_mode_selected")
	assertEvent(t, result.Context, "provider_selected")
	assertEvent(t, result.Context, "pipeline_completed")
}

func TestRunChatCompletionStreamLightweight(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan StreamResult, 1)
	go func() {
		done <- engine.RunChatCompletionStream(context.Background(), "req_stream_light", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
	}()
	for range chunkCh {
	}
	result := <-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeLightweight {
		t.Fatalf("expected lightweight mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "streaming_mode_selected")
	assertEvent(t, result.Context, "lightweight_mode_selected")
	assertEvent(t, result.Context, "lightweight_prompt_built")
	assertEvent(t, result.Context, "pipeline_completed")
}

func TestRunChatCompletionStreamStabilized(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeStabilized)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan StreamResult, 1)
	go func() {
		done <- engine.RunChatCompletionStream(context.Background(), "req_stream_stable", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
	}()
	for range chunkCh {
	}
	result := <-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeStabilized {
		t.Fatalf("expected stabilized mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "streaming_mode_selected")
	assertEvent(t, result.Context, "context_prepared")
	assertEvent(t, result.Context, "prompt_built")
	assertEvent(t, result.Context, "pipeline_completed")
}

func TestRunChatCompletionStreamStructuredRejected(t *testing.T) {
	engine := testEngine(&fakeAdapter{name: "ollama"}, config.DefaultConfig())

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	result := engine.RunChatCompletionStream(context.Background(), "req_stream_struct", api.ChatCompletionRequest{
		Model:  "local:auto",
		Stream: true,
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return JSON",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	}, chunkCh)

	if result.Error.Code != provider.StreamingUnsupported {
		t.Fatalf("expected streaming unsupported, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "streaming_structured_rejected")
}

func TestRunChatCompletionStreamStripsReasoningIncrementally(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeDirect)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan StreamResult, 1)
	go func() {
		done <- engine.RunChatCompletionStream(context.Background(), "req_stream_reason", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
	}()
	for range chunkCh {
	}
	result := <-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	assertEvent(t, result.Context, "pipeline_completed")
}

func TestRunChatCompletionStreamChunksForwarded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeDirect)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan struct{}, 1)
	var result StreamResult

	go func() {
		result = engine.RunChatCompletionStream(context.Background(), "req_stream_chunks", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
		close(done)
	}()

	var chunks []api.ChatCompletionChunk
	for chunk := range chunkCh {
		chunks = append(chunks, chunk)
	}
	<-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	// Verify chunks have content
	hasContent := false
	for _, c := range chunks {
		for _, choice := range c.Choices {
			if s, ok := choice.Delta.Content.(string); ok && s != "" {
				hasContent = true
			}
		}
	}
	if !hasContent {
		t.Fatal("expected at least one chunk with non-empty content")
	}
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
		Gumi: &api.GumiExtensions{
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
	if result.Response.Gumi == nil || !result.Response.Gumi.RepairApplied {
		t.Fatal("expected Gumi metadata repair_applied=true")
	}
	assertEvent(t, result.Context, "repair_completed")
	assertEvent(t, result.Context, "validation_completed_after_repair")
}

func TestRunChatCompletionRetriesInvalidStructuredOutput(t *testing.T) {
	adapter := &fakeAdapter{
		name: "ollama",
		responses: []*api.ChatCompletionResponse{
			response("not json"),
			response("{\"ok\":true}"),
		},
	}
	engine := testEngine(adapter, config.DefaultConfig())

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

	// Regression: retry must merge with the existing system prompt, not prepend a
	// second system message. Providers such as LM Studio reject multiple leading
	// system messages with "System message must be at the beginning".
	if len(adapter.seenMessages) == 0 {
		t.Fatal("expected provider request to have messages")
	}
	if adapter.seenMessages[0].Role != "system" {
		t.Fatalf("expected first message to be system, got %s", adapter.seenMessages[0].Role)
	}
	systemCount := 0
	for _, m := range adapter.seenMessages {
		if m.Role == "system" {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Fatalf("expected exactly one system message on retry, got %d: %#v", systemCount, adapter.seenMessages)
	}
	system, _ := adapter.seenMessages[0].Content.(string)
	if !strings.Contains(system, "Retry because the previous output failed validation") {
		t.Fatalf("expected retry instruction merged into system prompt, got %q", system)
	}
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
		Gumi: &api.GumiExtensions{
			Telemetry: &api.TelemetryExtension{IncludeMetadata: true},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Response.Gumi == nil {
		t.Fatal("expected Gumi metadata")
	}
	if result.Response.Gumi.RequestID != "req_meta" {
		t.Fatalf("expected request ID req_meta, got %s", result.Response.Gumi.RequestID)
	}
	if result.Response.Gumi.Provider != "ollama" {
		t.Fatalf("expected provider ollama, got %s", result.Response.Gumi.Provider)
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
	if result.Context.NormalizedRequest.Gumi == nil || result.Context.NormalizedRequest.Gumi.Thinking == nil || result.Context.NormalizedRequest.Gumi.Thinking.Enabled == nil {
		t.Fatal("expected thinking to be resolved from profile default")
	}
	if *result.Context.NormalizedRequest.Gumi.Thinking.Enabled {
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
		Gumi: &api.GumiExtensions{
			Thinking: &api.ThinkingConfig{Enabled: &trueVal},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.NormalizedRequest.Gumi == nil || result.Context.NormalizedRequest.Gumi.Thinking == nil || result.Context.NormalizedRequest.Gumi.Thinking.Enabled == nil {
		t.Fatal("expected thinking to be set")
	}
	if !*result.Context.NormalizedRequest.Gumi.Thinking.Enabled {
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
	if result.Context.NormalizedRequest.Gumi != nil && result.Context.NormalizedRequest.Gumi.Thinking != nil && result.Context.NormalizedRequest.Gumi.Thinking.Enabled != nil {
		t.Fatal("expected no thinking default for generic profile")
	}
}
func TestRunChatCompletionLightweightMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeLightweight {
		t.Fatalf("expected lightweight mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "lightweight_mode_selected")
	assertEvent(t, result.Context, "context_skipped")
	assertEvent(t, result.Context, "memory_skipped")
	assertEvent(t, result.Context, "session_skipped")
	assertEvent(t, result.Context, "lightweight_prompt_built")
	assertEvent(t, result.Context, "lightweight_guard_completed")
	assertEvent(t, result.Context, "validation_skipped")
	assertEvent(t, result.Context, "repair_skipped")
	if result.Context.ContextCompressed {
		t.Fatal("expected context compression to be skipped")
	}
	if result.Context.PromptPackage == nil {
		t.Fatal("expected prompt package")
	}
}

func TestRunChatCompletionLightweightAppliesProfileDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight_defaults", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
		Gumi: &api.GumiExtensions{Mode: string(ModeLightweight)},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	assertEvent(t, result.Context, "profile_defaults_applied")
	assertEvent(t, result.Context, "thinking_policy_applied")
	if adapter.seenReq.Temperature == nil {
		t.Fatal("expected profile temperature default to be applied")
	}
	if got := float64(*adapter.seenReq.Temperature); got < 0.29 || got > 0.31 {
		t.Fatalf("expected temperature ~0.3, got %v", got)
	}
	if adapter.seenReq.MaxTokens == nil || *adapter.seenReq.MaxTokens != 512 {
		t.Fatalf("expected max_tokens 512, got %v", adapter.seenReq.MaxTokens)
	}
	if result.Context.NormalizedRequest.Gumi == nil || result.Context.NormalizedRequest.Gumi.Thinking == nil || result.Context.NormalizedRequest.Gumi.Thinking.Enabled == nil {
		t.Fatal("expected thinking default resolved from profile")
	}
	if result.Context.ThinkingTelemetry == nil || result.Context.ThinkingTelemetry.ThinkingEnabled != "false" {
		t.Fatalf("expected thinking telemetry to report false, got %v", result.Context.ThinkingTelemetry)
	}
	if *result.Context.NormalizedRequest.Gumi.Thinking.Enabled {
		t.Fatal("expected thinking to be disabled from qwen3.5-2b profile")
	}
}

func TestRunChatCompletionLightweightPreservesAppSystemPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight_system", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{
			{Role: "developer", Content: "App system prompt: use early returns."},
			{Role: "user", Content: "Refactor this."},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if len(adapter.seenMessages) != 2 {
		t.Fatalf("expected 2 messages preserved, got %d: %#v", len(adapter.seenMessages), adapter.seenMessages)
	}
	if adapter.seenMessages[0].Role != "developer" {
		t.Fatalf("expected first message preserved as developer, got %s", adapter.seenMessages[0].Role)
	}
	content, _ := adapter.seenMessages[0].Content.(string)
	if !strings.HasPrefix(content, "App system prompt: use early returns.") {
		t.Fatalf("expected app system prompt preserved, got %q", content)
	}
	assertEvent(t, result.Context, "lightweight_prompt_built")
	if result.Context.PromptPackage == nil || result.Context.PromptPackage.SystemPrompt != "" {
		t.Fatal("expected no new system prompt to be added when app provides one")
	}
}

func TestRunChatCompletionLightweightSkipsContextCompression(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight_context", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.ContextCompressed {
		t.Fatal("expected context compression to be skipped in lightweight mode")
	}
	if result.Context.ContextPackage != nil {
		t.Fatal("expected no context package in lightweight mode")
	}
	assertEvent(t, result.Context, "context_skipped")
}

func TestRunChatCompletionLightweightJSONOnlyWithResponseFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{
		name:     "ollama",
		response: response(`{"ok":true}`),
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight_json", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return JSON",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
		Gumi: &api.GumiExtensions{
			Mode:       string(ModeLightweight),
			Validation: &api.ValidationConfig{Enabled: true, Repair: true},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	assertEvent(t, result.Context, "lightweight_format_instruction_added")
	if len(adapter.seenMessages) == 0 || adapter.seenMessages[0].Role != "system" {
		t.Fatalf("expected system message with JSON instruction, got %#v", adapter.seenMessages)
	}
	system, _ := adapter.seenMessages[0].Content.(string)
	if !strings.Contains(system, "Return a valid JSON object") {
		t.Fatalf("expected JSON instruction in system prompt, got %q", system)
	}
}

func TestRunChatCompletionLightweightNoJSONInstructionWithoutResponseFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_lightweight_no_json", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "What is 2+2?",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	for _, ev := range result.Context.Events {
		if ev.Event == "lightweight_format_instruction_added" {
			t.Fatal("expected no JSON instruction event when response_format is absent")
		}
	}
	if len(adapter.seenMessages) == 0 || adapter.seenMessages[0].Role != "system" {
		t.Fatalf("expected minimal system prompt, got %#v", adapter.seenMessages)
	}
	system, _ := adapter.seenMessages[0].Content.(string)
	if strings.Contains(system, "JSON") {
		t.Fatalf("expected no JSON instruction without response_format, got %q", system)
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

func TestPipelineManagedThinkingRequestOverride(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeStabilized)
	adapter := &fakeAdapter{name: "ollama", models: []provider.ModelInfo{{Name: "qwen3.5:2b"}}}
	engine := testEngine(adapter, cfg)

	trueVal := true
	result := engine.RunChatCompletion(context.Background(), "req_think_on", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Explain the trade-offs of using a mutex versus a channel in Go.",
		}},
		Gumi: &api.GumiExtensions{
			Mode:     string(ModeStabilized),
			Thinking: &api.ThinkingConfig{Enabled: &trueVal},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.NormalizedRequest.Gumi == nil || result.Context.NormalizedRequest.Gumi.Thinking == nil || result.Context.NormalizedRequest.Gumi.Thinking.Enabled == nil {
		t.Fatal("expected thinking enabled from request override")
	}
	if !*result.Context.NormalizedRequest.Gumi.Thinking.Enabled {
		t.Fatal("expected thinking enabled")
	}
	if result.Context.ThinkingMode != "full" {
		t.Fatalf("expected thinking mode full, got %q", result.Context.ThinkingMode)
	}
}

func TestPipelineManagedThinkingDisabledForJSONFormat(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeStabilized)
	adapter := &fakeAdapter{
		name:     "ollama",
		models:   []provider.ModelInfo{{Name: "qwen2.5-coder:7b"}},
		response: response(`{"ok":true}`),
	}
	engine := testEngine(adapter, cfg)

	trueVal := true
	result := engine.RunChatCompletion(context.Background(), "req_think_json", api.ChatCompletionRequest{
		Model: "ollama:qwen2.5-coder:7b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Return a JSON object.",
		}},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
		Gumi: &api.GumiExtensions{
			Mode:     string(ModeStabilized),
			Thinking: &api.ThinkingConfig{Enabled: &trueVal},
		},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.NormalizedRequest.Gumi == nil || result.Context.NormalizedRequest.Gumi.Thinking == nil || result.Context.NormalizedRequest.Gumi.Thinking.Enabled == nil {
		t.Fatal("expected thinking decision in normalized request")
	}
	if *result.Context.NormalizedRequest.Gumi.Thinking.Enabled {
		t.Fatal("expected thinking disabled for JSON format by policy")
	}
	if result.Context.ThinkingMode != "disabled" {
		t.Fatalf("expected thinking mode disabled, got %q", result.Context.ThinkingMode)
	}
}

func TestPipelineStripsReasoningFromResponse(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeStabilized)
	adapter := &fakeAdapter{
		name:     "ollama",
		response: response("```thinking\nI should reason step by step.\n```\nThe answer is 42."),
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_strip_reasoning", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "What is the answer?",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	content, _ := result.Response.Choices[0].Message.Content.(string)
	if strings.Contains(content, "thinking") || strings.Contains(content, "reason step by step") {
		t.Fatalf("expected reasoning stripped, got %q", content)
	}
	if !strings.Contains(content, "The answer is 42.") {
		t.Fatalf("expected final answer preserved, got %q", content)
	}
	if !result.Context.ReasoningContentPresent {
		t.Fatal("expected reasoning content present metadata")
	}
	assertEvent(t, result.Context, "reasoning_content_detected")
}

func TestRunChatCompletionToolShimForWeakModels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeLightweight)
	adapter := &fakeAdapter{
		name:     "ollama",
		models:   []provider.ModelInfo{{Name: "qwen3.5:2b"}},
		response: response(`{"tool": "read_file", "arguments": {"path": "main.go"}}`),
	}
	engine := testEngine(adapter, cfg)

	tools := []api.Tool{{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "read_file",
			Description: "Read a file",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"path"},
			},
		},
	}}

	result := engine.RunChatCompletion(context.Background(), "req_tool_shim", api.ChatCompletionRequest{
		Model:    "ollama:qwen3.5:2b",
		Messages: []api.Message{{Role: "user", Content: "Read main.go"}},
		Tools:    tools,
		Gumi:     &api.GumiExtensions{Mode: string(ModeLightweight)},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if adapter.seenReq.Tools != nil && len(adapter.seenReq.Tools) != 0 {
		t.Fatalf("expected native tools cleared for weak model, got %v", adapter.seenReq.Tools)
	}
	if len(adapter.seenMessages) == 0 {
		t.Fatal("expected provider request to have messages")
	}
	system, _ := adapter.seenMessages[0].Content.(string)
	if !strings.Contains(system, "read_file") || !strings.Contains(system, "JSON object") {
		t.Fatalf("expected tool instructions in system prompt, got %q", system)
	}
	if len(result.Response.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 parsed tool call, got %v", result.Response.Choices[0].Message.ToolCalls)
	}
	if result.Response.Choices[0].Message.ToolCalls[0].Function.Name != "read_file" {
		t.Fatalf("expected tool name read_file, got %q", result.Response.Choices[0].Message.ToolCalls[0].Function.Name)
	}
	assertEvent(t, result.Context, "tool_shim_enabled")
	assertEvent(t, result.Context, "tool_shim_parsed")
}

func TestAgentModeStepLimitExceeded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	messages := make([]api.Message, 0, 35)
	for i := 0; i < 35; i++ {
		messages = append(messages, api.Message{Role: "assistant", Content: "step"})
	}
	messages = append(messages, api.Message{Role: "user", Content: "continue"})

	result := engine.RunChatCompletion(context.Background(), "req_agent_step_limit", api.ChatCompletionRequest{
		Model:    "local:auto",
		Messages: messages,
	})

	if string(result.Error.Code) != "AGENT_STEP_LIMIT_EXCEEDED" {
		t.Fatalf("expected AGENT_STEP_LIMIT_EXCEEDED, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "agent_mode_selected")
	assertEvent(t, result.Context, "agent_step_check")
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestAgentModeAllowsNormalRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_agent_normal", api.ChatCompletionRequest{
		Model: "local:auto",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeAgent {
		t.Fatalf("expected agent mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "agent_mode_selected")
	// Thinking defaults to disabled in agent mode when no profile policy exists.
	if result.Context.ThinkingMode != "disabled" {
		t.Fatalf("expected thinking disabled, got %s", result.Context.ThinkingMode)
	}
	if result.Context.ThinkingDecisionReason != "no_default" {
		t.Fatalf("expected thinking decision reason 'no_default', got %s", result.Context.ThinkingDecisionReason)
	}
	assertEvent(t, result.Context, "agent_guard_completed")
	assertEvent(t, result.Context, "agent_completed")
}

func TestAgentModeForcesThinkingOff(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_agent_think", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	// The qwen3.5-2b profile has Defaults.Thinking=false, so thinking is
	// disabled via profile default, not forced disable.
	if result.Context.ThinkingMode != "disabled" {
		t.Fatalf("expected thinking disabled in agent mode, got %s", result.Context.ThinkingMode)
	}
	if result.Context.ThinkingDecisionReason != "profile_default_legacy_disabled" {
		t.Fatalf("expected thinking decision reason 'profile_default_legacy_disabled', got %s", result.Context.ThinkingDecisionReason)
	}
}

func TestAgentModeDetectsRepeatedToolCallStrict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	messages := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}

	result := engine.RunChatCompletion(context.Background(), "req_agent_loop", api.ChatCompletionRequest{
		Model:    "local:auto",
		Messages: messages,
	})

	if string(result.Error.Code) != "AGENT_TOOL_CALL_LOOP" {
		t.Fatalf("expected AGENT_TOOL_CALL_LOOP, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "agent_tool_loop_check")
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestAgentModeStreaming(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	done := make(chan StreamResult, 1)
	go func() {
		done <- engine.RunChatCompletionStream(context.Background(), "req_agent_stream", api.ChatCompletionRequest{
			Model:  "local:auto",
			Stream: true,
			Messages: []api.Message{{
				Role:    "user",
				Content: "Hello",
			}},
		}, chunkCh)
	}()
	for range chunkCh {
	}
	result := <-done

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	if result.Context.RuntimeMode != ModeAgent {
		t.Fatalf("expected agent mode, got %s", result.Context.RuntimeMode)
	}
	assertEvent(t, result.Context, "agent_mode_selected")
	// Thinking defaults to disabled in agent mode when using generic profile.
	if result.Context.ThinkingMode != "disabled" {
		t.Fatalf("expected thinking disabled, got %s", result.Context.ThinkingMode)
	}
	assertEvent(t, result.Context, "streaming_agent_completed")
}

func TestAgentModeStreamingStepLimitExceeded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{name: "ollama"}
	engine := testEngine(adapter, cfg)

	messages := make([]api.Message, 0, 35)
	for i := 0; i < 35; i++ {
		messages = append(messages, api.Message{Role: "assistant", Content: "step"})
	}
	messages = append(messages, api.Message{Role: "user", Content: "continue"})

	chunkCh := make(chan api.ChatCompletionChunk, 10)
	result := engine.RunChatCompletionStream(context.Background(), "req_agent_stream_limit", api.ChatCompletionRequest{
		Model:    "local:auto",
		Stream:   true,
		Messages: messages,
	}, chunkCh)

	if string(result.Error.Code) != "AGENT_STEP_LIMIT_EXCEEDED" {
		t.Fatalf("expected AGENT_STEP_LIMIT_EXCEEDED, got %s", result.Error.Code)
	}
	assertEvent(t, result.Context, "agent_step_check")
	assertEvent(t, result.Context, "pipeline_failed")
}

func TestAgentModeAppliesProfileDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Runtime.Mode = string(ModeAgent)
	adapter := &fakeAdapter{
		name:   "ollama",
		models: []provider.ModelInfo{{Name: "qwen3.5:2b"}},
	}
	engine := testEngine(adapter, cfg)

	result := engine.RunChatCompletion(context.Background(), "req_agent_profile", api.ChatCompletionRequest{
		Model: "ollama:qwen3.5:2b",
		Messages: []api.Message{{
			Role:    "user",
			Content: "Hello",
		}},
	})

	if result.Error.Code != "" {
		t.Fatalf("unexpected pipeline error: %v", result.Error)
	}
	assertEvent(t, result.Context, "profile_defaults_applied")
	if adapter.seenReq.Temperature == nil {
		t.Fatal("expected profile temperature default to be applied")
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
