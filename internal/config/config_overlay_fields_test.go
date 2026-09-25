package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func completeOverlayConfig() Config {
	input, output := uint32(0), uint32(1)
	return Config{Mode: ModeServer, StreamMode: AudioModeDuplex, Duplex: true,
		Port: 5001, Address: "localhost", DeviceID: &input, InputDeviceID: &input, OutputDeviceID: &output,
		Devices:    []DeviceEntry{{ID: 0, Name: "mic", Type: DeviceInput, Role: RoleCapture, AGC: true, Volume: 1}},
		SampleRate: 24000, Channels: 1, VirtualMic: true, Loopback: true, AudioBufferFrames: 3,
		OpusBitrate: 32000, OpusComplexity: 4, OpusApplication: "audio", OpusDTX: true, OpusFEC: true,
		Password: "test-value", PasswordHash: "test-hash", TLSCert: "cert", TLSKey: "key", TLS: true, TLSInsecure: true,
		STUNServers: []string{"stun:example.invalid"}, TURNServers: []TURNServer{{URL: "turn:example.invalid", Username: "user", Credential: "test-value"}},
		MaxClients: 8, MaxReconnectAttempts: 2, ReconnectIntervalSec: 4, AutoReconnect: true, AutoReconnectAttempts: 3,
		MaxFailedAttempts: 2, BanFilePath: "bans.json", LogLevel: "debug", LogToFile: true, LogFile: "session.log",
		TrustedProxies: []string{"localhost"}, AEC: true, ServerMuted: true, RecordMode: "mix", Nickname: "tester",
		HWIDRequired: true, RateLimit: 3, NoSIMDOptimization: true, NoPoolWarmup: true}
}

func TestLoadOverAllFileFields(t *testing.T) {
	isolateOverlayEnv(t)
	expected := completeOverlayConfig()
	for _, value := range []any{expected, expected.toNested()} {
		data, err := yaml.Marshal(value)
		require.NoError(t, err)
		cfg, presence, err := LoadOver(Config{Mode: ModeServer}, overlayFile(t, string(data)))
		require.NoError(t, err)
		assert.Equal(t, expected, cfg)
		assert.Equal(t, OverridePresence{StreamMode: true, TLS: true, Devices: true, DeviceID: true, InputDeviceID: true, OutputDeviceID: true}, presence)
	}
}

func TestLoadOverJSON(t *testing.T) {
	isolateOverlayEnv(t)
	expected := completeOverlayConfig()
	data, err := json.Marshal(expected.toNested())
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	cfg, _, err := LoadOver(Config{Mode: ModeServer}, path)
	require.NoError(t, err)
	assert.Equal(t, expected, cfg)
}

func TestLoadOverAllEnvironmentFields(t *testing.T) {
	isolateOverlayEnv(t)
	expected := completeOverlayConfig()
	data, err := yaml.Marshal(expected)
	require.NoError(t, err)
	var fields map[string]yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &fields))
	for key, node := range fields {
		setOverlayNodeEnv(t, key, &node)
	}
	cfg, _, err := LoadOver(Config{Mode: ModeServer}, "")
	require.NoError(t, err)
	assert.Equal(t, expected, cfg)
}

func setOverlayNodeEnv(t *testing.T, key string, node *yaml.Node) {
	t.Helper()
	value := node.Value
	if node.Kind != yaml.ScalarNode {
		node.Style = yaml.FlowStyle
		data, err := yaml.Marshal(node)
		require.NoError(t, err)
		value = string(data)
	}
	t.Setenv("ECHOWARP_"+strings.ToUpper(key), value)
}

func TestLoadOverEnvironmentAliases(t *testing.T) {
	isolateOverlayEnv(t)
	for _, key := range []string{"ECHOWARP_DEVICES", "ECHOWARP_AUDIO_DEVICES"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, `[{id: 0, role: capture, agc: true, volume: 0, mix_input_id: 2, mix_input_name: local}]`)
			cfg, presence, err := LoadOver(Config{}, "")
			require.NoError(t, err)
			require.Len(t, cfg.Devices, 1)
			assert.True(t, cfg.Devices[0].AGC)
			assert.Zero(t, cfg.Devices[0].Volume)
			assert.Equal(t, uint32(2), *cfg.Devices[0].MixInputID)
			assert.True(t, presence.Devices)
		})
	}
}

func TestLoadOverUnmappedFieldsAndEmptyMode(t *testing.T) {
	isolateOverlayEnv(t)
	t.Setenv("ECHOWARP_MODE", "")
	t.Setenv("ECHOWARP_RECORD_DIR", "recordings")
	cfg, presence, err := LoadOver(Config{Mode: ModeServer, StreamMode: AudioModeDuplex, Duplex: true}, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Mode: ModeServer, RecordDir: "recordings"}, cfg)
	assert.Equal(t, OverridePresence{StreamMode: true}, presence)
}

func TestLoadOverNestedFilePriority(t *testing.T) {
	isolateOverlayEnv(t)
	cfg, _, err := LoadOver(Config{}, overlayFile(t, "port: 1\nnetwork: {port: 2}\n'': ignored"))
	require.NoError(t, err)
	assert.Equal(t, Config{Port: 2}, cfg)
}

func TestLoadOverDoesNotDiscoverBaseFile(t *testing.T) {
	isolateOverlayEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(EchoWarpDir(), "config.yaml"), []byte("port: 1234"), 0600))
	cfg, presence, err := LoadOver(Config{Mode: ModeServer}, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Mode: ModeServer}, cfg)
	assert.Equal(t, OverridePresence{}, presence)
}

func TestLoadOverFileSizeBoundary(t *testing.T) {
	isolateOverlayEnv(t)
	path := overlayFile(t, "#"+strings.Repeat("x", (1<<20)-1))
	_, _, err := LoadOver(Config{}, path)
	require.NoError(t, err)
}

func TestLoadOverEmptyScalarErrors(t *testing.T) {
	isolateOverlayEnv(t)
	for _, key := range []string{"ECHOWARP_PORT", "ECHOWARP_TLS", "ECHOWARP_DEVICE_ID"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "")
			_, _, err := LoadOver(Config{}, "")
			require.Error(t, err)
		})
	}
}

func TestLoadOverLegacyStringList(t *testing.T) {
	isolateOverlayEnv(t)
	cfg, _, err := LoadOver(Config{}, overlayFile(t, "network: {stun_servers: 'stun:one, stun:two'}"))
	require.NoError(t, err)
	assert.Equal(t, []string{"stun:one", "stun:two"}, cfg.STUNServers)
}
