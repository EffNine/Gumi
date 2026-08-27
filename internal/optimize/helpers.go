package optimize

import (
	"context"
	"errors"
	"time"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/candidate"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

// findDuplicateConfig reports the ID of an already-generated candidate with
// an identical configuration, or "".
func findDuplicateConfig(cands []candidate.Candidate, cfg backend.Config) string {
	for _, c := range cands {
		if c.Config == cfg {
			return c.ID
		}
	}
	return ""
}

// findById returns a pointer to the candidate with the given ID.
func findById(cands []candidate.Candidate, id string) *candidate.Candidate {
	for i := range cands {
		if cands[i].ID == id {
			return &cands[i]
		}
	}
	return nil
}

// syncOne refreshes one candidate's mutable fields in the report after new
// evidence lands, appending the row when it does not exist yet (frontier
// probes enter the candidate slice after the report skeleton is built), so
// partial artifacts stay current if the run dies later.
func syncOne(rep *report.Report, c candidate.Candidate) {
	if rep == nil {
		return
	}
	for i := range rep.Candidates {
		if rep.Candidates[i].ID != c.ID {
			continue
		}
		totalLayers := rep.Model.Layers
		rc := toCandidateReport(c, totalLayers)
		rep.Candidates[i].Status = rc.Status
		rep.Candidates[i].PrefillTPS = rc.PrefillTPS
		rep.Candidates[i].DecodeTPS = rc.DecodeTPS
		rep.Candidates[i].PeakVRAMGB = rc.PeakVRAMGB
		rep.Candidates[i].PerfRuns = rc.PerfRuns
		rep.Candidates[i].DecodeHalfRange = rc.DecodeHalfRange
		rep.Candidates[i].SmokePassed = rc.SmokePassed
		rep.Candidates[i].SmokeTotal = rc.SmokeTotal
		rep.Candidates[i].CapabilityPassed = rc.CapabilityPassed
		rep.Candidates[i].CapabilityTotal = rc.CapabilityTotal
		rep.Candidates[i].CapabilityRate = rc.CapabilityRate
		rep.Candidates[i].GatePassed = rc.GatePassed
		rep.Candidates[i].GateReason = rc.GateReason
		rep.Candidates[i].Error = rc.Error
		rep.Candidates[i].ProbeOnly = rc.ProbeOnly
		rep.Candidates[i].DominatedBy = rc.DominatedBy
		if rc.Confidence != nil {
			rep.Candidates[i].Confidence = rc.Confidence
		}
		return
	}
	rep.Candidates = append(rep.Candidates, toCandidateReport(c, rep.Model.Layers))
}

// runSmokeCapability appends smoke + capability evidence to an already
// perf-probed candidate. Errors are recorded on the measurement rather than
// returned: a failed task is evidence, not a reason to abort the session.
//
// Each tier receives per-run-timeout × task-count — one wall-clock budget
// cannot cover a 12-task battery on a 30B model without false deadlines.
func runSmokeCapability(ctx context.Context, e *verify.Engine, c *candidate.Candidate,
	p *workload.Profile, opts Options) {

	m := c.Measured
	if m == nil {
		m = &candidate.Measurement{}
		c.Measured = m
	}

	smokeCtx, cancelSmoke := context.WithTimeout(ctx,
		opts.PerRunTimeout*time.Duration(maxInt(1, len(p.SmokeTasks))))
	defer cancelSmoke()
	smoke, err := e.RunSuite(smokeCtx, c.Config, p.SmokeTasks)
	if err != nil {
		m.Error = err.Error()
		recordSuiteTimeout(m, err)
		return
	}
	m.Smoke = smoke

	if opts.TierMax >= verify.TierCapability && len(p.CapabilityTasks) > 0 {
		capCtx, cancelCap := context.WithTimeout(ctx,
			opts.PerRunTimeout*time.Duration(len(p.CapabilityTasks)))
		defer cancelCap()
		capability, err := e.RunSuite(capCtx, c.Config, p.CapabilityTasks)
		if err != nil {
			m.Error = err.Error()
			recordSuiteTimeout(m, err)
			return
		}
		m.Capability = capability
	}
}

// recordSuiteTimeout keeps timeout classification honest when a suite (not
// a perf probe) hits its budget.
func recordSuiteTimeout(m *candidate.Measurement, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, backend.ErrTimedOut) {
		m.Timeouts++
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
