// Package profiles defines and loads Novexa model profiles.
package profiles

import (
	"fmt"
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
)

// Profile describes model-specific behaviour for one local model family.
type Profile struct {
	ID             string              `yaml:"id"`
	Name           string              `yaml:"name,omitempty"`
	Version        int                 `yaml:"version,omitempty"`
	Family         string              `yaml:"family,omitempty"`
	Size           string              `yaml:"size,omitempty"`
	Type           string              `yaml:"type,omitempty"`
	Aliases        []string            `yaml:"aliases,omitempty"`
	Models         map[string][]string `yaml:"models,omitempty"`
	ContextLimit   int                 `yaml:"context_limit,omitempty"`
	Capabilities   Capabilities        `yaml:"capabilities,omitempty"`
	Defaults       Defaults            `yaml:"defaults,omitempty"`
	Context        ContextSettings     `yaml:"context,omitempty"`
	Prompt         PromptSettings      `yaml:"prompt,omitempty"`
	Guard          GuardSettings       `yaml:"guard,omitempty"`
	ThinkingPolicy ThinkingPolicy      `yaml:"thinking_policy,omitempty"`
	Notes          []string            `yaml:"notes,omitempty"`
}

// Capabilities describes what a model is expected to handle well.
type Capabilities struct {
	Chat             bool   `yaml:"chat,omitempty"`
	Streaming        bool   `yaml:"streaming,omitempty"`
	StructuredOutput string `yaml:"structured_output,omitempty"`
	JSONMode         string `yaml:"json_mode,omitempty"`
	ToolCalling      string `yaml:"tool_calling,omitempty"`
	Vision           bool   `yaml:"vision,omitempty"`
	LongContext      string `yaml:"long_context,omitempty"`
	Coding           string `yaml:"coding,omitempty"`
	Reasoning        string `yaml:"reasoning,omitempty"`
	CreativeWriting  string `yaml:"creative_writing,omitempty"`
}

// Defaults holds recommended generation parameters.
type Defaults struct {
	Temperature   *float64 `yaml:"temperature,omitempty"`
	TopP          *float64 `yaml:"top_p,omitempty"`
	RepeatPenalty *float64 `yaml:"repeat_penalty,omitempty"`
	MaxTokens     *int     `yaml:"max_tokens,omitempty"`
	Stop          []string `yaml:"stop,omitempty"`
	Thinking      *bool    `yaml:"thinking"`
}

// ContextSettings describes how the Context Engine should treat this model.
type ContextSettings struct {
	Strategy               string `yaml:"strategy,omitempty"`
	MaxInputTokens         int    `yaml:"max_input_tokens,omitempty"`
	PreserveRecentMessages int    `yaml:"preserve_recent_messages,omitempty"`
	Compression            bool   `yaml:"compression,omitempty"`
}

// PromptSettings describes how the Prompt Engine should build instructions.
type PromptSettings struct {
	Style                string   `yaml:"style,omitempty"`
	InstructionStrength  string   `yaml:"instruction_strength,omitempty"`
	JSONInstructionStyle string   `yaml:"json_instruction_style,omitempty"`
	ToolInstructionStyle string   `yaml:"tool_instruction_style,omitempty"`
	SystemPromptStyle    string   `yaml:"system_prompt_style,omitempty"`
	Instructions         []string `yaml:"instructions,omitempty"`
	InstructionAssist    bool     `yaml:"instruction_assist,omitempty"`
}

// GuardSettings describes reliability guard preferences for the model.
type GuardSettings struct {
	AntiLoop              string `yaml:"anti_loop,omitempty"`
	ContextOverflow       bool   `yaml:"context_overflow,omitempty"`
	StructuredOutput      bool   `yaml:"structured_output,omitempty"`
	JSONRepair            bool   `yaml:"json_repair,omitempty"`
	RepetitionDetection   bool   `yaml:"repetition_detection,omitempty"`
	ToolCallValidation    bool   `yaml:"tool_call_validation,omitempty"`
	ToolCallLoopDetection bool   `yaml:"tool_call_loop_detection,omitempty"`
}

// ThinkingPolicy controls when and how a model may use internal reasoning.
// Reasoning text is never stored by Novexa; only safe metadata is recorded.
type ThinkingPolicy struct {
	Allowed              bool     `yaml:"allowed,omitempty"`
	DefaultMode          string   `yaml:"default_mode,omitempty"`
	StripReasoning       bool     `yaml:"strip_reasoning,omitempty"`
	ReasoningTokenBudget int      `yaml:"reasoning_token_budget,omitempty"`
	EnableWhen           []string `yaml:"enable_when,omitempty"`
	DisableWhen          []string `yaml:"disable_when,omitempty"`
}

// DefaultReasoningTokenBudget is the number of tokens reserved for reasoning
// when a profile enables managed thinking but does not specify a budget.
const DefaultReasoningTokenBudget = 2048

