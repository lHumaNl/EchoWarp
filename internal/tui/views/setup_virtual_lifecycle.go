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

	case tea.KeyUp:
		if o.focusGroup > 0 {
			o.focusGroup--
		}

	case tea.KeyDown:
		if o.focusGroup < 2 {
			o.focusGroup++
		}

	case tea.KeyLeft, tea.KeyRight, tea.KeySpace:
		switch o.focusGroup {
		case 0:
			o.onStopIdx = 1 - o.onStopIdx
		case 1:
			o.onStartIdx = 1 - o.onStartIdx
		}
	}

	return VSLifecycleNone
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

	title := styles.SetupColumnTitle.Render("Virtual Sink Options")

	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// OnStop group
	sb.WriteString(o.renderGroup("After stream stops:", o.focusGroup == 0,
		[]string{"Delete sink", "Keep sink"}, o.onStopIdx))
	sb.WriteString("\n")

	// OnStart group
	sb.WriteString(o.renderGroup("On next startup:", o.focusGroup == 1,
		[]string{"Recreate automatically", "Don't recreate"}, o.onStartIdx))
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
