// Package runner implements the benchmark test loop and condition dispatch.
package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/EffNine/gumi/benchmark"
	"github.com/EffNine/gumi/benchmark/report"
	"github.com/EffNine/gumi/benchmark/scorer"
)

// Config describes the parameters for a benchmark run.
type Config struct {
	Model         string
	Provider      string
	Mode          string
	Attempts      int
	Conditions    []string
	FrontierKey   string
	FrontierModel string
	OutputDir     string
	JSONOutput    bool
	APIKey        string
	BaseURL       string
}

// Run is the main entry point for the benchmark subsystem.
// It validates configuration, loads suites, executes tests, scores results,
// and returns a compiled report.
func Run(cfg Config) (*report.Report, error) {
	if cfg.Model == "" {
		return nil, errors.New("model is required")
	}
	if cfg.Attempts <= 0 {
		cfg.Attempts = 3
	}

	orch := NewOrchestrator(cfg)
	return orch.Execute()
}

// Orchestrator manages the full benchmark lifecycle: loading suites, dispatching
// tests across conditions and attempts, collecting results, scoring, and reporting.
type Orchestrator struct {
	config Config
}

// NewOrchestrator creates a new Orchestrator with the given configuration.
func NewOrchestrator(cfg Config) *Orchestrator {
	return &Orchestrator{
		config: cfg,
	}
}

// Execute runs the benchmark and returns the compiled report.
//
// The execution flow is:
//  1. Resolve model tier (small / medium / frontier)
//  2. Load YAML test suites matching the tier
//  3. Parse conditions from config
//  4. Create provider clients (direct, gumi, optional frontier)
//  5. Loop: suites → tests → conditions → attempts
//  6. Score each response against constraints
//  7. Aggregate per-capability metrics
//  8. Run degradation detection
//  9. Compute overall adaptive score
//  10. Build report and write outputs
func (o *Orchestrator) Execute() (*report.Report, error) {
	// 1. Resolve model tier
	tier, err := ResolveTier(o.config.Model, o.config.Provider)
	if err != nil {
		tier = TierMedium
	}

	// 2. Load suites matching tier
	suites, err := LoadSuites(tier)
	if err != nil {
		return nil, fmt.Errorf("loading suites: %w", err)
	}

	// 3. Parse conditions
	conditions := ParseConditions(o.config.Conditions)
	if len(conditions) == 0 {
		conditions = []Condition{ConditionDirect, ConditionGumiStabilized}
	}

	// 4. Create provider clients
	directClient := NewProviderClient(o.directBaseURL(), "")
	gumiClient := NewProviderClient(o.gumiBaseURL(), o.config.APIKey)

	var frontierClient *ProviderClient
	if o.config.FrontierModel != "" {
		frontierURL := o.frontierBaseURL()
		frontierClient = NewProviderClient(frontierURL, o.config.FrontierKey)
	}

	condMgr := NewConditionManager(
		o.config.Model, o.config.Provider,
		o.config.FrontierModel, o.config.FrontierKey,
	)

	// 5. Test loop
	runID := fmt.Sprintf("%s-%s", sanitizeName(o.config.Model), time.Now().UTC().Format("20060102T150405Z"))
	var allResults []benchmark.TestResult
	testCategories := make(map[string]string) // testID → category

	ctx := context.Background()

	for _, suite := range suites {
		for _, test := range suite.Tests {
			for _, cond := range conditions {
				client := o.clientForCondition(cond, directClient, gumiClient, frontierClient)
				if client == nil {
					continue
				}

				for attempt := 1; attempt <= o.config.Attempts; attempt++ {
					result := o.runSingleAttempt(ctx, client, condMgr, cond, test, suite.Category, attempt, runID)
					allResults = append(allResults, result)
					testCategories[test.ID] = suite.Category
				}
			}
		}
	}

	// 6. Aggregate results by capability
	caps := scorer.AggregateCapabilities(allResults, testCategories)

	// 7. Degradation check
	degReport := scorer.RunDegradationChecks(allResults, testCategories)

	// 8. Compute latency overhead
	latencyOverhead := computeLatencyOverhead(allResults)

	// 9. Compute overall score
	score, worthIt := scorer.OverallScore(caps, degReport, latencyOverhead, string(tier))

	// 10. Convert scorer capabilities to benchmark capabilities
	benchmarkCaps := make(map[string]benchmark.Capability)
	for k, v := range caps {
		benchmarkCaps[k] = benchmark.Capability{
			Direct:     benchmark.MetricSet{Mean: v.Direct.Mean, Std: v.Direct.Std, N: v.Direct.N},
			Gumi:     benchmark.MetricSet{Mean: v.Gumi.Mean, Std: v.Gumi.Std, N: v.Gumi.N},
			Delta:      v.Delta,
			EffectSize: v.EffectSize,
		}
	}

	// 11. Build report
	result := &report.Report{
		RunResult: benchmark.RunResult{
			SchemaVersion: 1,
			RunID:         runID,
			Model:         o.config.Model,
			Provider:      o.config.Provider,
			ModelTier:     string(tier),
			Config: benchmark.RunConfig{
				Attempts:   o.config.Attempts,
				Conditions: o.config.Conditions,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			},
			Summary: benchmark.Summary{
				OverallScore:      score,
				LatencyOverheadMs: latencyOverhead,
				DegradationRate:   degReport.DegradationRate,
				WorthIt:           worthIt,
			},
			Capabilities: benchmarkCaps,
			Degradation:  convertDegradationReport(degReport),
			PerTest:      allResults,
		},
	}

	// 12. Write outputs
	if o.config.OutputDir != "" {
		jsonPath := filepath.Join(o.config.OutputDir, runID+".json")
		mdPath := filepath.Join(o.config.OutputDir, runID+".md")

		if err := report.WriteJSON(result, jsonPath); err != nil {
			return nil, fmt.Errorf("writing JSON report: %w", err)
		}
		if err := report.WriteMarkdown(result, mdPath); err != nil {
			return nil, fmt.Errorf("writing markdown report: %w", err)
		}
	}

	return result, nil
}

