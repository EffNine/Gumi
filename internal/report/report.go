// Package report renders optimization results as markdown and JSON.
package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EffNine/gumi/internal/backend"
)

// ConfidenceReport is the deterministic confidence rating with its evidence.
type ConfidenceReport struct {
	Level     string   `json:"level"`
	Positives []string `json:"positives,omitempty"`
	Negatives []string `json:"negatives,omitempty"`
}

// Evidence statuses form the product's vocabulary. They are mutually
// exclusive per candidate and must never be conflated:
//
//	SCREENED     evaluated against the battery, but no verification ran
//	             (dry-run plans).
//	PROBED       exploratory frontier operating point with measured
//	             performance/stability evidence but no capability verdict
//	             of its own (also: dominated points whose battery budget
//	             was saved). PROBED rows are evidence, not recommendations.
//	VERIFIED     passed the capability gate and operational checks.
//	RECOMMENDED  a VERIFIED configuration selected as preferred operating
//	             point on measured evidence (implies VERIFIED).
//	REJECTED     failed capability or stability requirements (gate failure,
//	             infeasibility, OOM/timeout).
//	UNKNOWN      evidence insufficient for any determination (e.g.
//	             non-classified backend failure); confidence is never
//	             fabricated for these.
//
// OPERATIONALLY TIED is relational, not a status: it is expressed through
// Report.Ranking when two VERIFIED configurations sit within measurement
// uncertainty.
const (
	StatusScreened    = "SCREENED"
	StatusProbed      = "PROBED"
	StatusVerified    = "VERIFIED"
	StatusRecommended = "RECOMMENDED"
	StatusRejected    = "REJECTED"
	StatusUnknown     = "UNKNOWN"
)

// CandidateReport is one verified (or rejected) candidate in a report.
type CandidateReport struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status,omitempty"`
	Rationale        string `json:"rationale"`
	Slot             string `json:"slot,omitempty"` // policy slot provenance (Phase 7)
	Context          int    `json:"context_tokens"`
	KVCache          string `json:"kv_cache"`
	GPULayers        string `json:"gpu_layers"` // "max" or number
	BatchSize        int    `json:"batch_size,omitempty"`
	UBatchSize       int    `json:"ubatch_size,omitempty"`
	ExpertsOnCPU     bool   `json:"experts_on_cpu"`
	Experimental     bool   `json:"experimental,omitempty"`
	ExperimentalNote string `json:"experimental_note,omitempty"`
	Feasible         bool   `json:"feasible"`
	InfeasibleReason string `json:"infeasible_reason,omitempty"`
	ProbeOnly        bool   `json:"probe_only,omitempty"`
	DominatedBy      string `json:"dominated_by,omitempty"`

	PrefillTPS float64 `json:"prefill_tps,omitempty"`
	DecodeTPS  float64 `json:"decode_tps,omitempty"`
	PeakVRAMGB float64 `json:"peak_vram_gb,omitempty"`

	// Performance-stability evidence from repeated probes.
	PerfRuns        int     `json:"perf_runs,omitempty"`
	DecodeHalfRange float64 `json:"decode_half_range,omitempty"` // (max-min)/2 tok/s

	SmokePassed      int     `json:"smoke_passed,omitempty"`
	SmokeTotal       int     `json:"smoke_total,omitempty"`
	CapabilityPassed int     `json:"capability_passed,omitempty"`
	CapabilityTotal  int     `json:"capability_total,omitempty"`
	CapabilityRate   float64 `json:"capability_rate,omitempty"`

	GatePassed bool   `json:"gate_passed"`
	GateReason string `json:"gate_reason,omitempty"`
	Error      string `json:"error,omitempty"`

	Score      float64           `json:"score,omitempty"`
	Confidence *ConfidenceReport `json:"confidence,omitempty"`
}

// WasMeasured reports whether this candidate went through backend
// verification (false for dry-run plans).
func (c *CandidateReport) WasMeasured() bool {
	return c.SmokeTotal > 0 || c.CapabilityTotal > 0 || c.Error != "" ||
		c.DecodeTPS > 0 || c.PrefillTPS > 0
}

