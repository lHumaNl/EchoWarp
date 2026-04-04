package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_HasSaneDefaults(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 4415, cfg.Port)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 64000, cfg.OpusBitrate)
	assert.Equal(t, 5, cfg.OpusComplexity)
	assert.Equal(t, "voip", cfg.OpusApplication)
	assert.True(t, cfg.OpusDTX)
	assert.True(t, cfg.OpusFEC)
	assert.NotEmpty(t, cfg.STUNServers)
	assert.Equal(t, 1, cfg.MaxClients)
	assert.Equal(t, 5, cfg.MaxReconnectAttempts)
	assert.Equal(t, 3, cfg.ReconnectIntervalSec)
	assert.Equal(t, 5, cfg.MaxFailedAttempts)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoadFromFile_ValidYAML_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
role: server
port: 5000
sample_rate: 48000
channels: 2
password: "secret123"
log_level: debug
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, ModeServer, cfg.Mode)
	assert.Equal(t, 5000, cfg.Port)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, "secret123", cfg.Password)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoadFromFile_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	err := os.WriteFile(path, []byte("{{{{invalid yaml"), 0600)
	require.NoError(t, err)

	_, err = LoadFromFile(path)
	assert.Error(t, err)
}

func TestLoadFromFile_MissingFile_ReturnsError(t *testing.T) {
	// Use a path inside t.TempDir() that does not exist — works on all platforms.
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "no-such-dir", "config.yaml"))
	assert.Error(t, err)
}

func TestLoadFromFile_PartialConfig_MergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")

	content := `
role: client
address: "192.168.1.1"
port: 5000
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, ModeClient, cfg.Mode)
	assert.Equal(t, "192.168.1.1", cfg.Address)
	assert.Equal(t, 5000, cfg.Port)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 64000, cfg.OpusBitrate)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestSaveToFile_CreatesValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.yaml")

	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Port = 5000
	cfg.Password = "secret"

	err := cfg.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, ModeServer, loaded.Mode)
	assert.Equal(t, 5000, loaded.Port)
	assert.Equal(t, "secret", loaded.Password)
}

func TestSaveToFile_CorrectPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permissions are not enforced on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "perms.yaml")

	cfg := DefaultConfig()
	cfg.Password = "secret"
	err := cfg.SaveToFile(path)
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestValidate_ValidConfig_NoErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

func TestValidate_InvalidPort_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Port = 0
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)

	cfg.Port = 70000
	errs = cfg.Validate()
	assert.NotEmpty(t, errs)
}

func TestValidate_ClientWithoutAddress_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeClient
	cfg.Address = ""
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
}

func TestValidate_InvalidSampleRate_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.SampleRate = 22050
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
}

func TestValidate_InvalidLogLevel_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.LogLevel = "verbose"
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
}

func TestValidate_ValidChannels(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer

	cfg.Channels = 1
	assert.Empty(t, cfg.Validate())

	cfg.Channels = 2
	assert.Empty(t, cfg.Validate())

	cfg.Channels = 3
	assert.NotEmpty(t, cfg.Validate())
}

func TestConfig_SafeString_MasksSecrets(t *testing.T) {
	cfg := &Config{
		Mode:         ModeServer,
		Port:         5000,
		Password:     "secret123",
		PasswordHash: "hash456",
		TURNServers: []TURNServer{
			{URL: "turn:example.com", Username: "user", Credential: "turnsecret"},
		},
	}

	result := cfg.SafeString()

	assert.NotContains(t, result, "secret123")
	assert.NotContains(t, result, "hash456")
	assert.NotContains(t, result, "turnsecret")
	assert.Contains(t, result, "***REDACTED***")
	assert.Contains(t, result, "turn:example.com")
	assert.Contains(t, result, "user")
}

func TestConfig_SafeString_EmptySecrets(t *testing.T) {
	cfg := &Config{
		Mode: ModeClient,
		Port: 5000,
	}

	result := cfg.SafeString()

	assert.NotContains(t, result, "***REDACTED***")
}

func TestValidate_TrustedProxies(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		expectErrors   bool
	}{
		{
			name:           "empty_list_valid",
			trustedProxies: []string{},
			expectErrors:   false,
		},
		{
			name:           "localhost_valid",
			trustedProxies: []string{"localhost"},
			expectErrors:   false,
		},
		{
			name:           "valid_ipv4",
			trustedProxies: []string{"192.168.1.1"},
			expectErrors:   false,
		},
		{
			name:           "valid_ipv6",
			trustedProxies: []string{"::1", "2001:db8::1"},
			expectErrors:   false,
		},
		{
			name:           "valid_cidr",
			trustedProxies: []string{"10.0.0.0/8", "192.168.0.0/16"},
			expectErrors:   false,
		},
		{
			name:           "mixed_valid",
			trustedProxies: []string{"localhost", "10.0.0.0/8", "192.168.1.100"},
			expectErrors:   false,
		},
		{
			name:           "invalid_ip",
			trustedProxies: []string{"not-an-ip"},
			expectErrors:   true,
		},
		{
			name:           "invalid_cidr",
			trustedProxies: []string{"10.0.0.0/33"},
			expectErrors:   true,
		},
		{
			name:           "empty_string",
			trustedProxies: []string{""},
			expectErrors:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = ModeServer
			cfg.TrustedProxies = tt.trustedProxies
			errs := cfg.Validate()

			if tt.expectErrors {
				assert.NotEmpty(t, errs, "expected validation errors for %v", tt.trustedProxies)
			} else {
				assert.Empty(t, errs, "unexpected validation errors for %v: %v", tt.trustedProxies, errs)
			}
		})
	}
}

func TestLoadFromFile_TrustedProxies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted.yaml")

	content := `
