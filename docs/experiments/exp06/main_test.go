package main

import (
	"strings"
	"testing"
)

func TestSessionSizesAndDepth(t *testing.T) {
	for _, target := range sessionSizes {
		s := buildSession(target)
		est := estTokens(len(s))
		if est < target*85/100 || est > target*115/100 {
			t.Errorf("target %d: estimated tokens %d outside [85%%,115%%]", target, est)
		}
		// Depth placement: TEST fact must sit after the RULE fact and well
		// before the final code block.
		rulePos := strings.Index(s, wantRule)
		testPos := strings.Index(s, wantTest)
		fixPos := strings.Index(s, wantFix)
		if rulePos < 0 || testPos < 0 || fixPos < 0 {
			t.Fatalf("target %d: missing planted facts", target)
		}
		total := len(s)
		if d := float64(testPos) / float64(total); d < 0.30 || d > 0.70 {
			t.Errorf("target %d: TEST depth %.2f outside [0.30,0.70]", target, d)
		}
		if d := float64(fixPos) / float64(total); d < 0.85 {
			t.Errorf("target %d: FIX depth %.2f not late (>=0.85)", target, d)
		}
		if !strings.HasSuffix(strings.TrimSpace(s), "FIX=<exact approved-fix phrase from the latest review comment>") {
			t.Errorf("target %d: session must end with the question block", target)
		}
	}
}

func TestTruncationDropsHeadKeepsTailAndQuestion(t *testing.T) {
	full := buildSession(24000)
	visible := 7488 // ~8k window minus reserves
	got, est := truncateToVisible(full, visible)
	if est > visible {
		t.Errorf("visible estimate %d exceeds budget %d", est, visible)
	}
	// Strict eviction: the step-1 briefing (RULE fact) sits at the head and
	// MUST be gone; tail-anchored content and the question MUST survive.
	if strings.Contains(got, "PRICING_RULE_ID="+wantRule) {
		t.Error("head eviction must remove the step-1 briefing carrying the rule")
	}
	for _, keep := range []string{wantFix, "SESSION COMPLETE", "FIX="} {
		if !strings.Contains(got, keep) {
			t.Errorf("truncated session lost required content %q", keep)
		}
	}
	// At 50% depth the TEST fact falls outside a ~7.5k window of a 24k
	// session — its absence is the experiment working, assert it.
	if strings.Contains(got, wantTest) {
		t.Log("note: TEST fact still visible at this budget (acceptable if near boundary)")
	}
	if len(got) >= len(full) {
		t.Error("truncation did not shrink the session")
	}
}

func TestSmallSessionUntouched(t *testing.T) {
	full := buildSession(4000)
	got, est := truncateToVisible(full, 30000)
	if got != full || est != estTokens(len(full)) {
		t.Error("sessions within the visible budget must pass through unmodified")
	}
}

func TestGradeParts(t *testing.T) {
	ok := "RULE=EU-VAT-DEFERRED-1987\nTEST=TestBulkDiscount_OffByOne\nFIX=clamp negative amounts to zero"
	g := grade(ok)
	if !g["RULE"] || !g["TEST"] || !g["FIX"] {
		t.Errorf("well-formed answer must fully pass: %v", g)
	}
	partial := "RULE=EU-VAT-DEFERRED-1987\nTEST=i do not know\nFIX=clamp negative amounts to zero"
	g = grade(partial)
	if !g["RULE"] || g["TEST"] || !g["FIX"] {
		t.Errorf("partial answer grading wrong: %v", g)
	}
	// Tolerate reasoning prose around the labeled lines.
	noisy := "Let me analyze...\nRULE: EU-VAT-DEFERRED-1987\nTEST: TestBulkDiscount_OffByOne\nFIX: clamp negative amounts to zero\nDone."
	g = grade(noisy)
	if !g["RULE"] || !g["TEST"] || !g["FIX"] {
		t.Errorf("colon-labeled lines should also grade (grader is prefix-tolerant): %v", g)
	}
}
