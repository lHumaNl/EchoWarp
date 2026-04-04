//go:build !arm64 || !gc || noasm

package audio

// Stub implementations for NEON functions on non-arm64 platforms or when assembly is disabled.
// These should never be called because CPU detection in mix.go will select appropriate fallbacks.

func mixAccumulateNEON(dst, src []float32) {
	mixAccumulatePure(dst, src)
}

func mixGainNEON(dst []float32, gain float32) {
	mixGainPure(dst, gain)
}

func mixTanhNEON(dst []float32) {
	mixTanhPure(dst)
}
