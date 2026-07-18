package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestLoadJSONLSuite(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "test.jsonl")
	data := []byte(`{"task_id":"HumanEval/0","prompt":"def f():\n    pass\n","canonical_solution":"def f(): pass\n","test":"def test_f():\n    assert f() is None\n    \n","entry_point":"f"}` + "\n")
	if err := os.WriteFile(sourcePath, data, 0644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	base := benchmark.Suite{
		ID:             "humaneval",
		Category:       "coding",
		Tier:           "medium",
		TimeoutSeconds: 30,
		MaxTokens:      256,
	}

	suite, err := loadJSONLSuite(base, sourcePath)
	if err != nil {
		t.Fatalf("loadJSONLSuite: %v", err)
	}

	if len(suite.Tests) != 1 {
		t.Fatalf("expected 1 test, got %d", len(suite.Tests))
	}

	test := suite.Tests[0]
	if test.ID != "HumanEval/0" {
		t.Errorf("expected id HumanEval/0, got %s", test.ID)
	}
	if test.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30, got %d", test.TimeoutSeconds)
	}
	if test.MaxTokens != 256 {
		t.Errorf("expected max tokens 256, got %d", test.MaxTokens)
	}
	if len(test.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(test.Constraints))
	}
	if test.Constraints[0].Operator != "python_exec" {
		t.Errorf("expected python_exec operator, got %s", test.Constraints[0].Operator)
	}
}
