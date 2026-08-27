package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EffNine/gumi/internal/verify"
	"github.com/EffNine/gumi/internal/workload"
)

func runProfiles(args []string) {
	fs := newFlagSet("profiles")
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		osExit(2)
	}

	type profileJSON struct {
		Name             string                 `json:"name"`
		Description      string                 `json:"description"`
		Objective        string                 `json:"objective"`
		HardConstraints  []string               `json:"hard_constraints,omitempty"`
		PreferredMetrics []string               `json:"preferred_metrics,omitempty"`
		MinContext       int                    `json:"min_context"`
		QualityPriority  float64                `json:"quality_priority"`
		LatencyPriority  float64                `json:"latency_priority"`
		PrefillBound     bool                   `json:"prefill_bound,omitempty"`
		DecodeBound      bool                   `json:"decode_bound,omitempty"`
		DepthBound       bool                   `json:"depth_bound,omitempty"`
		SmokeTasks       int                    `json:"smoke_tasks"`
		CapabilityTasks  int                    `json:"capability_tasks"`
		Golden           []workload.GoldenGroup `json:"golden_groups,omitempty"`
		Notes            []string               `json:"notes,omitempty"`
	}

	golden := workload.Golden()
	var all []profileJSON
	for _, name := range workload.Names() {
		p, err := workload.Get(name)
		if err != nil {
			fail("%v", err)
		}
		all = append(all, profileJSON{
			Name: p.Name, Description: p.Description, MinContext: p.MinContext,
			QualityPriority: p.QualityPriority, LatencyPriority: p.LatencyPriority,
			PrefillBound: p.PrefillBound, DecodeBound: p.DecodeBound, DepthBound: p.DepthBound,
			SmokeTasks: len(p.SmokeTasks), CapabilityTasks: len(p.CapabilityTasks),
			Golden: golden[p.Name], Notes: p.Notes,
			Objective: p.Objective, HardConstraints: p.HardConstraints,
			PreferredMetrics: p.PreferredMetrics,
		})
		if *jsonOut {
			continue
		}
		fmt.Println(p.Name)
		fmt.Printf("  %s\n", p.Description)
		fmt.Printf("  objective: %s\n", p.Objective)
		for _, hc := range p.HardConstraints {
			fmt.Printf("  constraint: %s\n", hc)
		}
		fmt.Printf("  min context %d, quality %.2f / latency %.2f, tasks: %d smoke + %d capability\n",
			p.MinContext, p.QualityPriority, p.LatencyPriority,
			len(p.SmokeTasks), len(p.CapabilityTasks))
		if sens := sensitivityLabel(p); sens != "" {
			fmt.Printf("  sensitivity: %s\n", sens)
		}
		for _, t := range p.SmokeTasks {
			printTaskRef(t)
		}
		for _, t := range p.CapabilityTasks {
			printTaskRef(t)
		}
		if groups := golden[p.Name]; len(groups) > 0 {
			fmt.Println("  golden benchmark:")
			for _, g := range groups {
				fmt.Printf("    - %s [%s]\n", g.Name, g.Eval)
			}
		}
		for _, n := range p.Notes {
			fmt.Printf("  note: %s\n", n)
		}
	}
	if *jsonOut {
		b, _ := json.MarshalIndent(all, "", "  ")
		fmt.Println(string(b))
	}
}

func printTaskRef(t verify.Task) {
	fmt.Printf("    - [%s] %s (%s)\n", t.Tier.String(), t.ID, t.Category)
}

// sensitivityLabel renders the declared resource-sensitivity classification.
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
	return strings.Join(parts, ", ")
}
