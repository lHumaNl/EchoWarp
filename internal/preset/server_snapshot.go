package preset

import (
	"slices"
	"sync"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

const snapshotVersion = 1

// Serializes SaveSnapshot read-modify-write operations within this process only.
var snapshotMu sync.Mutex

// FromConfig captures runtime settings without role, recording, or config device selectors.
// The caller supplies named devices and virtual lifecycle metadata after successful startup.
func FromConfig(cfg config.Config, devices recent.DevicePreset) ModePreset {
	mp := ModePreset{Version: snapshotVersion, Devices: cloneSnapshotDevices(devices.Devices),
		VirtualSinks: slices.Clone(devices.VirtualSinks)}
	snapshotAudio(&mp, cfg)
	snapshotSecurity(&mp, cfg)
	snapshotLogging(&mp, cfg)
	return mp
}

func snapshotAudio(mp *ModePreset, cfg config.Config) {
	mp.ServerMuted, mp.AEC = cfg.ServerMuted, cfg.AEC
	mp.SampleRate, mp.Channels = cfg.SampleRate, cfg.Channels
	mp.OpusBitrate, mp.OpusComplexity = cfg.OpusBitrate, cfg.OpusComplexity
	mp.OpusApplication = cfg.OpusApplication
	mp.OpusDTX, mp.OpusFEC = cfg.OpusDTX, cfg.OpusFEC
	mp.AudioBufferFrames = cfg.AudioBufferFrames
	mp.Loopback, mp.VirtualMic = cfg.Loopback, cfg.VirtualMic
}

func snapshotSecurity(mp *ModePreset, cfg config.Config) {
	mp.Port, mp.MaxClients = cfg.Port, cfg.MaxClients
	mp.Password, mp.PasswordHash = cfg.Password, cfg.PasswordHash
	mp.TLS, mp.TLSCert, mp.TLSKey = cfg.TLS || cfg.IsTLSEnabled(), cfg.TLSCert, cfg.TLSKey
	mp.MaxFailedAttempts, mp.RateLimit = cfg.MaxFailedAttempts, cfg.RateLimit
	mp.HWIDRequired, mp.BanFilePath = cfg.HWIDRequired, cfg.BanFilePath
	mp.TrustedProxies = slices.Clone(cfg.TrustedProxies)
	mp.STUNServers = slices.Clone(cfg.STUNServers)
	mp.TURNServers = slices.Clone(cfg.TURNServers)
}

func snapshotLogging(mp *ModePreset, cfg config.Config) {
	mp.LogLevel, mp.LogFile, mp.LogToFile = cfg.LogLevel, cfg.LogFile, cfg.LogToFile
	mp.NoSIMDOptimization, mp.NoPoolWarmup = cfg.NoSIMDOptimization, cfg.NoPoolWarmup
}

func cloneSnapshotDevices(devices []recent.PresetDevice) []recent.PresetDevice {
	cloned := slices.Clone(devices)
	for i := range cloned {
		cloned[i].VolumeSet = true
		if devices[i].MixInputID != nil {
			id := *devices[i].MixInputID
			cloned[i].MixInputID = &id
		}
		if devices[i].VirtualSink != nil {
			sink := *devices[i].VirtualSink
			cloned[i].VirtualSink = &sink
		}
	}
	return cloned
}

// Apply restores the selected audio mode without changing the caller's server/client role.
// Devices must be resolved separately by name and direction before explicit selectors are overlaid.
// Version zero retains base values for settings that legacy presets did not capture.
func Apply(base config.Config, mode string, mp ModePreset) config.Config {
	base.StreamMode = config.AudioMode(mode)
	base.SyncFromStreamMode()
	base.Devices, base.DeviceID, base.InputDeviceID, base.OutputDeviceID = nil, nil, nil, nil
	applyLegacySnapshot(&base, mode, mp)
	if mp.Version >= snapshotVersion {
		applySnapshotAudio(&base, mp)
		applySnapshotSecurity(&base, mp)
		applySnapshotLogging(&base, mp)
	} else if base.Conference && len(mp.Devices) == 0 {
		// Old device-less conference profiles were hubs, never implicit microphones.
		base.ServerMuted = true
	}
	return base
}

func applyLegacySnapshot(cfg *config.Config, mode string, mp ModePreset) {
	defaults := DefaultsFor(mode)
	cfg.Port, cfg.MaxClients = mp.Port, mp.MaxClients
	if cfg.Port == 0 {
		cfg.Port = defaults.Port
	}
	if cfg.MaxClients == 0 {
		cfg.MaxClients = defaults.MaxClients
	}
	cfg.Password, cfg.TLSCert, cfg.TLSKey = mp.Password, mp.TLSCert, mp.TLSKey
	cfg.TLS = mp.TLS || (mp.Version == 0 && mp.TLSCert != "" && mp.TLSKey != "")
	if mp.LogLevel != "" {
		cfg.LogLevel = mp.LogLevel
	}
}

func applySnapshotAudio(cfg *config.Config, mp ModePreset) {
	cfg.ServerMuted, cfg.AEC = mp.ServerMuted, mp.AEC
	cfg.SampleRate, cfg.Channels = mp.SampleRate, mp.Channels
	cfg.OpusBitrate, cfg.OpusComplexity = mp.OpusBitrate, mp.OpusComplexity
	cfg.OpusApplication = mp.OpusApplication
	cfg.OpusDTX, cfg.OpusFEC = mp.OpusDTX, mp.OpusFEC
	cfg.AudioBufferFrames = mp.AudioBufferFrames
	cfg.Loopback, cfg.VirtualMic = mp.Loopback, mp.VirtualMic
}

func applySnapshotSecurity(cfg *config.Config, mp ModePreset) {
	cfg.PasswordHash = mp.PasswordHash
	cfg.MaxFailedAttempts, cfg.RateLimit = mp.MaxFailedAttempts, mp.RateLimit
	cfg.HWIDRequired, cfg.BanFilePath = mp.HWIDRequired, mp.BanFilePath
	cfg.TrustedProxies = slices.Clone(mp.TrustedProxies)
	cfg.STUNServers = slices.Clone(mp.STUNServers)
	cfg.TURNServers = slices.Clone(mp.TURNServers)
	if !mp.TLS {
		// Config.IsTLSEnabled also checks paths; disabled TLS must not revive them.
		cfg.TLSCert, cfg.TLSKey = "", ""
	}
}

func applySnapshotLogging(cfg *config.Config, mp ModePreset) {
	cfg.LogLevel, cfg.LogFile, cfg.LogToFile = mp.LogLevel, mp.LogFile, mp.LogToFile
	cfg.NoSIMDOptimization, cfg.NoPoolWarmup = mp.NoSIMDOptimization, mp.NoPoolWarmup
}

// SaveSnapshot replaces the current mode's profile and preserves other modes.
// Call only after the listener has successfully started; this is not a cross-process lock.
func SaveSnapshot(cfg config.Config, devices recent.DevicePreset) error {
	return SaveModeSnapshot(cfg.AudioMode(), FromConfig(cfg, devices))
}

// SaveModeSnapshot persists an already collected TUI snapshot with the same
// corruption checks and write serialization as headless SaveSnapshot.
func SaveModeSnapshot(mode string, mp ModePreset) error {
	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	sp, err := loadServerPresets()
	if err != nil {
		return err
	}
	sp.Set(mode, mp)
	sp.LastMode = mode
	return Save(sp)
}
