package optimize

// The V1 tuning engine. This file implements the empirical search loop that
// turns Gumi from a configuration calculator into an auto-tuner:
//
//	INSPECT → PROBE → DISCOVER BACKEND → GENERATE INITIAL CONFIGS (policy)
//	  → MEASURE REFERENCE → FRONTIER SWEEP → BOUNDARY REFINEMENT
//	  → VARIANT LINES (+dominance pruning) → CAPABILITY-GATE THE FRONTIER
//	  → FINAL VERIFICATION → VERIFIED PROFILES
//
// Everything measured here is real: every PASS/REJECT decision in the loop
// is grounded in llama.cpp runs against the user's actual hardware. The
// deterministic arithmetic (internal/candidate, internal/search) only
// decides WHAT to test next — never whether something works.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/candidate"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/search"
	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

// session carries one tuning run's mutable state.
type session struct {
	engine    *verify.Engine
	opts      Options
	profile   *workload.Profile
	gen       *candidate.Generator
	model     *gguf.ModelInfo
	caps      backend.Capabilities
	objective search.Objective

	// The exploration line for the context frontier sweep.
	lineKV    string
	lineSplit bool

	// obs registers every measured operating point by candidate ID for
	// dominance checks; bestDecode tracks the best stable decode mean seen.
	obs        map[string]search.Observation
	measured   []*candidate.Candidate // all candidates with recorded evidence
	bestDecode float64

	// frontier accumulates the context-frontier evidence for the report.
	frontier report.FrontierSection
	rep      *report.Report

	emit func(Event)
}

// discoverCaps asks the runner what it supports. Runners without capability
// discovery are treated permissively — their legacy-flag retry chains remain
// the final arbiter of what a build accepts.
func discoverCaps(runner backend.Runner) backend.Capabilities {
	if cs, ok := runner.(backend.CapabilitySource); ok {
		return cs.Capabilities()
	}
	return backend.Capabilities{}
}

// selectExplorationLine picks the execution line used for the context
// frontier sweep: among KV precisions the backend supports and placements
// this model may use (family whitelist AND backend -ot support), the one
// whose deterministic arithmetic reaches the largest context. Fidelity
// breaks ties. Pure given its inputs; no hardware names, no throughput
// assumptions.
func selectExplorationLine(g *candidate.Generator, caps backend.Capabilities) (kv string, expertsCPU bool) {
	kv, expertsCPU = "f16", false
	best := g.MaxContextFor("f16", false)
	placementAllowed := g.SplitAllowed() && (caps.OverrideTensor || !caps.Discovered)
	consider := func(k string, split bool) {
		reach := g.MaxContextFor(k, split)
		if reach > best ||
			(reach == best && search.KVRank(k) > search.KVRank(kv)) {
			best, kv, expertsCPU = reach, k, split
		}
	}
	for _, k := range caps.SupportedKVTunables() {
		consider(k, false)
		if placementAllowed {
			consider(k, true)
		}
	}
	if placementAllowed {
		// Expert placement may unlock a larger f16 window too.
		if r := g.MaxContextFor("f16", true); r > best {
			kv, expertsCPU = "f16", true
		}
	}
	return kv, expertsCPU
}

// probePerf performs LOAD → WARMUP → repeated PERF PROBES for one config.
// Warmup absorbs cold-start effects so ranking never uses first-load numbers.
// Errors classify as OOM/timeout/other exactly like full measurements.
func (s *session) probePerf(ctx context.Context, cfg backend.Config) (*candidate.Measurement, error) {
	m := &candidate.Measurement{}
	runCtx, cancel := context.WithTimeout(ctx, s.opts.PerRunTimeout*time.Duration(s.opts.PerfRuns+1))
	defer cancel()

	// Warmup: short generation on the real prompt shape, result discarded.
	if _, err := s.engine.MeasurePerf(runCtx, cfg, s.profile.PerfPromptTokens, warmupGenTokens); err != nil {
		recordProbeFailure(m, err)
		return m, classifyProbeErr(err, m)
	}
	return s.sampleRounds(runCtx, m, cfg, s.opts.PerfRuns)
}