role: server
trusted_proxies:
  - localhost
  - 10.0.0.0/8
  - 192.168.1.100
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"localhost", "10.0.0.0/8", "192.168.1.100"}, cfg.TrustedProxies)
}

// ============================================================================
// Viper Integration Tests
// ============================================================================

func TestLoadWithViper_DefaultsOnly_Success(t *testing.T) {
	cfg, err := LoadWithViper("", ModeServer)
	require.NoError(t, err)

	// Verify defaults are applied
	assert.Equal(t, ModeServer, cfg.Mode)
	assert.Equal(t, 4415, cfg.Port)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 64000, cfg.OpusBitrate)
	assert.Equal(t, 5, cfg.OpusComplexity)
	assert.Equal(t, "voip", cfg.OpusApplication)
	assert.True(t, cfg.OpusDTX)
	assert.True(t, cfg.OpusFEC)
	assert.NotEmpty(t, cfg.STUNServers)
	assert.Equal(t, 1, cfg.MaxClients)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoadWithViper_ConfigFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
role: client
port: 9999
address: 192.168.1.100
sample_rate: 44100
channels: 2
password: secret123
log_level: debug
max_clients: 10
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadWithViper(path, ModeServer)
	require.NoError(t, err)

	// Config file values should be loaded
	assert.Equal(t, ModeServer, cfg.Mode) // Mode is overridden by parameter
	assert.Equal(t, 9999, cfg.Port)
	assert.Equal(t, "192.168.1.100", cfg.Address)
	assert.Equal(t, uint32(44100), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, "secret123", cfg.Password)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 10, cfg.MaxClients)
}

