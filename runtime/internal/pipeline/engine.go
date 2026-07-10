package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	"github.com/novexa/novexa/runtime/internal/logger"
	promptengine "github.com/novexa/novexa/runtime/internal/prompt"
	"github.com/novexa/novexa/runtime/internal/provider"
	"github.com/novexa/novexa/runtime/internal/telemetry"
)

const (
	defaultWorkspaceID = "default"
	defaultRetryMax    = 2
)

// Engine orchestrates the request lifecycle for chat completions.
type Engine struct {
	cfg           *config.Config
	manager       *provider.Manager
	log           *logger.Logger
	telemetry     *telemetry.Writer
	contextEngine *contextengine.Engine
	promptEngine  *promptengine.Engine
}

// Result is returned to Gateway Engine after pipeline execution.
type Result struct {
	Response     *api.ChatCompletionResponse
	Context      *Context
	ProviderName string
	Error        provider.ProviderError
}

// New creates a Pipeline Engine.
func New(cfg *config.Config, manager *provider.Manager, log *logger.Logger) *Engine {
	return &Engine{
		cfg:           cfg,
		manager:       manager,
		log:           log,
		contextEngine: contextengine.New(),
		promptEngine:  promptengine.New(),
	}
}

// SetTelemetry attaches a telemetry writer. The writer may be nil; the engine
// will simply skip telemetry recording.
func (e *Engine) SetTelemetry(t *telemetry.Writer) {
	e.telemetry = t
}

// RunChatCompletion executes a normalized chat completion request.
func (e *Engine) RunChatCompletion(ctx context.Context, requestID string, req api.ChatCompletionRequest) Result {
	pc := e.newContext(requestID, req)
	pc.AddEvent("pipeline", "request_received", SeverityInfo, "chat completion request received", nil)
	pc.AddEvent("pipeline", "pipeline_started", SeverityInfo, "pipeline execution started", map[string]string{
		"mode": string(pc.RuntimeMode),
	})

	if req.Stream {
		return e.fail(pc, provider.ProviderError{
			Code:       provider.StreamingUnsupported,
			Message:    "streaming chat completions are not supported in Sprint 4",
			Suggestion: "Set stream=false until streaming support is implemented.",
		}, "streaming is not supported by the current pipeline")
	}

	pc.AddEvent("workspace", "workspace_resolved", SeverityInfo, "workspace resolved", map[string]string{
		"workspace_id": pc.WorkspaceID,
	})
	pc.AddEvent("config", "config_resolved", SeverityInfo, "request config snapshot resolved", map[string]string{
		"runtime_mode": string(pc.RuntimeMode),
	})
	pc.AddEvent("session", "session_resolved", SeverityInfo, "session resolved", map[string]string{
		"session_id": pc.SessionID,
	})

	switch pc.RuntimeMode {
	case ModeDirect:
		return e.runDirect(ctx, pc)
	case ModeStabilized, ModeStructured:
		return e.runSkeleton(ctx, pc)
	case ModeAgent:
		return e.fail(pc, provider.ProviderError{
			Code:       provider.ProviderMisconfigured,
			Message:    "agent mode is reserved for a future Novexa release",
			Suggestion: "Use direct, stabilized, or structured mode.",
		}, "unsupported runtime mode")
	default:
		return e.fail(pc, provider.ProviderError{
			Code:       provider.ProviderMisconfigured,
			Message:    fmt.Sprintf("runtime mode %q is not supported", pc.RuntimeMode),
			Suggestion: "Use direct, stabilized, or structured mode.",
		}, "unsupported runtime mode")
	}
}

func (e *Engine) newContext(requestID string, req api.ChatCompletionRequest) *Context {
	mode := RuntimeMode(e.cfg.Runtime.Mode)
	if req.ResponseFormat != nil && (req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json_schema") {
		mode = ModeStructured
	}
	if req.Novexa != nil && req.Novexa.Mode != "" {
		mode = RuntimeMode(req.Novexa.Mode)
	}

	sessionID := ""
	if req.Novexa != nil && req.Novexa.Session != nil {
		sessionID = req.Novexa.Session.ID
	}
	if sessionID == "" {
		sessionID = "ephemeral:" + requestID
	}

	return &Context{
		RequestID:         requestID,
		TraceID:           requestID,
		WorkspaceID:       defaultWorkspaceID,
		SessionID:         sessionID,
		RuntimeMode:       mode,
		Stream:            req.Stream,
		IncomingRequest:   req,
		NormalizedRequest: req,
		ConfigSnapshot:    e.cfg,
		MessagesOriginal:  append([]api.Message(nil), req.Messages...),
		RequestedModel:    req.Model,
		Retry: RetryState{
			Attempt:     1,
			MaxAttempts: defaultRetryMax,
		},
	}
}

