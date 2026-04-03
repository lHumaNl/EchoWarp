package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderKickOverlay_Basic(t *testing.T) {
	p := KickOverlayParams{
		ClientNickname:   "Alice",
		Reasons:          []string{"(no reason)", "Spam / flooding", "AFK / idle too long", "Custom reason..."},
		RecentStartIndex: -1,
		CustomStartIndex: 3,
		SelectedIndex:    0,
		FocusButton:      0,
		Width:            80,
		Height:           30,
	}
	result := RenderKickOverlay(p)
	assert.Contains(t, result, "Kick Alice")
	assert.Contains(t, result, "(no reason)")
	assert.Contains(t, result, "Spam / flooding")
	assert.Contains(t, result, "Custom reason...")
	assert.Contains(t, result, "[Kick]")
	assert.Contains(t, result, "[Cancel]")
}

func TestRenderKickOverlay_WithRecent(t *testing.T) {
	p := KickOverlayParams{
		ClientNickname:   "Bob",
		Reasons:          []string{"(no reason)", "Spam / flooding", "my recent reason", "Custom reason..."},
		RecentStartIndex: 2,
		CustomStartIndex: 3,
		SelectedIndex:    2,
		FocusButton:      0,
		Width:            80,
		Height:           30,
	}
	result := RenderKickOverlay(p)
	assert.Contains(t, result, "recent")
	assert.Contains(t, result, "my recent reason")
}

func TestRenderKickOverlay_CustomEditing(t *testing.T) {
	p := KickOverlayParams{
		ClientNickname:   "Charlie",
		Reasons:          []string{"(no reason)", "Custom reason..."},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		SelectedIndex:    1,
		CustomText:       "typing here",
		CustomEditing:    true,
		FocusButton:      0,
		Width:            80,
		Height:           30,
	}
	result := RenderKickOverlay(p)
	assert.Contains(t, result, "typing here")
	assert.Contains(t, result, "Custom:")
}

func TestRenderKickOverlay_SelectionHighlight(t *testing.T) {
	p := KickOverlayParams{
		ClientNickname:   "Dave",
		Reasons:          []string{"(no reason)", "Spam / flooding"},
		RecentStartIndex: -1,
		CustomStartIndex: 2,
		SelectedIndex:    1,
		FocusButton:      0,
		Width:            80,
		Height:           30,
	}
	result := RenderKickOverlay(p)
	// The selected item should have the cursor glyph
	assert.Contains(t, result, "▸")
}

func TestRenderKickOverlay_ButtonFocus(t *testing.T) {
	p := KickOverlayParams{
		ClientNickname:   "Eve",
		Reasons:          []string{"(no reason)"},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		SelectedIndex:    0,
		FocusButton:      1, // [Kick] focused
		Width:            80,
		Height:           30,
	}
	result := RenderKickOverlay(p)
	assert.Contains(t, result, "[Kick]")
}
