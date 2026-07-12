package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	guardengine "github.com/novexa/novexa/runtime/internal/guard"
	instructionengine "github.com/novexa/novexa/runtime/internal/instruction"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/profiles"
	promptengine "github.com/novexa/novexa/runtime/internal/prompt"
	"github.com/novexa/novexa/runtime/internal/provider"
	repairengine "github.com/novexa/novexa/runtime/internal/repair"
	"github.com/novexa/novexa/runtime/internal/telemetry"
	thinkingengine "github.com/novexa/novexa/runtime/internal/thinking"
	toolengine "github.com/novexa/novexa/runtime/internal/tool"
	validationengine "github.com/novexa/novexa/runtime/internal/validation"
)

const (
	defaultWorkspaceID    = "default"
	defaultRetryMax       = 2
	defaultReservedOutput = 2048
	defaultReservedSystem = 600
)

// Engine orchestrates the request lifecycle for chat completions.
type Engine struct {
	cfg                *config.Config
	manager            *provider.Manager
	log                *logger.Logger
	telemetry          *telemetry.Writer
	contextEngine      *contextengine.Engine
	promptEngine       *promptengine.Engine
	guardEngine        *guardengine.Engine
	validation         *validationengine.Engine
	repair             *repairengine.Engine
	toolEngine         *toolengine.Engine
	instructionEngine  *instructionengine.Engine
	profileResolver    *profiles.Resolver
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
		cfg:               cfg,
		manager:           manager,
		log:               log,
		contextEngine:     contextengine.New(),
		promptEngine:      promptengine.New(),
		guardEngine:       guardengine.New(),
		validation:        validationengine.New(),
		repair:            repairengine.New(),
		toolEngine:        toolengine.New(),
		instructionEngine: instructionengine.New(),
		profileResolver:   resolver,
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
			Message:    "streaming chat completions are not supported in this release",
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
	case ModeLightweight:
		return e.runLightweight(ctx, pc)
	case ModeStabilized, ModeStructured:
		return e.runStabilized(ctx, pc)
	case ModeAgent:
		return e.fail(pc, provider.ProviderError{
			Code:       provider.ProviderMisconfigured,
			Message:    "agent mode is reserved for a future Novexa release",
			Suggestion: "Use direct, lightweight, stabilized, or structured mode.",
		}, "unsupported runtime mode")
	default:
		return e.fail(pc, provider.ProviderError{
			Code:       provider.ProviderMisconfigured,
			Message:    fmt.Sprintf("runtime mode %q is not supported", pc.RuntimeMode),
			Suggestion: "Use direct, lightweight, stabilized, or structured mode.",
		}, "unsupported runtime mode")
	}
}

