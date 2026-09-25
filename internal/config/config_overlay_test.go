package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func overlayFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overlay.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func isolateOverlayEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "ECHOWARP_") {
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
	}
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
}

func TestLoadOverSparseBase(t *testing.T) {
	isolateOverlayEnv(t)
	base := Config{Mode: ModeServer, Port: 1234, Password: "inherited", TLS: true, Reverse: true}
	cfg, presence, err := LoadOver(base, overlayFile(t, "logging: {level: debug}"))
	require.NoError(t, err)
	base.LogLevel = "debug"
	assert.Equal(t, base, cfg)
	assert.Equal(t, OverridePresence{}, presence)
}

func TestLoadOverExplicitEmptyFileValues(t *testing.T) {
	isolateOverlayEnv(t)
	base := Config{Password: "inherited", TLSCert: "cert", TLSKey: "key", TLS: true,
		ServerMuted: true, MaxClients: 8, RateLimit: 4, OpusComplexity: 5, Devices: []DeviceEntry{{ID: 1}}}
	path := overlayFile(t, "security: {password: '', tls: {cert: '', key: '', enabled: false}, rate_limit: 0}\n"+
		"audio: {devices: []}\nconnection: {max_clients: 0}\nopus: {complexity: 0}\nserver_muted: false")
	cfg, presence, err := LoadOver(base, path)
	require.NoError(t, err)
	assert.Equal(t, Config{Devices: []DeviceEntry{}}, cfg)
	assert.Equal(t, OverridePresence{TLS: true, Devices: true}, presence)
}

func TestLoadOverEnvironmentPriority(t *testing.T) {
	isolateOverlayEnv(t)
	t.Setenv("ECHOWARP_NETWORK_PORT", "6001")
	t.Setenv("ECHOWARP_PORT", "6002")
	t.Setenv("ECHOWARP_PASSWORD", "")
	t.Setenv("ECHOWARP_SECURITY_TLS_CERT", "")
	t.Setenv("ECHOWARP_TLS", "false")
	t.Setenv("ECHOWARP_SERVER_MUTED", "false")
	t.Setenv("ECHOWARP_MAX_CLIENTS", "0")
	base := Config{Password: "inherited", TLSCert: "cert", TLS: true, ServerMuted: true, MaxClients: 8}
	cfg, presence, err := LoadOver(base, overlayFile(t, "network: {port: 6000}\nsecurity: {password: file}"))
	require.NoError(t, err)
	assert.Equal(t, Config{Port: 6002}, cfg)
	assert.Equal(t, OverridePresence{TLS: true}, presence)
}

func TestLoadOverNestedEnvironmentBeatsFlatFile(t *testing.T) {
	isolateOverlayEnv(t)
	t.Setenv("ECHOWARP_NETWORK_PORT", "6001")
	t.Setenv("ECHOWARP_CONNECTION_AUTO_RECONNECT_ATTEMPTS", "0")
	cfg, _, err := LoadOver(Config{AutoReconnectAttempts: 5}, overlayFile(t, "port: 6000"))
	require.NoError(t, err)
	assert.Equal(t, Config{Port: 6001}, cfg)
}

func TestLoadOverDeviceMetadata(t *testing.T) {
	isolateOverlayEnv(t)
	for _, prefix := range []string{"", "audio: "} {
		cfg, presence, err := LoadOver(Config{}, overlayFile(t, prefix+"{devices: ["+
			"{id: 0, name: mic, type: input, role: capture, volume: 0, muted: false, agc: true, mix_input_id: 0, mix_input_name: local},"+
			"{id: 1, role: playback}]}"))
		require.NoError(t, err)
		require.Len(t, cfg.Devices, 2)
		zero := uint32(0)
		assert.Equal(t, DeviceEntry{ID: 0, Name: "mic", Type: DeviceInput, Role: RoleCapture,
			AGC: true, MixInputID: &zero, MixInputName: "local"}, cfg.Devices[0])
		assert.Equal(t, 1.0, cfg.Devices[1].Volume)
		assert.True(t, presence.Devices)
	}
}

