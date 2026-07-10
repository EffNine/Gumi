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
