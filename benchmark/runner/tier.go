package runner

import (
	"regexp"
	"strconv"
	"strings"
)

// ModelTier classifies a model by its capability level.
type ModelTier string

const (
	TierSmall    ModelTier = "small"
	TierMedium   ModelTier = "medium"
	TierFrontier ModelTier = "frontier"
)

// ResolveTier determines the appropriate tier for a given model and provider combination.
//
// Resolution order:
//  1. Known frontier providers (anthropic, openai, google) → frontier
//  2. Known frontier model name patterns (gpt-4, claude-4, gemini-2, fable) → frontier
//  3. Size-based heuristic from model name (e.g., "7b", "12b", "70b")
//  4. Default → medium
func ResolveTier(model string, provider string) (ModelTier, error) {
	// 1. Check provider-based heuristic
	switch provider {
	case "anthropic", "openai", "google":
		return TierFrontier, nil
	}

	// 2. Check model name heuristics for known frontier models
	modelLower := strings.ToLower(model)
	frontierPatterns := []string{"gpt-4", "claude-4", "gemini-2", "fable", "claude-3.5"}
	for _, pat := range frontierPatterns {
		if strings.Contains(modelLower, pat) {
			return TierFrontier, nil
		}
	}

	// 3. Size-based heuristic for local models
	// Extract model size from name (e.g., "1b", "7b", "8b", "12b", "70b", "qwen3.5-9b")
	re := regexp.MustCompile(`(\d+)[bB]`)
	matches := re.FindStringSubmatch(modelLower)
	if len(matches) >= 2 {
		size, err := strconv.Atoi(matches[1])
		if err == nil {
			switch {
			case size < 8:
				return TierSmall, nil
			case size <= 32:
				return TierMedium, nil
			default:
				return TierFrontier, nil
			}
		}
	}

	// 4. Default to medium for unknown local models
	return TierMedium, nil
}

// SuitesToRun returns the list of suite tier IDs that should be executed for this model tier.
// Degradation checks are always included for the applicable tiers.
func (t ModelTier) SuitesToRun() []string {
	switch t {
	case TierSmall:
		return []string{"easy", "medium", "cosmetic"}
	case TierMedium:
		return []string{"easy", "medium", "hard", "cosmetic", "semantic"}
	case TierFrontier:
		return []string{"hard", "frontier", "cosmetic", "semantic"}
	default:
		return []string{"easy", "medium"}
	}
}
