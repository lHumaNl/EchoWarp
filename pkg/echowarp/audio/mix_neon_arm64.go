//go:build arm64 && gc && !noasm

package audio

// hasNEON is always true on arm64 (ARMv8 spec guarantees NEON support).
// No runtime check needed.
const hasNEON = true

// NEON SIMD implementations for ARM64.
// ARMv8 guarantees NEON support, so no runtime detection needed.
// Process 4 float32 per iteration using 128-bit V registers.

//go:noescape
func mixAccumulateNEON(dst, src []float32)

//go:noescape
func mixGainNEON(dst []float32, gain float32)

//go:noescape
func mixTanhNEON(dst []float32)
