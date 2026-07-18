package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/EffNine/gumi/benchmark"
)

// loadJSONLSuite converts a JSONL data source into a populated Suite. Each
// line must contain at least "task_id", "prompt", "test", and "entry_point".
func loadJSONLSuite(baseSuite benchmark.Suite, sourcePath string) (benchmark.Suite, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return benchmark.Suite{}, fmt.Errorf("opening JSONL %s: %w", sourcePath, err)
	}
	defer f.Close()

	defaultTimeout := baseSuite.TimeoutSeconds
	if defaultTimeout == 0 {
		defaultTimeout = 60
	}
	defaultMaxTokens := baseSuite.MaxTokens
	if defaultMaxTokens == 0 {
		defaultMaxTokens = 512
	}

	var tests []benchmark.SuiteTest
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row humanevalRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return benchmark.Suite{}, fmt.Errorf("parsing JSONL row in %s: %w", sourcePath, err)
		}

		tests = append(tests, benchmark.SuiteTest{
			ID:             row.TaskID,
			Difficulty:     baseSuite.Tier,
			Description:    fmt.Sprintf("HumanEval problem %s", row.TaskID),
			Prompt:         row.Prompt,
			TimeoutSeconds: defaultTimeout,
			MaxTokens:      defaultMaxTokens,
			Constraints: []benchmark.Constraint{
				{
					Field:    "python_exec",
					Operator: "python_exec",
					Value: map[string]interface{}{
						"test":            row.Test,
						"entry_point":     row.EntryPoint,
						"prompt":          row.Prompt,
						"timeout_seconds": defaultTimeout,
					},
				},
			},
		})
	}

	if err := scanner.Err(); err != nil {
		return benchmark.Suite{}, fmt.Errorf("scanning JSONL %s: %w", sourcePath, err)
	}

	baseSuite.Tests = tests
	return baseSuite, nil
}

// resolveDataSource returns an absolute path for a suite's data_source value.
func resolveDataSource(suitesDir, category, dataSource string) string {
	if filepath.IsAbs(dataSource) {
		return dataSource
	}
	// First check the category directory.
	candidate := filepath.Join(suitesDir, category, dataSource)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// Fall back to a path relative to the suites root.
	return filepath.Join(suitesDir, dataSource)
}

// humanevalRow mirrors the canonical HumanEval JSONL record.
type humanevalRow struct {
	TaskID            string `json:"task_id"`
	Prompt            string `json:"prompt"`
	CanonicalSolution string `json:"canonical_solution"`
	Test              string `json:"test"`
	EntryPoint        string `json:"entry_point"`
}
