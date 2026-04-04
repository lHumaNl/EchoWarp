package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// DeviceOverlayItem represents a device in the overlay.
type DeviceOverlayItem struct {
	ID         uint32
	Name       string
	IsInput    bool
	Channels   uint32
	SampleRate uint32
	BitDepth   uint32
	Muted      bool
}

// DeviceOverlayParams contains the state needed to render the device overlay.
type DeviceOverlayParams struct {
	Devices  []DeviceOverlayItem
	Section  int // 0=input, 1=output
	Selected int // selected index within current section
	Width    int
	Height   int
}

// RenderDeviceOverlay renders the device management overlay centered on screen.
func RenderDeviceOverlay(p DeviceOverlayParams) string {
	// Split devices into input and output.
	var inputs, outputs []DeviceOverlayItem
	for _, d := range p.Devices {
		if d.IsInput {
			inputs = append(inputs, d)
		} else {
			outputs = append(outputs, d)
		}
	}

	title := styles.AppTitle.Render("Devices")

	var sections []string

	// Input section
	inputHeader := "🎤 Input"
	if p.Section == 0 {
		inputHeader = styles.SelectedItem.Render(inputHeader)
	} else {
		inputHeader = styles.Help.Render(inputHeader)
	}
	sections = append(sections, inputHeader, styles.Help.Render("  "+strings.Repeat("─", 40)))

	if len(inputs) == 0 {
		sections = append(sections, styles.Help.Render("  (none)"))
	} else {
		for i, d := range inputs {
			sections = append(sections, renderDeviceOverlayRow(d, p.Section == 0 && i == p.Selected))
		}
	}

	sections = append(sections, "")

	// Output section
	outputHeader := "🔊 Output"
	if p.Section == 1 {
		outputHeader = styles.SelectedItem.Render(outputHeader)
	} else {
		outputHeader = styles.Help.Render(outputHeader)
	}
	sections = append(sections, outputHeader, styles.Help.Render("  "+strings.Repeat("─", 40)))

	if len(outputs) == 0 {
		sections = append(sections, styles.Help.Render("  (none)"))
	} else {
		for i, d := range outputs {
			sections = append(sections, renderDeviceOverlayRow(d, p.Section == 1 && i == p.Selected))
		}
	}

	help := styles.Help.Render("Tab: section   ↑↓: select   Enter: mute/unmute   Esc: close")

	content := title + "\n\n" + strings.Join(sections, "\n") + "\n\n" + help

	boxWidth := 56
	if p.Width < boxWidth+6 {
		boxWidth = p.Width - 6
	}
	if boxWidth < 36 {
		boxWidth = 36
	}

	box := styles.OverlayBorder.Width(boxWidth).Render(content)

	boxRenderedWidth := lipgloss.Width(box)
	leftPad := (p.Width - boxRenderedWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	boxHeight := lipgloss.Height(box)
	topPad := (p.Height - boxHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	var result strings.Builder
	if topPad > 0 {
		result.WriteString(strings.Repeat("\n", topPad))
	}
	for _, line := range strings.Split(box, "\n") {
		fmt.Fprintf(&result, "%s%s\n", strings.Repeat(" ", leftPad), line)
	}

	return result.String()
}

func renderDeviceOverlayRow(d DeviceOverlayItem, selected bool) string {
	icon := "🔊"
	if d.Muted {
		icon = "🔇"
	}

	info := formatDeviceOverlayInfo(d)

	prefix := "  "
	if selected {
		prefix = styles.SelectedItem.Render(styles.CursorGlyph + " ")
	}

	return prefix + icon + " " + d.Name + "  " + styles.Help.Render(info)
}

func formatDeviceOverlayInfo(d DeviceOverlayItem) string {
	ch := ""
	if d.Channels > 0 {
		ch = FormatChannels(d.Channels)
	}
	bd := ""
	if d.BitDepth > 0 {
		bd = formatBitDepth(d.BitDepth)
	}
	sr := ""
	if d.SampleRate > 0 {
		sr = formatSampleRate(d.SampleRate)
	}
	return fmt.Sprintf("%s %s %s", ch, bd, sr)
}
