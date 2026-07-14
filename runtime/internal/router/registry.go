package router

import (
	"fmt"
	"sort"
	"strings"

	"github.com/EffNine/gumi/runtime/internal/profiles"
)

// ---------------------------------------------------------------------------
// CodingModel
// ---------------------------------------------------------------------------

// CodingStrength ranks a model's coding capability from 1 (weakest) to 3 (strongest).
type CodingStrength int

const (
	CodingStrengthNone   CodingStrength = 0
	CodingStrengthWeak   CodingStrength = 1
	CodingStrengthMedium CodingStrength = 2
	CodingStrengthStrong CodingStrength = 3
)

func parseCodingStrength(s string) CodingStrength {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strong":
		return CodingStrengthStrong
	case "medium":
		return CodingStrengthMedium
	case "weak":
		return CodingStrengthWeak
	}
	return CodingStrengthNone
}

func (s CodingStrength) String() string {
	switch s {
	case CodingStrengthStrong:
		return "strong"
	case CodingStrengthMedium:
		return "medium"
	case CodingStrengthWeak:
		return "weak"
	}
	return "none"
}

// ModelSizeCategory groups models by parameter count.
type ModelSizeCategory string

const (
	SizeTiny   ModelSizeCategory = "tiny"   // < 3B
	SizeSmall  ModelSizeCategory = "small"  // 3-7B
	SizeMedium ModelSizeCategory = "medium" // 7-15B
	SizeLarge  ModelSizeCategory = "large"  // >= 15B
)

func classifySize(size string) ModelSizeCategory {
	s := strings.ToLower(strings.TrimSpace(size))
	switch {
	case strings.HasPrefix(s, "1") || strings.HasPrefix(s, "2"):
		return SizeTiny
	case strings.HasPrefix(s, "3") || strings.HasPrefix(s, "4") || strings.HasPrefix(s, "5") || strings.HasPrefix(s, "6"):
		return SizeSmall
	case strings.HasPrefix(s, "7") || strings.HasPrefix(s, "8") || strings.HasPrefix(s, "9") || strings.HasPrefix(s, "10") || strings.HasPrefix(s, "11") || strings.HasPrefix(s, "12") || strings.HasPrefix(s, "13") || strings.HasPrefix(s, "14"):
		return SizeMedium
	default:
		return SizeLarge
	}
}

// CodingModelRegistryEntry holds a model's coding-relevant attributes.
type CodingModelRegistryEntry struct {
	ProfileID      string            `json:"profile_id"`
	Provider       string            `json:"provider"`
	ModelName      string            `json:"model_name"`
	CodingStrength CodingStrength    `json:"coding_strength"`
	ToolCalling    string            `json:"tool_calling"`
	Reasoning      string            `json:"reasoning"`
	ContextLimit   int               `json:"context_limit"`
	SizeCategory   ModelSizeCategory `json:"size_category"`
	Profile        *profiles.Profile `json:"-"`
}

// CodingModelRegistry indexes all available models by their coding capabilities.
type CodingModelRegistry struct {
	entries []CodingModelRegistryEntry
}

// NewCodingModelRegistry builds a registry from a list of profiles and the
// currently configured providers. Each profile may map to multiple provider/model
// pairs (e.g., the same profile matches "ollama:qwen3:8b" and "lmstudio:qwen3-8b").
func NewCodingModelRegistry(
	profilesList []*profiles.Profile,
	providerModels map[string][]string, // providerKey → model names
) *CodingModelRegistry {
	r := &CodingModelRegistry{}
	seen := map[string]bool{}

	for _, p := range profilesList {
		if p == nil {
			continue
		}
		cs := parseCodingStrength(p.Capabilities.Coding)
		size := classifySize(p.Size)
		tc := p.Capabilities.ToolCalling
		if tc == "" {
			tc = "unknown"
		}
		reasoning := p.Capabilities.Reasoning
		if reasoning == "" {
			reasoning = "unknown"
		}
		ctxLimit := p.ContextLimit
		if ctxLimit <= 0 {
			ctxLimit = 32000
		}

		// For each provider that has models matching this profile, create an entry.
		for providerKey, models := range providerModels {
			for _, modelName := range models {
				key := providerKey + ":" + modelName
				if seen[key] {
					continue
				}
				seen[key] = true
				r.entries = append(r.entries, CodingModelRegistryEntry{
					ProfileID:      p.ID,
					Provider:       providerKey,
					ModelName:      modelName,
					CodingStrength: cs,
					ToolCalling:    tc,
					Reasoning:      reasoning,
					ContextLimit:   ctxLimit,
					SizeCategory:   size,
					Profile:        p,
				})
			}
		}
	}

	sort.Slice(r.entries, func(i, j int) bool {
		return r.entries[i].CodingStrength > r.entries[j].CodingStrength
	})

	return r
}

