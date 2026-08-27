// Package tests provides the built-in agentic-coding verification fixtures:
// tiny repositories with objectively checkable tasks.
//
// Evaluation never depends on an LLM judge. Bug-fix fixtures inject the
// model's answer into the fixture copy and execute the real test command
// (exit status decides); navigation fixtures compare against an exact
// expected answer. Executable fixtures degrade gracefully: when the local
// toolchain (python3, rustc) is missing they are excluded from the suite and
// reported as unavailable instead of failing spuriously.
package tests

import (
	"fmt"
	"os/exec"
	"sort"
	"sync"

	"github.com/EffNine/gumi/internal/verify"
)

// Fixture describes one built-in benchmark task.
type Fixture struct {
	Name     string // unique task id, e.g. "python_bug_fix"
	Category string
	Language string // "python" | "rust" | "" (no execution)

	// Prompt renders the full task prompt (file contents included).
	Prompt func() string
	// Check validates the model output. For exec fixtures it writes the
	// extracted answer into a temp copy of the fixture and runs the tests.
	Check func(output string) error

	MaxTokens int
}

var (
	probeOnce sync.Once
	probeOK   map[string]bool // language -> toolchain available
)

func probeToolchains() {
	probeOnce.Do(func() {
		probeOK = map[string]bool{
			"python": hasBin("python3"),
			"rust":   hasBin("bash") && hasBin("rustc"),
			"":       true,
		}
	})
}

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// registry returns every fixture unconditionally (for documentation/tests).
func registry() []Fixture {
	return []Fixture{
		pythonBugFix(),
		rustRefactor(),
		repositoryNavigation(),
	}
}

// Tasks returns the executable-capable fixtures bound as verify.Tasks,
// filtered by local toolchain availability so a run never mixes different
// suites mid-flight.
func Tasks() []verify.Task {
	probeToolchains()
	out := []verify.Task{}
	for _, f := range registry() {
		if !probeOK[f.Language] {
			continue
		}
		check := f.Check
		prompt := f.Prompt
		out = append(out, verify.Task{
			ID:        f.Name,
			Category:  f.Category,
			Tier:      verify.TierCapability,
			MaxTokens: f.MaxTokens,
			PromptFn: func(int) verify.BuiltPrompt {
				return verify.BuiltPrompt{Text: prompt(), Check: check}
			},
		})
	}
	return out
}

// Unavailable lists fixture names excluded because their toolchain is
// missing locally. Surfaced in reports/profiles so gaps stay visible.
func Unavailable() []string {
	probeToolchains()
	var out []string
	for _, f := range registry() {
		if !probeOK[f.Language] {
			out = append(out, fmt.Sprintf("%s (%s toolchain not found)", f.Name, f.Language))
		}
	}
	sort.Strings(out)
	return out
}

// OptionalTaskIDs lists fixture-backed task IDs whose presence in a suite
// depends on local toolchains. Golden-suite validation treats these as
// conditionally present; everything else is mandatory.
func OptionalTaskIDs() []string {
	var out []string
	for _, f := range registry() {
		if f.Language != "" {
			out = append(out, f.Name)
		}
	}
	sort.Strings(out)
	return out
}
