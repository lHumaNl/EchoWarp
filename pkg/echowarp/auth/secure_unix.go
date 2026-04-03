//go:build unix || darwin || linux || freebsd || netbsd || openbsd
// +build unix darwin linux freebsd netbsd openbsd

package auth

import (
	"syscall"
)

// lockMemory attempts to lock the memory region to prevent swapping.
// Returns true if successful, false otherwise.
// This is a best-effort operation - failure is non-fatal.
func lockMemory(b []byte) bool {
	if len(b) == 0 {
		return false
	}

	// mlock the memory region to prevent it from being swapped to disk
	// This is important for keeping sensitive data (keys, passwords) secure
	err := syscall.Mlock(b)
	return err == nil
}

// unlockMemory unlocks a previously locked memory region.
func unlockMemory(b []byte) {
	if len(b) == 0 {
		return
	}

	// Best effort unlock - ignore errors
	_ = syscall.Munlock(b) //nolint:errcheck
}
