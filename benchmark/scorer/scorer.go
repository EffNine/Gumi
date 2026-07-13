// Package scorer implements the scoring engine for benchmark test results.
package scorer

import (
	"fmt"
	"strings"

	"github.com/novexa/novexa/benchmark"
)

// Scorer evaluates model responses against test constraints and produces scores.
type Scorer struct {
	checks map[string]CheckFunc
}

// New creates a new Scorer with the default check registry.
func New() *Scorer {
	reg := make(map[string]CheckFunc)
	for name, fn := range CheckRegistry {
		reg[name] = fn
	}
	return &Scorer{checks: reg}
}

// Score evaluates a single test's response against its constraints.
// Returns a TestResult with per-constraint subscores (1.0 = pass, 0.0 = fail).
// If there are no constraints, the test is assumed to pass.
func (s *Scorer) Score(test benchmark.SuiteTest, response string) benchmark.TestResult {
	result := benchmark.TestResult{
		TestID:    test.ID,
		Passed:    true,
		Subscores: make(map[string]float64),
	}

	if len(test.Constraints) == 0 {
		return result
	}

	var errors []string
	allPassed := true

	for _, constraint := range test.Constraints {
		checkFn, ok := s.checks[constraint.Operator]
		if !ok {
			result.Subscores[constraint.Field] = 0.0
			errors = append(errors, fmt.Sprintf("unknown operator %q for field %q", constraint.Operator, constraint.Field))
			allPassed = false
			continue
		}

		checkResult := checkFn(response, constraint)
		if checkResult.Passed {
			result.Subscores[constraint.Field] = 1.0
		} else {
			result.Subscores[constraint.Field] = 0.0
			errors = append(errors, fmt.Sprintf("%s: %s", constraint.Field, checkResult.Details))
			allPassed = false
		}
	}

	result.Passed = allPassed
	if len(errors) > 0 {
		result.Error = strings.Join(errors, "; ")
	}

	return result
}

// AggregateCapabilities groups test results by category (capability) and condition,
// computes per-capability MetricSets for direct and novexa conditions, and calculates
// delta and effect size for each capability.
func AggregateCapabilities(results []benchmark.TestResult, testCategories map[string]string) map[string]Capability {
	// Group results by (category, condition-type)
	type groupKey struct {
		category  string
		isNovexa  bool // true for novexa-* conditions, false for direct
	}

	groups := make(map[groupKey][]benchmark.TestResult)
	for _, r := range results {
		cat := testCategories[r.TestID]
		if cat == "" || cat == "degradation" {
			continue // skip degradation tests in capability aggregation
		}
		isNovexa := strings.HasPrefix(r.Condition, "novexa-")
		key := groupKey{category: cat, isNovexa: isNovexa}
		groups[key] = append(groups[key], r)
	}

	caps := make(map[string]Capability)

	// Collect all unique categories
	categories := make(map[string]bool)
	for k := range groups {
		categories[k.category] = true
	}

	for cat := range categories {
		directResults := groups[groupKey{category: cat, isNovexa: false}]
		novexaResults := groups[groupKey{category: cat, isNovexa: true}]

		directMS := Aggregate(directResults, nil)
		novexaMS := Aggregate(novexaResults, nil)

		delta := novexaMS.Mean - directMS.Mean
		effectSize := CohenD(directMS, novexaMS)

		caps[cat] = Capability{
			Direct:      directMS,
			Novexa:      novexaMS,
			Delta:       delta,
			EffectSize:  effectSize,
		}
	}

	return caps
}
