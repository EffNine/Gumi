package scorer

import "math"

// CohenD computes Cohen's d effect size between two groups.
// A positive value indicates the novexa group has a higher mean than the direct group.
func CohenD(direct, novexa MetricSet) float64 {
	if direct.N < 2 || novexa.N < 2 {
		return 0
	}

	pooledVar := (float64(direct.N-1)*direct.Std*direct.Std +
		float64(novexa.N-1)*novexa.Std*novexa.Std) /
		float64(direct.N+novexa.N-2)

	pooledStd := math.Sqrt(pooledVar)
	if pooledStd == 0 {
		return 0
	}

	return (novexa.Mean - direct.Mean) / pooledStd
}

// EffectStars returns a star rating string for a Cohen's d value.
//
//	| d range       | Rating |
//	|---------------|--------|
//	| d < 0.2       | —      |
//	| 0.2 ≤ d < 0.5 | ★      |
//	| 0.5 ≤ d < 0.8 | ★★     |
//	| d ≥ 0.8       | ★★★    |
func EffectStars(d float64) string {
	if d < 0.2 {
		return "—"
	}
	if d < 0.5 {
		return "★"
	}
	if d < 0.8 {
		return "★★"
	}
	return "★★★"
}
