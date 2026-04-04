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

func testCfg() config.Config {
	cfg := config.DefaultConfig()
	cfg.Address = "10.10.0.28"
	cfg.Port = 4415
	return cfg
}

func TestConfigSaveOverlay_NewPrefillsName(t *testing.T) {
	o := NewConfigSaveOverlay("my-config", config.ModeServer, testCfg())
	assert.True(t, o.visible)
	assert.Equal(t, ConfigSaveTyping, o.mode)
	assert.Equal(t, "my-config", o.nameInput.Value())
}

func TestConfigSaveOverlay_NewEmptyName(t *testing.T) {
	o := NewConfigSaveOverlay("", config.ModeServer, testCfg())
	assert.Equal(t, "", o.nameInput.Value())
}

func TestConfigSaveOverlay_Placeholder(t *testing.T) {
	cfg := testCfg()
	o := NewConfigSaveOverlay("", config.ModeClient, cfg)
	assert.Equal(t, "10.10.0.28_4415", o.nameInput.Placeholder)
}

func TestConfigSaveOverlay_EscCancels(t *testing.T) {
	o := NewConfigSaveOverlay("", config.ModeServer, testCfg())
	action, name := o.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, ConfigSaveCancelled, action)
	assert.Empty(t, name)
}

func TestConfigSaveOverlay_EmptyNameUsesPlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	cfg := testCfg()
	o := NewConfigSaveOverlay("", config.ModeClient, cfg)
	action, name := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigSaveDone, action)
	assert.Equal(t, "10.10.0.28_4415", name)
}

func TestConfigSaveOverlay_InvalidNameRejected(t *testing.T) {
	o := NewConfigSaveOverlay("foo/bar", config.ModeServer, testCfg())
	action, _ := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigSaveNone, action)
}

func TestConfigSaveOverlay_ValidNameNoExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	o := NewConfigSaveOverlay("test-config", config.ModeServer, testCfg())
	action, name := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigSaveDone, action)
	assert.Equal(t, "test-config", name)
}

func TestConfigSaveOverlay_ExistingFileOverwritesDirectly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	dir := filepath.Join(tmpDir, "configs", "server")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.yaml"), []byte("mode: server"), 0600))

	o := NewConfigSaveOverlay("existing", config.ModeServer, testCfg())
	action, name := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigSaveDone, action)
	assert.Equal(t, "existing", name)
	assert.Equal(t, "existing", o.existing) // overwrite detected
}

func TestConfigSaveOverlay_ShowHide(t *testing.T) {
	o := NewConfigSaveOverlay("", config.ModeServer, testCfg())
	assert.True(t, o.visible)
	o.Hide()
	assert.False(t, o.visible)
	o.Show("new-name", config.ModeClient, testCfg())
	assert.True(t, o.visible)
	assert.Equal(t, "new-name", o.nameInput.Value())
	assert.Equal(t, config.ModeClient, o.cfgMode)
}

func TestConfigSaveOverlay_View(t *testing.T) {
	o := NewConfigSaveOverlay("test", config.ModeServer, testCfg())
	view := o.View(80)
	assert.Contains(t, view, "Save Config")
}

func TestConfigSaveOverlay_CheckboxClientOnly(t *testing.T) {
	// Client mode: checkbox visible
	o := NewConfigSaveOverlay("", config.ModeClient, testCfg())
	view := o.View(80)
	assert.Contains(t, view, "Save as profile")

	// Server mode: no checkbox
	o2 := NewConfigSaveOverlay("", config.ModeServer, testCfg())
	view2 := o2.View(80)
	assert.NotContains(t, view2, "Save as profile")
}

func TestConfigSaveOverlay_CheckboxToggle(t *testing.T) {
	o := NewConfigSaveOverlay("", config.ModeClient, testCfg())
	assert.False(t, o.saveAsProfile)

	// Move to checkbox row
	o.focusRow = 0
	o.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
	assert.True(t, o.saveAsProfile)
	assert.Equal(t, "profile", o.nameInput.Placeholder)

	o.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
	assert.False(t, o.saveAsProfile)
	assert.Equal(t, "10.10.0.28_4415", o.nameInput.Placeholder)
}

func TestConfigSaveOverlay_TabCyclesFocus(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	o := NewConfigSaveOverlay("", config.ModeClient, testCfg())
	assert.Equal(t, 1, o.focusRow) // starts on text input

	// No list items, so max is 1. Tab: 1 -> 0 -> 1
	o.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 0, o.focusRow) // checkbox

	o.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, o.focusRow) // back to input
}

func TestConfigSaveOverlay_TabServerNoCheckbox(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	o := NewConfigSaveOverlay("", config.ModeServer, testCfg())
	assert.Equal(t, 1, o.focusRow)

	// Server mode: min is 1, max is 1 (no list). Tab stays on 1.
	o.HandleKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, o.focusRow)
}

func TestConfigSaveOverlay_ListSelectCopiesName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	// Create existing config files
	dir := filepath.Join(tmpDir, "configs", "client")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "home_server.yaml"), []byte("role: client\naddress: 10.0.0.1"), 0600))

	o := NewConfigSaveOverlay("", config.ModeClient, testCfg())
	require.True(t, len(o.existingList) > 0)

	// Navigate to list
	o.focusRow = 2
	o.listCursor = 0

	// Press Enter to copy name
	action, _ := o.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, ConfigSaveNone, action)
	assert.Equal(t, "home_server", o.nameInput.Value())
	assert.Equal(t, 1, o.focusRow) // focus moved to input
}

func TestConfigSaveOverlay_OverwriteHintInView(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ECHOWARP_CONFIG_DIR", tmpDir)

	dir := filepath.Join(tmpDir, "configs", "server")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.yaml"), []byte("mode: server"), 0600))

	o := NewConfigSaveOverlay("existing", config.ModeServer, testCfg())
	view := o.View(80)
	assert.Contains(t, view, "overwrite")
}

func TestIsValidConfigName(t *testing.T) {
	assert.True(t, isValidConfigName("my-config"))
	assert.True(t, isValidConfigName("config_v2"))
	assert.False(t, isValidConfigName(""))
	assert.False(t, isValidConfigName("foo/bar"))
	assert.False(t, isValidConfigName("foo\\bar"))
	assert.False(t, isValidConfigName("foo..bar"))

	long := ""
	for i := 0; i < 65; i++ {
		long += "a"
	}
	assert.False(t, isValidConfigName(long))
	assert.True(t, isValidConfigName(long[:64]))
}
