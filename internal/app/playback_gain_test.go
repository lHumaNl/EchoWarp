package app

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyPlaybackGainMute(t *testing.T) {
	// No gain, no mute — frame untouched
	frame := []float32{0.5, -0.5, 0.25, -0.25}
	applyPlaybackGainMute(frame, nil, nil)
	assert.Equal(t, []float32{0.5, -0.5, 0.25, -0.25}, frame)

	// Mute flag — zeroed
	var mute atomic.Bool
	mute.Store(true)
	frame = []float32{0.5, -0.5, 0.25, -0.25}
	applyPlaybackGainMute(frame, nil, &mute)
	assert.Equal(t, []float32{0, 0, 0, 0}, frame)

	// Gain 0.5 — halved
	gain := NewDeviceGainControl(0.5)
	frame = []float32{1.0, -1.0, 0.5, -0.5}
	applyPlaybackGainMute(frame, gain, nil)
	assert.InDelta(t, 0.5, frame[0], 0.001)
	assert.InDelta(t, -0.5, frame[1], 0.001)
	assert.InDelta(t, 0.25, frame[2], 0.001)
	assert.InDelta(t, -0.25, frame[3], 0.001)

	// Gain 1.5 — amplified + soft-clipped with tanh.
	// Small signal (0.4 × 1.5 = 0.6) -> tanh(0.6) ≈ 0.537 (gentle curve).
	// Loud signal (1.0 × 1.5 = 1.5) -> tanh(1.5) ≈ 0.905 (saturated, never >1).
	gain2 := NewDeviceGainControl(1.5)
	frame = []float32{0.4, -0.4, 1.0, -1.0}
	applyPlaybackGainMute(frame, gain2, nil)
	// Verify soft-clip: all samples must be within [-1, +1] (no hard clip).
	for i, s := range frame {
		assert.Less(t, float64(s), 1.0, "sample %d should be < 1.0", i)
		assert.Greater(t, float64(s), -1.0, "sample %d should be > -1.0", i)
	}
	// Verify amplification: 0.4 × 1.5 = 0.6, tanh(0.6) ≈ 0.537 — still louder than 0.4.
	assert.Greater(t, float64(frame[0]), 0.4, "amplified sample should be louder than input")
	assert.Less(t, float64(frame[1]), -0.4, "amplified negative sample should be louder")

	// Gain mute takes precedence over gain value
	gain3 := NewDeviceGainControl(0.8)
	gain3.SetMuted(true)
	frame = []float32{0.5, -0.5}
	applyPlaybackGainMute(frame, gain3, nil)
	assert.Equal(t, []float32{0, 0}, frame)
}
