// Package report implements output writers for benchmark results.
package report

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/EffNine/gumi/benchmark"
)

// Report is the full output of a benchmark run, ready for serialization.
type Report struct {
	RunResult benchmark.RunResult `json:"run_result"`
}

// WriteJSON serializes the report to a JSON file at the given path.
func WriteJSON(report *Report, path string) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing report file: %w", err)
	}

	return nil
}
