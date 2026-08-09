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

func TestBuildPythonTestSource_NoPrompt(t *testing.T) {
	generated := "def add(a, b): return a + b"
	test := "assert add(1, 2) == 3"
	src := buildPythonTestSource("", generated, test, "add")
	if !strings.Contains(src, "def add") {
		t.Errorf("expected generated code preserved, source:\n%s", src)
	}
	if !strings.Contains(src, "assert add") {
		t.Errorf("expected test code appended, source:\n%s", src)
	}
}

func TestBuildPythonTestSource_NoEntryPoint(t *testing.T) {
	prompt := "def helper(): pass\n"
	generated := "def main(): pass"
	test := "assert True"
	src := buildPythonTestSource(prompt, generated, test, "")
	if !strings.Contains(src, "def helper") {
		t.Errorf("expected prompt code included when no entry point, source:\n%s", src)
	}
}

func TestPythonBinary(t *testing.T) {
	bin := pythonBinary()
	if bin != "python3" && bin != "python" {
		t.Errorf("pythonBinary() = %q, want python3 or python", bin)
	}
}

func TestStringFromMap_MissingKey(t *testing.T) {
	m := map[string]interface{}{"a": "1"}
	got := stringFromMap(m, "b")
	if got != "" {
		t.Errorf("stringFromMap(missing key) = %q, want empty", got)
	}
}

func TestStringFromMap_NonStringValue(t *testing.T) {
	m := map[string]interface{}{"count": 42}
	got := stringFromMap(m, "count")
	if got != "42" {
		t.Errorf("stringFromMap(int value) = %q, want '42'", got)
	}
}

func TestStripImports(t *testing.T) {
	prompt := "from typing import List\n\n# comment\n\ndef has_close_elements(numbers: List[float], threshold: float) -> bool:\n    pass\n"
	src := stripImports(prompt)
	if strings.Contains(src, "from typing import List") {
		t.Errorf("expected imports stripped, got:\n%s", src)
	}
	if !strings.Contains(src, "def has_close_elements") {
		t.Errorf("expected function body preserved, got:\n%s", src)
	}
}
