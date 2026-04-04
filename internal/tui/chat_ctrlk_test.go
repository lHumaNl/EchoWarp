package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
)

// TestCtrlK_NoKickOverlay verifies Ctrl+K no longer opens kick overlay (removed hotkey).
func TestCtrlK_NoKickOverlay(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Mode: config.ModeServer}
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.cmdCh = make(chan ClientCommand, 16)
	m.focusedArea = FocusClientList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m = updated.(Model)
	assert.Equal(t, OverlayNone, m.overlay, "Ctrl+K should not open kick overlay")
}

// TestCtrlB_NoBanOverlay verifies Ctrl+B no longer opens ban overlay (removed hotkey).
func TestCtrlB_NoBanOverlay(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Mode: config.ModeServer}
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.cmdCh = make(chan ClientCommand, 16)
	m.focusedArea = FocusClientList

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m = updated.(Model)
	assert.Equal(t, OverlayNone, m.overlay, "Ctrl+B should not open ban overlay")
}
