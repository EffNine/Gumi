//go:build windows

package backend

import "syscall"

// procGroupAttrs is a no-op on windows (CREATE_NEW_PROCESS_GROUP would need
// syscall.SysProcAttr{CreationFlags}); plain Kill is used there.
func procGroupAttrs() *syscall.SysProcAttr {
	return nil
}

// killProcessGroup is a no-op on windows in the MVP.
func killProcessGroup(pid int) {}
