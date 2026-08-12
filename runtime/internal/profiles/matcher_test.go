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
	r := NewResolver(profiles, nil, nil)

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
	r := NewResolver(profiles, nil, nil)
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
	r := NewResolver(profiles, nil, nil)
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
	r := NewResolver(profiles, nil, nil)

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
	r := NewResolver(profiles, nil, nil)
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
	r := NewResolver(profiles, nil, nil)
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
	r := NewResolver(profiles, nil, nil)
	m := r.Resolve("lmstudio", "google/gemma-4-e4b")
	if m.Profile.ID != "gemma-4-e4b" {
		t.Fatalf("expected gemma-4-e4b, got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolverAlwaysIncludesGenericFallback(t *testing.T) {
	r := NewResolver(nil, nil, nil)
	m := r.Resolve("ollama", "anything")
	if m.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local, got %q", m.Profile.ID)
	}
}

func TestResolveFamilyPicksBestMatchBySize(t *testing.T) {
	profiles := []*Profile{
		{
			ID:     "qwen3.5-9b",
			Family: "qwen",
			Size:   "9b",
		},
		{
			ID:     "qwen3.5-2b",
			Family: "qwen",
			Size:   "2b",
		},
	}
	r := NewResolver(profiles, nil, nil)
	m := r.Resolve("ollama", "qwen3.5:2b")
	if m.Profile.ID != "qwen3.5-2b" {
		t.Fatalf("expected qwen3.5-2b (size 2b matches :2b), got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolveFamilyPicksBestMatchByID(t *testing.T) {
	profiles := []*Profile{
		{
			ID:     "qwen2.5-coder-7b",
			Family: "qwen",
			Size:   "7b",
		},
		{
			ID:     "qwen3.5-2b",
			Family: "qwen",
			Size:   "2b",
		},
	}
	r := NewResolver(profiles, nil, nil)
	m := r.Resolve("ollama", "qwen3.5:2b")
	if m.Profile.ID != "qwen3.5-2b" {
		t.Fatalf("expected qwen3.5-2b (id match), got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolveFamilyTieBreaksByLongestFamily(t *testing.T) {
	// Edge case: two profiles with different-length families, no size/id
	// disambiguation. The longer family should win.
	profiles := []*Profile{
		{
			ID:     "short-family",
			Family: "ab",
		},
		{
			ID:     "long-family",
			Family: "abc",
		},
	}
	r := NewResolver(profiles, nil, nil)
	// Model contains both "ab" and "abc" — "abc" is longer, so it wins.
	m := r.Resolve("ollama", "abc-model")
	if m.Profile.ID != "long-family" {
		t.Fatalf("expected long-family (longer family 'abc'), got %q", m.Profile.ID)
	}
	if m.IsFallback {
		t.Fatal("expected a real match, not fallback")
	}
}

func TestResolveFamilyFallsBackWhenNoFamilyMatch(t *testing.T) {
	profiles := []*Profile{
		{
			ID:     "qwen3-8b",
			Family: "qwen",
		},
	}
	r := NewResolver(profiles, nil, nil)
	m := r.Resolve("ollama", "llama3.1:8b")
	if m.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local fallback, got %q", m.Profile.ID)
	}
	if !m.IsFallback {
		t.Fatal("expected fallback match")
	}
}

func TestResolveFamilyOrderIndependence(t *testing.T) {
	// Two profiles with same family "qwen", sizes "2b" and "7b".
	// Model "qwen3.5:2b" must resolve to the 2b profile regardless of
	// which profile appears first in the list.
	small := &Profile{
		ID:     "qwen3.5-2b",
		Family: "qwen",
		Size:   "2b",
	}
	large := &Profile{
		ID:     "qwen3.5-9b",
		Family: "qwen",
		Size:   "9b",
	}

	// Order 1: small first.
	r1 := NewResolver([]*Profile{small, large}, nil, nil)
	m1 := r1.Resolve("ollama", "qwen3.5:2b")
	if m1.Profile.ID != "qwen3.5-2b" {
		t.Fatalf("(small first) expected qwen3.5-2b, got %q", m1.Profile.ID)
	}

	// Order 2: large first.
	r2 := NewResolver([]*Profile{large, small}, nil, nil)
	m2 := r2.Resolve("ollama", "qwen3.5:2b")
	if m2.Profile.ID != "qwen3.5-2b" {
		t.Fatalf("(large first) expected qwen3.5-2b, got %q", m2.Profile.ID)
	}
}

// ── Sprint 17R5: Profile Integrity Tests ────────────────────────

func TestResolveGemma34bToCorrectProfile(t *testing.T) {
	loader := NewDefaultLoader()
	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load profiles: %v", err)
	}
	if len(loaded.Warnings) > 0 {
		t.Fatalf("profile load produced warnings (indicates broken profiles): %v", loaded.Warnings)
	}
	resolver := NewResolver(loaded.Profiles, loaded.BrokenIDs, loaded.BrokenAlts)

	match := resolver.Resolve("ollama", "gemma3:4b")
	if match.IsFallback {
		t.Fatal("gemma3:4b should resolve to its own profile, not fallback")
	}
	if match.Profile.ID != "gemma3-4b" {
		t.Fatalf("expected gemma3-4b profile, got %s (reason=%s)", match.Profile.ID, match.Reason)
	}
}

func TestResolveGemma31bToCorrectProfile(t *testing.T) {
	loader := NewDefaultLoader()
	loaded, _ := loader.Load()
	resolver := NewResolver(loaded.Profiles, loaded.BrokenIDs, loaded.BrokenAlts)

	match := resolver.Resolve("ollama", "gemma3:1b")
	if match.IsFallback {
		t.Fatal("gemma3:1b should resolve to its own profile, not fallback")
	}
	if match.Profile.ID != "gemma3-1b" {
		t.Fatalf("expected gemma3-1b profile, got %s (reason=%s)", match.Profile.ID, match.Reason)
	}
}

func TestResolveLlama323bToCorrectProfile(t *testing.T) {
	loader := NewDefaultLoader()
	loaded, _ := loader.Load()
	resolver := NewResolver(loaded.Profiles, loaded.BrokenIDs, loaded.BrokenAlts)

	match := resolver.Resolve("ollama", "llama3.2:3b")
	if match.IsFallback {
		t.Fatal("llama3.2:3b should resolve to its own profile, not fallback")
	}
	if match.Profile.ID != "llama3.2-3b" {
		t.Fatalf("expected llama3.2-3b profile, got %s (reason=%s)", match.Profile.ID, match.Reason)
	}
}

func TestResolveGemma4E4bToCorrectProfile(t *testing.T) {
	loader := NewDefaultLoader()
	loaded, _ := loader.Load()
	resolver := NewResolver(loaded.Profiles, loaded.BrokenIDs, loaded.BrokenAlts)

	match := resolver.Resolve("ollama", "gemma-4:4b")
	if match.IsFallback {
		t.Fatal("gemma-3-4b should resolve to its own profile, not fallback")
	}
	if match.Profile.ID != "gemma-4-e4b" {
		t.Fatalf("expected gemma-4-e4b profile, got %s (reason=%s)", match.Profile.ID, match.Reason)
	}
}

func TestAllProfilesLoadWithoutWarnings(t *testing.T) {
	loader := NewDefaultLoader()
	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded.Warnings) > 0 {
		for _, w := range loaded.Warnings {
			t.Errorf("profile load warning: %s", w)
		}
		t.Fatalf("expected 0 warnings, got %d", len(loaded.Warnings))
	}
	if len(loaded.Profiles) < 16 {
		t.Fatalf("expected at least 16 profiles, got %d", len(loaded.Profiles))
	}
}

func TestBrokenProfileDoesNotResolveToUnrelatedProfile(t *testing.T) {
	// Simulate a broken profile by creating a resolver without the gemma3-4b profile
	// but with gemma3-12b present. Verify that if gemma3-4b were missing,
	// it would fall back to family matching, but a PARSED profile always wins.
	profiles := []*Profile{
		{ID: "gemma3-12b", Family: "gemma", Size: "12b", Models: map[string][]string{"ollama": {"gemma3:12b"}}},
		{ID: "generic-local", Family: "unknown", Size: "unknown"},
	}
	resolver := NewResolver(profiles, nil, nil)

	// Without gemma3-4b, this falls back to family match (gemma3-12b)
	match := resolver.Resolve("ollama", "gemma3:4b")
	if !match.IsFallback && match.Profile.ID != "gemma3-12b" {
		t.Fatalf("without gemma3-4b profile, expected family fallback to gemma3-12b, got %s", match.Profile.ID)
	}

	// With gemma3-4b present, it should resolve directly
	profiles = append(profiles, &Profile{
		ID: "gemma3-4b", Family: "gemma", Size: "4b",
		Models: map[string][]string{"ollama": {"gemma3:4b", "gemma3:latest"}},
	})
	resolver = NewResolver(profiles, nil, nil)
	match = resolver.Resolve("ollama", "gemma3:4b")
	if match.IsFallback {
		t.Fatal("with gemma3-4b present, should not fallback")
	}
	if match.Profile.ID != "gemma3-4b" {
		t.Fatalf("expected gemma3-4b, got %s", match.Profile.ID)
	}
}

func TestBrokenProfileBlocksFamilyFallback(t *testing.T) {
	// When a profile ID is in brokenIDs, resolving that exact model must NOT
	// fall through to an unrelated family match.
	profiles := []*Profile{
		{ID: "qwen3-8b", Family: "qwen", Size: "8b"},
		{ID: "generic-local", Family: "unknown", Size: "unknown"},
	}
	brokenIDs := []string{"qwen3-4b"}
	resolver := NewResolver(profiles, brokenIDs, nil)

	// qwen3-4b exactly matches a broken profile ID -> must return generic fallback
	// with reason "broken_profile", NOT family-match to qwen3-8b.
	match := resolver.Resolve("ollama", "qwen3-4b")
	if !match.IsFallback {
		t.Fatalf("expected fallback for broken profile, got profile %q reason=%q", match.Profile.ID, match.Reason)
	}
	if match.Profile.ID != "generic-local" {
		t.Fatalf("expected generic-local fallback, got %q", match.Profile.ID)
	}
	if match.Reason != "broken_profile" {
		t.Fatalf("expected reason broken_profile, got %q", match.Reason)
	}
}

func TestUnknownModelStillFallsBackNormally(t *testing.T) {
	// A genuinely unknown model (no matching profile at all) should still use
	// family heuristic / generic fallback as before.
	profiles := []*Profile{
		{ID: "qwen3-8b", Family: "qwen", Size: "8b"},
		{ID: "generic-local", Family: "unknown", Size: "unknown"},
	}
	resolver := NewResolver(profiles, nil, nil)

	match := resolver.Resolve("ollama", "some-unknown-family-model")
	// Should get generic fallback since no family match exists.
	if !match.IsFallback {
		t.Fatalf("expected generic fallback for unknown model, got profile %q", match.Profile.ID)
	}
}

func TestBrokenProfileDoesNotAffectUnrelatedModels(t *testing.T) {
	// A broken qwen3-4b profile should not affect resolution of unrelated models.
	profiles := []*Profile{
		{ID: "llama3.1-8b", Family: "llama", Size: "8b"},
		{ID: "generic-local", Family: "unknown", Size: "unknown"},
	}
	brokenIDs := []string{"qwen3-4b"}
	resolver := NewResolver(profiles, brokenIDs, nil)

	// llama3.1:8b should still resolve via family heuristic to llama3.1-8b.
	match := resolver.Resolve("ollama", "llama3.1:8b")
	if match.IsFallback {
		t.Fatal("llama3.1:8b should not fall back when a family match exists")
	}
	if match.Profile.ID != "llama3.1-8b" {
		t.Fatalf("expected llama3.1-8b, got %q", match.Profile.ID)
	}
}
