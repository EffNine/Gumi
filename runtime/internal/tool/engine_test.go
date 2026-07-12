package tool

import (
	"strings"
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/profiles"
)

func TestBuildInstructions(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"path"},
				},
			},
		},
	}

	engine := New()
	block, warnings := engine.BuildInstructions(tools, nil)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if block == "" {
		t.Fatal("expected non-empty instruction block")
	}
	for _, want := range []string{"read_file", "Read a file", `"path"`, "JSON object", `"tool"`} {
		if !strings.Contains(block, want) {
			t.Errorf("instruction block missing %q; got:\n%s", want, block)
		}
	}
}

func TestBuildInstructionsWithStyleNone(t *testing.T) {
	engine := New()
	profile := &profiles.Profile{Prompt: profiles.PromptSettings{ToolInstructionStyle: "none"}}
	block, warnings := engine.BuildInstructions([]api.Tool{{Type: "function", Function: api.ToolFunction{Name: "x"}}}, profile)
	if block != "" {
		t.Errorf("expected empty block for style none, got %q", block)
	}
	if len(warnings) == 0 {
		t.Error("expected warning for style none")
	}
}

func TestBuildInstructionsEmptyTools(t *testing.T) {
	engine := New()
	block, warnings := engine.BuildInstructions(nil, nil)
	if block != "" || len(warnings) != 0 {
		t.Errorf("expected empty output for empty tools, got block=%q warnings=%v", block, warnings)
	}
}

func TestNormalizeAssistantContentSingleToolCall(t *testing.T) {
	content := `{"tool": "read_file", "arguments": {"path": "main.go"}}`
	result := NormalizeAssistantContent(content)
	if !result.IsToolCall {
		t.Fatal("expected tool call result")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected tool name read_file, got %q", result.ToolCalls[0].Function.Name)
	}
	if !strings.Contains(result.ToolCalls[0].Function.Arguments, `"path"`) {
		t.Errorf("expected arguments to contain path, got %q", result.ToolCalls[0].Function.Arguments)
	}
}

func TestNormalizeAssistantContentArray(t *testing.T) {
	content := `[{"tool": "a", "arguments": {}}, {"tool": "b", "arguments": {"x": 1}}]`
	result := NormalizeAssistantContent(content)
	if !result.IsToolCall || len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %v", result)
	}
	if result.ToolCalls[0].Function.Name != "a" || result.ToolCalls[1].Function.Name != "b" {
		t.Errorf("unexpected tool names: %v", result.ToolCalls)
	}
}

func TestNormalizeAssistantContentMarkdownFence(t *testing.T) {
	content := "```json\n{\"tool\": \"t\", \"arguments\": {}}\n```"
	result := NormalizeAssistantContent(content)
	if !result.IsToolCall || len(result.ToolCalls) != 1 {
		t.Fatalf("expected tool call inside fence, got %v", result)
	}
	if result.ToolCalls[0].Function.Name != "t" {
		t.Errorf("expected tool t, got %q", result.ToolCalls[0].Function.Name)
	}
}

func TestNormalizeAssistantContentPlainText(t *testing.T) {
	content := "I will use the read_file tool."
	result := NormalizeAssistantContent(content)
	if result.IsToolCall {
		t.Error("expected plain text, got tool call")
	}
	if result.Content != content {
		t.Errorf("expected content preserved, got %q", result.Content)
	}
}

func TestValidateToolCallsValid(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "read_file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"path"},
				},
			},
		},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if !report.Valid {
		t.Fatalf("expected valid report, got issues %v", report.Issues)
	}
}

func TestValidateToolCallsUnknownTool(t *testing.T) {
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "unknown", Arguments: `{}}`}},
	}
	report := ValidateToolCalls(calls, []api.Tool{})
	if report.Valid {
		t.Fatal("expected invalid report for unknown tool")
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != "UNKNOWN_TOOL" {
		t.Errorf("expected UNKNOWN_TOOL issue, got %v", report.Issues)
	}
}

func TestValidateToolCallsMissingRequiredArgument(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "read_file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"path"},
				},
			},
		},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "read_file", Arguments: `{}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for missing required argument")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "MISSING_REQUIRED_ARGUMENT" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected MISSING_REQUIRED_ARGUMENT issue, got %v", report.Issues)
	}
}

func TestValidateToolCallsInvalidJSON(t *testing.T) {
	tools := []api.Tool{
		{Type: "function", Function: api.ToolFunction{Name: "read_file", Parameters: map[string]interface{}{}}},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "read_file", Arguments: `{not json}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for invalid JSON arguments")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "INVALID_JSON_ARGUMENTS" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INVALID_JSON_ARGUMENTS issue, got %v", report.Issues)
	}
}
