package views

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func setupLoadTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)
	dir := filepath.Join(tmpDir, "configs", "server")
	require.NoError(t, os.MkdirAll(dir, 0700))
	return dir
}

func TestConfigLoadOverlay_EmptyList(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	assert.True(t, o.visible)
	assert.Empty(t, o.entries)
}

func TestConfigLoadOverlay_ListsConfigs(t *testing.T) {
	dir := setupLoadTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha.yaml"), []byte("mode: server\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "beta.yaml"), []byte("mode: server\n"), 0600))

	o := NewConfigLoadOverlay(config.ModeServer)
	assert.Len(t, o.entries, 2)
}

func TestConfigLoadOverlay_NavigateAndSelect(t *testing.T) {
	dir := setupLoadTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha.yaml"), []byte("mode: server\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "beta.yaml"), []byte("mode: server\n"), 0600))

	o := NewConfigLoadOverlay(config.ModeServer)
	assert.Equal(t, 0, o.selectedIndex)

	// Move down
	o.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, o.selectedIndex)

	// Move up
	o.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, o.selectedIndex)

	// Don't go below 0
	o.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, o.selectedIndex)

	// Select
	action, entry := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigLoadSelected, action)
	assert.NotNil(t, entry)
}

func TestConfigLoadOverlay_EscCancels(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	action, _ := o.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ConfigLoadCancelled, action)
}

func TestConfigLoadOverlay_DeleteFlow(t *testing.T) {
	dir := setupLoadTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deleteme.yaml"), []byte("mode: server\n"), 0600))

	o := NewConfigLoadOverlay(config.ModeServer)
	require.Len(t, o.entries, 1)

	// Ctrl+D to trigger delete confirm
	action, _ := o.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Equal(t, ConfigLoadNone, action)
	assert.Equal(t, ConfigLoadConfirmDelete, o.mode)
	assert.Equal(t, "deleteme", o.deleteName)

	// Esc to cancel delete
	action, _ = o.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ConfigLoadNone, action)
	assert.Equal(t, ConfigLoadBrowse, o.mode)

	// Ctrl+D again, then confirm
	o.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	action, entry := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigLoadDeleted, action)
	assert.NotNil(t, entry)
	assert.Equal(t, "deleteme", entry.Name)

	// File should be deleted
	_, err := os.Stat(filepath.Join(dir, "deleteme.yaml"))
	assert.True(t, os.IsNotExist(err))
	assert.Empty(t, o.entries)
}

func TestConfigLoadOverlay_EnterOnEmptyList(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	action, entry := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigLoadNone, action)
	assert.Nil(t, entry)
}

func TestConfigLoadOverlay_ShowHide(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	assert.True(t, o.visible)
	o.Hide()
	assert.False(t, o.visible)
	o.Show(config.ModeServer)
	assert.True(t, o.visible)
}

func TestConfigLoadOverlay_View(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	view := o.View(80)
	assert.Contains(t, view, "Load Config")
	assert.Contains(t, view, "No saved configs")
}

func TestConfigLoadOverlay_ViewWithEntries(t *testing.T) {
	dir := setupLoadTestDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("mode: server\n"), 0600))

	o := NewConfigLoadOverlay(config.ModeServer)
	view := o.View(80)
	assert.Contains(t, view, "Load Config")
	assert.Contains(t, view, "test")
}

func TestConfigLoadOverlay_ClientTitle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)
	dir := filepath.Join(tmpDir, "configs", "client")
	require.NoError(t, os.MkdirAll(dir, 0700))

	o := NewConfigLoadOverlay(config.ModeClient)
	view := o.View(80)
	assert.Contains(t, view, "Load Config / Profile")
}

func TestConfigLoadOverlay_ClientEmptyMessage(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "configs", "client"), 0700))

	o := NewConfigLoadOverlay(config.ModeClient)
	view := o.View(80)
	assert.Contains(t, view, "No saved configs or profiles")
}

func TestConfigLoadOverlay_ServerEmptyMessage(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	o := NewConfigLoadOverlay(config.ModeServer)
	view := o.View(80)
	assert.Contains(t, view, "No saved configs")
	assert.NotContains(t, view, "or profiles")
}

func TestConfigLoadOverlay_HintLoadProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)
	dir := filepath.Join(tmpDir, "configs", "client")
	require.NoError(t, os.MkdirAll(dir, 0700))
	// Profile: no address field
	profileYAML := "role: client\nnickname: Gamer\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mygamer.yaml"), []byte(profileYAML), 0600))

	o := NewConfigLoadOverlay(config.ModeClient)
	require.NotEmpty(t, o.entries)
	view := o.View(80)
	assert.Contains(t, view, "enter: load profile")
}

func TestConfigLoadOverlay_HintLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)
	dir := filepath.Join(tmpDir, "configs", "client")
	require.NoError(t, os.MkdirAll(dir, 0700))
	// Config: has address field
	configYAML := "role: client\naddress: 10.0.0.1\nport: 4415\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "myserver.yaml"), []byte(configYAML), 0600))

	o := NewConfigLoadOverlay(config.ModeClient)
	require.NotEmpty(t, o.entries)
	view := o.View(80)
	assert.Contains(t, view, "enter: load")
	assert.NotContains(t, view, "enter: load profile")
}
