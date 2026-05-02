package views

import (
	"strings"
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
	assert.Contains(t, view, "Create Virtual Audio Device")
	assert.Contains(t, view, "ctrl+u clears")
	assert.Contains(t, view, "EchoWarp▌")
	assert.Contains(t, view, "Playback EchoWarp")
	assert.Contains(t, view, "[Create]")
	assert.Contains(t, view, "[Cancel]")
	assert.Contains(t, view, "Capture EchoWarp")
}

func TestVirtualOverlay_CustomNameTypingUpdatesPreview(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Discord")})
	view := overlay.View(80)

	assert.Equal(t, VirtualActionNone, action)
	assert.Equal(t, "Discord", overlay.BaseName())
	assert.Contains(t, view, "Discord▌")
	assert.Contains(t, view, "Playback Discord")
	assert.Contains(t, view, "Capture Discord")
}

func TestVirtualOverlay_NameEditingBackspaceAndClear(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	overlay.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Discord")})

	overlay.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "Discor", overlay.BaseName())
	overlay.Update(tea.KeyMsg{Type: tea.KeyCtrlU})

	assert.Empty(t, overlay.NameInput)
	assert.Contains(t, overlay.View(80), "type name…▌")
}

func TestVirtualOverlay_CreateCompositeNotClippedAtSmallHeight(t *testing.T) {
	m := SetupModel{width: 80, height: 14}
	overlay := NewVirtualDeviceOverlay(false, echowarpSinkName).View(80)

	view := m.compositeOverlay("base", overlay)

	assert.Contains(t, view, "Create Virtual Audio Device")
	assert.Contains(t, view, "[Create]")
	assert.Contains(t, view, "esc: cancel")
	assert.Contains(t, view, "╰")
	assert.GreaterOrEqual(t, len(strings.Split(view, "\n")), len(strings.Split(overlay, "\n")))
}

func TestVirtualOverlay_ExistsState(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "EchoWarp")
	view := overlay.View(80)
	assert.Contains(t, view, "Manage Virtual Audio Devices")
	assert.Contains(t, view, "Remove EchoWarp")
	assert.Contains(t, view, "Create new virtual device")
	assert.Contains(t, view, "[Remove]")
	assert.Contains(t, view, "[Cancel]")
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

func TestVirtualOverlay_CreateNewWhenDevicesExist(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "EchoWarp")
	overlay.SetDevices([]VirtualOverlayDevice{{SinkName: "EchoWarp", BaseName: "EchoWarp", Removable: true}})

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, VirtualActionNone, action)
	action = overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionNone, action)
	assert.True(t, overlay.IsCreateMode())
	assert.Equal(t, "EchoWarp 2", overlay.BaseName())
	assert.Contains(t, overlay.View(80), "Playback EchoWarp 2")

	action = overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionCreate, action)
}

func TestVirtualOverlay_CreateNewSuggestsNextEchoWarpSuffix(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "EchoWarp")
	overlay.SetDevices([]VirtualOverlayDevice{
		{SinkName: "EchoWarp", BaseName: "EchoWarp", Removable: true},
		{SinkName: "custom", BaseName: "EchoWarp 2", Removable: true},
	})
	overlay.RowIdx = len(overlay.Devices)

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, VirtualActionNone, action)
	assert.Equal(t, "EchoWarp 3", overlay.BaseName())
}

func TestVirtualOverlay_RemoveTargetsSelectedSink(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "studio-a")
	overlay.SetDevices([]VirtualOverlayDevice{
		{SinkName: "studio-a", Removable: true},
		{SinkName: "studio-b", Removable: true},
	})

	overlay.Update(tea.KeyMsg{Type: tea.KeyDown})
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, VirtualActionRemove, action)
	assert.Equal(t, "studio-b", overlay.SinkName)
}

func TestVirtualOverlay_NonRemovableDeviceShowsOwnershipError(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(true, "studio-a")
	overlay.SetDevices([]VirtualOverlayDevice{{SinkName: "studio-a", Owner: "server"}})

	view := overlay.View(80)
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.Contains(t, view, "owned by server (not removable)")
	assert.Equal(t, VirtualActionNone, action)
	assert.Equal(t, "studio-a virtual audio device is owned by server", overlay.Error)
	assert.NotContains(t, overlay.View(80), "pulseaudio-utils")
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

func TestVirtualOverlay_MissingPulseAudioUtilityErrorShowsInstallGuidance(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	overlay.Error = `pactl failed: exec: "pactl": executable file not found in $PATH`
	view := overlay.View(80)
	assert.Contains(t, view, "pactl failed")
	assert.Contains(t, view, "pulseaudio-utils")
	assert.Contains(t, view, "[OK]")

	// Enter on error dismisses
	action := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VirtualActionCancel, action)
}

func TestVirtualOverlay_NonInstallErrorDoesNotShowInstallGuidance(t *testing.T) {
	overlay := NewVirtualDeviceOverlay(false, "EchoWarp")
	overlay.Error = "pactl failed: exit status 1"

	view := overlay.View(80)

	assert.Contains(t, view, "pactl failed")
	assert.NotContains(t, view, "pulseaudio-utils")
	assert.NotContains(t, view, "Make sure PulseAudio is installed")
	assert.Contains(t, view, "[OK]")
}

func TestPulseAudioInstallGuidanceClassifier(t *testing.T) {
	assert.True(t, shouldShowPulseAudioInstallGuidance("pactl: command not found"))
	assert.True(t, shouldShowPulseAudioInstallGuidance("install pulseaudio-utils"))
	assert.False(t, shouldShowPulseAudioInstallGuidance("pactl failed: exit status 1"))
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
