package audio

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAGCProcessor_Name(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())
	assert.Equal(t, "agc", agc.Name())
}

func TestAGCProcessor_PassthroughWhenDisabled(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())
	agc.SetEnabled(false)

	input := []float32{0.5, -0.3, 0.1, -0.8}
	orig := make([]float32, len(input))
	copy(orig, input)

	result, err := agc.Process(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, orig, result)
}

func TestAGCProcessor_EmptyInput(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())

	result, err := agc.Process(context.Background(), []float32{})
	require.NoError(t, err)
	assert.Empty(t, result)

	result, err = agc.Process(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestAGCProcessor_AmplifiesQuietSignal(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())

	const frameSize = 960
	frame := make([]float32, frameSize)
	for i := range frame {
		// Quiet sine wave at 0.01 amplitude (~-40 dBFS)
		frame[i] = 0.01 * float32(math.Sin(2*math.Pi*440*float64(i)/48000))
	}

	inputRMS := rmsFloat32(frame)

	// Run multiple frames to let AGC converge
	var result []float32
	for i := 0; i < 100; i++ {
		buf := make([]float32, frameSize)
		copy(buf, frame)
		var err error
		result, err = agc.Process(context.Background(), buf)
		require.NoError(t, err)
	}

	outputRMS := rmsFloat32(result)
	assert.Greater(t, outputRMS, inputRMS*2, "AGC should amplify quiet signal significantly")
}

func TestAGCProcessor_AttenuatesLoudSignal(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())

	const frameSize = 960
	frame := make([]float32, frameSize)
	for i := range frame {
		// Loud sine wave at 0.9 amplitude (~-0.9 dBFS)
		frame[i] = 0.9 * float32(math.Sin(2*math.Pi*440*float64(i)/48000))
	}

	inputRMS := rmsFloat32(frame)

	var result []float32
	for i := 0; i < 100; i++ {
		buf := make([]float32, frameSize)
		copy(buf, frame)
		var err error
		result, err = agc.Process(context.Background(), buf)
		require.NoError(t, err)
	}

	outputRMS := rmsFloat32(result)
	assert.Less(t, outputRMS, inputRMS, "AGC should attenuate loud signal")
}

func TestAGCProcessor_EnableDisable(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())
	assert.True(t, agc.IsEnabled())

	agc.SetEnabled(false)
	assert.False(t, agc.IsEnabled())

	agc.SetEnabled(true)
	assert.True(t, agc.IsEnabled())
}

func TestAGCProcessor_CurrentGainDB(t *testing.T) {
	agc := NewAGCProcessor(DefaultAGCConfig())
	// Initial gain is 1.0 → 0 dB
	gainDB := agc.CurrentGainDB()
	assert.InDelta(t, 0.0, gainDB, 0.01, "initial gain should be 0 dB")
}

func TestAGCProcessor_DefaultConfig(t *testing.T) {
	cfg := DefaultAGCConfig()
	assert.Equal(t, -18.0, cfg.TargetDBFS)
	assert.Equal(t, 20.0, cfg.MaxGain)
	assert.Equal(t, 5.0, cfg.AttackMs)
	assert.Equal(t, 50.0, cfg.ReleaseMs)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
}

func TestAGCProcessor_ClippingProtection(t *testing.T) {
	// Use high max gain and quiet signal that will be amplified past 1.0
	cfg := DefaultAGCConfig()
	cfg.MaxGain = 50.0
	agc := NewAGCProcessor(cfg)

	const frameSize = 960
	frame := make([]float32, frameSize)
	for i := range frame {
		frame[i] = 0.1 * float32(math.Sin(2*math.Pi*440*float64(i)/48000))
	}

	// Let AGC ramp up gain
	for i := 0; i < 200; i++ {
		buf := make([]float32, frameSize)
		copy(buf, frame)
		_, err := agc.Process(context.Background(), buf)
		require.NoError(t, err)
	}

	// Now feed a loud burst — gain is high, output should clip to [-1, 1]
	loud := make([]float32, frameSize)
	for i := range loud {
		loud[i] = 0.8 * float32(math.Sin(2*math.Pi*440*float64(i)/48000))
	}
	result, err := agc.Process(context.Background(), loud)
	require.NoError(t, err)

	for i, s := range result {
		assert.LessOrEqual(t, s, float32(1.0), "sample %d exceeds +1.0", i)
		assert.GreaterOrEqual(t, s, float32(-1.0), "sample %d below -1.0", i)
	}
}

func rmsFloat32(samples []float32) float64 {
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}
