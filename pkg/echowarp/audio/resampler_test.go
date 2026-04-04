package audio

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResamplerProcessor_Name(t *testing.T) {
	r := NewResamplerProcessor(44100, 48000, 1, 1)
	assert.Equal(t, "resampler", r.Name())
}

func TestResamplerProcessor_NeedsResampling(t *testing.T) {
	assert.False(t, NewResamplerProcessor(48000, 48000, 2, 2).NeedsResampling())
	assert.True(t, NewResamplerProcessor(44100, 48000, 1, 1).NeedsResampling())
	assert.True(t, NewResamplerProcessor(48000, 48000, 1, 2).NeedsResampling())
}

func TestResamplerProcessor_NoOp(t *testing.T) {
	r := NewResamplerProcessor(48000, 48000, 2, 2)
	in := []float32{0.1, 0.2, 0.3, 0.4}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestResamplerProcessor_Upsample(t *testing.T) {
	r := NewResamplerProcessor(24000, 48000, 1, 1)
	// 4 input samples at 24k → ~8 output samples at 48k
	in := []float32{0.0, 1.0, 0.0, -1.0}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)

	// Output should be roughly double the input length.
	assert.InDelta(t, float64(len(in)*2), float64(len(out)), 2)

	// First sample should match input.
	assert.InDelta(t, 0.0, float64(out[0]), 0.01)
}

func TestResamplerProcessor_Downsample(t *testing.T) {
	r := NewResamplerProcessor(48000, 24000, 1, 1)
	// 8 input samples at 48k → ~4 output samples at 24k
	in := make([]float32, 8)
	for i := range in {
		in[i] = float32(math.Sin(2 * math.Pi * float64(i) / 8))
	}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)
	assert.InDelta(t, float64(len(in)/2), float64(len(out)), 2)
}

func TestResamplerProcessor_MonoToStereo(t *testing.T) {
	r := NewResamplerProcessor(48000, 48000, 1, 2)
	in := []float32{0.5, -0.5, 0.3}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)
	expected := []float32{0.5, 0.5, -0.5, -0.5, 0.3, 0.3}
	assert.Equal(t, expected, out)
}

func TestResamplerProcessor_StereoToMono(t *testing.T) {
	r := NewResamplerProcessor(48000, 48000, 2, 1)
	in := []float32{0.4, 0.6, -0.2, -0.8}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)
	assert.InDelta(t, 0.5, float64(out[0]), 0.001)
	assert.InDelta(t, -0.5, float64(out[1]), 0.001)
}

func TestResamplerProcessor_CombinedRateAndChannels(t *testing.T) {
	// 24k mono → 48k stereo
	r := NewResamplerProcessor(24000, 48000, 1, 2)
	in := []float32{0.0, 1.0, 0.0, -1.0}
	out, err := r.Process(context.Background(), in)
	require.NoError(t, err)

	// Rate doubles (~8 mono samples), then stereo doubles again (~16).
	assert.InDelta(t, float64(len(in)*4), float64(len(out)), 4)
	// Stereo: every pair should be equal.
	for i := 0; i+1 < len(out); i += 2 {
		assert.Equal(t, out[i], out[i+1], "stereo pair at %d should match", i)
	}
}

func TestResamplerProcessor_StatefulContinuity(t *testing.T) {
	r := NewResamplerProcessor(24000, 48000, 1, 1)

	// Process two consecutive chunks and verify continuity.
	chunk1 := make([]float32, 100)
	chunk2 := make([]float32, 100)
	for i := range chunk1 {
		chunk1[i] = float32(math.Sin(2 * math.Pi * float64(i) / 100))
	}
	for i := range chunk2 {
		chunk2[i] = float32(math.Sin(2 * math.Pi * float64(i+100) / 100))
	}

	out1, err := r.Process(context.Background(), chunk1)
	require.NoError(t, err)
	out2, err := r.Process(context.Background(), chunk2)
	require.NoError(t, err)

	// Total output should be roughly double total input.
	totalOut := len(out1) + len(out2)
	assert.InDelta(t, 400, float64(totalOut), 4)

	// Check continuity: last sample of out1 and first sample of out2 should
	// not have a large discontinuity (the sine wave is smooth).
	if len(out1) > 0 && len(out2) > 0 {
		diff := math.Abs(float64(out2[0] - out1[len(out1)-1]))
		assert.Less(t, diff, float64(0.2), "discontinuity between chunks should be small")
	}
}

func TestResamplerProcessor_ImplementsAudioProcessor(t *testing.T) {
	var _ AudioProcessor = (*ResamplerProcessor)(nil)
}

func TestResamplerProcessor_EmptyInput(t *testing.T) {
	r := NewResamplerProcessor(24000, 48000, 1, 1)
	out, err := r.Process(context.Background(), []float32{})
	require.NoError(t, err)
	assert.Empty(t, out)
}
