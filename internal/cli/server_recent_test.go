package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func seedServerRecent(t *testing.T) {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.StreamMode = config.AudioModeNormal
	cfg.Port = 5500
	cfg.Password = "profile-secret"
	cfg.SampleRate = 24000
	sp := preset.ServerPresets{LastMode: "normal", Presets: map[string]preset.ModePreset{}}
	sp.Set("normal", preset.FromConfig(cfg, recent.DevicePreset{Devices: []recent.PresetDevice{{ID: 99, Name: "Saved microphone", IsInput: true, Volume: .6}}}))
	cfg.StreamMode = config.AudioModeConference
	cfg.ServerMuted = true
	cfg.Port = 5501
	cfg.MaxClients = 8
	sp.Set("conference", preset.FromConfig(cfg, recent.DevicePreset{}))
	require.NoError(t, preset.Save(sp))
}

func TestLegacyDeviceSelectorsWaitForModeResolution(t *testing.T) {
	for _, role := range []config.Mode{config.ModeServer, config.ModeClient} {
		for _, mode := range []config.AudioMode{config.AudioModeNormal, config.AudioModeReverse} {
			t.Run(string(role)+"/"+string(mode), func(t *testing.T) {
				seedServerRecent(t)
				sp := preset.Load()
				profile := config.DefaultConfig()
				profile.StreamMode = mode
				profile.SyncFromStreamMode()
				sp.Set(string(mode), preset.FromConfig(profile, recent.DevicePreset{}))
				require.NoError(t, preset.Save(sp))
				path := filepath.Join(t.TempDir(), "legacy.yml")
				require.NoError(t, os.WriteFile(path, []byte("audio:\n  input_device_id: 1\n  output_device_id: 7\n"), 0600))
				var cfg config.Config
				var intent startup.Intent
				var err error
				if role == config.ModeServer {
					cmd := newServerCmd()
					require.NoError(t, cmd.ParseFlags([]string{"--recent", "--mode", string(mode), "--config", path}))
					cfg, intent, err = loadServerStartup(cmd)
				} else {
					cmd := newClientCmd()
					require.NoError(t, cmd.ParseFlags([]string{"--address", "host", "--config", path}))
					var loaded *config.Config
					loaded, err = loadClientConfig(cmd)
					require.NoError(t, err)
					cfg = *loaded
					intent, err = prepareStartupIntent(cmd, &cfg)
				}
				require.NoError(t, err)
				inventory := []audio.AudioDevice{{ID: 1, Name: "Mic", IsInput: true}, {ID: 7, Name: "Speakers"}}
				network := startup.Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
					return &probe.ProbeServerResult{Mode: string(mode)}, nil
				}}
				prepared, _, err := startup.Resolve(t.Context(), cfg, inventory, intent, network)
				require.NoError(t, err)
				require.Len(t, prepared.Devices, 1)
				want := uint32(1)
				if role == config.ModeServer && mode == config.AudioModeReverse || role == config.ModeClient && mode == config.AudioModeNormal {
					want = 7
				}
				require.Equal(t, want, *prepared.DeviceID)
			})
		}
	}
}

func TestServerRecentTLSRestorationAndExplicitDisable(t *testing.T) {
	seedServerRecent(t)
	cert, key := generateSelfSignedCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, cert, 0600))
	require.NoError(t, os.WriteFile(keyPath, key, 0600))
	sp := preset.Load()
	mp := sp.Presets["normal"]
	mp.TLS = true
	mp.TLSCert = certPath
	mp.TLSKey = keyPath
	sp.Presets["normal"] = mp
	require.NoError(t, preset.Save(sp))
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	cfg, _, err := loadServerStartup(cmd)
	require.NoError(t, err)
	tlsConfig, err := setupTLSConfig(&cfg)
	require.NoError(t, err)
	require.NotNil(t, tlsConfig)
	require.True(t, preset.FromConfig(cfg, recent.DevicePreset{}).TLS)
	t.Setenv("ECHOWARP_TLS", "false")
	cfg, _, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.Empty(t, cfg.TLSCert)
	require.Empty(t, cfg.TLSKey)
	tlsConfig, err = setupTLSConfig(&cfg)
	require.NoError(t, err)
	require.Nil(t, tlsConfig)
}

func TestServerRecentBadTLSAndEmptyDevicesFailClosed(t *testing.T) {
	seedServerRecent(t)
	sp := preset.Load()
	mp := sp.Presets["normal"]
	mp.TLS = true
	mp.TLSCert = "missing-cert"
	mp.TLSKey = "missing-key"
	sp.Presets["normal"] = mp
	require.NoError(t, preset.Save(sp))
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	_, _, err = startup.Resolve(t.Context(), cfg, nil, intent, startup.Network{})
	require.ErrorContains(t, err, "TLS")
	mp.TLS = false
	mp.TLSCert = ""
	mp.TLSKey = ""
	sp.Presets["normal"] = mp
	require.NoError(t, preset.Save(sp))
	cmd = newServerCmd()
	emptyDevices := filepath.Join(t.TempDir(), "empty.yml")
	require.NoError(t, os.WriteFile(emptyDevices, []byte("audio:\n  devices: []\n"), 0600))
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--config", emptyDevices}))
	cfg, intent, err = loadServerStartup(cmd)
	require.NoError(t, err)
	_, _, err = startup.Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "Saved microphone", IsInput: true}}, intent, startup.Network{})
	require.ErrorContains(t, err, "missing required")
}

