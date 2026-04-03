package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPauseState(t *testing.T) {
	ps := NewPauseState([]string{"mic-1", "mic-2"})
	require.NotNil(t, ps)
	assert.Equal(t, 2, ps.Count())
	assert.False(t, ps.IsPaused("mic-1"))
	assert.False(t, ps.IsPaused("mic-2"))
	assert.False(t, ps.IsAllPaused())
	assert.Empty(t, ps.PausedDevices())
}

func TestPauseState_Toggle(t *testing.T) {
	ps := NewPauseState([]string{"mic-1", "mic-2"})

	paused := ps.Toggle("mic-1")
	assert.True(t, paused)
	assert.True(t, ps.IsPaused("mic-1"))
	assert.False(t, ps.IsPaused("mic-2"))
	assert.False(t, ps.IsAllPaused())
	assert.Equal(t, []string{"mic-1"}, ps.PausedDevices())

	paused = ps.Toggle("mic-1")
	assert.False(t, paused)
	assert.False(t, ps.IsPaused("mic-1"))
}

func TestPauseState_ToggleAll(t *testing.T) {
	ps := NewPauseState([]string{"mic-1", "mic-2"})

	allPaused := ps.ToggleAll()
	assert.True(t, allPaused)
	assert.True(t, ps.IsAllPaused())
	assert.True(t, ps.IsPaused("mic-1"))
	assert.True(t, ps.IsPaused("mic-2"))
	assert.Equal(t, []string{"mic-1", "mic-2"}, ps.PausedDevices())

	allPaused = ps.ToggleAll()
	assert.False(t, allPaused)
	assert.False(t, ps.IsAllPaused())
	assert.Empty(t, ps.PausedDevices())
}

func TestPauseState_ToggleAll_PartialPause(t *testing.T) {
	ps := NewPauseState([]string{"mic-1", "mic-2"})
	ps.Toggle("mic-1") // only mic-1 paused

	// ToggleAll when some are unpaused → pause all
	allPaused := ps.ToggleAll()
	assert.True(t, allPaused)
	assert.True(t, ps.IsAllPaused())
}

func TestPauseState_Empty(t *testing.T) {
	ps := NewPauseState(nil)
	assert.Equal(t, 0, ps.Count())
	assert.False(t, ps.IsAllPaused())
	assert.Empty(t, ps.PausedDevices())
}

func TestPauseState_SetPaused(t *testing.T) {
	ps := NewPauseState([]string{"mic-1"})
	ps.SetPaused("mic-1", true)
	assert.True(t, ps.IsPaused("mic-1"))
	ps.SetPaused("mic-1", false)
	assert.False(t, ps.IsPaused("mic-1"))
}

func TestPauseState_Devices(t *testing.T) {
	ps := NewPauseState([]string{"mic-1", "mic-2", "mic-3"})
	devices := ps.Devices()
	assert.Equal(t, []string{"mic-1", "mic-2", "mic-3"}, devices)

	// Verify returned slice is a copy
	devices[0] = "modified"
	assert.Equal(t, "mic-1", ps.Devices()[0])
}

func TestPauseState_SingleDevice(t *testing.T) {
	ps := NewPauseState([]string{"mic-1"})

	ps.Toggle("mic-1")
	assert.True(t, ps.IsAllPaused())

	ps.Toggle("mic-1")
	assert.False(t, ps.IsAllPaused())
}
