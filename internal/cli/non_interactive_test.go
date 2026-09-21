package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
)

func TestPrepareNonInteractiveAudioConfig_ValidModes(t *testing.T) {
	tests := []struct {
		name            string
		cfg             config.Config
		wantDeviceID    uint32
		wantInputID     uint32
		wantOutputID    uint32
		wantDeviceCount int
		wantNilDeviceID bool
		wantNilInputID  bool
		wantNilOutputID bool
	}{
		{
			name:            "normal server capture device",
			cfg:             configWithDevices(config.ModeServer, config.AudioModeNormal, device(1, config.RoleCapture)),
			wantDeviceID:    1,
			wantInputID:     1,
			wantNilOutputID: true,
			wantDeviceCount: 1,
		},
		{
			name:            "reverse server playback device",
			cfg:             configWithDevices(config.ModeServer, config.AudioModeReverse, device(2, config.RolePlayback)),
			wantDeviceID:    2,
			wantOutputID:    2,
			wantNilInputID:  true,
			wantDeviceCount: 1,
		},
		{
			name:            "muted conference server is a device-less hub",
			cfg:             configWithMode(config.ModeServer, config.AudioModeConference),
			wantNilDeviceID: true,
			wantNilInputID:  true,
			wantNilOutputID: true,
			wantDeviceCount: 0,
		},
		{
			name:            "normal client playback device",
			cfg:             configWithDevices(config.ModeClient, config.AudioModeNormal, device(3, config.RolePlayback)),
			wantDeviceID:    3,
			wantOutputID:    3,
			wantNilInputID:  true,
			wantDeviceCount: 1,
		},
		{
			name:            "reverse client capture device",
			cfg:             configWithDevices(config.ModeClient, config.AudioModeReverse, device(4, config.RoleCapture)),
			wantDeviceID:    4,
			wantInputID:     4,
			wantNilOutputID: true,
			wantDeviceCount: 1,
		},
		{
			name: "duplex client role devices",
			cfg: configWithDevices(
				config.ModeClient,
				config.AudioModeDuplex,
				device(5, config.RoleCapture),
				device(6, config.RolePlayback),
			),
			wantDeviceID:    6,
			wantInputID:     5,
			wantOutputID:    6,
			wantDeviceCount: 2,
		},
		{
			name: "duplex server uses playback as legacy device",
			cfg: configWithDevices(
				config.ModeServer,
				config.AudioModeDuplex,
				device(15, config.RoleCapture),
				device(16, config.RolePlayback),
			),
			wantDeviceID:    16,
			wantInputID:     15,
			wantOutputID:    16,
			wantDeviceCount: 2,
		},
		{
			name: "conference client input and output fields",
			cfg: func() config.Config {
				cfg := configWithMode(config.ModeClient, config.AudioModeConference)
				inputID, outputID := uint32(7), uint32(8)
				cfg.InputDeviceID = &inputID
				cfg.OutputDeviceID = &outputID
				return cfg
			}(),
			wantDeviceID:    8,
			wantInputID:     7,
			wantOutputID:    8,
			wantDeviceCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			require.NoError(t, prepareNonInteractiveAudioConfig(&cfg))

			assertDeviceID(t, cfg.DeviceID, tt.wantDeviceID, tt.wantNilDeviceID)
			assertDeviceID(t, cfg.InputDeviceID, tt.wantInputID, tt.wantNilInputID)
			assertDeviceID(t, cfg.OutputDeviceID, tt.wantOutputID, tt.wantNilOutputID)
			assert.Len(t, cfg.Devices, tt.wantDeviceCount)
		})
	}
}

func TestPrepareNonInteractiveAudioConfig_LegacyDeviceSupportsAllModes(t *testing.T) {
	cfg := configWithMode(config.ModeClient, config.AudioModeDuplex)
	deviceID := uint32(9)
	cfg.DeviceID = &deviceID

	require.NoError(t, prepareNonInteractiveAudioConfig(&cfg))
	assert.Equal(t, deviceID, *cfg.DeviceID)
}

func TestPrepareNonInteractiveAudioConfig_RejectsMissingRoles(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name:    "normal server needs capture",
			cfg:     configWithMode(config.ModeServer, config.AudioModeNormal),
			wantErr: "capture device",
		},
		{
			name:    "normal client needs playback",
			cfg:     configWithMode(config.ModeClient, config.AudioModeNormal),
			wantErr: "playback device",
		},
		{
			name: "duplex needs playback",
			cfg: configWithDevices(
				config.ModeServer,
				config.AudioModeDuplex,
				device(1, config.RoleCapture),
			),
			wantErr: "playback device",
		},
		{
			name: "conference participant server needs capture",
			cfg: func() config.Config {
				cfg := configWithMode(config.ModeServer, config.AudioModeConference)
				cfg.ServerMuted = false
				return cfg
			}(),
			wantErr: "capture device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			err := prepareNonInteractiveAudioConfig(&cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestPrepareProbedClientConfig_ValidatesAuthoritativeServerMode(t *testing.T) {
	cfg := configWithDevices(
		config.ModeClient,
		config.AudioModeDuplex,
		device(21, config.RoleCapture),
	)
	cmd := newClientCmd()

	err := prepareProbedClientConfig(cmd, &cfg, &probe.ProbeServerResult{
		Mode:       string(config.AudioModeReverse),
		SampleRate: cfg.SampleRate,
		Channels:   cfg.Channels,
	})

	require.NoError(t, err)
	assert.Equal(t, config.AudioModeReverse, cfg.GetAudioMode())
	assert.False(t, cfg.Duplex)
	assert.True(t, cfg.Reverse)
	assert.Equal(t, uint32(21), *cfg.DeviceID)
}

func configWithMode(mode config.Mode, audioMode config.AudioMode) config.Config {
	cfg := config.DefaultConfig()
	cfg.Mode = mode
	cfg.StreamMode = audioMode
	cfg.SyncFromStreamMode()
	if mode == config.ModeClient {
		cfg.Address = "127.0.0.1"
	}
	if mode == config.ModeServer && audioMode == config.AudioModeConference {
		cfg.ServerMuted = true
	}
	return cfg
}

func configWithDevices(mode config.Mode, audioMode config.AudioMode, devices ...config.DeviceEntry) config.Config {
	cfg := configWithMode(mode, audioMode)
	cfg.Devices = devices
	return cfg
}

func device(id uint32, role config.DeviceRole) config.DeviceEntry {
	return config.DeviceEntry{ID: id, Role: role, Volume: 1.0}
}

func assertDeviceID(t *testing.T, actual *uint32, expected uint32, wantNil bool) {
	t.Helper()
	if wantNil {
		assert.Nil(t, actual)
		return
	}
	if assert.NotNil(t, actual) {
		assert.Equal(t, expected, *actual)
	}
}
