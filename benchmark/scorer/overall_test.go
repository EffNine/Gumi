package scorer

import (
	"math"
	"testing"
)

func TestOverallScore_AllPerfect(t *testing.T) {
	caps := map[string]Capability{
		"json":         {Delta: 1.0},
		"instruction":  {Delta: 1.0},
		"repetition":   {Delta: 1.0},
		"tool_calling": {Delta: 1.0},
		"reasoning":    {Delta: 1.0},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	score, worthIt := OverallScore(caps, degrad, 0, "medium")
	// For medium tier: weights sum = 0.25+0.25+0.10+0.20+0.15+0.10 = 1.05
	// All deltas=1.0, degradation rate=0, latency=0
	// score = 1.05 - 0 = 1.05
	if score != 1.05 {
		t.Errorf("all perfect: score=%v, want 1.05", score)
	}
	if !worthIt {
		t.Errorf("all perfect: worthIt should be true")
	}
}

func TestOverallScore_AllZeroDeltas(t *testing.T) {
	caps := map[string]Capability{
		"json":         {Delta: 0},
		"instruction":  {Delta: 0},
		"repetition":   {Delta: 0},
		"tool_calling": {Delta: 0},
		"reasoning":    {Delta: 0},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	score, worthIt := OverallScore(caps, degrad, 0, "medium")
	// Score should be the degradation weight (since all deltas are 0 and degradation rate is 0
	// but with zero latency). medium has Degradation: 0.10.
	// weightedSum = 0*0.25 + 0*0.25 + 0*0.10 + 0*0.20 + 0*0.15 + (1-0)*0.10 = 0.10
	// latencyPenalty = min(0/1000, 0.10) = 0
	// score = 0.10 - 0 = 0.10
	want := 0.10
	if math.Abs(score-want) > 1e-4 {
		t.Errorf("all zero deltas: score=%v, want %v", score, want)
	}
	worthItWant := score > 0.05
	if worthIt != worthItWant {
		t.Errorf("all zero deltas: worthIt=%v, want %v", worthIt, worthItWant)
	}
}

func TestOverallScore_HighDegradation(t *testing.T) {
	caps := map[string]Capability{
		"json":        {Delta: 0.5},
		"instruction": {Delta: 0.5},
	}
	degrad := DegradationReport{DegradationRate: 0.9}
	scorePerfectDegrad := DegradationReport{DegradationRate: 0.0}
	scoreHigh, _ := OverallScore(caps, degrad, 0, "medium")
	scoreLow, _ := OverallScore(caps, scorePerfectDegrad, 0, "medium")
	if scoreHigh >= scoreLow {
		t.Errorf("high degradation should lower score: highDeg=%v, lowDeg=%v", scoreHigh, scoreLow)
	}
}

func TestOverallScore_HighLatency(t *testing.T) {
	caps := map[string]Capability{
		"json": {Delta: 1.0},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	scoreHighLat, _ := OverallScore(caps, degrad, 5000, "medium")
	scoreNoLat, _ := OverallScore(caps, degrad, 0, "medium")
	if scoreHighLat >= scoreNoLat {
		t.Errorf("high latency should lower score: highLat=%v, noLat=%v", scoreHighLat, scoreNoLat)
	}
}

func TestOverallScore_FrontierDegradationWeight(t *testing.T) {
	caps := map[string]Capability{
		"json":        {Delta: 0},
		"instruction": {Delta: 0},
	}
	degrad := DegradationReport{DegradationRate: 0.5}
	scoreFrontier, _ := OverallScore(caps, degrad, 0, "frontier")
	scoreSmall, _ := OverallScore(caps, degrad, 0, "small")
	// Frontier has Degradation: 0.50 weight; Small has 0.05.
	// For frontier: weightedSum = (1-0.5)*0.50 = 0.25
	// For small: weightedSum = (1-0.5)*0.05 = 0.025
	// So frontier score should be higher from degradation alone.
	if scoreFrontier <= scoreSmall {
		t.Errorf("frontier tier should weight degradation more: frontier=%v, small=%v", scoreFrontier, scoreSmall)
	}
}

func TestOverallScore_SmallTierJSONWeight(t *testing.T) {
	caps := map[string]Capability{
		"json": {Delta: 1.0},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	scoreSmall, _ := OverallScore(caps, degrad, 0, "small")
	// Small has JSON: 0.35, Degradation: 0.05
	// Small score = 1.0*0.35 + (1-0)*0.05 = 0.40
	if scoreSmall < 0.39 || scoreSmall > 0.41 {
		t.Errorf("small tier score=%v, want ≈0.40", scoreSmall)
	}
}

func TestOverallScore_UnknownTier(t *testing.T) {
	caps := map[string]Capability{
		"json": {Delta: 0.5},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	score, worthIt := OverallScore(caps, degrad, 0, "nonexistent")
	if score <= 0 {
		t.Errorf("unknown tier should fall back to medium weights; score=%v", score)
	}
	// Medium weight for JSON is 0.25
	// weightedSum = 0.5*0.25 + (1-0)*0.10 = 0.125 + 0.10 = 0.225
	// latencyPenalty = 0
	// score = 0.225
	want := 0.225
	if math.Abs(score-want) > 1e-4 {
		t.Errorf("unknown tier fallback: score=%v, want %v", score, want)
	}
	if !worthIt {
		t.Errorf("unknown tier with positive delta: worthIt should be true")
	}
}

func TestOverallScore_LatencyCappedAtWeight(t *testing.T) {
	caps := map[string]Capability{
		"json": {Delta: 1.0},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	// Medium tier has LatencyCost: 0.10
	// latencyPenalty = min(200/1000, 0.10) = min(0.20, 0.10) = 0.10
	scoreCapped, _ := OverallScore(caps, degrad, 200, "medium")
	scoreUncapped, _ := OverallScore(caps, degrad, 50, "medium")
	// At 50ms: latencyPenalty = min(0.05, 0.10) = 0.05
	// At 200ms: latencyPenalty = min(0.20, 0.10) = 0.10
	// score = weightedSum - latencyPenalty
	if scoreCapped >= scoreUncapped {
		t.Errorf("higher latency should give lower score: capped=%v, uncapped=%v", scoreCapped, scoreUncapped)
	}
}

func TestOverallScore_WorthItThreshold(t *testing.T) {
	caps := map[string]Capability{
		"json": {Delta: 0.01},
	}
	degrad := DegradationReport{DegradationRate: 0.0}
	// weightedSum = 0.01*0.25 + 0.10 = 0.1025 (medium tier)
	// With 60ms latency: penalty = min(0.06, 0.10) = 0.06
	// score = 0.1025 - 0.06 = 0.0425 → worthIt = false (< 0.05)
	score, worthIt := OverallScore(caps, degrad, 60, "medium")
	if worthIt {
		t.Errorf("score=%v should be below worth-it threshold (0.05)", score)
	}
}
