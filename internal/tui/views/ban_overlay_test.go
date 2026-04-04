package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderBanOverlay_Basic(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Alice",
		IP:               "10.10.0.28",
		Nickname:         "Alice",
		HWIDAvailable:    false,
		CriteriaIP:       true,
		Reasons:          []string{"(no reason)", "Spam / flooding", "Custom reason..."},
		RecentStartIndex: -1,
		CustomStartIndex: 2,
		SelectedReason:   0,
		FocusSection:     0,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "Ban Alice")
	assert.Contains(t, result, "Ban by:")
	assert.Contains(t, result, "[x] IP")
	assert.Contains(t, result, "10.10.0.28")
	assert.Contains(t, result, "[ ] Nickname")
	assert.NotContains(t, result, "HWID")
	assert.Contains(t, result, "[Confirm]")
	assert.Contains(t, result, "[Cancel]")
}

func TestRenderBanOverlay_WithHWID(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Bob",
		IP:               "192.168.1.5",
		Nickname:         "Bob",
		HWID:             "a3f8deadbeef1234c2d1",
		HWIDAvailable:    true,
		CriteriaIP:       true,
		CriteriaHWID:     true,
		Reasons:          []string{"(no reason)"},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		FocusSection:     0,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "HWID")
	assert.Contains(t, result, "a3f8...c2d1")
	assert.Contains(t, result, "IP fallback")
}

func TestRenderBanOverlay_CriteriaNavigation(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Charlie",
		IP:               "10.0.0.1",
		Nickname:         "Charlie",
		HWIDAvailable:    false,
		CriteriaIP:       true,
		Reasons:          []string{"(no reason)"},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		FocusSection:     0,
		CriteriaIndex:    1, // Nickname highlighted
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	// Cursor glyph should be present (on Nickname row)
	assert.Contains(t, result, "▸")
}

func TestRenderBanOverlay_ReasonSection(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Dave",
		IP:               "10.0.0.1",
		Nickname:         "Dave",
		CriteriaIP:       true,
		Reasons:          []string{"(no reason)", "Spam / flooding", "recent reason", "Custom reason..."},
		RecentStartIndex: 2,
		CustomStartIndex: 3,
		SelectedReason:   2,
		FocusSection:     1,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "recent")
	assert.Contains(t, result, "Reason (optional):")
}

func TestRenderBanOverlay_CustomEditing(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Eve",
		IP:               "10.0.0.1",
		Nickname:         "Eve",
		CriteriaIP:       true,
		Reasons:          []string{"(no reason)", "Custom reason..."},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		SelectedReason:   1,
		CustomText:       "my custom reason",
		CustomEditing:    true,
		FocusSection:     1,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "my custom reason")
	assert.Contains(t, result, "Custom:")
}

func TestRenderBanOverlay_ValidationError(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Frank",
		IP:               "10.0.0.1",
		Nickname:         "Frank",
		Reasons:          []string{"(no reason)"},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		ValidationError:  "Select at least one ban criterion",
		FocusSection:     2,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "Select at least one ban criterion")
}

func TestRenderBanOverlay_ButtonFocus(t *testing.T) {
	p := BanOverlayParams{
		ClientNickname:   "Grace",
		IP:               "10.0.0.1",
		Nickname:         "Grace",
		CriteriaIP:       true,
		Reasons:          []string{"(no reason)"},
		RecentStartIndex: -1,
		CustomStartIndex: 1,
		FocusSection:     2,
		ButtonFocus:      0,
		Width:            80,
		Height:           30,
	}
	result := RenderBanOverlay(p)
	assert.Contains(t, result, "[Confirm]")
	// Cursor should appear on confirm button
	assert.Contains(t, result, "▸")
}
