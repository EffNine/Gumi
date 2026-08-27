package search

import (
	"fmt"
	"sort"
)

// ProfileLabel names one recommended operating-point role.
type ProfileLabel string

const (
	LabelQuality    ProfileLabel = "QUALITY"
	LabelBalanced   ProfileLabel = "BALANCED"
	LabelSpeed      ProfileLabel = "SPEED"
	LabelMaxContext ProfileLabel = "MAX CONTEXT"
)

// Pick assigns profile labels to verified candidates.
type Pick struct {
	ID       string
	Labels   []ProfileLabel
	TiedWith []string // ids operationally indistinguishable from this pick
	Note     string
}

// ProfilesResult is the deterministic multi-profile selection outcome.
type ProfilesResult struct {
	Picks []Pick
	Notes []string
}

// Ranked is the minimal verified-candidate summary SelectProfiles consumes.
// The pipeline derives it from measured, gate-passing candidates only —
// capability-ineligible candidates can never be picked (capability stays
// absolute; ranking never overrides the gate).
type Ranked struct {
	ID      string
	Context int
	KVQ     int // KVRank of KV precision
	Obs     Observation
	Score   float64 // workload-weighted utility from the ranker
	CapRate float64
}

// tied reports whether two observations are operationally
// indistinguishable on decode throughput: overlapping repetition ranges and
// equal capability rates. Deliberately conservative — ties are declared,
// not manufactured.
func tied(a, b Ranked) bool {
	if a.CapRate != b.CapRate {
		return false
	}
	da := a.Obs.DecodeHalfRange + b.Obs.DecodeHalfRange
	if da == 0 {
		// No variance recorded anywhere: zero observed variance means an
		// UNKNOWN noise floor, so fall back to a safe relative margin
		// instead of claiming separation or a tie blindly.
		hi, lo := a.Obs.DecodeMean, b.Obs.DecodeMean
		if hi < lo {
			hi, lo = lo, hi
		}
		return hi-lo <= 0.05*lo+0.1
	}
	diff := a.Obs.DecodeMean - b.Obs.DecodeMean
	if diff < 0 {
		diff = -diff
	}
	return diff <= da
}

// betterForSpeed orders by decode lower bound, then resource frugality.
func speedBetter(a, b Ranked) bool {
	da, db := a.Obs.DecodeLowerBound(), b.Obs.DecodeLowerBound()
	if da != db {
		return da > db
	}
	va, vb := a.Obs.PeakVRAM, b.Obs.PeakVRAM
	if va > 0 && vb > 0 && va != vb {
		return va < vb
	}
	if a.Context != b.Context {
		return a.Context < b.Context
	}
	return false
}

// betterForQuality orders by capability rate, then KV fidelity, then context.
func qualityBetter(a, b Ranked) bool {
	if a.CapRate != b.CapRate {
		return a.CapRate > b.CapRate
	}
	if a.KVQ != b.KVQ {
		return a.KVQ > b.KVQ
	}
	return a.Context > b.Context
}

// SelectProfiles distributes QUALITY / BALANCED / SPEED / MAX CONTEXT labels
// across verified candidates. Rules:
//
//	MAX CONTEXT: largest passing context (tie: less VRAM, then faster).
//	SPEED:       fastest decode lower bound (tie: less VRAM, smaller ctx).
//	QUALITY:     best capability rate, then highest KV fidelity, larger ctx.
//	BALANCED:    best workload utility among candidates not already labeled;
//	             when every candidate is already labeled, the utility-best
//	             candidate shares BALANCED with its earned labels.
//
// Labels may collapse onto one configuration — that is reported as a tie,
// never manufactured into distinct recommendations. With fewer than two
// verified candidates a single pick carries all applicable labels.
func SelectProfiles(cands []Ranked) ProfilesResult {
	var res ProfilesResult
	if len(cands) == 0 {
		res.Notes = append(res.Notes, "no verified candidates — no profiles generated")
		return res
	}
	pool := make([]Ranked, len(cands))
	copy(pool, cands)
	byID := map[string]Ranked{}
	for _, c := range pool {
		byID[c.ID] = c
	}
	labelsOf := map[string][]ProfileLabel{}

	best := func(less func(a, b Ranked) bool) Ranked {
		bestC := pool[0]
		for _, c := range pool[1:] {
			if less(c, bestC) {
				bestC = c
			}
		}
		return bestC
	}

	mc := best(func(a, b Ranked) bool {
		if a.Context != b.Context {
			return a.Context > b.Context
		}
		return speedBetter(a, b)
	})
	sp := best(speedBetter)
	ql := best(qualityBetter)

	labelsOf[mc.ID] = append(labelsOf[mc.ID], LabelMaxContext)
	labelsOf[sp.ID] = append(labelsOf[sp.ID], LabelSpeed)
	labelsOf[ql.ID] = append(labelsOf[ql.ID], LabelQuality)

	// BALANCED prefers an unlabeled candidate with the best utility score;
	// when none remains it joins the utility-best labeled candidate.
	var rest []Ranked
	for _, c := range pool {
		if len(labelsOf[c.ID]) == 0 {
			rest = append(rest, c)
		}
	}
	if len(rest) > 0 {
		bal := rest[0]
		for _, c := range rest[1:] {
			if c.Score > bal.Score {
				bal = c
			}
		}
		labelsOf[bal.ID] = append(labelsOf[bal.ID], LabelBalanced)
	} else {
		bal := pool[0]
		for _, c := range pool[1:] {
			if c.Score > bal.Score {
				bal = c
			}
		}
		if !hasLabel(labelsOf[bal.ID], LabelBalanced) {
			labelsOf[bal.ID] = append(labelsOf[bal.ID], LabelBalanced)
			res.Notes = append(res.Notes,
				fmt.Sprintf("BALANCED shares %s: every verified configuration serves a distinct role", bal.ID))
		}
	}

	// Operational-tie detection between picks whose decode repetition ranges
	// overlap at equal capability: reported honestly instead of manufactured
	// into distinct winners.
	pickIDs := make([]string, 0, len(labelsOf))
	for id := range labelsOf {
		pickIDs = append(pickIDs, id)
	}
	sort.Strings(pickIDs)
	ties := map[string][]string{}
	for i, a := range pickIDs {
		for _, b := range pickIDs[i+1:] {
			if tied(byID[a], byID[b]) {
				ties[a] = append(ties[a], b)
				ties[b] = append(ties[b], a)
			}
		}
	}
	for _, id := range pickIDs {
		res.Picks = append(res.Picks, Pick{ID: id, Labels: labelsOf[id], TiedWith: ties[id]})
	}
	return res
}

func hasLabel(ls []ProfileLabel, l ProfileLabel) bool {
	for _, x := range ls {
		if x == l {
			return true
		}
	}
	return false
}
