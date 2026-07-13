package report

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/novexa/novexa/benchmark"
)

// WriteMarkdown generates a human-readable markdown report from the benchmark result.
// It follows the format specified in section 7.1 of the benchmark specification.
func WriteMarkdown(r *Report, path string) error {
	if r == nil {
		return fmt.Errorf("report is nil")
	}

	res := r.RunResult

	var b strings.Builder

	// Header
	b.WriteString("# Novexa Benchmark Report\n\n")
	b.WriteString(fmt.Sprintf("**Model:** %s  ·  **Provider:** %s  ·  **Tier:** %s\n",
		escMD(res.Model), escMD(res.Provider), escMD(res.ModelTier)))
	b.WriteString(fmt.Sprintf("**Run:** %s  ·  **Attempts per condition:** %d\n\n",
		escMD(res.RunID), res.Config.Attempts))

	// 1. Overall table
	b.WriteString("## Overall\n\n")
	b.WriteString("| Metric | Direct | Novexa (best) | Delta | Frontier Ceiling |\n")
	b.WriteString("|--------|--------|---------------|-------|------------------|\n")

	// Compute best novexa score across capabilities
	directOverall, novexaOverall := computeOverallDirectNovexa(res.Capabilities)
	deltaOverall := novexaOverall - directOverall

	b.WriteString(fmt.Sprintf("| **Overall Score*** | %.2f | %.2f | **%+.2f** | — |\n",
		directOverall, novexaOverall, deltaOverall))

	// Latency
	directLat, novexaLat := computeLatencies(res.PerTest)
	b.WriteString(fmt.Sprintf("| Latency (avg) | %.0fms | %.0fms | %+.0fms | — |\n",
		directLat, novexaLat, novexaLat-directLat))

	// Degradation
	degPct := res.Degradation.DegradationRate * 100
	b.WriteString(fmt.Sprintf("| Degradation Rate | — | %.1f%% | — | — |\n", degPct))

	worthItStr := "❌ No"
	if res.Summary.WorthIt {
		worthItStr = "✅ Yes"
	}
	b.WriteString(fmt.Sprintf("| **Worth it?** | | **%s** | | |\n\n", worthItStr))
	b.WriteString("*Unweighted average across all capabilities. The actual Overall Score uses adaptive weights per model tier (see Overall Score section).*\n\n")

	// 2. By Capability table
	b.WriteString("## By Capability\n\n")
	b.WriteString("| Capability | Direct | Novexa | Δ | Effect Size |\n")
	b.WriteString("|-----------|--------|--------|---|-------------|\n")

	capOrder := []string{"json", "instruction", "tool_calling", "reasoning", "repetition"}
	capNames := map[string]string{
		"json":          "JSON",
		"instruction":   "Instruction",
		"tool_calling":  "Tool-calling",
		"reasoning":     "Reasoning",
		"repetition":    "Repetition",
	}

	for _, cap := range capOrder {
		c, ok := res.Capabilities[cap]
		if !ok {
			continue
		}
		name := capNames[cap]
		if name == "" {
			name = cap
		}
		directStr := formatMetricSet(c.Direct)
		novexaStr := formatMetricSet(c.Novexa)
		stars := effectStars(c.Delta, c.EffectSize)
		effectStr := fmt.Sprintf("%.2fσ", c.EffectSize)
		b.WriteString(fmt.Sprintf("| %s | %s | %s | **%+.2f** %s | %s |\n",
			name, directStr, novexaStr, c.Delta, stars, effectStr))
	}

	b.WriteString("\n*★ = small (d≥0.2) · ★★ = medium (d≥0.5) · ★★★ = large (d≥0.8)*\n\n")

	// 3. By Difficulty bar chart
	b.WriteString("## By Difficulty\n\n")
	b.WriteString("```\n")
	difficulties := []struct {
		label  string
		target string
	}{{"Easy", "70-90%%"}, {"Medium", "40-70%%"}, {"Hard", "10-40%%"}, {"Frontier", "30-70%%"}}
	for _, d := range difficulties {
		bar := difficultyBar(res.PerTest, d.label)
		b.WriteString(fmt.Sprintf("%s (target %s): %s\n", d.label, d.target, bar))
	}
	b.WriteString("```\n\n")

	// 4. Degradation Check table
	b.WriteString("## Degradation Check\n\n")
	b.WriteString("| Severity | Count | Rate |\n")
	b.WriteString("|----------|-------|------|\n")

	var cosmeticCount, semanticCount int
	for _, c := range res.Degradation.Corruptions {
		switch c.Severity {
		case "cosmetic":
			cosmeticCount++
		case "semantic":
			semanticCount++
		}
	}
	totalDeg := len(res.Degradation.Corruptions)
	totalDegTests := res.Degradation.TotalTests

	if totalDegTests > 0 {
		b.WriteString(fmt.Sprintf("| Cosmetic | %d | %.1f%% |\n", cosmeticCount, float64(cosmeticCount)/float64(totalDegTests)*100))
		b.WriteString(fmt.Sprintf("| Semantic | %d | %.1f%% |\n", semanticCount, float64(semanticCount)/float64(totalDegTests)*100))
		b.WriteString(fmt.Sprintf("| **Total** | %d | %.1f%% |\n", totalDeg, float64(totalDeg)/float64(totalDegTests)*100))
	} else {
		b.WriteString("| Cosmetic | 0 | 0.0% |\n")
		b.WriteString("| Semantic | 0 | 0.0% |\n")
		b.WriteString("| **Total** | 0 | 0.0% |\n")
	}

	// Corruption examples (first 3)
	if len(res.Degradation.Corruptions) > 0 {
		b.WriteString("\n### Corruption Examples\n\n")
		limit := 3
		if len(res.Degradation.Corruptions) < limit {
			limit = len(res.Degradation.Corruptions)
		}
		for i := 0; i < limit; i++ {
			c := res.Degradation.Corruptions[i]
			b.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", c.TestID, c.Severity, truncateStr(c.Original, 80)))
		}
	}
	b.WriteString("\n")

	// 5. Per-Test Detail table
	b.WriteString("## Per-Test Detail\n\n")
	b.WriteString("| Test | Condition | Pass | Subscores | Latency |\n")
	b.WriteString("|------|-----------|------|-----------|---------|\n")

	// Sort by test ID then condition
	sorted := sortResults(res.PerTest)
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

		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			r.TestID, r.Condition, passMark, subscoresStr, latencyStr))
	}

	// Footer
	b.WriteString("\n---\n*Report generated by Novexa Benchmark subsystem.*\n")

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("writing markdown report: %w", err)
	}

	return nil
}

