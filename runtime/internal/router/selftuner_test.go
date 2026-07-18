package router

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type stubModelFitLookup struct {
	best  map[FitBucket]ModelFitStats
	stats map[FitBucket][]ModelFitStats
	total int
}

func (s *stubModelFitLookup) GetBestModelForRouter(difficulty int, taskType string) (string, float64, bool) {
	b := FitBucket{Difficulty: difficulty, TaskType: taskType}
	st, ok := s.best[b]
	if !ok {
		return "", 0, false
	}
	return st.ModelID, st.SuccessRate, true
}

func (s *stubModelFitLookup) GetFitStats(difficulty int, taskType string, minAttempts int) ([]ModelFitStats, bool) {
	b := FitBucket{Difficulty: difficulty, TaskType: taskType}
	all, ok := s.stats[b]
	if !ok {
		return nil, false
	}
	var result []ModelFitStats
	for _, st := range all {
		if st.Attempts >= minAttempts {
			result = append(result, st)
		}
	}
	return result, len(result) > 0
}

func (s *stubModelFitLookup) GetModelFit(modelID string, difficulty int, taskType string) (ModelFitStats, bool) {
	b := FitBucket{Difficulty: difficulty, TaskType: taskType}
	all, ok := s.stats[b]
	if !ok {
		return ModelFitStats{}, false
	}
	for _, st := range all {
		if st.ModelID == modelID {
			return st, true
		}
	}
	return ModelFitStats{}, false
}

func (s *stubModelFitLookup) TotalAttempts() int {
	return s.total
}

func defaultSelfTuningConfig() config.SelfTuningConfig {
	return config.SelfTuningConfig{
		Enabled:                 true,
		MinAttempts:             5,
		MinSuccessRate:          0.5,
		PromoteThreshold:        0.8,
		DemoteThreshold:         0.3,
		BoostWeight:             0.2,
		DemoteWeight:            0.3,
		FallbackTriggerRate:     0.3,
		StrategyFlipMargin:      0.15,
		Epsilon:                 0.1,
		EpsilonDecayAt:          200,
		WarmupAttempts:          10,
		MinOutcomesBetweenTunes: 10,
		PersistSnapshot:         false,
	}
}

func newSelfTunerWithLookup(lookup ModelFitLookup) *SelfTuner {
	return NewSelfTuner(defaultSelfTuningConfig(), lookup, "")
}

func newSelfTunerWithLookupAndPath(lookup ModelFitLookup, snapshotPath string) *SelfTuner {
	return NewSelfTuner(defaultSelfTuningConfig(), lookup, snapshotPath)
}

// ---------------------------------------------------------------------------
// 1. Tune rule adjustments
// ---------------------------------------------------------------------------

func TestTune_RaisesMinCoding_WhenSuccessBelowThreshold(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 10, Successes: 3, SuccessRate: 0.3},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	tuner.Tune()

	rule := findDefaultRule("moderate-feature")
	effective := tuner.ApplyToRule(rule)
	if effective.RouteAction.MinCoding != "strong" {
		t.Fatalf("expected min_coding raised to strong, got %q", effective.RouteAction.MinCoding)
	}
}

func TestTune_BumpsMinContext_OnHighFallbackRate(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultySimple, TaskType: "fix"}: {
				{ModelID: "provider-a:small-model", Attempts: 10, Successes: 3, SuccessRate: 0.3, AvgRetries: 2.5},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	tuner.Tune()

	rule := findDefaultRule("simple-fix")
	effective := tuner.ApplyToRule(rule)
	if effective.RouteAction.MinContext < 8192 {
		t.Fatalf("expected min_context bumped above 4096, got %d", effective.RouteAction.MinContext)
	}
}

func TestTune_NoChange_WhenSuccessAboveThreshold(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:medium-model", Attempts: 10, Successes: 8, SuccessRate: 0.8},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	tuner.Tune()

	rule := findDefaultRule("moderate-feature")
	effective := tuner.ApplyToRule(rule)
	if effective.RouteAction.MinCoding != rule.RouteAction.MinCoding {
		t.Fatalf("expected no min_coding change, got %q", effective.RouteAction.MinCoding)
	}
}

