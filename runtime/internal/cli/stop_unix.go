//go:build !windows

package cli

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// isProcessRunning returns true if a process with the given PID exists.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// stopProcess reads the PID file, sends SIGTERM to the running process, waits
// up to 30 seconds for graceful shutdown, and falls back to SIGKILL. It
// returns true if the process was successfully stopped (or was already gone),
// and false if stopping failed. The PID file is always removed on success.
func stopProcess() bool {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Novexa does not appear to be running (no PID file found).")
			return true
		}
		fmt.Fprintf(os.Stderr, "failed to read PID file: %v\n", err)
		return false
	}

	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); err != nil {
		fmt.Fprintf(os.Stderr, "invalid PID file content: %s\n", strings.TrimSpace(string(data)))
		return false
	}

	if !isProcessRunning(pid) {
		fmt.Printf("Novexa does not appear to be running (PID %d not found).\n", pid)
		removePidFile()
		return true
	}

	fmt.Printf("Stopping Novexa (PID %d)...\n", pid)

	// Send SIGTERM for graceful shutdown.
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "failed to signal Novexa (PID %d): %v\n", pid, err)
		return false
	}

	// Poll up to 30 seconds for the process to exit.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessRunning(pid) {
			removePidFile()
			fmt.Printf("Novexa stopped (PID %d).\n", pid)
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Graceful timeout reached — send SIGKILL.
	fmt.Println("Graceful shutdown timed out, sending SIGKILL...")
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop Novexa (PID %d).\n", pid)
		return false
	}

	// Wait 2 seconds for SIGKILL to take effect.
	time.Sleep(2 * time.Second)
	if !isProcessRunning(pid) {
		removePidFile()
		fmt.Printf("Novexa forcefully stopped (PID %d).\n", pid)
		return true
	}

	fmt.Fprintf(os.Stderr, "Failed to stop Novexa (PID %d).\n", pid)
	return false
}
