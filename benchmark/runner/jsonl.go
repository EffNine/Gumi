package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/EffNine/gumi/benchmark"
)

// loadJSONLSuite converts a JSONL data source into a populated Suite.
// It auto-detects the format based on the fields present in the first row:
//   - HumanEval format: task_id, prompt, test, entry_point
//   - GSM8K format: task_id, question, answer
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

	// Peek at the first row to detect format.
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return benchmark.Suite{}, fmt.Errorf("empty JSONL file: %s", sourcePath)
	}
	firstRow := scanner.Bytes()

	var probe map[string]interface{}
	if err := json.Unmarshal(firstRow, &probe); err != nil {
		return benchmark.Suite{}, fmt.Errorf("parsing first JSONL row in %s: %w", sourcePath, err)
	}

	_, hasTest := probe["test"]
	_, hasEntryPoint := probe["entry_point"]
	_, hasQuestion := probe["question"]
	_, hasAnswer := probe["answer"]

	var tests []benchmark.SuiteTest

	if hasTest && hasEntryPoint {
		// HumanEval format
		var row humanevalRow
		if err := json.Unmarshal(firstRow, &row); err != nil {
			return benchmark.Suite{}, fmt.Errorf("parsing HumanEval row in %s: %w", sourcePath, err)
		}
		tests = append(tests, buildHumanevalTest(row, baseSuite.Tier, defaultTimeout, defaultMaxTokens))

		for scanner.Scan() {
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				return benchmark.Suite{}, fmt.Errorf("parsing HumanEval row in %s: %w", sourcePath, err)
			}
			tests = append(tests, buildHumanevalTest(row, baseSuite.Tier, defaultTimeout, defaultMaxTokens))
		}
	} else if hasQuestion && hasAnswer {
		// GSM8K format
		var row gsm8kRow
		if err := json.Unmarshal(firstRow, &row); err != nil {
			return benchmark.Suite{}, fmt.Errorf("parsing GSM8K row in %s: %w", sourcePath, err)
		}
		tests = append(tests, buildGSM8KTest(row, baseSuite.Tier, defaultTimeout, defaultMaxTokens))

		for scanner.Scan() {
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				return benchmark.Suite{}, fmt.Errorf("parsing GSM8K row in %s: %w", sourcePath, err)
			}
			tests = append(tests, buildGSM8KTest(row, baseSuite.Tier, defaultTimeout, defaultMaxTokens))
		}
	} else {
		return benchmark.Suite{}, fmt.Errorf("unknown JSONL format in %s: fields %v", sourcePath, keys(probe))
	}

	if err := scanner.Err(); err != nil {
		return benchmark.Suite{}, fmt.Errorf("scanning JSONL %s: %w", sourcePath, err)
	}

	baseSuite.Tests = tests
	return baseSuite, nil
}

func buildHumanevalTest(row humanevalRow, tier string, timeout, maxTokens int) benchmark.SuiteTest {
	return benchmark.SuiteTest{
		ID:             row.TaskID,
		Difficulty:     tier,
		Description:    fmt.Sprintf("HumanEval problem %s", row.TaskID),
		Prompt:         row.Prompt,
		TimeoutSeconds: timeout,
		MaxTokens:      maxTokens,
		Constraints: []benchmark.Constraint{
			{
				Field:    "python_exec",
				Operator: "python_exec",
				Value: map[string]interface{}{
					"test":            row.Test,
					"entry_point":     row.EntryPoint,
					"prompt":          row.Prompt,
					"timeout_seconds": timeout,
				},
			},
		},
	}
}

func buildGSM8KTest(row gsm8kRow, tier string, timeout, maxTokens int) benchmark.SuiteTest {
	return benchmark.SuiteTest{
		ID:             row.TaskID,
		Difficulty:     tier,
		Description:    fmt.Sprintf("GSM8K problem %s", row.TaskID),
		Prompt:         row.Question,
		TimeoutSeconds: timeout,
		MaxTokens:      maxTokens,
		Constraints: []benchmark.Constraint{
			{
				Field:    "math_answer",
				Operator: "math_answer",
				Value: map[string]interface{}{
					"answer": row.Answer,
				},
			},
		},
	}
}

// keys returns the sorted keys of a map for error messages.
func keys(m map[string]interface{}) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
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

// gsm8kRow mirrors the GSM8K JSONL record.
type gsm8kRow struct {
	TaskID   string `json:"task_id"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
