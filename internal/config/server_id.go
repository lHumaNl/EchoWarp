// Package config — server_id.go manages per-port server UUIDs.
// The file ~/.config/echowarp/server_id.yaml stores a map of port→UUID,
// allowing the same physical server to be recognized across different networks.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var (
	serverIDMu       sync.Mutex
	serverIDFile     = "server_id.yaml"
	serverIDFileJSON = "server_id.json" // legacy
)

// serverIDFilePath returns the path to the server ID YAML file.
func serverIDFilePath() string {
	return filepath.Join(EchoWarpDir(), serverIDFile)
}

// serverIDFilePathJSON returns the legacy JSON path.
func serverIDFilePathJSON() string {
	return filepath.Join(EchoWarpDir(), serverIDFileJSON)
}

// loadServerIDs reads the port→UUID map from disk.
// Tries YAML first, then falls back to legacy JSON.
// Returns an empty map if both are missing or corrupt.
func loadServerIDs() map[string]string {
	// Try .yaml first, fall back to .yml for backward compatibility
	yamlPath := serverIDFilePath()
	readPath := yamlPath
	if _, statErr := os.Stat(yamlPath); os.IsNotExist(statErr) {
		ymlPath := strings.TrimSuffix(yamlPath, ".yaml") + ".yml"
		if _, err2 := os.Stat(ymlPath); err2 == nil {
			readPath = ymlPath
		}
	}

	// Try YAML first
	data, err := os.ReadFile(readPath)
	if err == nil {
		var m map[string]string
		if unmarshalErr := yaml.Unmarshal(data, &m); unmarshalErr == nil {
			return m
		}
	}
	// Fall back to legacy JSON
	data, err = os.ReadFile(serverIDFilePathJSON())
	if err != nil {
		return make(map[string]string)
	}
	var m map[string]string
	if unmarshalErr := json.Unmarshal(data, &m); unmarshalErr != nil {
		return make(map[string]string)
	}
	return m
}

// saveServerIDs writes the port→UUID map to disk in YAML format.
func saveServerIDs(m map[string]string) error {
	path := serverIDFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ReadServerID returns the UUID for the given port, or "" if none exists.
// Does not generate a new UUID — use ServerID for that.
func ReadServerID(port int) string {
	serverIDMu.Lock()
	defer serverIDMu.Unlock()

	key := fmt.Sprintf("%d", port)
	m := loadServerIDs()
	return m[key]
}

// ServerID returns the UUID for the given port.
// If no UUID exists for that port, a new one is generated and persisted.
// Thread-safe; uses a file-level mutex.
func ServerID(port int) string {
	serverIDMu.Lock()
	defer serverIDMu.Unlock()

	key := fmt.Sprintf("%d", port)
	m := loadServerIDs()
	if id, ok := m[key]; ok && id != "" {
		return id
	}

	id := uuid.New().String()
	m[key] = id
	_ = saveServerIDs(m) //nolint:errcheck — best-effort persistence; UUID still returned
	return id
}
