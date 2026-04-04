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

// ServerPresets stores device presets for server mode (keyed by audio mode).
type ServerPresets struct {
	Presets map[string]recent.DevicePreset `json:"presets" yaml:"presets"`
}

// FilePath returns ~/.config/echowarp/server_presets.yaml.
func FilePath() string {
	return filepath.Join(config.EchoWarpDir(), "server_presets.yaml")
}

// jsonFilePath returns the legacy JSON path.
func jsonFilePath() string {
	return filepath.Join(config.EchoWarpDir(), "server_presets.json")
}

// Load reads server presets from disk. Tries YAML first, then falls back to legacy JSON.
// Returns empty struct on missing/corrupt file.
func Load() ServerPresets {
	empty := ServerPresets{Presets: make(map[string]recent.DevicePreset)}

	// Try .yaml first, fall back to .yml for backward compatibility
	yamlPath := FilePath()
	loadPath := yamlPath
	if _, statErr := os.Stat(yamlPath); os.IsNotExist(statErr) {
		ymlPath := strings.TrimSuffix(yamlPath, ".yaml") + ".yml"
		if _, err2 := os.Stat(ymlPath); err2 == nil {
			loadPath = ymlPath
		}
	}

	// Try YAML first
	data, err := os.ReadFile(loadPath)
	if err != nil {
		// Fall back to legacy JSON
		data, err = os.ReadFile(jsonFilePath())
		if err != nil {
			return empty
		}
		var sp ServerPresets
		if err := json.Unmarshal(data, &sp); err != nil {
			return empty
		}
		if sp.Presets == nil {
			sp.Presets = make(map[string]recent.DevicePreset)
		}
		return sp
	}

	var sp ServerPresets
	if err := yaml.Unmarshal(data, &sp); err != nil {
		return empty
	}
	if sp.Presets == nil {
		sp.Presets = make(map[string]recent.DevicePreset)
	}
	return sp
}

// Get returns the device preset for the given mode, or nil if not found.
func (sp *ServerPresets) Get(mode string) *recent.DevicePreset {
	if sp.Presets == nil {
		return nil
	}
	p, ok := sp.Presets[mode]
	if !ok {
		return nil
	}
	return &p
}

// Set stores a device preset for the given mode.
func (sp *ServerPresets) Set(mode string, p recent.DevicePreset) {
	if sp.Presets == nil {
		sp.Presets = make(map[string]recent.DevicePreset)
	}
	sp.Presets[mode] = p
}

// Save writes server presets to disk in YAML format.
func Save(sp ServerPresets) error {
	dir := filepath.Dir(FilePath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(sp)
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(), data, 0600)
}
