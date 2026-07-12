package repair

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/validation"
)

func TestRepairJSONExtractsFencedObject(t *testing.T) {
	resp := response("```json\n{\"ok\":true}\n```")
	report := New().Repair(resp, validation.Report{
		Repairable:              true,
		SuggestedRepairStrategy: validation.StrategyLocalParseRepair,
	})

	if !report.Success {
		t.Fatalf("expected repair success: %#v", report)
	}
	content, _ := validation.AssistantContent(resp)
	if content != "{\"ok\":true}" {
		t.Fatalf("expected compact JSON, got %q", content)
	}
}

func TestRepairJSONExtractsFromPythonFence(t *testing.T) {
	// RNJ-1 wraps JSON in ```python blocks with surrounding code.
	resp := response("```python\nimport json\n\ndata = {\n    \"name\": \"test\",\n    \"value\": 42\n}\nprint(data)\n```")
	report := New().Repair(resp, validation.Report{
		Repairable:              true,
		SuggestedRepairStrategy: validation.StrategyLocalParseRepair,
	})

	if !report.Success {
		t.Fatalf("expected repair success: %#v", report)
	}
	content, _ := validation.AssistantContent(resp)
	if content != "{\"name\":\"test\",\"value\":42}" {
		t.Fatalf("expected compact JSON extracted from python fence, got %q", content)
	}
}

func TestRepairRepetitionRemovesExtraLines(t *testing.T) {
	resp := response("a\na\na\na")
	report := New().Repair(resp, validation.Report{
		Repairable:              true,
		SuggestedRepairStrategy: validation.StrategyRegexCleanup,
	})

	if !report.Success {
		t.Fatalf("expected repair success: %#v", report)
	}
	content, _ := validation.AssistantContent(resp)
	if content != "a\na" {
		t.Fatalf("expected repeated lines cleaned, got %q", content)
	}
}

func response(content string) *api.ChatCompletionResponse {
	return &api.ChatCompletionResponse{
		Choices: []api.Choice{{
			Message:      api.Message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
	}
}
