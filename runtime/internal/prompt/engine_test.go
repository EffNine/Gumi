package promptengine

import (
	"strings"
	"testing"

	"github.com/EffNine/gumi/runtime/internal/api"
	contextengine "github.com/EffNine/gumi/runtime/internal/context"
)

func TestBuildAddsSystemPromptAndPreservesUserMessage(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "system", Content: "Use concise answers."},
			{Role: "user", Content: "Hello"},
		},
		ContextPackage: contextengine.Package{
			ActiveRequest:  "Hello",
			PreservedFacts: []string{"Gumi must stay local-first."},
		},
		ExistingSystem: []string{"Use concise answers."},
	})

	if len(out.FinalMessages) != 2 {
		t.Fatalf("expected 2 final messages, got %d", len(out.FinalMessages))
	}
	if out.FinalMessages[0].Role != "system" {
		t.Fatalf("expected first message system, got %s", out.FinalMessages[0].Role)
	}
	system, _ := out.FinalMessages[0].Content.(string)
	if !strings.Contains(system, "Gumi Runtime") {
		t.Fatalf("expected Gumi runtime instruction, got %q", system)
	}
	if !strings.Contains(system, "Gumi must stay local-first") {
		t.Fatalf("expected preserved fact in system prompt, got %q", system)
	}
	if out.FinalMessages[1].Role != "user" {
		t.Fatalf("expected user message preserved, got %s", out.FinalMessages[1].Role)
	}
}

func TestBuildStructuredPromptAppliesJSONInstructions(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "structured",
		Messages: []api.Message{
			{Role: "user", Content: "Return JSON"},
		},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	})

	if !out.Report.ResponseFormatApplied {
		t.Fatal("expected response format applied")
	}
	system, _ := out.FinalMessages[0].Content.(string)
	if !strings.Contains(system, "valid JSON object") {
		t.Fatalf("expected JSON instruction, got %q", system)
	}
}

func TestBuildStabilizedPromptAvoidsImplicitStructuredOutput(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "What is 2+2? Answer in one word."},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	if strings.Contains(system, "think step-by-step") {
		t.Fatalf("expected no generic reasoning instructions, got %q", system)
	}
	if strings.Contains(system, "Do not convert plain-text answers into JSON") {
		t.Fatalf("expected no conflicting JSON instruction, got %q", system)
	}
	if !strings.Contains(system, "one word") {
		t.Fatalf("expected exact-format instruction, got %q", system)
	}
	if out.Report.ResponseFormatApplied {
		t.Fatal("did not expect response format to be applied")
	}
}

func TestBuildPlainTextInputDoesNotIncludeJSONBlock(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "What is 2+2?"},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	if strings.Contains(system, "Do not convert plain-text answers into JSON") {
		t.Fatalf("plain-text request should not receive JSON anti-conversion instruction, got %q", system)
	}
	if strings.Contains(system, "unless they explicitly request JSON") {
		t.Fatalf("plain-text request should not receive JSON conditional instruction, got %q", system)
	}
}

func TestBuildGenericPromptNoConflictingJSONDirectives(t *testing.T) {
	engine := New()

	// No ResponseFormat set — generic stabilized mode
	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	jsonNegativeCount := strings.Count(system, "Do not convert")
	jsonConditionalCount := strings.Count(system, "explicitly request JSON")
	if jsonNegativeCount > 0 || jsonConditionalCount > 0 {
		t.Fatalf("generic prompt must not contain competing JSON directives, got %q", system)
	}
}

func TestBuildGenericPromptNoThinkStepByStep(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "Explain photosynthesis."},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	if strings.Contains(system, "think step-by-step") {
		t.Fatalf("generic prompt must not contain 'think step-by-step', got %q", system)
	}
	if strings.Contains(system, "break multi-part requests into subtasks") {
		t.Fatalf("generic prompt must not contain subtask-breaking instruction, got %q", system)
	}
	if strings.Contains(system, "verify your response before output") {
		t.Fatalf("generic prompt must not contain verification instruction, got %q", system)
	}
}

func TestBuildGenericPromptPreservesExplicitUserRequirements(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "Answer in exactly one word: what is the capital of France?"},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	if !strings.Contains(system, "one word") {
		t.Fatalf("generic prompt must preserve exact-format user requirements, got %q", system)
	}
}

func TestBuildPreservesDirectAnsweringAndConciseness(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "stabilized",
		Messages: []api.Message{
			{Role: "user", Content: "Tell me about Go."},
		},
	})

	system, _ := out.FinalMessages[0].Content.(string)
	if !strings.Contains(system, "Gumi Runtime") {
		t.Fatalf("expected Gumi runtime identity, got %q", system)
	}
	if !strings.Contains(system, "expert AI assistant") {
		t.Fatalf("expected expert assistant identity, got %q", system)
	}
	if !strings.Contains(system, "directly and concisely") {
		t.Fatalf("expected direct/concise behavior, got %q", system)
	}
}

func TestBuildStructuredModeStillAppliesJSONInstructions(t *testing.T) {
	engine := New()

	out := engine.Build(Input{
		RuntimeMode: "structured",
		Messages: []api.Message{
			{Role: "user", Content: "Return JSON"},
		},
		ResponseFormat: &api.ResponseFormat{Type: "json_object"},
	})

	if !out.Report.ResponseFormatApplied {
		t.Fatal("expected response format applied in structured mode")
	}
	system, _ := out.FinalMessages[0].Content.(string)
	if !strings.Contains(system, "valid JSON object") {
		t.Fatalf("expected JSON instruction in structured mode, got %q", system)
	}
}
