package guard

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/provider"
)

func TestCheckBlocksEmptyPrompt(t *testing.T) {
	out := New().Check(Input{
		Messages: []api.Message{{Role: "user", Content: "   "}},
	})

	if !out.Report.Blocked {
		t.Fatal("expected blocked guard report")
	}
	if out.Error.Code != provider.EmptyPrompt {
		t.Fatalf("expected empty prompt, got %s", out.Error.Code)
	}
}

func TestCheckAllowsUsablePrompt(t *testing.T) {
	out := New().Check(Input{
		Messages: []api.Message{{Role: "user", Content: "hello"}},
	})

	if out.Report.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s", out.Report.Decision)
	}
}

func TestCheckDetectsToolCallLoop(t *testing.T) {
	messages := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	out := New().Check(Input{Messages: messages})
	found := false
	for _, w := range out.Warnings {
		if w == "tool call loop detected in conversation history" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tool call loop warning, got warnings %v", out.Warnings)
	}
}
