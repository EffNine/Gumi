package scorer

import (
	"math"
	"testing"
)

func TestCohenD_Identical(t *testing.T) {
	direct := MetricSet{Mean: 0.5, Std: 0.1, N: 100}
	novexa := MetricSet{Mean: 0.5, Std: 0.1, N: 100}
	d := CohenD(direct, novexa)
	if d != 0 {
		t.Errorf("identical sets: CohenD = %v, want 0", d)
	}
}

func TestCohenD_LargePositive(t *testing.T) {
	direct := MetricSet{Mean: 0.2, Std: 0.1, N: 50}
	novexa := MetricSet{Mean: 0.9, Std: 0.1, N: 50}
	d := CohenD(direct, novexa)
	if d < 0.8 {
		t.Errorf("large delta: CohenD = %v, want >= 0.8 (★★★)", d)
	}
}

func TestCohenD_SmallDelta(t *testing.T) {
	// Very small difference should produce d < 0.2
	direct := MetricSet{Mean: 0.50, Std: 0.5, N: 100}
	novexa := MetricSet{Mean: 0.51, Std: 0.5, N: 100}
	d := CohenD(direct, novexa)
	if d >= 0.2 {
		t.Errorf("small delta: CohenD = %v, want < 0.2 (—)", d)
	}
}

func TestCohenD_NegativeDelta(t *testing.T) {
	direct := MetricSet{Mean: 0.8, Std: 0.15, N: 30}
	novexa := MetricSet{Mean: 0.3, Std: 0.15, N: 30}
	d := CohenD(direct, novexa)
	if d >= 0 {
		t.Errorf("negative delta: CohenD = %v, want < 0", d)
	}
}

func TestCohenD_SmallSamples(t *testing.T) {
	direct := MetricSet{Mean: 0.5, Std: 0.1, N: 1}
	novexa := MetricSet{Mean: 0.8, Std: 0.1, N: 1}
	d := CohenD(direct, novexa)
	if d != 0 {
		t.Errorf("small samples (N=1): CohenD = %v, want 0", d)
	}
}

func TestCohenD_ZeroStd(t *testing.T) {
	direct := MetricSet{Mean: 0.5, Std: 0, N: 10}
	novexa := MetricSet{Mean: 0.8, Std: 0, N: 10}
	d := CohenD(direct, novexa)
	if d != 0 {
		t.Errorf("zero pooled std: CohenD = %v, want 0", d)
	}
}

func TestCohenD_UnevenSizes(t *testing.T) {
	direct := MetricSet{Mean: 0.3, Std: 0.2, N: 100}
	novexa := MetricSet{Mean: 0.7, Std: 0.25, N: 20}
	d := CohenD(direct, novexa)
	if d < 0.5 || d > 3.0 {
		t.Errorf("uneven sizes: CohenD = %v, want roughly in (0.5, 3.0)", d)
	}
}

func TestEffectStars(t *testing.T) {
	tests := []struct {
		d    float64
		want string
	}{
		{d: 0.0, want: "—"},
		{d: 0.1, want: "—"},
		{d: 0.19, want: "—"},
		{d: 0.2, want: "★"},
		{d: 0.3, want: "★"},
		{d: 0.49, want: "★"},
		{d: 0.5, want: "★★"},
		{d: 0.6, want: "★★"},
		{d: 0.79, want: "★★"},
		{d: 0.8, want: "★★★"},
		{d: 1.0, want: "★★★"},
		{d: 2.0, want: "★★★"},
		{d: -0.1, want: "—"},
		{d: -0.5, want: "—"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := EffectStars(tt.d)
			if got != tt.want {
				t.Errorf("EffectStars(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

// Additional edge cases for CohenD

func TestCohenD_ExactFormula(t *testing.T) {
	// Known values: direct mean=0, std=1, N=100; novexa mean=0.5, std=1, N=100
	// Pooled variance = ((99*1) + (99*1)) / 198 = 1
	// Pooled std = 1
	// Cohen's d = (0.5 - 0) / 1 = 0.5
	direct := MetricSet{Mean: 0, Std: 1, N: 100}
	novexa := MetricSet{Mean: 0.5, Std: 1, N: 100}
	d := CohenD(direct, novexa)
	if math.Abs(d-0.5) > 1e-10 {
		t.Errorf("CohenD = %v, want 0.5", d)
	}
}