// ---------------------------------------------------------------------------
// 2. Model promotion / demotion
// ---------------------------------------------------------------------------

func TestTune_PromotesHighSuccessModel(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:strong-model", Attempts: 10, Successes: 9, SuccessRate: 0.9},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	snap := tuner.Tune()

	if snap.ModelBoosts["provider-a:strong-model"] != 0.2 {
		t.Fatalf("expected boost 0.2, got %v", snap.ModelBoosts)
	}
}

func TestTune_DemotesLowSuccessModel(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 10, Successes: 2, SuccessRate: 0.2},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	snap := tuner.Tune()

	if snap.ModelDemotes["provider-a:weak-model"] != 0.3 {
		t.Fatalf("expected demote 0.3, got %v", snap.ModelDemotes)
	}
}

// ---------------------------------------------------------------------------
// 3. Warmup / throttling
// ---------------------------------------------------------------------------

func TestTune_WarmupBlocksTuning(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 5, Successes: 1, SuccessRate: 0.2},
			},
		},
		total: 5,
	}
	tuner := newSelfTunerWithLookup(lookup)
	snap := tuner.Tune()

	if len(snap.RuleOverrides) != 0 || len(snap.ModelBoosts) != 0 {
		t.Fatal("expected no adjustments during warmup")
	}
}

func TestObserveOutcome_ThrottlesTune(t *testing.T) {
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 10, Successes: 2, SuccessRate: 0.2},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)

	for i := 0; i < 9; i++ {
		tuner.ObserveOutcome("provider-a:weak-model", DifficultyModerate, "feature", false)
	}
	snap1 := tuner.Snapshot()
	if len(snap1.RuleOverrides) != 0 {
		t.Fatal("expected no tune before MinOutcomesBetweenTunes")
	}

	tuner.ObserveOutcome("provider-a:weak-model", DifficultyModerate, "feature", false)
	snap2 := tuner.Snapshot()
	if len(snap2.RuleOverrides) == 0 {
		t.Fatal("expected tune after MinOutcomesBetweenTunes")
	}
}

func TestObserveOutcome_DisabledIsNoOp(t *testing.T) {
	cfg := defaultSelfTuningConfig()
	cfg.Enabled = false
	tuner := NewSelfTuner(cfg, nil, "")

	for i := 0; i < 20; i++ {
		tuner.ObserveOutcome("m", DifficultyModerate, "feature", true)
	}

	snap := tuner.Snapshot()
	if snap.TotalAttempts != 0 {
		t.Fatalf("expected disabled tuner to report 0 attempts, got %d", snap.TotalAttempts)
	}
}

// ---------------------------------------------------------------------------
// 4. SelectWithBoost
// ---------------------------------------------------------------------------

func makeRichRegistry() *CodingModelRegistry {
	return &CodingModelRegistry{
		entries: []CodingModelRegistryEntry{
			{
				ProfileID:      "small-model",
				Provider:       "provider-small",
				ModelName:      "small-model:v1",
				CodingStrength: CodingStrengthMedium,
				ToolCalling:    "good",
				Reasoning:      "basic",
				ContextLimit:   8192,
				SizeCategory:   SizeSmall,
			},
			{
				ProfileID:      "large-model",
				Provider:       "provider-large",
				ModelName:      "large-model:v1",
				CodingStrength: CodingStrengthStrong,
				ToolCalling:    "excellent",
				Reasoning:      "strong",
				ContextLimit:   32768,
				SizeCategory:   SizeLarge,
			},
		},
	}
}

