package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/optimize"
	"github.com/EffNine/gumi/internal/report"
	"github.com/EffNine/gumi/internal/verify"
)

// runTune implements the V1 product command:
//
//	gumi tune <model.gguf> [--workload W] [--min-decode X] [flags]
//
// The user gives Gumi a model (and optionally a workload); Gumi probes the
// machine, discovers backend capabilities, searches configurations and the
// context frontier on real measurements, verifies capability, and reports
// verified profiles. No llama.cpp knowledge required.
func runTune(args []string, command string) {
	args = normalizeArgs(args, map[string]bool{
		"workload": true, "out": true, "backend-bin": true,
		"tier": true, "timeout": true, "gate-slack": true,
		"baseline": true, "min-decode": true, "max-refine-steps": true,
	})
	fs := newFlagSet(command)
	workloadName := fs.String("workload", defaultWorkload(command),
		"workload profile: agentic_coding | chat")
	minDecode := fs.Float64("min-decode", 0,
		"absolute decode floor in tok/s for the frontier and recommendations (e.g. 25); "+
			"when unset, the workload's relative practicality rule applies")
	tier := fs.String("tier", "capability", "verification depth: smoke | capability")
	outDir := fs.String("out", "", "output directory (default: reports/<model>-<workload>-<timestamp>)")
	dryRun := fs.Bool("dry-run", false, "plan the search without running the backend")
	backendBin := fs.String("backend-bin", "", "path to llama-cli (default: search PATH)")
	timeoutMin := fs.Float64("timeout", 10, "per-run timeout in minutes")
	slack := fs.Float64("gate-slack", 0, "allowed capability regression vs reference (0..1)")
	bandwidth := fs.Bool("bandwidth", false, "measure RAM bandwidth (~1s)")
	perfRuns := fs.Int("perf-runs", 3, "perf probe repetitions per candidate (stability evidence)")
	refineSteps := fs.Int("max-refine-steps", 4, "context boundary refinement probes")
	baseline := fs.String("baseline", "", "human config spec to verify alongside Gumi's search, e.g. 'ngl=33,c=16384,kv=q8_0,fa'")
	if err := fs.Parse(args); err != nil {
		osExit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "error: %s requires a model path\n", command)
		fs.Usage()
		osExit(2)
	}

	var tierMax verify.Tier
	switch *tier {
	case "smoke":
		tierMax = verify.TierSmoke
	case "capability":
		tierMax = verify.TierCapability
	default:
		fail("unknown tier %q (use smoke|capability)", *tier)
	}

	printTunerHeader(fs.Arg(0), *workloadName)

	var lastStage string
	progress := func(ev optimize.Event) {
		switch ev.Kind {
		case optimize.EvStage:
			lastStage = ev.Text
			fmt.Printf("\n%s\n", ev.Text)
		case optimize.EvPass:
			fmt.Println("  " + ev.Text)
		case optimize.EvReject:
			fmt.Println("  " + ev.Text)
		case optimize.EvInfo:
			if strings.HasPrefix(ev.Text, "[SAME]") || strings.HasPrefix(ev.Text, "[SKIP]") {
				fmt.Println("  " + ev.Text)
			} else if lastStage != "" {
				fmt.Println("  " + ev.Text)
			}
		}
	}

	opts := optimize.Options{
		ModelPath:        fs.Arg(0),
		Workload:         *workloadName,
		TierMax:          tierMax,
		OutDir:           *outDir,
		DryRun:           *dryRun,
		BackendBin:       *backendBin,
		PerRunTimeout:    time.Duration(*timeoutMin * float64(time.Minute)),
		GateSlack:        *slack,
		MeasureBandwidth: *bandwidth,
		PerfRuns:         *perfRuns,
		MinDecode:        *minDecode,
		MaxRefineSteps:   *refineSteps,
		Progress:         progress,
	}
	if strings.TrimSpace(*baseline) != "" {
		opts.BaselineSpecs = []string{*baseline}
	}

	rep, dir, err := optimize.Run(context.Background(), opts)
	if err != nil {
		if rep != nil && dir != "" {
			fmt.Fprintf(os.Stderr, "\npartial artifacts written to %s\n", dir)
		}
		fail("%v", err)
	}

	fmt.Println()
	printTunerResult(rep, opts.DryRun)

	// Full evidence report follows the compact summary — same content as
	// report.md in the artifacts directory.
	fmt.Println(rep.RenderMarkdown())
	fmt.Printf("Full report and machine-readable results: %s\n", dir)

	if rep.Objective != nil && !rep.Objective.Achieved {
		osExit(1)
	}
	if rep.WinnerID == "" && !opts.DryRun {
		fmt.Println("No configuration passed verification.")
		osExit(1)
	}
}