func TestLoadWithViper_EnvVarOverride_Success(t *testing.T) {
	// Set environment variables
	t.Setenv("ECHOWARP_PORT", "8888")
	t.Setenv("ECHOWARP_PASSWORD", "envpassword")
	t.Setenv("ECHOWARP_SAMPLE_RATE", "24000")
	t.Setenv("ECHOWARP_CHANNELS", "2")
	t.Setenv("ECHOWARP_LOG_LEVEL", "error")

	cfg, err := LoadWithViper("", ModeClient)
	require.NoError(t, err)

	// Env vars should override defaults
	assert.Equal(t, 8888, cfg.Port)
	assert.Equal(t, "envpassword", cfg.Password)
	assert.Equal(t, uint32(24000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, "error", cfg.LogLevel)
}

func TestLoadWithViper_Priority_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Config file sets port to 7000
	content := `
port: 7000
password: filepassword
log_level: warn
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	// Env var sets port to 9000
	t.Setenv("ECHOWARP_PORT", "9000")
	t.Setenv("ECHOWARP_LOG_LEVEL", "debug")

	cfg, err := LoadWithViper(path, ModeServer)
	require.NoError(t, err)

	// Env vars should override config file
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, "filepassword", cfg.Password) // Not overridden, should come from file
	assert.Equal(t, "debug", cfg.LogLevel)        // Env overrides file
}

func TestLoadWithViper_EnvVarCommaSeparated_Success(t *testing.T) {
	t.Setenv("ECHOWARP_STUN_SERVERS", "stun:custom1.com:3478,stun:custom2.com:3478")

	cfg, err := LoadWithViper("", ModeServer)
	require.NoError(t, err)

	// Comma-separated env var should be parsed into slice
	assert.Equal(t, []string{"stun:custom1.com:3478", "stun:custom2.com:3478"}, cfg.STUNServers)
}

func TestLoadWithViper_InvalidConfigFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")

	err := os.WriteFile(path, []byte("{{{{invalid yaml"), 0600)
	require.NoError(t, err)

	_, err = LoadWithViper(path, ModeServer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadWithViper_MissingConfigFile_ReturnsError(t *testing.T) {
	_, err := LoadWithViper(filepath.Join(t.TempDir(), "no-such-dir", "config.yaml"), ModeServer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadWithViper_AllEnvVars_Success(t *testing.T) {
	// Test all environment variable mappings
	t.Setenv("ECHOWARP_MODE", "reverse")
	t.Setenv("ECHOWARP_PORT", "5555")
	t.Setenv("ECHOWARP_ADDRESS", "10.0.0.1")
	t.Setenv("ECHOWARP_DEVICE_ID", "42")
	t.Setenv("ECHOWARP_SAMPLE_RATE", "16000")
	t.Setenv("ECHOWARP_CHANNELS", "2")
	t.Setenv("ECHOWARP_VIRTUAL_MIC", "true")
	t.Setenv("ECHOWARP_OPUS_BITRATE", "128000")
	t.Setenv("ECHOWARP_OPUS_COMPLEXITY", "10")
	t.Setenv("ECHOWARP_OPUS_APPLICATION", "audio")
	t.Setenv("ECHOWARP_OPUS_DTX", "false")
	t.Setenv("ECHOWARP_OPUS_FEC", "false")
	t.Setenv("ECHOWARP_TLS_CERT", "/path/to/cert.pem")
	t.Setenv("ECHOWARP_TLS_KEY", "/path/to/key.pem")
	t.Setenv("ECHOWARP_TLS", "true")
	t.Setenv("ECHOWARP_TLS_INSECURE", "true")
	t.Setenv("ECHOWARP_MAX_CLIENTS", "5")
	t.Setenv("ECHOWARP_MAX_RECONNECT_ATTEMPTS", "10")
	t.Setenv("ECHOWARP_RECONNECT_INTERVAL_SEC", "5")
	t.Setenv("ECHOWARP_MAX_FAILED_ATTEMPTS", "3")
	t.Setenv("ECHOWARP_BAN_FILE_PATH", "/custom/ban.json")
	t.Setenv("ECHOWARP_LOG_LEVEL", "warn")
	t.Setenv("ECHOWARP_LOG_TO_FILE", "true")
	t.Setenv("ECHOWARP_LOG_FILE", "/var/log/echowarp.log")
	t.Setenv("ECHOWARP_NO_SIMD_OPTIMIZATION", "true")
	t.Setenv("ECHOWARP_NO_POOL_WARMUP", "true")

	cfg, err := LoadWithViper("", ModeServer)
	require.NoError(t, err)

	// Mode should be overridden by parameter, not env var
	assert.Equal(t, ModeServer, cfg.Mode)

	// Verify all env vars are loaded
	assert.True(t, cfg.Reverse)
	assert.Equal(t, 5555, cfg.Port)
	assert.Equal(t, "10.0.0.1", cfg.Address)
	require.NotNil(t, cfg.DeviceID)
	assert.Equal(t, uint32(42), *cfg.DeviceID)
	assert.Equal(t, uint32(16000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.True(t, cfg.VirtualMic)
	assert.Equal(t, 128000, cfg.OpusBitrate)
	assert.Equal(t, 10, cfg.OpusComplexity)
	assert.Equal(t, "audio", cfg.OpusApplication)
	assert.False(t, cfg.OpusDTX)
	assert.False(t, cfg.OpusFEC)
	assert.Equal(t, "/path/to/cert.pem", cfg.TLSCert)
	assert.Equal(t, "/path/to/key.pem", cfg.TLSKey)
	assert.True(t, cfg.TLS)
	assert.True(t, cfg.TLSInsecure)
	assert.Equal(t, 5, cfg.MaxClients)
	assert.Equal(t, 10, cfg.MaxReconnectAttempts)
	assert.Equal(t, 3, cfg.MaxFailedAttempts)
	assert.Equal(t, 5, cfg.ReconnectIntervalSec)
	assert.Equal(t, "/custom/ban.json", cfg.BanFilePath)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.True(t, cfg.LogToFile)
	assert.Equal(t, "/var/log/echowarp.log", cfg.LogFile)
	assert.True(t, cfg.NoSIMDOptimization)
	assert.True(t, cfg.NoPoolWarmup)
}

func TestLoadWithViper_PartialConfig_MergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")

	// Only set some values
	content := `
port: 6000
address: 192.168.1.50
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadWithViper(path, ModeClient)
	require.NoError(t, err)

	// Specified values from file
	assert.Equal(t, 6000, cfg.Port)
	assert.Equal(t, "192.168.1.50", cfg.Address)

	// Unspecified values should retain defaults
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
	assert.Equal(t, 64000, cfg.OpusBitrate)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestValidate_DuplexAndReverse_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Duplex = true
	cfg.Reverse = true
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "duplex and reverse cannot be used together")
}

