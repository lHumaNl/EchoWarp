// Package preset provides persistence for device presets (server mode).
package preset

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// ModePreset captures all per-mode settings including device selection.
// Fields matching DefaultsFor(mode) are omitted from YAML via pre-marshal zeroing in Save.
// Version 1 makes omitted zero/false/empty fields authoritative, unlike legacy snapshots.
type ModePreset struct {
	Version            int                        `json:"version,omitempty" yaml:"version,omitempty"`
	Devices            []recent.PresetDevice      `json:"devices,omitempty"     yaml:"devices,omitempty"`
	VirtualSinks       []recent.VirtualSinkPreset `json:"virtual_sinks,omitempty" yaml:"virtual_sinks,omitempty"`
	Port               int                        `json:"port,omitempty"        yaml:"port,omitempty"`
	Password           string                     `json:"password,omitempty"    yaml:"password,omitempty"`
	MaxClients         int                        `json:"max_clients,omitempty" yaml:"max_clients,omitempty"`
	TLS                bool                       `json:"tls,omitempty"         yaml:"tls,omitempty"`
	TLSCert            string                     `json:"tls_cert,omitempty"    yaml:"tls_cert,omitempty"`
	TLSKey             string                     `json:"tls_key,omitempty"     yaml:"tls_key,omitempty"`
	LogLevel           string                     `json:"log_level,omitempty"   yaml:"log_level,omitempty"`
	ServerMuted        bool                       `json:"server_muted,omitempty" yaml:"server_muted,omitempty"`
	AEC                bool                       `json:"aec,omitempty" yaml:"aec,omitempty"`
	SampleRate         uint32                     `json:"sample_rate,omitempty" yaml:"sample_rate,omitempty"`
	Channels           uint32                     `json:"channels,omitempty" yaml:"channels,omitempty"`
	OpusBitrate        int                        `json:"opus_bitrate,omitempty" yaml:"opus_bitrate,omitempty"`
	OpusComplexity     int                        `json:"opus_complexity,omitempty" yaml:"opus_complexity,omitempty"`
	OpusApplication    string                     `json:"opus_application,omitempty" yaml:"opus_application,omitempty"`
	OpusDTX            bool                       `json:"opus_dtx,omitempty" yaml:"opus_dtx,omitempty"`
	OpusFEC            bool                       `json:"opus_fec,omitempty" yaml:"opus_fec,omitempty"`
	AudioBufferFrames  int                        `json:"audio_buffer_frames,omitempty" yaml:"audio_buffer_frames,omitempty"`
	Loopback           bool                       `json:"loopback,omitempty" yaml:"loopback,omitempty"`
	VirtualMic         bool                       `json:"virtual_mic,omitempty" yaml:"virtual_mic,omitempty"`
	MaxFailedAttempts  int                        `json:"max_failed_attempts,omitempty" yaml:"max_failed_attempts,omitempty"`
	HWIDRequired       bool                       `json:"hwid_required,omitempty" yaml:"hwid_required,omitempty"`
	BanFilePath        string                     `json:"ban_file_path,omitempty" yaml:"ban_file_path,omitempty"`
	TrustedProxies     []string                   `json:"trusted_proxies,omitempty" yaml:"trusted_proxies,omitempty"`
	RateLimit          int                        `json:"rate_limit,omitempty" yaml:"rate_limit,omitempty"`
	STUNServers        []string                   `json:"stun_servers,omitempty" yaml:"stun_servers,omitempty"`
	TURNServers        []config.TURNServer        `json:"turn_servers,omitempty" yaml:"turn_servers,omitempty"`
	PasswordHash       string                     `json:"password_hash,omitempty" yaml:"password_hash,omitempty"`
	LogFile            string                     `json:"log_file,omitempty" yaml:"log_file,omitempty"`
	LogToFile          bool                       `json:"log_to_file,omitempty" yaml:"log_to_file,omitempty"`
	NoSIMDOptimization bool                       `json:"no_simd_optimization,omitempty" yaml:"no_simd_optimization,omitempty"`
	NoPoolWarmup       bool                       `json:"no_pool_warmup,omitempty" yaml:"no_pool_warmup,omitempty"`
}

// ServerPresets is the top-level preset container. Each mode is a self-contained snapshot.
type ServerPresets struct {
	LastMode string                `json:"last_mode,omitempty" yaml:"last_mode,omitempty"`
	Presets  map[string]ModePreset `json:"presets"             yaml:"presets"`
}

// DefaultsFor returns the default ModePreset for the given mode. Fields in a stored
// ModePreset that match these defaults are omitted from YAML on Save.
func DefaultsFor(mode string) ModePreset {
	d := ModePreset{
		Port:       4415,
		MaxClients: 1,
	}
	if mode == "conference" {
		d.MaxClients = 2
	}
	return d
}

// legacyServerSettings is the old global-settings block. Only used during migration
// in Load(). Not emitted on save.
type legacyServerSettings struct {
	LastMode   string `yaml:"last_mode,omitempty"   json:"last_mode,omitempty"`
	Port       int    `yaml:"port,omitempty"        json:"port,omitempty"`
	Password   string `yaml:"password,omitempty"    json:"password,omitempty"`
	TLS        bool   `yaml:"tls,omitempty"         json:"tls,omitempty"`
	TLSCert    string `yaml:"tls_cert,omitempty"    json:"tls_cert,omitempty"`
	TLSKey     string `yaml:"tls_key,omitempty"     json:"tls_key,omitempty"`
	MaxClients int    `yaml:"max_clients,omitempty" json:"max_clients,omitempty"`
}