func (e *Engine) newContext(requestID string, req api.ChatCompletionRequest) *Context {
	mode := RuntimeMode(e.cfg.Runtime.Mode)
	if req.Novexa != nil && req.Novexa.Mode != "" {
		mode = RuntimeMode(req.Novexa.Mode)
	}
	if mode != ModeLightweight && mode != ModeDirect && req.ResponseFormat != nil && (req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json_schema") {
		mode = ModeStructured
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

func (e *Engine) runLightweight(ctx context.Context, pc *Context) Result {
	pc.AddEvent("pipeline", "lightweight_mode_selected", SeverityInfo, "lightweight mode selected", nil)
	pc.AddEvent("session", "session_skipped", SeverityInfo, "session resolution skipped in lightweight mode", nil)
	pc.AddEvent("context", "context_skipped", SeverityInfo, "context compression skipped in lightweight mode", nil)
	pc.AddEvent("memory", "memory_skipped", SeverityInfo, "memory retrieval skipped in lightweight mode", nil)

	if result := e.resolveProviderAndProfile(ctx, pc); result.Error.Code != "" {
		return result
	}

	e.prepareToolShim(pc)
	e.applyProfileDefaults(pc, &pc.NormalizedRequest)
	e.applyThinkingPolicy(pc)
	e.buildLightweightPrompt(pc)
	e.applyInstructionAssist(pc)

	if result := e.applyLightweightGuard(pc); result.Error.Code != "" {
		return result
	}

	return e.callProviderGenerate(ctx, pc)
}

// lightweightProfileDefaultsApplied records profile defaults into the
// normalized request before prompt building so that the final provider call
// and telemetry reflect the resolved model tuning.

func (e *Engine) runStabilized(ctx context.Context, pc *Context) Result {
	if result := e.resolveProviderAndProfile(ctx, pc); result.Error.Code != "" {
		return result
	}
	e.prepareToolShim(pc)
	if pc.RuntimeMode == ModeStructured {
		pc.AddEvent("guard", "structured_output_guard_enabled", SeverityInfo, "structured output guard and validation enabled", nil)
	}
	e.applyProfileDefaults(pc, &pc.NormalizedRequest)
	e.applyThinkingPolicy(pc)
	e.prepareContext(pc)
	e.buildPrompt(pc)
	e.applyInstructionAssist(pc)
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

// prepareToolShim enables prompt-based tool calling for models whose profile
// declares tool_calling: weak. It stores the original tools, builds tool
// instructions for the Prompt Engine, and clears the native tools list so the
// thin provider adapter does not attempt unsupported native tool calling.
func (e *Engine) prepareToolShim(pc *Context) {
	req := &pc.NormalizedRequest
	if len(req.Tools) == 0 {
		return
	}
	if pc.ModelProfile == nil || !isWeakToolCalling(pc.ModelProfile.Capabilities.ToolCalling) {
		return
	}

	pc.OriginalTools = append([]api.Tool(nil), req.Tools...)
	pc.ToolShimActive = true

	instructions, warnings := e.toolEngine.BuildInstructions(pc.OriginalTools, pc.ModelProfile)
	pc.ToolInstructions = instructions
	for _, w := range warnings {
		pc.Warnings = append(pc.Warnings, w)
		pc.AddEvent("tool", "tool_shim_warning", SeverityWarning, w, nil)
	}
	pc.ToolSchemaHint = toolengine.SchemaHint(pc.OriginalTools)

	req.Tools = nil
	req.ToolChoice = nil

	pc.AddEvent("tool", "tool_shim_enabled", SeverityInfo, "prompt-based tool calling shim enabled", map[string]string{
		"profile_id":  pc.ModelProfile.ID,
		"tool_count":  fmt.Sprintf("%d", len(pc.OriginalTools)),
		"tools":       pc.ToolSchemaHint,
		"instruction": fmt.Sprintf("%t", instructions != ""),
	})
}

func isWeakToolCalling(v string) bool {
	switch strings.ToLower(v) {
	case "weak", "none", "unknown", "medium":
		return true
	}
	return false
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

func (e *Engine) applyLightweightGuard(pc *Context) Result {
	messages := pc.NormalizedRequest.Messages

	if latestUserMessage(messages) == "" {
		return e.fail(pc, provider.ProviderError{
			Code:       provider.EmptyPrompt,
			Message:    "the prompt is empty after normalization",
			Suggestion: "Provide a non-empty user message.",
		}, "lightweight guard blocked empty prompt")
	}

	pc.AddEvent("guard", "empty_prompt_check_passed", SeverityInfo, "lightweight guard: prompt is not empty", nil)

	if pc.ModelProfile != nil && pc.ModelProfile.ContextLimit > 0 {
		estimated := contextengine.EstimateMessages(messages)
		budget := pc.ModelProfile.ContextLimit - defaultReservedOutput - defaultReservedSystem
		if budget > 0 && estimated > budget {
			pc.AddEvent("guard", "context_overflow_warning", SeverityWarning, "lightweight guard: messages may exceed model context budget", map[string]string{
				"estimated_tokens":   fmt.Sprintf("%d", estimated),
				"approximate_budget": fmt.Sprintf("%d", budget),
			})
			pc.Warnings = append(pc.Warnings, "context_overflow_estimate: messages may exceed model context budget")
		}
	}

	unsupported := unsupportedFeature(pc.NormalizedRequest, pc.ModelProfile)
	if unsupported != "" {
		pc.AddEvent("guard", "unsupported_feature_warning", SeverityWarning, "lightweight guard: unsupported feature requested", map[string]string{
			"feature": unsupported,
		})
		pc.Warnings = append(pc.Warnings, "unsupported feature: "+unsupported)
	}

	pc.AddEvent("guard", "lightweight_guard_completed", SeverityInfo, "lightweight guard checks completed", map[string]string{
		"anti_loop": pc.ModelProfile.Guard.AntiLoop,
	})
	return Result{}
}

func unsupportedFeature(req api.ChatCompletionRequest, profile *profiles.Profile) string {
	if len(req.Tools) > 0 {
		if profile == nil || profile.Capabilities.ToolCalling == "none" || profile.Capabilities.ToolCalling == "unknown" {
			return "tool calling"
		}
	}
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_schema" {
		if profile == nil || profile.Capabilities.StructuredOutput == "none" || profile.Capabilities.StructuredOutput == "unknown" || profile.Capabilities.StructuredOutput == "weak" {
			return "json_schema"
		}
	}
	return ""
}

func latestUserMessage(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.ToLower(messages[i].Role) != "user" {
			continue
		}
		if s, ok := messages[i].Content.(string); ok {
			return strings.TrimSpace(s)
		}
		if messages[i].Content != nil {
			return "non-text"
		}
	}
	return ""
}

// latestSystemMessage returns the first system message content from the message
// list, scanning forward. Agent frameworks (e.g. Terminus-2) often put JSON
// format instructions in the system prompt rather than the user message.
func latestSystemMessage(messages []api.Message) string {
	for _, msg := range messages {
		if strings.ToLower(msg.Role) != "system" {
			continue
		}
		if s, ok := msg.Content.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
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
	if out.Report.ToolResultsSummarized > 0 {
		pc.AddEvent("context", "tool_results_summarized", SeverityInfo, "old tool results summarized to fit context budget", map[string]string{
			"summarized_items": fmt.Sprintf("%d", out.Report.ToolResultsSummarized),
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
		RuntimeMode:      string(pc.RuntimeMode),
		Messages:         pc.MessagesCompressed,
		ContextPackage:   pkg,
		ResponseFormat:   pc.NormalizedRequest.ResponseFormat,
		ExistingSystem:   existingSystem,
		ModelProfile:     pc.ModelProfile,
		ToolInstructions: pc.ToolInstructions,
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

// buildLightweightPrompt assembles the minimal prompt policy for lightweight
// mode. It preserves existing system/developer/user messages, injects a minimal
// system prompt only if none exists, optionally appends a short anti-loop hint
// when guard.anti_loop != off, and appends a minimal JSON/schema instruction
// only when response_format is set.
func (e *Engine) buildLightweightPrompt(pc *Context) {
	messages := pc.NormalizedRequest.Messages
	hasSystemOrDeveloper := false
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			hasSystemOrDeveloper = true
			break
		}
	}

	final := make([]api.Message, 0, len(messages)+1)
	if !hasSystemOrDeveloper {
		final = append(final, api.Message{Role: "system", Content: minimalSystemPrompt(pc.ModelProfile)})
		pc.AddEvent("prompt", "minimal_system_prompt_added", SeverityInfo, "lightweight mode added minimal system prompt", map[string]string{
			"source": profileOrGeneric(pc.ModelProfile, "prompt.system_prompt_style"),
		})
	}
	final = append(final, messages...)

	if pc.ModelProfile != nil && pc.ModelProfile.Guard.AntiLoop != "" && strings.ToLower(pc.ModelProfile.Guard.AntiLoop) != "off" {
		final = appendLightweightHint(final, "Stay focused on the current request and avoid repeating earlier steps.")
		pc.AddEvent("prompt", "anti_loop_hint_added", SeverityInfo, "lightweight mode added short anti-loop hint", map[string]string{
			"anti_loop": pc.ModelProfile.Guard.AntiLoop,
		})
	}

	if pc.ToolInstructions != "" {
		final = appendLightweightHint(final, pc.ToolInstructions)
		pc.AddEvent("prompt", "tool_instructions_added", SeverityInfo, "lightweight mode added tool instructions", map[string]string{
			"tools": pc.ToolSchemaHint,
		})
	}

	instructions := buildLightweightFormatInstructions(pc.NormalizedRequest.ResponseFormat, pc.ModelProfile)
	if instructions != "" {
		final = appendLightweightHint(final, instructions)
	}

	if pc.NormalizedRequest.ResponseFormat != nil && pc.NormalizedRequest.ResponseFormat.Type != "" {
		pc.AddEvent("prompt", "lightweight_format_instruction_added", SeverityInfo, "lightweight mode added minimal format instruction", map[string]string{
			"response_format": pc.NormalizedRequest.ResponseFormat.Type,
		})
	}

	pc.NormalizedRequest.Messages = final
	pc.PromptPackage = &promptengine.Package{
		SystemPrompt:  lightweightSystemContent(final),
		FinalMessages: final,
	}
	pc.PromptReport = &promptengine.Report{
		SystemPromptAdded:     !hasSystemOrDeveloper,
		ResponseFormatApplied: pc.NormalizedRequest.ResponseFormat != nil && pc.NormalizedRequest.ResponseFormat.Type != "",
		FinalMessageCount:     len(final),
	}
	pc.AddEvent("prompt", "lightweight_prompt_built", SeverityInfo, "Prompt Engine built lightweight prompt", map[string]string{
		"system_prompt_added":     fmt.Sprintf("%t", !hasSystemOrDeveloper),
		"response_format_applied": fmt.Sprintf("%t", pc.NormalizedRequest.ResponseFormat != nil && pc.NormalizedRequest.ResponseFormat.Type != ""),
		"final_message_count":     fmt.Sprintf("%d", len(final)),
	})
}

func minimalSystemPrompt(profile *profiles.Profile) string {
	if profile != nil && profile.Prompt.SystemPromptStyle == "minimal" && len(profile.Prompt.Instructions) > 0 {
		return profile.Prompt.Instructions[0]
	}
	return "You are a helpful assistant. Answer the user's request directly and clearly."
}

func buildLightweightFormatInstructions(format *api.ResponseFormat, profile *profiles.Profile) string {
	if format == nil || format.Type == "" {
		return ""
	}
	style := "explicit"
	if profile != nil && profile.Prompt.JSONInstructionStyle != "" {
		style = strings.ToLower(profile.Prompt.JSONInstructionStyle)
	}
	switch format.Type {
	case "json_object":
		switch style {
		case "simple":
			return "Return a valid JSON object."
		case "schema_first":
			return "Return a valid JSON object. Do not wrap it in markdown fences or add explanatory prose. The root must be an object."
		default: // explicit
			return "Return a valid JSON object. Do not wrap it in markdown fences or add explanatory prose. The root must be an object."
		}
	case "json_schema":
		schemaHint := ""
		required := ""
		if format.JSONSchema != nil {
			if data, err := json.Marshal(format.JSONSchema.Schema); err == nil && len(data) > 0 {
				schemaHint = string(data)
			}
			if req, ok := format.JSONSchema.Schema["required"].([]interface{}); ok && len(req) > 0 {
				parts := make([]string, 0, len(req))
				for _, r := range req {
					if s, ok := r.(string); ok {
						parts = append(parts, s)
					}
				}
				required = strings.Join(parts, ", ")
			}
		}
		base := "Return JSON matching the requested schema. Do not wrap it in markdown fences or add explanatory prose."
		if schemaHint != "" {
			base += " Schema: " + schemaHint
		}
		if required != "" {
			base += " Required top-level keys: " + required + "."
		}
		if style == "explicit" || style == "schema_first" {
			base += " Return ONLY the raw JSON object. No markdown fences, no code blocks, no explanation."
		}
		return base
	default:
		return ""
	}
}

func appendLightweightHint(messages []api.Message, hint string) []api.Message {
	if len(messages) == 0 {
		return messages
	}
	out := make([]api.Message, 0, len(messages))
	out = append(out, messages...)

	// Prefer appending the hint to the first system/developer message so the
	// existing app system prompt stays intact and no extra system message is
	// inserted when one already exists.
	for i := range out {
		if out[i].Role == "system" || out[i].Role == "developer" {
			if s, ok := out[i].Content.(string); ok {
				out[i].Content = strings.TrimSpace(s) + "\n\n" + hint
				return out
			}
		}
	}

	// No system/developer message exists; prepend a new system hint.
	return append([]api.Message{{Role: "system", Content: hint}}, out...)
}

func lightweightSystemContent(messages []api.Message) string {
	if len(messages) == 0 {
		return ""
	}
	if messages[0].Role == "system" {
		if s, ok := messages[0].Content.(string); ok {
			return s
		}
	}
	return ""
}

func profileOrGeneric(profile *profiles.Profile, field string) string {
	if profile == nil {
		return "generic fallback"
	}
	return profile.ID
}

func copyThinkingToRequest(req *api.ChatCompletionRequest, enabled *bool) {
	if req == nil || enabled == nil {
		return
	}
	if req.Novexa == nil {
		req.Novexa = &api.NovexaExtensions{}
	}
	if req.Novexa.Thinking == nil {
		req.Novexa.Thinking = &api.ThinkingConfig{}
	}
	req.Novexa.Thinking.Enabled = enabled
}

func (e *Engine) callProviderGenerate(ctx context.Context, pc *Context) Result {
	providerReq := pc.NormalizedRequest
	providerReq.Model = pc.SelectedModel

	// Profile defaults and thinking policy are already applied to
	// pc.NormalizedRequest before prompt building. The copy for the provider
	// request inherits them.

	// Do not let the provider adapter see a response_format when the tool shim
	// is active. The model is instructed to return a JSON tool call, which is
	// then parsed into tool_calls; a native response_format would conflict.
	if pc.ToolShimActive {
		providerReq.ResponseFormat = nil
	}

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

	// Strip reasoning/thinking traces before any downstream parsing or validation.
	// Actual reasoning text is never stored; only safe metadata is recorded.
	e.stripReasoningContent(pc)

	e.normalizeToolResponse(pc)

	// Instruction-following validation: check response against extracted
	// constraints. If constraints fail, retry with stronger hints.
	// Skip instruction retry when structured output validation will also
	// retry (JSON validity), to avoid double-consuming provider responses.
	if !e.validateInstructions(pc) && pc.InstructionRetryCount <= defaultRetryMax {
		// Only do instruction retry for non-JSON constraints, or when
		// structured validation won't also retry.
		hasJSONViolation := false
		for _, c := range pc.InstructionConstraints {
			if c.Type == "json" {
				content := ""
				if len(pc.ProviderResponse.Choices) > 0 {
					if s, ok := pc.ProviderResponse.Choices[0].Message.Content.(string); ok {
						content = s
					}
				}
				if !e.instructionEngine.Validate(content, []instructionengine.Constraint{c}).Passed {
					hasJSONViolation = true
				}
			}
		}
		skipRetry := hasJSONViolation && pc.NormalizedRequest.ResponseFormat != nil && pc.NormalizedRequest.ResponseFormat.Type != ""

		if !skipRetry {
			pc.AddEvent("instruction", "instruction_retry_triggered", SeverityWarning, "retrying due to instruction constraint violations", map[string]string{
				"attempt": fmt.Sprintf("%d", pc.InstructionRetryCount),
			})

			// Build retry hint and inject into messages.
			content := ""
			if len(pc.ProviderResponse.Choices) > 0 {
				if s, ok := pc.ProviderResponse.Choices[0].Message.Content.(string); ok {
					content = s
				}
			}
			v := e.instructionEngine.Validate(content, pc.InstructionConstraints)
			retryHint := e.instructionEngine.BuildRetryHint(v.Violations, pc.InstructionConstraints)
			if retryHint != "" {
				messages := pc.NormalizedRequest.Messages
				for i, msg := range messages {
					if msg.Role == "system" {
						if s, ok := msg.Content.(string); ok {
							messages[i].Content = s + "\n\n" + retryHint
						}
						break
					}
				}
			}

			// Retry the provider call.
			retryReq := providerReq
			retryResp, retryResult := e.generateOnce(ctx, pc, adapter, retryReq)
			if retryResult.Error.Code != "" {
				return retryResult
			}
			pc.ProviderResponse = retryResp
			pc.FinalResponse = retryResp
			e.stripReasoningContent(pc)
			e.normalizeToolResponse(pc)
			pc.AddEvent("instruction", "instruction_retry_completed", SeverityInfo, "instruction constraint retry completed", nil)
		}
	}

	// Record safe thinking telemetry metadata.
	pc.ThinkingTelemetry = resolveThinkingTelemetry(pc)

	pc.AddEvent("provider", "provider_request_completed", SeverityInfo, "provider request completed", map[string]string{
		"provider":   pc.SelectedProvider,
		"model":      pc.SelectedModel,
		"latency_ms": fmt.Sprintf("%d", pc.ProviderLatency.Milliseconds()),
	})
	pc.AddEvent("response", "response_normalized", SeverityInfo, "provider response normalized", nil)

	if e.shouldValidate(pc) {
		if result := e.validateRepairAndMaybeRetry(ctx, pc, adapter, providerReq); result.Error.Code != "" {
			return result
		}
	} else {
		pc.ValidationPassed = true
		pc.AddEvent("validation", "validation_skipped", SeverityInfo, "validation skipped in lightweight mode", map[string]string{
			"reason": "response_format not present and novexa.validation not enabled",
		})
		pc.AddEvent("repair", "repair_skipped", SeverityInfo, "repair skipped in lightweight mode", map[string]string{
			"reason": "validation disabled by default in lightweight mode",
		})
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

// shouldValidate returns true when validation/repair should run. In
// lightweight mode this happens only when response_format is present or when
// explicitly enabled via novexa.validation.enabled.
func (e *Engine) shouldValidate(pc *Context) bool {
	if pc.RuntimeMode != ModeLightweight {
		return true
	}
	if pc.ToolShimActive {
		// Tool-shim responses are parsed into tool_calls; lightweight mode
		// only validates the tool calls, not the response as JSON.
		return true
	}
	if pc.NormalizedRequest.ResponseFormat != nil && pc.NormalizedRequest.ResponseFormat.Type != "" {
		return true
	}
	if pc.IncomingRequest.Novexa != nil && pc.IncomingRequest.Novexa.Validation != nil && pc.IncomingRequest.Novexa.Validation.Enabled {
		return true
	}
	return false
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

// applyInstructionAssist extracts constraints from the user prompt, injects
// explicit reminders into the system prompt, and stores constraints for
// post-generation validation. It activates when the model profile has
// prompt.instruction_assist: true or when the request is in structured mode.
//
// It scans both the last user message and the system prompt for JSON format
// requirements, so that agent frameworks that put JSON instructions in the
// system prompt (e.g. Terminus-2) also benefit from Novexa's JSON enforcement.
func (e *Engine) applyInstructionAssist(pc *Context) {
	// Activate when profile explicitly enables it, or when in structured mode.
	if pc.ModelProfile != nil && !pc.ModelProfile.Prompt.InstructionAssist && pc.RuntimeMode != ModeStructured {
		return
	}

	userMsg := latestUserMessage(pc.NormalizedRequest.Messages)
	systemMsg := latestSystemMessage(pc.NormalizedRequest.Messages)

	// Scan all available text for constraints: user message plus system prompt.
	var combinedMsg string
	if userMsg != "" && systemMsg != "" {
		combinedMsg = systemMsg + "\n" + userMsg
	} else if userMsg != "" {
		combinedMsg = userMsg
	} else if systemMsg != "" {
		combinedMsg = systemMsg
	} else {
		return
	}

	result := e.instructionEngine.Extract(combinedMsg)
	if !result.HasConstraints {
		return
	}

	pc.InstructionConstraints = result.Constraints
	pc.InstructionHintInjected = true

	// Inject constraint hints into the system prompt.
	messages := pc.NormalizedRequest.Messages
	for i, msg := range messages {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok {
				messages[i].Content = s + "\n\n" + result.HintBlock
			}
			break
		}
	}

	pc.AddEvent("instruction", "instruction_assist_applied", SeverityInfo, "instruction-following assist activated", map[string]string{
		"constraint_count": fmt.Sprintf("%d", len(result.Constraints)),
	})
}

// validateInstructions checks the provider response against extracted constraints.
// Returns true if all constraints pass. When constraints fail, it stores
// violations in the pipeline context for retry handling.
func (e *Engine) validateInstructions(pc *Context) bool {
	if len(pc.InstructionConstraints) == 0 || pc.ProviderResponse == nil {
		return true
	}

	content := ""
	if len(pc.ProviderResponse.Choices) > 0 {
		if s, ok := pc.ProviderResponse.Choices[0].Message.Content.(string); ok {
			content = s
		}
	}

	v := e.instructionEngine.Validate(content, pc.InstructionConstraints)
	if v.Passed {
		pc.AddEvent("instruction", "instruction_check_passed", SeverityInfo, "all instruction constraints satisfied", map[string]string{
			"constraints": fmt.Sprintf("%d", len(v.Satisfied)),
		})
		return true
	}

	pc.AddEvent("instruction", "instruction_check_failed", SeverityWarning, "some instruction constraints violated", map[string]string{
		"violations": strings.Join(v.Violations, "; "),
	})

	// Store violations for retry logic.
	pc.InstructionRetryCount++

	return false
}

// applyThinkingPolicy resolves whether managed thinking should be enabled for
// this request. It considers the request override first, then the profile's
// thinking_policy rules, then the legacy profile Defaults.Thinking boolean.
func (e *Engine) applyThinkingPolicy(pc *Context) {
	if pc.ModelProfile == nil {
		return
	}

	mode, reason, decided := e.resolveThinkingMode(pc)
	if !decided {
		pc.ThinkingMode = mode
		pc.ThinkingDecisionReason = reason
		pc.AddEvent("profile", "thinking_policy_applied", SeverityInfo, "no managed thinking policy or default for profile", map[string]string{
			"profile_id":      pc.ModelProfile.ID,
			"decision_reason": reason,
		})
		return
	}

	enabled := mode != "disabled"

	pc.ThinkingMode = mode
	pc.ThinkingDecisionReason = reason

	req := &pc.NormalizedRequest
	copyThinkingToRequest(req, &enabled)

	budget := pc.ModelProfile.ThinkingPolicy.ReasoningTokenBudget
	if budget <= 0 {
		budget = profiles.DefaultReasoningTokenBudget
	}
	pc.ThinkingReasoningBudget = budget

	outputBudget := defaultReservedOutput
	if req.MaxTokens != nil {
		outputBudget = *req.MaxTokens
	}

	if enabled {
		// Reserve reasoning budget from max_tokens if it is unset or large enough.
		if req.MaxTokens == nil {
			total := outputBudget + budget
			req.MaxTokens = &total
			outputBudget = total - budget
		} else if *req.MaxTokens > budget {
			outputBudget = *req.MaxTokens - budget
		}
	}
	pc.ThinkingOutputBudget = outputBudget

	pc.AddEvent("profile", "thinking_policy_applied", SeverityInfo, "applied managed thinking policy", map[string]string{
		"profile_id":        pc.ModelProfile.ID,
		"thinking_enabled":    fmt.Sprintf("%t", enabled),
		"thinking_mode":       mode,
		"decision_reason":     reason,
		"reasoning_budget":    fmt.Sprintf("%d", pc.ThinkingReasoningBudget),
		"output_budget":       fmt.Sprintf("%d", pc.ThinkingOutputBudget),
	})
}

// resolveThinkingMode returns the thinking mode and the reason for the decision.
// The second return value is true when a decision was actually made (request
// override, legacy default, or managed policy). When false, the caller should
// leave the request thinking setting unspecified.
func (e *Engine) resolveThinkingMode(pc *Context) (string, string, bool) {
	req := pc.NormalizedRequest
	policy := pc.ModelProfile.ThinkingPolicy
	requestThinking := req.Novexa.GetThinkingEnabled()

	// Request explicit disable always wins.
	if requestThinking != nil && !*requestThinking {
		return "disabled", "request_override_disabled", true
	}

	// If the profile has no managed policy and no legacy default, leave the
	// decision unspecified so the provider can use its own default.
	if policy.DefaultMode == "" && pc.ModelProfile.Defaults.Thinking == nil {
		return "disabled", "no_default", false
	}

	// Workflow guards from policy.disable_when apply even to request overrides.
	// Novexa refuses to let thinking corrupt JSON or tool workflows.
	hasTools := len(req.Tools) > 0 || len(pc.OriginalTools) > 0
	hasJSONFormat := req.ResponseFormat != nil && (req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json_schema")
	isOneWord := isOneWordRequest(req.Messages)

	disableReason := ""
	for _, item := range policy.DisableWhen {
		switch strings.ToLower(item) {
		case "tool_calling":
			if hasTools {
				disableReason = "policy_disable_tool_calling"
			}
		case "response_format_json":
			if hasJSONFormat {
				disableReason = "policy_disable_json_format"
			}
		case "one_word_answer":
			if isOneWord {
				disableReason = "policy_disable_one_word"
			}
		}
	}

	if disableReason != "" {
		if requestThinking != nil && *requestThinking {
			return "disabled", disableReason + "_overrides_request", true
		}
		return "disabled", disableReason, true
	}

	// Request explicit enable now wins because no guard blocked it.
	if requestThinking != nil && *requestThinking {
		return "full", "request_override_enabled", true
	}

	// Legacy Defaults.Thinking boolean still applies if no policy exists.
	if policy.DefaultMode == "" {
		if pc.ModelProfile.Defaults.Thinking != nil {
			if *pc.ModelProfile.Defaults.Thinking {
				return "full", "profile_default_legacy_enabled", true
			}
			return "disabled", "profile_default_legacy_disabled", true
		}
		return "disabled", "no_default", false
	}

	// Managed thinking policy.
	if !policy.Allowed {
		return "disabled", "policy_not_allowed", true
	}

	// Workflow enablers from policy.enable_when.
	for _, item := range policy.EnableWhen {
		switch strings.ToLower(item) {
		case "multi_step_task", "debugging", "unknown_domain":
			// Heuristic: longer, complex prompts get the benefit of the doubt.
			if promptTokenEstimate(req.Messages) > 200 {
				return modeForPolicy(policy.DefaultMode), "policy_enable_complex_prompt", true
			}
		case "context_too_large":
			if pc.ContextPackage != nil && pc.ContextPackage.TokenBudget.OverflowTokens > 0 {
				return modeForPolicy(policy.DefaultMode), "policy_enable_context_overflow", true
			}
		}
	}

	return modeForPolicy(policy.DefaultMode), "policy_default_mode", true
}

func modeForPolicy(mode string) string {
	switch strings.ToLower(mode) {
	case "light", "full":
		return strings.ToLower(mode)
	}
	return "disabled"
}

func isOneWordRequest(messages []api.Message) bool {
	text := latestUserMessage(messages)
	fields := strings.Fields(text)
	return len(fields) == 1 && len(text) < 20
}

func promptTokenEstimate(messages []api.Message) int {
	total := 0
	for _, m := range messages {
		total += estimateMessageTokens(m)
	}
	return total
}

func estimateMessageTokens(msg api.Message) int {
	if s, ok := msg.Content.(string); ok {
		return contextengine.EstimateText(s)
	}
	return 0
}

func thinkingPolicySource(request, profile *bool) string {
	if request != nil {
		return "request"
	}
	if profile != nil {
		return "profile"
	}
	return "unspecified"
}

// stripReasoningContent removes reasoning/thinking traces from the assistant
// response and records safe metadata. Actual reasoning text is never stored.
func (e *Engine) stripReasoningContent(pc *Context) {
	if pc.FinalResponse == nil || len(pc.FinalResponse.Choices) == 0 {
		return
	}

	msg := &pc.FinalResponse.Choices[0].Message
	content, ok := msg.Content.(string)
	if !ok {
		return
	}

	// If the provider returned reasoning in a separate field, account for it.
	if msg.ReasoningContent != "" {
		content = msg.ReasoningContent + "\n" + content
		msg.ReasoningContent = ""
	}

	result := thinkingengine.ExtractAndStrip(content)

	// Fallback: if no explicit reasoning markers found, try prose reasoning
	// stripping for local models that emit free-form reasoning without tags.
	if !result.ReasoningPresent {
		proseResult := thinkingengine.ExtractAndStripProse(content)
		if proseResult.ReasoningPresent {
			result = proseResult
			pc.AddEvent("thinking", "prose_reasoning_detected", SeverityInfo, "prose reasoning detected and stripped from response", map[string]string{
				"reasoning_length": fmt.Sprintf("%d", result.ReasoningLength),
			})
		}
	}

	msg.Content = result.CleanContent
	pc.ReasoningContentPresent = result.ReasoningPresent
	pc.ReasoningLength = result.ReasoningLength

	if result.ReasoningPresent {
		pc.AddEvent("thinking", "reasoning_content_detected", SeverityInfo, "reasoning content detected and stripped from response", map[string]string{
			"reasoning_length": fmt.Sprintf("%d", result.ReasoningLength),
		})
	}
}

// resolveThinkingTelemetry records safe metadata about thinking behaviour.
// Actual reasoning text is never stored.
func resolveThinkingTelemetry(pc *Context) *ThinkingTelemetry {
	t := &ThinkingTelemetry{
		ThinkingEnabled:         "unspecified",
		ThinkingMode:            pc.ThinkingMode,
		ThinkingDecisionReason:  pc.ThinkingDecisionReason,
		ReasoningContentPresent: pc.ReasoningContentPresent,
		ReasoningLength:         pc.ReasoningLength,
		OutputTokenBudget:       pc.ThinkingOutputBudget,
		ReasoningTokenBudget:    pc.ThinkingReasoningBudget,
	}
	if pc.NormalizedRequest.Novexa != nil && pc.NormalizedRequest.Novexa.Thinking != nil && pc.NormalizedRequest.Novexa.Thinking.Enabled != nil {
		if *pc.NormalizedRequest.Novexa.Thinking.Enabled {
			t.ThinkingEnabled = "true"
		} else {
			t.ThinkingEnabled = "false"
		}
	}
	return t
}

func (e *Engine) generateOnce(ctx context.Context, pc *Context, adapter provider.ProviderAdapter, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, Result) {
	const maxRetries = 2
	var lastErr error
	var lastNormalized provider.ProviderError

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			errMsg := lastErr.Error()
			isModelLoadErr := isModelLoadingError(errMsg)
			isFormatErr := isResponseFormatError(errMsg)

			pc.AddEvent("provider", "provider_retry", SeverityWarning, "retrying provider request after error", map[string]string{
				"attempt":          fmt.Sprintf("%d", attempt),
				"error":            errMsg,
				"model_load_error": fmt.Sprintf("%t", isModelLoadErr),
				"format_error":     fmt.Sprintf("%t", isFormatErr),
			})

			// Only strip response_format when the error specifically indicates
			// the provider rejected the format — not for model-loading failures
			// or transient unavailability. Stripping it unconditionally degrades
			// JSON quality on retries that succeed.
			if isFormatErr {
				req.ResponseFormat = nil
			}

			// Backoff: model-loading errors need much longer waits (LM Studio
			// takes 3-10s to load a model into memory). Other errors use a
			// moderate exponential backoff.
			var backoff time.Duration
			if isModelLoadErr {
				backoff = time.Duration(attempt) * 3 * time.Second
			} else {
				backoff = time.Duration(attempt) * 2 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, e.fail(pc, provider.ProviderError{
					Code:    provider.ProviderTimeout,
					Message: "context cancelled during provider retry",
					Cause:   ctx.Err(),
				}, "provider retry cancelled")
			case <-time.After(backoff):
			}
		}

		start := time.Now()
		resp, err := adapter.Generate(ctx, req)
		pc.ProviderLatency += time.Since(start)
		if err != nil {
			var normalized provider.ProviderError
			if !errors.As(err, &normalized) {
				normalized = adapter.NormalizeError(err)
			}
			lastErr = err
			lastNormalized = normalized
			// Retry on provider errors (unavailable, bad response, timeout).
			if normalized.Code == provider.ProviderUnavailable ||
				normalized.Code == provider.ProviderBadResponse ||
				normalized.Code == provider.ProviderTimeout {
				continue
			}
			return nil, e.fail(pc, normalized, "provider request failed")
		}
		return resp, Result{}
	}

	// All retries exhausted.
	return nil, e.fail(pc, lastNormalized, "provider request failed after retries")
}

// isModelLoadingError detects LM Studio / llama.cpp model-loading failures.
// These are transient — the model is being swapped into memory and the engine
// wasn't ready yet. They need a longer backoff than other retryable errors.
func isModelLoadingError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "failed to load model") ||
		strings.Contains(lower, "engine protocol startup was aborted") ||
		strings.Contains(lower, "model is still loading") ||
		strings.Contains(lower, "loading adapter")
}

// isResponseFormatError detects when a provider explicitly rejects the
// response_format parameter. Only in this case should we strip it on retry.
func isResponseFormatError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "response_format") ||
		strings.Contains(lower, "json_schema") ||
		strings.Contains(lower, "json_object") ||
		strings.Contains(lower, "unsupported parameter") ||
		strings.Contains(lower, "unknown parameter")
}

// normalizeToolResponse converts prompt-based tool calls in the assistant
// message back into OpenAI-style tool_calls when the tool shim is active.
func (e *Engine) normalizeToolResponse(pc *Context) {
	if !pc.ToolShimActive || pc.FinalResponse == nil || len(pc.FinalResponse.Choices) == 0 {
		return
	}

	msg := &pc.FinalResponse.Choices[0].Message
	content, ok := msg.Content.(string)
	if !ok {
		return
	}

	parsed := toolengine.NormalizeAssistantContent(content)
	if !parsed.IsToolCall {
		pc.AddEvent("tool", "tool_shim_plain_text", SeverityInfo, "tool shim response did not contain a tool call", map[string]string{
			"content_length": fmt.Sprintf("%d", len(content)),
		})
		return
	}

	msg.Content = ""
	msg.ToolCalls = parsed.ToolCalls
	pc.AddEvent("tool", "tool_shim_parsed", SeverityInfo, "tool shim parsed assistant content into tool_calls", map[string]string{
		"tool_count": fmt.Sprintf("%d", len(parsed.ToolCalls)),
		"tools":      toolengine.SchemaHint(pc.OriginalTools),
	})
}

// checkToolCalls validates parsed tool calls when the prompt-based shim was
// active. It returns true when there are no tool calls or all calls are valid.
func (e *Engine) checkToolCalls(pc *Context) bool {
	if !pc.ToolShimActive || pc.FinalResponse == nil || len(pc.FinalResponse.Choices) == 0 {
		return true
	}

	calls := pc.FinalResponse.Choices[0].Message.ToolCalls
	if len(calls) == 0 {
		return true
	}

	report := toolengine.ValidateToolCalls(calls, pc.OriginalTools)
	if report.Valid {
		pc.AddEvent("tool", "tool_calls_valid", SeverityInfo, "tool shim tool calls passed validation", map[string]string{
			"tool_count": fmt.Sprintf("%d", len(calls)),
		})
		return true
	}

	pc.AddEvent("tool", "tool_calls_invalid", SeverityWarning, "tool shim tool calls failed validation", map[string]string{
		"issue_count": fmt.Sprintf("%d", len(report.Issues)),
	})
	for _, issue := range report.Issues {
		pc.AddEvent("tool", issue.Code, SeverityWarning, issue.Message, map[string]string{
			"tool_name": issue.ToolCall.Function.Name,
		})
	}
	return false
}

func (e *Engine) validateRepairAndMaybeRetry(ctx context.Context, pc *Context, adapter provider.ProviderAdapter, providerReq api.ChatCompletionRequest) Result {
	report := e.validation.Validate(validationengine.Input{
		Response:       pc.FinalResponse,
		ResponseFormat: pc.NormalizedRequest.ResponseFormat,
		RuntimeMode:    string(pc.RuntimeMode),
	})
	pc.ValidationReport = &report
	toolValid := e.checkToolCalls(pc)
	pc.ValidationPassed = report.Passed && toolValid
	pc.AddEvent("validation", "validation_completed", severityForValidation(report), "Validation Engine completed response checks", map[string]string{
		"passed":          fmt.Sprintf("%t", report.Passed),
		"tool_calls_valid": fmt.Sprintf("%t", toolValid),
		"severity":        report.Severity,
		"issues":          fmt.Sprintf("%d", len(report.Issues)),
	})
	for _, issue := range report.Issues {
		pc.AddEvent("validation", string(issue.Code), SeverityWarning, issue.Message, map[string]string{
			"location": issue.Location,
		})
	}
	if report.Passed && toolValid {
		e.recordValidationReport(pc, report)
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

	// Persist the repair report so failures can be diagnosed post-hoc.
	if e.telemetry != nil && e.telemetry.Enabled() {
		e.telemetry.RecordRepairReport(context.Background(), telemetry.RepairReportRecord{
			RequestID:       pc.RequestID,
			Attempted:       repairReport.Attempted,
			Success:         repairReport.Success,
			Strategy:        repairReport.Strategy,
			RetryRequested:  repairReport.RetryRequested,
			ChangesApplied:  len(repairReport.Changes),
			RemainingIssues: len(repairReport.RemainingIssues),
		})
	}

	if repairReport.Success {
		second := e.validation.Validate(validationengine.Input{
			Response:       pc.FinalResponse,
			ResponseFormat: pc.NormalizedRequest.ResponseFormat,
			RuntimeMode:    string(pc.RuntimeMode),
		})
		toolValidAfterRepair := e.checkToolCalls(pc)
		pc.ValidationReport = &second
		pc.ValidationPassed = second.Passed && toolValidAfterRepair
		pc.AddEvent("validation", "validation_completed_after_repair", severityForValidation(second), "Validation Engine checked repaired response", map[string]string{
			"passed":           fmt.Sprintf("%t", second.Passed),
			"tool_calls_valid": fmt.Sprintf("%t", toolValidAfterRepair),
		})
		if second.Passed && toolValidAfterRepair {
			e.recordValidationReport(pc, second)
			return Result{}
		}
		report = second
	}

	if (repairReport.RetryRequested || !pc.ValidationPassed || shouldRetryValidation(report, pc)) && pc.Retry.Attempt < pc.Retry.MaxAttempts {
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
		if pc.ToolShimActive {
			retryReq.Messages = prependToolRetryInstruction(retryReq.Messages)
		}
		resp, result := e.generateOnce(ctx, pc, adapter, retryReq)
		if result.Error.Code != "" {
			return result
		}
		pc.ProviderResponse = resp
		pc.FinalResponse = resp
		if resp != nil {
			pc.SelectedModel = resp.Model
		}
		e.normalizeToolResponse(pc)
		retryReport := e.validation.Validate(validationengine.Input{
			Response:       pc.FinalResponse,
			ResponseFormat: pc.NormalizedRequest.ResponseFormat,
			RuntimeMode:    string(pc.RuntimeMode),
		})
		toolValidAfterRetry := e.checkToolCalls(pc)
		pc.ValidationReport = &retryReport
		pc.ValidationPassed = retryReport.Passed && toolValidAfterRetry
		pc.AddEvent("validation", "validation_completed_after_retry", severityForValidation(retryReport), "Validation Engine checked retried response", map[string]string{
			"passed":           fmt.Sprintf("%t", retryReport.Passed),
			"tool_calls_valid": fmt.Sprintf("%t", toolValidAfterRetry),
		})
		if retryReport.Passed && toolValidAfterRetry {
			pc.Retry.RetryHistory = append(pc.Retry.RetryHistory, RetryRecord{
				Attempt:   pc.Retry.Attempt,
				Reason:    "validation_failed",
				Strategy:  "stricter_structured_prompt",
				Result:    "success",
				Timestamp: time.Now().UTC(),
			})
			e.recordValidationReport(pc, retryReport)
			return Result{}
		}
		report = retryReport
	}

	// Build a human-readable summary of the validation issues for the error
	// details so the errors table is useful for debugging instead of storing {}.
	issueSummary := formatValidationIssues(report.Issues)

	// Persist the final validation report for post-hoc diagnosis.
	e.recordValidationReport(pc, report)

	return e.fail(pc, provider.ProviderError{
		Code:       provider.ValidationFailed,
		Message:    "model output failed validation and could not be repaired",
		Suggestion: "Try a clearer prompt, structured response_format, or a more capable local model.",
		Cause:      errors.New(issueSummary),
	}, "validation failed")
}

// recordValidationReport persists a validation report via telemetry. It is
// called at every validation outcome (pass, pass-after-repair, pass-after-retry,
// and final failure) so the validation_reports table has complete diagnostic
// data.
func (e *Engine) recordValidationReport(pc *Context, report validationengine.Report) {
	if e.telemetry == nil || !e.telemetry.Enabled() {
		return
	}
	issues := make([]telemetry.ValidationIssueRecord, len(report.Issues))
	for i, iss := range report.Issues {
		issues[i] = telemetry.ValidationIssueRecord{
			Code:     string(iss.Code),
			Message:  iss.Message,
			Location: iss.Location,
		}
	}
	e.telemetry.RecordValidationReport(context.Background(), telemetry.ValidationReportRecord{
		RequestID:               pc.RequestID,
		Passed:                  report.Passed,
		Severity:                report.Severity,
		Repairable:              report.Repairable,
		SuggestedRepairStrategy: string(report.SuggestedRepairStrategy),
		Issues:                  issues,
	})
}

// formatValidationIssues turns a slice of validation issues into a compact
// string suitable for storing in the error details_json cause field.
func formatValidationIssues(issues []validationengine.Issue) string {
	if len(issues) == 0 {
		return "no specific issues recorded"
	}
	parts := make([]string, 0, len(issues))
	for _, iss := range issues {
		loc := iss.Location
		if loc == "" {
			loc = "response"
		}
		parts = append(parts, fmt.Sprintf("%s at %s: %s", iss.Code, loc, iss.Message))
	}
	return strings.Join(parts, "; ")
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
	instruction := "Retry because the previous output failed validation. Return only valid JSON if JSON was requested. Do not include markdown fences, repeated text, or explanatory prose."
	return mergeSystemInstruction(messages, instruction)
}

func prependToolRetryInstruction(messages []api.Message) []api.Message {
	instruction := "Retry because the previous tool call was invalid. Return ONLY a JSON object like {\"tool\":\"name\",\"arguments\":{...}} with a valid tool name and arguments. Do not wrap it in markdown fences."
	return mergeSystemInstruction(messages, instruction)
}

// mergeSystemInstruction appends instruction to the first system/developer
// message if one exists, otherwise prepends a new system message.
// This prevents chat templates (e.g. LM Studio's) that require a single leading
// system message from raising "System message must be at the beginning".
func mergeSystemInstruction(messages []api.Message, instruction string) []api.Message {
	if len(messages) == 0 {
		return []api.Message{{Role: "system", Content: instruction}}
	}
	out := make([]api.Message, 0, len(messages))
	injected := false
	for _, msg := range messages {
		if !injected && (msg.Role == "system" || msg.Role == "developer") {
			if s, ok := msg.Content.(string); ok {
				msg.Content = strings.TrimSpace(s) + "\n\n" + instruction
				injected = true
			}
		}
		out = append(out, msg)
	}
	if !injected {
		out = append([]api.Message{{Role: "system", Content: instruction}}, out...)
	}
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

	// Ensure lightweight telemetry captures the resolved profile, applied
	// defaults, and skipped engines so clients can inspect what ran.
	if pc.RuntimeMode == ModeLightweight {
		pc.AddEvent("telemetry", "lightweight_telemetry_recorded", SeverityInfo, "lightweight telemetry recorded", map[string]string{
			"runtime_mode":     string(pc.RuntimeMode),
			"profile_id":       profileOrGeneric(pc.ModelProfile, "id"),
			"profile_fallback": fmt.Sprintf("%t", pc.ModelProfile != nil && pc.ModelProfile.ID == "generic-local"),
			"skipped_engines":  "context,memory,session,validation,repair,heavy_guard",
		})
	}

	e.telemetry.RecordPipelineEvents(ctx, events)

	if pc.ProviderError != nil {
		e.telemetry.RecordError(ctx, pc.RequestID, "pipeline", *pc.ProviderError)
	}
}