func (e *Engine) runDirect(ctx context.Context, pc *Context) Result {
	pc.AddEvent("pipeline", "direct_mode_selected", SeverityInfo, "direct mode selected", nil)
	return e.callProvider(ctx, pc)
}

func (e *Engine) runSkeleton(ctx context.Context, pc *Context) Result {
	pc.AddEvent("profile", "model_profile_skipped", SeverityInfo, "model profile resolution is scheduled for Sprint 8", nil)
	if pc.RuntimeMode == ModeStructured {
		pc.AddEvent("guard", "structured_mode_skeleton", SeverityInfo, "Structured mode skeleton active; validation and repair start in Sprint 7", nil)
	}
	e.prepareContext(pc)
	e.buildPrompt(pc)
	return e.callProvider(ctx, pc)
}

func (e *Engine) prepareContext(pc *Context) {
	strategy := ""
	maxInputTokens := 0
	preserveRecent := 0
	if pc.IncomingRequest.Novexa != nil && pc.IncomingRequest.Novexa.Context != nil {
		strategy = pc.IncomingRequest.Novexa.Context.Strategy
		maxInputTokens = pc.IncomingRequest.Novexa.Context.MaxInputTokens
		preserveRecent = pc.IncomingRequest.Novexa.Context.PreserveRecentMessages
	}

	out := e.contextEngine.Prepare(contextengine.Input{
		RequestID:              pc.RequestID,
		RuntimeMode:            string(pc.RuntimeMode),
		Messages:               pc.NormalizedRequest.Messages,
		Strategy:               strategy,
		MaxInputTokens:         maxInputTokens,
		PreserveRecentMessages: preserveRecent,
	})

	pc.MessagesNormalized = out.NormalizedMessages
	pc.MessagesCompressed = out.FinalMessages
	pc.ContextPackage = &out.Package
	pc.ContextReport = &out.Report
	pc.ContextCompressed = out.Report.ItemsRemoved > 0 || out.Report.EstimatedTokensAfter < out.Report.EstimatedTokensBefore
	pc.Warnings = append(pc.Warnings, out.Warnings...)
	pc.NormalizedRequest.Messages = out.FinalMessages

	pc.AddEvent("context", "context_prepared", SeverityInfo, "Context Engine prepared request context", map[string]string{
		"strategy":         out.Report.StrategyUsed,
		"tokens_before":    fmt.Sprintf("%d", out.Report.EstimatedTokensBefore),
		"tokens_after":     fmt.Sprintf("%d", out.Report.EstimatedTokensAfter),
		"items_removed":    fmt.Sprintf("%d", out.Report.ItemsRemoved),
		"duplicates":       fmt.Sprintf("%d", out.Report.DuplicatesRemoved),
		"context_compress": fmt.Sprintf("%t", pc.ContextCompressed),
	})
	if out.Report.DuplicatesRemoved > 0 {
		pc.AddEvent("context", "duplicate_context_removed", SeverityInfo, "duplicate context removed", map[string]string{
			"removed_items": fmt.Sprintf("%d", out.Report.DuplicatesRemoved),
		})
	}
	if out.Report.FallbackUsed {
		pc.AddEvent("context", "context_trimmed", SeverityInfo, "context trimmed to fit token budget", map[string]string{
			"items_removed": fmt.Sprintf("%d", out.Report.ItemsRemoved),
		})
	}
}

func (e *Engine) buildPrompt(pc *Context) {
	existingSystem := []string{}
	for _, msg := range pc.MessagesCompressed {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok && s != "" {
				existingSystem = append(existingSystem, s)
			}
		}
	}

	var pkg contextengine.Package
	if pc.ContextPackage != nil {
		pkg = *pc.ContextPackage
	}

	out := e.promptEngine.Build(promptengine.Input{
		RuntimeMode:    string(pc.RuntimeMode),
		Messages:       pc.MessagesCompressed,
		ContextPackage: pkg,
		ResponseFormat: pc.NormalizedRequest.ResponseFormat,
		ExistingSystem: existingSystem,
	})

	pc.PromptPackage = &out.Package
	pc.PromptReport = &out.Report
	pc.Warnings = append(pc.Warnings, out.Warnings...)
	pc.NormalizedRequest.Messages = out.FinalMessages

	pc.AddEvent("prompt", "prompt_built", SeverityInfo, "Prompt Engine built provider-ready messages", map[string]string{
		"system_prompt_added":     fmt.Sprintf("%t", out.Report.SystemPromptAdded),
		"response_format_applied": fmt.Sprintf("%t", out.Report.ResponseFormatApplied),
		"final_message_count":     fmt.Sprintf("%d", out.Report.FinalMessageCount),
	})
	if out.Report.ResponseFormatApplied {
		pc.AddEvent("prompt", "structured_prompt_applied", SeverityInfo, "structured output instructions applied", nil)
	}
}