func TestValidate_DuplexOnly_NoErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Duplex = true
	errs := cfg.Validate()
	assert.Empty(t, errs)
}

func TestEffectiveAudioBufferFrames_ModeBased(t *testing.T) {
	cfg := DefaultConfig()
	// Default (normal mode, AudioBufferFrames=0) → 5
	assert.Equal(t, 5, cfg.EffectiveAudioBufferFrames())

	// Duplex → 3
	cfg.Duplex = true
	assert.Equal(t, 3, cfg.EffectiveAudioBufferFrames())

	// Conference → 3
	cfg.Duplex = false
	cfg.Conference = true
	assert.Equal(t, 3, cfg.EffectiveAudioBufferFrames())

	// Explicit override takes priority
	cfg.AudioBufferFrames = 7
	assert.Equal(t, 7, cfg.EffectiveAudioBufferFrames())
}

func TestAudioMode(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "normal", cfg.AudioMode())

	cfg.Reverse = true
	assert.Equal(t, "reverse", cfg.AudioMode())

	cfg.Reverse = false
	cfg.Duplex = true
	assert.Equal(t, "duplex", cfg.AudioMode())
}

func TestNestedConfig_DuplexRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Duplex = true
	cfg.SyncToStreamMode()
	id1 := uint32(1)
	id2 := uint32(2)
	cfg.InputDeviceID = &id1
	cfg.OutputDeviceID = &id2

	nested := cfg.toNested()
	assert.Equal(t, AudioModeDuplex, nested.StreamMode)
	assert.Equal(t, &id1, nested.Audio.InputDeviceID)
	assert.Equal(t, &id2, nested.Audio.OutputDeviceID)

	restored := fromNested(DefaultConfig(), nested)
	assert.True(t, restored.Duplex)
	assert.Equal(t, &id1, restored.InputDeviceID)
	assert.Equal(t, &id2, restored.OutputDeviceID)
}

// ============================================================================
// Multi-Device (Phase 1.5) Tests
// ============================================================================

func TestDeviceEntry_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Duplex = true
	cfg.Devices = []DeviceEntry{
		{ID: 1, Role: RoleCapture, Volume: 1.0},
		{ID: 3, Role: RoleCapture, Volume: 0.5},
		{ID: 2, Role: RolePlayback, Volume: 1.0},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "multi.yaml")
	err := cfg.SaveToFile(path)
	require.NoError(t, err)

	loaded, err := LoadFromFile(path)
	require.NoError(t, err)
	require.Len(t, loaded.Devices, 3)
	assert.Equal(t, uint32(1), loaded.Devices[0].ID)
	assert.Equal(t, RoleCapture, loaded.Devices[0].Role)
	assert.Equal(t, 1.0, loaded.Devices[0].Volume)
	assert.Equal(t, uint32(3), loaded.Devices[1].ID)
	assert.Equal(t, 0.5, loaded.Devices[1].Volume)
	assert.Equal(t, RolePlayback, loaded.Devices[2].Role)
}

