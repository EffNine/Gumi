// Package optimize orchestrates the full pipeline:
//
//	MODEL -> Geometry Inspector -> Hardware Probe -> Candidate Generator ->
//	Backend Tester -> Capability Verification -> Profile Generator -> Report
package optimize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/candidate"
	"github.com/EffNine/gumi/internal/confidence"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/search"
	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

// Options configures a pipeline run.
type Options struct {
	ModelPath        string
	Workload         string
	TierMax          verify.Tier // TierSmoke or TierCapability
	OutDir           string      // empty => auto under ./reports/
	DryRun           bool
	BackendBin       string        // path to llama-cli
	PerRunTimeout    time.Duration // default 10m
	GateSlack        float64       // default 0 (strict parity with reference)
	MeasureBandwidth bool
	PerfRuns         int // perf probe repetitions for stability evidence; default 3
	// MinDecode is the user-declared absolute decode floor (tok/s). When
	// zero, the workload's relative practicality rule governs the frontier
	// instead — Gumi never enforces a universal throughput requirement.
	MinDecode float64
	// MaxRefineSteps bounds context-boundary refinement probes; default 4.
	MaxRefineSteps int
	// BaselineSpecs are human-provided configuration specs ("ngl=33,c=8192,...")
	// admitted as CURRENT-BASELINE candidates so real-world configurations can
	// be compared against Gumi's through the identical measurement + gate.
	BaselineSpecs []string
	Version       string
	// Progress receives live tuning events for console UX. Nil is valid.
	Progress func(Event)
}

// EventKind classifies one tuning-progress event.
type EventKind string

const (
	EvStage  EventKind = "stage"
	EvInfo   EventKind = "info"
	EvPass   EventKind = "pass"
	EvReject EventKind = "reject"
)

// Event is one human-readable progress item emitted while tuning.
type Event struct {
	Kind EventKind
	Text string
}

// Defaults fills zero-valued options.
func (o *Options) Defaults() {
	if o.TierMax == 0 {
		o.TierMax = verify.TierCapability
	}
	if o.PerRunTimeout == 0 {
		o.PerRunTimeout = 10 * time.Minute
	}
	if o.PerfRuns <= 0 {
		o.PerfRuns = 3
	}
	if o.MaxRefineSteps <= 0 {
		o.MaxRefineSteps = 4
	}
}

// Seams for deterministic testing: tests override these to inject a fake
// backend runner and fixed hardware facts.
var (
	newRunner     = func(bin string) backend.Runner { return backend.NewLlamaCLI(bin) }
	probeHardware = func(opts hardware.Options) (*hardware.Info, error) {
		return hardware.Detect(opts)
	}
)

