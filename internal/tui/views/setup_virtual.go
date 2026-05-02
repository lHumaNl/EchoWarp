package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// VirtualAction represents the result of a virtual device overlay interaction.
type VirtualAction int

const (
	VirtualActionNone   VirtualAction = iota
	VirtualActionCreate               // user confirmed creation
	VirtualActionRemove               // user confirmed removal
	VirtualActionCancel               // user pressed Cancel or Esc
)

type virtualOverlayMode int

const (
	virtualOverlayModeCreate virtualOverlayMode = iota
	virtualOverlayModeManage
)

// VirtualOverlayDevice is an existing virtual audio device shown in manage mode.
type VirtualOverlayDevice struct {
	SinkName     string
	BaseName     string
	CaptureName  string
	PlaybackName string
	Owner        string
	Removable    bool
}

// VirtualDeviceOverlay shows a confirmation screen for creating/removing
// a Linux virtual audio device.
type VirtualDeviceOverlay struct {
	Exists             bool   // true if virtual mic already created
	SinkName           string // e.g. "EchoWarp"
	CaptureName        string // user-facing capture device name for existing sinks
	NameInput          string // user-provided base name for new virtual devices
	ButtonIdx          int    // 0=Create/Remove, 1=Cancel
	RowIdx             int    // selected existing device or create-new row in manage mode
	Error              string // error from pactl (if any)
	Devices            []VirtualOverlayDevice
	NameEdited         bool
	NonActionableError bool
	mode               virtualOverlayMode
}

// NewVirtualDeviceOverlay creates the overlay.
func NewVirtualDeviceOverlay(exists bool, sinkName string) *VirtualDeviceOverlay {
	overlay := &VirtualDeviceOverlay{
		Exists:    exists,
		SinkName:  sinkName,
		NameInput: defaultVirtualBaseName,
	}
	if exists {
		overlay.mode = virtualOverlayModeManage
		overlay.SetDevices([]VirtualOverlayDevice{{SinkName: sinkName, Removable: true}})
	}
	return overlay
}

func (v *VirtualDeviceOverlay) SetDevices(devices []VirtualOverlayDevice) {
	v.Devices = devices
	if len(v.Devices) == 0 && v.Exists && v.SinkName != "" {
		v.Devices = []VirtualOverlayDevice{{SinkName: v.SinkName, Removable: true}}
	}
	if v.RowIdx > len(v.Devices) {
		v.RowIdx = len(v.Devices)
	}
}

func (v *VirtualDeviceOverlay) BaseName() string {
	return normalizeVirtualBaseName(v.NameInput)
}

func (v *VirtualDeviceOverlay) ExistingCaptureName() string {
	if selected, ok := v.selectedDevice(); ok && selected.CaptureName != "" {
		return selected.CaptureName
	}
	if v.CaptureName != "" {
		return v.CaptureName
	}
	return "Monitor of " + v.SinkName
}

func (v *VirtualDeviceOverlay) IsCreateMode() bool {
	return v.mode == virtualOverlayModeCreate
}

// Update handles key events and returns the resulting action.
func (v *VirtualDeviceOverlay) Update(msg tea.KeyMsg) VirtualAction {
	switch msg.Type {
	case tea.KeyEsc:
		return VirtualActionCancel

	case tea.KeyLeft:
		if v.ButtonIdx > 0 {
			v.ButtonIdx--
		}

	case tea.KeyRight:
		if v.ButtonIdx < 1 {
			v.ButtonIdx++
		}

	case tea.KeyEnter:
		if v.Error != "" {
			// Error state: OK dismisses
			return VirtualActionCancel
		}
		if v.ButtonIdx == 1 {
			return VirtualActionCancel
		}
		if v.mode == virtualOverlayModeManage {
			return v.updateManageEnter()
		}
		return VirtualActionCreate
	case tea.KeyUp:
		if v.mode == virtualOverlayModeManage && v.RowIdx > 0 {
			v.RowIdx--
		}
	case tea.KeyDown:
		if v.mode == virtualOverlayModeManage && v.RowIdx < len(v.Devices) {
			v.RowIdx++
		}
	case tea.KeyBackspace:
		v.backspaceNameInput()
	case tea.KeyCtrlU:
		v.clearNameInput()
	case tea.KeyRunes:
		v.appendNameInput(string(msg.Runes))
	}

	return VirtualActionNone
}

