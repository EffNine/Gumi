package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
)

// ValidationIssue describes a problem with a tool call.
type ValidationIssue struct {
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	ToolCall api.ToolCall `json:"tool_call"`
}

// ValidationReport is the result of checking parsed tool calls.
type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateToolCalls checks that every tool call references a known tool and
// that arguments are valid JSON. It performs a best-effort schema check on
// required top-level keys for V1.
func ValidateToolCalls(calls []api.ToolCall, tools []api.Tool) ValidationReport {
	report := ValidationReport{Valid: true}
	if len(calls) == 0 {
		return report
	}

	toolMap := make(map[string]api.Tool, len(tools))
	for _, t := range tools {
		if t.Type == "function" {
			toolMap[t.Function.Name] = t
		}
	}

	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			report.addIssue("MISSING_TOOL_NAME", "tool call is missing a function name", call)
			continue
		}
		tool, ok := toolMap[name]
		if !ok {
			report.addIssue("UNKNOWN_TOOL", fmt.Sprintf("tool %q is not in the available tools list", name), call)
			continue
		}

		args := strings.TrimSpace(call.Function.Arguments)
		if args == "" {
			report.addIssue("EMPTY_ARGUMENTS", fmt.Sprintf("tool %q call has empty arguments", name), call)
			continue
		}

		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(args), &decoded); err != nil {
			report.addIssue("INVALID_JSON_ARGUMENTS", fmt.Sprintf("tool %q arguments are not valid JSON: %v", name, err), call)
			continue
		}

		if issue := checkRequiredKeys(tool, decoded); issue != "" {
			report.addIssue("MISSING_REQUIRED_ARGUMENT", fmt.Sprintf("tool %q: %s", name, issue), call)
		}
	}

	return report
}

func (r *ValidationReport) addIssue(code, message string, call api.ToolCall) {
	r.Valid = false
	r.Issues = append(r.Issues, ValidationIssue{
		Code:     code,
		Message:  message,
		ToolCall: call,
	})
}

// checkRequiredKeys verifies that the decoded arguments contain any required
// top-level keys declared in the tool's JSON Schema parameters.
func checkRequiredKeys(tool api.Tool, args map[string]interface{}) string {
	if tool.Type != "function" || len(tool.Function.Parameters) == 0 {
		return ""
	}
	req, ok := tool.Function.Parameters["required"].([]interface{})
	if !ok {
		return ""
	}
	for _, r := range req {
		key, ok := r.(string)
		if !ok {
			continue
		}
		if _, present := args[key]; !present {
			return fmt.Sprintf("missing required argument %q", key)
		}
	}
	return ""
}

// HasDuplicateToolCall returns true if the same tool call (name + arguments)
// appears in the recent assistant/tool history.
func HasDuplicateToolCall(call api.ToolCall, history []api.Message) bool {
	if call.Function.Name == "" {
		return false
	}
	target := canonicalToolSignature(call)
	for _, msg := range history {
		if msg.Role != "assistant" {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if canonicalToolSignature(tc) == target {
				return true
			}
		}
	}
	return false
}

func canonicalToolSignature(call api.ToolCall) string {
	return call.Function.Name + "|" + strings.TrimSpace(call.Function.Arguments)
}
