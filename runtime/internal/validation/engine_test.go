package validation

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
)

func TestValidateInvalidJSON(t *testing.T) {
	engine := New()
	report := engine.Validate(Input{
		RuntimeMode:    "structured",
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
		Response:       responseWithContent("not json", "stop"),
	})

	if report.Passed {
		t.Fatal("expected validation failure")
	}
	if report.Issues[0].Code != IssueInvalidJSON {
		t.Fatalf("expected invalid JSON issue, got %s", report.Issues[0].Code)
	}
	if report.SuggestedRepairStrategy != StrategyLocalParseRepair {
		t.Fatalf("expected local parse repair, got %s", report.SuggestedRepairStrategy)
	}
}

func TestValidateRepetition(t *testing.T) {
	engine := New()
	report := engine.Validate(Input{
		Response: responseWithContent("repeat\nrepeat\nrepeat\nrepeat", "stop"),
	})

	if report.Passed {
		t.Fatal("expected repetition failure")
	}
	if report.Issues[0].Code != IssueRepetition {
		t.Fatalf("expected repetition issue, got %s", report.Issues[0].Code)
	}
}

func TestValidateRepetitionSkipsJSON(t *testing.T) {
	engine := New()
	// JSON with repeated structural elements should NOT trigger repetition.
	jsonContent := `{"items":[{"type":"a"},{"type":"a"},{"type":"a"}]}`
	report := engine.Validate(Input{
		Response:       responseWithContent(jsonContent, "stop"),
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
		RuntimeMode:    "stabilized",
	})
	for _, iss := range report.Issues {
		if iss.Code == IssueRepetition {
			t.Fatalf("repetition should not fire for JSON output, got issue: %s", iss.Message)
		}
	}
}

func TestValidateRepetitionSkipsStructuredMode(t *testing.T) {
	engine := New()
	// Structured mode forces JSON; repetition should not fire even without
	// explicit response_format.
	jsonContent := `{"a":1,"b":1,"c":1}`
	report := engine.Validate(Input{
		Response:    responseWithContent(jsonContent, "stop"),
		RuntimeMode: "structured",
	})
	for _, iss := range report.Issues {
		if iss.Code == IssueRepetition {
			t.Fatalf("repetition should not fire in structured mode, got issue: %s", iss.Message)
		}
	}
}

func TestExtractJSONCandidate(t *testing.T) {
	got := ExtractJSONCandidate("Here:\n```json\n{\"ok\":true}\n```")
	if got != "{\"ok\":true}" {
		t.Fatalf("unexpected candidate %q", got)
	}
}

func TestExtractJSONCandidatePythonFence(t *testing.T) {
	// RNJ-1 wraps JSON in ```python blocks with surrounding code.
	content := "```python\nimport json\n\ndata = {\n    \"name\": \"test\",\n    \"value\": 42\n}\n\njson_object = json.dumps(data)\nprint(json_object)\n```"
	got := ExtractJSONCandidate(content)
	if got != "{\n    \"name\": \"test\",\n    \"value\": 42\n}" {
		t.Fatalf("unexpected candidate from python fence: %q", got)
	}
}

func TestExtractJSONCandidateBareFence(t *testing.T) {
	got := ExtractJSONCandidate("```\n{\"ok\":true}\n```")
	if got != "{\"ok\":true}" {
		t.Fatalf("unexpected candidate from bare fence: %q", got)
	}
}

func TestRequiresJSONDetectsPythonFence(t *testing.T) {
	// In stabilized mode (no response_format), content wrapped in ```python
	// should be detected as requiring JSON validation so repair can extract it.
	content := "```python\nimport json\ndata = {\"name\": \"test\"}\n```"
	if !requiresJSON(nil, "stabilized", content) {
		t.Fatal("requiresJSON should return true for python-fenced content")
	}
}

func responseWithContent(content string, finish string) *api.ChatCompletionResponse {
	return &api.ChatCompletionResponse{
		Choices: []api.Choice{{
			Message:      api.Message{Role: "assistant", Content: content},
			FinishReason: finish,
		}},
	}
}