// Run executes the pipeline end-to-end.
func Run(ctx context.Context, opts Options) (*report.Report, string, error) {
	opts.Defaults()

	modelInfo, err := gguf.Inspect(opts.ModelPath)
	if err != nil {
		return nil, "", err
	}
	hw, err := probeHardware(hardware.Options{
		ModelPath:        opts.ModelPath,
		MeasureBandwidth: opts.MeasureBandwidth,
	})
	if err != nil {
		return nil, "", fmt.Errorf("hardware probe: %w", err)
	}
	profile, err := workload.Get(opts.Workload)
	if err != nil {
		return nil, "", err
	}
	gen, err := candidate.NewGenerator(modelInfo, hw, profile)
	if err != nil {
		return nil, "", err
	}

	// Backend capability discovery precedes planning for real runs: the
	// generator must never produce configurations the installed build
	// cannot express (TASK 12 — suppress upstream, record why, continue).
	var runner backend.Runner
	var caps backend.Capabilities
	if !opts.DryRun {
		emit := func(ev Event) {
			if opts.Progress != nil {
				opts.Progress(ev)
			}
		}
		emit(Event{Kind: EvStage, Text: "Discovering backend capabilities..."})
		runner = newRunner(opts.BackendBin)
		if err := runner.Available(ctx); err != nil {
			return nil, "", fmt.Errorf("verification backend unavailable: %w", err)
		}
		caps = discoverCaps(runner)
		gen.ApplyBackendCaps(caps)
	}

	cands := gen.Generate()
	for i, spec := range opts.BaselineSpecs {
		cfg, err := backend.ParseConfigSpec(spec)
		if err != nil {
			return nil, "", fmt.Errorf("baseline %d: %w", i+1, err)
		}
		id := "baseline"
		name := "CURRENT-BASELINE"
		if i > 0 {
			id = fmt.Sprintf("baseline-%d", i+1)
			name = fmt.Sprintf("CURRENT-BASELINE-%d", i+1)
		}
		cands = append(cands, gen.BaselineCandidate(id, name, cfg))
	}
	vramBudget := gen.VRAMBudget()

	outDir := opts.OutDir
	if outDir == "" {
		base := strings.TrimSuffix(filepath.Base(opts.ModelPath), filepath.Ext(opts.ModelPath))
		outDir = filepath.Join("reports", sanitize(base)+"-"+opts.Workload+"-"+time.Now().Format("20060102-150405"))
	}

	rep := &report.Report{
		GeneratedAt: time.Now().UTC(),
		Version:     opts.Version,
		Workload:    profile.Name,
		Model:       modelSummary(modelInfo),
		Hardware:    hardwareSummary(hw),
	}
	if idx := indexOfKind(cands, candidate.KindReference); idx >= 0 {
		rep.Reference = referenceSection(&cands[idx], modelInfo.LayerCount)
	}
	rep.Policy = policySection(gen, profile)
	for _, n := range profile.Notes {
		rep.Limitations = append(rep.Limitations, n)
	}
	if opts.TierMax < verify.TierCapability {
		rep.Limitations = append(rep.Limitations,
			"tier=smoke: capability suite not executed; PASS/REJECT reflects smoke checks only")
	}

	for _, c := range cands {
		rep.Candidates = append(rep.Candidates, toCandidateReport(c, modelInfo.LayerCount))
	}

	// V1 tuning sections exist even in plan mode: they show WHAT would be
	// searched, clearly labeled as planned rather than measured.
	if opts.DryRun {
		lineKV, lineSplit := selectExplorationLine(gen, discoverCaps(nil))
		plannedTheoretical := gen.MaxContextFor(lineKV, lineSplit)
		rep.Frontier = &report.FrontierSection{
			LineKV:         lineKV,
			LineExpertsCPU: lineSplit,
			TheoreticalMax: plannedTheoretical,
			CoarseTested:   search.Ladder(profile.MinContext, min64(int(modelInfo.TrainContext), plannedTheoretical)),
			BoundaryReason: "planned only (dry run): no backend measurements performed",
		}
		for i := range cands {
			if cands[i].Feasible {
				rep.WinnerID = cands[i].ID
				break
			}
		}
		rep.Objective = &report.ObjectiveSection{
			UserFloorTPS: opts.MinDecode,
			Retention:    profile.DecodeRetention,
			Achieved:     true, // nothing measured yet; no failure to report
			Statement: "planned only (dry run): " +
				objectiveDescribe(opts.MinDecode, profile.DecodeRetention),
		}
		if err := rep.WriteArtifacts(outDir); err != nil {
			return rep, outDir, err
		}
		writeAuxArtifacts(outDir, cands, hw)
		return rep, outDir, nil
	}

	emit := func(ev Event) {
		if opts.Progress != nil {
			opts.Progress(ev)
		}
	}
	emitStage := func(text string) { emit(Event{Kind: EvStage, Text: text}) }

	engine := verify.NewEngine(runner, opts.ModelPath)

	suppressed := suppressedDimensions(caps)
	rep.Backend = &report.BackendCapsSection{
		Backend:        runner.Name(),
		Discovered:     caps.Discovered,
		FlashAttention: caps.FlashAttention || !caps.Discovered,
		KVTunables:     caps.SupportedKVTunables(),
		OverrideTensor: caps.OverrideTensor || !caps.Discovered,
		SingleTurn:     caps.SingleTurn,
		Suppressed:     suppressed,
	}
	if !caps.Discovered {
		rep.Limitations = append(rep.Limitations,
			"backend capabilities could not be discovered from --help; flag support relies on the legacy retry chain")
	}
	for _, sup := range suppressed {
		rep.Limitations = append(rep.Limitations, sup)
	}

	sess := &session{
		engine:  engine,
		opts:    opts,
		profile: profile,
		gen:     gen,
		model:   modelInfo,
		caps:    caps,
		obs:     map[string]search.Observation{},
		rep:     rep,
		emit:    emit,
	}

	// ---- STAGE A: REFERENCE -------------------------------------------------
	// Measure the conservative control first; it anchors every paired
	// comparison and freezes the practicality baseline. DecodeRetention is
	// thereafter evaluated against this frozen REFERENCE baseline, not against
	// later faster discoveries (see tuner.go registerObs: best observed is
	// tracked separately and never redefines the floor).
	refIdx := indexOfKind(cands, candidate.KindReference)
	if refIdx < 0 {
		return nil, outDir, fmt.Errorf("internal: no reference candidate generated")
	}
	emitStage("Measuring REFERENCE configuration...")
	refRes, err := measureWithRetry(ctx, engine, &cands[refIdx], profile, opts)
	if err != nil {
		cands[refIdx].Measured = &candidate.Measurement{Error: err.Error()}
		syncReportCandidates(rep, cands)
		rep.Candidates[refIdx].Error = err.Error()
		_ = rep.WriteArtifacts(outDir)
		writeAuxArtifacts(outDir, cands, hw)
		return rep, outDir, fmt.Errorf("reference configuration failed verification: %w", err)
	}
	cands[refIdx].Measured = refRes
	sess.measured = append(sess.measured, &cands[refIdx])
	sess.registerObs(&cands[refIdx])

	sess.objective = search.Objective{
		Floor:     opts.MinDecode,
		Retention: profile.DecodeRetention,
		Baseline:  refRes.DecodeTPS,
	}

	var refCap *verify.SuiteResult
	if refRes.Capability != nil {
		refCap = refRes.Capability
	}

	// ---- STAGES B/C: CONTEXT FRONTIER SWEEP + BOUNDARY REFINEMENT ----------
	frontierBlocked := opts.MinDecode > 0 && refRes.DecodeTPS < opts.MinDecode
	if frontierBlocked {
		// Even the conservative control misses an explicitly requested
		// floor; growing context cannot recover throughput. Skip straight
		// to variant lines, which CAN change the picture.
		emit(Event{Kind: EvInfo, Text: fmt.Sprintf(
			"reference decode %.1f tok/s already below --min-decode %.1f; skipping context growth",
			refRes.DecodeTPS, opts.MinDecode)})
		sess.frontier.BoundaryReason = "skipped: reference configuration already below the user performance floor"
		sess.frontier.MaxPractical = profile.MinContext
	} else {
		emitStage("Searching context frontier...")
		sess.lineKV, sess.lineSplit = selectExplorationLine(gen, caps)
		sess.sweepAndRefine(ctx, &cands, rep)
	}

	// ---- STAGE D: VARIANT LINES --------------------------------------------
	// Policy-slot candidates (QUALITY / BALANCED / SPEED / HIGH-CONTEXT /
	// EXPERT-SPLIT) plus human baselines. Each gets warmup + repeated perf
	// probes; the expensive capability battery runs only for configurations
	// not dominated by already-measured evidence.
	emitStage("Testing configuration variants...")
	for i := range cands {
		c := &cands[i]
		if i == refIdx || !c.Feasible {
			continue
		}
		// Candidates that already carry evidence (frontier probes measured
		// during the sweep) keep their recorded verdicts; re-processing
		// would duplicate work and let latecomers dominate them wrongly.
		if c.Measured != nil {
			syncOne(rep, *c)
			continue
		}
		isBaseline := c.Kind == candidate.KindBaseline
		if sess.suppressUnsupported(c) {
			sess.emitf(EvInfo, "[SKIP] %s — %s", c.Name, c.InfeasibleReason)
			syncOne(rep, *c)
			continue
		}
		if sess.adoptMeasurement(c) {
			sess.emitf(EvInfo, "[SAME] %s — identical configuration already measured", c.Name)
			syncOne(rep, *c)
			continue
		}
		if isBaseline {
			// Human baselines always receive the full identical treatment.
			res, merr := measure(ctx, engine, c, profile, opts)
			if merr != nil {
				c.Measured = &candidate.Measurement{Error: merr.Error()}
			} else {
				c.Measured = res
			}
		} else {
			res, perr := sess.probePerf(ctx, c.Config)
			if perr != nil {
				res.Error = perr.Error()
				c.Measured = res
				sess.emitf(EvReject, "[REJECT] %s — %s", c.Name, perr.Error())
			} else {
				c.Measured = res
				// Variants are judged against the same performance
				// objective as the frontier; the verdict is recorded (and
				// gates profile eligibility) but does not by itself reject:
				// capability remains the absolute gate.
				ok, why := sess.objective.Evaluate(search.Stats{
					Mean: res.DecodeTPS, HalfRange: halfRangeOf(res), RunsOK: res.RunsOK,
					OOM: res.OOMEvents, Timeouts: res.Timeouts,
				})
				res.ObjectiveMet = &ok
				res.ObjectiveNote = why
				o := sess.registerObs(c)
				if dom := sess.dominatedBy(o); dom != "" {
					c.DominatedBy = dom
					sess.emitf(EvInfo, "[SKIP] %s — dominated by %s; tuning budget saved", c.Name, dom)
					syncOne(rep, *c)
					continue
				}
			}
		}
		sess.measured = append(sess.measured, c)
		if c.Measured == nil || c.Measured.Error != "" {
			syncOne(rep, *c)
			continue
		}
		if !isBaseline {
			runSmokeCapability(ctx, engine, c, profile, opts)
		}

		// Capability gate: never recommend faster-but-worse.
		gatePassed := false
		gateReason := ""
		switch {
		case c.Measured.Error != "":
			gateReason = c.Measured.Error
		case c.Measured.Smoke != nil && c.Measured.Smoke.Rate < 1.0:
			gateReason = fmt.Sprintf("smoke verification failed (%d/%d)", c.Measured.Smoke.Passed, c.Measured.Smoke.Total)
		case c.Measured.Capability != nil && refCap != nil:
			gatePassed, gateReason = verify.Gate(refCap, c.Measured.Capability, opts.GateSlack)
		case isBaseline && c.Measured.Capability == nil:
			gatePassed = true
			gateReason = "smoke verification passed (no capability suite compared)"
		default:
			gatePassed = true
			gateReason = "smoke verification passed (no capability suite compared)"
		}
		c.Gate = &candidate.GateResult{Passed: gatePassed, Reason: gateReason}
		sess.registerObs(c)
		syncOne(rep, *c)
		if gatePassed {
			sess.emitf(EvPass, "[PASS] %s — %s", c.Name, gateReason)
		} else {
			sess.emitf(EvReject, "[REJECT] %s — %s", c.Name, gateReason)
		}
	}
	// Reference always passes its own gate by definition.
	cands[refIdx].Gate = &candidate.GateResult{Passed: true, Reason: "reference configuration"}
	syncOne(rep, cands[refIdx])

	// ---- STAGE E: CAPABILITY-GATE THE FRONTIER ------------------------------
	// MAX PRACTICAL CONTEXT is a recommendation like any other: the full
	// battery must clear it. On regression, step down through measured
	// passing levels.
	emitStage("Verifying frontier capability...")
	sess.rep = rep
	sess.gateFrontier(ctx, cands, refCap)
	rep.Frontier = &(sess.frontier)

	rank(cands, profile)

	// Deterministic confidence scoring from measured evidence.
	for i := range cands {
		if cands[i].Measured == nil {
			continue
		}
		cands[i].Confidence = assessConfidence(&cands[i], vramBudget)
	}

	winnerID := ""
	for _, c := range cands {
		if c.Feasible && c.Gate != nil && c.Gate.Passed && c.Measured != nil && c.Measured.Error == "" {
			winnerID = c.ID
			break
		}
	}

	// Ranking confidence separates "is this candidate eligible?" (gate) from
	// "how sure are we it is the fastest eligible option?" (this). When the
	// top two passers are operationally indistinguishable, prefer the safer
	// operating margin instead of manufacturing a speed winner.
	if runnerUp := findCandidate(cands, nthPasserID(cands, winnerID, 1)); runnerUp != nil {
		if w := findCandidate(cands, winnerID); w != nil &&
			rankingAssessment(cands, winnerID).Indistinguishable && saferMargin(runnerUp, w, vramBudget) {
			winnerID = runnerUp.ID
		}
	}
	rep.WinnerID = winnerID
	rep.Ranking = rankingAssessment(cands, winnerID)

	// ---- VERIFIED PROFILES --------------------------------------------------
	// Profile candidates must satisfy BOTH absolute gates: capability (the
	// paired gate) and the declared performance objective. The REFERENCE
	// anchor stays eligible either way — it defines the machine's baseline.
	var picks []search.Ranked
	for i := range cands {
		c := &cands[i]
		if !passEligible(*c) {
			continue
		}
		if c.ID != "reference" && !objectiveSatisfied(c) {
			continue
		}
		o := sess.obs[c.ID]
		picks = append(picks, search.Ranked{
			ID: c.ID, Context: c.Config.ContextTokens,
			KVQ: search.KVRank(c.Config.KVCacheType),
			Obs: o, Score: c.Score, CapRate: capRate(c),
		})
	}
	profileRes := search.SelectProfiles(picks)
	for _, p := range profileRes.Picks {
		entry := report.ProfileEntry{TiedWith: p.TiedWith}
		entry.Labels = make([]string, len(p.Labels))
		for li, l := range p.Labels {
			entry.Labels[li] = string(l)
		}
		if c := findCandidate(cands, p.ID); c != nil {
			m := c.Measured
			entry.CandidateID = c.ID
			entry.Name = c.Name
			entry.Context = c.Config.ContextTokens
			entry.KVCache = strings.ToUpper(c.Config.KVCacheType)
			if m != nil {
				entry.DecodeTPS = m.DecodeTPS
				entry.PrefillTPS = m.PrefillTPS
				entry.PeakVRAMGB = float64(m.PeakVRAM) / (1 << 30)
				if m.Capability != nil {
					entry.CapRate = m.Capability.Rate
				}
			}
			if c.Confidence != nil {
				entry.Confidence = string(c.Confidence.Level)
			}
		}
		rep.Profiles = append(rep.Profiles, entry)
	}
	rep.Limitations = append(rep.Limitations, profileRes.Notes...)

	// ---- FINAL VERIFICATION -------------------------------------------------
	// Re-test every recommended operating point with fresh perf rounds;
	// new samples join the existing evidence.
	confirmIDs := []string{winnerID}
	for _, p := range profileRes.Picks {
		confirmIDs = append(confirmIDs, p.ID)
	}
	sess.confirmFinals(ctx, confirmIDs, cands, rep)

	// ---- OBJECTIVE OUTCOME --------------------------------------------------
	// The baseline is frozen at REFERENCE; best observed is reported separately
	// and never moves the floor. See tuner.go registerObs and search.Objective.
	effectiveFloor := sess.objective.EffectiveFloor()
	obj := &report.ObjectiveSection{
		UserFloorTPS:          opts.MinDecode,
		Retention:             profile.DecodeRetention,
		BaselineDecodeTPS:     sess.objective.Baseline,
		EffectiveFloorTPS:     effectiveFloor,
		BestObservedDecodeTPS: sess.bestDecode,
	}
	if effectiveFloor <= 0 {
		obj.Achieved = winnerID != ""
		obj.Statement = objectiveDescribe(opts.MinDecode, profile.DecodeRetention) +
			" — stable verified configuration found"
	} else {
		best := findCandidate(cands, winnerID)
		met := false
		if best != nil && best.Measured != nil {
			lb := best.Measured.DecodeTPS - halfRangeOf(best.Measured)
			met = lb+1e-9 >= effectiveFloor
		}
		obj.Achieved = winnerID != "" && met
		if obj.Achieved {
			obj.Statement = fmt.Sprintf("objective met: %s", sess.objective.Describe())
		} else {
			statement := fmt.Sprintf("TARGET NOT ACHIEVED: no verified configuration reaches %s",
				sess.objective.Describe())
			if best := findCandidate(cands, winnerID); best != nil && best.Measured != nil {
				statement += fmt.Sprintf("; best verified: %.1f tok/s at %s context",
					best.Measured.DecodeTPS, label1024(best.Config.ContextTokens))
			}
			obj.Statement = statement
			rep.Limitations = append(rep.Limitations, obj.Statement)
		}
	}
	rep.Objective = obj

	if w := findCandidate(cands, winnerID); w != nil {
		rep.Exports = ptr(backend.RenderExports(opts.ModelPath, w.Config))
	}

	syncReportCandidates(rep, cands)
	// Recommendation is a status upgrade over VERIFIED, never over anything
	// else — dry-run winners stay SCREENED. Applied after sync, which
	// recomputes statuses from raw evidence.
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == winnerID && rep.Candidates[i].Status == report.StatusVerified {
			rep.Candidates[i].Status = report.StatusRecommended
		}
	}
	if err := rep.WriteArtifacts(outDir); err != nil {
		return rep, outDir, err
	}
	writeAuxArtifacts(outDir, cands, hw)
	return rep, outDir, nil
}

