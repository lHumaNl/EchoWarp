package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderClientPopup_NormalMode(t *testing.T) {
	items := []PopupMenuItem{
		{Label: "Mute outgoing", Hotkey: "Ctrl+O", Action: "mute_outgoing", IsToggle: true},
		{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		{Label: "Ban", Hotkey: "Ctrl+B", Action: "ban"},
		{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	}
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Client-2",
		Items:          items,
		SelectedIndex:  0,
		Width:          34,
	})
	require.NotEmpty(t, result)
	assert.Contains(t, result, "Client-2")
	assert.Contains(t, result, "Mute outgoing")
	assert.Contains(t, result, "Kick")
	assert.Contains(t, result, "Ban")
	assert.Contains(t, result, "Cancel")
	assert.Contains(t, result, "Ctrl+O")
	assert.Contains(t, result, "Ctrl+K")
}

func TestRenderClientPopup_ReverseMode(t *testing.T) {
	items := []PopupMenuItem{
		{Label: "Mute incoming", Hotkey: "Ctrl+M", Action: "mute_incoming", IsToggle: true},
		{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		{Label: "Ban", Hotkey: "Ctrl+B", Action: "ban"},
		{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	}
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Client-3",
		Items:          items,
		SelectedIndex:  1,
		Width:          34,
	})
	require.NotEmpty(t, result)
	assert.Contains(t, result, "Client-3")
	assert.Contains(t, result, "Mute incoming")
	// Index 1 (Kick) should be selected, index 0 should not have cursor
	lines := strings.Split(result, "\n")
	var kickLine string
	for _, l := range lines {
		if strings.Contains(l, "Kick") {
			kickLine = l
			break
		}
	}
	assert.Contains(t, kickLine, "\u25b8") // cursor glyph
}

func TestRenderClientPopup_DuplexMode(t *testing.T) {
	items := []PopupMenuItem{
		{Label: "Mute outgoing", Hotkey: "Ctrl+O", Action: "mute_outgoing", IsToggle: true},
		{Label: "Mute incoming", Hotkey: "Ctrl+M", Action: "mute_incoming", IsToggle: true},
		{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		{Label: "Ban", Hotkey: "Ctrl+B", Action: "ban"},
		{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	}
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Client-1",
		Items:          items,
		SelectedIndex:  0,
		Width:          36,
	})
	require.NotEmpty(t, result)
	assert.Contains(t, result, "Mute outgoing")
	assert.Contains(t, result, "Mute incoming")
	assert.Contains(t, result, "Ctrl+O")
	assert.Contains(t, result, "Ctrl+M")
}

func TestRenderClientPopup_ToggleActive(t *testing.T) {
	items := []PopupMenuItem{
		{Label: "Mute outgoing", Hotkey: "Ctrl+O", Action: "mute_outgoing", IsToggle: true, IsActive: true},
		{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	}
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Client-2",
		Items:          items,
		SelectedIndex:  0,
		Width:          34,
	})
	require.NotEmpty(t, result)
	// When IsActive, "Mute outgoing" becomes "Unmute outgoing"
	assert.Contains(t, result, "Unmute outgoing")
	assert.NotContains(t, result, "Mute outgoing")
}

func TestRenderClientPopup_SelectionHighlight(t *testing.T) {
	items := []PopupMenuItem{
		{Label: "Mute outgoing", Hotkey: "Ctrl+O", Action: "mute_outgoing"},
		{Label: "Kick", Hotkey: "Ctrl+K", Action: "kick"},
		{Label: "Cancel", Hotkey: "Esc", Action: "cancel"},
	}

	// Test selection at index 2 (Cancel)
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Test",
		Items:          items,
		SelectedIndex:  2,
		Width:          34,
	})
	lines := strings.Split(result, "\n")
	var cancelLine string
	for _, l := range lines {
		if strings.Contains(l, "Cancel") {
			cancelLine = l
			break
		}
	}
	assert.Contains(t, cancelLine, "\u25b8") // cursor glyph on Cancel
}

func TestRenderClientPopup_EmptyItems(t *testing.T) {
	result := RenderClientPopup(ClientPopupParams{
		ClientNickname: "Client-1",
		Items:          nil,
		SelectedIndex:  0,
		Width:          34,
	})
	assert.Empty(t, result)
}
