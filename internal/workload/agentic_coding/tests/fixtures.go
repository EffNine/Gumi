package tests

import (
	"fmt"
	"strings"

	"github.com/EffNine/gumi/internal/verify"
)

// ---- python_bug_fix ----------------------------------------------------
//
// One focused bug (swapped min/max in clamp) makes the fixture test fail.
// The model must return the corrected file; evaluation runs the real test
// script and requires exit status 0. No LLM judge anywhere.

const pyBuggy = `"""Range utilities used by the config loader."""


def clamp(value, low, high):
    """Clamp value into the inclusive range [low, high]."""
    return min(low, max(value, high))


def percent(part, whole):
    """Return part/whole as a percentage number (e.g. 25 for 0.25)."""
    return part / whole * 100


def slugify(text):
    """Lowercase text and replace spaces with dashes."""
    return text.strip().lower().replace(" ", "-")
`

const pyTest = `import sys

from calculator import clamp, percent, slugify


def main():
    checks = [
        ("clamp middle", clamp(5, 0, 10) == 5),
        ("clamp below", clamp(-3, 0, 10) == 0),
        ("clamp above", clamp(42, 0, 10) == 10),
        ("percent", percent(1, 4) == 25),
        ("slugify", slugify(" Hello World ") == "hello-world"),
    ]
    failed = [name for name, ok in checks if not ok]
    if failed:
        print("FAILED: " + ", ".join(failed))
        return 1
    print("ALL TESTS PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
`

func pythonBugFix() Fixture {
	prompt := func() string {
		return "The Python module below contains a bug; its test suite fails.\n\n" +
			"```python\n" + pyBuggy + "```\n\n" +
			"Failing test output:\n```\nFAILED: clamp below, clamp above\n```\n\n" +
			"Output ONLY the complete corrected calculator.py file content. No explanations."
	}
	return Fixture{
		Name:      "python_bug_fix",
		Category:  "coding_exec",
		Language:  "python",
		Prompt:    prompt,
		Check:     codeInjectCheck(map[string]string{"calculator.py": pyBuggy, "test_calculator.py": pyTest}, "calculator.py", []string{"python3", "test_calculator.py"}),
		MaxTokens: 4096,
	}
}

// ---- rust_refactor -------------------------------------------------------
//
// Single-file Rust program with one buggy predicate; the model returns the
// corrected file, which is compiled with rustc --test and executed.

const rsBuggy = `//! Token budget helpers for a tiny agent loop.

pub fn is_even(n: i64) -> bool {
    n % 2 == 1
}

pub fn budget_left(total: i64, used: i64) -> i64 {
    total - used
}

pub fn truncate_chars(s: &str, max: usize) -> String {
    s.chars().take(max).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn even_numbers() {
        assert!(is_even(4));
        assert!(!is_even(7));
        assert!(is_even(0));
    }

    #[test]
    fn budget_arithmetic() {
        assert_eq!(budget_left(100, 40), 60);
        assert_eq!(budget_left(10, 10), 0);
    }

    #[test]
    fn truncation_respects_char_boundary() {
        assert_eq!(truncate_chars("gumi", 2), "gu");
    }
}
`

func rustRefactor() Fixture {
	prompt := func() string {
		return "The Rust file below has a failing unit test.\n\n" +
			"```rust\n" + rsBuggy + "```\n\n" +
			"Failing test output:\n```\ntest tests::even_numbers ... FAILED\nassertion failed: !is_even(7)\n```\n\n" +
			"Output ONLY the complete corrected main.rs file content. No explanations."
	}
	return Fixture{
		Name:     "rust_refactor",
		Category: "coding_exec",
		Language: "rust",
		Prompt:   prompt,
		Check: codeInjectCheck(
			map[string]string{"main.rs": rsBuggy},
			"main.rs",
			[]string{"bash", "-c", "rustc --edition 2021 --test main.rs -o gumi_fixture_test && ./gumi_fixture_test"},
		),
		MaxTokens: 4096,
	}
}

// ---- repository_navigation ------------------------------------------------
//
// A multi-file mini repository: the model must locate where a constant is
// defined by reading the provided sources. Exact-match validation.

type navFile struct{ path, body string }

var navFiles = []navFile{
	{"README.md", "# retry-agent\n\nA minimal agent skeleton. Configuration lives in config.py.\n"},
	{"config.py", "# Central runtime configuration.\n\nMAX_RETRIES = 3\nTIMEOUT_SECONDS = 30\nLOG_LEVEL = \"info\"\n"},
	{"agent/loop.go", "// Package agent runs the tool loop.\n\npackage agent\n\nimport \"fmt\"\n\n// Run executes up to MaxSteps iterations.\nfunc Run(maxSteps int) error {\n\tfor i := 0; i < maxSteps; i++ {\n\t\tif err := step(i); err != nil {\n\t\t\treturn fmt.Errorf(\"step %d: %w\", i, err)\n\t\t}\n\t}\n\treturn nil\n}\n"},
	{"agent/tools/search.py", "\"\"\"Web search tool.\"\"\"\n\nfrom config import TIMEOUT_SECONDS\n\n\ndef search(query):\n    # uses TIMEOUT_SECONDS for the HTTP timeout\n    ...\n"},
	{"agent/tools/shell.py", "\"\"\"Shell command tool with bounded retries.\"\"\"\n\n# NOTE: retry policy intentionally read from the central configuration.\n\n\ndef run(cmd):\n    ...\n"},
}

func repositoryNavigation() Fixture {
	prompt := func() string {
		var b strings.Builder
		b.WriteString("You are given a small repository. Identify where MAX_RETRIES is defined.\n\nRepository files:\n")
		for _, f := range navFiles {
			fmt.Fprintf(&b, "--- %s ---\n%s\n", f.path, f.body)
		}
		b.WriteString("\nWhich file defines MAX_RETRIES? Reply with only its path relative to the repository root.")
		return b.String()
	}
	return Fixture{
		Name:     "repository_navigation",
		Category: "repository_reasoning",
		Language: "",
		Prompt:   prompt,
		Check: func(output string) error {
			// MAX_RETRIES lives in ./config.py at the fixture root; accept
			// the bare name or any path ending in it.
			got := strings.TrimRight(verify.Normalize(output), " .")
			if got == "config.py" || strings.HasSuffix(got, "config.py") {
				return nil
			}
			return fmt.Errorf("expected path config.py, got %q", verify.TruncateSnip(output))
		},
		MaxTokens: 1024,
	}
}

// ---- shared check builders ---------------------------------------------

// codeInjectCheck extracts the corrected file from the model output, injects
// it into a temp copy of the fixture and executes the real test command.
func codeInjectCheck(files map[string]string, target string, cmd []string) func(string) error {
	return func(output string) error {
		code := ExtractCode(output)
		if code == "" {
			return fmt.Errorf("empty answer")
		}
		return evaluate(files, target, code, cmd)
	}
}