// objectiveDescribe renders the declared objective for reports.
func objectiveDescribe(floor, retention float64) string {
	switch {
	case floor > 0:
		return fmt.Sprintf("user floor: decode >= %.1f tok/s", floor)
	case retention > 0:
		return fmt.Sprintf("workload practicality: retain >= %.0f%% of frozen reference baseline", retention*100)
	default:
		return "objective: stable execution ranked by workload utility"
	}
}

// suppressedDimensions lists tuning dimensions unavailable in this backend
// build, each with the reason. Suppression keeps the session running on the
// supported remainder instead of failing outright.
func suppressedDimensions(caps backend.Capabilities) []string {
	if !caps.Discovered {
		return nil
	}
	var out []string
	if len(caps.KVTypes) == 0 {
		out = append(out, "KV cache quantization suppressed: backend help lists no cache-type choices")
	} else {
		for _, t := range backend.TunableKVTypes {
			if !caps.SupportedKV(t) {
				out = append(out, fmt.Sprintf("KV cache type %s suppressed: not accepted by this backend build", t))
			}
		}
	}
	if !caps.OverrideTensor {
		out = append(out, "expert tensor placement (-ot) suppressed: backend lacks override-tensor support")
	}
	return out
}

// measureWithRetry measures the reference, degrading context on OOM so that
// a too-ambitious control does not sink the whole optimization.
func measureWithRetry(ctx context.Context, e *verify.Engine, c *candidate.Candidate,
	p *workload.Profile, opts Options) (*candidate.Measurement, error) {

	res, err := measure(ctx, e, c, p, opts)
	if err == nil && res.Error == "" {
		return res, nil
	}
	isOOM := (err != nil && errors.Is(err, backend.ErrOutOfMemory)) ||
		(err == nil && res.OOMEvents > 0)
	if isOOM && c.Config.ContextTokens > 2048 {
		halved := *c
		halved.Config.ContextTokens = c.Config.ContextTokens / 2
		res2, err2 := measure(ctx, e, &halved, p, opts)
		if err2 == nil && res2.Error == "" {
			c.Config.ContextTokens = halved.Config.ContextTokens
			c.Rationale += fmt.Sprintf(" (context degraded to %d after OOM)", halved.Config.ContextTokens)
			return res2, nil
		}
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%s", res.Error)
}

func measure(ctx context.Context, e *verify.Engine, c *candidate.Candidate,
	p *workload.Profile, opts Options) (*candidate.Measurement, error) {

	m := &candidate.Measurement{}
	runCtx, cancel := context.WithTimeout(ctx, opts.PerRunTimeout*time.Duration(opts.PerfRuns))
	defer cancel()

	// Repeated perf probes: reproducibility evidence for confidence scoring.
	var prefillSum, decodeSum float64
	for i := 0; i < opts.PerfRuns; i++ {
		metrics, err := e.MeasurePerf(runCtx, c.Config, p.PerfPromptTokens, p.PerfGenTokens)
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
		case <-ctx.Done():
			return m, ctx.Err()
		default:
		}
	}
	if m.RunsOK > 0 {
		m.PrefillTPS = prefillSum / float64(m.RunsOK)
		m.DecodeTPS = decodeSum / float64(m.RunsOK)
	} else if m.OOMEvents+m.Timeouts+m.RunsFailed > 0 {
		if m.OOMEvents > 0 {
			return nil, fmt.Errorf("perf run failed: %w", backend.ErrOutOfMemory)
		}
		if m.Timeouts > 0 {
			return nil, fmt.Errorf("perf run failed: %w", backend.ErrTimedOut)
		}
		return nil, fmt.Errorf("all %d perf runs failed", m.RunsFailed)
	}

	smoke, err := e.RunSuite(runCtx, c.Config, p.SmokeTasks)
	if err != nil {
		return m, nil
	}
	m.Smoke = smoke

	if opts.TierMax >= verify.TierCapability && len(p.CapabilityTasks) > 0 {
		capCtx, cancelCap := context.WithTimeout(ctx, opts.PerRunTimeout*time.Duration(len(p.CapabilityTasks)))
		defer cancelCap()
		capability, err := e.RunSuite(capCtx, c.Config, p.CapabilityTasks)
		if err != nil {
			return m, nil
		}
		m.Capability = capability
	}
	return m, nil
}

