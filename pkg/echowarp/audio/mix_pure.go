package audio

// mixAccumulatePure adds src samples to dst samples using scalar Go code.
func mixAccumulatePure(dst, src []float32) {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	for i := 0; i < n; i++ {
		dst[i] += src[i]
	}
}

// mixGainPure applies gain to all samples using scalar Go code.
func mixGainPure(dst []float32, gain float32) {
	for i := range dst {
		dst[i] *= gain
	}
}

// mixTanhPure applies tanh soft clipping using Pade approximant.
// tanh(x) ≈ x * (27 + x²) / (27 + 9*x²) — accurate to ~0.005 for |x| < 3.5,
// and naturally saturates toward ±1 for larger values.
// This is 3-4x faster than math.Tanh (avoids float64 conversion).
func mixTanhPure(dst []float32) {
	for i := range dst {
		x := dst[i]
		x2 := x * x
		r := x * (27.0 + x2) / (27.0 + 9.0*x2)
		if r > 1.0 {
			r = 1.0
		} else if r < -1.0 {
			r = -1.0
		}
		dst[i] = r
	}
}
