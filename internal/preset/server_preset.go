// Package preset provides persistence for device presets (server mode).
package preset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// ModePreset captures all per-mode settings including device selection.
// Fields matching DefaultsFor(mode) are omitted from YAML via pre-marshal zeroing in Save.
type ModePreset struct {
	Devices    []recent.PresetDevice `json:"devices,omitempty"     yaml:"devices,omitempty"`
	Port       int                   `json:"port,omitempty"        yaml:"port,omitempty"`
	Password   string                `json:"password,omitempty"    yaml:"password,omitempty"`
	MaxClients int                   `json:"max_clients,omitempty" yaml:"max_clients,omitempty"`
	TLS        bool                  `json:"tls,omitempty"         yaml:"tls,omitempty"`
	TLSCert    string                `json:"tls_cert,omitempty"    yaml:"tls_cert,omitempty"`
	TLSKey     string                `json:"tls_key,omitempty"     yaml:"tls_key,omitempty"`
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
	LastMode string                         `yaml:"last_mode,omitempty" json:"last_mode,omitempty"`
	Presets  map[string]recent.DevicePreset `yaml:"presets"             json:"presets"`
	Settings legacyServerSettings           `yaml:"settings,omitempty"  json:"settings,omitempty"`
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
	yamlPath := FilePath()
	loadPath := yamlPath
	if _, statErr := os.Stat(yamlPath); os.IsNotExist(statErr) {
		ymlPath := strings.TrimSuffix(yamlPath, ".yaml") + ".yml"
		if _, err2 := os.Stat(ymlPath); err2 == nil {
			loadPath = ymlPath
		}
	}
	data, err := os.ReadFile(loadPath)
	if err == nil {
		return data, false, nil
	}
	// Fall back to legacy JSON
	data, err = os.ReadFile(jsonFilePath())
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Load reads server presets from disk. Tries YAML first, then falls back to legacy JSON.
// Migrates legacy `settings:` block into per-mode ModePreset fields on the fly.
// Returns empty struct on missing/corrupt file.
func Load() ServerPresets {
	empty := ServerPresets{Presets: make(map[string]ModePreset)}

	data, isJSON, err := readPresetData()
	if err != nil {
		return empty
	}

	// Parse legacy shape first — tolerant of missing fields, catches `settings:` block
	// and `presets.<mode>.devices` entries alike.
	var legacy legacyServerPresetsFile
	if isJSON {
		if err := json.Unmarshal(data, &legacy); err != nil {
			return empty
		}
	} else {
		if err := yaml.Unmarshal(data, &legacy); err != nil {
			return empty
		}
	}

	sp := ServerPresets{
		LastMode: legacy.LastMode, // top-level last_mode (new format)
		Presets:  make(map[string]ModePreset),
	}

	hasLegacySettings := legacy.Settings != (legacyServerSettings{})
	if sp.LastMode == "" && hasLegacySettings {
		sp.LastMode = legacy.Settings.LastMode
	}

	// Seed each mode with devices from legacy shape + migrate settings block into it.
	for mode, legacyPreset := range legacy.Presets {
		mp := ModePreset{Devices: legacyPreset.Devices}
		if hasLegacySettings {
			if mp.Port == 0 {
				mp.Port = legacy.Settings.Port
			}
			if mp.Password == "" {
				mp.Password = legacy.Settings.Password
			}
			if mp.MaxClients == 0 {
				mp.MaxClients = legacy.Settings.MaxClients
			}
			if !mp.TLS && legacy.Settings.TLS {
				mp.TLS = true
			}
			if mp.TLSCert == "" {
				mp.TLSCert = legacy.Settings.TLSCert
			}
			if mp.TLSKey == "" {
				mp.TLSKey = legacy.Settings.TLSKey
			}
		}
		sp.Presets[mode] = mp
	}

	// Also parse new-shape fields (port/password/max_clients/tls*) directly nested under
	// each mode entry. These OVERRIDE legacy-migrated values (the file may already be in
	// the new format, or it may be a hand-edited mix).
	var newShape struct {
		LastMode string                `yaml:"last_mode,omitempty" json:"last_mode,omitempty"`
		Presets  map[string]ModePreset `yaml:"presets"             json:"presets"`
	}
	if isJSON {
		_ = json.Unmarshal(data, &newShape)
	} else {
		_ = yaml.Unmarshal(data, &newShape)
	}
	for mode, newMp := range newShape.Presets {
		existing, ok := sp.Presets[mode]
		if !ok {
			existing = ModePreset{Devices: newMp.Devices}
		}
		if newMp.Port != 0 {
			existing.Port = newMp.Port
		}
		if newMp.Password != "" {
			existing.Password = newMp.Password
		}
		if newMp.MaxClients != 0 {
			existing.MaxClients = newMp.MaxClients
		}
		if newMp.TLS {
			existing.TLS = true
		}
		if newMp.TLSCert != "" {
			existing.TLSCert = newMp.TLSCert
		}
		if newMp.TLSKey != "" {
			existing.TLSKey = newMp.TLSKey
		}
		sp.Presets[mode] = existing
	}

	return sp
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
		// Password/TLSCert/TLSKey default to "" — omitempty handles them.
		// TLS defaults to false — omitempty handles it.
		cleaned.Presets[mode] = mp
	}

	dir := filepath.Dir(FilePath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cleaned)
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(), data, 0600)
}
