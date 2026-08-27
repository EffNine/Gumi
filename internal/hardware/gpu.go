package hardware

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// detectGPUs runs available GPU detectors in priority order:
// nvidia-smi, rocm-smi, then lspci as a vendor-only fallback.
func detectGPUs(notes *[]string) []GPU {
	var gpus []GPU

	if out, err := runCommand("nvidia-smi",
		"--query-gpu=name,memory.total,memory.used,driver_version,compute_cap",
		"--format=csv,noheader,nounits"); err == nil {
		for _, g := range parseNvidiaSMI(out) {
			g.Source = "nvidia-smi"
			gpus = append(gpus, g)
		}
	} else if out, err := runCommand("nvidia-smi",
		"--query-gpu=name,memory.total,memory.used,driver_version",
		"--format=csv,noheader,nounits"); err == nil {
		// Older drivers without compute_cap support.
		for _, g := range parseNvidiaSMI(out) {
			g.Source = "nvidia-smi"
			gpus = append(gpus, g)
		}
	} else {
		*notes = append(*notes, "nvidia-smi unavailable (no NVIDIA driver or tooling)")
	}

	if out, err := runCommand("rocm-smi", "--showmeminfo", "vram", "--showproductname", "--json"); err == nil && strings.TrimSpace(out) != "" {
		for _, g := range parseROCmSMI(out) {
			g.Source = "rocm-smi"
			gpus = append(gpus, g)
		}
	}

	if !hasVendor(gpus, "amd") {
		if out, err := runCommand("lspci", "-nn"); err == nil {
			gpus = append(gpus, parseLspciFallback(out, gpus)...)
		}
	}
	if len(gpus) == 0 {
		*notes = append(*notes, "no discrete GPU detected")
	}
	return gpus
}

func hasVendor(gpus []GPU, vendor string) bool {
	for _, g := range gpus {
		if g.Vendor == vendor {
			return true
		}
	}
	return false
}

// parseNvidiaSMI parses `nvidia-smi --query-gpu=... --format=csv,noheader`.
// Fields: name,memory.total,memory.used[,driver][,compute_cap] (MiB units).
func parseNvidiaSMI(out string) []GPU {
	var gpus []GPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		g := GPU{
			Vendor: "nvidia",
			Name:   strings.TrimSpace(fields[0]),
		}
		totalMiB, _ := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
		usedMiB, _ := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64)
		g.VRAMTotalBytes = totalMiB << 20
		if totalMiB >= usedMiB {
			g.VRAMFreeBytes = (totalMiB - usedMiB) << 20
		}
		if len(fields) >= 4 {
			g.DriverVersion = strings.TrimSpace(fields[3])
		}
		if len(fields) >= 5 {
			g.ComputeCapability = strings.TrimSpace(fields[4])
		}
		gpus = append(gpus, g)
	}
	return gpus
}

// rocmmiJSON is the subset of rocm-smi --json output we consume.
type rocmmiJSON struct {
	CardList []struct {
		ID                   int    `json:"id"`
		VRAMTotalMemoryBytes uint64 `json:"VRAM TOTAL MEMORY (B)"`
		VRAMUsedMemoryBytes  uint64 `json:"VRAM USED MEMORY (B)"`
		VramProductName      string `json:"Vram Product Name,omitempty"`
	} `json:"card_list"`
}

func gpuJSON(vendor, name string, total, free uint64) GPU {
	return GPU{Vendor: vendor, Name: name, VRAMTotalBytes: total, VRAMFreeBytes: free}
}

// parseROCmSMI parses `rocm-smi --showmeminfo vram --showproductname --json`.
func parseROCmSMI(out string) []GPU {
	var parsed rocmmiJSON
	if err := unmarshalJSON([]byte(out), &parsed); err != nil {
		return nil
	}
	var gpus []GPU
	for _, c := range parsed.CardList {
		name := c.VramProductName
		if name == "" {
			name = fmt.Sprintf("AMD GPU %d", c.ID)
		}
		free := uint64(0)
		if c.VRAMTotalMemoryBytes >= c.VRAMUsedMemoryBytes {
			free = c.VRAMTotalMemoryBytes - c.VRAMUsedMemoryBytes
		}
		gpus = append(gpus, gpuJSON("amd", name, c.VRAMTotalMemoryBytes, free))
	}
	return gpus
}

// lspciEntry matches the vendor:device ids we care about.
var lspciVendors = map[string]string{
	"10de": "nvidia",
	"1002": "amd",
	"8086": "intel",
	"1022": "amd",
}

var lspciClasses = []string{"VGA compatible controller", "3D controller", "Display controller"}

// parseLspciFallback lists adapters with known vendors not already detected.
// VRAM is unknown from lspci alone — entries carry no memory facts.
// The fallback is vendor-only: if a vendor already has VRAM-backed data
// from nvidia-smi/rocm-smi, no lspci entry for that vendor is added — it
// would duplicate the device without useful data and pollute the report.
func parseLspciFallback(out string, existing []GPU) []GPU {
	haveVendor := map[string]bool{}
	haveKey := map[string]bool{}
	for _, g := range existing {
		haveKey[g.Vendor+"/"+g.Name] = true
		if g.VRAMTotalBytes > 0 {
			haveVendor[g.Vendor] = true
		}
	}
	var gpus []GPU
	for _, line := range strings.Split(out, "\n") {
		matchedClass := false
		for _, cls := range lspciClasses {
			if strings.Contains(line, cls) {
				matchedClass = true
				break
			}
		}
		if !matchedClass {
			continue
		}
		bracket := strings.LastIndex(line, "[")
		if bracket < 0 {
			continue
		}
		ids := line[bracket+1:]
		if closeIdx := strings.Index(ids, "]"); closeIdx >= 0 {
			ids = ids[:closeIdx]
		}
		parts := strings.Split(ids, ":")
		if len(parts) < 2 {
			continue
		}
		vendorID := strings.ToLower(parts[len(parts)-2])
		vendor, ok := lspciVendors[vendorID]
		if !ok {
			continue
		}
		if haveVendor[vendor] {
			continue
		}
		// Extract the device name: the segment after "]: " in lspci's
		// "<slot> <class> [<class-id>]: <vendor> <device> [<vendor:device>] ..." format.
		// Using the first colon yields the slot prefix ("00.0 VGA ..."), not the vendor.
		devName := ""
		if idx := strings.Index(line, "]: "); idx >= 0 {
			devName = strings.TrimSpace(line[idx+3:])
			if i := strings.Index(devName, " ["); i >= 0 {
				devName = strings.TrimSpace(devName[:i])
			}
		}
		if devName == "" {
			devName = strings.TrimSpace(line[strings.LastIndex(line, ":")+1:])
			if i := strings.Index(devName, " ["); i >= 0 {
				devName = strings.TrimSpace(devName[:i])
			}
		}
		key := vendor + "/" + devName
		if haveKey[key] {
			continue
		}
		haveKey[key] = true
		gpus = append(gpus, GPU{Vendor: vendor, Name: devName, Source: "lspci"})
	}
	return gpus
}

// runCommand executes a detector binary and returns trimmed stdout.
func runCommand(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
