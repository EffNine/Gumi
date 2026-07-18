package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/EffNine/gumi/benchmark"
	"gopkg.in/yaml.v3"
)

// Manifest is the top-level structure of the benchmark suite manifest file
// (_manifest.yaml). It lists all test categories and their supported tiers.
type Manifest struct {
	Categories []ManifestCategory `yaml:"categories"`
}

// ManifestCategory describes one test category within the benchmark manifest,
// including its display name and the difficulty tiers it supports.
type ManifestCategory struct {
	ID    string   `yaml:"id"`
	Name  string   `yaml:"name"`
	Tiers []string `yaml:"tiers"`
}

// suiteFile wraps the YAML structure for a single suite file.
type suiteFile struct {
	Suite benchmark.Suite `yaml:"suite"`
}

// LoadSuites loads all test suites matching the given model tier.
// It reads YAML files from the benchmark/suites/ directory based on the manifest.
func LoadSuites(tier ModelTier) ([]benchmark.Suite, error) {
	tiersToLoad := tier.SuitesToRun()
	suitesDir := SuitesDir()

	manifestPath := filepath.Join(suitesDir, "_manifest.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading suite manifest at %s: %w", manifestPath, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parsing suite manifest: %w", err)
	}

	var suites []benchmark.Suite
	for _, cat := range manifest.Categories {
		for _, tierName := range tiersToLoad {
			if !containsTier(cat.Tiers, tierName) {
				continue
			}

			suite, err := loadSuiteFile(cat.ID, tierName)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("loading suite %s/%s: %w", cat.ID, tierName, err)
			}
			suites = append(suites, suite)
		}
	}

	return suites, nil
}

// loadSuiteFile reads and parses a single suite YAML file.
func loadSuiteFile(category, tier string) (benchmark.Suite, error) {
	suitesDir := SuitesDir()
	data, err := os.ReadFile(filepath.Join(suitesDir, category, tier+".yaml"))
	if err != nil {
		return benchmark.Suite{}, err
	}

	var sf suiteFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return benchmark.Suite{}, fmt.Errorf("parsing suite YAML: %w", err)
	}

	if sf.Suite.Category == "" {
		sf.Suite.Category = category
	}

	if sf.Suite.DataSource != "" {
		sourcePath := resolveDataSource(suitesDir, category, sf.Suite.DataSource)
		return loadJSONLSuite(sf.Suite, sourcePath)
	}

	return sf.Suite, nil
}

// containsTier checks whether a tier name is in a list of tiers.
func containsTier(tiers []string, target string) bool {
	for _, t := range tiers {
		if t == target {
			return true
		}
	}
	return false
}

// SuitesDir returns the absolute path to the benchmark suites directory by
// probing a set of known relative paths. Returns "suites" if none are found.
func SuitesDir() string {
	candidates := []string{
		"suites",
		"../suites",
		"benchmark/suites",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "suites"
}
