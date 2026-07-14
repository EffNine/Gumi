// Command run is a standalone binary for testing the benchmark subsystem.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/EffNine/gumi/benchmark/runner"
)

func main() {
	model := flag.String("model", "", "Model name (e.g., \"ornith-1.0-9b@q4_k_m\")")
	mode := flag.String("mode", "auto", "Execution mode: auto | quick | thorough | frontier")
	attempts := flag.Int("attempts", 3, "Attempts per condition")
	provider := flag.String("provider", "", "Provider (auto-detect, or lmstudio/ollama/anthropic/openai)")
	conditions := flag.String("conditions", "direct,gumi-stabilized", "Comma-separated conditions to test")
	frontierKey := flag.String("frontier-key", "", "API key for frontier baseline run")
	frontierModel := flag.String("frontier-model", "", "Frontier model name")
	outputDir := flag.String("output", "benchmarks/reports/", "Output directory for reports")
	jsonOutput := flag.Bool("json", false, "Machine-readable JSON output to stdout")
	apiKey := flag.String("api-key", "", "API key for the provider")
	baseURL := flag.String("base-url", "http://127.0.0.1:8787", "Runtime API base URL")

	flag.Parse()

	if *model == "" {
		fmt.Fprintln(os.Stderr, "error: --model is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg := runner.Config{
		Model:         *model,
		Provider:      *provider,
		Mode:          *mode,
		Attempts:      *attempts,
		Conditions:    parseConditions(*conditions),
		FrontierKey:   *frontierKey,
		FrontierModel: *frontierModel,
		OutputDir:     *outputDir,
		JSONOutput:    *jsonOutput,
		APIKey:        *apiKey,
		BaseURL:       *baseURL,
	}

	log.Printf("benchmark starting — model=%s mode=%s attempts=%d", cfg.Model, cfg.Mode, cfg.Attempts)

	result, err := runner.Run(cfg)
	if err != nil {
		log.Fatalf("benchmark failed: %v", err)
	}

	if cfg.JSONOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling report: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		log.Printf("benchmark complete — run_id=%s", result.RunResult.RunID)
	}
}

// parseConditions splits a comma-separated condition string into a slice.
func parseConditions(s string) []string {
	if s == "" {
		return nil
	}
	var conds []string
	for _, c := range splitAndTrim(s) {
		conds = append(conds, c)
	}
	return conds
}

// splitAndTrim splits a string on commas and trims whitespace from each part.
func splitAndTrim(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	part := trimSpace(s[start:])
	if part != "" {
		result = append(result, part)
	}
	return result
}

// trimSpace removes leading and trailing whitespace from a string.
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