func TestLoadOverModeAndLegacyDevices(t *testing.T) {
	isolateOverlayEnv(t)
	t.Setenv("ECHOWARP_ROLE", "client")
	t.Setenv("ECHOWARP_MODE", "duplex")
	t.Setenv("ECHOWARP_INPUT_DEVICE_ID", "0")
	t.Setenv("ECHOWARP_AUDIO_OUTPUT_DEVICE_ID", "1")
	cfg, presence, err := LoadOver(Config{Mode: ModeServer}, overlayFile(t, "role: client\nmode: reverse\ndevice_id: 2"))
	require.NoError(t, err)
	assert.Equal(t, ModeServer, cfg.Mode)
	assert.True(t, cfg.Duplex)
	assert.False(t, cfg.Reverse)
	assert.Equal(t, OverridePresence{StreamMode: true, DeviceID: true, InputDeviceID: true, OutputDeviceID: true}, presence)
	assert.Equal(t, []DeviceEntry{{ID: 0, Role: RoleCapture, Volume: 1}, {ID: 1, Role: RolePlayback, Volume: 1}}, cfg.EffectiveDevices())
}

func TestLoadOverEmptyEnvironmentLists(t *testing.T) {
	isolateOverlayEnv(t)
	for _, value := range []string{"", "[]"} {
		t.Setenv("ECHOWARP_DEVICES", value)
		t.Setenv("ECHOWARP_STUN_SERVERS", value)
		cfg, presence, err := LoadOver(Config{Devices: []DeviceEntry{{ID: 1}}, STUNServers: []string{"stun:base"}}, "")
		require.NoError(t, err)
		assert.Empty(t, cfg.Devices)
		assert.Empty(t, cfg.STUNServers)
		assert.True(t, presence.Devices)
	}
}

func TestLoadOverDoesNotShareBaseDevices(t *testing.T) {
	isolateOverlayEnv(t)
	id := uint32(7)
	base := Config{DeviceID: &id, Devices: []DeviceEntry{{ID: id, MixInputID: &id}}, STUNServers: []string{"stun:base"}}
	cfg, _, err := LoadOver(base, "")
	require.NoError(t, err)
	cfg.Devices[0].ID = 8
	*cfg.Devices[0].MixInputID = 9
	*cfg.DeviceID = 10
	cfg.STUNServers[0] = "stun:changed"
	assert.Equal(t, uint32(7), base.Devices[0].ID)
	assert.Equal(t, uint32(7), id)
	assert.Equal(t, []string{"stun:base"}, base.STUNServers)
}

func TestLoadOverRedactedInvalidValues(t *testing.T) {
	isolateOverlayEnv(t)
	for _, content := range []string{"password: [private-marker", "port: private-marker", "tls: private-marker",
		"audio: {device_id: -1}", "channels: 1.5", "port: 1\nport: 2", "audio: {devices: [{id: private-marker}]}",
		"port: 1\n---\npassword: private-marker", "network: private-marker"} {
		_, _, err := LoadOver(Config{}, overlayFile(t, content))
		require.Error(t, err, "expected malformed input rejection")
		assert.NotContains(t, err.Error(), "private-marker")
	}
}

func TestLoadOverRedactedInvalidEnvironment(t *testing.T) {
	isolateOverlayEnv(t)
	for _, key := range []string{"ECHOWARP_PORT", "ECHOWARP_TLS", "ECHOWARP_DEVICE_ID", "ECHOWARP_DEVICES"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "private-marker")
			_, _, err := LoadOver(Config{}, "")
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "private-marker")
		})
	}
}

func TestLoadOverFileSizeLimit(t *testing.T) {
	isolateOverlayEnv(t)
	path := overlayFile(t, "#"+strings.Repeat("x", 1<<20))
	_, _, err := LoadOver(Config{}, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
	_, err = LoadWithViper(path, ModeServer)
	require.Error(t, err)
}

func TestLoadWithViperClientDefaultsAndClearing(t *testing.T) {
	isolateOverlayEnv(t)
	cfg, err := LoadWithViper("", ModeClient)
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.AutoReconnectAttempts)
	t.Setenv("ECHOWARP_AUTO_RECONNECT_ATTEMPTS", "0")
	t.Setenv("ECHOWARP_PASSWORD", "")
	cfg, err = LoadWithViper(overlayFile(t, "security: {password: inherited}"), ModeClient)
	require.NoError(t, err)
	assert.Zero(t, cfg.AutoReconnectAttempts)
	assert.Empty(t, cfg.Password)
}