// nthPasserID returns the ID of the n-th gate-passing candidate in ranked
// order (n=0 is the current winner). Empty when fewer passers exist.
func nthPasserID(cands []candidate.Candidate, skipID string, n int) string {
	seen := 0
	for _, c := range cands {
		if !passEligible(c) || c.ID == skipID {
			continue
		}
		if seen == n {
			return c.ID
		}
		seen++
	}
	return ""
}

// sampleSet converts recorded perf samples for ranking math.
func sampleSet(m *candidate.Measurement) confidence.SampleSet {
	var s confidence.SampleSet
	if m == nil {
		return s
	}
	for _, p := range m.PerfSamples {
		s.Decode = append(s.Decode, p.DecodeTPS)
		s.Prefill = append(s.Prefill, p.PrefillTPS)
	}
	return s
}

// rankingAssessment compares the winner with its strongest passing rival.
func rankingAssessment(cands []candidate.Candidate, winnerID string) *report.RankingReport {
	winner := findCandidate(cands, winnerID)
	if winner == nil {
		return nil
	}
	runnerID := nthPasserID(cands, winnerID, 1)
	if runnerID == "" {
		return &report.RankingReport{
			Level:    string(confidence.High),
			WinnerID: winnerID,
			Note:     "single gate-passing candidate — no ranking comparison required",
		}
	}
	runner := findCandidate(cands, runnerID)
	rk := confidence.RankConfidence(sampleSet(winner.Measured), sampleSet(runner.Measured))
	return &report.RankingReport{
		Level:             string(rk.Level),
		Indistinguishable: rk.Indistinguishable,
		Note:              rk.Note,
		WinnerID:          winnerID,
		RunnerUpID:        runnerID,
	}
}

