package leaderboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHumanevalScores(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "humaneval_scores.json")
	data := []byte(`[
		{"benchmark":"HumanEval","model":"qwen2.5-coder:7b","score":0.824,"source":"qwen report"}
	]`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write scores: %v", err)
	}

	scores, err := LoadHumanevalScores(tmpDir)
	if err != nil {
		t.Fatalf("LoadHumanevalScores: %v", err)
	}

	if got := scores.Lookup("qwen2.5-coder:7b"); got != 0.824 {
		t.Errorf("expected 0.824, got %f", got)
	}
	if got := scores.Lookup("ollama:qwen2.5-coder:7b"); got != 0.824 {
		t.Errorf("expected provider-prefixed lookup to return 0.824, got %f", got)
	}
	if got := scores.Lookup("unknown-model"); got != 0 {
		t.Errorf("expected 0 for unknown model, got %f", got)
	}
}