// runSingleAttempt executes one test attempt: builds the request, calls the provider,
// scores the response, and stores the artifact.
func (o *Orchestrator) runSingleAttempt(
	ctx context.Context,
	client *ProviderClient,
	condMgr *ConditionManager,
	cond Condition,
	test benchmark.SuiteTest,
	category string,
	attempt int,
	runID string,
) benchmark.TestResult {
	req := condMgr.BuildRequest(cond, test)

	// Apply per-test timeout
	timeout := time.Duration(test.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second // default
	}
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	resp, err := client.ChatCompletion(testCtx, req)
	latency := time.Since(start)

	result := benchmark.TestResult{
		TestID:    test.ID,
		Condition: string(cond),
		Attempt:   attempt,
		Passed:    false,
		Subscores: make(map[string]float64),
		LatencyMs: latency.Seconds() * 1000,
	}

	if err != nil {
		result.Error = err.Error()
		return result
	}

	responseText := ""
	if len(resp.Choices) > 0 {
		responseText = resp.Choices[0].Message.Content
	}
	result.Output = responseText

	scored := scorer.New().Score(test, responseText)
	result.Passed = scored.Passed
	result.Subscores = scored.Subscores

	// Store raw output as artifact
	if responseText != "" {
		_ = report.StoreArtifact(runID, test.ID, string(cond), attempt, responseText)
	}

	return result
}

// clientForCondition selects the appropriate provider client based on the condition.
func (o *Orchestrator) clientForCondition(cond Condition, direct, gumi, frontier *ProviderClient) *ProviderClient {
	switch cond {
	case ConditionDirect:
		return direct
	case ConditionFrontier:
		return frontier
	default:
		// All gumi-* conditions route through the gumi runtime
		return gumi
	}
}

// directBaseURL returns the URL for direct (raw provider) API calls.
// This always goes to the raw provider (LM Studio), never through Gumi.
// The BaseURL config is only for the Gumi runtime, not for direct calls.
func (o *Orchestrator) directBaseURL() string {
	return "http://192.168.0.164:1234"
}

// gumiBaseURL returns the URL for Gumi runtime API calls.
func (o *Orchestrator) gumiBaseURL() string {
	return o.config.BaseURL
}

// frontierBaseURL returns the base URL for frontier API calls based on the provider.
func (o *Orchestrator) frontierBaseURL() string {
	switch o.config.Provider {
	case "anthropic":
		return "https://api.anthropic.com"
	case "openai":
		return "https://api.openai.com"
	case "google":
		return "https://generativelanguage.googleapis.com"
	default:
		return "https://api.anthropic.com"
	}
}

// computeLatencyOverhead calculates the average latency overhead of Gumi conditions
// compared to direct condition across all results.
func computeLatencyOverhead(results []benchmark.TestResult) float64 {
	var directLatency, gumiLatency float64
	var directCount, gumiCount int

	for _, r := range results {
		if r.Condition == string(ConditionDirect) {
			directLatency += r.LatencyMs
			directCount++
		} else if strings.HasPrefix(r.Condition, "gumi-") {
			gumiLatency += r.LatencyMs
			gumiCount++
		}
	}

	if directCount == 0 || gumiCount == 0 {
		return 0
	}

	directAvg := directLatency / float64(directCount)
	gumiAvg := gumiLatency / float64(gumiCount)

	overhead := gumiAvg - directAvg
	if overhead < 0 {
		overhead = 0
	}
	return overhead
}

// convertDegradationReport converts a scorer.DegradationReport to a benchmark.DegradationReport.
func convertDegradationReport(dr scorer.DegradationReport) benchmark.DegradationReport {
	corruptions := make([]benchmark.CorruptionRecord, len(dr.Corruptions))
	for i, c := range dr.Corruptions {
		corruptions[i] = benchmark.CorruptionRecord{
			TestID:   c.TestID,
			Original: c.Original,
			Repaired: c.Repaired,
			Severity: c.Severity,
		}
	}

	return benchmark.DegradationReport{
		OverRepairCount: dr.OverRepairCount,
		TotalTests:      dr.TotalTests,
		DegradationRate: dr.DegradationRate,
		Corruptions:     corruptions,
		LatencyOverhead: dr.LatencyOverhead,
	}
}

// sanitizeName replaces characters that are problematic in filenames.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		"@", "-",
		" ", "-",
		":", "-",
		"\n", "",
		"\r", "",
	)
	return replacer.Replace(name)
}
