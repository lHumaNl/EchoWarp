package preset

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func snapshotConfig() config.Config {
	return config.Config{
		Mode: config.ModeClient, StreamMode: config.AudioModeConference, Conference: true,
		Port: 5020, MaxClients: 8, ServerMuted: true, AEC: true,
		SampleRate: 24000, Channels: 1, OpusBitrate: 32000, OpusComplexity: 7,
		OpusApplication: "audio", OpusDTX: true, OpusFEC: true, AudioBufferFrames: 9,
		Loopback: true, VirtualMic: true, MaxFailedAttempts: 12, RateLimit: 6,
		Password: "test-only-password", PasswordHash: "test-only-hash",
		TLS: true, TLSCert: "test.crt", TLSKey: "test.key",
		STUNServers: []string{"stun:example.invalid"},
		TURNServers: []config.TURNServer{{URL: "turn:example.invalid", Username: "test", Credential: "test-only"}},
		LogLevel:    "debug", LogFile: "test.log", LogToFile: true,
		NoSIMDOptimization: true, NoPoolWarmup: true,
	}
}

func TestSnapshotFullRoundtrip(t *testing.T) {
	for _, muted := range []bool{false, true} {
		t.Run(map[bool]string{false: "participant", true: "hub"}[muted], func(t *testing.T) {
			t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
			cfg := snapshotConfig()
			cfg.ServerMuted = muted
			require.NoError(t, SaveSnapshot(cfg, recent.DevicePreset{}))
			sp := Load()
			require.Equal(t, "conference", sp.LastMode)
			require.Equal(t, 1, sp.Presets[sp.LastMode].Version)
			base := config.Config{Mode: config.ModeClient}
			assert.Equal(t, cfg, Apply(base, sp.LastMode, sp.Presets[sp.LastMode]))
		})
	}
}

func TestSnapshotZeroValuesOverrideBase(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := config.Config{Mode: config.ModeClient, Port: 5021, MaxClients: 3}
	require.NoError(t, SaveSnapshot(cfg, recent.DevicePreset{}))
	got := Apply(snapshotConfig(), "normal", Load().Presets["normal"])
	cfg.StreamMode = config.AudioModeNormal
	assert.Equal(t, cfg, got)
}

func TestSnapshotClearsUncheckedDeviceSelection(t *testing.T) {
	id := uint32(42)
	base := config.DefaultConfig()
	base.Devices = []config.DeviceEntry{{ID: id}}
	base.DeviceID, base.InputDeviceID, base.OutputDeviceID = &id, &id, &id
	for _, version := range []int{0, 1} {
		mp := ModePreset{Version: version, Devices: []recent.PresetDevice{{ID: id, Name: "Mic"}}}
		got := Apply(base, "duplex", mp)
		assert.Empty(t, got.EffectiveDevices())
		assert.Nil(t, got.DeviceID)
		assert.Nil(t, got.InputDeviceID)
		assert.Nil(t, got.OutputDeviceID)
	}
	assert.Len(t, base.Devices, 1)
	assert.Same(t, &id, base.DeviceID)
}

func TestSnapshotSelectedModeAndDefaults(t *testing.T) {
	for _, mode := range []string{"normal", "reverse", "duplex", "conference"} {
		base := snapshotConfig()
		got := Apply(base, mode, ModePreset{})
		assert.Equal(t, config.AudioMode(mode), got.GetAudioMode())
		assert.Equal(t, mode == "reverse", got.Reverse)
		assert.Equal(t, mode == "duplex", got.Duplex)
		assert.Equal(t, mode == "conference", got.Conference)
		assert.Equal(t, DefaultsFor(mode).Port, got.Port)
		assert.Equal(t, DefaultsFor(mode).MaxClients, got.MaxClients)
		assert.Equal(t, base.Mode, got.Mode)
		assert.Equal(t, base.SampleRate, got.SampleRate)
		assert.Equal(t, base.OpusComplexity, got.OpusComplexity)
	}
}

func TestSnapshotLegacyHubAndExplicitParticipant(t *testing.T) {
	base := config.DefaultConfig()
	assert.True(t, Apply(base, "conference", ModePreset{}).ServerMuted)
	assert.False(t, Apply(base, "conference", ModePreset{Version: 1}).ServerMuted)
	base.ServerMuted = true
	assert.False(t, Apply(base, "conference", ModePreset{Version: 1}).ServerMuted)
}

func TestSnapshotTLSCompatibility(t *testing.T) {
	base := snapshotConfig()
	legacy := ModePreset{TLSCert: "legacy.crt", TLSKey: "legacy.key"}
	assert.True(t, Apply(base, "normal", legacy).TLS)
	legacy.Version = 1
	got := Apply(base, "normal", legacy)
	assert.False(t, got.TLS)
	assert.Empty(t, got.TLSCert)
	assert.Empty(t, got.TLSKey)
	legacy.TLS = true
	legacy.TLSKey = ""
	got = Apply(base, "normal", legacy)
	assert.True(t, got.TLS, "incomplete TLS must remain enabled for validation")
	assert.Equal(t, "legacy.crt", got.TLSCert)
	assert.Empty(t, got.TLSKey)
}

func snapshotDevices() recent.DevicePreset {
	id := uint32(7)
	sink := recent.VirtualSinkPreset{ID: "custom", SinkName: "CustomSink", OnStart: recent.SinkKeep, OnStop: recent.SinkDelete}
	return recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 4, Name: "Mic", IsInput: true, AGC: true,
			MixInputID: &id, MixInputName: "Mix", Virtual: true, VirtualSink: &sink}},
		VirtualSinks: []recent.VirtualSinkPreset{sink},
	}
}

