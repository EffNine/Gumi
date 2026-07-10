package contextengine

import (
	"testing"

	"github.com/novexa/novexa/runtime/internal/api"
)

func TestPrepareNormalizesDeduplicatesAndTrims(t *testing.T) {
	engine := New()
	long := "This is a verbose assistant message that can be trimmed when the context budget is tight. "

	out := engine.Prepare(Input{
		RuntimeMode:    "stabilized",
		Strategy:       "hybrid",
		MaxInputTokens: 20,
		Messages: []api.Message{
			{Role: "SYSTEM", Content: " Novexa must stay local-first. "},
			{Role: "assistant", Content: long + long + long},
			{Role: "assistant", Content: long + long + long},
			{Role: "user", Content: "Proceed sprint 6"},
		},
	})

	if len(out.FinalMessages) == 0 {
		t.Fatal("expected final messages")
	}
	if out.FinalMessages[0].Role != "system" {
		t.Fatalf("expected normalized system role, got %q", out.FinalMessages[0].Role)
	}
	if out.Package.ActiveRequest != "Proceed sprint 6" {
		t.Fatalf("expected latest user request preserved, got %q", out.Package.ActiveRequest)
	}
	if out.Report.DuplicatesRemoved == 0 {
		t.Fatal("expected duplicate removal")
	}
	if out.Report.ItemsRemoved == 0 {
		t.Fatal("expected items removed")
	}
	if out.Report.EstimatedTokensAfter > out.Report.EstimatedTokensBefore {
		t.Fatal("expected token estimate not to increase")
	}
}

func TestPrepareDirectModeUsesNoneStrategy(t *testing.T) {
	engine := New()

	out := engine.Prepare(Input{
		RuntimeMode: "direct",
		Messages: []api.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if out.Report.StrategyUsed != "none" {
		t.Fatalf("expected none strategy, got %s", out.Report.StrategyUsed)
	}
}

func TestEstimateText(t *testing.T) {
	if got := EstimateText("abcd"); got != 1 {
		t.Fatalf("expected 1 token, got %d", got)
	}
	if got := EstimateText(""); got != 0 {
		t.Fatalf("expected 0 tokens, got %d", got)
	}
}
