//go:build !linux

package backend

import (
	"os/exec"
	"strconv"
	"strings"
)

// readProcessRSS reads total RSS of a process on non-linux platforms via ps.
// Returns false when unavailable; metrics stay unknown rather than guessed.
func readProcessRSS(pid int) (uint64, bool) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, false
	}
	kb, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, false
	}
	return kb << 10, true
}
