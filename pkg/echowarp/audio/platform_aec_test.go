package audio

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPlatformAEC(t *testing.T) {
	aec := GetPlatformAEC()

	switch runtime.GOOS {
	case "darwin":
		assert.NotNil(t, aec)
		assert.Equal(t, "CoreAudio", aec.Name())
		assert.False(t, aec.Available(), "stub should report not available")
		// Start should fail since it's a stub
		err := aec.Start(48000, 1)
		assert.Error(t, err)
		// Process passthrough when not started
		samples := []float32{0.5, -0.5}
		out, err := aec.Process(context.Background(), samples)
		assert.NoError(t, err)
		assert.Equal(t, samples, out)
		assert.NoError(t, aec.Stop())

	case "linux":
		assert.NotNil(t, aec)
		assert.Equal(t, "PulseAudio", aec.Name())
		assert.False(t, aec.Available(), "stub should report not available")
		err := aec.Start(48000, 1)
		assert.Error(t, err)
		samples := []float32{0.5, -0.5}
		out, err := aec.Process(context.Background(), samples)
		assert.NoError(t, err)
		assert.Equal(t, samples, out)
		assert.NoError(t, aec.Stop())

	default:
		assert.Nil(t, aec, "no platform AEC on %s", runtime.GOOS)
	}
}
