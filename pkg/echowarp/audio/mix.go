// Package audio provides audio processing utilities including SIMD-optimized mixing operations.
package audio

import (
	"sync"

	"golang.org/x/sys/cpu"
)

var simdMu sync.RWMutex

// mixFunc is the function signature for audio sample accumulation.
type mixFunc func(dst, src []float32)

// mixGainFunc is the function signature for applying gain to audio samples.
type mixGainFunc func(dst []float32, gain float32)

// mixTanhFunc is the function signature for applying tanh soft clipping.
type mixTanhFunc func(dst []float32)

// Function pointers initialized at startup based on CPU capabilities.
var (
	mixAccumulateFn mixFunc
	mixGainFn       mixGainFunc
	mixTanhFn       mixTanhFunc
)

// hasSIMDSupport indicates whether SIMD optimizations are being used.
var hasSIMDSupport bool

// simdLevel describes which SIMD level is active.
var simdLevel string

func init() {
	selectSIMD()
}

// selectSIMD detects CPU features and assigns the best available implementations.
// Priority: AVX (8 floats/op) > SSE (4 floats/op) > NEON (4 floats/op) > Pure Go.
func selectSIMD() {
	simdMu.Lock()
	defer simdMu.Unlock()
	selectSIMDLocked()
}

func selectSIMDLocked() {
	// x86_64: AVX (Sandy Bridge+, 256-bit, 8 floats per op)
	if cpu.X86.HasAVX {
		mixAccumulateFn = mixAccumulateAVX
		mixGainFn = mixGainAVX
		mixTanhFn = mixTanhAVX
		hasSIMDSupport = true
		simdLevel = "AVX"
		return
	}

	// x86_64: SSE (all amd64 CPUs, 128-bit, 4 floats per op)
	if cpu.X86.HasSSE2 {
		mixAccumulateFn = mixAccumulateSSE
		mixGainFn = mixGainSSE
		mixTanhFn = mixTanhSSE
		hasSIMDSupport = true
		simdLevel = "SSE"
		return
	}

	// ARM64: NEON (ARMv8 guarantees NEON, 128-bit, 4 floats per op)
	if hasNEON {
		mixAccumulateFn = mixAccumulateNEON
		mixGainFn = mixGainNEON
		mixTanhFn = mixTanhNEON
		hasSIMDSupport = true
		simdLevel = "NEON"
		return
	}

	setPureGo()
}

func setPureGo() {
	mixAccumulateFn = mixAccumulatePure
	mixGainFn = mixGainPure
	mixTanhFn = mixTanhPure
	hasSIMDSupport = false
	simdLevel = "none"
}

// DisableSIMD forces pure Go implementations. Call before any audio processing.
// This is used by the --no-simd-optimization CLI flag.
func DisableSIMD() {
	simdMu.Lock()
	defer simdMu.Unlock()
	setPureGo()
}

// EnableSIMD re-enables hardware SIMD dispatch based on CPU capabilities.
func EnableSIMD() {
	selectSIMD()
}

// MixAccumulate adds src samples to dst samples (dst[i] += src[i]).
func MixAccumulate(dst, src []float32) {
	simdMu.RLock()
	fn := mixAccumulateFn
	simdMu.RUnlock()
	fn(dst, src)
}

// MixGain applies gain to all samples in dst (dst[i] *= gain).
func MixGain(dst []float32, gain float32) {
	simdMu.RLock()
	fn := mixGainFn
	simdMu.RUnlock()
	fn(dst, gain)
}

// MixTanh applies tanh soft clipping to prevent distortion.
// Uses Pade approximant with clamping to [-1, 1] fused into each implementation.
func MixTanh(dst []float32) {
	simdMu.RLock()
	fn := mixTanhFn
	simdMu.RUnlock()
	fn(dst)
}

// HasSIMDSupport returns true if SIMD optimizations are active.
func HasSIMDSupport() bool {
	simdMu.RLock()
	defer simdMu.RUnlock()
	return hasSIMDSupport
}

// SIMDLevel returns the active SIMD level ("AVX", "SSE", "NEON", or "none").
func SIMDLevel() string {
	simdMu.RLock()
	defer simdMu.RUnlock()
	return simdLevel
}

// CPUHasSIMD returns true if the CPU supports any SIMD instruction set,
// regardless of whether SIMD is currently enabled or disabled.
func CPUHasSIMD() bool {
	return cpu.X86.HasAVX || cpu.X86.HasSSE2 || hasNEON
}
