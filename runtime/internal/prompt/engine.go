// Package promptengine builds model-ready prompts for Novexa requests.
package promptengine

import (
	"encoding/json"
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
	"github.com/novexa/novexa/runtime/internal/profiles"
)

// Input is the Prompt Engine request.
type Input struct {
	RuntimeMode    string
	Messages       []api.Message
	ContextPackage contextengine.Package
	ResponseFormat *api.ResponseFormat
	ExistingSystem []string
	ModelProfile   *profiles.Profile
}

// Output is the Prompt Engine result.
type Output struct {
	Package       Package
	FinalMessages []api.Message
	Report        Report
	Warnings      []string
}

// Package describes the prompt assembled for the provider.
type Package struct {
	SystemPrompt               string        `json:"system_prompt,omitempty"`
	DeveloperInstructions      []string      `json:"developer_instructions,omitempty"`
	ContextBlock               string        `json:"context_block,omitempty"`
	ResponseFormatInstructions string        `json:"response_format_instructions,omitempty"`
	ModelProfileInstructions   []string      `json:"model_profile_instructions,omitempty"`
	FinalMessages              []api.Message `json:"final_messages,omitempty"`
}

// Report describes what the Prompt Engine changed.
type Report struct {
	SystemPromptAdded          bool     `json:"system_prompt_added"`
	ResponseFormatApplied      bool     `json:"response_format_applied"`
	ProfileInstructionsApplied bool     `json:"profile_instructions_applied"`
	FinalMessageCount          int      `json:"final_message_count"`
	Warnings                   []string `json:"warnings,omitempty"`
}

// Engine builds provider-ready message arrays.
type Engine struct{}

// New creates a Prompt Engine.
func New() *Engine {
	return &Engine{}
}

// Build assembles a clear system prompt and final message list.
func (e *Engine) Build(in Input) Output {
	system := buildSystemPrompt(in)
	contextBlock := buildContextBlock(in.ContextPackage)
	formatInstructions := buildFormatInstructions(in.ResponseFormat)
	profileInstructions := buildProfileInstructions(in.ModelProfile)
	warnings := []string{}

	parts := []string{system}
	if contextBlock != "" {
		parts = append(parts, contextBlock)
	}
	if formatInstructions != "" {
		parts = append(parts, formatInstructions)
	}
	if len(profileInstructions) > 0 {
		parts = append(parts, strings.Join(profileInstructions, "\n"))
	}
	finalSystem := strings.Join(parts, "\n\n")

	final := make([]api.Message, 0, len(in.Messages)+1)
	systemAdded := false
	if strings.TrimSpace(finalSystem) != "" {
		final = append(final, api.Message{Role: "system", Content: finalSystem})
		systemAdded = true
	}
	for _, msg := range in.Messages {
		if msg.Role == "system" {
			continue
		}
		final = append(final, msg)
	}

	return Output{
		Package: Package{
			SystemPrompt:               finalSystem,
			DeveloperInstructions:      in.ExistingSystem,
			ContextBlock:               contextBlock,
			ResponseFormatInstructions: formatInstructions,
			ModelProfileInstructions:   profileInstructions,
			FinalMessages:              final,
		},
		FinalMessages: final,
		Report: Report{
			SystemPromptAdded:          systemAdded,
			ResponseFormatApplied:      formatInstructions != "",
			ProfileInstructionsApplied: len(profileInstructions) > 0,
			FinalMessageCount:          len(final),
			Warnings:                   warnings,
		},
		Warnings: warnings,
	}
}

func buildSystemPrompt(in Input) string {
	lines := []string{
		"You are responding through Novexa Runtime, a local-first AI runtime layer.",
		"Answer the user's current request directly and clearly.",
		"Preserve the user's intent and do not invent facts.",
		"If information is missing or uncertain, say so instead of guessing.",
	}
	if in.RuntimeMode == "structured" {
		lines = append(lines, "Return only the requested structured output. Avoid prose outside the structure.")
	} else if in.ResponseFormat == nil || in.ResponseFormat.Type == "" {
		lines = append(lines, "Do not convert plain-text answers into JSON, YAML, XML, or another structured format unless the user explicitly asks for that format.")
		lines = append(lines, "If the user requests one word, one token, or an exact format, output only that requested content.")
	}
	for _, existing := range in.ExistingSystem {
		if strings.TrimSpace(existing) != "" {
			lines = append(lines, "User/system instruction: "+strings.TrimSpace(existing))
		}
	}
	return strings.Join(lines, "\n")
}

func buildProfileInstructions(p *profiles.Profile) []string {
	if p == nil || len(p.Prompt.Instructions) == 0 {
		return nil
	}
	var result []string
	for _, line := range p.Prompt.Instructions {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func buildContextBlock(pkg contextengine.Package) string {
	var lines []string
	if pkg.ActiveRequest != "" {
		lines = append(lines, "Current request: "+pkg.ActiveRequest)
	}
	if len(pkg.PreservedFacts) > 0 {
		lines = append(lines, "Preserved facts:")
		for _, fact := range pkg.PreservedFacts {
			lines = append(lines, "- "+fact)
		}
	}
	if pkg.OmittedContentSummary != "" {
		lines = append(lines, "Omitted context: "+pkg.OmittedContentSummary)
	}
	if len(lines) == 0 {
		return ""
	}
	return "Novexa context package:\n" + strings.Join(lines, "\n")
}

func buildFormatInstructions(format *api.ResponseFormat) string {
	if format == nil || format.Type == "" {
		return ""
	}
	switch format.Type {
	case "json_object":
		return "Response format: return a valid JSON object. Do not wrap it in markdown fences."
	case "json_schema":
		schema := ""
		if format.JSONSchema != nil {
			if data, err := json.Marshal(format.JSONSchema.Schema); err == nil && len(data) > 0 {
				schema = string(data)
			}
		}
		if schema == "" {
			return "Response format: return JSON matching the requested schema. Do not wrap it in markdown fences."
		}
		return "Response format: return JSON matching this schema. Do not wrap it in markdown fences.\nSchema: " + schema
	default:
		return "Response format: follow requested response_format type " + format.Type + "."
	}
}
