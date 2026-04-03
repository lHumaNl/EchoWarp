package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLevelMeter(t *testing.T) {
	lm := NewLevelMeter(2)
	require.NotNil(t, lm)
	assert.Equal(t, 2, lm.Channels())
	assert.Len(t, lm.Levels(), 2)
}

func TestNewLevelMeter_MinChannels(t *testing.T) {
	lm := NewLevelMeter(0)
	assert.Equal(t, 1, lm.Channels())
}

func TestLevelMeter_Feed_Empty(t *testing.T) {
	lm := NewLevelMeter(2)
	lm.Feed(nil)
	lm.Feed([]float32{})
	for _, l := range lm.Levels() {
		assert.Equal(t, 0.0, l)
	}
}

func TestLevelMeter_Feed_Silence(t *testing.T) {
	lm := NewLevelMeter(2)
	silence := make([]float32, 960)
	lm.Feed(silence)
	for _, l := range lm.Levels() {
		assert.Equal(t, 0.0, l)
	}
}

func TestLevelMeter_Feed_Mono(t *testing.T) {
	lm := NewLevelMeter(1)
	// Full-scale signal
	samples := make([]float32, 100)
	for i := range samples {
		samples[i] = 1.0
	}
	lm.Feed(samples)
	levels := lm.Levels()
	assert.Equal(t, 1, len(levels))
	assert.Greater(t, levels[0], 0.5, "full-scale mono should have significant level")
}

func TestLevelMeter_Feed_Stereo(t *testing.T) {
	lm := NewLevelMeter(2)
	// Interleaved: L=1.0, R=0.0
	samples := make([]float32, 200)
	for i := 0; i < len(samples); i += 2 {
		samples[i] = 1.0   // L
		samples[i+1] = 0.0 // R
	}
	lm.Feed(samples)
	levels := lm.Levels()
	assert.Greater(t, levels[0], 0.5, "left channel should be loud")
	assert.Equal(t, 0.0, levels[1], "right channel should be silent")
}

func TestLevelMeter_Smoothing(t *testing.T) {
	lm := NewLevelMeter(1)
	// Feed loud signal
	loud := make([]float32, 100)
	for i := range loud {
		loud[i] = 1.0
	}
	lm.Feed(loud)
	level1 := lm.Levels()[0]

	// Feed silence — smoothing should retain some level
	silence := make([]float32, 100)
	lm.Feed(silence)
	level2 := lm.Levels()[0]

	assert.Greater(t, level1, 0.5)
	assert.Greater(t, level2, 0.0, "smoothing should retain some level after silence")
	assert.Less(t, level2, level1, "level should decrease after silence")
}

func TestLevelMeter_ThreadSafety(t *testing.T) {
	lm := NewLevelMeter(2)
	done := make(chan struct{})

	go func() {
		defer close(done)
		samples := make([]float32, 960)
		for i := 0; i < 100; i++ {
			lm.Feed(samples)
		}
	}()

	for i := 0; i < 100; i++ {
		_ = lm.Levels()
	}
	<-done
}
