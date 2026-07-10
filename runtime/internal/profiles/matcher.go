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
// family heuristic, generic fallback.
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

	// Family heuristic: model name contains the profile family.
	for _, p := range r.profiles {
		if p == nil || p.Family == "" || p.Family == "unknown" {
			continue
		}
		family := strings.ToLower(p.Family)
		if strings.Contains(model, family) {
			return &Match{Profile: p, Reason: "family"}
		}
	}

	return fallback("no matching profile")
}

func fallback(reason string) *Match {
	return &Match{
		Profile:    GenericFallback(),
		IsFallback: true,
		Reason:     reason,
	}
}
