// Package recent persists the most recently connected servers to disk.
package recent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/internal/config"
)

// MaxEntries is the maximum number of recent servers to keep.
const MaxEntries = 10

// DevicePreset stores the device selection for a specific audio mode.
type DevicePreset struct {
	Devices []PresetDevice `json:"devices" yaml:"devices"`
}

// SinkLifecycle controls virtual sink behavior on start/stop.
type SinkLifecycle string

const (
	// SinkKeep leaves the virtual sink as-is.
	SinkKeep SinkLifecycle = "keep"
	// SinkDelete removes the virtual sink.
	SinkDelete SinkLifecycle = "delete"
	// SinkRecreate removes and re-creates the virtual sink.
	SinkRecreate SinkLifecycle = "recreate"
)

// VirtualSinkPreset stores parameters for a PulseAudio virtual sink
// that was created alongside a device preset.
type VirtualSinkPreset struct {
	ModuleType string        `json:"module_type" yaml:"module_type"`
	SinkName   string        `json:"sink_name" yaml:"sink_name"`
	OnStop     SinkLifecycle `json:"on_stop" yaml:"on_stop"`
	OnStart    SinkLifecycle `json:"on_start" yaml:"on_start"`
}

// PresetDevice represents a single device in a preset.
type PresetDevice struct {
	ID           uint32             `json:"id" yaml:"id"`
	Name         string             `json:"name" yaml:"name"`
	IsInput      bool               `json:"is_input" yaml:"is_input"`
	Virtual      bool               `json:"virtual" yaml:"virtual"`
	MixInputID   *uint32            `json:"mix_input_id,omitempty" yaml:"mix_input_id,omitempty"`
	MixInputName string             `json:"mix_input_name,omitempty" yaml:"mix_input_name,omitempty"`
	VirtualSink  *VirtualSinkPreset `json:"virtual_sink,omitempty" yaml:"virtual_sink,omitempty"`
	Volume       float64            `json:"volume,omitempty" yaml:"volume,omitempty"`
	AGC          bool               `json:"agc,omitempty" yaml:"agc,omitempty"`
}

// Server represents a recently connected server.
type Server struct {
	Address       string                  `json:"address" yaml:"address"`
	Port          int                     `json:"port" yaml:"port"`
	Hostname      string                  `json:"hostname" yaml:"hostname"`
	ServerID      string                  `json:"server_id,omitempty" yaml:"server_id,omitempty"` // Stable per-port UUID; empty for old entries.
	LastConnected time.Time               `json:"last_connected" yaml:"last_connected"`
	Presets       map[string]DevicePreset `json:"presets,omitempty" yaml:"presets,omitempty"`
}

// serverJSON is used for backward-compatible deserialization from legacy JSON.
type serverJSON struct {
	Address       string                  `json:"address"`
	Port          int                     `json:"port"`
	Hostname      string                  `json:"hostname"`
	Nickname      string                  `json:"nickname"` // deprecated, read-only
	ServerID      string                  `json:"server_id,omitempty"`
	LastConnected time.Time               `json:"last_connected"`
	Presets       map[string]DevicePreset `json:"presets,omitempty"`
}

// UnmarshalJSON provides backward compatibility: reads both "hostname" and legacy "nickname" fields.
func (s *Server) UnmarshalJSON(data []byte) error {
	var raw serverJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Address = raw.Address
	s.Port = raw.Port
	s.Hostname = raw.Hostname
	if s.Hostname == "" {
		s.Hostname = raw.Nickname // migrate nickname → hostname
	}
	s.ServerID = raw.ServerID
	s.LastConnected = raw.LastConnected
	s.Presets = raw.Presets
	return nil
}

// FilePath returns the path to the recent servers YAML file.
// Uses the unified EchoWarp config directory (~/.config/echowarp/).
func FilePath() (string, error) {
	return filepath.Join(config.EchoWarpDir(), "recent_servers.yaml"), nil
}

// jsonFilePath returns the legacy JSON path for backward compatibility.
func jsonFilePath() string {
	return filepath.Join(config.EchoWarpDir(), "recent_servers.json")
}

// Load reads the recent servers list from disk.
// Tries YAML first, then falls back to legacy JSON.
// Returns an empty slice if neither file exists or data is corrupt.
func Load() ([]Server, error) {
	path, err := FilePath()
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	// Try .yaml first, fall back to .yml for backward compatibility
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		ymlPath := strings.TrimSuffix(path, ".yaml") + ".yml"
		if _, err2 := os.Stat(ymlPath); err2 == nil {
			path = ymlPath
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// Fall back to legacy JSON
		jsonPath := jsonFilePath()
		data, err = os.ReadFile(jsonPath)
		if err != nil {
			return nil, nil //nolint:nilerr
		}
		// Parse legacy JSON
		var servers []Server
		if err := json.Unmarshal(data, &servers); err != nil {
			return nil, nil //nolint:nilerr
		}
		defaultPresetVolumes(servers)
		return filterValid(servers), nil
	}

	// Parse YAML
	var servers []Server
	if err := yaml.Unmarshal(data, &servers); err != nil {
		return nil, nil //nolint:nilerr
	}
	defaultPresetVolumes(servers)
	return filterValid(servers), nil
}

// defaultPresetVolumes sets Volume to 1.0 for any PresetDevice where it is zero
// (backward compatibility with presets saved before the Volume field existed).
func defaultPresetVolumes(servers []Server) {
	for i := range servers {
		for k, preset := range servers[i].Presets {
			for j := range preset.Devices {
				if preset.Devices[j].Volume == 0 {
					preset.Devices[j].Volume = 1.0
				}
			}
			servers[i].Presets[k] = preset
		}
	}
}

// filterValid removes entries with empty address or zero port.
func filterValid(servers []Server) []Server {
	valid := servers[:0]
	for _, s := range servers {
		if s.Address != "" && s.Port > 0 {
			valid = append(valid, s)
		}
	}
	return valid
}

// Save writes the recent servers list to disk in YAML format, creating the directory if needed.
func Save(servers []Server) error {
	path, err := FilePath()
	if err != nil {
		return err
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0700); mkdirErr != nil {
		return mkdirErr
	}
	data, err := yaml.Marshal(servers)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Add upserts a server into the list.
// Matching priority:
//  1. By ServerID (if both have one) — handles servers that moved to a new IP.
//     When matched by ServerID but addr:port differs, the stored addr:port is updated.
//  2. By address:port — fallback for old entries without a ServerID.
//
// If the server already exists, it is updated and moved to front.
// Presets from an existing entry are preserved when the new entry has no presets.
// If the list exceeds MaxEntries, the oldest entry is evicted.
func Add(servers []Server, s Server) []Server {
	filtered := make([]Server, 0, len(servers))
	for _, existing := range servers {
		matched := false
		// Primary match: both have ServerID
		if s.ServerID != "" && existing.ServerID == s.ServerID {
			matched = true
		} else if existing.Address == s.Address && existing.Port == s.Port {
			// Fallback match: addr:port
			matched = true
		}
		if matched {
			// Preserve presets from existing entry if new entry has none
			if len(s.Presets) == 0 && len(existing.Presets) > 0 {
				s.Presets = existing.Presets
			}
			continue
		}
		filtered = append(filtered, existing)
	}

	// Prepend the new/updated entry
	result := make([]Server, 0, len(filtered)+1)
	result = append(result, s)
	result = append(result, filtered...)

	// Evict oldest if over limit
	if len(result) > MaxEntries {
		result = result[:MaxEntries]
	}
	return result
}
