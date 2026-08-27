package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// evalTimeout bounds one fixture test execution.
const evalTimeout = 60 * time.Second

// maxTestOutput keeps failure diagnostics bounded.
const maxTestOutput = 600

// Evaluate materializes the fixture into a fresh temp directory, injects the
// model's answer as the target file, and runs the fixture's test command.
// A zero exit status means pass; anything else fails with the test output
// tail as objective evidence.
func evaluate(files map[string]string, injectFile, answer string, cmd []string) error {
	dir, err := os.MkdirTemp("", "gumi-fixture-")
	if err != nil {
		return fmt.Errorf("fixture temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(files[name]), 0o644); err != nil {
			return err
		}
	}
	if injectFile != "" {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(injectFile)),
			[]byte(answer), 0o644); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), evalTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	c.Dir = dir
	outBytes, err := c.CombinedOutput()
	tail := tailOf(string(outBytes), maxTestOutput)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("fixture tests timed out after %s\n%s", evalTimeout, tail)
		}
		return fmt.Errorf("fixture tests failed (exit status)\n%s", tail)
	}
	return nil
}

// ExtractCode pulls the code block out of a model answer. Reasoning-style
// models often repeat the task (including the original fenced snippet)
// before answering, so the LAST complete fenced block is the answer; raw
// trimmed output is the fallback when no fences exist.
func ExtractCode(output string) string {
	s := output
	close := strings.LastIndex(s, "```")
	if close >= 0 {
		open := strings.LastIndex(s[:close], "```")
		if open >= 0 {
			rest := s[open+3 : close]
			// Skip the language tag on the opening fence line.
			if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
				return strings.TrimSpace(rest[nl+1:])
			}
			return strings.TrimSpace(rest)
		}
	}
	return strings.TrimSpace(s)
}

func tailOf(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