func (e *Engine) callProvider(ctx context.Context, pc *Context) Result {
	pc.AddEvent("provider", "provider_selection_started", SeverityInfo, "provider selection started", map[string]string{
		"requested_model": pc.RequestedModel,
	})

	resolution, perr := e.manager.ResolveModel(ctx, pc.NormalizedRequest.Model)
	if perr.Code != "" {
		return e.fail(pc, perr, "provider selection failed")
	}

	pc.SelectedProvider = resolution.ProviderKey
	pc.SelectedModel = resolution.ModelName
	pc.AddEvent("provider", "provider_selected", SeverityInfo, "provider selected", map[string]string{
		"provider": resolution.ProviderKey,
		"model":    resolution.ModelName,
	})
	pc.AddEvent("provider", "provider_request_started", SeverityInfo, "provider request started", nil)

	providerReq := pc.NormalizedRequest
	providerReq.Model = resolution.ModelName

	start := time.Now()
	resp, err := resolution.Adapter.Generate(ctx, providerReq)
	pc.ProviderLatency = time.Since(start)
	if err != nil {
		var normalized provider.ProviderError
		if !errors.As(err, &normalized) {
			normalized = resolution.Adapter.NormalizeError(err)
		}
		return e.fail(pc, normalized, "provider request failed")
	}

	pc.ProviderResponse = resp
	pc.FinalResponse = resp
	if resp != nil {
		pc.SelectedModel = resp.Model
	}
	pc.AddEvent("provider", "provider_request_completed", SeverityInfo, "provider request completed", map[string]string{
		"provider":   pc.SelectedProvider,
		"model":      pc.SelectedModel,
		"latency_ms": fmt.Sprintf("%d", pc.ProviderLatency.Milliseconds()),
	})
	pc.AddEvent("response", "response_normalized", SeverityInfo, "provider response normalized", nil)
	pc.AddEvent("validation", "validation_completed", SeverityInfo, "Validation Engine skeleton placeholder", map[string]string{
		"passed": "true",
	})
	pc.AddEvent("telemetry", "telemetry_recorded", SeverityInfo, "telemetry recorded", nil)
	pc.AddEvent("pipeline", "pipeline_completed", SeverityInfo, "pipeline completed successfully", nil)

	e.recordTelemetry(ctx, pc)

	if resp != nil && resp.Novexa == nil && pc.IncomingRequest.Novexa != nil && pc.IncomingRequest.Novexa.Telemetry != nil && pc.IncomingRequest.Novexa.Telemetry.IncludeMetadata {
		resp.Novexa = &api.NovexaMetadata{
			RequestID:         pc.RequestID,
			Provider:          pc.SelectedProvider,
			RuntimeMode:       string(pc.RuntimeMode),
			ContextCompressed: pc.ContextCompressed,
			ValidationPassed:  true,
			RepairApplied:     false,
			RetryCount:        pc.Retry.Attempt - 1,
			LatencyMs:         pc.ProviderLatency.Milliseconds(),
		}
	}

	return Result{
		Response:     pc.FinalResponse,
		Context:      pc,
		ProviderName: pc.SelectedProvider,
	}
}

func (e *Engine) fail(pc *Context, perr provider.ProviderError, message string) Result {
	pc.ProviderError = &perr
	pc.Errors = append(pc.Errors, perr.Message)
	pc.AddEvent("pipeline", "pipeline_failed", SeverityError, message, map[string]string{
		"code": string(perr.Code),
	})
	pc.AddEvent("telemetry", "telemetry_recorded", SeverityInfo, "telemetry recorded", nil)

	e.recordTelemetry(context.Background(), pc)

	if e.log != nil {
		e.log.Error("pipeline failed", perr, "request_id", pc.RequestID, "code", string(perr.Code))
	}

	return Result{
		Context:      pc,
		ProviderName: pc.SelectedProvider,
		Error:        perr,
	}
}

func (e *Engine) recordTelemetry(ctx context.Context, pc *Context) {
	if e.telemetry == nil {
		return
	}

	events := make([]telemetry.PipelineEventRecord, len(pc.Events))
	for i, ev := range pc.Events {
		events[i] = telemetry.PipelineEventRecord{
			RequestID: ev.RequestID,
			Timestamp: ev.Timestamp,
			Engine:    ev.Engine,
			Event:     ev.Event,
			Severity:  string(ev.Severity),
			Message:   ev.Message,
			Metadata:  ev.Metadata,
		}
	}
	e.telemetry.RecordPipelineEvents(ctx, events)

	if pc.ProviderError != nil {
		e.telemetry.RecordError(ctx, pc.RequestID, "pipeline", *pc.ProviderError)
	}
}
