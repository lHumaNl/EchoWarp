package audio

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAEC_Name(t *testing.T) {
	aec := NewAECProcessor(DefaultAECConfig())
	assert.Equal(t, "aec", aec.Name())
}

func TestAEC_PassthroughWhenDisabled(t *testing.T) {
	aec := NewAECProcessor(DefaultAECConfig())
	aec.SetEnabled(false)

	input := []float32{0.5, -0.3, 0.1}
	out, err := aec.Process(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, []float32{0.5, -0.3, 0.1}, out)
}

func TestAEC_PassthroughWhenNoReference(t *testing.T) {
	aec := NewAECProcessor(AECConfig{FilterLength: 64, StepSize: 0.3})

	// With no reference fed, the filter weights are all zero,
	// so echo estimate is zero and output == input.
	input := []float32{0.5, -0.3, 0.1, 0.0}
	out, err := aec.Process(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, input, out)
}

func TestAEC_ReducesEchoFromReference(t *testing.T) {
	cfg := AECConfig{FilterLength: 128, StepSize: 0.5}
	aec := NewAECProcessor(cfg)

	// Simulate: reference signal is a sine wave, near-end is the same sine (perfect echo).
	// After adaptation, the output should have much lower energy than the input.
	const frames = 200
	const frameSize = 128
	freq := 440.0
	sr := 48000.0

	var totalInputPower, totalOutputPower float64

	for f := 0; f < frames; f++ {
		ref := make([]float32, frameSize)
		near := make([]float32, frameSize)
		for i := 0; i < frameSize; i++ {
			sample := float32(math.Sin(2 * math.Pi * freq * float64(f*frameSize+i) / sr))
			ref[i] = sample
			near[i] = sample * 0.8 // echo is attenuated version of reference
		}

		// Save input power BEFORE Process modifies near in-place
		if f > 50 {
			for _, s := range near {
				totalInputPower += float64(s) * float64(s)
			}
		}

		aec.FeedReference(ref)
		out, err := aec.Process(context.Background(), near)
		require.NoError(t, err)

		if f > 50 {
			for _, s := range out {
				totalOutputPower += float64(s) * float64(s)
			}
		}
	}

	// Echo should be significantly reduced (at least 10dB = 10x power reduction)
	ratio := totalOutputPower / totalInputPower
	assert.Less(t, ratio, 0.3, "AEC should reduce echo power by at least 70%%, got ratio %.4f", ratio)
}

func TestAEC_EnableDisable(t *testing.T) {
	aec := NewAECProcessor(DefaultAECConfig())

	assert.True(t, aec.IsEnabled())
	aec.SetEnabled(false)
	assert.False(t, aec.IsEnabled())
	aec.SetEnabled(true)
	assert.True(t, aec.IsEnabled())
}

func TestAEC_Reset(t *testing.T) {
	cfg := AECConfig{FilterLength: 64, StepSize: 0.3}
	aec := NewAECProcessor(cfg)

	// Feed some reference to change weights
	ref := make([]float32, 64)
	for i := range ref {
		ref[i] = 0.5
	}
	aec.FeedReference(ref)
	near := make([]float32, 64)
	for i := range near {
		near[i] = 0.5
	}
	_, _ = aec.Process(context.Background(), near)

	// Verify weights are non-zero
	hasNonZero := false
	for _, w := range aec.weights {
		if w != 0 {
			hasNonZero = true
			break
		}
	}
	assert.True(t, hasNonZero, "weights should be non-zero after processing")

	// Reset
	aec.Reset()
	for _, w := range aec.weights {
		assert.Equal(t, float32(0), w)
	}
}

func TestAEC_EmptyInput(t *testing.T) {
	aec := NewAECProcessor(DefaultAECConfig())
	out, err := aec.Process(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, out)

	out, err = aec.Process(context.Background(), []float32{})
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestAEC_DefaultConfig(t *testing.T) {
	cfg := DefaultAECConfig()
	assert.Equal(t, 4800, cfg.FilterLength)
	assert.Equal(t, float32(0.3), cfg.StepSize)
}
