package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpectrumAnalyzer(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	require.NotNil(t, sa)
	assert.Equal(t, 16, sa.NumBands())
	assert.Len(t, sa.Bands(), 16)
}

func TestNewSpectrumAnalyzer_DefaultBands(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 0)
	assert.Equal(t, DefaultBands, sa.NumBands())
}

func TestSpectrumAnalyzer_Feed_Empty(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	sa.Feed(nil)
	sa.Feed([]float32{})
	// Should not panic, bands stay zero
	for _, b := range sa.Bands() {
		assert.Equal(t, 0.0, b)
	}
}

func TestSpectrumAnalyzer_Feed_Silence(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	silence := make([]float32, 960)
	sa.Feed(silence)
	for _, b := range sa.Bands() {
		assert.Equal(t, 0.0, b)
	}
}

func TestSpectrumAnalyzer_Feed_Sine(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	// Generate 1kHz sine at full scale
	samples := make([]float32, 2048)
	for i := range samples {
		samples[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 48000))
	}
	sa.Feed(samples)
	bands := sa.Bands()

	// At least one band should have nonzero energy
	maxVal := 0.0
	for _, b := range bands {
		if b > maxVal {
			maxVal = b
		}
	}
	assert.Greater(t, maxVal, 0.1, "sine wave should produce visible spectrum energy")
}

func TestSpectrumAnalyzer_Feed_ShortBuffer(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	// Buffer shorter than fftSize — should not panic, zero-pads
	short := make([]float32, 100)
	for i := range short {
		short[i] = 0.5
	}
	sa.Feed(short)
	// Should produce some energy
	bands := sa.Bands()
	assert.Len(t, bands, 16)
}

func TestSpectrumAnalyzer_Smoothing(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	// Feed a loud sine
	samples := make([]float32, 2048)
	for i := range samples {
		samples[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 48000))
	}
	sa.Feed(samples)
	bands1 := sa.Bands()

	// Feed silence — smoothing should keep some energy
	silence := make([]float32, 2048)
	sa.Feed(silence)
	bands2 := sa.Bands()

	// The band that had energy should still have some (smoothing decay, not instant drop)
	maxBand := 0
	for i, b := range bands1 {
		if b > bands1[maxBand] {
			maxBand = i
		}
	}
	if bands1[maxBand] > 0.1 {
		assert.Greater(t, bands2[maxBand], 0.0, "smoothing should retain some energy after silence")
	}
}

func TestSpectrumAnalyzer_ThreadSafety(t *testing.T) {
	sa := NewSpectrumAnalyzer(48000, 16)
	done := make(chan struct{})

	go func() {
		defer close(done)
		samples := make([]float32, 960)
		for i := 0; i < 100; i++ {
			sa.Feed(samples)
		}
	}()

	for i := 0; i < 100; i++ {
		_ = sa.Bands()
	}
	<-done
}

func TestBuildLogBands(t *testing.T) {
	ranges := buildLogBands(16, 20, 24000)
	assert.Len(t, ranges, 16)
	// First band starts at ~20Hz
	assert.InDelta(t, 20.0, ranges[0].lo, 1.0)
	// Last band ends at ~24000Hz
	assert.InDelta(t, 24000.0, ranges[15].hi, 100.0)
	// Bands should be contiguous
	for i := 1; i < len(ranges); i++ {
		assert.InDelta(t, ranges[i-1].hi, ranges[i].lo, 0.01)
	}
}
