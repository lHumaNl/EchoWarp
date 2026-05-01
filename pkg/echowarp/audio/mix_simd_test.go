package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSIMDCorrectness_Accumulate(t *testing.T) {
	t.Logf("SIMD level: %s (hasSIMD=%v)", SIMDLevel(), HasSIMDSupport())

	size := 1923 // non-aligned to test tail handling
	src := make([]float32, size)
	dstPure := make([]float32, size)
	dstSIMD := make([]float32, size)

	for i := range src {
		src[i] = float32(i%100) * 0.01
		dstPure[i] = float32((i+50)%100) * 0.01
		dstSIMD[i] = dstPure[i]
	}

	mixAccumulatePure(dstPure, src)
	mixAccumulateFn(dstSIMD, src)

	for i := range dstPure {
		assert.InDelta(t, dstPure[i], dstSIMD[i], 0.0001, "accumulate mismatch at %d", i)
	}
}

func TestSIMDCorrectness_Gain(t *testing.T) {
	size := 1923
	dataPure := make([]float32, size)
	dataSIMD := make([]float32, size)
	gain := float32(0.5774)

	for i := range dataPure {
		dataPure[i] = float32(i%100) * 0.01
		dataSIMD[i] = dataPure[i]
	}

	mixGainPure(dataPure, gain)
	mixGainFn(dataSIMD, gain)

	for i := range dataPure {
		assert.InDelta(t, dataPure[i], dataSIMD[i], 0.0001, "gain mismatch at %d", i)
	}
}

func TestSIMDCorrectness_Tanh(t *testing.T) {
	// Use values in [-3, 3] range where Pade is accurate
	size := 61
	dataPure := make([]float32, size)
	dataSIMD := make([]float32, size)

	for i := range dataPure {
		dataPure[i] = float32(i-30) * 0.1 // range [-3.0, 3.0]
		dataSIMD[i] = dataPure[i]
	}

	mixTanhPure(dataPure)
	mixTanhFn(dataSIMD)

	for i := range dataPure {
		assert.InDelta(t, dataPure[i], dataSIMD[i], 0.001, "tanh mismatch at %d", i)
	}
}

func TestMixTanh_ClampsOutput(t *testing.T) {
	dst := []float32{5.0, -5.0, 0.5, -0.5}
	MixTanh(dst)
	assert.LessOrEqual(t, dst[0], float32(1.0))
	assert.GreaterOrEqual(t, dst[1], float32(-1.0))
	assert.InDelta(t, 0.4621, dst[2], 0.01)
	assert.InDelta(t, -0.4621, dst[3], 0.01)
}

func TestTanhPade_AccuracyVsMathTanh(t *testing.T) {
	// Pade approximant is accurate for |x| < 3, diverges slightly beyond.
	// After clamping, output is always in [-1, 1].
	for _, x := range []float32{-3, -1, -0.5, 0, 0.5, 1, 3} {
		expected := float32(math.Tanh(float64(x)))
		x2 := x * x
		pade := x * (27.0 + x2) / (27.0 + 9.0*x2)
		assert.InDelta(t, expected, pade, 0.02, "Pade inaccurate at x=%f", x)
	}

	// For |x| > 3.5, Pade overshoots but MixTanh clamps to [-1, 1]
	dst := []float32{5.0, -5.0}
	MixTanh(dst)
	assert.LessOrEqual(t, dst[0], float32(1.0))
	assert.GreaterOrEqual(t, dst[1], float32(-1.0))
}

func TestDisableSIMD(t *testing.T) {
	origLevel := SIMDLevel()
	origHas := HasSIMDSupport()

	DisableSIMD()
	assert.False(t, HasSIMDSupport())
	assert.Equal(t, "none", SIMDLevel())

	// Verify pure Go still works
	dst := []float32{1, 2, 3, 4}
	src := []float32{10, 20, 30, 40}
	MixAccumulate(dst, src)
	assert.Equal(t, []float32{11, 22, 33, 44}, dst)

	// Restore
	selectSIMD()
	assert.Equal(t, origLevel, SIMDLevel())
	assert.Equal(t, origHas, HasSIMDSupport())
}

func TestSIMDStateConcurrentAccess(t *testing.T) {
	done := make(chan struct{})
	stopped := make(chan struct{})
	defer EnableSIMD()
	go func() {
		defer close(stopped)
		toggleSIMDUntilDone(done)
	}()

	for i := 0; i < 1000; i++ {
		_ = SIMDLevel()
		_ = HasSIMDSupport()
		dst := []float32{1, 2, 3, 4}
		MixGain(dst, 0.5)
		MixTanh(dst)
	}
	close(done)
	<-stopped
}

func toggleSIMDUntilDone(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		default:
			DisableSIMD()
			EnableSIMD()
		}
	}
}

func BenchmarkMixAccumulate_Pure(b *testing.B) {
	dst := make([]float32, 1920)
	src := make([]float32, 1920)
	for i := range src {
		src[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mixAccumulatePure(dst, src)
	}
}

func BenchmarkMixAccumulate_SIMD(b *testing.B) {
	dst := make([]float32, 1920)
	src := make([]float32, 1920)
	for i := range src {
		src[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mixAccumulateFn(dst, src)
	}
}

func BenchmarkMixGain_Pure(b *testing.B) {
	dst := make([]float32, 1920)
	for i := range dst {
		dst[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mixGainPure(dst, 0.5774)
	}
}

func BenchmarkMixGain_SIMD(b *testing.B) {
	dst := make([]float32, 1920)
	for i := range dst {
		dst[i] = float32(i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mixGainFn(dst, 0.5774)
	}
}

func BenchmarkMixTanh_Pure(b *testing.B) {
	dst := make([]float32, 1920)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range dst {
			dst[j] = float32(j%100-50) * 0.1
		}
		mixTanhPure(dst)
	}
}

func BenchmarkMixTanh_SIMD(b *testing.B) {
	dst := make([]float32, 1920)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range dst {
			dst[j] = float32(j%100-50) * 0.1
		}
		mixTanhFn(dst)
	}
}
