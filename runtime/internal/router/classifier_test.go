package router

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/config"
)

func TestClassifyDifficultyFromTraceback(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role: "user",
		Content: `Traceback (most recent call last):
  File "main.py", line 42, in <module>
    result = process_data(input)
  File "main.py", line 25, in process_data
    return transform(value)
  File "utils/helpers.py", line 10, in transform
    return data[key]
KeyError: 'missing_key'`,
	}}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyModerate {
		t.Fatalf("expected difficulty >= %d (moderate) for traceback, got %d", DifficultyModerate, profile.Difficulty)
	}
	if !profile.HasTraceback {
		t.Fatal("expected has_traceback=true")
	}
}

func TestClassifyHintOverrides(t *testing.T) {
	c := NewCodingTaskClassifier(nil)
	msgs := []api.Message{{
		Role:    "user",
		Content: "fix the typo in the readme",
	}}
	hints := &api.RoutingExtensions{
		HintDifficulty: 5,
		HintTaskType:   "refactor",
	}
	profile := c.Classify(msgs, 0, 0, hints)
	if profile.Difficulty != 5 {
		t.Fatalf("expected difficulty 5 from hint, got %d", profile.Difficulty)
	}
	if profile.TaskType != TaskRefactor {
		t.Fatalf("expected task type 'refactor' from hint, got %s", profile.TaskType)
	}
}

func TestApplyEscalationRetries(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 2, Steps: 10, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 0, 3, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after retry escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationSteps(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 5, Repetitions: 10},
	})
	msgs := []api.Message{{Role: "user", Content: "hello"}}
	profile := c.Classify(msgs, 6, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after step escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}

func TestApplyEscalationRepetitions(t *testing.T) {
	c := NewCodingTaskClassifier(&config.ClassifierConfig{
		EscalationThreshold: config.EscalationThreshold{Retries: 10, Steps: 10, Repetitions: 3},
	})
	msgs := []api.Message{
		{Role: "user", Content: "read the file"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_1"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
		{Role: "tool", Content: "package main", ToolCallID: "call_2"},
		{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{{Function: api.ToolFunction{Name: "read_file", Arguments: `{"path":"main.go"}`}}}},
	}
	profile := c.Classify(msgs, 0, 0, nil)
	if profile.Difficulty < DifficultyComplex {
		t.Fatalf("expected difficulty >= %d after repetition escalation, got %d", DifficultyComplex, profile.Difficulty)
	}
}
