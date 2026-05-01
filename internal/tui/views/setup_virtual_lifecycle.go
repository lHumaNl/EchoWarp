package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// VirtualSinkLifecycleAction is returned by the overlay Update.
type VirtualSinkLifecycleAction int

const (
	VSLifecycleNone   VirtualSinkLifecycleAction = iota
	VSLifecycleSave                              // user pressed Enter on Save
	VSLifecycleCancel                            // user pressed Esc (use defaults)
)

// VirtualSinkLifecycleOverlay lets the user choose OnStop and OnStart behavior.
type VirtualSinkLifecycleOverlay struct {
	// 0 = OnStop group, 1 = OnStart group, 2 = Save button
	focusGroup int
	// OnStop: 0=Delete, 1=Keep
	onStopIdx int
	// OnStart: 0=Recreate, 1=Don't recreate
	onStartIdx int
}

// NewVirtualSinkLifecycleOverlay creates the overlay with defaults (Delete + Recreate).
func NewVirtualSinkLifecycleOverlay() *VirtualSinkLifecycleOverlay {
	return &VirtualSinkLifecycleOverlay{
		focusGroup: 0,
		onStopIdx:  0, // Delete
		onStartIdx: 0, // Recreate
	}
}

// OnStop returns the selected OnStop lifecycle.
func (o *VirtualSinkLifecycleOverlay) OnStop() recent.SinkLifecycle {
	if o.onStopIdx == 1 {
		return recent.SinkKeep
	}
	return recent.SinkDelete
}

// OnStart returns the selected OnStart lifecycle.
func (o *VirtualSinkLifecycleOverlay) OnStart() recent.SinkLifecycle {
	if o.onStartIdx == 1 {
		return recent.SinkKeep
	}
	return recent.SinkRecreate
}

// Update handles key events.
func (o *VirtualSinkLifecycleOverlay) Update(msg tea.KeyMsg) VirtualSinkLifecycleAction {
	switch msg.Type {
	case tea.KeyEsc:
		return VSLifecycleCancel

	case tea.KeyEnter:
		return VSLifecycleSave

	case tea.KeyTab:
		o.focusGroup = (o.focusGroup + 1) % 3

	case tea.KeyShiftTab:
		o.focusGroup = (o.focusGroup + 2) % 3

	case tea.KeyUp:
		o.selectPreviousOption()

	case tea.KeyDown:
		o.selectNextOption()

	case tea.KeyLeft:
		if o.focusGroup > 0 {
			o.focusGroup--
		}

	case tea.KeyRight:
		if o.focusGroup < 2 {
			o.focusGroup++
		}

	case tea.KeySpace:
		o.toggleFocusedOption()
	}

	return VSLifecycleNone
}

func (o *VirtualSinkLifecycleOverlay) selectPreviousOption() {
	switch o.focusGroup {
	case 0:
		o.onStopIdx = (o.onStopIdx + 1) % 2
	case 1:
		o.onStartIdx = (o.onStartIdx + 1) % 2
	}
}

func (o *VirtualSinkLifecycleOverlay) selectNextOption() {
	switch o.focusGroup {
	case 0:
		o.onStopIdx = (o.onStopIdx + 1) % 2
	case 1:
		o.onStartIdx = (o.onStartIdx + 1) % 2
	}
}

func (o *VirtualSinkLifecycleOverlay) toggleFocusedOption() {
	switch o.focusGroup {
	case 0:
		o.onStopIdx = 1 - o.onStopIdx
	case 1:
		o.onStartIdx = 1 - o.onStartIdx
	}
}

// View renders the overlay.
func (o *VirtualSinkLifecycleOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 45 {
		overlayW = 45
	}
	if overlayW < 35 {
		overlayW = 35
	}

	title := styles.SetupColumnTitle.Render("Virtual Audio Device Options")

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// OnStop group
	sb.WriteString(o.renderGroup("After stream stops:", o.focusGroup == 0,
		[]string{"Delete virtual device", "Keep virtual device"}, o.onStopIdx))
	sb.WriteString("\n")

	// OnStart group
	sb.WriteString(o.renderGroup("On next startup:", o.focusGroup == 1,
		[]string{"Create it again automatically", "Do not create automatically"}, o.onStartIdx))
	sb.WriteString("\n")

	// Buttons
	saveStyle := styles.SetupDimValue
	if o.focusGroup == 2 {
		saveStyle = styles.SetupReadyHint
	}
	buttons := saveStyle.Render("[Enter] Save") + "     " + styles.SetupDimValue.Render("[Esc] Defaults")
	btnW := lipgloss.Width(buttons)
	pad := (overlayW - 4 - btnW) / 2
	if pad < 0 {
		pad = 0
	}
	sb.WriteString(strings.Repeat(" ", pad) + buttons)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(sb.String())
}

func (o *VirtualSinkLifecycleOverlay) renderGroup(label string, focused bool, options []string, selected int) string {
	var sb strings.Builder

	labelStyle := styles.SetupDimValue
	if focused {
		labelStyle = styles.ConnParamValue
	}
	sb.WriteString(labelStyle.Render(label))
	sb.WriteString("\n")

	for i, opt := range options {
		radio := "○"
		if i == selected {
			radio = "●"
		}

		optStyle := styles.SetupDimValue
		if focused && i == selected {
			optStyle = styles.SetupReadyHint
		}
		sb.WriteString("  " + optStyle.Render(radio+" "+opt))
		sb.WriteString("\n")
	}

	return sb.String()
}
