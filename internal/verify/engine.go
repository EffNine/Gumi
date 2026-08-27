package verify

import (
	"context"
	"fmt"
	"time"

	"github.com/EffNine/gumi/internal/backend"
)

// Engine runs verification suites against a backend runner.
//
// The engine enforces the core product rule: a candidate is only as good as
// its measured capability. All runs use fixed seeds and temperature 0 so the
// same model + prompts + seeds produce paired, comparable results.
type Engine struct {
	runner       backend.Runner
	modelPath    string
	perTaskLimit int
}

// NewEngine creates an engine bound to one model and backend.
func NewEngine(runner backend.Runner, modelPath string) *Engine {
	return &Engine{runner: runner, modelPath: modelPath, perTaskLimit: 512}
}

// Runner exposes the underlying backend.
func (e *Engine) Runner() backend.Runner { return e.runner }

// MeasurePerf performs the performance probe: one run with a filler prompt of
// approximately promptTokens and generation of genTokens.
func (e *Engine) MeasurePerf(ctx context.Context, cfg backend.Config, promptTokens, genTokens int) (*backend.Metrics, error) {
	spec := backend.RunSpec{
		ModelPath: e.modelPath,
		Config:    cfg,
		Prompt:    FillerPrompt(promptTokens),
		MaxTokens: genTokens,
		Purpose:   "perf",
	}
	res, err := e.runner.Run(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("perf run failed: %w", err)
	}
	m := res.Metrics
	if m.DecodeTPS <= 0 || m.PrefillTPS <= 0 {
		return nil, fmt.Errorf("perf run produced no usable timing data")
	}
	return &m, nil
}

// TaskOutcome records one task result.
type TaskOutcome struct {
	TaskID   string `json:"task_id"`
	Category string `json:"category"`
	Tier     string `json:"tier"`
	Passed   bool   `json:"passed"`
	Error    string `json:"error,omitempty"`
	Output   string `json:"output_snippet,omitempty"`
}

// SuiteResult aggregates outcomes for one tier.
type SuiteResult struct {
	Tier     Tier          `json:"-"`
	TierName string        `json:"tier"`
	Outcomes []TaskOutcome `json:"outcomes"`
	Passed   int           `json:"passed"`
	Total    int           `json:"total"`
	Rate     float64       `json:"rate"`
}

// RunSuite executes tasks sequentially against one configuration.
func (e *Engine) RunSuite(ctx context.Context, cfg backend.Config, tasks []Task) (*SuiteResult, error) {
	res := &SuiteResult{Tier: TierCapability}
	if len(tasks) > 0 {
		res.Tier = tasks[0].Tier
		res.TierName = tasks[0].Tier.String()
	}
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		outcome := e.runTask(ctx, cfg, t)
		res.Outcomes = append(res.Outcomes, outcome)
		res.Total++
		if outcome.Passed {
			res.Passed++
		}
	}
	if res.Total > 0 {
		res.Rate = float64(res.Passed) / float64(res.Total)
	}
	return res, nil
}

func (e *Engine) runTask(ctx context.Context, cfg backend.Config, t Task) TaskOutcome {
	built := t.Build(cfg.ContextTokens)
	maxTok := t.MaxTokens
	if maxTok <= 0 {
		maxTok = e.perTaskLimit
	}
	spec := backend.RunSpec{
		ModelPath: e.modelPath,
		Config:    cfg,
		Prompt:    built.Text,
		MaxTokens: maxTok,
		Purpose:   "task:" + t.ID,
	}
	started := time.Now()
	res, err := e.runner.Run(ctx, spec)
	outcome := TaskOutcome{
		TaskID:   t.ID,
		Category: t.Category,
		Tier:     t.Tier.String(),
	}
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}
	_ = started
	check := built.Check
	if check == nil {
		check = nonEmptyCheck
	}
	if err := check(res.Output); err != nil {
		outcome.Error = err.Error()
		outcome.Output = TruncateSnip(res.Output)
		return outcome
	}
	outcome.Passed = true
	outcome.Output = TruncateSnip(res.Output)
	return outcome
}

func nonEmptyCheck(out string) error {
	if len(Normalize(out)) == 0 {
		return fmt.Errorf("empty output")
	}
	return nil
}

// Gate compares a candidate against the reference using paired results.
//
// Rules:
//   - smoke tier must pass fully for candidates;
//   - capability rate must not fall below reference minus slack (default 0).
//
// A faster configuration that fails the gate is rejected — always.
func Gate(ref, cand *SuiteResult, slack float64) (bool, string) {
	if cand == nil {
		return false, "no candidate result"
	}
	if ref == nil {
		// No reference measured (tier=smoke mode): require perfect smoke.
		if cand.Rate < 1.0 {
			return false, fmt.Sprintf("smoke gate failed (%d/%d passed)", cand.Passed, cand.Total)
		}
		return true, "smoke gate passed"
	}
	if cand.Rate < ref.Rate-slack {
		return false, fmt.Sprintf(
			"capability regression: candidate %.0f%% < reference %.0f%% (slack %.2f)",
			cand.Rate*100, ref.Rate*100, slack)
	}
	return true, fmt.Sprintf("capability preserved (%.0f%% vs reference %.0f%%)", cand.Rate*100, ref.Rate*100)
}

// DeepEval is explicitly out of MVP scope.
func DeepEval() error {
	return fmt.Errorf("deep evaluation (Tier 3) is not implemented in the MVP")
}
