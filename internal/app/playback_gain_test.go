package app

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestApplyPlaybackGainMute(t *testing.T) {
	// No gain, no mute — frame untouched
	frame := []float32{0.5, -0.5, 0.25, -0.25}
	applyPlaybackGainMute(context.Background(), frame, nil, nil, nil)
	assert.Equal(t, []float32{0.5, -0.5, 0.25, -0.25}, frame)

	// Mute flag — zeroed
	var mute atomic.Bool
	mute.Store(true)
	frame = []float32{0.5, -0.5, 0.25, -0.25}
	applyPlaybackGainMute(context.Background(), frame, nil, &mute, nil)
	assert.Equal(t, []float32{0, 0, 0, 0}, frame)

	// Gain 0.5 — halved
	gain := NewDeviceGainControl(0.5)
	frame = []float32{1.0, -1.0, 0.5, -0.5}
	applyPlaybackGainMute(context.Background(), frame, gain, nil, nil)
	assert.InDelta(t, 0.5, frame[0], 0.001)
	assert.InDelta(t, -0.5, frame[1], 0.001)
	assert.InDelta(t, 0.25, frame[2], 0.001)
	assert.InDelta(t, -0.25, frame[3], 0.001)

	// Gain 1.5 — amplified + soft-clipped with tanh.
	// Small signal (0.4 × 1.5 = 0.6) -> tanh(0.6) ≈ 0.537 (gentle curve).
	// Loud signal (1.0 × 1.5 = 1.5) -> tanh(1.5) ≈ 0.905 (saturated, never >1).
	gain2 := NewDeviceGainControl(1.5)
	frame = []float32{0.4, -0.4, 1.0, -1.0}
	applyPlaybackGainMute(context.Background(), frame, gain2, nil, nil)
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
	applyPlaybackGainMute(context.Background(), frame, gain3, nil, nil)
	assert.Equal(t, []float32{0, 0}, frame)
}

func TestBuildAGCProcessors_AlwaysCreatesForRuntimeToggle(t *testing.T) {
	// All devices get processors so AGC toggle works at runtime,
	// with initial enabled state matching DeviceEntry.AGC.
	devices := []config.DeviceEntry{
		{ID: 1, Name: "Mic1", Role: config.RoleCapture, AGC: false},
		{ID: 2, Name: "Mic2", Role: config.RoleCapture, AGC: true},
	}
	m := buildAGCProcessors(devices, 48000)
	require.NotNil(t, m)
	require.Contains(t, m, uint32(1))
	require.Contains(t, m, uint32(2))
	assert.False(t, m[1].IsEnabled(), "device 1 should start disabled (AGC:false)")
	assert.True(t, m[2].IsEnabled(), "device 2 should start enabled (AGC:true)")

	// Runtime toggle enables device 1.
	m[1].SetEnabled(true)
	assert.True(t, m[1].IsEnabled())
}

func TestBuildAGCProcessors_EmptyReturnsNil(t *testing.T) {
	m := buildAGCProcessors(nil, 48000)
	assert.Nil(t, m)
	m = buildAGCProcessors([]config.DeviceEntry{}, 48000)
	assert.Nil(t, m)
}