// probeRounds runs n additional perf rounds WITHOUT warmup — used to
// re-test final recommendations so confirmation samples join the evidence.
func (s *session) probeRounds(ctx context.Context, cfg backend.Config, n int) (*candidate.Measurement, error) {
	m := &candidate.Measurement{}
	runCtx, cancel := context.WithTimeout(ctx, s.opts.PerRunTimeout*time.Duration(n))
	defer cancel()
	return s.sampleRounds(runCtx, m, cfg, n)
}

// sampleRounds is the shared repeated-sampling loop.
func (s *session) sampleRounds(runCtx context.Context, m *candidate.Measurement,
	cfg backend.Config, n int) (*candidate.Measurement, error) {

	var prefillSum, decodeSum float64
	for i := 0; i < n; i++ {
		metrics, err := s.engine.MeasurePerf(runCtx, cfg, s.profile.PerfPromptTokens, s.profile.PerfGenTokens)
		switch {
		case errors.Is(err, backend.ErrOutOfMemory):
			m.OOMEvents++
		case errors.Is(err, backend.ErrTimedOut), errors.Is(err, context.DeadlineExceeded):
			m.Timeouts++
		case err != nil:
			m.RunsFailed++
		default:
			sample := candidate.PerfSample{
				PrefillTPS: metrics.PrefillTPS,
				DecodeTPS:  metrics.DecodeTPS,
				PeakVRAM:   metrics.PeakVRAMBytes,
			}
			m.PerfSamples = append(m.PerfSamples, sample)
			m.RunsOK++
			prefillSum += sample.PrefillTPS
			decodeSum += sample.DecodeTPS
			if sample.PeakVRAM > m.PeakVRAM {
				m.PeakVRAM = sample.PeakVRAM // conservative peak across repeats
			}
			if metrics.PeakRAMBytes > m.PeakRAM {
				m.PeakRAM = metrics.PeakRAMBytes
			}
		}
		select {
		case <-runCtx.Done():
			return m, classifyProbeErr(backend.ErrTimedOut, m)
		default:
		}
	}
	if m.RunsOK > 0 {
		m.PrefillTPS = prefillSum / float64(m.RunsOK)
		m.DecodeTPS = decodeSum / float64(m.RunsOK)
		return m, nil
	}
	return m, classifyProbeErr(fmt.Errorf("all perf probes failed (%d oom, %d timeouts, %d errors)",
		m.OOMEvents, m.Timeouts, m.RunsFailed), m)
}

const warmupGenTokens = 24

func recordProbeFailure(m *candidate.Measurement, err error) {
	switch {
	case errors.Is(err, backend.ErrOutOfMemory):
		m.OOMEvents++
	case errors.Is(err, backend.ErrTimedOut), errors.Is(err, context.DeadlineExceeded):
		m.Timeouts++
	default:
		m.RunsFailed++
	}
}

func classifyProbeErr(err error, m *candidate.Measurement) error {
	switch {
	case m.OOMEvents > 0 || errors.Is(err, backend.ErrOutOfMemory):
		return fmt.Errorf("probe failed: %w", backend.ErrOutOfMemory)
	case m.Timeouts > 0 || errors.Is(err, backend.ErrTimedOut):
		return fmt.Errorf("probe failed: %w", backend.ErrTimedOut)
	default:
		return err
	}
}

// registerObs converts a measured candidate into a search observation and
// records it for dominance checks.
func (s *session) registerObs(c *candidate.Candidate) search.Observation {
	o := search.Observation{
		ID:         c.ID,
		Context:    c.Config.ContextTokens,
		KVQ:        search.KVRank(c.Config.KVCacheType),
		Batch:      c.Config.BatchSize,
		UBatch:     c.Config.UBatchSize,
		ExpertsCPU: c.Config.ExpertsOnCPU,
		CapRate:    -1, // unmeasured until the battery says otherwise
	}
	if m := c.Measured; m != nil {
		o.DecodeMean = m.DecodeTPS
		o.Prefill = m.PrefillTPS
		o.PeakVRAM = m.PeakVRAM
		o.PeakRAM = m.PeakRAM
		if len(m.PerfSamples) > 1 {
			dlo, dhi := m.PerfSamples[0].DecodeTPS, m.PerfSamples[0].DecodeTPS
			plo, phi := m.PerfSamples[0].PrefillTPS, m.PerfSamples[0].PrefillTPS
			for _, smp := range m.PerfSamples {
				if smp.DecodeTPS < dlo {
					dlo = smp.DecodeTPS
				}
				if smp.DecodeTPS > dhi {
					dhi = smp.DecodeTPS
				}
				if smp.PrefillTPS < plo {
					plo = smp.PrefillTPS
				}
				if smp.PrefillTPS > phi {
					phi = smp.PrefillTPS
				}
			}
			o.DecodeHalfRange = (dhi - dlo) / 2
			o.PrefillHalfRange = (phi - plo) / 2
		}
		stable := m.Error == "" && m.OOMEvents == 0 && m.Timeouts == 0 && m.RunsOK > 0
		o.Stable = stable
		if m.Capability != nil {
			o.CapRate = m.Capability.Rate
		}
		if stable && m.DecodeTPS > s.bestDecode {
			// The relative practicality rule anchors on the best decode the
			// machine actually demonstrated anywhere in this run.
			s.bestDecode = m.DecodeTPS
			if s.opts.MinDecode <= 0 {
				s.objective.Baseline = s.bestDecode
			}
		}
	}
	s.obs[c.ID] = o
	return o
}

