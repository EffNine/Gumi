// Package leaderboard provides curated public benchmark scores for comparison.
package leaderboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScoreEntry is a single curated leaderboard score.
type ScoreEntry struct {
	Benchmark string  `json:"benchmark"`
	Model     string  `json:"model"`
	Score     float64 `json:"score"`
	Source    string  `json:"source,omitempty"`
}

// ScoreMap indexes leaderboard entries by normalized model name.
type ScoreMap map[string]ScoreEntry

// LoadHumanevalScores reads the curated HumanEval score file from the given
// directory. If dir is empty, it defaults to the current working directory.
func LoadHumanevalScores(dir string) (ScoreMap, error) {
	path := filepath.Join(dir, "humaneval_scores.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var entries []ScoreEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	m := make(ScoreMap, len(entries))
	for _, e := range entries {
		m[normalizeModelName(e.Model)] = e
	}
	return m, nil
}

// Lookup returns the published HumanEval score for a model, or 0 if unknown.
// The model name is normalized so provider prefixes and tag variations match.
func (m ScoreMap) Lookup(model string) float64 {
	if m == nil {
		return 0
	}
	key := normalizeModelName(model)
	if e, ok := m[key]; ok {
		return e.Score
	}
	// Try stripping the provider prefix if the first attempt failed.
	if idx := strings.Index(key, ":"); idx >= 0 {
		if e, ok := m[key[idx+1:]]; ok {
			return e.Score
		}
	}
	return 0
}

// normalizeModelName lowercases and removes common provider prefixes and
// whitespace so "ollama:qwen2.5-coder:7b" matches "qwen2.5-coder:7b".
func normalizeModelName(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	for _, prefix := range []string{"ollama:", "lmstudio:", "openai-compatible:", "openai_compatible_local:"} {
		model = strings.TrimPrefix(model, prefix)
	}
	return model
}
