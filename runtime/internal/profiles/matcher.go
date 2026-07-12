// Package profiles resolves a provider/model pair to a model profile.
package profiles

import (
	"strings"
)

// Match describes the result of profile resolution.
type Match struct {
	Profile    *Profile
	IsFallback bool
	Reason     string
}

// Resolver selects the best matching profile for a provider/model pair.
type Resolver struct {
	profiles []*Profile
}

// NewResolver creates a resolver from a list of loaded profiles.
// If the list does not contain generic-local, the fallback profile is
// appended automatically.
func NewResolver(profiles []*Profile) *Resolver {
	r := &Resolver{profiles: append([]*Profile(nil), profiles...)}
	hasGeneric := false
	for _, p := range r.profiles {
		if p != nil && p.ID == "generic-local" {
			hasGeneric = true
			break
		}
	}
	if !hasGeneric {
		r.profiles = append(r.profiles, GenericFallback())
	}
	return r
}

// Resolve matches a provider-native model name to a profile.
// Matching order: provider-specific alias, global alias, profile id,
// family heuristic (scored), generic fallback.
func (r *Resolver) Resolve(providerKey, modelName string) *Match {
	key := strings.ToLower(strings.TrimSpace(providerKey))
	model := strings.ToLower(strings.TrimSpace(modelName))
	if model == "" {
		return fallback("empty model name")
	}

	// Provider-specific aliases take priority.
	for _, p := range r.profiles {
		if p == nil {
			continue
		}
		aliases, ok := p.Models[key]
		if !ok {
			continue
		}
		for _, alias := range aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == model {
				return &Match{Profile: p, Reason: "provider_alias"}
			}
		}
	}

	// Global aliases and profile ids are exact matches.
	for _, p := range r.profiles {
		if p == nil {
			continue
		}
		for _, alias := range p.Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == model {
				return &Match{Profile: p, Reason: "global_alias"}
			}
		}
		if strings.ToLower(p.ID) == model {
			return &Match{Profile: p, Reason: "profile_id"}
		}
	}

	// Family heuristic: score all matching profiles, pick the best.
	var best *Profile
	var bestScore int
	var bestFamilyLen int
	var bestIDLen int
	for _, p := range r.profiles {
		score := scoreFamilyMatch(model, p, key)
		if score <= 0 {
			continue
		}
		family := strings.ToLower(p.Family)
		if score > bestScore ||
			(score == bestScore && len(family) > bestFamilyLen) ||
			(score == bestScore && len(family) == bestFamilyLen && len(p.ID) > bestIDLen) {
			best = p
			bestScore = score
			bestFamilyLen = len(family)
			bestIDLen = len(p.ID)
		}
	}
	if best != nil {
		return &Match{Profile: best, Reason: "family"}
	}

	return fallback("no matching profile")
}

// scoreFamilyMatch returns a positive score if p's family is contained in
// model, or 0 if no match. Higher score = better match.
// Scoring factors (cumulative):
//   - Base: len(family) — longer family substring is more specific.
//   - Size bonus (+100): profile Size appears as a delimited token in model.
//   - ID bonus (+50):  profile ID (with "-" → ":" normalization) is a
//     substring of model.
//   - Provider alias bonus (+20): a provider-specific model alias is a
//     substring of model (or vice versa).
func scoreFamilyMatch(model string, p *Profile, providerKey string) int {
	if p == nil || p.Family == "" || p.Family == "unknown" {
		return 0
	}
	family := strings.ToLower(p.Family)
	if !strings.Contains(model, family) {
		return 0
	}

	score := len(family)

	// Size bonus: profile Size as a delimited token in the model name.
	if p.Size != "" {
		size := strings.ToLower(p.Size)
		if containsToken(model, size) {
			score += 100
		}
	}

	// ID bonus: normalize ID by replacing "-" with ":" and check substring.
	if p.ID != "" {
		normalizedID := strings.ReplaceAll(strings.ToLower(p.ID), "-", ":")
		if strings.Contains(model, normalizedID) {
			score += 50
		}
	}

	// Provider-specific model alias bonus.
	if providerKey != "" && p.Models != nil {
		if aliases, ok := p.Models[providerKey]; ok {
			for _, alias := range aliases {
				a := strings.ToLower(strings.TrimSpace(alias))
				if strings.Contains(model, a) || strings.Contains(a, model) {
					score += 20
					break
				}
			}
		}
	}

	return score
}

// containsToken reports whether token appears in s bounded by common
// delimiters (:, /, @, -) or at string boundaries.
func containsToken(s, token string) bool {
	if s == token {
		return true
	}
	delims := []string{":", "/", "@", "-"}
	for _, d := range delims {
		if strings.HasPrefix(s, token+d) || strings.HasSuffix(s, d+token) {
			return true
		}
		if strings.Contains(s, d+token+d) {
			return true
		}
	}
	return false
}

func fallback(reason string) *Match {
	return &Match{
		Profile:    GenericFallback(),
		IsFallback: true,
		Reason:     reason,
	}
}
