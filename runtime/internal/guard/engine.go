// Package guard applies pre-generation reliability checks.
package guard

import (
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	"github.com/novexa/novexa/runtime/internal/provider"
)

// Decision is the Guard Engine decision.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionWarn  Decision = "warn"
	DecisionBlock Decision = "block"
)

// Report describes guard outcome.
type Report struct {
	Decision Decision `json:"decision"`
	Warnings []string `json:"warnings,omitempty"`
	Blocked  bool     `json:"blocked"`
	Reason   string   `json:"reason,omitempty"`
}

// Input is the Guard Engine request.
type Input struct {
	Messages       []api.Message
	ResponseFormat *api.ResponseFormat
	RuntimeMode    string
	ContextReport  *contextengine.Report
}

// Output is the Guard Engine result.
type Output struct {
	Report   Report
	Error    provider.ProviderError
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

	decision := DecisionAllow
	if len(warnings) > 0 {
		decision = DecisionWarn
	}
	return Output{
		Report:   Report{Decision: decision, Warnings: warnings},
		Warnings: warnings,
	}
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