// Validate checks that a loaded profile is internally consistent.
// Invalid profiles are skipped by the loader rather than crashing the runtime.
func Validate(p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("profile is missing required field: id")
	}
	if p.Version <= 0 {
		return fmt.Errorf("profile %q is missing a valid version", p.ID)
	}
	for _, cap := range []string{
		p.Capabilities.StructuredOutput,
		p.Capabilities.ToolCalling,
		p.Capabilities.LongContext,
		p.Capabilities.Coding,
		p.Capabilities.Reasoning,
		p.Capabilities.CreativeWriting,
		p.Capabilities.JSONMode,
	} {
		if cap != "" && !isQualityCapability(cap) {
			return fmt.Errorf("profile %q has unknown capability value %q", p.ID, cap)
		}
	}
	if p.Guard.AntiLoop != "" && !isAntiLoopLevel(p.Guard.AntiLoop) {
		return fmt.Errorf("profile %q has unknown anti_loop level %q", p.ID, p.Guard.AntiLoop)
	}
	if p.Context.Strategy != "" && !isContextStrategy(p.Context.Strategy) {
		return fmt.Errorf("profile %q has unknown context strategy %q", p.ID, p.Context.Strategy)
	}
	if p.Prompt.Style != "" && !isPromptStyle(p.Prompt.Style) {
		return fmt.Errorf("profile %q has unknown prompt style %q", p.ID, p.Prompt.Style)
	}
	if p.Prompt.InstructionStrength != "" && !isInstructionStrength(p.Prompt.InstructionStrength) {
		return fmt.Errorf("profile %q has unknown instruction_strength %q", p.ID, p.Prompt.InstructionStrength)
	}
	if p.Prompt.JSONInstructionStyle != "" && !isJSONInstructionStyle(p.Prompt.JSONInstructionStyle) {
		return fmt.Errorf("profile %q has unknown json_instruction_style %q", p.ID, p.Prompt.JSONInstructionStyle)
	}
	if p.Prompt.ToolInstructionStyle != "" && !isToolInstructionStyle(p.Prompt.ToolInstructionStyle) {
		return fmt.Errorf("profile %q has unknown tool_instruction_style %q", p.ID, p.Prompt.ToolInstructionStyle)
	}
	if p.ThinkingPolicy.DefaultMode != "" && !isThinkingMode(p.ThinkingPolicy.DefaultMode) {
		return fmt.Errorf("profile %q has unknown thinking_policy.default_mode %q", p.ID, p.ThinkingPolicy.DefaultMode)
	}
	return nil
}

func isQualityCapability(v string) bool {
	switch strings.ToLower(v) {
	case "none", "weak", "medium", "strong", "unknown":
		return true
	}
	return false
}

func isAntiLoopLevel(v string) bool {
	switch strings.ToLower(v) {
	case "off", "light", "standard", "aggressive":
		return true
	}
	return false
}

func isContextStrategy(v string) bool {
	switch strings.ToLower(v) {
	case "none", "trim", "summarize", "compress", "hybrid":
		return true
	}
	return false
}

func isPromptStyle(v string) bool {
	switch strings.ToLower(v) {
	case "general", "technical", "coding", "reasoning", "creative", "structured":
		return true
	}
	return false
}

func isInstructionStrength(v string) bool {
	switch strings.ToLower(v) {
	case "light", "standard", "strong", "strict":
		return true
	}
	return false
}

func isJSONInstructionStyle(v string) bool {
	switch strings.ToLower(v) {
	case "none", "simple", "explicit", "schema_first":
		return true
	}
	return false
}

func isToolInstructionStyle(v string) bool {
	switch strings.ToLower(v) {
	case "none", "simple", "explicit", "schema_first":
		return true
	}
	return false
}

func isThinkingMode(v string) bool {
	switch strings.ToLower(v) {
	case "disabled", "light", "full":
		return true
	}
	return false
}

// ApplyDefaults fills unset generation fields on a provider request with values
// from the selected profile. Explicit user values are never overwritten.
func ApplyDefaults(p *Profile, req *api.ChatCompletionRequest) {
	if p == nil || req == nil {
		return
	}
	if p.Defaults.Temperature != nil && req.Temperature == nil {
		v := float32(*p.Defaults.Temperature)
		req.Temperature = &v
	}
	if p.Defaults.TopP != nil && req.TopP == nil {
		v := float32(*p.Defaults.TopP)
		req.TopP = &v
	}
	if p.Defaults.MaxTokens != nil && req.MaxTokens == nil {
		v := *p.Defaults.MaxTokens
		req.MaxTokens = &v
	}
	if len(p.Defaults.Stop) > 0 && req.Stop == nil {
		req.Stop = p.Defaults.Stop
	}
}

// GenericFallback returns a safe built-in profile used when no match exists.
func GenericFallback() *Profile {
	temperature := 0.5
	topP := 0.9
	repeatPenalty := 1.1
	maxTokens := 2048
	return &Profile{
		ID:      "generic-local",
		Name:    "Generic Local Model",
		Version: 1,
		Family:  "unknown",
		Size:    "unknown",
		Type:    "instruct",
		Capabilities: Capabilities{
			Chat:             true,
			Streaming:        false,
			StructuredOutput: "weak",
			JSONMode:         "weak",
			ToolCalling:      "unknown",
			Vision:           false,
			LongContext:      "unknown",
			Coding:           "unknown",
			Reasoning:        "unknown",
			CreativeWriting:  "unknown",
		},
		Defaults: Defaults{
			Temperature:   &temperature,
			TopP:          &topP,
			RepeatPenalty: &repeatPenalty,
			MaxTokens:     &maxTokens,
		},
		Context: ContextSettings{
			Strategy:       "hybrid",
			MaxInputTokens: 8000,
			Compression:    true,
		},
		Prompt: PromptSettings{
			Style:                "general",
			InstructionStrength:  "standard",
			JSONInstructionStyle: "explicit",
			SystemPromptStyle:    "direct",
		},
		Guard: GuardSettings{
			AntiLoop:            "standard",
			ContextOverflow:     true,
			StructuredOutput:    true,
			JSONRepair:          true,
			RepetitionDetection: true,
		},
		Notes: []string{"Unknown model profile. Conservative settings applied."},
	}
}
