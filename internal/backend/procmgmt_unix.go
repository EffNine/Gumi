//go:build !windows

package backend

import (
	"syscall"
)

// procGroupAttrs puts the child in its own process group so timeouts can kill
// the whole tree (llama.cpp may spawn helpers).
func procGroupAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup SIGKILLs everything in the child's process group.
func killProcessGroup(pid int) {
	if pid <= 1 {
		return // never signal init/reaper
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
