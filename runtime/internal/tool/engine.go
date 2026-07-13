// Package tool implements a prompt-based tool-calling shim for local models
// that do not reliably support native OpenAI-style tool_calls.
package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/profiles"
)

// Input is the Tool Engine request.
type Input struct {
	Messages     []api.Message
	Tools        []api.Tool
	ToolChoice   interface{}
	ModelProfile *profiles.Profile
	RuntimeMode  string
}

// Output is the Tool Engine result.
type Output struct {
	FinalMessages        []api.Message
	ToolSchemaHint       string
	InstructionsInjected bool
	Warnings             []string
}

// Engine converts native tool requests into model-friendly prompt instructions.
type Engine struct{}

// New creates a Tool Engine.
func New() *Engine {
	return &Engine{}
}

// BuildInstructions returns the raw tool instruction block for the Prompt
// Engine to embed into the system prompt. It does not mutate messages.
func (e *Engine) BuildInstructions(tools []api.Tool, profile *profiles.Profile) (string, []string) {
	if len(tools) == 0 {
		return "", nil
	}
	style := resolveToolInstructionStyle(profile)
	if style == "none" {
		return "", []string{"tool instruction style is none; skipping tool prompt injection"}
	}
	block := buildToolBlock(tools, style)
	if block == "" {
		return "", []string{"tool block is empty after formatting"}
	}
	return block, nil
}

// Build injects tool instructions into the message list when tools are present.
// It returns messages with instructions appended to the system/developer prompt
// and clears the explicit tool list so the thin provider adapter does not attempt
// native tool calling.
func (e *Engine) Build(in Input) Output {
	out := Output{
		FinalMessages: append([]api.Message(nil), in.Messages...),
	}
	if len(in.Tools) == 0 {
		return out
	}

	style := resolveToolInstructionStyle(in.ModelProfile)
	if style == "none" {
		out.Warnings = append(out.Warnings, "tool instruction style is none; skipping tool prompt injection")
		return out
	}

	toolBlock := buildToolBlock(in.Tools, style)
	if toolBlock == "" {
		out.Warnings = append(out.Warnings, "tool block is empty after formatting")
		return out
	}

	out.ToolSchemaHint = SchemaHint(in.Tools)
	out.InstructionsInjected = true
	out.FinalMessages = injectSystemInstruction(out.FinalMessages, toolBlock)
	return out
}

func resolveToolInstructionStyle(p *profiles.Profile) string {
	if p != nil && p.Prompt.ToolInstructionStyle != "" {
		return strings.ToLower(p.Prompt.ToolInstructionStyle)
	}
	return "explicit"
}

func buildToolBlock(tools []api.Tool, style string) string {
	var lines []string
	lines = append(lines, "You have access to the following tools:")
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		fn := t.Function
		lines = append(lines, fmt.Sprintf("- %s: %s", fn.Name, fn.Description))
		if len(fn.Parameters) > 0 {
			schema, _ := json.Marshal(fn.Parameters)
			lines = append(lines, fmt.Sprintf("  Parameters (JSON Schema): %s", string(schema)))
		}
	}

	lines = append(lines, "")
	switch style {
	case "schema_first":
		lines = append(lines, "To use a tool, respond with a single JSON object matching the tool's parameters schema. Add the top-level key \"tool\" with the tool name.")
	case "simple":
		lines = append(lines, "To use a tool, reply with JSON like: {\"tool\":\"name\",\"arguments\":{...}}")
	default: // explicit
		lines = append(lines, "To use a tool, reply with ONLY a JSON object in this exact format:")
		lines = append(lines, `  {"tool": "<tool_name>", "arguments": {<parameters>}}`)
		lines = append(lines, "Do not wrap the JSON in markdown fences. Do not add explanatory prose before or after the JSON.")
	}
	lines = append(lines, "If no tool is needed, reply normally in plain text.")

	return strings.Join(lines, "\n")
}

// SchemaHint returns a comma-separated list of tool names for telemetry.
func SchemaHint(tools []api.Tool) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		if t.Type == "function" {
			names = append(names, t.Function.Name)
		}
	}
	return strings.Join(names, ", ")
}

func injectSystemInstruction(messages []api.Message, instruction string) []api.Message {
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
