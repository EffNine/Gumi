package runner

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestConditionManager_BuildRequest_Direct(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionDirect, benchmark.SuiteTest{Prompt: "hello", MaxTokens: 100})

	if req.Model != "my-model" {
		t.Errorf("direct: Model=%q, want %q", req.Model, "my-model")
	}
	if req.Gumi != nil {
		t.Errorf("direct: Gumi should be nil, got %+v", req.Gumi)
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

func TestConditionManager_BuildRequest_GumiDirect(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionGumiDirect, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("gumi-direct: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Gumi != nil {
		t.Errorf("gumi-direct: Gumi should be nil, got %+v", req.Gumi)
	}
}

func TestConditionManager_BuildRequest_GumiLightweight(t *testing.T) {
	cm := NewConditionManager("my-model", "ollama", "", "")
	req := cm.BuildRequest(ConditionGumiLightweight, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "ollama:my-model" {
		t.Errorf("gumi-lightweight: Model=%q, want %q", req.Model, "ollama:my-model")
	}
	if req.Gumi == nil {
		t.Fatal("gumi-lightweight: Gumi should not be nil")
	}
	if req.Gumi.Mode != "lightweight" {
		t.Errorf("gumi-lightweight: Gumi.Mode=%q, want %q", req.Gumi.Mode, "lightweight")
	}
}

func TestConditionManager_BuildRequest_GumiStabilized(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionGumiStabilized, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("gumi-stabilized: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Gumi == nil {
		t.Fatal("gumi-stabilized: Gumi should not be nil")
	}
	if req.Gumi.Mode != "stabilized" {
		t.Errorf("gumi-stabilized: Gumi.Mode=%q, want %q", req.Gumi.Mode, "stabilized")
	}
}

func TestConditionManager_BuildRequest_GumiStructured(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "", "")
	req := cm.BuildRequest(ConditionGumiStructured, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "lmstudio:my-model" {
		t.Errorf("gumi-structured: Model=%q, want %q", req.Model, "lmstudio:my-model")
	}
	if req.Gumi == nil {
		t.Fatal("gumi-structured: Gumi should not be nil")
	}
	if req.Gumi.Mode != "structured" {
		t.Errorf("gumi-structured: Gumi.Mode=%q, want %q", req.Gumi.Mode, "structured")
	}
}

func TestConditionManager_BuildRequest_Frontier(t *testing.T) {
	cm := NewConditionManager("my-model", "lmstudio", "claude-4-sonnet", "sk-xxx")
	req := cm.BuildRequest(ConditionFrontier, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})

	if req.Model != "claude-4-sonnet" {
		t.Errorf("frontier: Model=%q, want %q", req.Model, "claude-4-sonnet")
	}
	if req.Gumi != nil {
		t.Errorf("frontier: Gumi should be nil, got %+v", req.Gumi)
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
			req := cm.BuildRequest(ConditionGumiDirect, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})
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
		{"multiple", []string{"direct", "gumi-stabilized", "frontier"}, []Condition{ConditionDirect, ConditionGumiStabilized, ConditionFrontier}},
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

	req2 := cm.BuildRequest(ConditionGumiStabilized, benchmark.SuiteTest{Prompt: "test", MaxTokens: 50})
	if req2.Temperature != 0.3 {
		t.Errorf("gumi temperature=%v, want 0.3", req2.Temperature)
	}
}
