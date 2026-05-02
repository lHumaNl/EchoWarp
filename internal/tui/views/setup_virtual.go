package views

import (
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

// VirtualDeviceOverlay shows a confirmation screen for creating/removing
// a Linux virtual audio device.
type VirtualDeviceOverlay struct {
	Exists      bool   // true if virtual mic already created
	SinkName    string // e.g. "EchoWarp"
	CaptureName string // user-facing capture device name for existing sinks
	NameInput   string // user-provided base name for new virtual devices
	ButtonIdx   int    // 0=Create/Remove, 1=Cancel
	Error       string // error from pactl (if any)
}

// NewVirtualDeviceOverlay creates the overlay.
func NewVirtualDeviceOverlay(exists bool, sinkName string) *VirtualDeviceOverlay {
	return &VirtualDeviceOverlay{
		Exists:    exists,
		SinkName:  sinkName,
		NameInput: defaultVirtualBaseName,
	}
}

func (v *VirtualDeviceOverlay) BaseName() string {
	return normalizeVirtualBaseName(v.NameInput)
}

func (v *VirtualDeviceOverlay) ExistingCaptureName() string {
	if v.CaptureName != "" {
		return v.CaptureName
	}
	return "Monitor of " + v.SinkName
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
		if v.Exists {
			return VirtualActionRemove
		}
		return VirtualActionCreate
	case tea.KeyBackspace:
		if !v.Exists && v.NameInput != "" {
			v.NameInput = v.NameInput[:len(v.NameInput)-1]
		}
	case tea.KeyRunes:
		if !v.Exists {
			v.NameInput += string(msg.Runes)
		}
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

	title := styles.SetupColumnTitle.Render("Create Virtual Audio Device")
	contentW := overlayW - 4

	var body string
	if v.Error != "" {
		body = v.viewError(contentW)
	} else if v.Exists {
		body = v.viewExists(contentW)
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

func (v *VirtualDeviceOverlay) viewCreate(contentW int) string {
	var sb strings.Builder

	sb.WriteString(styles.SetupDimValue.Render(
		"Creates playback/capture virtual devices."))
	sb.WriteString("\n\n")
	sb.WriteString(styles.SetupDimValue.Render("Name: "))
	sb.WriteString(styles.ConnParamValue.Render(v.BaseName()))
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

func (v *VirtualDeviceOverlay) viewExists(contentW int) string {
	var sb strings.Builder

	sb.WriteString(styles.SetupReadyHint.Render("✓ Virtual audio device active — " + v.SinkName))
	sb.WriteString("\n\n")
	sb.WriteString(styles.SetupDimValue.Render("In Discord / Zoom / OBS select:"))
	sb.WriteString("\n")
	sb.WriteString(styles.ConnParamValue.Render("  \"" + v.ExistingCaptureName() + "\" as microphone"))
	sb.WriteString("\n\n")

	sb.WriteString(v.renderButtons("[Remove]", "[OK]", contentW))
	sb.WriteString("\n\n")
	sb.WriteString(v.renderFooter(contentW))

	return sb.String()
}

func (v *VirtualDeviceOverlay) viewError(contentW int) string {
	var sb strings.Builder

	sb.WriteString(styles.ConnParamValue.Render("✗ " + v.Error))
	sb.WriteString("\n\n")
	sb.WriteString(styles.SetupDimValue.Render("Make sure PulseAudio is installed:"))
	sb.WriteString("\n")
	sb.WriteString(styles.ConnParamValue.Render("  sudo apt install pulseaudio-utils"))
	sb.WriteString("\n\n")

	okBtn := styles.SetupReadyHint.Render("[OK]")
	btnW := lipgloss.Width(okBtn)
	pad := (contentW - btnW) / 2
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(strings.Repeat(" ", pad) + okBtn)

	return sb.String()
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
