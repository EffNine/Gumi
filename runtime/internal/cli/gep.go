package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/EffNine/gumi/benchmark/gep/baselines"
	"github.com/EffNine/gumi/benchmark/gep/runner"
	"github.com/EffNine/gumi/benchmark/gep/types"
)

func runGEP(args []string) {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintln(os.Stderr, "usage: gumi gep run [flags]")
		printGEPRunUsage()
		os.Exit(1)
	}
	runGEPRun(args[1:])
}

func printGEPRunUsage() {
	fmt.Println()
	fmt.Println("GEP Benchmark Runner")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gumi gep run --model <model> --provider <provider> [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --model string         Model name (e.g., qwen3:8b, gemma3:4b)")
	fmt.Println("  --provider string      Provider type: ollama, lmstudio")
	fmt.Println("  --provider-url string  Direct provider API URL (e.g., http://localhost:11434)")
	fmt.Println("  --gumi-url string      Gumi runtime URL (e.g., http://127.0.0.1:8787)")
	fmt.Println("  --gumi-api-key string  Gumi runtime API key")
	fmt.Println("  --attempts int         Number of attempts per test (default: 3)")
	fmt.Println("  --suite string         Suite ID to run (e.g., instruction-following)")
	fmt.Println("  --difficulty string    Difficulty tier: easy, medium, hard (default: easy)")
	fmt.Println("  --conditions string    Comma-separated conditions: direct,gumi-stabilized (default: both)")
	fmt.Println("  --output string        Output directory for reports (default: ~/.gumi/gep/reports)")
	fmt.Println("  --scope string         Baseline scope: model, runtime (default: runtime)")
	fmt.Println("  --json                 Machine-readable JSON output")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434")
	fmt.Println("  gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 --gumi-url http://127.0.0.1:8787")
}

func runGEPRun(args []string) {
	fs := flag.NewFlagSet("gep run", flag.ContinueOnError)
	model := fs.String("model", "", "Model name (required)")
	provider := fs.String("provider", "", "Provider type: ollama, lmstudio (required)")
	providerURL := fs.String("provider-url", "", "Direct provider API URL (required)")
	gumiURL := fs.String("gumi-url", "", "Gumi runtime URL for gumi-stabilized condition")
	gumiAPIKey := fs.String("gumi-api-key", "", "Gumi runtime API key")
	attempts := fs.Int("attempts", 3, "Number of attempts per test")
	suite := fs.String("suite", "", "Suite ID to run")
	difficulty := fs.String("difficulty", "easy", "Difficulty tier: easy, medium, hard")
	conditions := fs.String("conditions", "direct,gumi-stabilized", "Comma-separated conditions")
	outputDir := fs.String("output", "", "Output directory (default: ~/.gumi/gep/reports)")
	scope := fs.String("scope", "runtime", "Baseline scope: model, runtime")
	jsonOutput := fs.Bool("json", false, "Machine-readable JSON output")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *model == "" {
		fmt.Fprintln(os.Stderr, "error: --model is required")
		printGEPRunUsage()
		os.Exit(1)
	}
	if *provider == "" {
		fmt.Fprintln(os.Stderr, "error: --provider is required")
		printGEPRunUsage()
		os.Exit(1)
	}
	if *providerURL == "" {
		fmt.Fprintln(os.Stderr, "error: --provider-url is required")
		printGEPRunUsage()
		os.Exit(1)
	}

	if *outputDir == "" {
		home, _ := os.UserHomeDir()
		*outputDir = filepath.Join(home, ".gumi", "gep", "reports")
	}

	// Parse conditions
	var condList []types.GEPCondition
	for _, c := range splitAndTrim(*conditions, ",") {
		switch c {
		case "direct":
			condList = append(condList, types.ConditionDirect)
		case "gumi-stabilized":
			condList = append(condList, types.ConditionGumiStabilized)
		}
	}
	if len(condList) == 0 {
		condList = []types.GEPCondition{types.ConditionDirect, types.ConditionGumiStabilized}
	}

	// Parse scope
	var runScope types.GEPScope
	switch *scope {
	case "model":
		runScope = types.ScopeModel
	case "runtime":
		runScope = types.ScopeRuntime
	default:
		runScope = types.ScopeRuntime
	}

	cfg := runner.RunConfig{
		Model:       *model,
		Provider:    types.ProviderType(*provider),
		ProviderURL: *providerURL,
		Attempts:    *attempts,
		SuiteID:     *suite,
		Difficulty:  types.DifficultyTier(*difficulty),
		Conditions:  condList,
		GumiURL:     *gumiURL,
		GumiAPIKey:  *gumiAPIKey,
		OutputDir:   *outputDir,
		Scope:       runScope,
	}

	fmt.Printf("Running GEP v%s benchmark\n", types.ProtocolVersion)
	fmt.Printf("Model: %s | Provider: %s | Conditions: %v | Scope: %s\n",
		cfg.Model, cfg.Provider, cfg.Conditions, cfg.Scope)
	fmt.Printf("Output: %s\n\n", cfg.OutputDir)

	report, err := runner.Run(cfg)
	if err != nil {
		log.Fatalf("GEP benchmark failed: %v", err)
	}

	// Save baseline
	home, _ := os.UserHomeDir()
	baselineDir := filepath.Join(home, ".gumi", "gep", "baselines")
	store := baselines.NewStore(baselineDir)
	if err := store.Save(report); err != nil {
		log.Printf("Warning: failed to save baseline: %v", err)
	}

	if *jsonOutput {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Println("\n=== Results ===")
	fmt.Printf("Overall Score: %.2f\n", report.Summary.OverallScore)
	fmt.Printf("Pass Rate: %.1f%%\n", report.Summary.PassRate*100)
	fmt.Printf("Total Tests: %d (Passed: %d)\n", report.Summary.TotalTests, report.Summary.PassedTests)
	fmt.Printf("Run ID: %s\n", report.RunID)
	fmt.Printf("Timestamp: %s\n", report.Config.Timestamp.Format(time.RFC3339))

	if report.Summary.DirectScore > 0 {
		fmt.Printf("\n--- Condition Breakdown ---\n")
		fmt.Printf("Direct Score: %.2f | Gumi Score: %.2f | Delta: %+.2f\n",
			report.Summary.DirectScore, report.Summary.GumiScore, report.Summary.ScoreDelta)
		fmt.Printf("Direct Pass Rate: %.1f%% | Gumi Pass Rate: %.1f%% | Delta: %+.1fpp\n",
			report.Summary.DirectPassRate*100, report.Summary.GumiPassRate*100, report.Summary.PassRateDelta*100)
		fmt.Printf("Direct Latency: %.0fms | Gumi Latency: %.0fms | Delta: %+.0fms\n",
			report.Summary.DirectLatencyMs, report.Summary.GumiLatencyMs, report.Summary.LatencyDeltaMs)
	}

	fmt.Println("\n--- Capabilities ---")
	for suiteType, cap := range report.Capabilities {
		fmt.Printf("  %s: direct=%.2f gumi=%.2f delta=%+.2f pass_rate_delta=%+.0fpp\n",
			suiteType, cap.Direct.Mean, cap.Gumi.Mean, cap.Delta, cap.PassRate*100)
	}

	fmt.Printf("\nWorth it: %v\n", report.Summary.WorthIt)
	fmt.Printf("Report saved to: %s\n", cfg.OutputDir)
}

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range splitString(s, sep) {
		part = trimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	left := 0
	right := len(s)
	for left < right && (s[left] == ' ' || s[left] == '\t' || s[left] == '\n' || s[left] == '\r') {
		left++
	}
	for right > left && (s[right-1] == ' ' || s[right-1] == '\t' || s[right-1] == '\n' || s[right-1] == '\r') {
		right--
	}
	return s[left:right]
}
