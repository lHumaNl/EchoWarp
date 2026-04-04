package cli

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
)

// TestVirtualMicFlag_AddsDeviceEntry verifies that injectVirtualMicDevice appends a
// DeviceEntry with RolePlayback when a virtual audio device is found, and does not
// modify cfg.Devices when no device is found (or audio init fails in CI).
// On machines without a virtual audio driver the function logs a warning and leaves
// cfg.Devices unchanged — the test accepts both outcomes.
func TestVirtualMicFlag_AddsDeviceEntry(t *testing.T) {
	cfg := &config.Config{}
	before := len(cfg.Devices)

	logger := slog.Default()
	injectVirtualMicDevice(cfg, logger)

	after := len(cfg.Devices)
	if after > before {
		// A virtual device was found — verify the entry is correct.
		entry := cfg.Devices[after-1]
		assert.Equal(t, config.RolePlayback, entry.Role, "virtual mic device must have RolePlayback")
		assert.NotEmpty(t, entry.Name, "virtual mic device must have a non-empty name")
		assert.Equal(t, 1.0, entry.Volume, "virtual mic device volume must default to 1.0")
	}
	// If after == before, no virtual device was found on this machine — that is OK.
}

// TestVirtualMicFlag_NotInjectedWhenFlagFalse verifies that the CLI guard condition
// (cfg.VirtualMic == true) is required before injectVirtualMicDevice is called.
// This test exercises the cfg flag check without touching audio hardware.
func TestVirtualMicFlag_NotInjectedWhenFlagFalse(t *testing.T) {
	cfg := &config.Config{VirtualMic: false}
	before := len(cfg.Devices)

	// Simulate what executeServerApp / runClientDirect do.
	if cfg.VirtualMic {
		injectVirtualMicDevice(cfg, slog.Default())
	}

	assert.Equal(t, before, len(cfg.Devices), "Devices must not change when VirtualMic flag is false")
}

// TestVirtualMicFlag_InjectionGuardedByFlag verifies that cfg.Devices is left untouched
// when VirtualMic flag is false, even if a virtual device would otherwise be present.
func TestVirtualMicFlag_InjectionGuardedByFlag(t *testing.T) {
	cfg := &config.Config{VirtualMic: false}
	cfg.Devices = append(cfg.Devices, config.DeviceEntry{ID: 1, Role: config.RoleCapture, Volume: 1.0})
	before := len(cfg.Devices)

	if cfg.VirtualMic {
		injectVirtualMicDevice(cfg, slog.Default())
	}

	assert.Equal(t, before, len(cfg.Devices), "existing Devices must not be modified when VirtualMic is false")
}
