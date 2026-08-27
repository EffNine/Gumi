package search

import (
	"strings"
	"testing"
)

func TestLadder(t *testing.T) {
	got := Ladder(16384, 131072)
	want := []int{32768, 65536, 131072}
	if len(got) != len(want) {
		t.Fatalf("Ladder = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Ladder = %v, want %v", got, want)
		}
	}
}

func TestLadderCappedByTrainingContext(t *testing.T) {
	// 40960 is 1.25×32768: the training-context endpoint earns its own probe.
	lvl := Ladder(16384, 40960)
	if len(lvl) != 2 || lvl[0] != 32768 || lvl[1] != 40960 {
		t.Fatalf("Ladder(16384,40960) = %v, want [32768 40960]", lvl)
	}
	// A cap just above a doubling adds nothing meaningful beyond it.
	got := Ladder(4096, 9000)
	if len(got) != 1 || got[0] != 8192 {
		t.Fatalf("Ladder(4096,9000) = %v, want [8192] (cap <1.25× last, not appended)", got)
	}
}

func TestLadderNoGrowthRoom(t *testing.T) {
	if got := Ladder(4096, 4096); got != nil {
		t.Fatalf("no-growth ladder = %v, want nil", got)
	}
}

func TestMidpointNotPowerOfTwo(t *testing.T) {
	// lo=65536 passes, hi=98304 fails -> boundary search lands between.
	mid := Midpoint(65536, 98304, 2048)
	if mid != 81920 { // (65536+98304)/2 rounded to 1024
		t.Fatalf("midpoint = %d, want 81920", mid)
	}
}

func TestMidpointConvergesToGranularity(t *testing.T) {
	lo, hi := 77824, 81920
	steps := 0
	for {
		mid := Midpoint(lo, hi, 2048)
		if mid == 0 {
			break
		}
		steps++
		// Deterministic scenario: everything above 80000 fails.
		if mid <= 80000 {
			lo = mid
		} else {
			hi = mid
		}
		if steps > 20 {
			t.Fatal("refinement did not converge")
		}
	}
	if hi-lo > 2048 {
		t.Fatalf("bracket not refined: [%d,%d]", lo, hi)
	}
	final := Midpoint(lo, 80100, 2048)
	if final != 0 && final%MinRefineGranularity != 0 {
		t.Fatalf("midpoints must stay granularity-aligned, got %d", final)
	}
}

func TestMidpointZeroWhenTight(t *testing.T) {
	if got := Midpoint(77824, 78848, 2048); got != 0 {
		t.Fatalf("tight bracket midpoint = %d, want 0", got)
	}
}

func obs(id string, ctx int, decode, halfrange, prefill, cap float64, vram uint64, stable bool) Observation {
	return Observation{
		ID: id, Context: ctx, DecodeMean: decode, DecodeHalfRange: halfrange,
		Prefill: prefill, CapRate: cap, PeakVRAM: vram, Stable: stable,
	}
}

func TestDominance(t *testing.T) {
	a := obs("a", 32768, 30, 1, 900, 1.0, 10<<30, true) // more VRAM
	b := obs("b", 32768, 31, 1, 950, 1.0, 8<<30, true)  // better everywhere
	if !DominatedBy(a, b) {
		t.Error("b dominates a on every axis")
	}
	c := obs("c", 32768, 30, 1, 900, 1.0, 12<<30, true)
	if DominatedBy(a, c) {
		t.Error("c uses more VRAM; it cannot dominate a")
	}
	d := obs("d", 16384, 45, 1, 1200, 1.0, 4<<30, true)
	if !DominatedBy(a, d) {
		t.Error("d: same-or-better speed, less context memory, less VRAM — dominates a")
	}
	if DominatedBy(d, a) {
		t.Error("a cannot dominate d back: slower and heavier")
	}
	// Equal performance but bigger window is NOT an improvement.
	big := obs("big", 65536, 30, 1, 900, 1.0, 10<<30, true)
	if DominatedBy(a, big) {
		t.Error("bigger context at equal speed/memory is more resource use, not dominance")
	}
	e := obs("e", 32768, 30, 1, 900, 1.0, 10<<30, false)
	if DominatedBy(a, e) {
		t.Error("unstable observations never dominate")
	}
	eq := obs("eq", 32768, 30, 1, 900, 1.0, 10<<30, true)
	if DominatedBy(a, eq) || DominatedBy(eq, a) {
		t.Error("equal observations do not dominate each other")
	}
	unk := obs("unk", 32768, 30, 1, 900, 1.0, 0, true) // VRAM unknown, else equal
	if DominatedBy(a, unk) {
		t.Error("unknown VRAM cannot prove dominance over recorded VRAM")
	}
	if DominatedBy(obs("x", 100, 99, 0, 9999, 1.0, 1<<20, true),
		obs("y", 100, 50, 0, 500, 0.5, 1<<20, true)) {
		t.Error("capability is a benefit axis: slower-but-capable is not dominated by faster-but-worse")
	}
}