// ReferenceSection documents why the REFERENCE configuration was selected —
// it is never a random default but a policy-chosen baseline.
type ReferenceSection struct {
	Name       string   `json:"name"`
	Context    int      `json:"context_tokens"`
	KVCache    string   `json:"kv_cache"`
	GPULayers  string   `json:"gpu_layers"`
	ExpertsCPU bool     `json:"experts_on_cpu"`
	Why        []string `json:"why"`
}

// RankingReport captures how strongly the measured evidence supports the
// winner's ordering against the runner-up — a different question from
// per-candidate capability confidence.
type RankingReport struct {
	Level             string `json:"level"` // HIGH | MEDIUM | LOW
	Indistinguishable bool   `json:"indistinguishable,omitempty"`
	Note              string `json:"note,omitempty"`
	WinnerID          string `json:"winner_id"`
	RunnerUpID        string `json:"runner_up_id,omitempty"`
}

// PolicyDecision is one sourced choice from the heuristic policy layer.
// Source tells the reader whether a decision came from a hardware fact, a
// model fact, deterministic arithmetic, the workload contract, or a
// heuristic motivated by prior measurement — knowledge categories that must
// stay distinguishable (Phase 7).
type PolicyDecision struct {
	Axis   string `json:"axis"`
	Impact string `json:"impact"`
	Source string `json:"source"`
	Choice string `json:"choice"`
	Why    string `json:"why,omitempty"`
}

// PolicySuppression records a candidate slot the policy declined and why.
type PolicySuppression struct {
	Slot   string `json:"slot"`
	Reason string `json:"reason"`
}

// PolicySection answers "why did Gumi test these configurations?" It is a
// deterministic trace of the generation policy, not a justification of
// outcomes: capability verdicts come only from verification.
type PolicySection struct {
	WorkloadContract []string            `json:"workload_contract,omitempty"`
	Decisions        []PolicyDecision    `json:"decisions"`
	AdmittedSlots    []string            `json:"admitted_slots,omitempty"`
	DeclinedSlots    []PolicySuppression `json:"declined_slots,omitempty"`
}

// FrontierSection records the context-frontier search: what was probed,
// where the practical boundary landed, and why growth stopped. It keeps the
// THEORETICAL capacity (deterministic memory arithmetic) visibly separate
// from the MEASURED SAFE PRACTICAL context.
type FrontierSection struct {
	LineKV              string `json:"line_kv,omitempty"`
	LineExpertsCPU      bool   `json:"line_experts_cpu,omitempty"`
	TheoreticalMax      int    `json:"theoretical_max_context,omitempty"`
	MaxPractical        int    `json:"max_practical_context,omitempty"`
	CapabilityGated     bool   `json:"capability_gated,omitempty"`
	FrontierCandidateID string `json:"frontier_candidate_id,omitempty"`
	BoundaryReason      string `json:"boundary_reason,omitempty"`
	CoarseTested        []int  `json:"coarse_levels_tested,omitempty"`
	Refined             []int  `json:"refinement_probes,omitempty"`
}

// ObjectiveSection states the performance objective and its outcome.
// TARGET NOT ACHIEVED is a valid result and must never be masked.
type ObjectiveSection struct {
	UserFloorTPS      float64 `json:"user_floor_tps,omitempty"`
	Retention         float64 `json:"workload_retention,omitempty"`
	BaselineDecodeTPS float64 `json:"baseline_decode_tps,omitempty"`
	EffectiveFloorTPS float64 `json:"effective_floor_tps,omitempty"`
	Achieved          bool    `json:"achieved"`
	Statement         string  `json:"statement,omitempty"`
}

// BackendCapsSection discloses what the installed backend supports and which
// tuning dimensions were suppressed as a result.
type BackendCapsSection struct {
	Backend        string   `json:"backend"`
	Discovered     bool     `json:"discovered"`
	FlashAttention bool     `json:"flash_attention"`
	KVTunables     []string `json:"kv_cache_types_supported,omitempty"`
	OverrideTensor bool     `json:"expert_placement_supported,omitempty"`
	SingleTurn     bool     `json:"single_turn,omitempty"`
	Suppressed     []string `json:"suppressed_dimensions,omitempty"`
}

