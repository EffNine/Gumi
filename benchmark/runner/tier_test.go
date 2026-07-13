package runner

import (
	"testing"
)

func TestResolveTier_FrontierProviders(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		provider string
	}{
		{"openai provider", "gpt-4o", "openai"},
		{"anthropic provider", "claude-4", "anthropic"},
		{"google provider", "gemini-2.0-flash", "google"},
		{"anthropic any model", "any-model", "anthropic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := ResolveTier(tt.model, tt.provider)
			if err != nil {
				t.Errorf("ResolveTier(%q, %q): unexpected error: %v", tt.model, tt.provider, err)
			}
			if tier != TierFrontier {
				t.Errorf("ResolveTier(%q, %q) = %v, want %v", tt.model, tt.provider, tier, TierFrontier)
			}
		})
	}
}

func TestResolveTier_FrontierModelNames(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{"gpt-4 pattern", "gpt-4-turbo"},
		{"claude-4 pattern", "claude-4-sonnet"},
		{"claude-3.5 pattern", "claude-3.5-sonnet"},
		{"gemini-2 pattern", "gemini-2.0-flash"},
		{"fable model", "fable-1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := ResolveTier(tt.model, "local")
			if err != nil {
				t.Errorf("ResolveTier(%q, 'local'): unexpected error: %v", tt.model, err)
			}
			if tier != TierFrontier {
				t.Errorf("ResolveTier(%q, 'local') = %v, want %v", tt.model, tier, TierFrontier)
			}
		})
	}
}

func TestResolveTier_SizeBased(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		provider string
		want     ModelTier
	}{
		{"7B model → small", "llama3.2-7b", "lmstudio", TierSmall},
		{"1B model → small", "gemma3-1b", "ollama", TierSmall},
		{"3B model → small", "gemma3-3b", "ollama", TierSmall},
		{"8B model → medium", "qwen2.5-coder-8b", "lmstudio", TierMedium},
		{"9B model → medium", "qwen3.5-9b", "lmstudio", TierMedium},
		{"12B model → medium", "gemma-12b", "lmstudio", TierMedium},
		{"32B model → medium", "deepseek-32b", "lmstudio", TierMedium},
		{"70B model → frontier", "llama-70b", "lmstudio", TierFrontier},
		{"120B model → frontier", "falcon-120b", "lmstudio", TierFrontier},
		{"unknown size → medium", "custom-model", "lmstudio", TierMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := ResolveTier(tt.model, tt.provider)
			if err != nil {
				t.Errorf("ResolveTier(%q, %q): unexpected error: %v", tt.model, tt.provider, err)
			}
			if tier != tt.want {
				t.Errorf("ResolveTier(%q, %q) = %v, want %v", tt.model, tt.provider, tier, tt.want)
			}
		})
	}
}

func TestResolveTier_B(t *testing.T) {
	// Test case-insensitivity for "b" suffix (uppercase B)
	tier, err := ResolveTier("model-7B", "lmstudio")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tier != TierSmall {
		t.Errorf("ResolveTier('model-7B', 'lmstudio') = %v, want %v", tier, TierSmall)
	}
}

func TestSuitesToRun(t *testing.T) {
	tests := []struct {
		tier ModelTier
		want []string
	}{
		{TierSmall, []string{"easy", "medium", "cosmetic"}},
		{TierMedium, []string{"easy", "medium", "hard", "cosmetic", "semantic"}},
		{TierFrontier, []string{"hard", "frontier", "cosmetic", "semantic"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got := tt.tier.SuitesToRun()
			if len(got) != len(tt.want) {
				t.Errorf("SuitesToRun(%v) = %v (len %d), want %v (len %d)",
					tt.tier, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SuitesToRun(%v) = %v, want %v", tt.tier, got, tt.want)
					return
				}
			}
		})
	}
}

func TestSuitesToRun_Unknown(t *testing.T) {
	tier := ModelTier("unknown")
	got := tier.SuitesToRun()
	want := []string{"easy", "medium"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SuitesToRun(unknown) = %v, want %v", got, want)
	}
}

func TestContainsTier(t *testing.T) {
	tests := []struct {
		tiers  []string
		target string
		want   bool
	}{
		{[]string{"easy", "medium", "hard"}, "medium", true},
		{[]string{"easy", "hard"}, "medium", false},
		{nil, "easy", false},
		{[]string{}, "easy", false},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := containsTier(tt.tiers, tt.target)
			if got != tt.want {
				t.Errorf("containsTier(%v, %q) = %v, want %v", tt.tiers, tt.target, got, tt.want)
			}
		})
	}
}
