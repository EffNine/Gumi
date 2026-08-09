// Package runner implements the GEP benchmark execution engine.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EffNine/gumi/benchmark/gep/providers"
	"github.com/EffNine/gumi/benchmark/gep/reports"
	"github.com/EffNine/gumi/benchmark/gep/scorer"
	"github.com/EffNine/gumi/benchmark/gep/types"
	"gopkg.in/yaml.v3"
)

// RunConfig is the configuration for a GEP benchmark run.
type RunConfig struct {
	Model       string               `yaml:"model"`
	Provider    types.ProviderType   `yaml:"provider"`
	ProviderURL string               `yaml:"provider_url"`
	APIKey      string               `yaml:"api_key,omitempty"`
	Attempts    int                  `yaml:"attempts"`
	SuiteID     string               `yaml:"suite_id,omitempty"`
	Difficulty  types.DifficultyTier `yaml:"difficulty,omitempty"`
	OutputDir   string               `yaml:"output_dir"`
	// Conditions controls which execution paths to run. Defaults to both.
	Conditions []types.GEPCondition `yaml:"conditions,omitempty"`
	// GumiURL is the Gumi runtime base URL for gumi-stabilized condition.
	GumiURL string `yaml:"gumi_url,omitempty"`
	// GumiAPIKey is the bearer token for the Gumi runtime.
	GumiAPIKey string `yaml:"gumi_api_key,omitempty"`
	// Scope sets the baseline scope: "model" or "runtime".
	Scope types.GEPScope `yaml:"scope,omitempty"`
}

// defaultConditions returns the conditions to run when none are specified.
func defaultConditions() []types.GEPCondition {
	return []types.GEPCondition{types.ConditionDirect, types.ConditionGumiStabilized}
}

// Run executes a GEP benchmark and returns the compiled report.
func Run(cfg RunConfig) (*types.GEPReport, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if cfg.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if cfg.ProviderURL == "" {
		return nil, fmt.Errorf("provider URL is required")
	}
	if cfg.Attempts <= 0 {
		cfg.Attempts = 3
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "benchmarks/gep/reports"
	}
	if len(cfg.Conditions) == 0 {
		cfg.Conditions = defaultConditions()
	}
	if cfg.Scope == "" {
		cfg.Scope = types.ScopeRuntime
	}

	// Create provider
	provider, err := providers.NewProvider(string(cfg.Provider), cfg.ProviderURL, cfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("creating provider: %w", err)
	}

	// Load suites
	suites, err := loadSuites(cfg.SuiteID, cfg.Difficulty)
	if err != nil {
		return nil, fmt.Errorf("loading suites: %w", err)
	}
	if len(suites) == 0 {
		return nil, fmt.Errorf("no suites found for suite_id=%q difficulty=%q", cfg.SuiteID, cfg.Difficulty)
	}

	// Execute tests
	runID := fmt.Sprintf("gep-%s-%s", sanitizeName(cfg.Model), time.Now().UTC().Format("20060102T150405Z"))
	var allResults []types.GEPResult
	scorerEngine := scorer.New()

	ctx := context.Background()

	for _, suite := range suites {
		for _, test := range suite.Tests {
			for _, cond := range cfg.Conditions {
				for attempt := 1; attempt <= cfg.Attempts; attempt++ {
					result := runAttempt(ctx, provider, cfg, cond, test, attempt, scorerEngine)
					result.SuiteID = suite.ID
					result.Provider = cfg.Provider
					result.Model = cfg.Model
					result.Timestamp = time.Now().UTC()
					allResults = append(allResults, result)
				}
			}
		}
	}

	// Aggregate results by capability and condition
	capabilities := aggregateCapabilities(allResults)

	// Compute summary with per-condition breakdown
	summary := computeSummary(allResults, cfg.Conditions)

	// Build report
	report := &types.GEPReport{
		SchemaVersion:   2,
		RunID:           runID,
		ProtocolVersion: types.ProtocolVersion,
		Config: types.GEPRunConfig{
			Model:        cfg.Model,
			Provider:     cfg.Provider,
			ProviderURL:  cfg.ProviderURL,
			APIKey:       cfg.APIKey,
			Timestamp:    time.Now().UTC(),
			Attempts:     cfg.Attempts,
			SuiteID:      cfg.SuiteID,
			Difficulty:   cfg.Difficulty,
			Conditions:   cfg.Conditions,
			GumiURL:      cfg.GumiURL,
			GumiAPIKey:   cfg.GumiAPIKey,
			Scope:        cfg.Scope,
		},
		Summary:      summary,
		Capabilities: capabilities,
		PerTest:      allResults,
	}

	// Write outputs
	if cfg.OutputDir != "" {
		jsonPath := filepath.Join(cfg.OutputDir, runID+".json")
		mdPath := filepath.Join(cfg.OutputDir, runID+".md")

		if err := reports.WriteJSON(report, jsonPath); err != nil {
			return nil, fmt.Errorf("writing JSON report: %w", err)
		}
		if err := reports.WriteMarkdown(report, mdPath); err != nil {
			return nil, fmt.Errorf("writing markdown report: %w", err)
		}
	}

	return report, nil
}