// ProfileEntry is one recommended operating-point role (QUALITY / BALANCED /
// SPEED / MAX CONTEXT) resolved from verified evidence.
type ProfileEntry struct {
	Labels      []string `json:"labels"`
	CandidateID string   `json:"candidate_id"`
	Name        string   `json:"name"`
	Context     int      `json:"context_tokens"`
	KVCache     string   `json:"kv_cache"`
	DecodeTPS   float64  `json:"decode_tps,omitempty"`
	PrefillTPS  float64  `json:"prefill_tps,omitempty"`
	PeakVRAMGB  float64  `json:"peak_vram_gb,omitempty"`
	CapRate     float64  `json:"capability_rate,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	TiedWith    []string `json:"tied_with,omitempty"`
	Note        string   `json:"note,omitempty"`
}

// Report is the full optimization output.
type Report struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Version     string            `json:"gumi_version"`
	Workload    string            `json:"workload"`
	Model       ModelSummary      `json:"model"`
	Hardware    HardwareSummary   `json:"hardware"`
	Reference   *ReferenceSection `json:"reference,omitempty"`
	Policy      *PolicySection    `json:"policy,omitempty"`
	Candidates  []CandidateReport `json:"candidates"`
	Ranking     *RankingReport    `json:"ranking,omitempty"`
	WinnerID    string            `json:"winner_id,omitempty"`

	// V1 tuning sections: empirical frontier, performance objective,
	// discovered backend capabilities, and the verified profile set.
	Frontier  *FrontierSection    `json:"frontier,omitempty"`
	Objective *ObjectiveSection   `json:"objective,omitempty"`
	Backend   *BackendCapsSection `json:"backend_capabilities,omitempty"`
	Profiles  []ProfileEntry      `json:"profiles,omitempty"`

	// Limitations carries pipeline-level disclosures (skipped toolchains,
	// tier caveats) merged into the report's Limitations section.
	Limitations []string `json:"limitations,omitempty"`

	Exports *backend.ExportBlock `json:"exports,omitempty"`
}

// ModelSummary describes the inspected model.
type ModelSummary struct {
	Path         string  `json:"path"`
	Architecture string  `json:"architecture"`
	Params       string  `json:"params"`
	Quant        string  `json:"quant"`
	Layers       int64   `json:"layers"`
	TrainContext int64   `json:"train_context"`
	FileSizeGB   float64 `json:"file_size_gb"`
	MoE          string  `json:"moe,omitempty"`
}

// HardwareSummary describes the probed machine.
type HardwareSummary struct {
	GPUs          []string `json:"gpus"`
	RAMTotalGB    float64  `json:"ram_total_gb,omitempty"`
	RAMAvailGB    float64  `json:"ram_available_gb,omitempty"`
	CPUModel      string   `json:"cpu_model,omitempty"`
	Threads       int      `json:"threads,omitempty"`
	FSType        string   `json:"filesystem,omitempty"`
	BandwidthGBps float64  `json:"bandwidth_gbps,omitempty"`
}

// RenderMarkdown produces the human-readable report. The layout answers the
// user's primary question first: "What should I run?" — then shows the
// evidence that justifies it.
func (r *Report) RenderMarkdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GUMI OPTIMIZATION REPORT\n\n")
	fmt.Fprintf(&b, "_Generated %s by gumi %s_\n\n", r.GeneratedAt.Format(time.RFC3339), r.Version)
	fmt.Fprintf(&b, "**Workload:** %s\n\n", r.Workload)

	m := r.Model
	fmt.Fprintf(&b, "## Model\n\n")
	fmt.Fprintf(&b, "- **%s** (%s, %s, %s)\n", filepath.Base(m.Path), m.Architecture, m.Params, m.Quant)
	if m.MoE != "" {
		fmt.Fprintf(&b, "  - MoE: %s\n", m.MoE)
	}
	fmt.Fprintf(&b, "  - Layers: %d, Training context: %s, File: %.1f GB\n",
		m.Layers, ctxOrUnknown(m.TrainContext), m.FileSizeGB)

	fmt.Fprintf(&b, "\n## Hardware\n\n")
	for _, g := range r.Hardware.GPUs {
		fmt.Fprintf(&b, "- GPU: %s\n", g)
	}
	if len(r.Hardware.GPUs) == 0 {
		b.WriteString("- GPU: none detected (CPU-only)\n")
	}
	if r.Hardware.RAMTotalGB > 0 {
		fmt.Fprintf(&b, "- RAM: %.0f GB total (%.0f GB available)\n",
			r.Hardware.RAMTotalGB, r.Hardware.RAMAvailGB)
	}
	if r.Hardware.CPUModel != "" {
		fmt.Fprintf(&b, "- CPU: %s (%d threads)\n", r.Hardware.CPUModel, r.Hardware.Threads)
	}
	if r.Hardware.FSType != "" {
		fmt.Fprintf(&b, "- Storage: %s\n", r.Hardware.FSType)
	}
	if r.Hardware.BandwidthGBps > 0 {
		fmt.Fprintf(&b, "- Memory bandwidth (measured): %.1f GB/s\n", r.Hardware.BandwidthGBps)
	}

	if r.Reference != nil {
		b.WriteString("\n## REFERENCE CONFIGURATION\n\n")
		ref := r.Reference
		fmt.Fprintf(&b, "**%s**: context %d, KV cache %s, GPU offload %s",
			ref.Name, ref.Context, ref.KVCache, ref.GPULayers)
		if ref.ExpertsCPU {
			b.WriteString(", experts in system RAM")
		}
		b.WriteString("\n\n**Why selected:**\n\n")
		for _, w := range ref.Why {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}

	r.writePolicy(&b)
	r.writeObjective(&b)
	r.writeBackendCaps(&b)
	r.writeFrontier(&b)

	b.WriteString("\n## Verified candidates\n\n")
	b.WriteString("| Config | Context | KV | GPU layers | Batch | Prefill tok/s | Decode tok/s | Capability | Confidence | Verdict |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Candidates {
		capCell := "-"
		if c.CapabilityTotal > 0 {
			capCell = fmt.Sprintf("%d/%d (%.0f%%)", c.CapabilityPassed, c.CapabilityTotal, c.CapabilityRate*100)
		} else if c.SmokeTotal > 0 {
			capCell = fmt.Sprintf("smoke %d/%d", c.SmokePassed, c.SmokeTotal)
		}
		confCell := "-"
		if c.Confidence != nil {
			confCell = c.Confidence.Level
		}
		name := c.Name
		if c.Experimental {
			name += " *(experimental)*"
		}
		verdict := "rejected"
		switch {
		case c.Error != "":
			verdict = "failed: " + c.Error
		case !c.Feasible:
			verdict = "infeasible"
		case c.GatePassed && c.ID == r.WinnerID:
			verdict = "**RECOMMENDED**"
		case c.GatePassed:
			verdict = "passed"
		case c.WasMeasured():
			verdict = c.GateReason
		default:
			verdict = "planned"
		}
		if c.Status != "" && verdict != "**RECOMMENDED**" {
			verdict = c.Status + " · " + verdict
		} else if c.Status != "" {
			verdict = c.Status
		}
		batchCell := "-"
		if c.BatchSize > 0 {
			batchCell = fmt.Sprintf("%d/%d", c.BatchSize, c.UBatchSize)
		}
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %.1f | %.1f | %s | %s | %s |\n",
			name, c.Context, strings.ToUpper(c.KVCache), c.GPULayers, batchCell,
			c.PrefillTPS, c.DecodeTPS, capCell, confCell, verdict)
	}

	w := r.findWinner()
	if w != nil {
		b.WriteString("\n## RECOMMENDED")
		if w.Confidence != nil && w.Confidence.Level == "HIGH" {
			b.WriteString(" ⭐")
		}
		fmt.Fprintf(&b, "\n\n### %s\n\n%s\n\n", w.Name, w.Rationale)

		b.WriteString("**Configuration:**\n\n")
		fmt.Fprintf(&b, "- Context: %d tokens\n", w.Context)
		fmt.Fprintf(&b, "- KV cache: %s\n", strings.ToUpper(w.KVCache))
		fmt.Fprintf(&b, "- GPU offload: %s\n", gpuLayersText(w.GPULayers))
		if w.BatchSize > 0 {
			fmt.Fprintf(&b, "- Batch: %d (ubatch %d)\n", w.BatchSize, w.UBatchSize)
		}
		if w.ExpertsOnCPU {
			b.WriteString("- Expert placement: experts in system RAM *(experimental)*\n")
		}
		if w.ExperimentalNote != "" {
			fmt.Fprintf(&b, "  - %s\n", w.ExperimentalNote)
		}

		if !w.WasMeasured() {
			b.WriteString("\n_Planned only (dry run): performance, capability, and confidence appear after a real verification run._\n")
		} else {
			fmt.Fprintf(&b, "\n**Performance (verified):**\n\n- Prefill: %.1f tok/s\n", w.PrefillTPS)
			if w.PerfRuns > 1 && w.DecodeHalfRange > 0 {
				fmt.Fprintf(&b, "- Decode: %.1f tok/s ± %.1f (%d runs)\n",
					w.DecodeTPS, w.DecodeHalfRange, w.PerfRuns)
			} else {
				fmt.Fprintf(&b, "- Decode: %.1f tok/s\n", w.DecodeTPS)
			}
			if w.PeakVRAMGB > 0 {
				fmt.Fprintf(&b, "- Peak VRAM: %.2f GB\n", w.PeakVRAMGB)
			}
			tier := "Tier 1 (smoke)"
			if w.CapabilityTotal > 0 {
				tier = fmt.Sprintf("Tier 2 PASSED (%d/%d)", w.CapabilityPassed, w.CapabilityTotal)
			} else if w.SmokeTotal > 0 {
				tier = fmt.Sprintf("Tier 1 PASSED (%d/%d)", w.SmokePassed, w.SmokeTotal)
			}
			fmt.Fprintf(&b, "\n**Capability:**\n\n%s\n", tier)

			if w.Confidence != nil {
				fmt.Fprintf(&b, "\n**Confidence:** %s\n", w.Confidence.Level)
				if len(w.Confidence.Positives) > 0 || len(w.Confidence.Negatives) > 0 {
					b.WriteString("\nEvidence:\n\n")
					for _, p := range w.Confidence.Positives {
						fmt.Fprintf(&b, "- + %s\n", p)
					}
					for _, n := range w.Confidence.Negatives {
						fmt.Fprintf(&b, "- − %s\n", n)
					}
				}
			}

			if r.Ranking != nil && r.Ranking.WinnerID == w.ID {
				fmt.Fprintf(&b, "\n**Ranking confidence:** %s\n", r.Ranking.Level)
				if r.Ranking.Note != "" {
					fmt.Fprintf(&b, "\nNote: %s\n", r.Ranking.Note)
				}
				if r.Ranking.Indistinguishable && r.Ranking.RunnerUpID != "" {
					if ru := r.findCandidateByID(r.Ranking.RunnerUpID); ru != nil {
						fmt.Fprintf(&b,
							"\n_Performance is operationally indistinguishable from %s within measurement noise; both are valid choices._\n",
							ru.Name)
					}
				}
			}
			r.writeAlternatives(&b, w)
		}
	}

	if r.Exports != nil {
		b.WriteString("\n## Exports\n\n")
		fmt.Fprintf(&b, "**llama.cpp (cli):**\n```bash\n%s\n```\n\n", r.Exports.LlamaCLI)
		fmt.Fprintf(&b, "**llama.cpp (server):**\n```bash\n%s\n```\n\n", r.Exports.LlamaServer)
		fmt.Fprintf(&b, "**LM Studio:**\n```json\n%s\n```\n\n", r.Exports.LMStudio)
		fmt.Fprintf(&b, "**Ollama (Modelfile):**\n```dockerfile\n%s\n```\n", r.Exports.Ollama)
	}

	r.writeProfiles(&b)
	r.writeRejected(&b)
	r.writeLimitations(&b)
	return b.String()
}

// writePolicy renders the deterministic generation-policy trace: which
// axes the policy prioritized (with sources), which candidate slots were
// spent, and which were declined with reasons. Heuristics chose what to
// test; they never decide what is safe.
func (r *Report) writePolicy(b *strings.Builder) {
	if r.Policy == nil {
		return
	}
	p := r.Policy
	b.WriteString("\n## WHY THESE CANDIDATES (generation policy)\n\n")
	for _, w := range p.WorkloadContract {
		fmt.Fprintf(b, "- %s\n", w)
	}
	if len(p.Decisions) > 0 {
		fmt.Fprintf(b, "\n| Axis | Impact | Source | Decision |\n|---|---|---|---|\n")
		for _, d := range p.Decisions {
			choice := d.Choice
			if d.Why != "" {
				choice += " — " + d.Why
			}
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", d.Axis, d.Impact, d.Source, choice)
		}
	}
	if len(p.AdmittedSlots) > 0 {
		b.WriteString("\n**Candidate slots used:**\n\n")
		for _, s := range p.AdmittedSlots {
			name := s
			for i := range r.Candidates {
				if r.Candidates[i].Slot == s {
					name = fmt.Sprintf("%s → **%s**", s, r.Candidates[i].Name)
					break
				}
			}
			fmt.Fprintf(b, "- %s\n", name)
		}
	}
	if len(p.DeclinedSlots) > 0 {
		b.WriteString("\n**Candidate slots declined:**\n\n")
		for _, s := range p.DeclinedSlots {
			fmt.Fprintf(b, "- %s — %s\n", s.Slot, s.Reason)
		}
	}
	b.WriteString("\n_Heuristics decided what to test; every capability claim comes from paired verification only._\n")
}

// writeRejected lists every non-passing configuration with its status and
// the concrete reason, so rejections are auditable.
func (r *Report) writeRejected(b *strings.Builder) {
	rows := make([]*CandidateReport, 0)
	for i := range r.Candidates {
		c := &r.Candidates[i]
		if c.Status == StatusRejected || c.Status == StatusUnknown {
			rows = append(rows, c)
		}
	}
	if len(rows) == 0 {
		b.WriteString("\n## Rejected configurations\n\nNone — all evaluated candidates passed the capability gate.\n")
		return
	}
	b.WriteString("\n## Rejected configurations\n\n")
	for _, c := range rows {
		reason := c.GateReason
		switch {
		case c.Error != "":
			reason = c.Error
		case !c.Feasible && c.InfeasibleReason != "":
			reason = c.InfeasibleReason
		}
		fmt.Fprintf(b, "- **%s** — %s: %s\n", c.Name, c.Status, reason)
	}
}

// writeLimitations surfaces everything a reader needs to judge evidence
// strength: skipped toolchains, thin repetitions, ties, unknown statuses,
// experimental labels. Omitted when there is nothing to disclose.
func (r *Report) writeLimitations(b *strings.Builder) {
	var items []string
	for _, c := range r.Candidates {
		if c.Status == StatusUnknown {
			items = append(items, fmt.Sprintf("%s could not be determined (%s)", c.Name, firstNonEmpty(c.Error, "insufficient evidence")))
		}
		if c.Experimental {
			items = append(items, c.Name+" relies on experimental expert placement; behavior varies across driver/backend versions")
		}
		if c.Confidence == nil && c.WasMeasured() {
			items = append(items, c.Name+" lacks a confidence rating (missing measurement data)")
		}
	}
	if r.Ranking != nil && r.Ranking.Level == "LOW" {
		items = append(items, "ranking confidence is LOW: "+firstNonEmpty(r.Ranking.Note, "measurement evidence does not support an ordering"))
	}
	for _, n := range r.Limitations {
		items = append(items, n)
	}
	b.WriteString("\n## Limitations\n\n")
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it] {
			continue
		}
		seen[it] = true
		fmt.Fprintf(b, "- %s\n", it)
	}
}

// writeObjective states what "good" meant for this run and whether the
// evidence achieved it. TARGET NOT ACHIEVED is a first-class outcome.
func (r *Report) writeObjective(b *strings.Builder) {
	if r.Objective == nil {
		return
	}
	o := r.Objective
	b.WriteString("\n## PERFORMANCE OBJECTIVE\n\n")
	fmt.Fprintf(b, "- **Target:** %s\n", firstNonEmpty(o.Statement, objectiveText(o)))
	if o.UserFloorTPS > 0 {
		fmt.Fprintf(b, "- User floor: %.1f tok/s decode\n", o.UserFloorTPS)
	}
	if o.Retention > 0 {
		fmt.Fprintf(b, "- Workload practicality: retain >= %.0f%% of best measured decode (%.1f tok/s baseline)\n",
			o.Retention*100, o.BaselineDecodeTPS)
	}
	if !o.Achieved {
		b.WriteString("\n**TARGET NOT ACHIEVED.** The configurations verified on this machine did not\n")
		b.WriteString("reach the stated objective. This is a valid result: Gumi does not fabricate a winner.\n")
	}
}

func objectiveText(o *ObjectiveSection) string {
	if o.EffectiveFloorTPS > 0 {
		return fmt.Sprintf("decode >= %.1f tok/s", o.EffectiveFloorTPS)
	}
	return "stable execution ranked by workload utility"
}

// writeBackendCaps discloses discovered backend capabilities and any tuning
// dimensions suppressed because this build cannot express them.
func (r *Report) writeBackendCaps(b *strings.Builder) {
	if r.Backend == nil {
		return
	}
	c := r.Backend
	b.WriteString("\n## BACKEND CAPABILITIES\n\n")
	discovered := "no (--help probe empty; retry chain arbitrates)"
	if c.Discovered {
		discovered = "yes"
	}
	fmt.Fprintf(b, "- Backend: %s (capabilities discovered: %s)\n", c.Backend, discovered)
	fmt.Fprintf(b, "- Flash attention support: %v; expert placement (-ot): %v\n", c.FlashAttention, c.OverrideTensor)
	if len(c.KVTunables) > 0 {
		fmt.Fprintf(b, "- KV cache types tunable: %s\n", strings.Join(c.KVTunables, ", "))
	} else {
		b.WriteString("- KV cache types tunable: none detected\n")
	}
	for _, s := range c.Suppressed {
		fmt.Fprintf(b, "- Suppressed: %s\n", s)
	}
}

// writeFrontier separates THEORETICAL capacity from MEASURED SAFE PRACTICAL
// context — the two are never conflated.
func (r *Report) writeFrontier(b *strings.Builder) {
	if r.Frontier == nil {
		return
	}
	f := r.Frontier
	b.WriteString("\n## CONTEXT FRONTIER\n\n")
	if f.TheoreticalMax > 0 {
		fmt.Fprintf(b, "- Theoretical capacity (memory arithmetic): %d tokens\n", f.TheoreticalMax)
	} else {
		b.WriteString("- Theoretical capacity (memory arithmetic): unknown\n")
	}
	if f.MaxPractical > 0 {
		gated := ""
		if f.CapabilityGated && f.FrontierCandidateID != "" {
			gated = ", capability VERIFIED at this window"
		} else if f.FrontierCandidateID == "" {
			gated = " (anchored at the workload minimum)"
		}
		fmt.Fprintf(b, "- **Max practical context (measured): %d tokens%s**\n", f.MaxPractical, gated)
	}
	if f.BoundaryReason != "" {
		fmt.Fprintf(b, "- Boundary: %s\n", f.BoundaryReason)
	}
	if len(f.CoarseTested) > 0 {
		fmt.Fprintf(b, "- Coarse levels probed: %s\n", ctxList(f.CoarseTested))
	}
	if len(f.Refined) > 0 {
		fmt.Fprintf(b, "- Refinement probes: %s\n", ctxList(f.Refined))
	}
}

// writeProfiles renders the multi-profile recommendation set. Profiles that
// collapse onto one configuration are reported as such — no manufactured
// distinctions.
func (r *Report) writeProfiles(b *strings.Builder) {
	if len(r.Profiles) == 0 {
		return
	}
	b.WriteString("\n## VERIFIED PROFILES\n\n")
	for _, p := range r.Profiles {
		fmt.Fprintf(b, "### %s\n\n", strings.Join(p.Labels, " / "))
		fmt.Fprintf(b, "- Config: %s, %s KV, context %d tokens (`%s`)\n",
			p.Name, p.KVCache, p.Context, p.CandidateID)
		if p.DecodeTPS > 0 {
			fmt.Fprintf(b, "- Decode: %.1f tok/s\n", p.DecodeTPS)
		}
		if p.PrefillTPS > 0 {
			fmt.Fprintf(b, "- Prefill: %.1f tok/s\n", p.PrefillTPS)
		}
		if p.PeakVRAMGB > 0 {
			fmt.Fprintf(b, "- Peak VRAM: %.2f GB\n", p.PeakVRAMGB)
		}
		if p.CapRate > 0 {
			fmt.Fprintf(b, "- Capability: %.0f%%\n", p.CapRate*100)
		}
		if p.Confidence != "" {
			fmt.Fprintf(b, "- Confidence: %s\n", p.Confidence)
		}
		if len(p.TiedWith) > 0 {
			fmt.Fprintf(b, "- Operationally tied with: %s (within measurement noise)\n", strings.Join(p.TiedWith, ", "))
		}
		if p.Note != "" {
			fmt.Fprintf(b, "- Note: %s\n", p.Note)
		}
		b.WriteString("\n")
	}
}

func ctxList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		if x >= 1024 && x%1024 == 0 {
			parts[i] = fmt.Sprintf("%dK", x/1024)
		} else {
			parts[i] = fmt.Sprintf("%d", x)
		}
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// writeAlternatives renders one-line tradeoff summaries for every other gate
// passer so users see the realistic options next to the recommendation.
func (r *Report) writeAlternatives(b *strings.Builder, winner *CandidateReport) {
	alts := make([]*CandidateReport, 0, len(r.Candidates))
	for i := range r.Candidates {
		c := &r.Candidates[i]
		if c.ID != winner.ID && c.GatePassed && c.Feasible &&
			c.Error == "" && (c.DecodeTPS > 0 || c.CapabilityTotal > 0) {
			alts = append(alts, c)
		}
	}
	if len(alts) == 0 {
		return
	}
	b.WriteString("\n### Alternatives\n\n")
	for _, a := range alts {
		line := tradeoff(a, winner)
		if r.Ranking != nil && r.Ranking.Indistinguishable && a.ID == r.Ranking.RunnerUpID {
			line += "; **operationally tied with the recommendation**"
		}
		fmt.Fprintf(b, "- **%s**: %s\n", a.Name, line)
	}
}

// tradeoff describes how an alternative differs from the winner,
// deterministically derived from measured numbers.
func tradeoff(a, w *CandidateReport) string {
	parts := []string{}
	if w.DecodeTPS > 0 && a.DecodeTPS > 0 {
		delta := a.DecodeTPS/w.DecodeTPS - 1
		switch {
		case delta >= 0.15:
			parts = append(parts, fmt.Sprintf("decode %.0f%% faster", delta*100))
		case delta <= -0.15:
			parts = append(parts, fmt.Sprintf("decode %.0f%% slower", -delta*100))
		default:
			parts = append(parts, "similar decode speed")
		}
	}
	if a.Context != w.Context {
		if a.Context > w.Context {
			parts = append(parts, fmt.Sprintf("larger context (%d)", a.Context))
		} else {
			parts = append(parts, fmt.Sprintf("smaller context (%d)", a.Context))
		}
	}
	kvRank := map[string]int{"f16": 3, "q8_0": 2, "q4_0": 1}
	if kvRank[a.KVCache] != kvRank[w.KVCache] {
		if kvRank[a.KVCache] > kvRank[w.KVCache] {
			parts = append(parts, "higher KV precision")
		} else {
			parts = append(parts, "lower KV precision")
		}
	}
	if len(parts) == 0 {
		return "same capability gate passed, different resource profile"
	}
	s := strings.Join(parts, ", ")
	if a.Experimental {
		s += "; experimental expert placement"
	}
	return s
}

func (r *Report) findWinner() *CandidateReport {
	for i := range r.Candidates {
		if r.Candidates[i].ID == r.WinnerID {
			return &r.Candidates[i]
		}
	}
	return nil
}

func (r *Report) findCandidateByID(id string) *CandidateReport {
	for i := range r.Candidates {
		if r.Candidates[i].ID == id {
			return &r.Candidates[i]
		}
	}
	return nil
}

func ctxOrUnknown(v int64) string {
	if v <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d", v)
}

func gpuLayersText(s string) string {
	if strings.HasPrefix(s, "max") || s == "999" {
		return "Maximum"
	}
	return s
}

// WriteArtifacts writes report.md, report.json under dir.
func (r *Report) WriteArtifacts(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mdPath := filepath.Join(dir, "report.md")
	if err := os.WriteFile(mdPath, []byte(r.RenderMarkdown()), 0o644); err != nil {
		return err
	}
	jsonBytes, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.json"), jsonBytes, 0o644)
}