func TestPruneDominated(t *testing.T) {
	obs := []Observation{
		obs("slow-big", 32768, 20, 1, 600, 1.0, 11<<30, true),
		obs("winner", 49152, 32, 1, 1000, 1.0, 8<<30, true),
		obs("niche", 16384, 40, 1, 700, 1.0, 5<<30, true), // faster + leaner + smaller window
	}
	survivors, pruned := PruneDominated(obs)
	if _, ok := pruned["slow-big"]; !ok {
		t.Fatalf("slow-big must be pruned, got %v", pruned)
	}
	if pruned["slow-big"] != "niche" {
		t.Fatalf("prune reason must name the dominator: %v", pruned)
	}
	if len(survivors) != 2 {
		t.Fatalf("survivors = %v", survivors)
	}
	if survivors[0].ID != "niche" || survivors[1].ID != "winner" {
		t.Fatalf("survivor order = %s,%s (context-ascending)", survivors[0].ID, survivors[1].ID)
	}
}

func TestObjectiveAbsoluteFloor(t *testing.T) {
	o := Objective{Floor: 25}
	if pass, why := o.Evaluate(Stats{Mean: 25.5, HalfRange: 0.2, RunsOK: 3}); !pass {
		t.Errorf("25.5±0.2 must pass floor 25: %s", why)
	}
	pass, why := o.Evaluate(Stats{Mean: 24.9, HalfRange: 0.1, RunsOK: 3})
	if pass {
		t.Error("24.9 must fail floor 25")
	}
	if !strings.Contains(why, "below target") {
		t.Errorf("rejection reason unclear: %s", why)
	}
	// Noise-aware: mean above floor but noisy lower bound below it.
	if pass, _ := o.Evaluate(Stats{Mean: 26, HalfRange: 2, RunsOK: 3}); pass {
		t.Error("26±2 has lower bound 24 < 25; conservative bound must fail")
	}
	if pass, _ := o.Evaluate(Stats{OOM: 1, Mean: 90}); pass {
		t.Error("OOM probes can never satisfy an objective")
	}
	if pass, _ := o.Evaluate(Stats{Timeouts: 1, Mean: 90}); pass {
		t.Error("timeouts can never satisfy an objective")
	}
	if pass, _ := o.Evaluate(Stats{}); pass {
		t.Error("no data can never satisfy an objective")
	}
}

func TestObjectiveRelativeRetention(t *testing.T) {
	o := Objective{Retention: 0.75, Baseline: 30} // floor resolves to 22.5
	if pass, _ := o.Evaluate(Stats{Mean: 23, HalfRange: 0.1, RunsOK: 3}); !pass {
		t.Error("23 >= 22.5 must pass relative objective")
	}
	if pass, _ := o.Evaluate(Stats{Mean: 22, HalfRange: 0.1, RunsOK: 3}); pass {
		t.Error("22 < 22.5 must fail relative objective")
	}
	if got := o.EffectiveFloor(); got != 22.5 {
		t.Errorf("effective floor = %v, want 22.5", got)
	}
}

func TestObjectiveStabilityOnly(t *testing.T) {
	o := Objective{}
	if pass, why := o.Evaluate(Stats{Mean: 3, HalfRange: 0.1, RunsOK: 2}); !pass {
		t.Errorf("no declared objective means stable points pass: %s", why)
	}
	if got := o.EffectiveFloor(); got != 0 {
		t.Errorf("unset objective floor = %v", got)
	}
}

