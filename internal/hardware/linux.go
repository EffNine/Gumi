package hardware

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// probeCPU reads CPU topology (linux: /proc/cpuinfo).
func probeCPU() CPUInfo {
	data, err := readFile("/proc/cpuinfo")
	if err != nil {
		return CPUInfo{LogicalCores: runtime.NumCPU()}
	}
	return parseCPUInfo(data)
}

// readFile is a seam for tests.
var readFile = os.ReadFile

// parseCPUInfo extracts model name, logical count, and unique physical cores
// from /proc/cpuinfo contents. Pure function; unit-testable.
func parseCPUInfo(data []byte) CPUInfo {
	var cpu CPUInfo
	logical := 0
	type coreKey struct{ phys, core string }
	seen := map[coreKey]bool{}
	haveIDs := false
	lastPhys := ""

	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := splitProcKV(line)
		if !ok {
			continue
		}
		switch k {
		case "model name":
			if cpu.ModelName == "" {
				cpu.ModelName = v
			}
		case "processor":
			logical++
		case "physical id":
			haveIDs = true
			lastPhys = v
		case "core id":
			haveIDs = true
			seen[coreKey{lastPhys, v}] = true
		}
	}
	cpu.LogicalCores = logical
	switch {
	case haveIDs && len(seen) > 0:
		cpu.PhysicalCores = len(seen)
	default:
		cpu.PhysicalCores = 0 // unknown topology; never guess
	}
	return cpu
}

func splitProcKV(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:i])
	v := strings.TrimSpace(line[i+1:])
	if k == "" {
		return "", "", false
	}
	return k, v, true
}

// probeMemory reads MemTotal/MemAvailable (/proc/meminfo on linux).
func probeMemory() Memory {
	data, err := readFile("/proc/meminfo")
	if err != nil {
		return Memory{}
	}
	return parseMemInfo(data)
}

// parseMemInfo parses /proc/meminfo contents (KiB fields).
func parseMemInfo(data []byte) Memory {
	var m Memory
	totalKB, availKB, freeKB := int64(-1), int64(-1), int64(-1)
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := splitProcKV(line)
		if !ok {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch k {
		case "MemTotal":
			totalKB = n
		case "MemAvailable":
			availKB = n
		case "MemFree":
			freeKB = n
		}
	}
	if totalKB >= 0 {
		m.TotalBytes = uint64(totalKB) << 10
	}
	switch {
	case availKB >= 0:
		m.AvailableBytes = uint64(availKB) << 10
	case totalKB >= 0 && freeKB >= 0:
		m.AvailableBytes = uint64(freeKB) << 10 // degraded but honest
	}
	return m
}
