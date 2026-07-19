package scorer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/EffNine/gumi/benchmark"
)

// pythonExecCheck evaluates a Python coding problem by writing the model's
// generated code plus the canonical test code to a temporary file and running
// it under a subprocess timeout.
func pythonExecCheck(response string, constraint benchmark.Constraint) CheckResult {
	params, ok := constraint.Value.(map[string]interface{})
	if !ok {
		return CheckResult{
			Passed:  false,
			Details: fmt.Sprintf("python_exec constraint value must be a map, got %T", constraint.Value),
		}
	}

	testCode := stringFromMap(params, "test")
	entryPoint := stringFromMap(params, "entry_point")
	promptCode := stringFromMap(params, "prompt")

	if testCode == "" {
		return CheckResult{Passed: false, Details: "python_exec missing test code"}
	}

	generated := extractPythonCode(response)
	if generated == "" {
		return CheckResult{Passed: false, Details: "no Python code found in response"}
	}

	// Ensure the entry point function exists. If the model only returned a body,
	// the problem prompt already contains the signature, so combining prompt +
	// generated code is usually enough. If the model returned a full function,
	// prepending the prompt would duplicate the signature, so we strip the
	// signature from the prompt when the response already defines it.
	source := buildPythonTestSource(promptCode, generated, testCode, entryPoint)

	tmpDir, err := os.MkdirTemp("", "gumi-humaneval-*")
	if err != nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("create temp dir: %v", err)}
	}
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(scriptPath, []byte(source), 0644); err != nil {
		return CheckResult{Passed: false, Details: fmt.Sprintf("write test script: %v", err)}
	}

	pyBin := pythonBinary()
	cmd := exec.Command(pyBin, scriptPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Default to 10s; allow override via constraint value.
	timeout := 10 * time.Second
	if to, ok := params["timeout_seconds"]; ok {
		if sec, ok := to.(int); ok && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err == nil {
			return CheckResult{Passed: true, Details: "python tests passed"}
		}
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = strings.TrimSpace(stdout.String())
		}
		if details == "" {
			details = err.Error()
		}
		// Truncate very long tracebacks for readability.
		details = truncate(details, 500)
		return CheckResult{Passed: false, Details: fmt.Sprintf("python tests failed: %s", details)}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return CheckResult{Passed: false, Details: fmt.Sprintf("python tests timed out after %s", timeout)}
	}
}

// extractPythonCode extracts Python code from a model response. It first looks
// for a fenced code block (```python ... ```) and returns only its content. If
// no fence is found, it returns the entire response (the model may have been
// prompted to emit raw code).
func extractPythonCode(response string) string {
	trimmed := strings.TrimSpace(response)

	// Look for a ```python or ``` fence and extract only the code inside it.
	// This handles models that wrap code in explanations.
	if strings.Contains(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		var inBlock bool
		var body []string
		for _, line := range lines {
			trimLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimLine, "```") {
				if !inBlock {
					inBlock = true
				} else {
					// Closing fence — stop collecting.
					break
				}
				continue
			}
			if inBlock {
				body = append(body, line)
			}
		}
		if len(body) > 0 {
			return strings.TrimSpace(strings.Join(body, "\n"))
		}
	}

	return trimmed
}

// buildPythonTestSource assembles the final Python script. It tries to avoid
// duplicate function signatures when the generated code already defines the
// entry point.
func buildPythonTestSource(prompt, generated, testCode, entryPoint string) string {
	var parts []string

	// Collect imports from the prompt if present.
	if prompt != "" {
		parts = append(parts, extractImports(prompt))
	}

	// If the generated code already defines the entry point, do not also emit
	// the prompt signature to avoid redefinition errors.
	if entryPoint != "" && strings.Contains(generated, "def "+entryPoint) {
		parts = append(parts, generated)
	} else if prompt != "" {
		parts = append(parts, stripImports(prompt))
		parts = append(parts, generated)
	} else {
		parts = append(parts, generated)
	}

	parts = append(parts, testCode)

	return strings.Join(parts, "\n\n")
}

// extractImports returns leading import statements from a source block.
func extractImports(src string) string {
	lines := strings.Split(src, "\n")
	var imports []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			imports = append(imports, line)
		} else if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			imports = append(imports, line)
		} else {
			break
		}
	}
	return strings.Join(imports, "\n")
}

// stripImports removes leading import/comment lines, returning the rest.
func stripImports(src string) string {
	lines := strings.Split(src, "\n")
	skip := true
	var body []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if skip && (trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ")) {
			continue
		}
		skip = false
		body = append(body, line)
	}
	return strings.Join(body, "\n")
}

// pythonBinary returns a usable Python interpreter name, preferring "python3"
// and falling back to "python".
func pythonBinary() string {
	for _, name := range []string{"python3", "python"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "python3"
}

// stringFromMap safely extracts a string value from a map[string]interface{}.
func stringFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
