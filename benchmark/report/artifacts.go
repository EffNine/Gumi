package report

import (
	"fmt"
	"os"
	"path/filepath"
)

// StoreArtifact writes a raw model output to the artifact storage directory.
// Artifacts are stored at ~/.gumi/benchmarks/<run-id>/artifacts/<test-id>-<condition>-<attempt>.txt.
func StoreArtifact(runID string, testID string, condition string, attempt int, output string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".gumi", "benchmarks", runID, "artifacts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating artifact directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s-%d.txt", testID, condition, attempt)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing artifact file: %w", err)
	}

	return nil
}
