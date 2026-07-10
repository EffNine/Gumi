package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	guardengine "github.com/novexa/novexa/runtime/internal/guard"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/profiles"
	promptengine "github.com/novexa/novexa/runtime/internal/prompt"
	"github.com/novexa/novexa/runtime/internal/provider"
	repairengine "github.com/novexa/novexa/runtime/internal/repair"
	"github.com/novexa/novexa/runtime/internal/telemetry"
	validationengine "github.com/novexa/novexa/runtime/internal/validation"
)

const (
	defaultWorkspaceID = "default"
	defaultRetryMax    = 2
)

// Engine orchestrates the request lifecycle for chat completions.
type Engine struct {
	cfg             *config.Config
	manager         *provider.Manager
	log             *logger.Logger
	telemetry       *telemetry.Writer
	contextEngine   *contextengine.Engine
	promptEngine    *promptengine.Engine
	guardEngine     *guardengine.Engine
	validation      *validationengine.Engine
	repair          *repairengine.Engine
	profileResolver *profiles.Resolver
}

// Result is returned to Gateway Engine after pipeline execution.
type Result struct {
	Response     *api.ChatCompletionResponse
	Context      *Context
	ProviderName string
	Error        provider.ProviderError
}

// New creates a Pipeline Engine and loads built-in model profiles.
func New(cfg *config.Config, manager *provider.Manager, log *logger.Logger) *Engine {
	loader := profiles.NewDefaultLoader()
	loaded, _ := loader.Load()
	resolver := profiles.NewResolver(loaded.Profiles)

	return &Engine{
		cfg:             cfg,
		manager:         manager,
		log:             log,
		contextEngine:   contextengine.New(),
		promptEngine:    promptengine.New(),
		guardEngine:     guardengine.New(),
		validation:      validationengine.New(),
		repair:          repairengine.New(),
		profileResolver: resolver,
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
		return e.runStabilized(ctx, pc)
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
	if result := e.resolveProviderAndProfile(ctx, pc); result.Error.Code != "" {
		return result
	}
	if result := e.applyGuard(pc); result.Error.Code != "" {
		return result
	}
	return e.callProviderGenerate(ctx, pc)
}

func (e *Engine) runStabilized(ctx context.Context, pc *Context) Result {
	if result := e.resolveProviderAndProfile(ctx, pc); result.Error.Code != "" {
		return result
	}
	if pc.RuntimeMode == ModeStructured {
		pc.AddEvent("guard", "structured_output_guard_enabled", SeverityInfo, "structured output guard and validation enabled", nil)
	}
	e.prepareContext(pc)
	e.buildPrompt(pc)
	if result := e.applyGuard(pc); result.Error.Code != "" {
		return result
	}
	return e.callProviderGenerate(ctx, pc)
}

func (e *Engine) resolveProviderAndProfile(ctx context.Context, pc *Context) Result {
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

	match := e.profileResolver.Resolve(resolution.ProviderKey, resolution.ModelName)
	pc.ModelProfile = match.Profile
	if match.IsFallback {
		pc.AddEvent("profile", "model_profile_fallback", SeverityWarning, "no matching profile found; using generic fallback", map[string]string{
			"profile_id": match.Profile.ID,
			"model":      resolution.ModelName,
			"reason":     match.Reason,
		})
	} else {
		pc.AddEvent("profile", "model_profile_applied", SeverityInfo, "model profile applied", map[string]string{
			"profile_id":   match.Profile.ID,
			"model":        resolution.ModelName,
			"match_reason": match.Reason,
		})
	}
	return Result{}
}

func (e *Engine) applyGuard(pc *Context) Result {
	out := e.guardEngine.Check(guardengine.Input{
		Messages:       pc.NormalizedRequest.Messages,
		ResponseFormat: pc.NormalizedRequest.ResponseFormat,
		RuntimeMode:    string(pc.RuntimeMode),
		ContextReport:  pc.ContextReport,
		ModelProfile:   pc.ModelProfile,
	})
	pc.GuardReport = &out.Report
	pc.Warnings = append(pc.Warnings, out.Warnings...)
	pc.AddEvent("guard", "guard_completed", SeverityInfo, "Guard Engine completed pre-generation checks", map[string]string{
		"decision": string(out.Report.Decision),
		"blocked":  fmt.Sprintf("%t", out.Report.Blocked),
	})
	if pc.ModelProfile != nil {
		pc.AddEvent("guard", "guard_profile_applied", SeverityInfo, "guard settings informed by model profile", map[string]string{
			"profile_id": pc.ModelProfile.ID,
			"anti_loop":  pc.ModelProfile.Guard.AntiLoop,
		})
	}
	for _, warning := range out.Warnings {
		pc.AddEvent("guard", "guard_warning", SeverityWarning, warning, nil)
	}
	if out.Error.Code != "" {
		return e.fail(pc, out.Error, "guard blocked request")
	}
	return Result{}
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
		ModelProfile:           pc.ModelProfile,
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
		ModelProfile:   pc.ModelProfile,
	})

	pc.PromptPackage = &out.Package
	pc.PromptReport = &out.Report
	pc.Warnings = append(pc.Warnings, out.Warnings...)
	pc.NormalizedRequest.Messages = out.FinalMessages

	pc.AddEvent("prompt", "prompt_built", SeverityInfo, "Prompt Engine built provider-ready messages", map[string]string{
		"system_prompt_added":          fmt.Sprintf("%t", out.Report.SystemPromptAdded),
		"response_format_applied":      fmt.Sprintf("%t", out.Report.ResponseFormatApplied),
		"profile_instructions_applied": fmt.Sprintf("%t", out.Report.ProfileInstructionsApplied),
		"final_message_count":          fmt.Sprintf("%d", out.Report.FinalMessageCount),
	})
	if out.Report.ResponseFormatApplied {
		pc.AddEvent("prompt", "structured_prompt_applied", SeverityInfo, "structured output instructions applied", nil)
	}
	if out.Report.ProfileInstructionsApplied && pc.ModelProfile != nil {
		pc.AddEvent("prompt", "profile_prompt_applied", SeverityInfo, "model profile prompt instructions applied", map[string]string{
			"profile_id": pc.ModelProfile.ID,
		})
	}
}

