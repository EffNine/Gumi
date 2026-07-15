// Package guard applies pre-generation reliability checks.
package guard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/runtime/internal/api"
	contextengine "github.com/EffNine/gumi/runtime/internal/context"
	"github.com/EffNine/gumi/runtime/internal/profiles"
	"github.com/EffNine/gumi/runtime/internal/provider"
)

// GuardErrorCode is a stable code for guard-specific failures.
type GuardErrorCode string

const (
	StepLimitExceeded GuardErrorCode = "AGENT_STEP_LIMIT_EXCEEDED"
	ToolCallLoop      GuardErrorCode = "AGENT_TOOL_CALL_LOOP"
	InvalidToolCall   GuardErrorCode = "AGENT_INVALID_TOOL_CALL"
)

// GuardError is a structured error returned by the guard engine for
// agent-specific failure paths. It implements the error interface.
type GuardError struct {
	Code       GuardErrorCode
	Message    string
	Suggestion string
}

func (e GuardError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Decision is the Guard Engine decision.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionWarn  Decision = "warn"
	DecisionBlock Decision = "block"
)

// Report describes guard outcome.
type Report struct {
	Decision       Decision `json:"decision"`
	Warnings       []string `json:"warnings,omitempty"`
	Blocked        bool     `json:"blocked"`
	Reason         string   `json:"reason,omitempty"`
	AppliedProfile bool     `json:"applied_profile,omitempty"`
	ProfileID      string   `json:"profile_id,omitempty"`
	AntiLoopLevel  string   `json:"anti_loop_level,omitempty"`
}

// Input is the Guard Engine request.
type Input struct {
	Messages       []api.Message
	ResponseFormat *api.ResponseFormat
	RuntimeMode    string
	ContextReport  *contextengine.Report
	ModelProfile   *profiles.Profile
}

// AgentInput is the agent-specific guard configuration.
type AgentInput struct {
	MaxSteps      int
	LoopDetection string
}

// Output is the Guard Engine result.
type Output struct {
	Report   Report
	Error    error
	Warnings []string
}

// Engine applies deterministic V1 guardrails.
type Engine struct{}

// New creates a Guard Engine.
func New() *Engine {
	return &Engine{}
}

// Check validates that the prompt is usable and records guard warnings.
func (e *Engine) Check(in Input) Output {
	if latestUserMessage(in.Messages) == "" {
		err := provider.ProviderError{
			Code:       provider.EmptyPrompt,
			Message:    "the prompt is empty after normalization",
			Suggestion: "Provide a non-empty user message.",
		}
		return Output{
			Report: Report{Decision: DecisionBlock, Blocked: true, Reason: string(provider.EmptyPrompt)},
			Error:  err,
		}
	}

	warnings := []string{}
	if in.ContextReport != nil && len(in.ContextReport.Warnings) > 0 {
		warnings = append(warnings, in.ContextReport.Warnings...)
	}
	if in.RuntimeMode == "structured" || (in.ResponseFormat != nil && in.ResponseFormat.Type != "") {
		warnings = append(warnings, "structured output validation enabled")
	}
	if hasToolCallLoop(in.Messages) {
		warnings = append(warnings, "tool call loop detected in conversation history")
	}

	report := Report{Decision: DecisionAllow, Warnings: warnings}
	if in.ModelProfile != nil {
		report.AppliedProfile = true
		report.ProfileID = in.ModelProfile.ID
		report.AntiLoopLevel = in.ModelProfile.Guard.AntiLoop
		if in.ModelProfile.Guard.AntiLoop == "aggressive" {
			warnings = append(warnings, "profile recommends aggressive anti-loop settings")
		}
		if in.ModelProfile.Guard.RepetitionDetection {
			warnings = append(warnings, "profile repetition detection enabled")
		}
		if in.ModelProfile.Guard.JSONRepair && (in.RuntimeMode == "structured" || (in.ResponseFormat != nil && in.ResponseFormat.Type != "")) {
			warnings = append(warnings, "profile JSON repair enabled")
		}
	}

	decision := DecisionAllow
	if len(warnings) > 0 {
		decision = DecisionWarn
	}
	report.Decision = decision
	report.Warnings = warnings
	return Output{
		Report:   report,
		Warnings: warnings,
	}
}

