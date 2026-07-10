package profiles

import (
	"testing"
)

func TestResolveProviderAlias(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "qwen3-8b",
			Version: 1,
			Family:  "qwen",
			Models: map[string][]string{
				"ollama": {"qwen3:8b", "qwen3:latest"},
			},
		},
	}
	r := NewResolver(profiles)

	cases := []struct {
		provider string
		model    string
		wantID   string
		reason   string
	}{
		{"ollama", "qwen3:8b", "qwen3-8b", "provider_alias"},
		{"ollama", "qwen3:latest", "qwen3-8b", "provider_alias"},
		{"", "qwen3:8b", "qwen3-8b", "global_alias"},
		{"", "qwen3:latest", "qwen3-8b", "global_alias"},
	}

	for _, tc := range cases {
		m := r.Resolve(tc.provider, tc.model)
		if m.Profile.ID != tc.wantID {
			t.Fatalf("%s/%s: expected %q, got %q", tc.provider, tc.model, tc.wantID, m.Profile.ID)
		}
		if m.IsFallback {
			t.Fatalf("%s/%s: expected a real match", tc.provider, tc.model)
		}
	}
}

func TestResolveUnknownModelUsesGenericFallback(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "qwen3-8b",
			Version: 1,
			Family:  "qwen",
			Models: map[string][]string{
				"ollama": {"qwen3:8b"},
			},
		},
	}
	r := NewResolver(profiles)
	m := r.Resolve("ollama", "some-random-model")
	if m.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local fallback, got %q", m.Profile.ID)
	}
	if !m.IsFallback {
		t.Fatal("expected fallback match")
	}
}

func TestResolveFamilyMatch(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "qwen3-8b",
			Version: 1,
			Family:  "qwen",
			Models: map[string][]string{
				"ollama": {"qwen3:8b"},
			},
		},
	}
	r := NewResolver(profiles)
	m := r.Resolve("ollama", "qwen2.5-coder:7b")
	if m.Profile.ID != "qwen3-8b" {
		t.Fatalf("expected family match qwen3-8b, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected family match, not fallback")
	}
}

func TestResolverAlwaysIncludesGenericFallback(t *testing.T) {
	r := NewResolver(nil)
	m := r.Resolve("ollama", "anything")
	if m.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local, got %q", m.Profile.ID)
	}
}
