// Package reports implements output writers for GEP benchmark results.
package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/EffNine/gumi/benchmark/gep/types"
)

// WriteJSON serializes the GEP report to a JSON file.
func WriteJSON(report *types.GEPReport, path string) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing report file: %w", err)
	}

	return nil
}

// WriteMarkdown generates a human-readable markdown report from the GEP result.
func WriteMarkdown(report *types.GEPReport, path string) error {
	if report == nil {
		return fmt.Errorf("report is nil")
	}

	var b strings.Builder

	// Header
	b.WriteString("# GEP Benchmark Report\n\n")
	b.WriteString(fmt.Sprintf("**Protocol:** GEP v%s  ·  **Schema:** v%d\n\n",
		report.ProtocolVersion, report.SchemaVersion))
	b.WriteString(fmt.Sprintf("**Model:** %s  ·  **Provider:** %s  ·  **Run:** %s\n\n",
		escMD(report.Config.Model), escMD(string(report.Config.Provider)), escMD(report.RunID)))
	b.WriteString(fmt.Sprintf("**Attempts:** %d  ·  **Timestamp:** %s\n\n",
		report.Config.Attempts, report.Config.Timestamp.Format("2006-01-02 15:04:05 UTC")))

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |")
	b.WriteString("\n|--------|-------|")
	b.WriteString(fmt.Sprintf("\n| **Overall Score** | %.2f |", report.Summary.OverallScore))
	if report.Summary.DirectScore > 0 {
		b.WriteString(fmt.Sprintf("\n| Direct Score | %.2f |", report.Summary.DirectScore))
		b.WriteString(fmt.Sprintf("\n| Gumi Score | %.2f |", report.Summary.GumiScore))
		b.WriteString(fmt.Sprintf("\n| Score Delta | %+.2f |", report.Summary.ScoreDelta))
	}
	b.WriteString(fmt.Sprintf("\n| Pass Rate | %.1f%% |", report.Summary.PassRate*100))
	if report.Summary.DirectPassRate > 0 {
		b.WriteString(fmt.Sprintf("\n| Direct Pass Rate | %.1f%% |", report.Summary.DirectPassRate*100))
		b.WriteString(fmt.Sprintf("\n| Gumi Pass Rate | %.1f%% |", report.Summary.GumiPassRate*100))
		b.WriteString(fmt.Sprintf("\n| Pass Rate Delta | %+.1fpp |", report.Summary.PassRateDelta*100))
	}
	b.WriteString(fmt.Sprintf("\n| Avg Latency | %.0fms |", report.Summary.AvgLatencyMs))
	if report.Summary.DirectLatencyMs > 0 {
		b.WriteString(fmt.Sprintf("\n| Direct Latency | %.0fms |", report.Summary.DirectLatencyMs))
		b.WriteString(fmt.Sprintf("\n| Gumi Latency | %.0fms |", report.Summary.GumiLatencyMs))
		b.WriteString(fmt.Sprintf("\n| Latency Delta | %+.0fms |", report.Summary.LatencyDeltaMs))
	}
	b.WriteString(fmt.Sprintf("\n| Total Tests | %d |", report.Summary.TotalTests))
	b.WriteString(fmt.Sprintf("\n| Passed | %d |", report.Summary.PassedTests))
	if report.Summary.WorthIt {
		b.WriteString("\n| **Worth it?** | ✅ Yes |")
	} else {
		b.WriteString("\n| **Worth it?** | ❌ No |")
	}
	b.WriteString("\n\n")

	// Capabilities with condition dimension
	b.WriteString("## Capabilities\n\n")
	b.WriteString("| Capability | Direct | Gumi | Delta | Pass Rate Delta |")
	b.WriteString("\n|-----------|--------|------|-------|-----------------|")

	capOrder := []string{"instruction_following", "structured_output", "consistency", "context_retention", "latency"}
	for _, cap := range capOrder {
		c, ok := report.Capabilities[cap]
		if !ok {
			continue
		}
		name := cap
		if name == "instruction_following" {
			name = "Instruction Following"
		} else if name == "structured_output" {
			name = "Structured Output"
		} else if name == "consistency" {
			name = "Consistency"
		} else if name == "context_retention" {
			name = "Context Retention"
		} else if name == "latency" {
			name = "Latency"
		}
		directMean := c.Direct.Mean
		gumiMean := c.Gumi.Mean
		b.WriteString(fmt.Sprintf("\n| %s | %.2f | %.2f | %+.2f | %+.2fpp |",
			name, directMean, gumiMean, c.Delta, c.PassRate*100))
	}
	b.WriteString("\n\n")

	// Per-test detail
	b.WriteString("## Per-Test Results\n\n")
	b.WriteString("| Test | Suite | Pass | Latency | Subscores |")
	b.WriteString("\n|------|-------|------|---------|-----------|")

	sorted := make([]types.GEPResult, len(report.PerTest))
	copy(sorted, report.PerTest)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SuiteID != sorted[j].SuiteID {
			return sorted[i].SuiteID < sorted[j].SuiteID
		}
		return sorted[i].TestID < sorted[j].TestID
	})

	for _, r := range sorted {
		passMark := "✅"
		if !r.Passed {
			passMark = "❌"
		}

		subscoreParts := make([]string, 0, len(r.Subscores))
		for field, score := range r.Subscores {
			subscoreParts = append(subscoreParts, fmt.Sprintf("%s:%.1f", field, score))
		}
		sort.Strings(subscoreParts)
		subscoresStr := strings.Join(subscoreParts, ", ")
		if subscoresStr == "" {
			subscoresStr = "—"
		}

		latencyStr := fmt.Sprintf("%.0fms", r.LatencyMs)
		if r.Error != "" {
			latencyStr = "err"
		}

		suiteShort := strings.ReplaceAll(r.SuiteID, "_", " ")
		b.WriteString(fmt.Sprintf("\n| %s | %s | %s | %s | %s |",
			r.TestID, suiteShort, passMark, latencyStr, subscoresStr))
	}
	b.WriteString("\n\n")

	// Footer
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("*Report generated by Gumi Evaluation Protocol (GEP) v%s*\n", report.ProtocolVersion))

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("writing markdown report: %w", err)
	}

	return nil
}

// escMD escapes simple markdown special characters.
func escMD(s string) string {
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "*", "\\*")
	return s
}
