package runner

import (
	"testing"

	"github.com/novexa/novexa/benchmark"
)

func TestConditionManager_BuildRequest_Direct(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionDirect, benchmark.SuiteTest{Prompt: "hello", MaxTokens: 100})

	if req.Model != "my-model" {
		t.Errorf("direct: Model=%q, want %q", req.Model, "my-model")
	}
	if req.Novexa != nil {
		t.Errorf("direct: Novexa should be nil, got %+v", req.Novexa)
	}
	if len(req.Messages) != 1 {
		t.Errorf("direct: expected 1 message, got %d", len(req.Messages))
	}
	if req.Messages[0].Content != "hello" {
		t.Errorf("direct: message content=%q, want %q", req.Messages[0].Content, "hello")
	}
	if req.MaxTokens != 100 {
		t.Errorf("direct: MaxTokens=%d, want 100", req.MaxTokens)
	}
}

func TestConditionManager_BuildRequest_NovexaDirect(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionNovexaDirect, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("novexa-direct: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Novexa != nil {
		t.Errorf("novexa-direct: Novexa should be nil, got %+v", req.Novexa)
	}
}

func TestConditionManager_BuildRequest_NovexaLightweight(t *testing.T) {
	cm := NewConditionManager("my-model", "ollama", "", "")
	req := cm.BuildRequest(ConditionNovexaLightweight, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "ollama:my-model" {
		t.Errorf("novexa-lightweight: Model=%q, want %q", req.Model, "ollama:my-model")
	}
	if req.Novexa == nil {
		t.Fatal("novexa-lightweight: Novexa should not be nil")
	}
	if req.Novexa.Mode != "lightweight" {
		t.Errorf("novexa-lightweight: Novexa.Mode=%q, want %q", req.Novexa.Mode, "lightweight")
	}
}

func TestConditionManager_BuildRequest_NovexaStabilized(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionNovexaStabilized, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("novexa-stabilized: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Novexa == nil {
		t.Fatal("novexa-stabilized: Novexa should not be nil")
	}
	if req.Novexa.Mode != "stabilized" {
		t.Errorf("novexa-stabilized: Novexa.Mode=%q, want %q", req.Novexa.Mode, "stabilized")
	}
}

func TestConditionManager_BuildRequest_NovexaStructured(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionNovexaStructured, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("novexa-structured: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Novexa == nil {
		t.Fatal("novexa-structured: Novexa should not be nil")
	}
	if req.Novexa.Mode != "structured" {
		t.Errorf("novexa-structured: Novexa.Mode=%q, want %q", req.Novexa.Mode, "structured")
	}
}

func TestConditionManager_BuildRequest_Frontier(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "claude-4-sonnet", "sk-xxx")
	req := cm.BuildRequest(ConditionFrontier, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "claude-4-sonnet" {
		t.Errorf("frontier: Model=%q, want %q", req.Model, "claude-4-sonnet")
	}
	if req.Novexa != nil {
		t.Errorf("frontier: Novexa should be nil, got %+v", req.Novexa)
	}
}

func TestConditionManager_BuildRequest_FrontierNoModel(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionFrontier, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	// When frontierModel is empty, the frontier condition falls back to the base model name.
	if req.Model != "my-model" {
		t.Errorf("frontier (no model): Model=%q, want %q (fallback)", req.Model, "my-model")
	}
}

func TestConditionManager_BuildRequest_ProviderPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     string
	}{
		{"lmstudio", "lmstudio", "lmstudio:"},
		{"ollama", "ollama", "ollama:"},
		{"unknown provider defaults to lmstudio", "openai", "lmstudio:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConditionManager("mymodel", tt.provider, "", "")
			req := cm.BuildRequest(ConditionNovexaDirect, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})
			expectedModel := tt.want + "mymodel"
			if req.Model != expectedModel {
				t.Errorf("provider=%q: Model=%q, want %q", tt.provider, req.Model, expectedModel)
			}
		})
	}
}

func TestParseConditions(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []Condition
	}{
		{"direct only", []string{"direct"}, []Condition{ConditionDirect}},
		{"multiple", []string{"direct", "novexa-stabilized", "frontier"}, []Condition{ConditionDirect, ConditionNovexaStabilized, ConditionFrontier}},
		{"empty", nil, nil},
		{"unknown preserved", []string{"custom-mode"}, []Condition{Condition("custom-mode")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseConditions(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseConditions(%v) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseConditions(%v) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestConditionManager_DefaultTemperature(t *testing.T) {
	cm := NewConditionManager("mymodel", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionDirect, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})
	if req.Temperature != 0.3 {
		t.Errorf("default Temperature=%v, want 0.3", req.Temperature)
	}

	req2 := cm.BuildRequest(ConditionNovexaStabilized, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})
	if req2.Temperature != 0.3 {
		t.Errorf("novexa temperature=%v, want 0.3", req2.Temperature)
	}
}
