package profiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfilesFromYAML(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "test-model.yaml", `
id: test-model
name: Test Model
version: 1
family: test
aliases:
  - test:7b
context_limit: 16000
capabilities:
  chat: true
  structured_output: medium
  tool_calling: weak
  long_context: medium
defaults:
  temperature: 0.5
  top_p: 0.9
  repeat_penalty: 1.1
  max_tokens: 2048
context:
  strategy: hybrid
  max_input_tokens: 12000
prompt:
  style: technical
  instructions:
    - "Answer concisely."
guard:
  anti_loop: standard
  json_repair: true
  repetition_detection: true
notes:
  - "Test profile."
`)

	loader := NewLoader(dir)
	result, err := loader.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(result.Warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", result.Warnings)
	}
	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(result.Profiles))
	}
	p := result.Profiles[0]
	if p.ID != "test-model" {
		t.Fatalf("expected id test-model, got %q", p.ID)
	}
	if p.Family != "test" {
		t.Fatalf("expected family test, got %q", p.Family)
	}
	if len(p.Aliases) != 1 || p.Aliases[0] != "test:7b" {
		t.Fatalf("unexpected aliases: %v", p.Aliases)
	}
	if p.ContextLimit != 16000 {
		t.Fatalf("expected context_limit 16000, got %d", p.ContextLimit)
	}
	if p.Defaults.Temperature == nil || *p.Defaults.Temperature != 0.5 {
		t.Fatalf("expected temperature 0.5, got %v", p.Defaults.Temperature)
	}
	if p.Defaults.MaxTokens == nil || *p.Defaults.MaxTokens != 2048 {
		t.Fatalf("expected max_tokens 2048, got %v", p.Defaults.MaxTokens)
	}
	if p.Context.MaxInputTokens != 12000 {
		t.Fatalf("expected max_input_tokens 12000, got %d", p.Context.MaxInputTokens)
	}
	if len(p.Prompt.Instructions) != 1 || p.Prompt.Instructions[0] != "Answer concisely." {
		t.Fatalf("unexpected instructions: %v", p.Prompt.Instructions)
	}
	if p.Guard.AntiLoop != "standard" {
		t.Fatalf("expected anti_loop standard, got %q", p.Guard.AntiLoop)
	}
}

func TestMissingProfileDirectoryReturnsGenericFallback(t *testing.T) {
	loader := NewLoader(filepath.Join(t.TempDir(), "does-not-exist"))
	result, err := loader.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(result.Profiles) != 1 {
		t.Fatalf("expected exactly one fallback profile, got %d", len(result.Profiles))
	}
	if result.Profiles[0].ID != "generic-local" {
		t.Fatalf("expected generic-local, got %q", result.Profiles[0].ID)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected a warning about missing profile directory")
	}
}

func TestInvalidProfileSkipped(t *testing.T) {
	dir := t.TempDir()
	writeProfile(t, dir, "valid.yaml", `
id: valid-model
version: 1
family: valid
capabilities:
  chat: true
`)
	writeProfile(t, dir, "invalid.yaml", `
id:
version: 1
family: invalid
`)
	writeProfile(t, dir, "bad-capability.yaml", `
id: bad-cap
version: 1
family: bad
capabilities:
  structured_output: impossible
`)

	loader := NewLoader(dir)
	result, err := loader.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(result.Profiles) != 1 {
		t.Fatalf("expected 1 valid profile, got %d", len(result.Profiles))
	}
	if result.Profiles[0].ID != "valid-model" {
		t.Fatalf("expected valid-model, got %q", result.Profiles[0].ID)
	}
	if len(result.Warnings) < 2 {
		t.Fatalf("expected warnings for invalid profiles, got %v", result.Warnings)
	}
}

func TestDefaultLoaderFindsRealProfiles(t *testing.T) {
	// This test runs from the runtime package tree. The default loader walks
	// upward looking for a profiles/ directory at the repository root.
	result, err := NewDefaultLoader().Load()
	if err != nil {
		t.Fatalf("default load failed: %v", err)
	}
	ids := map[string]bool{}
	for _, p := range result.Profiles {
		ids[p.ID] = true
	}
	for _, id := range []string{"generic-local", "qwen3-8b", "qwen2.5-coder-7b", "deepseek-r1-8b", "llama3.1-8b", "gemma3-12b", "mistral-small", "qwen3.5-2b", "qwen3.5-9b"} {
		if !ids[id] {
			t.Fatalf("expected built-in profile %q to be loaded; got %v", id, ids)
		}
	}
}

func writeProfile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test profile %s: %v", path, err)
	}
}
