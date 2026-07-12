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

func TestAgentGuardBlocksStepLimitExceeded(t *testing.T) {
	messages := make([]api.Message, 0, 35)
	for i := 0; i < 35; i++ {
		messages = append(messages, api.Message{Role: "assistant", Content: "step"})
	}
	out := New().CheckAgent(Input{Messages: messages}, AgentInput{MaxSteps: 30, LoopDetection: "strict"})
	if !out.Report.Blocked {
		t.Fatal("expected blocked guard report for step limit exceeded")
	}
	if out.Error.Code != provider.AGENT_STEP_LIMIT_EXCEEDED {
		t.Fatalf("expected AGENT_STEP_LIMIT_EXCEEDED, got %s", out.Error.Code)
	}
}

func TestAgentGuardAllowsBelowStepLimit(t *testing.T) {
	messages := make([]api.Message, 0, 5)
	for i := 0; i < 5; i++ {
		messages = append(messages, api.Message{Role: "assistant", Content: "step"})
	}
	messages = append(messages, api.Message{Role: "user", Content: "continue"})
	out := New().CheckAgent(Input{Messages: messages}, AgentInput{MaxSteps: 30, LoopDetection: "strict"})
	if out.Report.Blocked {
		t.Fatal("expected allowed guard report for below step limit")
	}
	if out.Report.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s", out.Report.Decision)
	}
}

func TestAgentGuardBlocksRepeatedToolCallStrict(t *testing.T) {
	messages := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	out := New().CheckAgent(Input{Messages: messages}, AgentInput{MaxSteps: 30, LoopDetection: "strict"})
	if !out.Report.Blocked {
		t.Fatal("expected blocked guard report for repeated tool call (strict)")
	}
	if out.Error.Code != provider.AGENT_TOOL_CALL_LOOP {
		t.Fatalf("expected AGENT_TOOL_CALL_LOOP, got %s", out.Error.Code)
	}
}

func TestAgentGuardWarnsRepeatedToolCallStandard(t *testing.T) {
	messages := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	out := New().CheckAgent(Input{Messages: messages}, AgentInput{MaxSteps: 30, LoopDetection: "standard"})
	if out.Report.Blocked {
		t.Fatal("expected non-blocked guard report for 2 repeated tool calls (standard)")
	}
	if out.Report.Decision != DecisionWarn {
		t.Fatalf("expected warn, got %s", out.Report.Decision)
	}
}