// dominatedBy returns the ID of a measured point that dominates `o`, or "".
//
// Only capability-CLEARED points may dominate: a config that failed the
// gate (or has not run the battery yet) is not a valid benchmark for
// pruning anything — otherwise one fast-but-dumb line could silence every
// slower-but-sane alternative.
func (s *session) dominatedBy(o search.Observation) string {
	dominee := ""
	for _, other := range s.obs {
		if other.ID == o.ID {
			continue
		}
		if !s.gateCleared(other.ID) {
			continue
		}
		if search.DominatedBy(o, other) {
			if dominee == "" || other.ID < dominee {
				dominee = other.ID // deterministic choice among dominators
			}
		}
	}
	return dominee
}

// gateCleared reports whether the named candidate passed its capability
// gate (the REFERENCE passes by definition once measured).
func (s *session) gateCleared(id string) bool {
	for _, c := range s.measured {
		if c.ID != id {
			continue
		}
		if c.Gate == nil {
			return false // battery not run: no domination rights yet
		}
		return c.Gate.Passed
	}
	return false
}

// adoptMeasurement copies evidence from an already-measured candidate with
// an identical configuration so the same operating point is never verified
// twice under two IDs.
func (s *session) adoptMeasurement(c *candidate.Candidate) bool {
	for _, other := range s.measured {
		if other.Config == c.Config {
			c.Measured = other.Measured
			c.Gate = other.Gate
			c.Confidence = other.Confidence
			return true
		}
	}
	return false
}

// suppressUnsupported marks candidates demanding features the discovered
// backend cannot express. Suppressed rows keep their place in the report
// with the reason — TASK 12: suppress, record why, continue tuning.
func (s *session) suppressUnsupported(c *candidate.Candidate) bool {
	if !s.caps.Discovered {
		return false
	}
	kv := strings.ToLower(c.Config.KVCacheType)
	if kv != "" && kv != "f16" && !s.caps.SupportedKV(kv) {
		c.Feasible = false
		c.InfeasibleReason = fmt.Sprintf(
			"suppressed: KV cache type %s is not supported by this backend build (accepted: %s)",
			kv, strings.Join(s.caps.KVTypes, ","))
		return true
	}
	if c.Config.ExpertsOnCPU && !s.caps.OverrideTensor {
		c.Feasible = false
		c.InfeasibleReason = "suppressed: expert tensor placement (-ot) is not supported by this backend build"
		return true
	}
	return false
}

