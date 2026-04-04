package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateHWID(t *testing.T) {
	hwid, err := GenerateHWID()
	require.NoError(t, err)
	assert.Len(t, hwid, 32, "HWID should be 32 hex chars (16 bytes)")

	// Deterministic: same inputs → same output.
	hwid2, err := GenerateHWID()
	require.NoError(t, err)
	assert.Equal(t, hwid, hwid2, "HWID should be deterministic")
}

func TestLoadOrGenerateHWID_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "hwid")

	// Patch hwidFilePath for this test.
	origFn := hwidFilePathFn
	hwidFilePathFn = func() (string, error) { return path, nil }
	defer func() { hwidFilePathFn = origFn }()

	hwid, err := LoadOrGenerateHWID()
	require.NoError(t, err)
	assert.Len(t, hwid, 32)

	// File should now exist.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), hwid)
}

func TestLoadOrGenerateHWID_ReadsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "hwid")
	err := os.WriteFile(path, []byte("abcdef1234567890abcdef1234567890\n"), 0o600)
	require.NoError(t, err)

	origFn := hwidFilePathFn
	hwidFilePathFn = func() (string, error) { return path, nil }
	defer func() { hwidFilePathFn = origFn }()

	hwid, err := LoadOrGenerateHWID()
	require.NoError(t, err)
	assert.Equal(t, "abcdef1234567890abcdef1234567890", hwid)
}

func TestGetPrimaryMAC(t *testing.T) {
	mac := getPrimaryMAC()
	// On CI/containers this may be empty, but on regular machines it should have a value.
	t.Logf("Primary MAC: %q", mac)
}

func TestNewSessionID(t *testing.T) {
	id := NewSessionID()
	assert.Len(t, id, 36, "UUID v4 should be 36 chars with hyphens")

	id2 := NewSessionID()
	assert.NotEqual(t, id, id2, "Session IDs should be unique")
}
