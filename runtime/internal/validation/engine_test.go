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

func responseWithContent(content string, finish string) *api.ChatCompletionResponse {
	return &api.ChatCompletionResponse{
		Choices: []api.Choice{{
			Message:      api.Message{Role: "assistant", Content: content},
			FinishReason: finish,
		}},
	}
}