func TestServerRecentRestoresFullProfileAndSafeDevice(t *testing.T) {
	seedServerRecent(t)
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	require.Empty(t, intent.Problem)
	require.Equal(t, 5500, cfg.Port)
	require.Equal(t, "profile-secret", cfg.Password)
	require.Equal(t, uint32(24000), cfg.SampleRate)
	prepared, _, err := startup.Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 3, Name: "Saved microphone", IsInput: true}}, intent, startup.Network{})
	require.NoError(t, err)
	require.Equal(t, uint32(3), *prepared.DeviceID)
	require.Equal(t, .6, prepared.Devices[0].Volume)
}

func TestServerRecentLayerPrecedenceAndExplicitEmpty(t *testing.T) {
	seedServerRecent(t)
	path := filepath.Join(t.TempDir(), "override.yml")
	require.NoError(t, os.WriteFile(path, []byte("network:\n  port: 5600\nsecurity:\n  password: file-secret\nserver_muted: true\n"), 0600))
	t.Setenv("ECHOWARP_PORT", "5700")
	t.Setenv("ECHOWARP_PASSWORD", "")
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--config", path, "--port", "5800", "--server-muted=false"}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	require.Empty(t, intent.Problem)
	require.Equal(t, 5800, cfg.Port)
	require.Empty(t, cfg.Password)
	require.False(t, cfg.ServerMuted)
	cmd = newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--password", ""}))
	cfg, _, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.Empty(t, cfg.Password)
}

func TestServerRecentModeAndHub(t *testing.T) {
	seedServerRecent(t)
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--mode", "conference"}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	require.Empty(t, intent.Problem)
	require.Equal(t, 5501, cfg.Port)
	require.True(t, cfg.ServerMuted)
	prepared, _, err := startup.Resolve(t.Context(), cfg, nil, intent, startup.Network{})
	require.NoError(t, err)
	require.Nil(t, prepared.DeviceID)
	cmd = newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--mode", "duplex"}))
	_, intent, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.Contains(t, intent.Problem, "duplex")
}

func TestServerRecentSecurityControlsAndRateLimit(t *testing.T) {
	seedServerRecent(t)
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.HWIDRequired = true
	cfg.BanFilePath = "custom-ban.yaml"
	cfg.RateLimit = 9
	// Exercise the same save API used on a successful listener event.
	require.NoError(t, preset.SaveSnapshot(cfg, recent.DevicePreset{Devices: []recent.PresetDevice{{Name: "Mic", IsInput: true, Volume: 1}}}))
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	restored, _, err := loadServerStartup(cmd)
	require.NoError(t, err)
	require.True(t, restored.HWIDRequired)
	require.Equal(t, "custom-ban.yaml", restored.BanFilePath)
	require.Equal(t, 9, restored.RateLimit)
	cmd = newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--hwid-required=false", "--ban-file", "", "--rate-limit", "0"}))
	restored, _, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.False(t, restored.HWIDRequired)
	require.Empty(t, restored.BanFilePath)
	require.Zero(t, restored.RateLimit)
	require.Nil(t, serverRateLimiter(restored))
	cmd = newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--rate-limit", "10"}))
	restored, _, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.Equal(t, 10, restored.RateLimit)
	limiter := serverRateLimiter(restored)
	require.NotNil(t, limiter)
	for range 10 {
		require.True(t, limiter.Allow("test-ip"))
	}
	require.False(t, limiter.Allow("test-ip"))
	cmd = newServerCmd()
	restored, _, err = loadServerStartup(cmd)
	require.NoError(t, err)
	require.Equal(t, 5, restored.RateLimit)
}

func TestServerRecentEnvironmentDeviceWinsOverFileArray(t *testing.T) {
	seedServerRecent(t)
	path := filepath.Join(t.TempDir(), "devices.yml")
	require.NoError(t, os.WriteFile(path, []byte("audio:\n  devices:\n    - id: 1\n      role: capture\n      volume: 1\n"), 0600))
	t.Setenv("ECHOWARP_INPUT_DEVICE_ID", "2")
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent", "--config", path}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	devices := []audio.AudioDevice{{ID: 1, Name: "File microphone", IsInput: true}, {ID: 2, Name: "Environment microphone", IsInput: true}}
	prepared, _, err := startup.Resolve(t.Context(), cfg, devices, intent, startup.Network{})
	require.NoError(t, err)
	require.Len(t, prepared.CaptureDevices(), 1)
	require.Equal(t, uint32(2), prepared.CaptureDevices()[0].ID)
	t.Setenv("ECHOWARP_INPUT_DEVICE_ID", "999")
	cfg, intent, err = loadServerStartup(cmd)
	require.NoError(t, err)
	_, _, err = startup.Resolve(t.Context(), cfg, devices, intent, startup.Network{})
	require.Error(t, err, "invalid high-priority ID must not fall back to file microphone")
}

func TestServerRecentLegacyVolumeDefaultsToUnity(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	fixture := "last_mode: normal\npresets:\n  normal:\n    devices:\n      - name: Mic\n        is_input: true\n"
	require.NoError(t, os.WriteFile(preset.FilePath(), []byte(fixture), 0600))
	cmd := newServerCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--recent"}))
	cfg, intent, err := loadServerStartup(cmd)
	require.NoError(t, err)
	prepared, _, err := startup.Resolve(t.Context(), cfg, []audio.AudioDevice{{ID: 1, Name: "Mic", IsInput: true}}, intent, startup.Network{})
	require.NoError(t, err)
	require.Equal(t, 1.0, prepared.Devices[0].Volume)
}
