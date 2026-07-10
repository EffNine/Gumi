// Package repair safely fixes common validation failures.
package repair

import (
	"encoding/json"
	"strings"

	"github.com/novexa/novexa/runtime/internal/api"
	"github.com/novexa/novexa/runtime/internal/validation"
)

// Report describes a repair attempt.
type Report struct {
	Attempted       bool     `json:"attempted"`
	Strategy        string   `json:"strategy,omitempty"`
	Success         bool     `json:"success"`
	Changes         []string `json:"changes,omitempty"`
	RemainingIssues []string `json:"remaining_issues,omitempty"`
	RetryRequested  bool     `json:"retry_requested"`
}

// Engine repairs invalid model output without inventing facts.
type Engine struct{}

// New creates a Repair Engine.
func New() *Engine {
	return &Engine{}
}

// Repair applies deterministic local repairs when safe.
func (e *Engine) Repair(resp *api.ChatCompletionResponse, validationReport validation.Report) Report {
	report := Report{
		Attempted: validationReport.Repairable,
		Strategy:  string(validationReport.SuggestedRepairStrategy),
	}
	if !validationReport.Repairable {
		return report
	}

	switch validationReport.SuggestedRepairStrategy {
	case validation.StrategyLocalParseRepair:
		return repairJSON(resp, report)
	case validation.StrategyRegexCleanup:
		return repairRepetition(resp, report)
	case validation.StrategyRetryGeneration:
		report.RetryRequested = true
		return report
	default:
		return report
	}
}

func repairJSON(resp *api.ChatCompletionResponse, report Report) Report {
	content, _ := validation.AssistantContent(resp)
	candidate := validation.ExtractJSONCandidate(content)
	if candidate == "" {
		report.RetryRequested = true
		return report
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(candidate), &decoded); err != nil {
		report.RetryRequested = true
		report.RemainingIssues = append(report.RemainingIssues, string(validation.IssueInvalidJSON))
		return report
	}
	clean, err := json.Marshal(decoded)
	if err != nil {
		report.RetryRequested = true
		return report
	}
	validation.SetAssistantContent(resp, string(clean))
	report.Success = true
	report.Changes = append(report.Changes, "extracted_valid_json")
	return report
}

func repairRepetition(resp *api.ChatCompletionResponse, report Report) Report {
	content, _ := validation.AssistantContent(resp)
	lines := strings.Split(content, "\n")
	counts := map[string]int{}
	var cleaned []string
	changed := false
	for _, line := range lines {
		key := strings.TrimSpace(line)
		if key == "" {
			cleaned = append(cleaned, line)
			continue
		}
		counts[key]++
		if counts[key] > 2 {
			changed = true
			continue
		}
		cleaned = append(cleaned, line)
	}
	if changed {
		validation.SetAssistantContent(resp, strings.TrimSpace(strings.Join(cleaned, "\n")))
		report.Success = true
		report.Changes = append(report.Changes, "removed_repeated_lines")
		return report
	}
	report.RetryRequested = true
	return report
}