func (v *VirtualDeviceOverlay) updateManageEnter() VirtualAction {
	if v.RowIdx == len(v.Devices) {
		v.mode = virtualOverlayModeCreate
		v.ButtonIdx = 0
		v.NameInput = v.suggestCreateBaseName()
		v.NameEdited = false
		return VirtualActionNone
	}
	if selected, ok := v.selectedDevice(); ok {
		if !selected.Removable {
			v.setNonActionableError(nonRemovableVirtualDeviceMessage(selected))
			return VirtualActionNone
		}
		v.SinkName = selected.SinkName
		v.CaptureName = selected.CaptureName
		return VirtualActionRemove
	}
	return VirtualActionNone
}

// View renders the overlay.
func (v *VirtualDeviceOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 55 {
		overlayW = 55
	}
	if overlayW < 35 {
		overlayW = 35
	}

	title := styles.SetupColumnTitle.Render(v.title())
	contentW := overlayW - 4

	var body string
	if v.Error != "" {
		body = v.viewError(contentW)
	} else if v.mode == virtualOverlayModeManage {
		body = v.viewManage(contentW)
	} else {
		body = v.viewCreate(contentW)
	}

	content := title + "\n\n" + body

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(content)
}

func (v *VirtualDeviceOverlay) title() string {
	if v.mode == virtualOverlayModeManage {
		return "Manage Virtual Audio Devices"
	}
	return "Create Virtual Audio Device"
}

func (v *VirtualDeviceOverlay) viewCreate(contentW int) string {
	var sb strings.Builder

	sb.WriteString(styles.SetupDimValue.Render(
		"Creates playback/capture virtual devices."))
	sb.WriteString("\n")
	sb.WriteString(styles.SetupDimValue.Render(
		"Type a custom name, backspace edits, ctrl+u clears."))
	sb.WriteString("\n\n")
	sb.WriteString(styles.SetupDimValue.Render("Name: "))
	sb.WriteString(styles.ConnParamValue.Render(v.renderNameInput()))
	sb.WriteString("\n")
	sb.WriteString(styles.SetupDimValue.Render("Playback: "))
	sb.WriteString(styles.ConnParamValue.Render("Playback " + v.BaseName()))
	sb.WriteString("\n")
	sb.WriteString(styles.SetupDimValue.Render("Capture:  "))
	sb.WriteString(styles.ConnParamValue.Render("Capture " + v.BaseName()))
	sb.WriteString("\n\n")

	sb.WriteString(v.renderButtons("[Create]", "[Cancel]", contentW))
	sb.WriteString("\n\n")
	sb.WriteString(v.renderFooter(contentW))

	return sb.String()
}

func (v *VirtualDeviceOverlay) viewManage(contentW int) string {
	var sb strings.Builder
	sb.WriteString(styles.SetupDimValue.Render("Existing virtual audio devices:"))
	sb.WriteString("\n\n")
	for i, device := range v.Devices {
		sb.WriteString(v.renderDeviceRow(i, device))
		sb.WriteString("\n")
	}
	sb.WriteString(v.renderCreateRow())
	sb.WriteString("\n\n")
	sb.WriteString(v.renderManageButtons(contentW))
	sb.WriteString("\n\n")
	sb.WriteString(v.renderFooter(contentW))
	return sb.String()
}

func (v *VirtualDeviceOverlay) renderDeviceRow(index int, device VirtualOverlayDevice) string {
	label := v.deviceRowLabel(device)
	if v.RowIdx == index {
		return styles.SetupReadyHint.Render("› " + label)
	}
	return styles.SetupDimValue.Render("  " + label)
}

func (v *VirtualDeviceOverlay) deviceRowLabel(device VirtualOverlayDevice) string {
	name := deviceDisplayName(device)
	if device.Removable {
		return fmt.Sprintf("Remove %s", name)
	}
	return fmt.Sprintf("%s — owned by %s (not removable)", name, deviceOwner(device))
}

func (v *VirtualDeviceOverlay) renderCreateRow() string {
	label := "Create new virtual device"
	if v.RowIdx == len(v.Devices) {
		return styles.SetupReadyHint.Render("› " + label)
	}
	return styles.SetupDimValue.Render("  " + label)
}

func (v *VirtualDeviceOverlay) renderManageButtons(contentW int) string {
	if v.RowIdx == len(v.Devices) {
		return v.renderButtons("[Create]", "[Cancel]", contentW)
	}
	return v.renderButtons("[Remove]", "[Cancel]", contentW)
}

func (v *VirtualDeviceOverlay) selectedDevice() (VirtualOverlayDevice, bool) {
	if v.RowIdx < 0 || v.RowIdx >= len(v.Devices) {
		return VirtualOverlayDevice{}, false
	}
	return v.Devices[v.RowIdx], true
}

func deviceDisplayName(device VirtualOverlayDevice) string {
	if device.PlaybackName != "" && device.CaptureName != "" {
		return device.PlaybackName + " / " + device.CaptureName
	}
	if device.PlaybackName != "" {
		return device.PlaybackName
	}
	return device.SinkName
}

