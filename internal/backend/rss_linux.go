//go:build linux

package backend

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// readProcessRSS reads VmRSS from /proc/<pid>/status (linux).
func readProcessRSS(pid int) (uint64, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb << 10, true
	}
	return 0, false
}
