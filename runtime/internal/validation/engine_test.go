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