// saferMargin reports whether `a` is the safer choice between two candidates
// whose performance is operationally indistinguishable. Deterministic order:
// higher capability rate, then larger measured VRAM headroom against the
// planning budget, then fewer error events; ties keep the scored order.
func saferMargin(a, b *candidate.Candidate, vramBudget uint64) bool {
	rateA, rateB := capRate(a), capRate(b)
	if rateA != rateB {
		return rateA > rateB
	}
	headA, headB := headroomFrac(a, vramBudget), headroomFrac(b, vramBudget)
	if headA != headB && headA >= 0 && headB >= 0 {
		return headA > headB
	}
	evA, evB := errEvents(a), errEvents(b)
	if evA != evB {
		return evA < evB
	}
	return false
}

func capRate(c *candidate.Candidate) float64 {
	if c.Measured != nil && c.Measured.Capability != nil {
		return c.Measured.Capability.Rate
	}
	return -1 // unknown sorts last
}

func headroomFrac(c *candidate.Candidate, vramBudget uint64) float64 {
	m := c.Measured
	if m == nil || m.PeakVRAM == 0 || vramBudget == 0 {
		return -1 // unknown sorts last
	}
	if m.PeakVRAM >= vramBudget {
		return 0
	}
	return float64(vramBudget-m.PeakVRAM) / float64(vramBudget)
}

