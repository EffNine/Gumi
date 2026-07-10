// Package pipeline coordinates the Novexa request lifecycle.
//
// Sprint 4 introduces the Pipeline Engine as the required path between the
// Gateway Engine and Provider Engine. Later sprints will attach context,
// prompt, validation, repair, telemetry, and plugin engines to this package.
package pipeline

import (
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	promptengine "github.com/novexa/novexa/runtime/internal/prompt"
	"github.com/novexa/novexa/runtime/internal/provider"
)

// Severity describes the importance of a pipeline event.
type Severity string

const (
	SeverityDebug   Severity = "debug"
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityFatal   Severity = "fatal"
)

// RuntimeMode controls how much processing the Pipeline performs.
type RuntimeMode string

const (
	ModeDirect     RuntimeMode = "direct"
	ModeStabilized RuntimeMode = "stabilized"
	ModeStructured RuntimeMode = "structured"
	ModeAgent      RuntimeMode = "agent"
)

// Event records a significant pipeline action.
type Event struct {
	Timestamp time.Time         `json:"timestamp"`
	RequestID string            `json:"request_id"`
	Engine    string            `json:"engine"`
	Event     string            `json:"event"`
	Severity  Severity          `json:"severity"`
	Message   string            `json:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// RetryState records Sprint 4 retry scaffolding. Real retry behavior is added
// when validation and repair engines exist.
type RetryState struct {
	Attempt      int           `json:"attempt"`
	MaxAttempts  int           `json:"max_attempts"`
	RetryReason  string        `json:"retry_reason,omitempty"`
	RetryHistory []RetryRecord `json:"retry_history,omitempty"`
}

// RetryRecord describes one retry decision.
type RetryRecord struct {
	Attempt        int       `json:"attempt"`
	Reason         string    `json:"reason"`
	Strategy       string    `json:"strategy"`
	ChangesApplied []string  `json:"changes_applied,omitempty"`
	Result         string    `json:"result,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// Context is the single source of truth during one chat completion request.
type Context struct {
	RequestID string `json:"request_id"`
	TraceID   string `json:"trace_id,omitempty"`

	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id,omitempty"`

	RuntimeMode RuntimeMode `json:"runtime_mode"`
	Stream      bool        `json:"stream"`

	IncomingRequest   api.ChatCompletionRequest `json:"incoming_request"`
	NormalizedRequest api.ChatCompletionRequest `json:"normalized_request"`
	ConfigSnapshot    *config.Config            `json:"-"`

	MessagesOriginal   []api.Message `json:"messages_original,omitempty"`
	MessagesNormalized []api.Message `json:"messages_normalized,omitempty"`
	MessagesCompressed []api.Message `json:"messages_compressed,omitempty"`

	ContextPackage    *contextengine.Package `json:"context_package,omitempty"`
	ContextReport     *contextengine.Report  `json:"context_report,omitempty"`
	ContextCompressed bool                   `json:"context_compressed"`

	PromptPackage *promptengine.Package `json:"prompt_package,omitempty"`
	PromptReport  *promptengine.Report  `json:"prompt_report,omitempty"`

	RequestedModel   string `json:"requested_model"`
	SelectedProvider string `json:"selected_provider,omitempty"`
	SelectedModel    string `json:"selected_model,omitempty"`

	ProviderResponse *api.ChatCompletionResponse `json:"provider_response,omitempty"`
	ProviderError    *provider.ProviderError     `json:"provider_error,omitempty"`
	ProviderLatency  time.Duration               `json:"provider_latency,omitempty"`

	FinalResponse *api.ChatCompletionResponse `json:"final_response,omitempty"`

	Retry RetryState `json:"retry"`

	Events   []Event  `json:"events"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

// AddEvent appends a pipeline event to the context.
func (c *Context) AddEvent(engine, event string, severity Severity, message string, metadata map[string]string) {
	c.Events = append(c.Events, Event{
		Timestamp: time.Now().UTC(),
		RequestID: c.RequestID,
		Engine:    engine,
		Event:     event,
		Severity:  severity,
		Message:   message,
		Metadata:  metadata,
	})
}
