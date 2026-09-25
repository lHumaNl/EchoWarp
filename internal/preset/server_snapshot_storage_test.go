package preset

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestSnapshotCorruptProfileIsNotOverwritten(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	const fixture = "presets: {normal: {port: test-only-sensitive-value}}"
	require.NoError(t, os.WriteFile(FilePath(), []byte(fixture), 0600))
	err := SaveSnapshot(snapshotConfig(), recent.DevicePreset{})
	require.EqualError(t, err, "invalid server mode preset")
	data, err := os.ReadFile(FilePath())
	require.NoError(t, err)
	assert.Equal(t, fixture, string(data))
	assert.Empty(t, Load().Presets)
}

func TestSnapshotWriteFailureDoesNotLeakSecrets(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, os.Mkdir(FilePath(), 0700))
	cfg := snapshotConfig()
	sp := ServerPresets{Presets: map[string]ModePreset{"normal": FromConfig(cfg, recent.DevicePreset{})}}
	err := Save(sp)
	require.EqualError(t, err, "cannot write server presets")
	assert.NotContains(t, err.Error(), cfg.Password)
	assert.NotContains(t, err.Error(), cfg.PasswordHash)
	assert.NotContains(t, err.Error(), cfg.TURNServers[0].Credential)
}

func TestSnapshotReadFailureDoesNotReplaceProfile(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	require.NoError(t, os.Mkdir(FilePath(), 0700))
	err := SaveSnapshot(snapshotConfig(), recent.DevicePreset{})
	require.EqualError(t, err, "cannot read server presets")
	info, err := os.Stat(FilePath())
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