func errEvents(c *candidate.Candidate) int {
	m := c.Measured
	if m == nil {
		return 0
	}
	return m.OOMEvents + m.Timeouts + m.RunsFailed
}

// assessConfidence derives the deterministic rating from measured evidence.
func assessConfidence(c *candidate.Candidate, vramBudget uint64) *confidence.Assessment {
	m := c.Measured
	f := confidence.Factors{
		GatePassed:      c.Gate != nil && c.Gate.Passed,
		SmokePassed:     smokePassed(m),
		SmokeTotal:      smokeTotal(m),
		PerfRunsOK:      m.RunsOK,
		PerfRunsFailed:  m.RunsFailed,
		PeakVRAMBytes:   m.PeakVRAM,
		VRAMBudgetBytes: vramBudget,
		OOMEvents:       m.OOMEvents,
		Timeouts:        m.Timeouts,
		Experimental:    c.Experimental,
	}
	for _, s := range m.PerfSamples {
		f.DecodeTPS = append(f.DecodeTPS, s.DecodeTPS)
	}
	if m.Capability != nil {
		f.HasCapability = true
		f.CapabilityRate = m.Capability.Rate
	}
	a := confidence.Assess(f)
	return &a
}

func smokePassed(m *candidate.Measurement) int {
	if m != nil && m.Smoke != nil {
		return m.Smoke.Passed
	}
	return 0
}

func smokeTotal(m *candidate.Measurement) int {
	if m != nil && m.Smoke != nil {
		return m.Smoke.Total
	}
	return 0
}

// rank orders candidates by profile-weighted utility among gate passers;
// deterministic tie-breaks preserve generation order.
func rank(cands []candidate.Candidate, p *workload.Profile) {
	type scored struct {
		idx int
		val float64
	}
	passers := []scored{}
	maxDecode, maxPrefill := 0.0, 0.0
	for i, c := range cands {
		if !passEligible(c) {
			continue
		}
		if c.Measured.DecodeTPS > maxDecode {
			maxDecode = c.Measured.DecodeTPS
		}
		if c.Measured.PrefillTPS > maxPrefill {
			maxPrefill = c.Measured.PrefillTPS
		}
		passers = append(passers, scored{i, 0})
	}
	for k := range passers {
		c := &cands[passers[k].idx]
		capScore := 1.0
		if c.Measured.Capability != nil {
			capScore = c.Measured.Capability.Rate
		}
		speed := 0.0
		if maxDecode > 0 {
			speed += 0.7 * c.Measured.DecodeTPS / maxDecode
		}
		if maxPrefill > 0 {
			speed += 0.3 * c.Measured.PrefillTPS / maxPrefill
		}
		val := p.QualityPriority*capScore + p.LatencyPriority*speed
		passers[k].val = val
		c.Score = val
	}
	sort.SliceStable(passers, func(a, b int) bool { return passers[a].val > passers[b].val })
	ordered := make([]candidate.Candidate, len(cands))
	used := map[int]bool{}
	next := 0
	for _, s := range passers {
		ordered[next] = cands[s.idx]
		used[s.idx] = true
		next++
	}
	for i := range cands {
		if !used[i] {
			ordered[next] = cands[i]
			next++
		}
	}
	copy(cands, ordered)
}

