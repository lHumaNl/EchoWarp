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

	// Gain 1.5 — amplified
	gain2 := NewDeviceGainControl(1.5)
	frame = []float32{0.4, -0.4}
	applyPlaybackGainMute(frame, gain2, nil)
	assert.InDelta(t, 0.6, frame[0], 0.001)
	assert.InDelta(t, -0.6, frame[1], 0.001)

	// Gain mute takes precedence over gain value
	gain3 := NewDeviceGainControl(0.8)
	gain3.SetMuted(true)
	frame = []float32{0.5, -0.5}
	applyPlaybackGainMute(frame, gain3, nil)
	assert.Equal(t, []float32{0, 0}, frame)
}
