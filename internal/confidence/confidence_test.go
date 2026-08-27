package confidence

import (
	"reflect"
	"strings"
	"testing"
)

func fullPassFactors() Factors {
	return Factors{
		GatePassed:      true,
		HasCapability:   true,
		CapabilityRate:  1.0,
		SmokePassed:     3,
		SmokeTotal:      3,
		PerfRunsOK:      3,
		DecodeTPS:       []float64{30.0, 30.6, 29.8},
		PeakVRAMBytes:   9 << 30,
		VRAMBudgetBytes: 12 << 30,
	}
}

func TestHighConfidence(t *testing.T) {
	a := Assess(fullPassFactors())
	if a.Level != High {
		t.Fatalf("level = %s, want HIGH (%v)", a.Level, a.Negatives)
	}
	if len(a.Negatives) != 0 {
		t.Errorf("unexpected negatives: %v", a.Negatives)
	}
	for _, want := range []string{
		"capability verification passed (Tier 2)",
		"3/3 stable perf runs",
		"stable decode latency",
	} {
		found := false
		for _, p := range a.Positives {
			if contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing positive %q in %v", want, a.Positives)
		}
	}
}

func TestLowOnGateFailAndOOM(t *testing.T) {
	f := fullPassFactors()
	f.GatePassed = false
	if got := Assess(f).Level; got != Low {
		t.Errorf("gate fail: level = %s, want LOW", got)
	}

	f = fullPassFactors()
	f.OOMEvents = 1
	a := Assess(f)
	if a.Level != Low {
		t.Errorf("oom: level = %s, want LOW", a.Level)
	}
	if !hasNeg(a, "out-of-memory") {
		t.Errorf("missing OOM negative: %v", a.Negatives)
	}

	f = fullPassFactors()
	f.Timeouts = 1
	if got := Assess(f).Level; got != Low {
		t.Errorf("timeout: level = %s, want LOW", got)
	}
}

func TestMediumPaths(t *testing.T) {
	f := fullPassFactors()
	f.PeakVRAMBytes = (12 << 30) - (300 << 20) // borderline headroom
	a := Assess(f)
	if a.Level != Medium {
		t.Errorf("borderline vram: level = %s, want MEDIUM", a.Level)
	}
	if !hasNeg(a, "borderline VRAM") {
		t.Errorf("missing borderline negative: %v", a.Negatives)
	}

	f = fullPassFactors()
	f.DecodeTPS = []float64{30, 55} // 45% spread
	a = Assess(f)
	if a.Level != Medium {
		t.Errorf("unstable latency: level = %s, want MEDIUM", a.Level)
	}
	if !hasNeg(a, "unstable decode latency") {
		t.Errorf("missing instability negative: %v", a.Negatives)
	}

	f = fullPassFactors()
	f.Experimental = true
	a = Assess(f)
	if a.Level != Medium {
		t.Errorf("experimental: level = %s, want MEDIUM", a.Level)
	}
	if !hasNeg(a, "experimental expert placement") {
		t.Errorf("missing experimental negative: %v", a.Negatives)
	}
}

// Unknown data must not fabricate positives or penalties.
func TestUnknownEvidenceNeutral(t *testing.T) {
	f := Factors{GatePassed: true}
	a := Assess(f)
	if a.Level != Medium {
		t.Errorf("level = %s, want MEDIUM", a.Level)
	}
	if hasPos(a, "stable") || hasPos(a, "headroom") || hasPos(a, "perf runs") {
		t.Errorf("unknown evidence produced positives: %v", a.Positives)
	}
	if len(a.Negatives) != 0 {
		t.Errorf("unknown evidence produced negatives: %v", a.Negatives)
	}
}

// Smoke-only runs can reach HIGH but say so honestly.
func TestSmokeOnlyHigh(t *testing.T) {
	f := Factors{
		GatePassed:    true,
		SmokePassed:   3,
		SmokeTotal:    3,
		PerfRunsOK:    2,
		DecodeTPS:     []float64{40, 41},
		HasCapability: false,
	}
	a := Assess(f)
	if a.Level != High {
		t.Fatalf("level = %s, want HIGH (%v)", a.Level, a.Negatives)
	}
	if !hasPos(a, "smoke verification passed (3/3)") {
		t.Errorf("evidence should cite smoke tier: %v", a.Positives)
	}
}

func TestDeterminism(t *testing.T) {
	first := Assess(fullPassFactors())
	for i := 0; i < 10; i++ {
		again := Assess(fullPassFactors())
		if again.Level != first.Level ||
			!reflect.DeepEqual(again.Positives, first.Positives) ||
			!reflect.DeepEqual(again.Negatives, first.Negatives) {
			t.Fatal("assessment is not deterministic")
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func hasNeg(a Assessment, sub string) bool { return hasStr(a.Negatives, sub) }
func hasPos(a Assessment, sub string) bool { return hasStr(a.Positives, sub) }

func hasStr(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