// sweepAndRefine explores the context frontier on the exploration line:
//
//	coarse doubling sweep → boundary refinement between last pass and
//	first failure → the practical frontier candidate (not yet capability-
//	gated; that happens in gateFrontier).
//
// Every probe appends a ProbeOnly candidate to cands so the report records
// ALL tested configurations, passes and failures alike.
func (s *session) sweepAndRefine(ctx context.Context, cands *[]candidate.Candidate, rep *report.Report) {
	s.frontier.LineKV = s.lineKV
	s.frontier.LineExpertsCPU = s.lineSplit
	s.frontier.TheoreticalMax = s.gen.MaxContextFor(s.lineKV, s.lineSplit)

	ladder := search.Ladder(s.profile.MinContext, min64(int(s.model.TrainContext), s.frontier.TheoreticalMax))
	s.frontier.CoarseTested = ladder
	if len(ladder) == 0 {
		s.frontier.BoundaryReason = "no growth room above the workload minimum within predicted memory or training context"
		return
	}

	lo := s.profile.MinContext // the reference at MinContext defines the baseline
	hi := 0                    // lowest failing level; 0 = none yet

	frontierID := ""
	for _, level := range ladder {
		id := fmt.Sprintf("frontier-%d", level)
		c := s.gen.FrontierCandidate(id, fmt.Sprintf("CTX-%s", label1024(level)), level, s.lineKV, s.lineSplit)
		if dup := findDuplicateConfig(*cands, c.Config); dup != "" {
			continue
		}
		*cands = append(*cands, c)
		cc := &(*cands)[len(*cands)-1]

		if !cc.Feasible {
			// Deterministic memory arithmetic says this cannot fit; treat as
			// the theoretical wall between lo and level.
			hi = level
			s.emitf(EvReject, "[SKIP] %s %s — %s", cc.ContextLabel(), s.lineKV, cc.InfeasibleReason)
			s.frontier.Refined = append(s.frontier.Refined, level)
			break
		}
		res, err := s.probePerf(ctx, cc.Config)
		cc.Measured = res
		if err != nil {
			note := err.Error()
			res.Error = note
			res.ObjectiveMet = ptr(false)
			res.ObjectiveNote = note
			s.emitf(EvReject, "[REJECT] %s %s — %s", cc.ContextLabel(), s.lineKV, note)
			hi = level
			s.frontier.Refined = append(s.frontier.Refined, level)
			break
		}
		ok, why := s.objective.Evaluate(search.Stats{
			Mean: res.DecodeTPS, HalfRange: halfRangeOf(res), RunsOK: res.RunsOK,
			OOM: res.OOMEvents, Timeouts: res.Timeouts,
		})
		res.ObjectiveMet = &ok
		res.ObjectiveNote = why
		if !ok {
			cc.Gate = &candidate.GateResult{Passed: false, Reason: why}
		}
		s.registerObs(cc)
		syncOne(rep, *cc)
		if ok {
			s.emitf(EvPass, "[PASS] %s %s — %s", cc.ContextLabel(), s.lineKV, why)
			lo = level
			frontierID = id
		} else {
			s.emitf(EvReject, "[REJECT] %s %s — %s", cc.ContextLabel(), s.lineKV, why)
			hi = level
			s.frontier.Refined = append(s.frontier.Refined, level)
			break
		}
	}

	// Boundary refinement: bisect until the bracket is tighter than the
	// granularity or the step budget runs out.
	granularity := refineGranularity(s.profile.Name)
	for step := 0; step < s.opts.MaxRefineSteps && hi > lo; step++ {
		mid := search.Midpoint(lo, hi, granularity)
		if mid == 0 {
			break
		}
		id := fmt.Sprintf("frontier-%d", mid)
		c := s.gen.FrontierCandidate(id, fmt.Sprintf("CTX-%s", label1024(mid)), mid, s.lineKV, s.lineSplit)
		if dup := findDuplicateConfig(*cands, c.Config); dup != "" {
			// Already probed (identical config): reuse its verdict.
			if prev := findById(*cands, dup); prev != nil && prev.Measured != nil &&
				prev.Measured.ObjectiveMet != nil && *prev.Measured.ObjectiveMet {
				lo = mid
			} else {
				break
			}
			continue
		}
		*cands = append(*cands, c)
		cc := &(*cands)[len(*cands)-1]
		s.frontier.Refined = append(s.frontier.Refined, mid)
		if !cc.Feasible {
			hi = mid
			s.emitf(EvReject, "[SKIP] %s %s — %s", cc.ContextLabel(), s.lineKV, cc.InfeasibleReason)
			continue
		}
		res, err := s.probePerf(ctx, cc.Config)
		cc.Measured = res
		if err != nil {
			res.Error = err.Error()
			res.ObjectiveMet = ptr(false)
			res.ObjectiveNote = err.Error()
			s.emitf(EvReject, "[REJECT] %s %s — %s", cc.ContextLabel(), s.lineKV, err.Error())
			hi = mid
			continue
		}
		ok, why := s.objective.Evaluate(search.Stats{
			Mean: res.DecodeTPS, HalfRange: halfRangeOf(res), RunsOK: res.RunsOK,
			OOM: res.OOMEvents, Timeouts: res.Timeouts,
		})
		res.ObjectiveMet = &ok
		res.ObjectiveNote = why
		if !ok {
			cc.Gate = &candidate.GateResult{Passed: false, Reason: why}
		}
		s.registerObs(cc)
		syncOne(rep, *cc)
		if ok {
			s.emitf(EvPass, "[PASS] %s %s — %s", cc.ContextLabel(), s.lineKV, why)
			lo = mid
			frontierID = id
		} else {
			s.emitf(EvReject, "[REJECT] %s %s — %s", cc.ContextLabel(), s.lineKV, why)
			hi = mid
		}
	}

	if hi == 0 {
		s.frontier.BoundaryReason = fmt.Sprintf(
			"every probed level up to %s met the objective before the exploration ceiling",
			label1024(lo))
	} else if lo >= s.profile.MinContext {
		s.frontier.BoundaryReason = fmt.Sprintf(
			"boundary located between %s (pass) and %s (fail)",
			label1024(lo), label1024(hi))
	} else {
		s.frontier.BoundaryReason = "the workload minimum already fails the performance objective"
	}
	s.frontier.MaxPractical = lo
	s.frontier.FrontierCandidateID = frontierID
}

