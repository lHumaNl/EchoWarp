//go:build darwin

package cli

import (
	"encoding/binary"
	"syscall"
)

// systemMemoryMB returns total physical RAM in megabytes on macOS.
// hw.memsize is a 64-bit value on all modern macOS systems.
func systemMemoryMB() uint64 {
	b, err := syscall.Sysctl("hw.memsize")
	if err != nil {
		return 0
	}
	// Sysctl returns a null-terminated C string; we need the raw bytes.
	// The kernel encodes hw.memsize as little-endian uint64 in the byte slice.
	raw := []byte(b)
	// Pad to 8 bytes if needed
	for len(raw) < 8 {
		raw = append(raw, 0)
	}
	totalBytes := binary.LittleEndian.Uint64(raw[:8])
	return totalBytes / (1024 * 1024)
}
