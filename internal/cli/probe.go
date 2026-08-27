package cli

import (
	"fmt"

	"github.com/EffNine/gumi/internal/hardware"
)

func runProbe(args []string) {
	fs := newFlagSet("probe")
	modelPath := fs.String("model", "", "path whose filesystem should be probed")
	bandwidth := fs.Bool("bandwidth", false, "measure RAM bandwidth (~1s)")
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		osExit(2)
	}

	info, err := hardware.Detect(hardware.Options{
		ModelPath:        *modelPath,
		MeasureBandwidth: *bandwidth,
	})
	if err != nil {
		fail("%v", err)
	}
	if *jsonOut {
		printJSON(info)
		return
	}
	fmt.Printf("OS/Arch:      %s/%s\n", info.OS, info.Arch)
	if info.CPU.ModelName != "" {
		fmt.Printf("CPU:          %s\n", info.CPU.ModelName)
	}
	switch {
	case info.CPU.PhysicalCores > 0:
		fmt.Printf("CPU cores:    %d physical, %d logical\n", info.CPU.PhysicalCores, info.CPU.LogicalCores)
	default:
		fmt.Printf("CPU cores:    %d logical (physical unknown)\n", info.CPU.LogicalCores)
	}
	if info.RAM.TotalBytes > 0 {
		fmt.Printf("RAM:          %.1f GB total, %.1f GB available\n",
			gb64(info.RAM.TotalBytes), gb64(info.RAM.AvailableBytes))
	} else {
		fmt.Println("RAM:          unknown")
	}
	for _, g := range info.GPUs {
		line := fmt.Sprintf("GPU:          %s [%s]", gpuName(g), g.Source)
		if g.VRAMTotalBytes > 0 {
			line += fmt.Sprintf(" VRAM %.1f GB total", gb64(g.VRAMTotalBytes))
			if g.VRAMFreeBytes > 0 {
				line += fmt.Sprintf(", %.1f GB free", gb64(g.VRAMFreeBytes))
			}
		} else {
			line += " VRAM unknown"
		}
		if g.ComputeCapability != "" {
			line += fmt.Sprintf(", compute capability %s", g.ComputeCapability)
		}
		fmt.Println(line)
	}
	if len(info.GPUs) == 0 {
		fmt.Println("GPU:          none detected")
	}
	if info.Storage.Known {
		mmap := "yes"
		if !info.Storage.MmapCapable {
			mmap = "no"
		}
		fmt.Printf("Storage:      %s (mmap capable: %s)\n", info.Storage.FSType, mmap)
	}
	if info.Bandwidth.Measured {
		fmt.Printf("Mem bandwidth: ~%.1f GB/s (measured estimate)\n", info.Bandwidth.GBps)
	}
	for _, n := range info.Notes {
		fmt.Printf("note:         %s\n", n)
	}
}

func gpuName(g hardware.GPU) string {
	if g.Name != "" {
		return g.Name
	}
	return g.Vendor + " GPU"
}

func gb64(b uint64) float64 { return float64(b) / (1 << 30) }