// ---- helpers ----

// computeOverallDirectNovexa computes the average direct and novexa scores across all capabilities.
func computeOverallDirectNovexa(caps map[string]benchmark.Capability) (direct, novexa float64) {
	var dSum, nSum float64
	var count int
	for _, c := range caps {
		dSum += c.Direct.Mean
		nSum += c.Novexa.Mean
		count++
	}
	if count > 0 {
		return dSum / float64(count), nSum / float64(count)
	}
	return 0, 0
}

// computeLatencies computes average latency for direct and novexa conditions.
func computeLatencies(results []benchmark.TestResult) (direct, novexa float64) {
	var dSum, nSum float64
	var dCount, nCount int
	for _, r := range results {
		if r.Condition == "direct" {
			dSum += r.LatencyMs
			dCount++
		} else if strings.HasPrefix(r.Condition, "novexa-") || r.Condition == "frontier" {
			nSum += r.LatencyMs
			nCount++
		}
	}
	if dCount > 0 {
		direct = dSum / float64(dCount)
	}
	if nCount > 0 {
		novexa = nSum / float64(nCount)
	}
	return
}

// formatMetricSet formats a MetricSet as "mean±std"
func formatMetricSet(ms benchmark.MetricSet) string {
	return fmt.Sprintf("%.2f±%.2f", ms.Mean, ms.Std)
}

// effectStars returns a star rating string based on the effect size.
func effectStars(delta, effectSize float64) string {
	// Only show stars for positive deltas (improvement)
	if delta <= 0 {
		return ""
	}
	d := math.Abs(effectSize)
	if d < 0.2 {
		return ""
	}
	if d < 0.5 {
		return "★"
	}
	if d < 0.8 {
		return "★★"
	}
	return "★★★"
}

// difficultyBar generates an ASCII bar chart for a given difficulty tier.
func difficultyBar(results []benchmark.TestResult, difficulty string) string {
	// Filter results whose test IDs contain the difficulty name (case-insensitive)
	diffLower := strings.ToLower(difficulty)
	var filtered []benchmark.TestResult
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.TestID), diffLower) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return "(no tests)"
	}

	// Compute direct vs novexa pass rates
	var directPass, directTotal int
	var novexaPass, novexaTotal int
	for _, r := range filtered {
		if r.Condition == "direct" {
			directTotal++
			if r.Passed {
				directPass++
			}
		} else if strings.HasPrefix(r.Condition, "novexa-") {
			novexaTotal++
			if r.Passed {
				novexaPass++
			}
		}
	}

	directPct := 0.0
	if directTotal > 0 {
		directPct = float64(directPass) / float64(directTotal) * 100
	}
	novexaPct := 0.0
	if novexaTotal > 0 {
		novexaPct = float64(novexaPass) / float64(novexaTotal) * 100
	}

	// Bar chart (20 chars wide)
	barWidth := 20
	directBars := int(directPct / 100.0 * float64(barWidth))
	novexaBars := int(novexaPct / 100.0 * float64(barWidth))

	directBar := strings.Repeat("█", directBars) + strings.Repeat("░", barWidth-directBars)
	novexaBar := strings.Repeat("█", novexaBars) + strings.Repeat("░", barWidth-novexaBars)

	// Estimate delta and effect for display
	delta := novexaPct - directPct
	displayDelta := fmt.Sprintf("%+.1f%%", delta)

	return fmt.Sprintf("%s  Direct %.1f%% → Novexa %.1f%% (%s)\n%s  Novexa",
		directBar, directPct, novexaPct, displayDelta, novexaBar)
}

// sortResults sorts test results by test ID then condition for deterministic output.
func sortResults(results []benchmark.TestResult) []benchmark.TestResult {
	sorted := make([]benchmark.TestResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TestID != sorted[j].TestID {
			return sorted[i].TestID < sorted[j].TestID
		}
		return sorted[i].Condition < sorted[j].Condition
	})
	return sorted
}

// escMD escapes simple markdown special characters.
func escMD(s string) string {
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "*", "\\*")
	return s
}

// truncateStr truncates a string to the given length, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
