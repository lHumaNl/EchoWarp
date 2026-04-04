//go:build !darwin && !linux && !windows

package cli

// systemMemoryMB returns 0 on unsupported platforms.
func systemMemoryMB() uint64 {
	return 0
}
