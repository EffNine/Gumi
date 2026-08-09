// Package baselines provides storage and comparison for GEP benchmark baselines.
package baselines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

// Store manages GEP baseline storage.
type Store struct {
	path string
}

// NewStore creates a new baseline store at the given path.
func NewStore(path string) *Store {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".gumi", "gep", "baselines")
	}
	return &Store{path: path}
}

// Save stores a GEP report as a baseline.
func (s *Store) Save(report *types.GEPReport) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	scope := report.Config.Scope
	if scope == "" {
		scope = types.ScopeRuntime
	}

	baseline := types.GEPBaseline{
		RunID:        report.RunID,
		Model:        report.Config.Model,
		Provider:     report.Config.Provider,
		Scope:        scope,
		Timestamp:    report.Config.Timestamp,
		OverallScore: report.Summary.OverallScore,
		Capabilities: make(map[string]types.GEPCapability),
		Config:       report.Config,
	}

	for k, v := range report.Capabilities {
		baseline.Capabilities[k] = v
	}

	dir := filepath.Join(s.path, string(scope), report.Config.Model)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating baseline directory: %w", err)
	}

	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling baseline: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.json", sanitizeName(baseline.RunID), baseline.Timestamp.Format("20060102T150405Z"))
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing baseline file: %w", err)
	}

	return nil
}

// Load loads all baselines for a model across all scopes.
func (s *Store) Load(model string) ([]types.GEPBaseline, error) {
	var allBaselines []types.GEPBaseline

	// Search in scoped directories (runtime, model)
	for _, scope := range []types.GEPScope{types.ScopeRuntime, types.ScopeModel} {
		modelDir := filepath.Join(s.path, string(scope), model)
		baselines, err := s.loadFromDir(modelDir)
		if err != nil {
			continue
		}
		allBaselines = append(allBaselines, baselines...)
	}

	// Also search in legacy unscoped directory for backward compatibility
	legacyDir := filepath.Join(s.path, model)
	legacyBaselines, err := s.loadFromDir(legacyDir)
	if err == nil {
		allBaselines = append(allBaselines, legacyBaselines...)
	}

	if len(allBaselines) == 0 {
		return nil, nil
	}

	// Sort by timestamp descending
	sort.Slice(allBaselines, func(i, j int) bool {
		return allBaselines[i].Timestamp.After(allBaselines[j].Timestamp)
	})

	return allBaselines, nil
}

// loadFromDir loads baselines from a single directory.
func (s *Store) loadFromDir(modelDir string) ([]types.GEPBaseline, error) {
	entries, err := os.ReadDir(modelDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading baseline directory: %w", err)
	}

	var baselines []types.GEPBaseline
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(modelDir, entry.Name()))
		if err != nil {
			continue
		}
		var b types.GEPBaseline
		if err := json.Unmarshal(data, &b); err != nil {
			continue
		}
		baselines = append(baselines, b)
	}

	return baselines, nil
}

// GetLatest returns the most recent baseline for a model, or nil if none exists.
func (s *Store) GetLatest(model string) (*types.GEPBaseline, error) {
	baselines, err := s.Load(model)
	if err != nil {
		return nil, err
	}
	if len(baselines) == 0 {
		return nil, nil
	}
	return &baselines[0], nil
}

// Compare compares a new report against the latest baseline for the same model.
func (s *Store) Compare(report *types.GEPReport) (*types.GEPRegression, error) {
	latest, err := s.GetLatest(report.Config.Model)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, nil // No baseline to compare against
	}

	regression := &types.GEPRegression{
		BaselineRunID:    latest.RunID,
		BaselineScore:    latest.OverallScore,
		CurrentScore:     report.Summary.OverallScore,
		Delta:            report.Summary.OverallScore - latest.OverallScore,
		Regression:       report.Summary.OverallScore < latest.OverallScore,
		CapabilityDeltas: make(map[string]types.CapabilityDelta),
	}

	// Compare per-capability scores
	for k, baselineCap := range latest.Capabilities {
		currentCap, ok := report.Capabilities[k]
		if !ok {
			continue
		}
		delta := currentCap.Gumi.Mean - baselineCap.Gumi.Mean
		regression.CapabilityDeltas[k] = types.CapabilityDelta{
			SuiteType:  currentCap.SuiteType,
			Baseline:   baselineCap.Gumi.Mean,
			Current:    currentCap.Gumi.Mean,
			Delta:      delta,
			Regression: delta < 0,
		}
	}

	return regression, nil
}

// ListModels returns all models that have baselines stored across all scopes.
func (s *Store) ListModels() ([]string, error) {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading baselines directory: %w", err)
	}

	modelSet := make(map[string]struct{})
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Entry could be a scope directory (runtime, model) or a legacy model dir
		scopeEntries, err := os.ReadDir(filepath.Join(s.path, entry.Name()))
		if err != nil {
			continue
		}
		for _, scopeEntry := range scopeEntries {
			if scopeEntry.IsDir() {
				modelSet[scopeEntry.Name()] = struct{}{}
			}
		}
		// If no subdirectories, treat as legacy model dir
		if len(scopeEntries) == 0 {
			modelSet[entry.Name()] = struct{}{}
		}
	}

	var models []string
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)
	return models, nil
}

func sanitizeName(name string) string {
	replacer := filepath.FromSlash("/")
	return replacer
}
