package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// Compile-time check: both types implement VolumeController.
var _ VolumeController = (*DeviceGainControl)(nil)
var _ VolumeController = (*audio.AudioMixer)(nil)

func TestVolumeController_DeviceGainControl(t *testing.T) {
	var ctrl VolumeController = NewDeviceGainControl(1.0)

	// GetSourceVolume ignores sourceID.
	assert.InDelta(t, 1.0, float64(ctrl.GetSourceVolume("any")), 0.001)

	// SetSourceVolume.
	ctrl.SetSourceVolume("ignored", 0.5)
	assert.InDelta(t, 0.5, float64(ctrl.GetSourceVolume("x")), 0.001)

	// Mute.
	assert.False(t, ctrl.IsSourceMuted("x"))
	ctrl.SetSourceMuted("x", true)
	assert.True(t, ctrl.IsSourceMuted("x"))
	ctrl.SetSourceMuted("x", false)
	assert.False(t, ctrl.IsSourceMuted("x"))
}

func TestVolumeController_GlobalMute(t *testing.T) {
	gc := NewDeviceGainControl(1.0)
	var ctrl VolumeController = gc

	// Global mute starts off.
	assert.False(t, ctrl.IsGlobalMuted())
	assert.False(t, gc.IsMuted())

	// Enable global mute.
	ctrl.SetGlobalMute(true)
	assert.True(t, ctrl.IsGlobalMuted())
	// IsMuted reflects global mute.
	assert.True(t, gc.IsMuted())

	// Per-source mute is still off.
	assert.False(t, ctrl.IsSourceMuted("x"))

	// Disable global mute.
	ctrl.SetGlobalMute(false)
	assert.False(t, ctrl.IsGlobalMuted())
	assert.False(t, gc.IsMuted())

	// Both per-source and global mute.
	ctrl.SetSourceMuted("x", true)
	ctrl.SetGlobalMute(true)
	assert.True(t, gc.IsMuted())

	// Remove per-source mute — still muted via global.
	ctrl.SetSourceMuted("x", false)
	assert.True(t, gc.IsMuted())

	// Remove global — now unmuted.
	ctrl.SetGlobalMute(false)
	assert.False(t, gc.IsMuted())
}
