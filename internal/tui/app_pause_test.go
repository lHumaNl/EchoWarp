package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// makePauseModel creates a streaming-screen Model with the given device count and a buffered pauseCh.
func makePauseModel(t *testing.T, deviceCount int) (Model, chan bool) {
	t.Helper()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}

	var devices []audio.AudioDevice
	for i := 0; i < deviceCount; i++ {
		devices = append(devices, audio.AudioDevice{
			ID: uint32(i + 1), Name: "mic", Channels: 1, SampleRate: 48000,
		})
	}

	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming

	pauseCh := make(chan bool, 4)
	m.pauseCh = pauseCh

	// Build device states (capture role) and initialize pauseState via WithDeviceStates.
	var states []DeviceState
	for _, d := range devices {
		states = append(states, DeviceState{ID: d.ID, Name: d.Name, Role: "capture"})
	}
	m = m.WithDeviceStates(states)

	return m, pauseCh
}

// pressCtrlP sends a Ctrl+P key message to the model.
func pressCtrlP(m Model) Model {
	result, _ := m.updateStreaming(tea.KeyMsg{Type: tea.KeyCtrlP}, nil)
	return result.(Model)
}

// drainBool returns the first value on ch or (false, false) if empty.
func drainBool(ch chan bool) (val bool, ok bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		return false, false
	}
}

// TestPauseCapture_SingleDevice_AllPausedOnFirstPress verifies that pressing Ctrl+P
// on a single-device setup sends ActionPause (true) when the device becomes paused.
func TestPauseCapture_SingleDevice_AllPausedOnFirstPress(t *testing.T) {
	t.Parallel()
	m, ch := makePauseModel(t, 1)

	// First press: single device → all paused → notify
	m = pressCtrlP(m)
	assert.True(t, m.paused)
	assert.True(t, m.allPausedNotified)

	val, ok := drainBool(ch)
	assert.True(t, ok, "expected notification on pauseCh")
	assert.True(t, val, "expected ActionPause (true)")

	// Second press: single device resumes → notify
	m = pressCtrlP(m)
	assert.False(t, m.paused)
	assert.False(t, m.allPausedNotified)

	val, ok = drainBool(ch)
	assert.True(t, ok, "expected resume notification on pauseCh")
	assert.False(t, val, "expected ActionResume (false)")
}

// TestPauseCapture_MultiDevice_NoNotifyOnPartialPause verifies that when multiple
// devices exist and they transition from none-paused to all-paused, the notification
// fires only once (not on individual device toggles, if they don't result in all-paused).
func TestPauseCapture_MultiDevice_AllPausedSendsNotify(t *testing.T) {
	t.Parallel()
	m, ch := makePauseModel(t, 2)

	// Ctrl+P toggles all devices at once (ToggleAll). With 2 devices,
	// first press pauses both → all-paused → notify.
	m = pressCtrlP(m)
	assert.True(t, m.paused)
	assert.True(t, m.allPausedNotified)

	val, ok := drainBool(ch)
	assert.True(t, ok, "expected ActionPause notification")
	assert.True(t, val)

	// Second press resumes both → notify ActionResume
	m = pressCtrlP(m)
	assert.False(t, m.paused)
	assert.False(t, m.allPausedNotified)

	val, ok = drainBool(ch)
	assert.True(t, ok, "expected ActionResume notification")
	assert.False(t, val)
}

// TestPauseCapture_NoDuplicateNotify verifies that pressing Ctrl+P repeatedly
// while already all-paused does not send duplicate ActionPause notifications.
func TestPauseCapture_NoDuplicateNotify(t *testing.T) {
	t.Parallel()
	m, ch := makePauseModel(t, 1)

	// Pause
	m = pressCtrlP(m)
	_, _ = drainBool(ch) // consume ActionPause

	// Pause again (should resume, not double-notify pause)
	m = pressCtrlP(m)
	val, ok := drainBool(ch)
	assert.True(t, ok)
	assert.False(t, val, "expected ActionResume on second press")

	// Third press: pause again
	m = pressCtrlP(m)
	val, ok = drainBool(ch)
	assert.True(t, ok)
	assert.True(t, val, "expected ActionPause on third press")

	// No extra notifications
	_, extra := drainBool(ch)
	assert.False(t, extra, "unexpected extra notification")
}

// TestPauseCapture_NoPauseStateNoPauseCh verifies that Ctrl+P is a no-op when pauseCh is nil.
func TestPauseCapture_NoPauseStateNoPauseCh(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}
	devices := []audio.AudioDevice{{ID: 1, Name: "mic", Channels: 1, SampleRate: 48000}}
	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	// pauseCh is nil → Ctrl+P should be a no-op
	m = pressCtrlP(m)
	assert.False(t, m.paused)
}

// TestPauseCapture_WithDeviceStates_InitialisesPauseState verifies that
// WithDeviceStates correctly initializes pauseState for capture devices only.
func TestPauseCapture_WithDeviceStates_InitialisesPauseState(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}
	devices := []audio.AudioDevice{{ID: 1, Name: "mic", Channels: 1, SampleRate: 48000}}
	m := NewModel(cfg, devices)

	// No capture devices → pauseState nil
	m = m.WithDeviceStates([]DeviceState{
		{ID: 1, Name: "speaker", Role: "playback"},
	})
	assert.Nil(t, m.pauseState, "no capture devices → pauseState should be nil")

	// With capture device
	m = m.WithDeviceStates([]DeviceState{
		{ID: 1, Name: "mic", Role: "capture"},
		{ID: 2, Name: "speaker", Role: "playback"},
	})
	assert.NotNil(t, m.pauseState)
	assert.Equal(t, 1, m.pauseState.Count())
	assert.False(t, m.pauseState.IsAllPaused())
}
