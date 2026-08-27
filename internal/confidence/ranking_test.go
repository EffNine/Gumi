package confidence

import (
	"strings"
	"testing"
)

// near-identical performance: must NOT claim an ordering.
func TestRankIndistinguishable(t *testing.T) {
	a := SampleSet{Decode: []float64{31.0, 31.2, 31.1}, Prefill: []float64{940, 945, 942}}
	b := SampleSet{Decode: []float64{31.1, 30.9, 31.3}, Prefill: []float64{938, 943, 941}}
	r := RankConfidence(a, b)
	if !r.Indistinguishable || r.Level != Low {
		t.Fatalf("level=%s indist=%v note=%q", r.Level, r.Indistinguishable, r.Note)
	}
	if !strings.Contains(r.Note, "indistinguishable") {
		t.Errorf("note should say indistinguishable: %q", r.Note)
	}
}

// clearly separated ranges: HIGH with no-overlap evidence.
func TestRankHigh(t *testing.T) {
	a := SampleSet{Decode: []float64{31.0, 31.4, 31.2}, Prefill: []float64{940, 946, 943}}
	b := SampleSet{Decode: []float64{24.5, 24.9, 24.7}, Prefill: []float64{255, 258, 256}}
	r := RankConfidence(a, b)
	if r.Level != High || r.Indistinguishable {
		t.Fatalf("level=%s indist=%v note=%q", r.Level, r.Indistinguishable, r.Note)
	}
	if !strings.Contains(r.Note, "no overlap") {
		t.Errorf("note should cite non-overlap: %q", r.Note)
	}
}

// means ordered but ranges touch: MEDIUM.
func TestRankMedium(t *testing.T) {
	a := SampleSet{Decode: []float64{31.0, 32.6}, Prefill: []float64{1000, 1100}}
	b := SampleSet{Decode: []float64{29.8, 30.9}, Prefill: []float64{900, 1005}}
	// dec sep=|31.8-30.35|=1.45; noise=max(1.6,1.1)=1.6 → ratio .91 → medium
	// pre sep=1050-952.5=97.5; noise=max(100,105)=105 → ratio .93
	r := RankConfidence(a, b)
	if r.Level != Medium || r.Indistinguishable {
		t.Fatalf("level=%s indist=%v note=%q", r.Level, r.Indistinguishable, r.Note)
	}
}

// metrics disagree within noise: LOW even if each metric alone looks ordered.
func TestRankMetricsDisagree(t *testing.T) {
	a := SampleSet{Decode: []float64{32.0, 33.0}, Prefill: []float64{800, 850}}
	b := SampleSet{Decode: []float64{28.0, 27.4}, Prefill: []float64{1200, 1300}}
	// decode favors a strongly (sep ~4.8, noise 1.0 → high ratio);
	// prefill favors b (sep ~450, noise 100 → ratio 4.5) — both decisive but
	// opposite directions.
	r := RankConfidence(a, b)
	if !r.Indistinguishable || r.Level != Low {
		t.Fatalf("expected LOW indistinguishable on conflicting metrics, got %s (%q)", r.Level, r.Note)
	}
	if !strings.Contains(r.Note, "different candidates") {
		t.Errorf("note should explain conflict: %q", r.Note)
	}
}

// single samples can never support ranking claims.
func TestRankInsufficientRepetitions(t *testing.T) {
	a := SampleSet{Decode: []float64{99.0}}
	b := SampleSet{Decode: []float64{10.0, 10.1}}
	r := RankConfidence(a, b)
	if r.Level != Low || !r.Indistinguishable {
		t.Fatalf("single sample must be LOW/indistinguishable, got %s", r.Level)
	}
	if !strings.Contains(r.Note, "insufficient repetitions") {
		t.Errorf("note = %q", r.Note)
	}

	r = RankConfidence(SampleSet{}, SampleSet{})
	if r.Level != Low || !r.Indistinguishable {
		t.Fatalf("empty telemetry must not fabricate confidence: %s", r.Level)
	}
}

// determinism: same inputs → identical verdict.
func TestRankDeterministic(t *testing.T) {
	a := SampleSet{Decode: []float64{31.0, 31.4, 31.2}, Prefill: []float64{940, 946, 943}}
	b := SampleSet{Decode: []float64{24.5, 24.9, 24.7}, Prefill: []float64{255, 258, 256}}
	first := RankConfidence(a, b)
	for i := 0; i < 5; i++ {
		again := RankConfidence(a, b)
		if again != first {
			t.Fatal("ranking assessment not deterministic")
		}
	}
}
