package scorer

import (
	"strings"
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

func TestPythonExecCheck_Pass(t *testing.T) {
	resp := `def add(a, b):
    return a + b`
	c := benchmark.Constraint{
		Field:    "python_exec",
		Operator: "python_exec",
		Value: map[string]interface{}{
			"test":        "assert add(1, 2) == 3\nassert add(-1, 1) == 0",
			"entry_point": "add",
		},
	}
	got := pythonExecCheck(resp, c)
	if !got.Passed {
		t.Fatalf("expected pass, got: %s", got.Details)
	}
}

func TestPythonExecCheck_MarkdownFences(t *testing.T) {
	resp := "```python\ndef add(a, b):\n    return a + b\n```"
	c := benchmark.Constraint{
		Field:    "python_exec",
		Operator: "python_exec",
		Value: map[string]interface{}{
			"test":        "assert add(1, 2) == 3",
			"entry_point": "add",
		},
	}
	got := pythonExecCheck(resp, c)
	if !got.Passed {
		t.Fatalf("expected pass after stripping fences, got: %s", got.Details)
	}
}

func TestPythonExecCheck_MissingFunction(t *testing.T) {
	resp := "def subtract(a, b): return a - b"
	c := benchmark.Constraint{
		Field:    "python_exec",
		Operator: "python_exec",
		Value: map[string]interface{}{
			"test":        "assert add(1, 2) == 3",
			"entry_point": "add",
		},
	}
	got := pythonExecCheck(resp, c)
	if got.Passed {
		t.Fatal("expected failure when entry point is missing")
	}
}

func TestPythonExecCheck_SyntaxError(t *testing.T) {
	resp := "def add(a, b): return a +"
	c := benchmark.Constraint{
		Field:    "python_exec",
		Operator: "python_exec",
		Value: map[string]interface{}{
			"test":        "assert add(1, 2) == 3",
			"entry_point": "add",
		},
	}
	got := pythonExecCheck(resp, c)
	if got.Passed {
		t.Fatal("expected failure on syntax error")
	}
	if !strings.Contains(got.Details, "failed") {
		t.Fatalf("expected failure details, got: %s", got.Details)
	}
}

func TestPythonExecCheck_Timeout(t *testing.T) {
	resp := `def loop():
    while True:
        pass`
	c := benchmark.Constraint{
		Field:    "python_exec",
		Operator: "python_exec",
		Value: map[string]interface{}{
			"test":            "loop()",
			"entry_point":     "loop",
			"timeout_seconds": 1,
		},
	}
	got := pythonExecCheck(resp, c)
	if got.Passed {
		t.Fatal("expected timeout failure")
	}
	if !strings.Contains(got.Details, "timed out") {
		t.Fatalf("expected timeout details, got: %s", got.Details)
	}
}

func TestExtractPythonCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain",
			in:   "  def f(): pass  ",
			want: "def f(): pass",
		},
		{
			name: "fenced python",
			in:   "```python\ndef f(): pass\n```",
			want: "def f(): pass",
		},
		{
			name: "fenced generic",
			in:   "```\ndef f(): pass\n```",
			want: "def f(): pass",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPythonCode(tc.in)
			if got != tc.want {
				t.Fatalf("extractPythonCode(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildPythonTestSource_NoDuplicateSignature(t *testing.T) {
	prompt := "from typing import List\n\ndef has_close_elements(numbers: List[float], threshold: float) -> bool:\n    pass\n"
	generated := "def has_close_elements(numbers, threshold):\n    return any(abs(numbers[i] - numbers[j]) < threshold for i in range(len(numbers)) for j in range(i+1, len(numbers)))"
	test := "assert has_close_elements([1.0, 2.0, 3.0], 0.5) == False"
	src := buildPythonTestSource(prompt, generated, test, "has_close_elements")
	if strings.Count(src, "def has_close_elements") != 1 {
		t.Fatalf("expected exactly one signature, source:\n%s", src)
	}
	if !strings.Contains(src, "from typing import List") {
		t.Fatalf("expected imports preserved, source:\n%s", src)
	}
}
