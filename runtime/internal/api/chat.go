package api

// ChatCompletionRequest mirrors the OpenAI chat completions request body.
// Unsupported fields are preserved so future provider adapters can pass them
// through safely.
type ChatCompletionRequest struct {
	Model            string            `json:"model"`
	Messages         []Message         `json:"messages"`
	Temperature      *float32          `json:"temperature,omitempty"`
	TopP             *float32          `json:"top_p,omitempty"`
	MaxTokens        *int              `json:"max_tokens,omitempty"`
	Stream           bool              `json:"stream,omitempty"`
	Stop             interface{}       `json:"stop,omitempty"`
	PresencePenalty  *float32          `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32          `json:"frequency_penalty,omitempty"`
	ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`
	Tools            []Tool            `json:"tools,omitempty"`
	ToolChoice       interface{}       `json:"tool_choice,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`

	// Novexa extensions. OpenAI-compatible clients that do not send this
	// field will ignore it safely.
	Novexa *NovexaExtensions `json:"novexa,omitempty"`
}

// NovexaExtensions holds Novexa-specific request overrides.
type NovexaExtensions struct {
	Mode       string              `json:"mode,omitempty"`
	Guard      *GuardConfig        `json:"guard,omitempty"`
	Context    *ContextConfig      `json:"context,omitempty"`
	Validation *ValidationConfig   `json:"validation,omitempty"`
	Session    *SessionConfig      `json:"session,omitempty"`
	Telemetry  *TelemetryExtension `json:"telemetry,omitempty"`
}

// GuardConfig holds per-request guard overrides.
type GuardConfig struct {
	AntiLoop         bool `json:"anti_loop,omitempty"`
	StructuredOutput bool `json:"structured_output,omitempty"`
	ContextOverflow  bool `json:"context_overflow,omitempty"`
}

// ContextConfig holds per-request context overrides.
type ContextConfig struct {
	Strategy               string `json:"strategy,omitempty"`
	MaxInputTokens         int    `json:"max_input_tokens,omitempty"`
	PreserveRecentMessages int    `json:"preserve_recent_messages,omitempty"`
}

// ValidationConfig holds per-request validation overrides.
type ValidationConfig struct {
	Enabled bool `json:"enabled,omitempty"`
	Repair  bool `json:"repair,omitempty"`
}

// SessionConfig holds per-request session overrides.
type SessionConfig struct {
	ID      string `json:"id,omitempty"`
	Persist bool   `json:"persist,omitempty"`
}

// TelemetryExtension holds per-request telemetry overrides.
type TelemetryExtension struct {
	IncludeMetadata       bool `json:"include_metadata,omitempty"`
	IncludePipelineEvents bool `json:"include_pipeline_events,omitempty"`
}

// Message represents a chat message in OpenAI format.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ResponseFormat describes the requested response format.
type ResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *JSONSchemaSpec `json:"json_schema,omitempty"`
}

// JSONSchemaSpec wraps a JSON schema definition.
type JSONSchemaSpec struct {
	Name   string                 `json:"name"`
	Schema map[string]interface{} `json:"schema"`
	Strict bool                   `json:"strict,omitempty"`
}

// Tool describes a callable tool.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction holds the function definition for a tool.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool invocation in an assistant message.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ChatCompletionResponse is the OpenAI-compatible non-streaming response.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`

	// Novexa metadata is only included when explicitly requested.
	Novexa *NovexaMetadata `json:"novexa,omitempty"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
}

// Usage reports token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NovexaMetadata reports runtime metadata when requested.
type NovexaMetadata struct {
	RequestID         string `json:"request_id"`
	Provider          string `json:"provider"`
	RuntimeMode       string `json:"runtime_mode"`
	ContextCompressed bool   `json:"context_compressed"`
	ValidationPassed  bool   `json:"validation_passed"`
	RepairApplied     bool   `json:"repair_applied"`
	RetryCount        int    `json:"retry_count"`
	LatencyMs         int64  `json:"latency_ms"`
}

// ChatCompletionChunk is a single Server-Sent Events chunk for streaming.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// ChunkChoice is a streaming choice fragment.
type ChunkChoice struct {
	Index        int         `json:"index"`
	Delta        Message     `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
}