func passEligible(c candidate.Candidate) bool {
	return c.Feasible && c.Measured != nil && c.Measured.Error == "" &&
		c.Gate != nil && c.Gate.Passed
}

// ---- report glue ------------------------------------------------------

func modelSummary(m *gguf.ModelInfo) report.ModelSummary {
	moe := ""
	if m.MoE != nil {
		moe = fmt.Sprintf("%d experts total", m.MoE.TotalExperts)
		if m.MoE.ActiveExperts > 0 {
			moe = fmt.Sprintf("%d/%d active experts", m.MoE.ActiveExperts, m.MoE.TotalExperts)
		}
	}
	return report.ModelSummary{
		Path:         m.Path,
		Architecture: m.Architecture,
		Params:       gguf.FormatParams(m.ParamCount),
		Quant:        m.QuantLabel,
		Layers:       m.LayerCount,
		TrainContext: m.TrainContext,
		FileSizeGB:   float64(m.FileSize) / (1 << 30),
		MoE:          moe,
	}
}

func hardwareSummary(h *hardware.Info) report.HardwareSummary {
	out := report.HardwareSummary{
		RAMTotalGB: float64(h.RAM.TotalBytes) / (1 << 30),
		RAMAvailGB: float64(h.RAM.AvailableBytes) / (1 << 30),
		CPUModel:   h.CPU.ModelName,
		Threads:    h.CPU.Threads(),
		FSType:     h.Storage.FSType,
	}
	if h.Bandwidth.Measured {
		out.BandwidthGBps = h.Bandwidth.GBps
	}
	for _, g := range h.GPUs {
		name := g.Name
		if name == "" {
			name = strings.ToUpper(g.Vendor)
		}
		if g.VRAMTotalBytes > 0 {
			name += fmt.Sprintf(" %.0fGB", float64(g.VRAMTotalBytes)/(1<<30))
		}
		out.GPUs = append(out.GPUs, name)
	}
	return out
}

func toCandidateReport(c candidate.Candidate, totalLayers int64) report.CandidateReport {
	rc := report.CandidateReport{
		ID:               c.ID,
		Name:             c.Name,
		Status:           classifyStatus(c),
		Rationale:        c.Rationale,
		Slot:             c.Slot,
		Context:          c.Config.ContextTokens,
		KVCache:          c.Config.KVCacheType,
		GPULayers:        gpuLayersString(int64(c.Config.GPULayers), totalLayers),
		ExpertsOnCPU:     c.Config.ExpertsOnCPU,
		BatchSize:        c.Config.BatchSize,
		UBatchSize:       c.Config.UBatchSize,
		Experimental:     c.Experimental,
		ExperimentalNote: c.ExperimentalNote,
		Feasible:         c.Feasible,
		InfeasibleReason: c.InfeasibleReason,
		ProbeOnly:        c.ProbeOnly,
		DominatedBy:      c.DominatedBy,
	}
	if c.Confidence != nil {
		rc.Confidence = &report.ConfidenceReport{
			Level:     string(c.Confidence.Level),
			Positives: c.Confidence.Positives,
			Negatives: c.Confidence.Negatives,
		}
	}
	return rc
}

// classifyStatus maps a candidate's evidence to the product vocabulary.
// See internal/report status constants; the mapping is deterministic:
//
//	no verification (dry-run plan)      → SCREENED
//	infeasible per planning math        → REJECTED
//	OOM/timeout during verification     → REJECTED (stability failure)
//	frontier probe missing objective    → REJECTED (recorded evidence)
//	dominated by a measured point       → PROBED (budget saved, not evidence)
//	other measurement errors            → UNKNOWN (insufficient evidence)
//	capability gate failure             → REJECTED
//	gate passed                         → VERIFIED
func classifyStatus(c candidate.Candidate) string {
	if c.Measured == nil {
		if c.Feasible {
			return report.StatusScreened
		}
		return report.StatusRejected
	}
	if !c.Feasible {
		return report.StatusRejected
	}
	m := c.Measured
	if m.Error != "" {
		if m.OOMEvents > 0 || m.Timeouts > 0 {
			return report.StatusRejected
		}
		return report.StatusUnknown
	}
	if c.ProbeOnly && m.Smoke == nil {
		if m.ObjectiveMet != nil && !*m.ObjectiveMet {
			return report.StatusRejected // the frontier rejected it on evidence
		}
		return report.StatusProbed
	}
	if c.DominatedBy != "" && m.Capability == nil && m.Smoke == nil {
		return report.StatusProbed
	}
	if c.Gate == nil {
		return report.StatusUnknown
	}
	if !c.Gate.Passed {
		return report.StatusRejected
	}
	return report.StatusVerified
}

// referenceSection renders the REFERENCE selection policy for the report.
func referenceSection(ref *candidate.Candidate, totalLayers int64) *report.ReferenceSection {
	return &report.ReferenceSection{
		Name:       ref.Name,
		Context:    ref.Config.ContextTokens,
		KVCache:    strings.ToUpper(ref.Config.KVCacheType),
		GPULayers:  gpuLayersString(int64(ref.Config.GPULayers), totalLayers),
		ExpertsCPU: ref.Config.ExpertsOnCPU,
		Why:        ref.ReferenceWhy,
	}
}