// gateFrontier runs the FULL capability battery on the frontier operating
// point — the absolute rule applies to MAX CONTEXT like everything else. On
// a capability regression it steps down through measured passing levels
// until a context clears the battery or none remain.
func (s *session) gateFrontier(ctx context.Context, cands []candidate.Candidate, refCap *verify.SuiteResult) {
	if s.frontier.FrontierCandidateID == "" || s.frontier.MaxPractical <= s.profile.MinContext {
		return // no frontier beyond the reference anchor
	}
	capableCeil := s.profile.MinContext // reference cleared here by construction
	current := s.frontier.MaxPractical
	attempts := 0
	for current > capableCeil && attempts < maxFrontierGateAttempts {
		c := findById(cands, fmt.Sprintf("frontier-%d", current))
		if c == nil || c.Measured == nil || c.Measured.Error != "" {
			next := nextLowerLevel(s.frontier, current, capableCeil)
			if next == 0 {
				break
			}
			current = next
			continue
		}
		attempts++
		runSmokeCapability(ctx, s.engine, c, s.profile, s.opts)
		gatePassed := false
		reason := ""
		switch {
		case c.Measured.Error != "":
			reason = c.Measured.Error
		case c.Measured.Smoke != nil && c.Measured.Smoke.Rate < 1.0:
			reason = fmt.Sprintf("smoke verification failed (%d/%d)",
				c.Measured.Smoke.Passed, c.Measured.Smoke.Total)
		case c.Measured.Capability != nil && refCap != nil:
			gatePassed, reason = verify.Gate(refCap, c.Measured.Capability, s.opts.GateSlack)
		default:
			gatePassed = true
			reason = "capability suite unavailable (tier=smoke)"
		}
		c.Gate = &candidate.GateResult{Passed: gatePassed, Reason: reason}
		if gatePassed {
			c.ProbeOnly = false // promoted: full battery cleared it
			s.frontier.CapabilityGated = true
			s.frontier.MaxPractical = current
			s.registerObs(c)
			s.emitf(EvPass, "[VERIFIED] %s %s — capability preserved (%s)",
				c.ContextLabel(), s.lineKV, reason)
			return
		}
		s.emitf(EvReject, "[REJECT] %s %s — capability regression", c.ContextLabel(), s.lineKV)
		syncOne(s.rep, *c)
		s.registerObs(c) // capability evidence lands even on failure
		next := nextLowerLevel(s.frontier, current, capableCeil)
		if next == 0 {
			break
		}
		current = next
	}
	s.frontier.MaxPractical = capableCeil
	s.frontier.CapabilityGated = true
	s.frontier.FrontierCandidateID = ""
}

