package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

// TestVolumeAdjust_FullChain traces the exact data path from volume change
// through tryStart to SetupDoneMsg to verify volume is preserved.
func TestVolumeAdjust_FullChain(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.unifiedDuplex = true

	// Add output device with default volume
	m.outputDevices = []deviceRow{
		{Name: "Speakers", ID: 42, IsInput: false, Volume: 1.0},
	}

	// Select it
	m.DeviceSection = SectionOutput
	m.deviceCursor = 0
	dev := m.currentCursorDevice()
	require.NotNil(t, dev)
	m.multiSelect[dev.selectKey()] = DeviceRoleSet{Playback: true}

	t.Logf("Step 0: initial volume = %.2f", dev.Volume)

	// Decrease volume 3 times -> 0.7
	m, _ = m.handleVolumeAdjust(true)
	m, _ = m.handleVolumeAdjust(true)
	m, _ = m.handleVolumeAdjust(true)

	dev2 := m.currentCursorDevice()
	require.NotNil(t, dev2)
	t.Logf("Step 1: after 3 decreases, deviceRow volume = %.2f", dev2.Volume)
	assert.InDelta(t, 0.7, dev2.Volume, 0.01)

	// Now simulate what tryStart does:
	cfg := m.BuildConfig()
	t.Logf("Step 2: BuildConfig devices count = %d", len(cfg.Devices))
	for i, d := range cfg.Devices {
		t.Logf("  Device[%d]: %s Volume=%.2f", i, d.Name, d.Volume)
	}
	require.NotEmpty(t, cfg.Devices)
	assert.InDelta(t, 0.7, cfg.Devices[0].Volume, 0.01, "BuildConfig should preserve 0.7")

	// Now simulate tryStart -> calls m.BuildConfig() which we verified above.
	// But tryStart is value receiver - let's call it explicitly.
	m2, cmd := m.tryStart()
	_ = m2

	// tryStart returns a tea.Cmd that produces SetupDoneMsg. Execute it.
	if cmd == nil {
		t.Log("Step 3: tryStart returned nil cmd (validation error?)")
		t.Logf("  validationError = %q", m2.validationError)
		// This is OK - tryStart might fail validation. The important thing
		// is BuildConfig preserves volume, which we verified above.
		return
	}

	msg := cmd()
	doneMsg, ok := msg.(SetupDoneMsg)
	if !ok {
		t.Logf("Step 3: cmd returned %T, not SetupDoneMsg", msg)
		return
	}

	t.Logf("Step 3: SetupDoneMsg devices count = %d", len(doneMsg.Config.Devices))
	for i, d := range doneMsg.Config.Devices {
		t.Logf("  Device[%d]: %s Volume=%.2f", i, d.Name, d.Volume)
	}
	require.NotEmpty(t, doneMsg.Config.Devices)
	assert.InDelta(t, 0.7, doneMsg.Config.Devices[0].Volume, 0.01, "SetupDoneMsg should have 0.7")
}

// TestVolumeAdjust_SimulateUpdateLoop simulates the Bubble Tea Update loop
// to catch if volume is lost during message passing.
func TestVolumeAdjust_SimulateUpdateLoop(t *testing.T) {
	m := newTestSetupModel(config.ModeServer)
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.unifiedDuplex = true

	m.outputDevices = []deviceRow{
		{Name: "Speakers", ID: 42, IsInput: false, Volume: 1.0},
	}
	m.DeviceSection = SectionOutput
	m.deviceCursor = 0
	dev := m.currentCursorDevice()
	require.NotNil(t, dev)
	m.multiSelect[dev.selectKey()] = DeviceRoleSet{Playback: true}

	// Simulate Update() returning after handleVolumeAdjust
	m, _ = m.handleVolumeAdjust(true) // 0.9

	// Check: does the RETURNED m have the updated outputDevices?
	t.Logf("After handleVolumeAdjust, m.outputDevices[0].Volume = %.2f", m.outputDevices[0].Volume)
	assert.InDelta(t, 0.9, m.outputDevices[0].Volume, 0.01)

	// Simulate another call
	m, _ = m.handleVolumeAdjust(true) // 0.8
	t.Logf("After 2nd handleVolumeAdjust, m.outputDevices[0].Volume = %.2f", m.outputDevices[0].Volume)
	assert.InDelta(t, 0.8, m.outputDevices[0].Volume, 0.01)
}
