package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/lHumaNl/echowarp/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdjustClientVolume_NonConference_AppliesGain — when a multiClient has a
// clientGain, adjustClientVolume updates it (per-client volume wiring fix for
// Bug 024). Clamps to [0, 1.5].
func TestAdjustClientVolume_NonConference_AppliesGain(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	gain := NewDeviceGainControl(1.0)
	mc := &multiClient{id: "c1", nickname: "C1"}
	mc.setClientGain(gain)
	app.mu.Lock()
	app.clients["c1"] = mc
	app.mu.Unlock()

	// +0.3 → 1.3
	app.adjustClientVolume("c1", 0.3)
	assert.InDelta(t, 1.3, float64(gain.Gain()), 1e-6)

	// -1.0 → 0.3
	app.adjustClientVolume("c1", -1.0)
	assert.InDelta(t, 0.3, float64(gain.Gain()), 1e-6)

	// Clamp low: -10 → 0
	app.adjustClientVolume("c1", -10)
	assert.Equal(t, float32(0), gain.Gain())

	// Clamp high: +10 → 1.5
	app.adjustClientVolume("c1", 10)
	assert.Equal(t, float32(1.5), gain.Gain())
}

func TestAdjustClientVolume_LogsRoundedVolumeWithoutChangingGain(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	app := NewServerApp(testServerConfig(), slog.New(slog.NewJSONHandler(&logs, nil)), nil, nil, nil)
	gain := NewDeviceGainControl(1.0)
	mc := &multiClient{id: "c1", nickname: "C1"}
	mc.setClientGain(gain)
	app.mu.Lock()
	app.clients["c1"] = mc
	app.mu.Unlock()

	app.adjustClientVolume("c1", -0.1)

	actualGain := gain.Gain()
	record := decodeSlogJSONRecord(t, logs.Bytes())
	loggedVolume, ok := record["volume"].(float64)
	require.True(t, ok, "volume should be logged as a JSON number")
	assert.Equal(t, float32(0.9), actualGain)
	assert.NotEqual(t, 0.9, float64(actualGain))
	assert.Equal(t, 0.9, loggedVolume)
	assert.NotEqual(t, float64(actualGain), loggedVolume)
}

func decodeSlogJSONRecord(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(data), &record))
	return record
}

// TestAdjustClientVolume_NoGain_DoesNotPanic — clients without clientGain
// (e.g. conference, or not yet wired) must not panic.
func TestAdjustClientVolume_NoGain_DoesNotPanic(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	app.mu.Lock()
	app.clients["c1"] = &multiClient{id: "c1", nickname: "C1"}
	app.mu.Unlock()

	app.adjustClientVolume("c1", 0.1) // no gain wired, logs only, no panic
}

func TestServerApp_ShouldUseLegacyMultiCapture_ForMultipleCaptureDevices(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Devices = []config.DeviceEntry{
		{ID: 1, Role: config.RoleCapture, Volume: 1.0},
		{ID: 2, Role: config.RoleCapture, Volume: 1.0},
	}
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	assert.True(t, app.shouldUseLegacyMultiCapture())
}

func TestServerApp_ShouldUseSharedCapture_ForSingleCaptureDevice(t *testing.T) {
	t.Parallel()
	cfg := testServerConfig()
	cfg.Devices = []config.DeviceEntry{{ID: 1, Role: config.RoleCapture, Volume: 1.0}}
	app := NewServerApp(cfg, testAppLogger(), nil, nil, nil)

	assert.False(t, app.shouldUseLegacyMultiCapture())
}