func nonRemovableVirtualDeviceMessage(device VirtualOverlayDevice) string {
	return fmt.Sprintf("%s virtual audio device is owned by %s", device.SinkName, deviceOwner(device))
}

func (v *VirtualDeviceOverlay) SetError(err error) {
	if err == nil {
		return
	}
	v.Error = err.Error()
	v.NonActionableError = isVirtualSinkOwnershipError(err)
}

func (v *VirtualDeviceOverlay) setNonActionableError(message string) {
	v.Error = message
	v.NonActionableError = true
}

func deviceOwner(device VirtualOverlayDevice) string {
	if device.Owner != "" {
		return device.Owner
	}
	return "another role"
}

func (v *VirtualDeviceOverlay) renderNameInput() string {
	if strings.TrimSpace(v.NameInput) == "" {
		return "type name…▌"
	}
	return v.BaseName() + "▌"
}

func (v *VirtualDeviceOverlay) appendNameInput(text string) {
	if v.mode != virtualOverlayModeCreate {
		return
	}
	if !v.NameEdited {
		v.NameInput = ""
		v.NameEdited = true
	}
	v.NameInput += text
}

func (v *VirtualDeviceOverlay) backspaceNameInput() {
	if v.mode != virtualOverlayModeCreate || v.NameInput == "" {
		return
	}
	v.NameEdited = true
	runes := []rune(v.NameInput)
	v.NameInput = string(runes[:len(runes)-1])
}

func (v *VirtualDeviceOverlay) clearNameInput() {
	if v.mode != virtualOverlayModeCreate {
		return
	}
	v.NameEdited = true
	v.NameInput = ""
}

func (v *VirtualDeviceOverlay) suggestCreateBaseName() string {
	used := v.usedVirtualBaseNames()
	if !used[defaultVirtualBaseName] {
		return defaultVirtualBaseName
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s %d", defaultVirtualBaseName, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

func (v *VirtualDeviceOverlay) usedVirtualBaseNames() map[string]bool {
	used := make(map[string]bool, len(v.Devices))
	for _, device := range v.Devices {
		if baseName := virtualOverlayDeviceBaseName(device); baseName != "" {
			used[baseName] = true
		}
	}
	return used
}

func virtualOverlayDeviceBaseName(device VirtualOverlayDevice) string {
	if device.BaseName != "" {
		return normalizeVirtualBaseName(device.BaseName)
	}
	if baseName, ok := strings.CutPrefix(device.PlaybackName, "Playback "); ok {
		return normalizeVirtualBaseName(baseName)
	}
	if device.SinkName == echowarpSinkName {
		return defaultVirtualBaseName
	}
	return ""
}

func (v *VirtualDeviceOverlay) viewError(contentW int) string {
	var sb strings.Builder

	sb.WriteString(styles.ConnParamValue.Render("✗ " + v.Error))
	sb.WriteString("\n\n")
	v.renderErrorGuidance(&sb)

	okBtn := styles.SetupReadyHint.Render("[OK]")
	btnW := lipgloss.Width(okBtn)
	pad := (contentW - btnW) / 2
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(strings.Repeat(" ", pad) + okBtn)

	return sb.String()
}

func (v *VirtualDeviceOverlay) renderErrorGuidance(sb *strings.Builder) {
	if v.NonActionableError {
		sb.WriteString(styles.SetupDimValue.Render("This device cannot be removed from this EchoWarp role."))
		sb.WriteString("\n\n")
		return
	}
	sb.WriteString(styles.SetupDimValue.Render("Make sure PulseAudio is installed:"))
	sb.WriteString("\n")
	sb.WriteString(styles.ConnParamValue.Render("  sudo apt install pulseaudio-utils"))
	sb.WriteString("\n\n")
}

func (v *VirtualDeviceOverlay) renderButtons(leftLabel, rightLabel string, contentW int) string {
	leftStyle := styles.SetupDimValue
	rightStyle := styles.SetupDimValue
	if v.ButtonIdx == 0 {
		leftStyle = styles.SetupReadyHint
	} else {
		rightStyle = styles.SetupReadyHint
	}

	buttons := leftStyle.Render(leftLabel) + "     " + rightStyle.Render(rightLabel)
	btnW := lipgloss.Width(buttons)
	pad := (contentW - btnW) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + buttons
}

func (v *VirtualDeviceOverlay) renderFooter(contentW int) string {
	footer := styles.SetupDimValue.Render("esc: cancel")
	footerW := lipgloss.Width(footer)
	footerPad := (contentW - footerW) / 2
	if footerPad < 0 {
		footerPad = 0
	}
	return strings.Repeat(" ", footerPad) + footer
}