func TestSnapshotCopiesDevicesAndMarksSilence(t *testing.T) {
	devices := snapshotDevices()
	mp := FromConfig(config.DefaultConfig(), devices)
	assert.True(t, mp.Devices[0].VolumeSet)
	assert.Zero(t, mp.Devices[0].Volume)
	assert.False(t, devices.Devices[0].VolumeSet)
	assert.Equal(t, devices.VirtualSinks, mp.VirtualSinks)
	assert.Equal(t, devices.Devices[0].VirtualSink, mp.Devices[0].VirtualSink)
	*mp.Devices[0].MixInputID = 99
	mp.Devices[0].VirtualSink.SinkName = "changed"
	mp.VirtualSinks[0].OnStart = recent.SinkRecreate
	assert.Equal(t, snapshotDevices(), devices)
}

func TestSnapshotCopiesNetworkSlices(t *testing.T) {
	cfg := snapshotConfig()
	mp := FromConfig(cfg, recent.DevicePreset{})
	cfg.STUNServers[0] = "changed"
	cfg.TURNServers[0].Credential = "changed"
	assert.Equal(t, snapshotConfig().STUNServers, mp.STUNServers)
	assert.Equal(t, snapshotConfig().TURNServers, mp.TURNServers)
	got := Apply(config.DefaultConfig(), "normal", mp)
	got.STUNServers[0] = "changed-again"
	got.TURNServers[0].Username = "changed-again"
	assert.Equal(t, snapshotConfig().STUNServers, mp.STUNServers)
	assert.Equal(t, snapshotConfig().TURNServers, mp.TURNServers)
}

func TestSnapshotDoesNotPersistRecordingOrRole(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := snapshotConfig()
	cfg.RecordMode, cfg.RecordDir = "both", "recordings"
	require.NoError(t, SaveSnapshot(cfg, recent.DevicePreset{}))
	data, err := os.ReadFile(FilePath())
	require.NoError(t, err)
	assert.NotContains(t, string(data), "record_")
	assert.NotContains(t, string(data), "role:")
	base := config.Config{Mode: config.ModeServer, RecordMode: "mix", RecordDir: "current"}
	got := Apply(base, "conference", Load().Presets["conference"])
	assert.Equal(t, base.Mode, got.Mode)
	assert.Equal(t, base.RecordMode, got.RecordMode)
	assert.Equal(t, base.RecordDir, got.RecordDir)
}

func TestSnapshotSaveMergesOtherModes(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	old := ModePreset{Devices: []recent.PresetDevice{{Name: "Old", Volume: 1}}, Port: 5050}
	require.NoError(t, Save(ServerPresets{Presets: map[string]ModePreset{"reverse": old}}))
	require.NoError(t, SaveSnapshot(snapshotConfig(), snapshotDevices()))
	sp := Load()
	assert.Equal(t, "conference", sp.LastMode)
	assert.Equal(t, old, sp.Presets["reverse"])
	assert.Equal(t, FromConfig(snapshotConfig(), snapshotDevices()), sp.Presets["conference"])
}

func TestSnapshotConcurrentSaveMergesModes(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	var workers sync.WaitGroup
	for _, mode := range []config.AudioMode{config.AudioModeNormal, config.AudioModeReverse, config.AudioModeDuplex, config.AudioModeConference} {
		workers.Go(func() {
			cfg := snapshotConfig()
			cfg.StreamMode = mode
			assert.NoError(t, SaveSnapshot(cfg, recent.DevicePreset{}))
		})
	}
	workers.Wait()
	assert.Len(t, Load().Presets, 4)
}

func TestSnapshotTightensPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX owner-only permissions are not available on Windows")
	}
	t.Setenv("ECHOWARP_CONFIG_DIR", filepath.Join(t.TempDir(), "presets"))
	require.NoError(t, SaveSnapshot(snapshotConfig(), recent.DevicePreset{}))
	require.NoError(t, os.Chmod(FilePath(), 0644))
	require.NoError(t, os.Chmod(filepath.Dir(FilePath()), 0755))
	require.NoError(t, SaveSnapshot(snapshotConfig(), recent.DevicePreset{}))
	file, err := os.Stat(FilePath())
	require.NoError(t, err)
	dir, err := os.Stat(filepath.Dir(FilePath()))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), file.Mode().Perm())
	assert.Equal(t, os.FileMode(0700), dir.Mode().Perm())
}

func TestSnapshotDefaultOmissionDoesNotMutateCaller(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	for _, mode := range []string{"normal", "conference"} {
		mp := FromConfig(config.DefaultConfig(), snapshotDevices())
		mp.MaxClients = DefaultsFor(mode).MaxClients
		sp := ServerPresets{Presets: map[string]ModePreset{mode: mp}}
		require.NoError(t, Save(sp))
		assert.Equal(t, mp, sp.Presets[mode])
		got := Apply(config.Config{}, mode, Load().Presets[mode])
		assert.Equal(t, mp.Port, got.Port)
		assert.Equal(t, mp.MaxClients, got.MaxClients)
	}
}

func TestSnapshotEmptyNetworkListsClearBase(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := snapshotConfig()
	cfg.STUNServers, cfg.TURNServers = []string{}, []config.TURNServer{}
	require.NoError(t, SaveSnapshot(cfg, recent.DevicePreset{}))
	got := Apply(snapshotConfig(), "conference", Load().Presets["conference"])
	assert.Empty(t, got.STUNServers)
	assert.Empty(t, got.TURNServers)
}
