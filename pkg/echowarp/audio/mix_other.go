//go:build !arm64

package audio

// hasNEON is false on non-arm64 platforms.
const hasNEON = false
