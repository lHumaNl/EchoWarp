package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestStartupIntentExplicitSelection(t *testing.T) {
	for _, args := range [][]string{{}, {"--device", "0"}, {"--playback-device", "0"}, {"--address", "example.org"}, {"--config", "client.yml"}} {
		cmd := newClientCmd()
		require.NoError(t, cmd.ParseFlags(args))
		cfg := config.DefaultConfig()
		intent, err := prepareStartupIntent(cmd, &cfg)
		require.NoError(t, err)
		require.Equal(t, len(args) > 0, intent.Requested)
	}
}

func TestNamedCaptureReplacesUntypedLegacyConfigDevice(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", root)
	path := filepath.Join(root, "server.yml")
	require.NoError(t, os.WriteFile(path, []byte("audio:\n  devices:\n    - id: 1\n      name: Old microphone\n      volume: 1\n"), 0600))
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--config", path, "--device-name", "USB microphone"}))
	cfg, err := loadConfig(cmd, config.ModeServer)
	require.NoError(t, err)
	require.Len(t, cfg.Devices, 1)
	require.Empty(t, cfg.Devices[0].Role)
	require.Empty(t, cfg.Devices[0].Type)
	intent, err := prepareStartupIntent(cmd, &cfg)
	require.NoError(t, err)
	prepared, _, err := startup.Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "Old microphone", IsInput: true}, {ID: 2, Name: "USB microphone", IsInput: true}}, intent, startup.Network{})
	require.NoError(t, err)
	require.Len(t, prepared.CaptureDevices(), 1)
	require.Equal(t, uint32(2), prepared.CaptureDevices()[0].ID)
	// The numeric role selector must also replace an untyped legacy entry.
	cmd = newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--config", path, "--capture-device", "2"}))
	cfg, err = loadConfig(cmd, config.ModeServer)
	require.NoError(t, err)
	intent, err = prepareStartupIntent(cmd, &cfg)
	require.NoError(t, err)
	prepared, _, err = startup.Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "Old microphone", IsInput: true}, {ID: 2, Name: "USB microphone", IsInput: true}}, intent, startup.Network{})
	require.NoError(t, err)
	require.Len(t, prepared.CaptureDevices(), 1)
	require.Equal(t, uint32(2), prepared.CaptureDevices()[0].ID)
}

func TestNamedPlaybackKeepsExplicitCaptureThroughCLIResolution(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, recent.Save([]recent.Server{{Address: "saved-host", Port: 4415, LastConnected: time.Now(), LastMode: "duplex", Presets: map[string]recent.DevicePreset{
		"duplex": {Devices: []recent.PresetDevice{{ID: 1, Name: "Saved microphone", IsInput: true, Volume: 1}}},
	}}}))
	cmd := newClientCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--capture-device", "2", "--device-name", "usb"}))
	cfg, err := loadClientConfig(cmd)
	require.NoError(t, err)
	intent, err := prepareStartupIntent(cmd, cfg)
	require.NoError(t, err)
	devices := []audio.AudioDevice{{ID: 1, Name: "Saved microphone", IsInput: true}, {ID: 2, Name: "Explicit microphone", IsInput: true}, {ID: 0, Name: "USB Audio Speakers"}}
	prepared, _, err := startup.Resolve(t.Context(), *cfg, devices, intent, startup.Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return &probe.ProbeServerResult{Mode: "duplex"}, nil
	}})
	require.NoError(t, err)
	require.Equal(t, uint32(2), *prepared.InputDeviceID)
	require.Equal(t, uint32(0), *prepared.OutputDeviceID)
	require.Equal(t, "Explicit microphone", prepared.CaptureDevices()[0].Name)
}

func TestHeadlessRecentKeepsExplicitCertificateVerification(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, recent.Save([]recent.Server{{Address: "host", Port: 4415, LastConnected: time.Now(), LastMode: "normal"}}))
	cmd := newClientCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--no-interactive", "--playback-device", "0", "--tls-insecure=false"}))
	cfg, err := loadClientConfig(cmd)
	require.NoError(t, err)
	intent, err := prepareStartupIntent(cmd, cfg)
	require.NoError(t, err)
	info := &probe.ProbeServerResult{Mode: "normal", TLSRequired: true, TLSSelfSigned: true}
	require.NoError(t, prepareRecentClientConfig(cmd, cfg, []audio.AudioDevice{{ID: 0, Name: "Speakers"}}, info, intent))
	require.True(t, cfg.TLS)
	require.False(t, cfg.TLSInsecure, "final config handed to tls.Config must preserve explicit verification")
}

func TestStartupRecentAndConflicts(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, recent.Save([]recent.Server{{Address: "old", Port: 4415, LastConnected: time.Now().Add(-time.Hour)}, {Address: "latest", Port: 4420, LastConnected: time.Now()}}))
	cmd := newClientCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--playback-device", "0"}))
	cfg := config.DefaultConfig()
	cfg.Address = "configured-host"
	intent, err := prepareStartupIntent(cmd, &cfg)
	require.NoError(t, err)
	require.True(t, intent.Requested)
	require.Equal(t, "latest", cfg.Address)
	require.Equal(t, 4420, cfg.Port)
	require.NotNil(t, intent.Recent)
	for _, args := range [][]string{{"--recent", "--address", "host"}, {"--recent", "--port", "4415"}} {
		cmd = newClientCmd()
		require.NoError(t, cmd.ParseFlags(args))
		_, err = prepareStartupIntent(cmd, &cfg)
		require.Error(t, err)
	}
}

func TestStartupRecentMissingHistoryStaysInTUI(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cmd := newClientCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	cfg := config.DefaultConfig()
	intent, err := prepareStartupIntent(cmd, &cfg)
	require.NoError(t, err)
	require.NotEmpty(t, intent.Problem)
	require.Empty(t, cfg.Address)
	require.Nil(t, cmd.Flags().Lookup("host"))
	require.Nil(t, cmd.Flags().Lookup("auto-start"))
}
