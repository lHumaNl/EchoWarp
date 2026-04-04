package audio

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSIMDLevel_Platform verifies that the correct SIMD path is selected
// based on the current CPU architecture.
func TestSIMDLevel_Platform(t *testing.T) {
	// Ensure we're testing the auto-selected level, not a manually disabled one.
	selectSIMD()

	level := SIMDLevel()
	hasSIMD := HasSIMDSupport()
	t.Logf("GOARCH=%s, SIMDLevel=%s, HasSIMDSupport=%v", runtime.GOARCH, level, hasSIMD)

	switch runtime.GOARCH {
	case "arm64":
		assert.Equal(t, "NEON", level, "arm64 must use NEON (guaranteed by ARMv8 spec)")
		assert.True(t, hasSIMD)

	case "amd64":
		// All amd64 CPUs have at least SSE2; most modern ones have AVX.
		require.Contains(t, []string{"AVX", "SSE"}, level,
			"amd64 must use AVX or SSE")
		assert.True(t, hasSIMD)

	default:
		// Other architectures (e.g. riscv64, 386) fall back to pure Go.
		assert.Equal(t, "none", level, "unsupported arch should fall back to pure Go")
		assert.False(t, hasSIMD)
	}
}

// TestSIMDLevel_FunctionalAfterSelect verifies that SIMD functions are callable
// and produce correct results with the platform-selected implementation.
func TestSIMDLevel_FunctionalAfterSelect(t *testing.T) {
	selectSIMD()

	dst := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0}
	src := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}

	MixAccumulate(dst, src)

	expected := []float32{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}
	for i := range expected {
		assert.InDelta(t, expected[i], dst[i], 0.0001,
			"MixAccumulate via %s mismatch at [%d]", SIMDLevel(), i)
	}
}
