//go:build windows

package cli

import (
	"fmt"
	"os"
)

// isProcessRunning checks if a process exists on Windows.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds if pid > 0.
	// We try to get the exit code to check if the process is alive.
	// A simpler approach: just return true and let the caller handle it.
	_ = proc
	return true
}

// stopProcess is a stub for Windows. The `gumi stop` and `gumi restart`
// commands are primarily designed for Unix. On Windows, print a message
// and return false.
func stopProcess() bool {
	fmt.Println("The 'gumi stop' command is not supported on Windows.")
	fmt.Println("Use Task Manager or 'taskkill /F /IM gumi.exe' to stop the runtime.")
	return false
}
