package promptengine

import (
	"strings"
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
	contextengine "github.com/novexa/novexa/runtime/internal/context"
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
			PreservedFacts: []string{"Novexa must stay local-first."},
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
	if !strings.Contains(system, "Novexa Runtime") {
		t.Fatalf("expected Novexa runtime instruction, got %q", system)
	}
	if !strings.Contains(system, "Novexa must stay local-first") {
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