// nextLowerLevel picks the next probe below current during capability
// step-down: the largest measured passing level below current, then the
// deterministic midpoint of the remaining bracket.
func nextLowerLevel(f report.FrontierSection, current, ceil int) int {
	best := 0
	seen := map[int]bool{}
	for _, l := range f.CoarseTested {
		seen[l] = true
	}
	for _, l := range f.Refined {
		seen[l] = true
	}
	for l := range seen {
		if l < current && l > ceil && l > best {
			best = l
		}
	}
	if best > 0 {
		return best
	}
	return search.Midpoint(ceil, current, MinRefineGranularityTokens)
}

const (
	maxFrontierGateAttempts    = 3
	MinRefineGranularityTokens = 2048
)

// confirmFinals re-tests each recommended profile with a fresh perf round
// (TASK: the final recommendation must be re-tested). New samples APPEND to
// the existing evidence; means and half-ranges are recomputed over all runs.
func (s *session) confirmFinals(ctx context.Context, ids []string, cands []candidate.Candidate, rep *report.Report) {
	uniq := dedupe(ids)
	if len(uniq) == 0 {
		return
	}
	s.emitStage("Final verification...")
	for _, id := range uniq {
		c := findById(cands, id)
		if c == nil || c.Measured == nil || c.Measured.Error != "" {
			continue
		}
		extra, err := s.probeRounds(ctx, c.Config, s.opts.PerfRuns)
		if err != nil {
			rep.Limitations = append(rep.Limitations, fmt.Sprintf(
				"%s: confirmation round unstable (%v); original verification stands", c.Name, err))
			continue
		}
		mergeConfirmSamples(c.Measured, extra)
		s.registerObs(c)
		syncOne(rep, *c)
	}
}

func mergeConfirmSamples(m *candidate.Measurement, extra *candidate.Measurement) {
	m.PerfSamples = append(m.PerfSamples, extra.PerfSamples...)
	m.RunsOK += extra.RunsOK
	m.OOMEvents += extra.OOMEvents
	m.Timeouts += extra.Timeouts
	m.RunsFailed += extra.RunsFailed
	if extra.PeakVRAM > m.PeakVRAM {
		m.PeakVRAM = extra.PeakVRAM
	}
	if extra.PeakRAM > m.PeakRAM {
		m.PeakRAM = extra.PeakRAM
	}
	if len(m.PerfSamples) > 0 {
		var ps, ds float64
		for _, smp := range m.PerfSamples {
			ps += smp.PrefillTPS
			ds += smp.DecodeTPS
		}
		n := float64(len(m.PerfSamples))
		m.PrefillTPS = ps / n
		m.DecodeTPS = ds / n
	}
}

// objectiveSatisfied reports whether a measured candidate met the declared
// performance objective. Candidates never evaluated against it (missing
// verdict) count as unsatisfied for profile eligibility — profiles are
// recommendations, and recommendations must clear every declared gate.
func objectiveSatisfied(c *candidate.Candidate) bool {
	if c.Measured == nil || c.Measured.ObjectiveMet == nil {
		return false
	}
	return *c.Measured.ObjectiveMet
}

// emitf emits one event when a progress sink is attached.
func (s *session) emitf(kind EventKind, format string, args ...any) {
	if s.emit != nil {
		s.emit(Event{Kind: kind, Text: fmt.Sprintf(format, args...)})
	}
}

func (s *session) emitStage(text string) { s.emitf(EvStage, "%s", text) }

// ---- small shared helpers ---------------------------------------------

func halfRangeOf(m *candidate.Measurement) float64 {
	if len(m.PerfSamples) < 2 {
		return 0
	}
	lo, hi := m.PerfSamples[0].DecodeTPS, m.PerfSamples[0].DecodeTPS
	for _, smp := range m.PerfSamples {
		if smp.DecodeTPS < lo {
			lo = smp.DecodeTPS
		}
		if smp.DecodeTPS > hi {
			hi = smp.DecodeTPS
		}
	}
	return (hi - lo) / 2
}

func label1024(ctx int) string {
	if ctx >= 1024 && ctx%1024 == 0 {
		return fmt.Sprintf("%dK", ctx/1024)
	}
	return fmt.Sprintf("%d", ctx)
}

func min64(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func refineGranularity(workloadName string) int {
	// Chat responsiveness lives at small contexts; agentic sessions tolerate
	// coarser steps near multi-hundred-K windows. Granularity stays modest
	// for both — the refinement budget bounds total cost.
	return MinRefineGranularityTokens
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
