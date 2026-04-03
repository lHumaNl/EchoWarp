//go:build !unix && !darwin && !linux && !freebsd && !netbsd && !openbsd
// +build !unix,!darwin,!linux,!freebsd,!netbsd,!openbsd

package auth

// lockMemory is a no-op on platforms that don't support mlock.
// Returns false to indicate that memory locking is not available.
func lockMemory(b []byte) bool {
	return false
}

// unlockMemory is a no-op on platforms that don't support munlock.
func unlockMemory(b []byte) {
	// No-op on unsupported platforms
}
