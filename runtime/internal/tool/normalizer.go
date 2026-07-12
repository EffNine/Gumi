package tool

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
)

// ParsedAssistantResult describes what the assistant message contains after
// normalization: plain text, one or more tool calls, or both.
type ParsedAssistantResult struct {
	Content   string
	ToolCalls []api.ToolCall
	IsToolCall bool
}

// NormalizeAssistantContent parses raw assistant content and extracts any
// prompt-based tool calls. It returns plain text if no tool call is found.
func NormalizeAssistantContent(content string) ParsedAssistantResult {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ParsedAssistantResult{Content: ""}
	}

	candidate := extractJSONCandidate(trimmed)
	if candidate == "" {
		return ParsedAssistantResult{Content: trimmed}
	}

	// Try single tool call object: {"tool":"name","arguments":{...}}
	var single struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(candidate), &single); err == nil && single.Tool != "" {
		args, _ := json.Marshal(single.Arguments)
		return ParsedAssistantResult{
			ToolCalls: []api.ToolCall{newToolCall(single.Tool, string(args))},
			IsToolCall: true,
		}
	}

	// Try array of tool call objects.
	var arr []struct {
		Tool      string                 `json:"tool"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(candidate), &arr); err == nil && len(arr) > 0 {
		var calls []api.ToolCall
		for _, item := range arr {
			if item.Tool == "" {
				continue
			}
			args, _ := json.Marshal(item.Arguments)
			calls = append(calls, newToolCall(item.Tool, string(args)))
		}
		if len(calls) > 0 {
			return ParsedAssistantResult{ToolCalls: calls, IsToolCall: true}
		}
	}

	// Try native OpenAI tool_calls shape embedded in JSON.
	var native struct {
		ToolCalls []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(candidate), &native); err == nil && len(native.ToolCalls) > 0 {
		var calls []api.ToolCall
		for _, tc := range native.ToolCalls {
			calls = append(calls, api.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: api.ToolFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		return ParsedAssistantResult{ToolCalls: calls, IsToolCall: true}
	}

	return ParsedAssistantResult{Content: trimmed}
}

func newToolCall(name, args string) api.ToolCall {
	return api.ToolCall{
		ID:   fmt.Sprintf("call_%s_%d", name, nowMs()),
		Type: "function",
		Function: api.ToolFunction{
			Name:      name,
			Arguments: args,
		},
	}
}

// nowMs returns a monotonic-ish identifier string. The exact value does not
// matter as long as IDs are unique within a request.
func nowMs() int64 {
	return time.Now().UnixMilli()
}

func extractJSONCandidate(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	// Strip markdown fences if present.
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	// Quick heuristic: if it doesn't look like JSON, skip.
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return ""
	}
	// Extract outermost object/array by brace matching.
	var start int
	if strings.HasPrefix(trimmed, "[") {
		start = 0
	} else {
		start = strings.Index(trimmed, "{")
		if start < 0 {
			return ""
		}
	}
	end := findMatchingClose(trimmed[start:])
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[start : start+end+1])
}

func findMatchingClose(s string) int {
	depth := 0
	var open, close byte
	switch s[0] {
	case '{':
		open, close = '{', '}'
	case '[':
		open, close = '[', ']'
	default:
		return -1
	}
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
