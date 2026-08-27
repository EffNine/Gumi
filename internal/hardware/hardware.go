// Package hardware probes the local machine: GPUs, CPU, RAM, and storage.
//
// Every value is either detected or left unknown (zero value). Gumi never
// fabricates hardware facts. Probing is best-effort per platform; parsers are
// pure functions so results can be unit-tested against fixture output.
package hardware

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// GPU describes one graphics adapter visible to the system.
type GPU struct {
	Vendor            string `json:"vendor"`                       // nvidia | amd | intel | unknown
	Name              string `json:"name"`                         // marketing name, may be empty
	VRAMTotalBytes    uint64 `json:"vram_total_bytes,omitempty"`   // 0 = unknown
	VRAMFreeBytes     uint64 `json:"vram_free_bytes,omitempty"`    // 0 = unknown
	ComputeCapability string `json:"compute_capability,omitempty"` // e.g. "12.0", NVIDIA only
	DriverVersion     string `json:"driver_version,omitempty"`
	Source            string `json:"source"` // which detector produced this entry
}

// IsDiscrete reports whether this looks like a discrete accelerator.
func (g GPU) IsDiscrete() bool {
	return g.VRAMTotalBytes > 0 || g.Vendor == "nvidia" || g.Vendor == "amd"
}

// CPUInfo describes processor topology.
type CPUInfo struct {
	ModelName     string `json:"model_name,omitempty"`
	PhysicalCores int    `json:"physical_cores"` // 0 = unknown
	LogicalCores  int    `json:"logical_cores"`  // 0 = unknown
}

// Threads returns the best available thread count (physical preferred).
func (c CPUInfo) Threads() int {
	if c.PhysicalCores > 0 {
		return c.PhysicalCores
	}
	return c.LogicalCores
}

// Memory describes system RAM.
type Memory struct {
	TotalBytes     uint64 `json:"total_bytes"`     // 0 = unknown
	AvailableBytes uint64 `json:"available_bytes"` // 0 = unknown
}

// Storage describes the filesystem that holds the model file.
type Storage struct {
	Path        string `json:"path"`
	FSType      string `json:"fs_type,omitempty"`
	MmapCapable bool   `json:"mmap_capable"`
	Known       bool   `json:"known"`
}

// Bandwidth is an optional measured host-memory bandwidth sample.
type Bandwidth struct {
	GBps     float64 `json:"gbps,omitempty"`
	Measured bool    `json:"measured"`
}

// Info is the complete probe result.
type Info struct {
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	GPUs      []GPU     `json:"gpus"`
	CPU       CPUInfo   `json:"cpu"`
	RAM       Memory    `json:"ram"`
	Storage   Storage   `json:"storage"`
	Bandwidth Bandwidth `json:"bandwidth"`
	Notes     []string  `json:"notes,omitempty"`
}

// Options tunes probing behavior.
type Options struct {
	ModelPath        string // path whose filesystem is probed (defaults to cwd)
	MeasureBandwidth bool   // run the RAM bandwidth micro-benchmark (~1s)
}

// TotalVRAMBytes sums known discrete VRAM across GPUs (0 = none detected).
func (i *Info) TotalVRAMBytes() uint64 {
	var t uint64
	for _, g := range i.GPUs {
		t += g.VRAMTotalBytes
	}
	return t
}

// FreeVRAMBytes sums known free VRAM across GPUs (0 = unknown).
func (i *Info) FreeVRAMBytes() uint64 {
	var t uint64
	for _, g := range i.GPUs {
		t += g.VRAMFreeBytes
	}
	return t
}

// PrimaryGPU returns the first discrete GPU, or nil.
func (i *Info) PrimaryGPU() *GPU {
	for idx := range i.GPUs {
		if i.GPUs[idx].IsDiscrete() {
			return &i.GPUs[idx]
		}
	}
	return nil
}

// HasGPU reports whether any usable discrete accelerator was detected.
func (i *Info) HasGPU() bool { return i.TotalVRAMBytes() > 0 }

// Summary renders a one-line human description of the machine.
func (i *Info) Summary() string {
	var b strings.Builder
	if g := i.PrimaryGPU(); g != nil {
		name := g.Name
		if name == "" {
			name = strings.ToUpper(g.Vendor)
		}
		if g.VRAMTotalBytes > 0 {
			fmt.Fprintf(&b, "%s %.0fGB", name, float64(g.VRAMTotalBytes)/(1<<30))
		} else {
			b.WriteString(name)
		}
	} else {
		b.WriteString("no discrete GPU")
	}
	fmt.Fprintf(&b, ", %d threads CPU", i.CPU.Threads())
	if i.RAM.TotalBytes > 0 {
		fmt.Fprintf(&b, ", %.0fGB RAM", float64(i.RAM.TotalBytes)/(1<<30))
	}
	return b.String()
}

// Detect probes the machine. Failures degrade to unknown values plus notes;
// Detect itself only errors on unrecoverable problems (currently never).
func Detect(opts Options) (*Info, error) {
	info := &Info{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	path := opts.ModelPath
	if path == "" {
		path = "."
	}

	info.CPU = probeCPU()
	info.RAM = probeMemory()
	info.GPUs = detectGPUs(&info.Notes)
	sort.SliceStable(info.GPUs, func(a, b int) bool {
		return info.GPUs[a].VRAMTotalBytes > info.GPUs[b].VRAMTotalBytes
	})
	info.Storage = probeStorage(path)
	if info.Storage.Known {
		info.Notes = append(info.Notes,
			fmt.Sprintf("model filesystem %s (mmap capable: %v)", info.Storage.FSType, info.Storage.MmapCapable))
	}
	if opts.MeasureBandwidth {
		bw, err := MeasureMemoryBandwidth()
		if err != nil {
			info.Notes = append(info.Notes, fmt.Sprintf("bandwidth measurement failed: %v", err))
		} else {
			info.Bandwidth = bw
		}
	}
	return info, nil
}