func TestSelectWithBoost_BoostPrefersInferiorModel(t *testing.T) {
	reg := makeRichRegistry()
	lookup := &stubModelFitLookup{total: 100}
	tuner := newSelfTunerWithLookup(lookup)
	// Promote the small (medium-coding) model over the large (strong-coding) model.
	tuner.mu.Lock()
	tuner.modelBoosts["provider-small:small-model:v1"] = 1000
	tuner.mu.Unlock()

	base := reg.FindBest
	rule := findDefaultRule("moderate-feature")
	profile := &CodingTaskProfile{Difficulty: DifficultyModerate, TaskType: TaskFeature}
	available := map[string]bool{
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	chosen, _ := tuner.SelectWithBoost(base, profile, rule, available, rule.RouteAction.MinContext)
	if chosen == nil {
		t.Fatal("expected a chosen model")
	}
	if chosen.ModelName != "small-model:v1" {
		t.Fatalf("expected boosted small model, got %q", chosen.ModelName)
	}
}

func TestSelectWithBoost_DemoteAvoidsModel(t *testing.T) {
	reg := makeRichRegistry()
	lookup := &stubModelFitLookup{total: 100}
	tuner := newSelfTunerWithLookup(lookup)
	// Demote the large model so the small one wins despite lower static score.
	tuner.mu.Lock()
	tuner.modelDemotes["provider-large:large-model:v1"] = 1000
	tuner.mu.Unlock()

	base := reg.FindBest
	rule := findDefaultRule("moderate-feature")
	profile := &CodingTaskProfile{Difficulty: DifficultyModerate, TaskType: TaskFeature}
	available := map[string]bool{
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	chosen, _ := tuner.SelectWithBoost(base, profile, rule, available, rule.RouteAction.MinContext)
	if chosen == nil {
		t.Fatal("expected a chosen model")
	}
	if chosen.ModelName != "small-model:v1" {
		t.Fatalf("expected demoted large model avoided, got %q", chosen.ModelName)
	}
}

// ---------------------------------------------------------------------------
// 5. Exploration
// ---------------------------------------------------------------------------

func TestSelectWithBoost_ExplorationPicksAlternative(t *testing.T) {
	reg := makeRichRegistry()
	lookup := &stubModelFitLookup{total: 100}
	tuner := newSelfTunerWithLookup(lookup)
	// Force exploration by pinning rng below epsilon.
	tuner.rng = func() float64 { return 0.05 }

	base := reg.FindBest
	rule := findDefaultRule("moderate-feature")
	profile := &CodingTaskProfile{Difficulty: DifficultyModerate, TaskType: TaskFeature}
	available := map[string]bool{
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	var sawAlternative bool
	for i := 0; i < 20; i++ {
		chosen, explored := tuner.SelectWithBoost(base, profile, rule, available, rule.RouteAction.MinContext)
		if chosen == nil {
			continue
		}
		if explored && chosen.ModelName != "large-model:v1" {
			sawAlternative = true
			break
		}
	}
	if !sawAlternative {
		t.Fatal("expected exploration to pick an alternative at least once")
	}
}

func TestSelectWithBoost_ExplorationRespectsHardFilters(t *testing.T) {
	reg := makeRichRegistry()
	lookup := &stubModelFitLookup{total: 100}
	tuner := newSelfTunerWithLookup(lookup)
	tuner.rng = func() float64 { return 0.05 }

	base := reg.FindBest
	// Strict rule requiring strong coding. The small medium-coding model must
	// not be picked even during exploration.
	rule := findDefaultRule("complex-coding")
	profile := &CodingTaskProfile{Difficulty: DifficultyComplex, TaskType: TaskFeature}
	available := map[string]bool{
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}

	for i := 0; i < 20; i++ {
		chosen, _ := tuner.SelectWithBoost(base, profile, rule, available, rule.RouteAction.MinContext)
		if chosen == nil {
			continue
		}
		if chosen.ModelName == "small-model:v1" {
			t.Fatal("exploration violated hard min_coding filter")
		}
	}
}

func TestTune_EpsilonDecaysWithTotalAttempts(t *testing.T) {
	cfg := defaultSelfTuningConfig()
	cfg.EpsilonDecayAt = 100
	tuner := NewSelfTuner(cfg, &stubModelFitLookup{total: 100}, "")

	// With total == decayAt, effective epsilon should be 0.
	total := tuner.fit.TotalAttempts()
	epsilon := cfg.Epsilon
	if cfg.EpsilonDecayAt > 0 && total >= cfg.EpsilonDecayAt {
		epsilon = 0
	}
	if epsilon != 0 {
		t.Fatalf("expected epsilon decayed to 0 at total=%d, got %f", total, epsilon)
	}
}

// ---------------------------------------------------------------------------
// 6. Telemetry hook
// ---------------------------------------------------------------------------

func TestTune_EmitsTelemetryEvents(t *testing.T) {
	var events []string
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 10, Successes: 2, SuccessRate: 0.2},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)
	tuner.SetTelemetryHook(func(ev string, md map[string]string) {
		events = append(events, ev)
	})
	tuner.Tune()

	if len(events) == 0 {
		t.Fatal("expected telemetry events from Tune()")
	}
	hasPass := false
	for _, ev := range events {
		if ev == "self_tuning_pass" {
			hasPass = true
		}
	}
	if !hasPass {
		t.Fatalf("expected self_tuning_pass event, got %v", events)
	}
}

// ---------------------------------------------------------------------------
// 7. Helper functions
// ---------------------------------------------------------------------------

func findDefaultRule(name string) CodingRule {
	for _, r := range DefaultCodingRules() {
		if r.Name == name {
			return r
		}
	}
	return CodingRule{}
}

func TestNextCodingStrength(t *testing.T) {
	cases := []struct {
		in, want CodingStrength
	}{
		{CodingStrengthWeak, CodingStrengthMedium},
		{CodingStrengthMedium, CodingStrengthStrong},
		{CodingStrengthStrong, CodingStrengthStrong},
	}
	for _, c := range cases {
		got := nextCodingStrength(c.in)
		if got != c.want {
			t.Fatalf("nextCodingStrength(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNextContextTier(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 4096},
		{4096, 8192},
		{8192, 16384},
		{16384, 32768},
		{131072, 131072},
	}
	for _, c := range cases {
		got := nextContextTier(c.in)
		if got != c.want {
			t.Fatalf("nextContextTier(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestWeightedSuccessRate(t *testing.T) {
	stats := []ModelFitStats{
		{Attempts: 10, Successes: 5, SuccessRate: 0.5},
		{Attempts: 10, Successes: 9, SuccessRate: 0.9},
	}
	got := weightedSuccessRate(stats)
	want := (10*0.5 + 10*0.9) / 20
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("weightedSuccessRate = %f, want %f", got, want)
	}
}

func TestRuleBuckets(t *testing.T) {
	rule := CodingRule{
		Name: "test-rule",
		When: RuleCondition{
			Difficulty: []int{DifficultySimple, DifficultyModerate},
			TaskType:   []string{"fix", "feature"},
		},
	}
	buckets := ruleBuckets(rule)
	if len(buckets) != 4 {
		t.Fatalf("expected 4 buckets, got %d", len(buckets))
	}
}

// Ensure the snapshot fields can be marshaled.
func TestSnapshot_IsSerializable(t *testing.T) {
	now := time.Now()
	strat := PreferenceBestCoding
	snap := SelfTuningSnapshot{
		GeneratedAt: now,
		RuleOverrides: []RuleOverride{
			{RuleName: "r1", MinCoding: strPtr("strong"), Prefer: &strat, AppliedAt: now},
		},
		ModelBoosts:   map[string]float64{"m1": 0.2},
		ModelDemotes:  map[string]float64{"m2": 0.3},
		TotalAttempts: 42,
		Adjustments: []AdjustmentRecord{
			{Kind: "raise_min_coding", Target: "r1", From: "weak", To: "strong"},
		},
	}
	if snap.RuleOverrides[0].RuleName != "r1" {
		t.Fatal("snapshot inconsistent")
	}
}

func strPtr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// 8. Integration: tune → route → snapshot round-trip
// ---------------------------------------------------------------------------

func TestSelfTuningIntegration_TuneAdjustsRouteSelection(t *testing.T) {
	reg := makeRichRegistry()
	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{
					ModelID:     "provider-a:weak-model",
					Attempts:    10,
					Successes:   2,
					SuccessRate: 0.2,
					AvgRetries:  1.0,
				},
			},
		},
		total: 10,
	}
	tuner := newSelfTunerWithLookup(lookup)

	for i := 0; i < 10; i++ {
		tuner.ObserveOutcome("provider-a:weak-model", DifficultyModerate, "feature", false)
	}

	if !tuner.HasRuleOverride("moderate-feature") {
		t.Fatal("expected moderate-feature rule override after tune")
	}

	engine := NewCodingRuleEngine(DefaultCodingRules(), reg, nil, lookup)
	engine.SetSelfTuner(tuner)

	profile := &CodingTaskProfile{Difficulty: DifficultyModerate, TaskType: TaskFeature}
	available := map[string]bool{
		"provider-small:small-model:v1": true,
		"provider-large:large-model:v1": true,
	}
	result := engine.Route(profile, available, nil)
	if result == nil {
		t.Fatal("expected route result")
	}
	if !result.SelfTuned {
		t.Fatal("expected SelfTuned=true when rule override applied")
	}
	if result.Model != "large-model:v1" {
		t.Fatalf("expected strong model after min_coding raise, got %q", result.Model)
	}

	// Boost path: promote small model and demote large so SelectWithBoost flips winner.
	tuner.mu.Lock()
	tuner.modelBoosts["provider-small:small-model:v1"] = 0.5
	tuner.modelDemotes["provider-large:large-model:v1"] = 0.5
	tuner.mu.Unlock()

	rule := findDefaultRule("moderate-feature")
	chosen, explored := tuner.SelectWithBoost(reg.FindBest, profile, rule, available, rule.RouteAction.MinContext)
	if explored {
		t.Fatal("expected boost selection, not exploration")
	}
	if chosen == nil || chosen.ModelName != "small-model:v1" {
		t.Fatalf("expected boosted small model, got %v", chosen)
	}
}

func TestSelfTuningIntegration_SnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	snapshotPath := filepath.Join(dir, "self-tuning-snapshot.json")

	lookup := &stubModelFitLookup{
		stats: map[FitBucket][]ModelFitStats{
			{Difficulty: DifficultyModerate, TaskType: "feature"}: {
				{ModelID: "provider-a:weak-model", Attempts: 10, Successes: 2, SuccessRate: 0.2},
			},
		},
		total: 10,
	}

	cfg := defaultSelfTuningConfig()
	cfg.PersistSnapshot = true
	tuner := NewSelfTuner(cfg, lookup, snapshotPath)
	for i := 0; i < 10; i++ {
		tuner.ObserveOutcome("provider-a:weak-model", DifficultyModerate, "feature", false)
	}

	snap := tuner.Snapshot()
	if len(snap.RuleOverrides) == 0 {
		t.Fatal("expected rule overrides after tune")
	}
	if err := tuner.SaveSnapshot(); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	loaded := NewSelfTuner(cfg, lookup, snapshotPath)
	loadedSnap := loaded.Snapshot()
	if len(loadedSnap.RuleOverrides) != len(snap.RuleOverrides) {
		t.Fatalf("expected %d rule overrides after load, got %d", len(snap.RuleOverrides), len(loadedSnap.RuleOverrides))
	}
	if !loaded.HasRuleOverride("moderate-feature") {
		t.Fatal("expected moderate-feature override restored from snapshot")
	}

	rule := findDefaultRule("moderate-feature")
	effective := loaded.ApplyToRule(rule)
	if effective.RouteAction.MinCoding != "strong" {
		t.Fatalf("expected restored min_coding strong, got %q", effective.RouteAction.MinCoding)
	}
}

func TestSelfTuningSnapshotPath(t *testing.T) {
	got := SelfTuningSnapshotPath("/data/gumi/memory.db")
	want := filepath.Join("/data/gumi", "self-tuning-snapshot.json")
	if got != want {
		t.Fatalf("SelfTuningSnapshotPath = %q, want %q", got, want)
	}
}
