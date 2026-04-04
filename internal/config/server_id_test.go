package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestServerID_GeneratesValidUUID(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())

	id := ServerID(4415)
	assert.NotEmpty(t, id)
	// UUID v4 format: 8-4-4-4-12 hex chars with dashes
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

func TestServerID_Idempotent(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())

	id1 := ServerID(4415)
	id2 := ServerID(4415)
	assert.Equal(t, id1, id2, "same port should always return same UUID")
}

func TestServerID_Persisted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	id := ServerID(4415)

	// Read the YAML file directly and verify persistence
	data, err := os.ReadFile(filepath.Join(dir, "server_id.yaml"))
	require.NoError(t, err)

	var m map[string]string
	require.NoError(t, yaml.Unmarshal(data, &m))
	assert.Equal(t, id, m["4415"])
}

func TestServerID_MultiPort(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())

	id1 := ServerID(4415)
	id2 := ServerID(4416)
	assert.NotEqual(t, id1, id2, "different ports should get different UUIDs")

	// Both should be stable
	assert.Equal(t, id1, ServerID(4415))
	assert.Equal(t, id2, ServerID(4416))
}

func TestServerID_ReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Pre-write a known UUID to disk in YAML format
	known := "550e8400-e29b-41d4-a716-446655440000"
	data, _ := yaml.Marshal(map[string]string{"4415": known})
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_id.yaml"), data, 0600))

	id := ServerID(4415)
	assert.Equal(t, known, id, "should reload existing UUID from disk")
}

func TestServerID_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write corrupt YAML — should not panic, should generate a new UUID
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_id.yaml"), []byte("{{bad yaml"), 0600))

	id := ServerID(4415)
	assert.NotEmpty(t, id)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

func TestServerID_JSONFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Pre-write a known UUID in legacy JSON format
	known := "550e8400-e29b-41d4-a716-446655440000"
	data, _ := json.Marshal(map[string]string{"4415": known})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_id.json"), data, 0600))

	id := ServerID(4415)
	assert.Equal(t, known, id, "should fall back to legacy JSON file")
}

func TestServerID_YAMLPreferredOverJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", dir)

	// Write both YAML and JSON with different values
	yamlID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	jsonID := "11111111-2222-3333-4444-555555555555"

	yamlData, _ := yaml.Marshal(map[string]string{"4415": yamlID})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_id.yaml"), yamlData, 0600))

	jsonData, _ := json.Marshal(map[string]string{"4415": jsonID})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server_id.json"), jsonData, 0600))

	id := ReadServerID(4415)
	assert.Equal(t, yamlID, id, "YAML should be preferred over JSON")
}