// defaultWorkload: `tune` is the zero-configuration entry point.
func defaultWorkload(command string) string {
	if command == "optimize" {
		return "" // historical behavior: --workload is required there
	}
	return "agentic_coding"
}

// printTunerHeader renders the session banner from cheap local facts
// (GGUF header + hardware probe). The pipeline re-derives them; both come
// from the same deterministic sources.
func printTunerHeader(modelPath, workloadName string) {
	fmt.Println("GUMI AUTO-TUNER")
	fmt.Println()
	if m, err := gguf.Inspect(modelPath); err == nil {
		fmt.Println("Model:")
		name := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
		moe := ""
		if m.MoE != nil {
			if m.MoE.ActiveExperts > 0 {
				moe = fmt.Sprintf(" (MoE %d/%d experts)", m.MoE.ActiveExperts, m.MoE.TotalExperts)
			} else {
				moe = fmt.Sprintf(" (MoE %d experts)", m.MoE.TotalExperts)
			}
		}
		fmt.Printf("  %s — %s %s%s\n", name, gguf.FormatParams(m.ParamCount), m.QuantLabel, moe)
		fmt.Printf("  %s architecture, %d layers, training context %s\n",
			m.Architecture, m.LayerCount, ctxOrUnknownLabel(m.TrainContext))
	}
	if hw, err := hardware.Detect(hardware.Options{ModelPath: modelPath}); err == nil {
		fmt.Println()
		fmt.Println("Hardware:")
		for _, g := range hw.GPUs {
			name := g.Name
			if name == "" {
				name = strings.ToUpper(g.Vendor)
			}
			if g.VRAMTotalBytes > 0 {
				name += fmt.Sprintf(" %.0fGB", float64(g.VRAMTotalBytes)/(1<<30))
			}
			fmt.Printf("  %s\n", name)
		}
		if len(hw.GPUs) == 0 {
			fmt.Println("  no CUDA GPU detected (CPU-only execution)")
		}
	}
	fmt.Println()
	fmt.Printf("Workload:\n  %s\n", workloadName)
}

func ctxOrUnknownLabel(v int64) string {
	if v <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", v)
}

func labelK(ctx int) string {
	if ctx >= 1024 && ctx%1024 == 0 {
		return fmt.Sprintf("%dK", ctx/1024)
	}
	return fmt.Sprintf("%d", ctx)
}

func findReportCandidate(rep *report.Report, id string) *report.CandidateReport {
	if id == "" {
		return nil
	}
	for i := range rep.Candidates {
		if rep.Candidates[i].ID == id {
			return &rep.Candidates[i]
		}
	}
	return nil
}

func printTunerResult(rep *report.Report, dryRun bool) {
	if rep.Frontier != nil && rep.Frontier.MaxPractical > 0 {
		fmt.Printf("MAX PRACTICAL CONTEXT\n  %s tokens", labelK(rep.Frontier.MaxPractical))
		if f := findReportCandidate(rep, rep.Frontier.FrontierCandidateID); f != nil && !dryRun {
			fmt.Printf(" — decode %.1f tok/s, prefill %.1f tok/s", f.DecodeTPS, f.PrefillTPS)
		}
		fmt.Println()
		fmt.Println()
	}
	for _, p := range rep.Profiles {
		fmt.Println(strings.Join(p.Labels, " / "))
		line := fmt.Sprintf("  %s — context %s, KV %s", p.Name, labelK(p.Context), p.KVCache)
		if p.DecodeTPS > 0 {
			line += fmt.Sprintf(", decode %.1f tok/s", p.DecodeTPS)
		}
		if p.CapRate > 0 {
			line += fmt.Sprintf(", capability %.0f%%", p.CapRate*100)
		}
		fmt.Println(line)
		conf := p.Confidence
		if conf == "" && dryRun {
			conf = "planned"
		}
		if conf != "" {
			fmt.Printf("  confidence: %s\n", conf)
		}
		if len(p.TiedWith) > 0 {
			fmt.Printf("  operationally tied with: %s\n", strings.Join(p.TiedWith, ", "))
		}
		fmt.Println()
	}
	if rep.Objective != nil && !dryRun {
		if !rep.Objective.Achieved {
			fmt.Printf("TARGET NOT ACHIEVED\n  %s\n\n", rep.Objective.Statement)
		} else if rep.Objective.Statement != "" {
			fmt.Printf("Objective: %s\n\n", rep.Objective.Statement)
		}
	}
	if rep.WinnerID != "" {
		if w := findReportCandidate(rep, rep.WinnerID); w != nil {
			fmt.Printf("RECOMMENDED\n  %s (%s)\n", w.Name, w.ID)
		}
	} else if dryRun {
		fmt.Println("Dry run complete — plan only, no backend measurements.")
	}
}
