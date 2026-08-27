package tests

import (
	"strings"
	"testing"
)

func TestRegistryIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range registry() {
		if seen[f.Name] {
			t.Errorf("duplicate fixture id %s", f.Name)
		}
		seen[f.Name] = true
		if f.Prompt == nil || f.Check == nil {
			t.Errorf("%s: prompt/check must be bound", f.Name)
		}
	}
}

func TestExtractCode(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "def f():\n    return 1", "def f():\n    return 1"},
		{"fenced py", "Here you go:\n```python\nprint(1)\n```", "print(1)"},
		{"fenced rust", "```rust\nfn main() {}\n```\ndone", "fn main() {}"},
		// Reasoning models echo the task (with its own fence) before
		// answering: the LAST block is the answer.
		{"echo then answer",
			"Task:\n```python\nbroken()\n```\n[Start thinking]\nfixing...\n[End thinking]\n```python\nfixed()\n```",
			"fixed()"},
		{"padded", "  \n x = 1 \n ", "x = 1"},
	}
	for _, tc := range cases {
		if got := ExtractCode(tc.in); got != tc.want {
			t.Errorf("%s: ExtractCode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNavigationCheck(t *testing.T) {
	f := repositoryNavigation()
	for _, good := range []string{"config.py", "The file is config.py.", "./config.py"} {
		if err := f.Check(good); err != nil {
			t.Errorf("Check(%q) rejected valid answer: %v", good, err)
		}
	}
	for _, bad := range []string{"agent/loop.go", "MAX_RETRIES = 3", ""} {
		if err := f.Check(bad); err == nil {
			t.Errorf("Check(%q) accepted wrong answer", bad)
		}
	}
}

func TestPromptContainsFixtureSources(t *testing.T) {
	for _, f := range registry() {
		p := f.Prompt()
		switch f.Name {
		case "python_bug_fix":
			if !strings.Contains(p, "def clamp") || !strings.Contains(p, "calculator.py") {
				t.Error("python prompt missing source or filename")
			}
		case "rust_refactor":
			if !strings.Contains(p, "fn is_even") {
				t.Error("rust prompt missing source")
			}
		case "repository_navigation":
			if !strings.Contains(p, "MAX_RETRIES") || !strings.Contains(p, "agent/tools/search.py") {
				t.Error("navigation prompt missing repo content")
			}
		}
	}
}

// TestPythonBugFixEvaluation executes the real fixture end-to-end when
// python3 is available: the original file must fail, a corrected file must
// pass.
func TestPythonBugFixEvaluation(t *testing.T) {
	if !hasBin("python3") {
		t.Skip("python3 not available")
	}
	files := map[string]string{
		"calculator.py":      pyBuggy,
		"test_calculator.py": pyTest,
	}
	if err := evaluate(files, "calculator.py", pyBuggy, []string{"python3", "test_calculator.py"}); err == nil {
		t.Error("buggy file must fail the fixture tests")
	}
	fixed := strings.Replace(pyBuggy, "return min(low, max(value, high))",
		"return max(low, min(value, high))", 1)
	if fixed == pyBuggy {
		t.Fatal("correction did not apply")
	}
	if err := evaluate(files, "calculator.py", fixed, []string{"python3", "test_calculator.py"}); err != nil {
		t.Errorf("fixed file must pass: %v", err)
	}
}

func TestRustRefactorEvaluation(t *testing.T) {
	if !hasBin("bash") || !hasBin("rustc") {
		t.Skip("rust toolchain not available")
	}
	fix := rustRefactor()
	files := map[string]string{"main.rs": rsBuggy}
	if fix.Check == nil {
		t.Fatal("rust check unbound")
	}

	if err := evaluate(files, "main.rs", rsBuggy,
		[]string{"bash", "-c", "rustc --edition 2021 --test main.rs -o gumi_fixture_test && ./gumi_fixture_test"}); err == nil {
		t.Error("buggy rust file must fail")
	}
	fixed := strings.Replace(rsBuggy, "n % 2 == 1", "n % 2 == 0", 1)
	if err := evaluate(files, "main.rs", fixed,
		[]string{"bash", "-c", "rustc --edition 2021 --test main.rs -o gumi_fixture_test && ./gumi_fixture_test"}); err != nil {
		t.Errorf("fixed rust file must pass: %v", err)
	}
}