// FindBest selects the best matching entry given a preference strategy and
// minimum requirements. Returns nil if no entry meets all requirements.
func (r *CodingModelRegistry) FindBest(
	strategy PreferenceStrategy,
	minCoding CodingStrength,
	minContext int,
	minReasoning string,
	maxSize ModelSizeCategory,
	availableModels map[string]bool, // "provider:model" → available
) *CodingModelRegistryEntry {
	candidates := r.filter(minCoding, minContext, minReasoning, maxSize, availableModels)
	if len(candidates) == 0 {
		// Relax constraints: try with no min requirements.
		candidates = r.filter(CodingStrengthNone, 0, "", "", availableModels)
		if len(candidates) == 0 {
			return nil
		}
	}

	switch strategy {
	case PreferenceFastest:
		// Fastest: prefer smallest model that meets requirements.
		sort.Slice(candidates, func(i, j int) bool {
			return sizeRank(candidates[i].SizeCategory) < sizeRank(candidates[j].SizeCategory)
		})
	case PreferenceBestCoding:
		// Best coding: prefer strongest coding ability.
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CodingStrength != candidates[j].CodingStrength {
				return candidates[i].CodingStrength > candidates[j].CodingStrength
			}
			return candidates[i].ContextLimit > candidates[j].ContextLimit
		})
	case PreferenceBestCombo:
		// Weighted score: coding×0.5 + reasoning×0.3 + tool_calling×0.2.
		sort.Slice(candidates, func(i, j int) bool {
			return comboScore(&candidates[i]) > comboScore(&candidates[j])
		})
	case PreferenceLargestContext:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].ContextLimit > candidates[j].ContextLimit
		})
	}

	return &candidates[0]
}

// List returns all registry entries.
func (r *CodingModelRegistry) List() []CodingModelRegistryEntry {
	return r.entries
}

// filter returns entries meeting all minimum requirements and that are in the
// available set (or all entries if availableModels is empty).
func (r *CodingModelRegistry) filter(
	minCoding CodingStrength,
	minContext int,
	minReasoning string,
	maxSize ModelSizeCategory,
	availableModels map[string]bool,
) []CodingModelRegistryEntry {
	var result []CodingModelRegistryEntry

	for _, e := range r.entries {
		if e.CodingStrength < minCoding {
			continue
		}
		if minContext > 0 && e.ContextLimit < minContext {
			continue
		}
		if minReasoning != "" && !reasoningAtLeast(e.Reasoning, minReasoning) {
			continue
		}
		if maxSize != "" && sizeRank(e.SizeCategory) > sizeRank(maxSize) {
			continue
		}
		if availableModels != nil {
			key := e.Provider + ":" + e.ModelName
			if !availableModels[key] {
				continue
			}
		}
		result = append(result, e)
	}

	return result
}

// reasoningAtLeast returns true if the model's reasoning level meets or exceeds
// the minimum required level.
func reasoningAtLeast(modelLevel, minLevel string) bool {
	levels := []string{"none", "weak", "medium", "strong"}
	mi, miOk := indexOf(levels, strings.ToLower(modelLevel))
	ni, niOk := indexOf(levels, strings.ToLower(minLevel))
	if !miOk || !niOk {
		return false
	}
	return mi >= ni
}

func indexOf(slice []string, s string) (int, bool) {
	for i, v := range slice {
		if v == s {
			return i, true
		}
	}
	return -1, false
}

func sizeRank(size ModelSizeCategory) int {
	switch size {
	case SizeTiny:
		return 1
	case SizeSmall:
		return 2
	case SizeMedium:
		return 3
	case SizeLarge:
		return 4
	}
	return 5
}

func comboScore(e *CodingModelRegistryEntry) float64 {
	cs := float64(e.CodingStrength) / 3.0 // 0..1
	rs := reasoningScore(e.Reasoning)     // 0..1
	ts := toolCallingScore(e.ToolCalling) // 0..1
	return cs*0.5 + rs*0.3 + ts*0.2
}

func reasoningScore(level string) float64 {
	switch strings.ToLower(level) {
	case "strong":
		return 1.0
	case "medium":
		return 0.7
	case "weak":
		return 0.3
	}
	return 0.0
}

func toolCallingScore(level string) float64 {
	switch strings.ToLower(level) {
	case "strong":
		return 1.0
	case "medium":
		return 0.7
	case "weak":
		return 0.3
	}
	return 0.0
}

// Describe returns a human-readable explanation of why this entry was selected.
func (e *CodingModelRegistryEntry) Describe() string {
	return fmt.Sprintf("%s/%s (coding:%s, reasoning:%s, context:%d, size:%s)",
		e.Provider, e.ModelName, e.CodingStrength,
		e.Reasoning, e.ContextLimit, e.SizeCategory)
}