// CheckAgent validates agent-mode requests with step budget and tool-call
// loop detection. It returns a block decision when the step budget is exceeded
// or when strict loop detection finds 3+ repeated tool calls.
// Agent-specific failure paths return GuardError instead of ProviderError.
func (e *Engine) CheckAgent(in Input, agentIn AgentInput) Output {
	// Step budget check: count assistant messages. If >= maxSteps, block.
	assistantCount := 0
	for _, msg := range in.Messages {
		if msg.Role == "assistant" {
			assistantCount++
		}
	}
	if agentIn.MaxSteps > 0 && assistantCount >= agentIn.MaxSteps {
		return Output{
			Report: Report{
				Decision: DecisionBlock,
				Blocked:  true,
				Reason:   string(StepLimitExceeded),
			},
			Error: GuardError{
				Code:       StepLimitExceeded,
				Message:    fmt.Sprintf("Agent step budget exhausted (%d/%d). Reset the session or increase max_steps.", assistantCount, agentIn.MaxSteps),
				Suggestion: "Reset the session or increase max_steps in the agent configuration.",
			},
		}
	}

	// Tool-call loop check: count repeated tool calls.
	warnings := []string{}
	loopDetected := false
	loopCount := countRepeatedToolCalls(in.Messages)
	if loopCount > 0 {
		loopDetected = true
		warnings = append(warnings, fmt.Sprintf("tool call loop detected: same tool call repeated %d times", loopCount))
	}

	// Strict loop detection: 3+ repetitions → block.
	loopDetection := strings.ToLower(agentIn.LoopDetection)
	if loopDetected && loopCount >= 3 && (loopDetection == "strict" || loopDetection == "aggressive") {
		return Output{
			Report: Report{
				Decision: DecisionBlock,
				Blocked:  true,
				Reason:   string(ToolCallLoop),
				Warnings: warnings,
			},
			Error: GuardError{
				Code:       ToolCallLoop,
				Message:    fmt.Sprintf("Agent tool call loop detected: same tool call repeated %d times. The agent framework must intervene.", loopCount),
				Suggestion: "The agent is repeating the same tool call. Try a different approach or report the blockage.",
			},
		}
	}

	// Standard loop detection: 2+ repetitions → warn.
	if loopDetected && loopCount >= 2 {
		warnings = append(warnings, "You appear to be repeating the same tool call. Try a different approach or report the blockage to the user.")
	}

	report := Report{
		Decision: DecisionAllow,
		Warnings: warnings,
	}
	if len(warnings) > 0 {
		report.Decision = DecisionWarn
	}
	return Output{
		Report:   report,
		Warnings: warnings,
	}
}

// normalizeToolCallArgs attempts to parse JSON arguments and re-marshal them
// with canonical key ordering so that semantically identical arguments produce
// the same string. On any parse failure it falls back to the trimmed raw string.
func normalizeToolCallArgs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return trimmed
	}
	// Re-marshal with sorted keys for canonical form.
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return trimmed
	}
	return string(normalized)
}

// toolCallSignature builds a canonical signature for a tool call, normalizing
// JSON arguments so that key ordering and whitespace differences are ignored.
func toolCallSignature(call api.ToolCall) string {
	return call.Function.Name + "\x00" + normalizeToolCallArgs(call.Function.Arguments)
}

// countRepeatedToolCalls returns the maximum repetition count of any tool call
// (same name + arguments) across all assistant messages.
func countRepeatedToolCalls(messages []api.Message) int {
	counts := map[string]int{}
	maxCount := 0
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, call := range msg.ToolCalls {
			if call.Function.Name == "" {
				continue
			}
			sig := toolCallSignature(call)
			counts[sig]++
			if counts[sig] > maxCount {
				maxCount = counts[sig]
			}
		}
	}
	return maxCount
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

// hasToolCallLoop returns true if an assistant message repeats a tool call
// (same name and arguments) that already appeared in an earlier assistant
// message in the same conversation.
func hasToolCallLoop(messages []api.Message) bool {
	seen := map[string]bool{}
	for _, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, call := range msg.ToolCalls {
			if call.Function.Name == "" {
				continue
			}
			sig := toolCallSignature(call)
			if seen[sig] {
				return true
			}
			seen[sig] = true
		}
	}
	return false
}
