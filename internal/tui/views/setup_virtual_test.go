package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVirtualOverlay_CreateConfirmation(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	require.NotNil(t, overlay)
	assert.False(t, overlay.Exists)

	view := overlay.View(80)
	assert.Contains(t, view, "Virtual Microphone")
	assert.Contains(t, view, "PulseAudio virtual sink")
	assert.Contains(t, view, "[Create]")
	assert.Contains(t, view, "[Cancel]")
	assert.Contains(t, view, "Monitor of EchoWarp")
}

func TestVirtualOverlay_ExistsState(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "EchoWarp")
	view := overlay.View(80)
	assert.Contains(t, view, "Virtual mic active")
	assert.Contains(t, view, "[Remove]")
	assert.Contains(t, view, "[OK]")
	assert.Contains(t, view, "Monitor of EchoWarp")
}

func TestVirtualOverlay_Create_Enter(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	// Default button is Create (index 0)
	assert.Equal(t, 0, overlay.ButtonIdx)
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionCreate, action)
}

func TestVirtualOverlay_Remove_Enter(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "EchoWarp")
	// Default button is Remove (index 0)
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionRemove, action)
}

func TestVirtualOverlay_Cancel(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	// Move to Cancel
	overlay.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, overlay.ButtonIdx)
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionCancel, action)
}

func TestVirtualOverlay_Esc(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, VirtualActionCancel, action)
}

func TestVirtualOverlay_ErrorState(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	overlay.Error = "pactl failed: command not found"
	view := overlay.View(80)
	assert.Contains(t, view, "pactl failed")
	assert.Contains(t, view, "pulseaudio-utils")
	assert.Contains(t, view, "[OK]")

	// Enter on error dismisses
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionCancel, action)
}

func TestVirtualOverlay_ButtonNavigation(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")

	// Start at 0, left does nothing
	overlay.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, overlay.ButtonIdx)

	// Right moves to 1
	overlay.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, overlay.ButtonIdx)

	// Right again — stays at 1
	overlay.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, overlay.ButtonIdx)

	// Left back to 0
	overlay.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, overlay.ButtonIdx)
}