// policySection renders the heuristic policy trace (Phase 7): separated
// facts, sourced axis decisions, and the candidate-slot budget outcome.
func policySection(gen *candidate.Generator, p *workload.Profile) *report.PolicySection {
	plan := gen.Plan()
	if plan == nil {
		return nil
	}
	ps := &report.PolicySection{
		WorkloadContract: []string{
			"objective: " + p.Objective,
			fmt.Sprintf("sensitivity: %s", sensitivityLabel(p)),
			fmt.Sprintf("hard context floor: %d tokens (candidates never plan below it)", p.MinContext),
		},
	}
	for _, d := range plan.Decisions {
		ps.Decisions = append(ps.Decisions, report.PolicyDecision{
			Axis:   string(d.Axis),
			Impact: string(d.Impact),
			Source: string(d.Source),
			Choice: d.Choice,
			Why:    d.Why,
		})
	}
	for _, s := range plan.Slots {
		ps.AdmittedSlots = append(ps.AdmittedSlots, string(s))
	}
	for _, s := range plan.Suppressed {
		ps.DeclinedSlots = append(ps.DeclinedSlots, report.PolicySuppression{
			Slot: string(s.Slot), Reason: s.Reason,
		})
	}
	return ps
}

// sensitivityLabel renders a profile's declared resource sensitivity.
func sensitivityLabel(p *workload.Profile) string {
	var parts []string
	if p.PrefillBound {
		parts = append(parts, "prefill-bound")
	}
	if p.DecodeBound {
		parts = append(parts, "decode-bound")
	}
	if p.DepthBound {
		parts = append(parts, "depth-bound")
	}
	if len(parts) == 0 {
		return "unclassified"
	}
	return strings.Join(parts, ", ")
}

// syncReportCandidates refreshes mutable fields after measurement.
func syncReportCandidates(rep *report.Report, cands []candidate.Candidate) {
	for i := range rep.Candidates {
		rc := &rep.Candidates[i]
		for _, c := range cands {
			if c.ID != rc.ID {
				continue
			}
			rc.Status = classifyStatus(c)
			rc.Feasible = c.Feasible
			rc.InfeasibleReason = c.InfeasibleReason
			rc.ProbeOnly = c.ProbeOnly
			rc.DominatedBy = c.DominatedBy
			if c.Measured != nil {
				rc.PrefillTPS = c.Measured.PrefillTPS
				rc.DecodeTPS = c.Measured.DecodeTPS
				rc.PeakVRAMGB = float64(c.Measured.PeakVRAM) / (1 << 30)
				rc.PerfRuns = len(c.Measured.PerfSamples)
				if len(c.Measured.PerfSamples) > 1 {
					lo, hi := c.Measured.PerfSamples[0].DecodeTPS, c.Measured.PerfSamples[0].DecodeTPS
					for _, s := range c.Measured.PerfSamples {
						if s.DecodeTPS < lo {
							lo = s.DecodeTPS
						}
						if s.DecodeTPS > hi {
							hi = s.DecodeTPS
						}
					}
					rc.DecodeHalfRange = (hi - lo) / 2
				}
				if c.Measured.Smoke != nil {
					rc.SmokePassed, rc.SmokeTotal = c.Measured.Smoke.Passed, c.Measured.Smoke.Total
				}
				if c.Measured.Capability != nil {
					rc.CapabilityPassed = c.Measured.Capability.Passed
					rc.CapabilityTotal = c.Measured.Capability.Total
					rc.CapabilityRate = c.Measured.Capability.Rate
				}
				rc.Error = c.Measured.Error
			}
			if c.Gate != nil {
				rc.GatePassed, rc.GateReason = c.Gate.Passed, c.Gate.Reason
			}
			rc.Score = c.Score
			if c.Confidence != nil {
				rc.Confidence = &report.ConfidenceReport{
					Level:     string(c.Confidence.Level),
					Positives: c.Confidence.Positives,
					Negatives: c.Confidence.Negatives,
				}
			}
			break
		}
	}
}

func gpuLayersString(n, totalLayers int64) string {
	switch {
	case n >= backend.MaxGPULayers:
		if totalLayers > 0 {
			return fmt.Sprintf("max (%d/%d)", totalLayers, totalLayers)
		}
		return "max"
	case n <= 0:
		return "0 (cpu)"
	default:
		if totalLayers > 0 {
			return fmt.Sprintf("%d/%d", n, totalLayers)
		}
		return fmt.Sprintf("%d", n)
	}
}

// ---- helpers ----------------------------------------------------------

func indexOfKind(cands []candidate.Candidate, k candidate.Kind) int {
	for i, c := range cands {
		if c.Kind == k {
			return i
		}
	}
	return -1
}

func findCandidate(cands []candidate.Candidate, id string) *candidate.Candidate {
	for i := range cands {
		if cands[i].ID == id {
			return &cands[i]
		}
	}
	return nil
}

func writeAuxArtifacts(dir string, cands []candidate.Candidate, hw *hardware.Info) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	if b, err := json.MarshalIndent(cands, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "candidates.json"), b, 0o644)
	}
	if b, err := json.MarshalIndent(hw, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "hardware.json"), b, 0o644)
	}
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}

func ptr[T any](v T) *T { return &v }