// runAttempt executes a single test attempt against the provider.
func runAttempt(ctx context.Context, provider providers.Provider, cfg RunConfig, cond types.GEPCondition, test types.GEPTest, attempt int, sc *scorer.Scorer) types.GEPResult {
	result := types.GEPResult{
		TestID:    test.ID,
		Attempt:   attempt,
		Condition: cond,
		Passed:    false,
		Subscores: make(map[string]float64),
	}

	// Build messages
	var messages []providers.ChatMessage
	if test.Type == "multi_turn" {
		for _, turn := range test.Turns {
			if turn.Role == "assistant" {
				break
			}
			messages = append(messages, providers.ChatMessage{
				Role:    turn.Role,
				Content: turn.Content,
			})
		}
	} else {
		messages = []providers.ChatMessage{
			{Role: "user", Content: test.Prompt},
		}
	}

	timeout := time.Duration(test.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	var resp *providers.ChatResponse
	var err error

	if cond == types.ConditionGumiStabilized && cfg.GumiURL != "" {
		resp, err = callGumiRuntime(testCtx, cfg, test, messages)
	} else {
		resp, err = provider.ChatCompletion(testCtx, providers.ChatRequest{
			Model:       cfg.Model,
			Messages:    messages,
			MaxTokens:   test.MaxTokens,
			Temperature: 0.3,
		})
	}
	latency := time.Since(start)

	result.LatencyMs = latency.Seconds() * 1000

	if err != nil {
		result.Error = err.Error()
		return result
	}

	responseText := ""
	if len(resp.Choices) > 0 {
		responseText = resp.Choices[0].Message.Content
	}
	result.Output = responseText

	// Handle self-consistency tests
	if test.Type == "self_consistency" && len(test.Variants) > 0 {
		return runSelfConsistencyAttempt(testCtx, provider, cfg, cond, test, attempt, sc, result)
	}

	// Score the response
	scored := sc.Score(test, responseText)
	result.Passed = scored.Passed
	result.Subscores = scored.Subscores
	if scored.Error != "" {
		result.Error = scored.Error
	}

	return result
}

// callGumiRuntime sends a request through the Gumi runtime API in stabilized mode.
func callGumiRuntime(ctx context.Context, cfg RunConfig, test types.GEPTest, messages []providers.ChatMessage) (*providers.ChatResponse, error) {
	reqBody := struct {
		Model    string              `json:"model"`
		Messages []providers.ChatMessage `json:"messages"`
		Stream   bool                `json:"stream"`
		Gumi     map[string]interface{} `json:"gumi,omitempty"`
	}{
		Model:    cfg.Model,
		Messages: messages,
		Stream:   false,
	}
	if cfg.GumiAPIKey != "" {
		reqBody.Gumi = map[string]interface{}{"mode": "stabilized"}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling gumi request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.GumiURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating gumi request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.GumiAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.GumiAPIKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gumi runtime request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gumi response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gumi runtime returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var gumiResp struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &gumiResp); err != nil {
		return nil, fmt.Errorf("decoding gumi response: %w", err)
	}

	if len(gumiResp.Choices) == 0 {
		return nil, fmt.Errorf("gumi runtime returned empty choices")
	}

	return &providers.ChatResponse{
		ID:    fmt.Sprintf("gumi-%d", time.Now().UnixNano()),
		Model: gumiResp.Model,
		Choices: []providers.ChatChoice{
			{
				Index:        0,
				Message:      providers.ChatMessage{Role: gumiResp.Choices[0].Message.Role, Content: gumiResp.Choices[0].Message.Content},
				FinishReason: gumiResp.Choices[0].FinishReason,
			},
		},
	}, nil
}

// runSelfConsistencyAttempt handles self-consistency test execution.
func runSelfConsistencyAttempt(ctx context.Context, provider providers.Provider, cfg RunConfig, cond types.GEPCondition, test types.GEPTest, attempt int, sc *scorer.Scorer, baseResult types.GEPResult) types.GEPResult {
	prompts := []string{test.Prompt}
	prompts = append(prompts, test.Variants...)

	if len(prompts) == 0 {
		baseResult.Error = "self_consistency test has no prompt variants"
		return baseResult
	}

	timeout := time.Duration(test.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	responses := make([]string, 0, len(prompts))
	var totalLatency float64
	var firstErr string

	for _, prompt := range prompts {
		testCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		resp, err := callGumiRuntime(testCtx, cfg, test, []providers.ChatMessage{{Role: "user", Content: prompt}})
		totalLatency += time.Since(start).Seconds() * 1000
		cancel()

		if err != nil {
			if firstErr == "" {
				firstErr = err.Error()
			}
			responses = append(responses, "")
			continue
		}
		text := ""
		if len(resp.Choices) > 0 {
			text = resp.Choices[0].Message.Content
		}
		responses = append(responses, text)
	}

	baseResult.LatencyMs = totalLatency
	baseResult.Output = strings.Join(responses, "\n---\n")
	if firstErr != "" {
		baseResult.Error = firstErr
	}

	// Score self-consistency
	prior := []string{}
	if len(responses) > 1 {
		prior = responses[:len(responses)-1]
	}

	scoringTest := test
	scoringTest.Constraints = append(scoringTest.Constraints, types.GEPConstraint{
		Field:    "self_consistency",
		Operator: "self_consistency",
		Value:    prior,
	})

	last := ""
	if len(responses) > 0 {
		last = responses[len(responses)-1]
	}
	scored := sc.Score(scoringTest, last)
	baseResult.Passed = scored.Passed && firstErr == ""
	baseResult.Subscores = scored.Subscores
	if scored.Error != "" {
		if baseResult.Error != "" {
			baseResult.Error = baseResult.Error + "; " + scored.Error
		} else {
			baseResult.Error = scored.Error
		}
	}

	consistency := scorer.ScoreSelfConsistency(responses)
	baseResult.Subscores["self_consistency"] = consistency
	if consistency != 1.0 {
		baseResult.Passed = false
	}

	if test.ExpectedAnswer != "" {
		needle := strings.ToLower(strings.TrimSpace(test.ExpectedAnswer))
		matched := false
		for _, resp := range responses {
			if strings.Contains(strings.ToLower(resp), needle) {
				matched = true
				break
			}
		}
		if matched {
			baseResult.Subscores["expected_answer"] = 1.0
		} else {
			baseResult.Subscores["expected_answer"] = 0.0
			baseResult.Passed = false
		}
	}

	return baseResult
}

// loadSuites reads GEP suite YAML files from the suites directory.
func loadSuites(suiteID string, difficulty types.DifficultyTier) ([]types.GESuite, error) {
	suitesDir := findSuitesDir()
	if suitesDir == "" {
		return nil, fmt.Errorf("suites directory not found")
	}

	if suiteID != "" {
		return loadSuiteFile(suitesDir, suiteID, difficulty)
	}

	var allSuites []types.GESuite
	difficulties := []types.DifficultyTier{types.TierEasy, types.TierMedium, types.TierHard}
	if difficulty != "" {
		difficulties = []types.DifficultyTier{difficulty}
	}

	for _, diff := range difficulties {
		suites, err := loadSuitesByDifficulty(suitesDir, diff)
		if err != nil {
			continue
		}
		allSuites = append(allSuites, suites...)
	}

	return allSuites, nil
}

// findSuitesDir locates the GEP suites directory.
func findSuitesDir() string {
	candidatePaths := []string{
		filepath.Join("benchmark", "gep", "suites"),
		filepath.Join("gep", "suites"),
		filepath.Join("benchmark", "suites"),
	}

	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	absPath, _ := filepath.Abs("suites")
	if _, err := os.Stat(absPath); err == nil {
		return absPath
	}

	return ""
}

// loadSuiteFile loads a single suite YAML file.
func loadSuiteFile(suitesDir, suiteID string, difficulty types.DifficultyTier) ([]types.GESuite, error) {
	categoryDirs, err := os.ReadDir(suitesDir)
	if err != nil {
		return nil, err
	}

	var suites []types.GESuite
	difficultyStr := string(difficulty)
	if difficulty == "" {
		difficultyStr = "easy"
	}

	for _, dir := range categoryDirs {
		if !dir.IsDir() {
			continue
		}
		filePath := filepath.Join(suitesDir, dir.Name(), difficultyStr+".yaml")
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Parse with generic map first to handle both top-level structures
		var raw struct {
			Suite types.GESuite   `yaml:"suite"`
			Tests []types.GEPTest `yaml:"tests"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}

		// If tests are at top level (not nested in suite), assign them
		if len(raw.Tests) > 0 {
			raw.Suite.Tests = raw.Tests
		}

		if suiteID != "" && raw.Suite.ID != suiteID {
			continue
		}

		suites = append(suites, raw.Suite)
	}

	return suites, nil
}

// loadSuitesByDifficulty loads all suites for a given difficulty level.
func loadSuitesByDifficulty(suitesDir string, difficulty types.DifficultyTier) ([]types.GESuite, error) {
	categoryDirs, err := os.ReadDir(suitesDir)
	if err != nil {
		return nil, err
	}

	var suites []types.GESuite
	difficultyStr := string(difficulty)

	for _, dir := range categoryDirs {
		if !dir.IsDir() {
			continue
		}
		filePath := filepath.Join(suitesDir, dir.Name(), difficultyStr+".yaml")
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Parse with generic map first to handle both top-level structures
		var raw struct {
			Suite types.GESuite   `yaml:"suite"`
			Tests []types.GEPTest `yaml:"tests"`
		}
		if err := yaml.Unmarshal(data, &raw); err != nil {
			continue
		}

		// If tests are at top level (not nested in suite), assign them
		if len(raw.Tests) > 0 {
			raw.Suite.Tests = raw.Tests
		}

		suites = append(suites, raw.Suite)
	}

	return suites, nil
}

// aggregateCapabilities groups results by suite type and computes per-condition metrics.
func aggregateCapabilities(results []types.GEPResult) map[string]types.GEPCapability {
	// Group by suite and condition
	type key struct {
		suiteID string
		cond    types.GEPCondition
	}
	groups := make(map[key][]types.GEPResult)
	for _, r := range results {
		k := key{suiteID: r.SuiteID, cond: r.Condition}
		groups[k] = append(groups[k], r)
	}

	caps := make(map[string]types.GEPCapability)
	for suiteID, res := range groups {
		// We'll aggregate by suiteID below
		_ = res
		_ = suiteID
	}

	// Build per-suite, per-condition aggregates
	suiteConds := make(map[string]map[types.GEPCondition][]types.GEPResult)
	for k, res := range groups {
		if suiteConds[k.suiteID] == nil {
			suiteConds[k.suiteID] = make(map[types.GEPCondition][]types.GEPResult)
		}
		suiteConds[k.suiteID][k.cond] = res
	}

	for suiteID, condGroups := range suiteConds {
		directResults := condGroups[types.ConditionDirect]
		gumiResults := condGroups[types.ConditionGumiStabilized]

		directMetric := computeMetricSet(directResults)
		gumiMetric := computeMetricSet(gumiResults)

		// Compute pass rates
		var directPassed, gumiPassed, totalDirect, totalGumi int
		for _, r := range directResults {
			totalDirect++
			if r.Passed {
				directPassed++
			}
		}
		for _, r := range gumiResults {
			totalGumi++
			if r.Passed {
				gumiPassed++
			}
		}
		directPassRate := float64(directPassed) / float64(max(totalDirect, 1))
		gumiPassRate := float64(gumiPassed) / float64(max(totalGumi, 1))

		delta := gumiMetric.Mean - directMetric.Mean

		caps[suiteID] = types.GEPCapability{
			SuiteType: types.SuiteType(suiteID),
			Direct:    directMetric,
			Gumi:      gumiMetric,
			Delta:     delta,
			PassRate:  gumiPassRate - directPassRate,
		}
	}

	return caps
}

// computeMetricSet computes statistical metrics from a group of results.
func computeMetricSet(results []types.GEPResult) types.GEPMetricSet {
	if len(results) == 0 {
		return types.GEPMetricSet{}
	}

	scores := make([]float64, len(results))
	for i, r := range results {
		var scoreSum, weightSum float64
		for _, score := range r.Subscores {
			scoreSum += score
			weightSum++
		}
		if weightSum > 0 {
			scores[i] = scoreSum / weightSum
		}
	}

	mean := meanOf(scores)
	std := stdOf(scores, mean)

	sorted := make([]float64, len(scores))
	copy(sorted, scores)
	sort.Float64s(sorted)

	return types.GEPMetricSet{
		Mean:   mean,
		Std:    std,
		N:      len(results),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Median: percentile(sorted, 0.5),
		P25:    percentile(sorted, 0.25),
		P75:    percentile(sorted, 0.75),
	}
}

// computeSummary computes the top-level summary statistics with per-condition breakdowns.
func computeSummary(results []types.GEPResult, conditions []types.GEPCondition) types.GEPSummary {
	if len(results) == 0 {
		return types.GEPSummary{}
	}

	// Separate results by condition
	byCond := make(map[types.GEPCondition][]types.GEPResult)
	for _, r := range results {
		byCond[r.Condition] = append(byCond[r.Condition], r)
	}

	computePerCondition := func(condResults []types.GEPResult) (overallScore, passRate, avgLatency float64, totalTests, passedTests int) {
		if len(condResults) == 0 {
			return 0, 0, 0, 0, 0
		}
		var totalLatency int
		for _, r := range condResults {
			totalLatency += int(r.LatencyMs)
			if r.Passed {
				passedTests++
			}
		}
		passRate = float64(passedTests) / float64(len(condResults))
		avgLatency = float64(totalLatency) / float64(len(condResults))

		var scoreSum float64
		var scoreCount int
		for _, r := range condResults {
			for _, score := range r.Subscores {
				scoreSum += score
				scoreCount++
			}
		}
		if scoreCount > 0 {
			overallScore = scoreSum / float64(scoreCount)
		}
		totalTests = len(condResults)
		return
	}

	directScore, directPassRate, directLatency, directTotal, directPassed := computePerCondition(byCond[types.ConditionDirect])
	gumiScore, gumiPassRate, gumiLatency, gumiTotal, gumiPassed := computePerCondition(byCond[types.ConditionGumiStabilized])

	overallScore := gumiScore
	if overallScore == 0 {
		overallScore = directScore
	}
	passRate := gumiPassRate
	if passRate == 0 {
		passRate = directPassRate
	}
	avgLatency := gumiLatency
	if avgLatency == 0 {
		avgLatency = directLatency
	}
	totalTests := gumiTotal
	if totalTests == 0 {
		totalTests = directTotal
	}
	passedTests := gumiPassed
	if passedTests == 0 {
		passedTests = directPassed
	}

	return types.GEPSummary{
		OverallScore:    math.Round(overallScore*100) / 100,
		DirectScore:     math.Round(directScore*100) / 100,
		GumiScore:       math.Round(gumiScore*100) / 100,
		ScoreDelta:      math.Round((gumiScore-directScore)*100) / 100,
		PassRate:        math.Round(passRate*100) / 100,
		DirectPassRate:  math.Round(directPassRate*100) / 100,
		GumiPassRate:    math.Round(gumiPassRate*100) / 100,
		PassRateDelta:   math.Round((gumiPassRate-directPassRate)*100) / 100,
		AvgLatencyMs:    math.Round(avgLatency*10) / 10,
		DirectLatencyMs: math.Round(directLatency*10) / 10,
		GumiLatencyMs:   math.Round(gumiLatency*10) / 10,
		LatencyDeltaMs:  math.Round((gumiLatency-directLatency)*10) / 10,
		TotalTests:      totalTests,
		PassedTests:     passedTests,
		WorthIt:         overallScore > 0.5,
	}
}

// ---- Statistical helpers ----

func meanOf(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	return sum / float64(len(xs))
}

func stdOf(xs []float64, mean float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var variance float64
	for _, v := range xs {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(xs)))
}

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lo := int(math.Floor(idx))
	hi := lo + 1
	if hi >= n {
		hi = n - 1
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// sanitizeName replaces characters problematic in filenames.
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