func ranked(id string, ctx, kvq int, dec, hr float64, vram uint64, cap float64, score float64) Ranked {
	return Ranked{
		ID: id, Context: ctx, KVQ: kvq,
		Obs:   Observation{ID: id, Context: ctx, DecodeMean: dec, DecodeHalfRange: hr, PeakVRAM: vram, Stable: true},
		Score: score, CapRate: cap,
	}
}

func TestSelectProfilesDistinctRoles(t *testing.T) {
	pool := []Ranked{
		ranked("speed16k", 16384, 1, 45, 1, 6<<30, 1.0, 0.70),
		ranked("bal32k", 32768, 2, 33, 1, 9<<30, 1.0, 0.90),
		ranked("max64k", 65536, 1, 28, 1, 10<<30, 1.0, 0.85),
		ranked("qual-f16", 16384, 3, 30, 1, 11<<30, 1.0, 0.75),
	}
	res := SelectProfiles(pool)
	byLabel := map[ProfileLabel]string{}
	for _, p := range res.Picks {
		for _, l := range p.Labels {
			byLabel[l] = p.ID
		}
	}
	if byLabel[LabelMaxContext] != "max64k" {
		t.Errorf("max context = %s", byLabel[LabelMaxContext])
	}
	if byLabel[LabelSpeed] != "speed16k" {
		t.Errorf("speed = %s", byLabel[LabelSpeed])
	}
	if byLabel[LabelQuality] != "qual-f16" {
		t.Errorf("quality = %s (KV fidelity breaks capability ties)", byLabel[LabelQuality])
	}
	if byLabel[LabelBalanced] != "bal32k" {
		t.Errorf("balanced = %s (best utility among unlabeled)", byLabel[LabelBalanced])
	}
	if len(res.Notes) != 0 {
		t.Errorf("distinct roles need no notes: %v", res.Notes)
	}
}

func TestSelectProfilesBalancedSharesWhenExhausted(t *testing.T) {
	pool := []Ranked{
		ranked("speed16k", 16384, 1, 45, 1, 6<<30, 1.0, 0.70),
		ranked("max64k", 65536, 1, 28, 1, 10<<30, 1.0, 0.85),
	}
	res := SelectProfiles(pool)
	shared := false
	for _, n := range res.Notes {
		if strings.Contains(n, "BALANCED shares") {
			shared = true
		}
	}
	if !shared {
		t.Fatalf("BALANCED must be reported as shared when all candidates carry labels: %v", res.Notes)
	}
}

func TestSelectProfilesSingleCandidateCarriesAllLabels(t *testing.T) {
	res := SelectProfiles([]Ranked{ranked("only", 16384, 1, 30, 1, 8<<30, 1.0, 0.5)})
	if len(res.Picks) != 1 {
		t.Fatalf("picks = %d", len(res.Picks))
	}
	p := res.Picks[0]
	if p.ID != "only" || len(p.Labels) != 4 {
		t.Fatalf("single candidate must carry every label: %+v", p)
	}
}

func TestSelectProfilesDeclaresOperationalTies(t *testing.T) {
	pool := []Ranked{
		ranked("a", 16384, 1, 30.0, 1.5, 6<<30, 1.0, 0.5),
		ranked("b", 32768, 1, 30.4, 1.5, 9<<30, 1.0, 0.5), // ranges overlap
	}
	res := SelectProfiles(pool)
	foundTie := false
	for _, p := range res.Picks {
		if len(p.TiedWith) > 0 {
			foundTie = true
		}
	}
	if !foundTie {
		t.Fatalf("overlapping ranges at equal capability must be reported as tied: %+v", res.Picks)
	}
}

func TestSelectProfilesNoManufacturedTies(t *testing.T) {
	pool := []Ranked{
		ranked("a", 16384, 1, 45.0, 0.5, 6<<30, 1.0, 0.5),
		ranked("b", 32768, 1, 28.0, 0.5, 9<<30, 1.0, 0.5),
	}
	res := SelectProfiles(pool)
	for _, p := range res.Picks {
		if len(p.TiedWith) > 0 {
			t.Fatalf("clear separation must not be called tied: %+v", res.Picks)
		}
	}
}

func TestSelectProfilesEmpty(t *testing.T) {
	res := SelectProfiles(nil)
	if len(res.Picks) != 0 || len(res.Notes) == 0 {
		t.Fatalf("empty input must yield no picks and a note: %+v", res)
	}
}
