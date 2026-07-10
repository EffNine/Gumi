package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/provider"
)

const (
	defaultWorkspaceID = "default"
	defaultRetryMax    = 2
)

// Engine orchestrates the request lifecycle for chat completions.
type Engine struct {
	cfg     *config.Config
	manager *provider.Manager
	log     *logger.Logger
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
		cfg:     cfg,
		manager: manager,
		log:     log,
	}
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
	if pc.RuntimeMode == ModeStabilized {
		pc.AddEvent("context", "context_skipped", SeverityInfo, "Context Engine skeleton placeholder; full processing starts in Sprint 6", nil)
		pc.AddEvent("prompt", "prompt_skipped", SeverityInfo, "Prompt Engine skeleton placeholder; full processing starts in Sprint 6", nil)
	}
	if pc.RuntimeMode == ModeStructured {
		pc.AddEvent("guard", "structured_mode_skeleton", SeverityInfo, "Structured mode skeleton active; validation and repair start in Sprint 7", nil)
	}
	return e.callProvider(ctx, pc)
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
	pc.AddEvent("telemetry", "telemetry_recorded", SeverityInfo, "Telemetry Engine skeleton placeholder", nil)
	pc.AddEvent("pipeline", "pipeline_completed", SeverityInfo, "pipeline completed successfully", nil)

	if resp != nil && resp.Novexa == nil && pc.IncomingRequest.Novexa != nil && pc.IncomingRequest.Novexa.Telemetry != nil && pc.IncomingRequest.Novexa.Telemetry.IncludeMetadata {
		resp.Novexa = &api.NovexaMetadata{
			RequestID:         pc.RequestID,
			Provider:          pc.SelectedProvider,
			RuntimeMode:       string(pc.RuntimeMode),
			ContextCompressed: false,
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
	pc.AddEvent("telemetry", "telemetry_recorded", SeverityInfo, "Telemetry Engine skeleton placeholder", nil)

	if e.log != nil {
		e.log.Error("pipeline failed", perr, "request_id", pc.RequestID, "code", string(perr.Code))
	}

	return Result{
		Context:      pc,
		ProviderName: pc.SelectedProvider,
		Error:        perr,
	}
}