func TestEffectiveDevices_MigrationFromLegacy(t *testing.T) {
	// Single DeviceID → one entry without role
	cfg := DefaultConfig()
	id := uint32(5)
	cfg.DeviceID = &id
	devices := cfg.EffectiveDevices()
	require.Len(t, devices, 1)
	assert.Equal(t, uint32(5), devices[0].ID)
	assert.Equal(t, DeviceRole(""), devices[0].Role)
	assert.Equal(t, 1.0, devices[0].Volume)
}

func TestEffectiveDevices_MigrationDuplex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Duplex = true
	in := uint32(1)
	out := uint32(2)
	cfg.InputDeviceID = &in
	cfg.OutputDeviceID = &out
	devices := cfg.EffectiveDevices()
	require.Len(t, devices, 2)
	assert.Equal(t, RoleCapture, devices[0].Role)
	assert.Equal(t, RolePlayback, devices[1].Role)
}

func TestEffectiveDevices_ExplicitDevicesTakePrecedence(t *testing.T) {
	cfg := DefaultConfig()
	id := uint32(99)
	cfg.DeviceID = &id
	cfg.Devices = []DeviceEntry{{ID: 1, Role: RoleCapture, Volume: 0.8}}
	devices := cfg.EffectiveDevices()
	require.Len(t, devices, 1)
	assert.Equal(t, uint32(1), devices[0].ID)
}

func TestCaptureDevices_Normal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Devices = []DeviceEntry{
		{ID: 1, Role: RoleCapture, Volume: 1.0},
		{ID: 2, Role: RolePlayback, Volume: 1.0},
		{ID: 3, Volume: 1.0}, // no role
	}
	caps := cfg.CaptureDevices()
	assert.Len(t, caps, 2) // explicit capture + no-role (normal mode)
}

func TestCaptureDevices_Reverse(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Reverse = true
	cfg.Devices = []DeviceEntry{
		{ID: 1, Volume: 1.0}, // no role, reverse → playback
	}
	caps := cfg.CaptureDevices()
	assert.Empty(t, caps)
	plays := cfg.PlaybackDevices()
	assert.Len(t, plays, 1)
}

func TestValidate_DevicesInvalidRole(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Devices = []DeviceEntry{{ID: 1, Role: "invalid", Volume: 1.0}}
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "must be \"capture\" or \"playback\"")
}

func TestValidate_DevicesInvalidVolume(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Devices = []DeviceEntry{{ID: 1, Role: RoleCapture, Volume: 3.0}}
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "must be 0.0-2.0")
}

func TestValidate_DuplexDevicesNeedBothRoles(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeServer
	cfg.Duplex = true
	cfg.Devices = []DeviceEntry{{ID: 1, Role: RoleCapture, Volume: 1.0}}
	errs := cfg.Validate()
	assert.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "playback device")
}

func TestNormalizeDeviceVolumes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Devices = []DeviceEntry{
		{ID: 1, Volume: 0},
		{ID: 2, Volume: -1},
		{ID: 3, Volume: 1.5},
	}
	cfg.NormalizeDeviceVolumes()
	assert.Equal(t, 1.0, cfg.Devices[0].Volume)
	assert.Equal(t, 1.0, cfg.Devices[1].Volume)
	assert.Equal(t, 1.5, cfg.Devices[2].Volume) // valid, unchanged
}

func TestLoadFromFile_NestedDevices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested.yaml")
	content := `
role: server
mode: duplex
audio:
  devices:
    - id: 1
      role: capture
      volume: 1.0
    - id: 3
      role: capture
      volume: 0.5
    - id: 2
      role: playback
      volume: 1.0
  sample_rate: 48000
  channels: 1
`
	err := os.WriteFile(path, []byte(content), 0600)
	require.NoError(t, err)

	cfg, err := LoadFromFile(path)
	require.NoError(t, err)
	require.Len(t, cfg.Devices, 3)
	assert.Equal(t, uint32(1), cfg.Devices[0].ID)
	assert.Equal(t, RoleCapture, cfg.Devices[0].Role)
	assert.Equal(t, 0.5, cfg.Devices[1].Volume)
}
