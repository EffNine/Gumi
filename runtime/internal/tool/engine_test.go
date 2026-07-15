package tool

import (
	"strings"
	"testing"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/profiles"
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

func TestValidateToolCallsWrongArgumentType(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "search_code",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"max_results": map[string]interface{}{"type": "integer"},
						"query":       map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"query"},
				},
			},
		},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "search_code", Arguments: `{"query":"find x","max_results":"not_a_number"}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for wrong argument type")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "INVALID_ARGUMENT_TYPE" {
			found = true
			if !strings.Contains(issue.Message, "max_results") || !strings.Contains(issue.Message, "integer") {
				t.Errorf("expected type mismatch message about max_results/integer, got: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected INVALID_ARGUMENT_TYPE issue, got %v", report.Issues)
	}
}

func TestValidateToolCallsEnumConstraint(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "set_log_level",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"level": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"debug", "info", "error"},
						},
					},
					"required": []interface{}{"level"},
				},
			},
		},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "set_log_level", Arguments: `{"level":"trace"}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for enum violation")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "INVALID_ARGUMENT_TYPE" {
			found = true
			if !strings.Contains(issue.Message, "trace") || !strings.Contains(issue.Message, "debug") {
				t.Errorf("expected enum violation message, got: %s", issue.Message)
			}
		}
	}
	if !found {
		t.Errorf("expected INVALID_ARGUMENT_TYPE for enum, got %v", report.Issues)
	}
}

func TestValidateToolCallsNestedObject(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "create_user",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{"type": "string"},
						"address": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"city": map[string]interface{}{"type": "string"},
								"zip":  map[string]interface{}{"type": "string"},
							},
							"required": []interface{}{"city", "zip"},
						},
					},
					"required": []interface{}{"name", "address"},
				},
			},
		},
	}
	// Missing nested required key "zip"
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "create_user", Arguments: `{"name":"Alice","address":{"city":"NYC"}}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for missing nested required key")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "INVALID_ARGUMENT_TYPE" && strings.Contains(issue.Message, "zip") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INVALID_ARGUMENT_TYPE for missing nested key, got %v", report.Issues)
	}
}

func TestValidateToolCallsArrayItemType(t *testing.T) {
	tools := []api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name: "batch_process",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"ids": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "integer",
							},
						},
					},
					"required": []interface{}{"ids"},
				},
			},
		},
	}
	// Array contains a string instead of integer
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "batch_process", Arguments: `{"ids":[1, "two", 3]}`}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for wrong array item type")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "INVALID_ARGUMENT_TYPE" && strings.Contains(issue.Message, `"ids"[1]`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INVALID_ARGUMENT_TYPE for array item, got %v", report.Issues)
	}
}

func TestSchemaViolations(t *testing.T) {
	report := ValidationReport{
		Valid: false,
		Issues: []ValidationIssue{
			{Code: "MISSING_REQUIRED_ARGUMENT", Message: `tool "read_file": missing required argument "path"`, ToolCall: api.ToolCall{}},
			{Code: "INVALID_ARGUMENT_TYPE", Message: `tool "search_code": argument "max_results" expected type "integer" but got "string"`, ToolCall: api.ToolCall{}},
		},
	}
	violations := SchemaViolations(report)
	if violations == "" {
		t.Fatal("expected non-empty violations summary")
	}
	if !strings.Contains(violations, "MISSING_REQUIRED_ARGUMENT") || !strings.Contains(violations, "INVALID_ARGUMENT_TYPE") {
		t.Errorf("violations should contain both issue codes, got: %s", violations)
	}
}

func TestSchemaViolationsValid(t *testing.T) {
	report := ValidationReport{Valid: true}
	if v := SchemaViolations(report); v != "" {
		t.Errorf("expected empty violations for valid report, got: %s", v)
	}
}

func TestValidateToolCallsEmptyCalls(t *testing.T) {
	report := ValidateToolCalls(nil, nil)
	if !report.Valid {
		t.Error("expected valid report for empty calls")
	}
}

func TestValidateToolCallsEmptyToolName(t *testing.T) {
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "", Arguments: `{}`}},
	}
	report := ValidateToolCalls(calls, []api.Tool{})
	if report.Valid {
		t.Fatal("expected invalid report for empty tool name")
	}
	if len(report.Issues) != 1 || report.Issues[0].Code != "MISSING_TOOL_NAME" {
		t.Errorf("expected MISSING_TOOL_NAME, got %v", report.Issues)
	}
}

// ---------------------------------------------------------------------------
// Regression: extractJSONCandidate strips ```python fences
// ---------------------------------------------------------------------------

func TestExtractJSONCandidatePythonFence(t *testing.T) {
	// Some models (e.g. Essential AI RNJ-1) wrap JSON inside ```python blocks.
	content := "```python\nimport json\n\ndata = {\n    \"name\": \"test\",\n    \"value\": 42\n}\n\njson_object = json.dumps(data)\nprint(json_object)\n```"
	candidate := extractJSONCandidate(content)
	if candidate == "" {
		t.Fatal("expected non-empty candidate from python fence")
	}
	if !strings.Contains(candidate, "\"name\"") || !strings.Contains(candidate, "\"value\"") {
		t.Fatalf("expected JSON keys in candidate, got %q", candidate)
	}
}

func TestExtractJSONCandidateBareFence(t *testing.T) {
	content := "```\n{\"ok\": true}\n```"
	candidate := extractJSONCandidate(content)
	if candidate != "{\"ok\": true}" {
		t.Fatalf("expected bare JSON from fence, got %q", candidate)
	}
}

func TestExtractJSONCandidateNoFence(t *testing.T) {
	content := `{"key": "value"}`
	candidate := extractJSONCandidate(content)
	if candidate != content {
		t.Fatalf("expected unchanged JSON, got %q", candidate)
	}
}

func TestValidateToolCallsEmptyArguments(t *testing.T) {
	tools := []api.Tool{
		{Type: "function", Function: api.ToolFunction{Name: "test", Parameters: map[string]interface{}{}}},
	}
	calls := []api.ToolCall{
		{Function: api.ToolFunction{Name: "test", Arguments: ""}},
	}
	report := ValidateToolCalls(calls, tools)
	if report.Valid {
		t.Fatal("expected invalid report for empty arguments")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "EMPTY_ARGUMENTS" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EMPTY_ARGUMENTS, got %v", report.Issues)
	}
}
