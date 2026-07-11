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

func TestResolveLMStudioModelToProfile(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "qwen3.5-9b",
			Version: 1,
			Family:  "qwen",
			Models: map[string][]string{
				"lmstudio": {"qwen/qwen3.5-9b"},
			},
		},
	}
	r := NewResolver(profiles)

	m := r.Resolve("lmstudio", "qwen/qwen3.5-9b")
	if m.Profile.ID != "qwen3.5-9b" {
		t.Fatalf("expected qwen3.5-9b, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
	if m.Reason != "provider_alias" {
		t.Fatalf("expected reason provider_alias, got %q", m.Reason)
	}
}

func TestResolveLMStudioQwen3_1_7b(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "qwen3-1.7b",
			Version: 1,
			Family:  "qwen",
			Models: map[string][]string{
				"lmstudio": {"qwen/qwen3-1.7b"},
			},
		},
	}
	r := NewResolver(profiles)
	m := r.Resolve("lmstudio", "qwen/qwen3-1.7b")
	if m.Profile.ID != "qwen3-1.7b" {
		t.Fatalf("expected qwen3-1.7b, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolveLMStudioOrnithQ4KM(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "ornith-1.0-9b-q4-km",
			Version: 1,
			Family:  "ornith",
			Models: map[string][]string{
				"lmstudio": {"ornith-1.0-9b@q4_k_m"},
			},
		},
	}
	r := NewResolver(profiles)
	m := r.Resolve("lmstudio", "ornith-1.0-9b@q4_k_m")
	if m.Profile.ID != "ornith-1.0-9b-q4-km" {
		t.Fatalf("expected ornith-1.0-9b-q4-km, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolveLMStudioGemma4E4B(t *testing.T) {
	profiles := []*Profile{
		{
			ID:      "gemma-4-e4b",
			Version: 1,
			Family:  "gemma",
			Models: map[string][]string{
				"lmstudio": {"google/gemma-4-e4b"},
			},
		},
	}
	r := NewResolver(profiles)
	m := r.Resolve("lmstudio", "google/gemma-4-e4b")
	if m.Profile.ID != "gemma-4-e4b" {
		t.Fatalf("expected gemma-4-e4b, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolverAlwaysIncludesGenericFallback(t *testing.T) {
	r := NewResolver(nil)
	m := r.Resolve("ollama", "anything")
	if m.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local, got %q", m.Profile.ID)
	}
}
