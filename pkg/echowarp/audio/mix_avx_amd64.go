//go:build amd64 && gc && !noasm

package audio

// AVX SIMD implementations for x86_64.
// Requires AVX (Sandy Bridge+). Process 8 float32 per iteration using 256-bit YMM registers.

//go:noescape
func mixAccumulateAVX(dst, src []float32)

//go:noescape
func mixGainAVX(dst []float32, gain float32)

//go:noescape
func mixTanhAVX(dst []float32)

// SSE SIMD implementations for x86_64.
// Works on ALL amd64 CPUs (SSE2 guaranteed by x86_64 spec).
// Process 4 float32 per iteration using 128-bit XMM registers.

//go:noescape
func mixAccumulateSSE(dst, src []float32)

//go:noescape
func mixGainSSE(dst []float32, gain float32)

//go:noescape
func mixTanhSSE(dst []float32)