// legacyServerPresetsFile is the old on-disk shape used only for migration parsing.
type legacyServerPresetsFile struct {
	LastMode string               `yaml:"last_mode,omitempty" json:"last_mode,omitempty"`
	Presets  map[string]yaml.Node `yaml:"presets"             json:"presets"`
	Settings legacyServerSettings `yaml:"settings,omitempty"  json:"settings,omitempty"`
}

// FilePath returns ~/.config/echowarp/server_presets.yaml.
func FilePath() string {
	return filepath.Join(config.EchoWarpDir(), "server_presets.yaml")
}

// jsonFilePath returns the legacy JSON path.
func jsonFilePath() string {
	return filepath.Join(config.EchoWarpDir(), "server_presets.json")
}

// readPresetData reads the preset file. Tries .yaml, then .yml, then legacy JSON.
// Returns (data, isJSON, err). isJSON indicates the legacy JSON path was used.
func readPresetData() ([]byte, bool, error) {
	paths := []string{FilePath(), strings.TrimSuffix(FilePath(), ".yaml") + ".yml", jsonFilePath()}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if !os.IsNotExist(err) {
			return data, path == jsonFilePath(), err
		}
	}
	return nil, false, os.ErrNotExist
}

// Load reads server presets from disk. Tries YAML first, then falls back to legacy JSON.
// Migrates legacy `settings:` block into per-mode ModePreset fields on the fly.
// Returns empty struct on missing/corrupt file.
func Load() ServerPresets {
	sp, err := loadServerPresets()
	if err != nil {
		return ServerPresets{Presets: make(map[string]ModePreset)}
	}
	return sp
}

func loadServerPresets() (ServerPresets, error) {
	data, isJSON, err := readPresetData()
	if os.IsNotExist(err) {
		return ServerPresets{Presets: make(map[string]ModePreset)}, nil
	}
	if err != nil {
		return ServerPresets{}, errors.New("cannot read server presets")
	}
	if isJSON && !json.Valid(data) {
		return ServerPresets{}, errors.New("invalid server presets")
	}
	return decodeServerPresets(data)
}

func decodeServerPresets(data []byte) (ServerPresets, error) {
	var legacy legacyServerPresetsFile
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return ServerPresets{}, errors.New("invalid server presets")
	}
	sp := ServerPresets{LastMode: legacy.LastMode, Presets: make(map[string]ModePreset)}
	if sp.LastMode == "" {
		sp.LastMode = legacy.Settings.LastMode
	}
	for mode, node := range legacy.Presets {
		mp := legacy.Settings.modePreset()
		// Decode onto the seed: absent fields inherit, explicit zero values replace it.
		if err := node.Decode(&mp); err != nil {
			return ServerPresets{}, errors.New("invalid server mode preset")
		}
		// An explicit legacy tls:false overrides inherited certificate paths.
		// Omitted tls still allows old cert/key-only profiles to imply TLS.
		if mp.Version == 0 && !mp.TLS {
			for i := 0; i+1 < len(node.Content); i += 2 {
				if node.Content[i].Value == "tls" {
					mp.TLSCert, mp.TLSKey = "", ""
					break
				}
			}
		}
		normalizeSnapshotVolumes(&mp, node)
		sp.Presets[mode] = mp
	}
	return sp, nil
}

func normalizeSnapshotVolumes(mp *ModePreset, node yaml.Node) {
	var devices *yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "devices" {
			devices = node.Content[i+1]
			break
		}
	}
	for i := range mp.Devices {
		d := &mp.Devices[i]
		if d.Volume != 0 || d.VolumeSet {
			continue
		}
		if devices != nil && i < len(devices.Content) {
			fields := devices.Content[i].Content
			for j := 0; j+1 < len(fields); j += 2 {
				if fields[j].Value == "volume" {
					d.VolumeSet = true
					break
				}
			}
		}
		if !d.VolumeSet {
			d.Volume = 1
		}
	}
}

func (s legacyServerSettings) modePreset() ModePreset {
	return ModePreset{Port: s.Port, Password: s.Password, MaxClients: s.MaxClients,
		TLS: s.TLS, TLSCert: s.TLSCert, TLSKey: s.TLSKey}
}

// Get returns the preset for the given mode, or nil if not found.
func (sp *ServerPresets) Get(mode string) *ModePreset {
	if sp.Presets == nil {
		return nil
	}
	p, ok := sp.Presets[mode]
	if !ok {
		return nil
	}
	return &p
}

// Set stores a preset for the given mode.
func (sp *ServerPresets) Set(mode string, p ModePreset) {
	if sp.Presets == nil {
		sp.Presets = make(map[string]ModePreset)
	}
	sp.Presets[mode] = p
}

// Save writes server presets to disk in YAML format. Fields matching DefaultsFor(mode)
// are zeroed pre-marshal so `omitempty` drops them, keeping the file minimal.
func Save(sp ServerPresets) error {
	data, err := yaml.Marshal(cleanServerPresets(sp))
	if err != nil {
		return errors.New("cannot encode server presets")
	}
	if err := writeServerPresets(data); err != nil {
		return errors.New("cannot write server presets")
	}
	return nil
}

func cleanServerPresets(sp ServerPresets) ServerPresets {
	cleaned := ServerPresets{
		LastMode: sp.LastMode,
		Presets:  make(map[string]ModePreset, len(sp.Presets)),
	}
	for mode, mp := range sp.Presets {
		d := DefaultsFor(mode)
		if mp.Port == d.Port {
			mp.Port = 0
		}
		if mp.MaxClients == d.MaxClients {
			mp.MaxClients = 0
		}
		cleaned.Presets[mode] = mp
	}
	return cleaned
}
