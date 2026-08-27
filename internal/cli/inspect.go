package cli

import (
	"fmt"
	"strconv"

	"github.com/EffNine/gumi/internal/gguf"
)

func runInspect(args []string) {
	fs := newFlagSet("inspect")
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		osExit(2)
	}
	if fs.NArg() != 1 {
		fs.Usage()
		osExit(2)
	}
	info, err := gguf.Inspect(fs.Arg(0))
	if err != nil {
		fail("%v", err)
	}
	if *jsonOut {
		printJSON(info)
		return
	}
	m := info
	fmt.Printf("Model:        %s\n", m.Name)
	fmt.Printf("Architecture: %s\n", m.Architecture)
	fmt.Printf("Parameters:   %s (%d)\n", gguf.FormatParams(m.ParamCount), m.ParamCount)
	fmt.Printf("Quantization: %s\n", m.QuantLabel)
	fmt.Printf("Layers:       %d\n", m.LayerCount)
	fmt.Printf("Hidden size:  %d\n", m.HiddenSize)
	fmt.Printf("Heads:        %d (KV: %d, head_dim %d)\n", m.HeadCount, m.KVHeadCount, m.HeadDim)
	if m.TrainContext > 0 {
		fmt.Printf("Train ctx:    %d tokens\n", m.TrainContext)
	} else {
		fmt.Println("Train ctx:    unknown")
	}
	if m.RopeFreqBase > 0 {
		line := fmt.Sprintf("RoPE base:    %.0f", m.RopeFreqBase)
		if m.RopeScaling != "" {
			line += fmt.Sprintf(" (scaling: %s", m.RopeScaling)
			if m.RopeScaleFact > 0 {
				line += fmt.Sprintf(", factor %.1f", m.RopeScaleFact)
			}
			line += ")"
		}
		fmt.Println(line)
	}
	if m.MoE != nil {
		fmt.Printf("MoE experts:  %d total", m.MoE.TotalExperts)
		if m.MoE.ActiveExperts > 0 {
			fmt.Printf(", %d active", m.MoE.ActiveExperts)
		}
		if m.MoE.ExpertFFNSize > 0 {
			fmt.Printf(", expert ffn %d", m.MoE.ExpertFFNSize)
		}
		fmt.Println()
		fmt.Printf("Expert bytes: %.2f GB of weights\n", float64(m.ExpertBytes)/(1<<30))
	}
	fmt.Printf("File size:    %.2f GB\n", float64(m.FileSize)/(1<<30))
	kv16 := m.KVBytesPerToken("f16")
	if kv16 > 0 {
		fmt.Printf("KV/token:     %s bytes at f16 (%.2f GB per 32k context)\n",
			formatInt(kv16), float64(kv16*32768)/(1<<30))
	}
}

func formatInt(n uint64) string {
	return strconv.FormatUint(n, 10)
}
