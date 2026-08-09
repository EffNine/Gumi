// GEP Benchmark Runner - Tests all models against all suites
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/EffNine/gumi/benchmark/gep/baselines"
	"github.com/EffNine/gumi/benchmark/gep/runner"
	"github.com/EffNine/gumi/benchmark/gep/types"
)

func main() {
	models := []struct {
		name string
		id   string
	}{
		{"qwen3-8b", "qwen3:8b"},
		{"gemma3-4b", "gemma3:4b"},
		{"llama3.1-8b", "llama3.1:8b"},
	}

	outputDir := "/home/afnan/workspace/Gumi/benchmarks/gep/reports"
	baselineDir := "/home/afnan/.gumi/gep/baselines"
	docsDir := "/home/afnan/workspace/Gumi/docs/reports"

	os.MkdirAll(outputDir, 0755)
	os.MkdirAll(baselineDir, 0755)
	os.MkdirAll(docsDir, 0755)

	store := baselines.NewStore(baselineDir)

	for _, model := range models {
		fmt.Printf("\n=== Benchmarking %s ===\n", model.name)

		cfg := runner.RunConfig{
			Model:       model.id,
			Provider:    types.ProviderOllama,
			ProviderURL: "http://localhost:11434",
			Attempts:    3,
			OutputDir:   outputDir,
		}

		report, err := runner.Run(cfg)
		if err != nil {
			log.Printf("ERROR benchmarking %s: %v", model.name, err)
			continue
		}

		fmt.Printf("Overall Score: %.2f\n", report.Summary.OverallScore)
		fmt.Printf("Pass Rate: %.0f%%\n", report.Summary.PassRate*100)
		fmt.Printf("Avg Latency: %.0fms\n", report.Summary.AvgLatencyMs)
		fmt.Printf("Total Tests: %d (Passed: %d)\n", report.Summary.TotalTests, report.Summary.PassedTests)
		fmt.Printf("Run ID: %s\n", report.RunID)

		fmt.Printf("\nCapabilities:\n")
		for suiteType, cap := range report.Capabilities {
			fmt.Printf("  %s: direct=%.3f gumi=%.3f delta=%.3f n=%d\n", suiteType, cap.Direct.Mean, cap.Gumi.Mean, cap.Delta, cap.Gumi.N)
		}

		if err := store.Save(report); err != nil {
			log.Printf("ERROR saving baseline: %v", err)
			continue
		}
		fmt.Printf("Baseline saved\n")

		// Copy reports to docs
		jsonPath := filepath.Join(outputDir, report.RunID+".json")
		mdPath := filepath.Join(outputDir, report.RunID+".md")
		copyFile(jsonPath, filepath.Join(docsDir, fmt.Sprintf("baseline_%s_%s.json", model.name, report.RunID)))
		copyFile(mdPath, filepath.Join(docsDir, fmt.Sprintf("baseline_%s_%s.md", model.name, report.RunID)))
	}

	fmt.Println("\n=== All benchmarks complete ===")
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.WriteFile(dst, data, 0644)
}