func (e *Engine) callProviderGenerate(ctx context.Context, pc *Context) Result {
	providerReq := pc.NormalizedRequest
	providerReq.Model = pc.SelectedModel
	e.applyProfileDefaults(pc, &providerReq)

	pc.AddEvent("provider", "provider_request_started", SeverityInfo, "provider request started", map[string]string{
		"provider": pc.SelectedProvider,
		"model":    pc.SelectedModel,
	})

	adapter, ok := e.manager.Adapter(pc.SelectedProvider)
	if !ok {
		return e.fail(pc, provider.ProviderError{
			Code:       provider.ProviderMisconfigured,
			Message:    fmt.Sprintf("provider %q is no longer available", pc.SelectedProvider),
			Suggestion: "Restart Novexa or check provider configuration.",
		}, "provider adapter missing after selection")
	}

	resp, result := e.generateOnce(ctx, pc, adapter, providerReq)
	if result.Error.Code != "" {
		return result
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

	if result := e.validateRepairAndMaybeRetry(ctx, pc, adapter, providerReq); result.Error.Code != "" {
		return result
	}

	pc.AddEvent("telemetry", "telemetry_recorded", SeverityInfo, "telemetry recorded", nil)
	pc.AddEvent("pipeline", "pipeline_completed", SeverityInfo, "pipeline completed successfully", nil)

	e.recordTelemetry(ctx, pc)

	if resp != nil && resp.Novexa == nil && pc.IncomingRequest.Novexa != nil && pc.IncomingRequest.Novexa.Telemetry != nil && pc.IncomingRequest.Novexa.Telemetry.IncludeMetadata {
		resp.Novexa = &api.NovexaMetadata{
			RequestID:         pc.RequestID,
			Provider:          pc.SelectedProvider,
			RuntimeMode:       string(pc.RuntimeMode),
			ContextCompressed: pc.ContextCompressed,
			ValidationPassed:  pc.ValidationPassed,
			RepairApplied:     pc.RepairApplied,
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

func (e *Engine) applyProfileDefaults(pc *Context, req *api.ChatCompletionRequest) {
	if pc.ModelProfile == nil {
		return
	}
	beforeTemp := req.Temperature
	beforeTopP := req.TopP
	beforeMaxTokens := req.MaxTokens
	profiles.ApplyDefaults(pc.ModelProfile, req)
	pc.AddEvent("profile", "profile_defaults_applied", SeverityInfo, "applied model profile defaults", map[string]string{
		"profile_id":  pc.ModelProfile.ID,
		"temperature": fmt.Sprintf("%t", beforeTemp == nil && req.Temperature != nil),
		"top_p":       fmt.Sprintf("%t", beforeTopP == nil && req.TopP != nil),
		"max_tokens":  fmt.Sprintf("%t", beforeMaxTokens == nil && req.MaxTokens != nil),
	})
}

func (e *Engine) generateOnce(ctx context.Context, pc *Context, adapter provider.ProviderAdapter, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, Result) {
	start := time.Now()
	resp, err := adapter.Generate(ctx, req)
	pc.ProviderLatency += time.Since(start)
	if err != nil {
		var normalized provider.ProviderError
		if !errors.As(err, &normalized) {
			normalized = adapter.NormalizeError(err)
		}
		return nil, e.fail(pc, normalized, "provider request failed")
	}
	return resp, Result{}
}

func (e *Engine) validateRepairAndMaybeRetry(ctx context.Context, pc *Context, adapter provider.ProviderAdapter, providerReq api.ChatCompletionRequest) Result {
	report := e.validation.Validate(validationengine.Input{
		Response:       pc.FinalResponse,
		ResponseFormat: pc.NormalizedRequest.ResponseFormat,
		RuntimeMode:    string(pc.RuntimeMode),
	})
	pc.ValidationReport = &report
	pc.ValidationPassed = report.Passed
	pc.AddEvent("validation", "validation_completed", severityForValidation(report), "Validation Engine completed response checks", map[string]string{
		"passed":   fmt.Sprintf("%t", report.Passed),
		"severity": report.Severity,
		"issues":   fmt.Sprintf("%d", len(report.Issues)),
	})
	for _, issue := range report.Issues {
		pc.AddEvent("validation", string(issue.Code), SeverityWarning, issue.Message, map[string]string{
			"location": issue.Location,
		})
	}
	if report.Passed {
		return Result{}
	}

	repairReport := e.repair.Repair(pc.FinalResponse, report)
	pc.RepairReport = &repairReport
	pc.RepairApplied = repairReport.Success
	pc.AddEvent("repair", "repair_completed", SeverityInfo, "Repair Engine completed response repair attempt", map[string]string{
		"attempted":        fmt.Sprintf("%t", repairReport.Attempted),
		"success":          fmt.Sprintf("%t", repairReport.Success),
		"strategy":         repairReport.Strategy,
		"retry_requested":  fmt.Sprintf("%t", repairReport.RetryRequested),
		"changes_applied":  fmt.Sprintf("%d", len(repairReport.Changes)),
		"remaining_issues": fmt.Sprintf("%d", len(repairReport.RemainingIssues)),
	})

	if repairReport.Success {
		second := e.validation.Validate(validationengine.Input{
			Response:       pc.FinalResponse,
			ResponseFormat: pc.NormalizedRequest.ResponseFormat,
			RuntimeMode:    string(pc.RuntimeMode),
		})
		pc.ValidationReport = &second
		pc.ValidationPassed = second.Passed
		pc.AddEvent("validation", "validation_completed_after_repair", severityForValidation(second), "Validation Engine checked repaired response", map[string]string{
			"passed": fmt.Sprintf("%t", second.Passed),
		})
		if second.Passed {
			return Result{}
		}
		report = second
	}

	if (repairReport.RetryRequested || shouldRetryValidation(report, pc)) && pc.Retry.Attempt < pc.Retry.MaxAttempts {
		pc.Retry.RetryReason = "validation_failed"
		pc.Retry.RetryHistory = append(pc.Retry.RetryHistory, RetryRecord{
			Attempt:   pc.Retry.Attempt,
			Reason:    "validation_failed",
			Strategy:  "stricter_structured_prompt",
			Result:    "retrying",
			Timestamp: time.Now().UTC(),
		})
		pc.Retry.Attempt++
		pc.AddEvent("pipeline", "retry_requested", SeverityWarning, "validation failed; retrying with stricter prompt", map[string]string{
			"attempt": fmt.Sprintf("%d", pc.Retry.Attempt),
		})
		retryReq := providerReq
		retryReq.Messages = prependRetryInstruction(providerReq.Messages)
		resp, result := e.generateOnce(ctx, pc, adapter, retryReq)
		if result.Error.Code != "" {
			return result
		}
		pc.ProviderResponse = resp
		pc.FinalResponse = resp
		if resp != nil {
			pc.SelectedModel = resp.Model
		}
		retryReport := e.validation.Validate(validationengine.Input{
			Response:       pc.FinalResponse,
			ResponseFormat: pc.NormalizedRequest.ResponseFormat,
			RuntimeMode:    string(pc.RuntimeMode),
		})
		pc.ValidationReport = &retryReport
		pc.ValidationPassed = retryReport.Passed
		pc.AddEvent("validation", "validation_completed_after_retry", severityForValidation(retryReport), "Validation Engine checked retried response", map[string]string{
			"passed": fmt.Sprintf("%t", retryReport.Passed),
		})
		if retryReport.Passed {
			pc.Retry.RetryHistory = append(pc.Retry.RetryHistory, RetryRecord{
				Attempt:   pc.Retry.Attempt,
				Reason:    "validation_failed",
				Strategy:  "stricter_structured_prompt",
				Result:    "success",
				Timestamp: time.Now().UTC(),
			})
			return Result{}
		}
		report = retryReport
	}

	return e.fail(pc, provider.ProviderError{
		Code:       provider.ValidationFailed,
		Message:    "model output failed validation and could not be repaired",
		Suggestion: "Try a clearer prompt, structured response_format, or a more capable local model.",
	}, "validation failed")
}

func severityForValidation(report validationengine.Report) Severity {
	if report.Passed {
		return SeverityInfo
	}
	if report.Severity == "error" {
		return SeverityError
	}
	return SeverityWarning
}

func shouldRetryValidation(report validationengine.Report, pc *Context) bool {
	return pc.RuntimeMode == ModeStructured && !report.Passed
}

func prependRetryInstruction(messages []api.Message) []api.Message {
	instruction := api.Message{
		Role:    "system",
		Content: "Retry because the previous output failed validation. Return only valid JSON if JSON was requested. Do not include markdown fences, repeated text, or explanatory prose.",
	}
	out := make([]api.Message, 0, len(messages)+1)
	out = append(out, instruction)
	out = append(out, messages...)
	return out
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
