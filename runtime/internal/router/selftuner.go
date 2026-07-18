package router

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
)

// FindBestFunc is the registry candidate-selection signature used by the
// self-tuner so it can be tested without a real CodingModelRegistry.
type FindBestFunc func(strategy PreferenceStrategy, minCoding CodingStrength, minContext int, minReasoning string, maxSize ModelSizeCategory, available map[string]bool) *CodingModelRegistryEntry

// ---------------------------------------------------------------------------
// ModelFitStats
// ---------------------------------------------------------------------------

// ModelFitStats holds aggregated performance for one model in a single
// (difficulty, task_type) bucket.
type ModelFitStats struct {
	ModelID      string  `json:"model_id"`
	Attempts     int     `json:"attempts"`
	Successes    int     `json:"successes"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	AvgRetries   float64 `json:"avg_retries"`
	LastUpdated  string  `json:"last_updated"`
}

// FitBucket is the query key for fit statistics.
type FitBucket struct {
	Difficulty int
	TaskType   string
}

// ---------------------------------------------------------------------------
// ModelFitLookup (extended)
// ---------------------------------------------------------------------------

// ModelFitLookup is the interface the router uses to query the memory engine
// for model performance data. Implemented by memory.MemoryEngine.
type ModelFitLookup interface {
	// GetBestModelForRouter returns the best model ID and success rate for a
	// given difficulty and task type. Returns ("", 0, false) if no model has
	// enough data.
	GetBestModelForRouter(difficulty int, taskType string) (modelID string, successRate float64, ok bool)

	// GetFitStats returns all observed models for a bucket, sorted by success
	// rate descending. minAttempts filters out under-sampled models; 0 disables.
	GetFitStats(difficulty int, taskType string, minAttempts int) ([]ModelFitStats, bool)

	// GetModelFit returns the stats for a single model in a bucket.
	GetModelFit(modelID string, difficulty int, taskType string) (ModelFitStats, bool)

	// TotalAttempts returns total observations across all buckets.
	TotalAttempts() int
}

// ---------------------------------------------------------------------------
// RuleOverride
// ---------------------------------------------------------------------------

// RuleOverride is a runtime adjustment to a rule's action, applied on top of
// the static CodingRule from DefaultCodingRules().
type RuleOverride struct {
	RuleName   string              `json:"rule_name"`
	MinCoding  *string             `json:"min_coding,omitempty"`
	MinContext *int                `json:"min_context,omitempty"`
	Prefer     *PreferenceStrategy `json:"prefer,omitempty"`
	Reason     string              `json:"reason,omitempty"`
	AppliedAt  time.Time           `json:"applied_at"`
}

// AdjustmentRecord records one change made during a tuning pass.
type AdjustmentRecord struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

// SelfTuningSnapshot is the serializable state emitted to telemetry.
type SelfTuningSnapshot struct {
	GeneratedAt   time.Time          `json:"generated_at"`
	RuleOverrides []RuleOverride     `json:"rule_overrides"`
	ModelBoosts   map[string]float64 `json:"model_boosts"`
	ModelDemotes  map[string]float64 `json:"model_demotes"`
	TotalAttempts int                `json:"total_attempts"`
	Adjustments   []AdjustmentRecord `json:"adjustments"`
}

// ---------------------------------------------------------------------------
// SelfTuner
// ---------------------------------------------------------------------------

// SelfTuner holds the in-memory overlay of rule adjustments and model
// promotion/demotion scores derived from observed outcomes.
type SelfTuner struct {
	cfg          config.SelfTuningConfig
	fit          ModelFitLookup
	snapshotPath string
	rng          func() float64
	now          func() time.Time
	telemetry    func(event string, md map[string]string)

	mu            sync.RWMutex
	outcomesSeen  int
	ruleOverrides map[string]RuleOverride
	modelBoosts   map[string]float64
	modelDemotes  map[string]float64
	lastSnapshot  SelfTuningSnapshot
}

// SelfTuningSnapshotPath returns the JSON snapshot path derived from the memory
// database directory, or ~/.gumi/self-tuning-snapshot.json when dbPath is empty.
func SelfTuningSnapshotPath(memoryDBPath string) string {
	if memoryDBPath != "" {
		return filepath.Join(filepath.Dir(memoryDBPath), "self-tuning-snapshot.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "self-tuning-snapshot.json"
	}
	return filepath.Join(home, ".gumi", "self-tuning-snapshot.json")
}

// NewSelfTuner constructs a tuner. fit may be nil (self-tuning disabled).
// snapshotPath is used when cfg.PersistSnapshot is true; pass "" to use the
// default ~/.gumi/self-tuning-snapshot.json location.
func NewSelfTuner(cfg config.SelfTuningConfig, fit ModelFitLookup, snapshotPath string) *SelfTuner {
	if snapshotPath == "" {
		snapshotPath = SelfTuningSnapshotPath("")
	}
	s := &SelfTuner{
		cfg:           cfg,
		fit:           fit,
		snapshotPath:  snapshotPath,
		rng:           rand.Float64,
		now:           time.Now,
		ruleOverrides: make(map[string]RuleOverride),
		modelBoosts:   make(map[string]float64),
		modelDemotes:  make(map[string]float64),
	}
	if cfg.PersistSnapshot {
		_ = s.LoadSnapshot()
	}
	return s
}

// SetTelemetryHook attaches a callback for significant self-tuning events.
func (s *SelfTuner) SetTelemetryHook(hook func(event string, md map[string]string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telemetry = hook
}

// Snapshot returns the most recent tuning snapshot.
func (s *SelfTuner) Snapshot() SelfTuningSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSnapshot
}

// HasRuleOverride reports whether a runtime rule override exists for ruleName.
func (s *SelfTuner) HasRuleOverride(ruleName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.ruleOverrides[ruleName]
	return ok
}

// SaveSnapshot writes the current tuning state to snapshotPath when persistence
// is enabled.
func (s *SelfTuner) SaveSnapshot() error {
	if !s.cfg.PersistSnapshot || s.snapshotPath == "" {
		return nil
	}

	s.mu.RLock()
	snap := s.lastSnapshot
	s.mu.RUnlock()

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.snapshotPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.snapshotPath, data, 0o644)
}

// LoadSnapshot restores rule overrides and model boost/demote maps from disk.
func (s *SelfTuner) LoadSnapshot() error {
	if !s.cfg.PersistSnapshot || s.snapshotPath == "" {
		return nil
	}

	data, err := os.ReadFile(s.snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var snap SelfTuningSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ruleOverrides = make(map[string]RuleOverride, len(snap.RuleOverrides))
	for _, ov := range snap.RuleOverrides {
		s.ruleOverrides[ov.RuleName] = ov
	}
	s.modelBoosts = snap.ModelBoosts
	if s.modelBoosts == nil {
		s.modelBoosts = make(map[string]float64)
	}
	s.modelDemotes = snap.ModelDemotes
	if s.modelDemotes == nil {
		s.modelDemotes = make(map[string]float64)
	}
	s.lastSnapshot = snap
	return nil
}

// ObserveOutcome records one routing outcome and optionally triggers a Tune()
// pass when enough new outcomes have accumulated and warmup is satisfied.
func (s *SelfTuner) ObserveOutcome(modelID string, difficulty int, taskType string, success bool) {
	if !s.cfg.Enabled || s.fit == nil {
		return
	}

	s.mu.Lock()
	s.outcomesSeen++
	shouldTune := s.outcomesSeen%s.cfg.MinOutcomesBetweenTunes == 0
	s.mu.Unlock()

	if shouldTune {
		s.Tune()
	}
}

// Tune runs one adjustment pass over the rule set + model scores.
// It returns the snapshot of changes applied this pass.
func (s *SelfTuner) Tune() SelfTuningSnapshot {
	snapshot := SelfTuningSnapshot{
		GeneratedAt:   s.now(),
		RuleOverrides: nil,
		ModelBoosts:   make(map[string]float64),
		ModelDemotes:  make(map[string]float64),
		TotalAttempts: s.fit.TotalAttempts(),
		Adjustments:   nil,
	}

	if !s.cfg.Enabled || s.fit == nil {
		s.mu.Lock()
		s.lastSnapshot = snapshot
		s.mu.Unlock()
		return snapshot
	}

	// Don't tune during warmup.
	if snapshot.TotalAttempts < s.cfg.WarmupAttempts {
		s.mu.Lock()
		s.lastSnapshot = snapshot
		s.mu.Unlock()
		s.emit("self_tuning_warmup", map[string]string{
			"total_attempts": fmt.Sprintf("%d", snapshot.TotalAttempts),
			"warmup":         fmt.Sprintf("%d", s.cfg.WarmupAttempts),
		})
		return snapshot
	}

	newOverrides := make(map[string]RuleOverride)
	newBoosts := make(map[string]float64)
	newDemotes := make(map[string]float64)
	var adjustments []AdjustmentRecord

	for _, rule := range DefaultCodingRules() {
		override := s.tuneRule(rule)
		if override != nil {
			newOverrides[rule.Name] = *override
			if override.MinCoding != nil {
				adjustments = append(adjustments, AdjustmentRecord{
					Kind:   "raise_min_coding",
					Target: rule.Name,
					From:   rule.RouteAction.MinCoding,
					To:     *override.MinCoding,
					Reason: override.Reason,
				})
			}
			if override.MinContext != nil {
				adjustments = append(adjustments, AdjustmentRecord{
					Kind:   "bump_min_context",
					Target: rule.Name,
					From:   fmt.Sprintf("%d", rule.RouteAction.MinContext),
					To:     fmt.Sprintf("%d", *override.MinContext),
					Reason: override.Reason,
				})
			}
			if override.Prefer != nil {
				adjustments = append(adjustments, AdjustmentRecord{
					Kind:   "flip_strategy",
					Target: rule.Name,
					From:   string(rule.RouteAction.Prefer),
					To:     string(*override.Prefer),
					Reason: override.Reason,
				})
			}
		}
	}

	// Promote / demote models based on per-bucket performance.
	for _, rule := range DefaultCodingRules() {
		for _, bucket := range ruleBuckets(rule) {
			stats, ok := s.fit.GetFitStats(bucket.Difficulty, bucket.TaskType, s.cfg.MinAttempts)
			if !ok || len(stats) == 0 {
				continue
			}
			for _, st := range stats {
				if st.Attempts < s.cfg.MinAttempts {
					continue
				}
				rate := st.SuccessRate
				if rate >= s.cfg.PromoteThreshold {
					if _, exists := newBoosts[st.ModelID]; !exists {
						newBoosts[st.ModelID] = s.cfg.BoostWeight
						adjustments = append(adjustments, AdjustmentRecord{
							Kind:   "promote",
							Target: st.ModelID,
							From:   fmt.Sprintf("%.2f", rate-s.cfg.BoostWeight),
							To:     fmt.Sprintf("%.2f", rate),
							Reason: fmt.Sprintf("success rate %.0f%% over %d attempts", rate*100, st.Attempts),
						})
					}
				} else if rate <= s.cfg.DemoteThreshold {
					if _, exists := newDemotes[st.ModelID]; !exists {
						newDemotes[st.ModelID] = s.cfg.DemoteWeight
						adjustments = append(adjustments, AdjustmentRecord{
							Kind:   "demote",
							Target: st.ModelID,
							From:   fmt.Sprintf("%.2f", rate+s.cfg.DemoteWeight),
							To:     fmt.Sprintf("%.2f", rate),
							Reason: fmt.Sprintf("success rate %.0f%% over %d attempts", rate*100, st.Attempts),
						})
					}
				}
			}
		}
	}

	// Populate snapshot serializable overrides slice.
	for _, ov := range newOverrides {
		snapshot.RuleOverrides = append(snapshot.RuleOverrides, ov)
	}
	sort.Slice(snapshot.RuleOverrides, func(i, j int) bool {
		return snapshot.RuleOverrides[i].RuleName < snapshot.RuleOverrides[j].RuleName
	})
	snapshot.ModelBoosts = newBoosts
	snapshot.ModelDemotes = newDemotes
	snapshot.Adjustments = adjustments

	s.mu.Lock()
	s.ruleOverrides = newOverrides
	s.modelBoosts = newBoosts
	s.modelDemotes = newDemotes
	s.lastSnapshot = snapshot
	s.mu.Unlock()

	s.emit("self_tuning_pass", map[string]string{
		"adjustments_count":    fmt.Sprintf("%d", len(adjustments)),
		"rule_overrides_count": fmt.Sprintf("%d", len(newOverrides)),
		"boosts_count":         fmt.Sprintf("%d", len(newBoosts)),
		"demotes_count":        fmt.Sprintf("%d", len(newDemotes)),
		"total_attempts":       fmt.Sprintf("%d", snapshot.TotalAttempts),
	})
	for _, adj := range adjustments {
		s.emit("self_tuning_adjustment", map[string]string{
			"kind":   adj.Kind,
			"target": adj.Target,
			"from":   adj.From,
			"to":     adj.To,
			"reason": adj.Reason,
		})
	}

	if s.cfg.PersistSnapshot {
		_ = s.SaveSnapshot()
	}

	return snapshot
}

// tuneRule computes a runtime override for a single rule, or nil if no
// adjustment is warranted.
func (s *SelfTuner) tuneRule(rule CodingRule) *RuleOverride {
	buckets := ruleBuckets(rule)
	if len(buckets) == 0 {
		return nil
	}

	var allStats []ModelFitStats
	for _, bucket := range buckets {
		stats, ok := s.fit.GetFitStats(bucket.Difficulty, bucket.TaskType, s.cfg.MinAttempts)
		if ok && len(stats) > 0 {
			allStats = append(allStats, stats...)
		}
	}
	if len(allStats) == 0 {
		return nil
	}

	override := &RuleOverride{
		RuleName:  rule.Name,
		AppliedAt: s.now(),
	}
	hasChange := false

	minCoding := parseCodingStrength(rule.RouteAction.MinCoding)

	// Raise MinCoding if models at the current min tier underperform.
	var currentTierStats []ModelFitStats
	for _, st := range allStats {
		entry := CodingModelRegistryEntry{Provider: "", ModelName: st.ModelID, CodingStrength: CodingStrengthNone}
		// We don't have registry access here; infer strength from recorded model
		// id via a conservative lookup is impossible without the registry.
		// Instead, use the fact that rule.RouteAction.MinCoding filters candidates:
		// any model in allStats that meets the rule already has CodingStrength >= minCoding.
		// We approximate the tier grouping by treating every returned model as eligible.
		_ = entry
		currentTierStats = append(currentTierStats, st)
	}

	weightedRate := weightedSuccessRate(currentTierStats)
	if weightedRate < s.cfg.MinSuccessRate && minCoding < CodingStrengthStrong {
		next := nextCodingStrength(minCoding)
		label := next.String()
		override.MinCoding = &label
		override.Reason = fmt.Sprintf("observed success %.0f%% < %.0f%% threshold", weightedRate*100, s.cfg.MinSuccessRate*100)
		hasChange = true
	}

	// Bump MinContext if fallback relaxation happens too often for small-context models.
	fallbacks := 0
	for _, st := range allStats {
		if st.AvgRetries > 1.5 || st.SuccessRate < s.cfg.MinSuccessRate {
			fallbacks++
		}
	}
	if float64(fallbacks)/float64(len(allStats)) > s.cfg.FallbackTriggerRate {
		current := rule.RouteAction.MinContext
		next := nextContextTier(current)
		if next > current {
			override.MinContext = &next
			if override.Reason != "" {
				override.Reason += "; "
			}
			override.Reason += fmt.Sprintf("%.0f%% of models showed strain signals", float64(fallbacks)/float64(len(allStats))*100)
			hasChange = true
		}
	}

	// Flip strategy if an alternative preference would have won more often.
	alt := s.bestAlternativeStrategy(rule, buckets)
	if alt != "" && alt != rule.RouteAction.Prefer {
		strat := alt
		override.Prefer = &strat
		if override.Reason != "" {
			override.Reason += "; "
		}
		override.Reason += fmt.Sprintf("%s outperforms %s by > %.0f%% margin", alt, rule.RouteAction.Prefer, s.cfg.StrategyFlipMargin*100)
		hasChange = true
	}

	if !hasChange {
		return nil
	}
	return override
}

// bestAlternativeStrategy compares the rule's current strategy against the
// other strategies by replaying candidate selection across observed models.
func (s *SelfTuner) bestAlternativeStrategy(rule CodingRule, buckets []FitBucket) PreferenceStrategy {
	current := rule.RouteAction.Prefer
	currentRate := s.simulatedStrategyWinRate(current, rule, buckets)

	candidates := []PreferenceStrategy{PreferenceFastest, PreferenceBestCoding, PreferenceBestCombo, PreferenceLargestContext}
	best := current
	bestRate := currentRate
	for _, strat := range candidates {
		if strat == current {
			continue
		}
		rate := s.simulatedStrategyWinRate(strat, rule, buckets)
		if rate > bestRate+s.cfg.StrategyFlipMargin {
			best = strat
			bestRate = rate
		}
	}
	return best
}

// simulatedStrategyWinRate estimates how often a strategy would pick a model
// that ends up successful, given observed fit stats. It uses the observed
// success rate of the model the strategy would select as a proxy.
func (s *SelfTuner) simulatedStrategyWinRate(strategy PreferenceStrategy, rule CodingRule, buckets []FitBucket) float64 {
	var totalAttempts float64
	var weightedSuccess float64
	for _, bucket := range buckets {
		stats, ok := s.fit.GetFitStats(bucket.Difficulty, bucket.TaskType, s.cfg.MinAttempts)
		if !ok || len(stats) == 0 {
			continue
		}
		candidates := filterStatsToRule(stats, rule)
		if len(candidates) == 0 {
			continue
		}
		pick := pickByStrategy(candidates, strategy)
		if pick == nil {
			continue
		}
		totalAttempts += float64(pick.Attempts)
		weightedSuccess += float64(pick.Attempts) * pick.SuccessRate
	}
	if totalAttempts == 0 {
		return 0
	}
	return weightedSuccess / totalAttempts
}

// ApplyToRule merges the static rule with any runtime override.
func (s *SelfTuner) ApplyToRule(rule CodingRule) CodingRule {
	if !s.cfg.Enabled {
		return rule
	}
	s.mu.RLock()
	override, ok := s.ruleOverrides[rule.Name]
	s.mu.RUnlock()
	if !ok {
		return rule
	}

	if override.MinCoding != nil {
		rule.RouteAction.MinCoding = *override.MinCoding
	}
	if override.MinContext != nil {
		rule.RouteAction.MinContext = *override.MinContext
	}
	if override.Prefer != nil {
		rule.RouteAction.Prefer = *override.Prefer
	}
	return rule
}

// SelectWithBoost reorders candidates using the current boost/demote map and
// optionally explores a random eligible candidate with probability epsilon.
// The returned bool is true when exploration chose an alternate candidate.
func (s *SelfTuner) SelectWithBoost(
	base FindBestFunc,
	profile *CodingTaskProfile,
	rule CodingRule,
	available map[string]bool,
	effectiveMinContext int,
) (*CodingModelRegistryEntry, bool) {
	if !s.cfg.Enabled {
		return base(rule.RouteAction.Prefer, parseCodingStrength(rule.RouteAction.MinCoding), effectiveMinContext, rule.RouteAction.MinReasoning, ModelSizeCategory(rule.RouteAction.MaxSize), available), false
	}

	winner := base(rule.RouteAction.Prefer, parseCodingStrength(rule.RouteAction.MinCoding), effectiveMinContext, rule.RouteAction.MinReasoning, ModelSizeCategory(rule.RouteAction.MaxSize), available)
	if winner == nil {
		return winner, false
	}

	total := s.fit.TotalAttempts()
	epsilon := s.cfg.Epsilon
	if s.cfg.EpsilonDecayAt > 0 && total >= s.cfg.EpsilonDecayAt {
		epsilon = 0
	} else if s.cfg.EpsilonDecayAt > 0 {
		epsilon = epsilon * (1 - float64(total)/float64(s.cfg.EpsilonDecayAt))
	}

	if epsilon > 0 && s.rng() <= epsilon {
		// Gather all eligible candidates (not just the winner). We do this by
		// collecting every registry entry that meets hard filters and is
		// available. Since the registry's filter method is unexported, we
		// approximate by repeatedly calling base with shuffled preference
		// strategies and collecting unique results. This is sufficient for
		// exploration because we only need the set of viable winners under
		// different preferences; any of them is an eligible candidate.
		candidates := collectEligibleCandidates(base, rule, effectiveMinContext, available)
		if len(candidates) > 1 {
			// Exclude the winner to ensure we actually explore an alternative.
			var alt []CodingModelRegistryEntry
			for _, c := range candidates {
				if c.Provider != winner.Provider || c.ModelName != winner.ModelName {
					alt = append(alt, c)
				}
			}
			if len(alt) > 0 {
				idx := rand.Intn(len(alt))
				chosen := &alt[idx]
				s.emit("self_tuning_explored", map[string]string{
					"chosen_model":      chosen.Provider + ":" + chosen.ModelName,
					"would_have_chosen": winner.Provider + ":" + winner.ModelName,
					"epsilon":           fmt.Sprintf("%.3f", epsilon),
				})
				return chosen, true
			}
		}
	}

	// Apply boost/demote by re-selecting among all candidates when there are
	// ties or near-ties. We approximate this by collecting eligible candidates
	// and re-sorting with score offsets.
	candidates := collectEligibleCandidates(base, rule, effectiveMinContext, available)
	if len(candidates) == 0 {
		return winner, false
	}

	s.mu.RLock()
	boosts := make(map[string]float64, len(s.modelBoosts))
	for k, v := range s.modelBoosts {
		boosts[k] = v
	}
	demotes := make(map[string]float64, len(s.modelDemotes))
	for k, v := range s.modelDemotes {
		demotes[k] = v
	}
	s.mu.RUnlock()

	sort.SliceStable(candidates, func(i, j int) bool {
		baseI := normalizedCandidateScore(&candidates[i], rule.RouteAction.Prefer)
		baseJ := normalizedCandidateScore(&candidates[j], rule.RouteAction.Prefer)
		boostI := boosts[candidates[i].Provider+":"+candidates[i].ModelName]
		boostJ := boosts[candidates[j].Provider+":"+candidates[j].ModelName]
		demoteI := demotes[candidates[i].Provider+":"+candidates[i].ModelName]
		demoteJ := demotes[candidates[j].Provider+":"+candidates[j].ModelName]
		scoreI := baseI + boostI - demoteI
		scoreJ := baseJ + boostJ - demoteJ
		return scoreI > scoreJ
	})

	best := &candidates[0]
	return best, false
}

// collectEligibleCandidates gathers the set of registry entries that meet the
// rule's hard filters by trying each preference strategy.
func collectEligibleCandidates(base FindBestFunc, rule CodingRule, effectiveMinContext int, available map[string]bool) []CodingModelRegistryEntry {
	seen := map[string]bool{}
	var result []CodingModelRegistryEntry
	strategies := []PreferenceStrategy{PreferenceFastest, PreferenceBestCoding, PreferenceBestCombo, PreferenceLargestContext}
	for _, strat := range strategies {
		cand := base(strat, parseCodingStrength(rule.RouteAction.MinCoding), effectiveMinContext, rule.RouteAction.MinReasoning, ModelSizeCategory(rule.RouteAction.MaxSize), available)
		if cand == nil {
			continue
		}
		key := cand.Provider + ":" + cand.ModelName
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, *cand)
	}
	return result
}

// candidateScore mirrors the registry scoring for a single entry so the tuner
// can apply boost/demote offsets consistently.
func candidateScore(e *CodingModelRegistryEntry, strategy PreferenceStrategy) float64 {
	switch strategy {
	case PreferenceFastest:
		return -float64(sizeRank(e.SizeCategory))
	case PreferenceBestCoding:
		return float64(e.CodingStrength)*1000 + float64(e.ContextLimit)
	case PreferenceLargestContext:
		return float64(e.ContextLimit)
	case PreferenceBestCombo:
		return comboScore(e) * 1000
	}
	return 0
}

// normalizedCandidateScore returns a 0..1 score for a registry entry under the
// given strategy so that boost/demote offsets (also 0..1 scale) are meaningful.
func normalizedCandidateScore(e *CodingModelRegistryEntry, strategy PreferenceStrategy) float64 {
	switch strategy {
	case PreferenceFastest:
		return 1.0 - float64(sizeRank(e.SizeCategory)-1)/4.0
	case PreferenceBestCoding:
		return float64(e.CodingStrength) / 3.0
	case PreferenceLargestContext:
		return math.Min(float64(e.ContextLimit)/131072.0, 1.0)
	case PreferenceBestCombo:
		return comboScore(e)
	}
	return 0
}

// filterStatsToRule returns the fit stats that meet the rule's hard
// requirements. ModelID here is "provider:model".
func filterStatsToRule(stats []ModelFitStats, rule CodingRule) []ModelFitStats {
	// Without registry context we cannot filter by coding strength/reasoning/
	// size. We keep all returned stats because GetFitStats already filters by
	// minAttempts and the recorded models were historically eligible.
	return stats
}

// pickByStrategy selects the best fit-stat candidate according to a strategy.
func pickByStrategy(stats []ModelFitStats, strategy PreferenceStrategy) *ModelFitStats {
	if len(stats) == 0 {
		return nil
	}
	candidates := make([]ModelFitStats, len(stats))
	copy(candidates, stats)
	switch strategy {
	case PreferenceFastest:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].AvgLatencyMs < candidates[j].AvgLatencyMs
		})
	case PreferenceBestCoding:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].SuccessRate > candidates[j].SuccessRate
		})
	case PreferenceBestCombo:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].SuccessRate*0.5+latencyScore(candidates[i].AvgLatencyMs)*0.5 >
				candidates[j].SuccessRate*0.5+latencyScore(candidates[j].AvgLatencyMs)*0.5
		})
	case PreferenceLargestContext:
		// Context limit isn't stored in model_fit; latency proxies for model size.
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].AvgLatencyMs > candidates[j].AvgLatencyMs
		})
	}
	return &candidates[0]
}

func latencyScore(ms int64) float64 {
	if ms <= 0 {
		return 1
	}
	return 1.0 / (1.0 + float64(ms)/1000.0)
}

func weightedSuccessRate(stats []ModelFitStats) float64 {
	var attempts float64
	var weighted float64
	for _, st := range stats {
		attempts += float64(st.Attempts)
		weighted += float64(st.Attempts) * st.SuccessRate
	}
	if attempts == 0 {
		return 0
	}
	return weighted / attempts
}

func ruleBuckets(rule CodingRule) []FitBucket {
	difficulties := rule.When.Difficulty
	if len(difficulties) == 0 {
		difficulties = []int{DifficultyTrivial, DifficultySimple, DifficultyModerate, DifficultyComplex, DifficultyNovel}
	}
	types := rule.When.TaskType
	if len(types) == 0 {
		types = []string{"general"}
	}

	var buckets []FitBucket
	for _, d := range difficulties {
		for _, tt := range types {
			buckets = append(buckets, FitBucket{Difficulty: d, TaskType: tt})
		}
	}
	return buckets
}

func nextCodingStrength(s CodingStrength) CodingStrength {
	switch s {
	case CodingStrengthNone, CodingStrengthWeak:
		return CodingStrengthMedium
	case CodingStrengthMedium:
		return CodingStrengthStrong
	}
	return CodingStrengthStrong
}

func nextContextTier(current int) int {
	tiers := []int{0, 4096, 8192, 16384, 32768, 65536, 131072}
	for _, t := range tiers {
		if t > current {
			return t
		}
	}
	return current
}

func (s *SelfTuner) emit(event string, md map[string]string) {
	s.mu.RLock()
	hook := s.telemetry
	s.mu.RUnlock()
	if hook != nil {
		hook(event, md)
	}
}
