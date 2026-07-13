package scorer

import "math"

// ModelWeights defines the weighting of each capability in the overall score
// for a given model tier.
type ModelWeights struct {
	JSON        float64
	Instruction float64
	Repetition  float64
	ToolCalling float64
	Reasoning   float64
	Degradation float64
	LatencyCost float64 // subtracted, not weighted
}

// WeightsByTier maps model tiers to their capability weight profiles.
var WeightsByTier = map[string]ModelWeights{
	"small": {
		JSON:        0.35,
		Instruction: 0.25,
		ToolCalling: 0.15,
		Reasoning:   0.10,
		Repetition:  0.10,
		Degradation: 0.05,
		LatencyCost: 0.05,
	},
	"medium": {
		JSON:        0.25,
		Instruction: 0.25,
		ToolCalling: 0.20,
		Reasoning:   0.15,
		Repetition:  0.10,
		Degradation: 0.10,
		LatencyCost: 0.10,
	},
	"frontier": {
		JSON:        0.05,
		Instruction: 0.05,
		ToolCalling: 0.10,
		Reasoning:   0.05,
		Repetition:  0.05,
		Degradation: 0.50,
		LatencyCost: 0.20,
	},
}

// Capability holds the direct and novexa metric sets for a single capability.
type Capability struct {
	Direct     MetricSet
	Novexa     MetricSet
	Delta      float64
	EffectSize float64
}

// OverallScore computes the adaptive overall score from per-capability deltas,
// degradation metrics, and latency overhead.
//
// The formula is a weighted sum of capability deltas minus a latency penalty.
// Returns the overall score and whether Novexa is "worth it" (> 5% improvement).
func OverallScore(caps map[string]Capability, degrad DegradationReport, latencyOverhead float64, tier string) (float64, bool) {
	w, ok := WeightsByTier[tier]
	if !ok {
		w = WeightsByTier["medium"]
	}

	weightedSum :=
		caps["json"].Delta*w.JSON +
			caps["instruction"].Delta*w.Instruction +
			caps["repetition"].Delta*w.Repetition +
			caps["tool_calling"].Delta*w.ToolCalling +
			caps["reasoning"].Delta*w.Reasoning +
			(1-degrad.DegradationRate)*w.Degradation

	latencyPenalty := math.Min(latencyOverhead/1000.0, w.LatencyCost)
	score := weightedSum - latencyPenalty
	worthIt := score > 0.05

	return math.Round(score*10000) / 10000, worthIt
}
