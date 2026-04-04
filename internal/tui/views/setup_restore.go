package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// RestoreOverlay manages the "Create virtual devices?" overlay shown when
// preset virtual devices are missing from the system.
type RestoreOverlay struct {
	Preset        recent.DevicePreset
	Mode          string
	HasVirtual    bool
	ButtonIdx     int  // 0=Create, 1=Skip
	ServerContext bool // true for server mode (changes subtitle text)

	// VirtualOnly mode: overlay only asks about creating missing virtual devices.
	// Non-virtual devices are already auto-restored at this point.
	VirtualOnly    bool
	MissingVirtual []recent.PresetDevice
}

// NewRestoreOverlay creates a restore overlay for the given preset and mode.
// Deprecated: use NewVirtualRestoreOverlay for the new auto-restore flow.
func NewRestoreOverlay(preset recent.DevicePreset, mode string) *RestoreOverlay {
	hasVirtual := false
	for _, d := range preset.Devices {
		if d.Virtual {
			hasVirtual = true
			break
		}
	}
	return &RestoreOverlay{
		Preset:     preset,
		Mode:       mode,
		HasVirtual: hasVirtual,
		ButtonIdx:  0,
	}
}

// NewVirtualRestoreOverlay creates a restore overlay that only asks about
// creating missing virtual devices. Non-virtual devices are already auto-restored.
func NewVirtualRestoreOverlay(missingVirtual []recent.PresetDevice, mode string) *RestoreOverlay {
	return &RestoreOverlay{
		Mode:           mode,
		HasVirtual:     true,
		VirtualOnly:    true,
		MissingVirtual: missingVirtual,
		ButtonIdx:      0,
	}
}

// RestoreAction describes what the user chose in the restore overlay.
type RestoreAction int

const (
	RestoreActionNone           RestoreAction = iota
	RestoreActionAll                          // Create virtual devices
	RestoreActionWithoutVirtual               // Skip virtual creation (legacy, unused in new flow)
	RestoreActionSkip                         // Skip / dismiss
)

// buttonCount returns the number of buttons.
func (r *RestoreOverlay) buttonCount() int {
	if r.VirtualOnly {
		return 2 // [Create] [Skip]
	}
	if r.HasVirtual {
		return 3
	}
	return 2
}

// Update handles key events. Returns the chosen action (RestoreActionNone if overlay stays open).
func (r *RestoreOverlay) Update(msg tea.KeyMsg) RestoreAction {
	switch msg.Type {
	case tea.KeyEsc:
		return RestoreActionSkip
	case tea.KeyLeft:
		if r.ButtonIdx > 0 {
			r.ButtonIdx--
		}
	case tea.KeyRight:
		if r.ButtonIdx < r.buttonCount()-1 {
			r.ButtonIdx++
		}
	case tea.KeyEnter:
		if r.VirtualOnly {
			switch r.ButtonIdx {
			case 0:
				return RestoreActionAll // "Create"
			case 1:
				return RestoreActionSkip // "Skip"
			}
		} else if r.HasVirtual {
			switch r.ButtonIdx {
			case 0:
				return RestoreActionAll
			case 1:
				return RestoreActionWithoutVirtual
			case 2:
				return RestoreActionSkip
			}
		} else {
			switch r.ButtonIdx {
			case 0:
				return RestoreActionAll
			case 1:
				return RestoreActionSkip
			}
		}
	}
	return RestoreActionNone
}

// View renders the restore overlay.
func (r *RestoreOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 50 {
		overlayW = 50
	}
	if overlayW < 35 {
		overlayW = 35
	}

	if r.VirtualOnly {
		return r.viewVirtualOnly(overlayW)
	}
	return r.viewLegacy(overlayW)
}

// viewVirtualOnly renders the overlay for missing virtual device creation.
func (r *RestoreOverlay) viewVirtualOnly(overlayW int) string {
	title := styles.SetupColumnTitle.Render("Create virtual devices?")
	subtitle := styles.SetupDimValue.Render("  These virtual devices from your last session were not found:")

	lines := make([]string, 0, 2+len(r.MissingVirtual))
	lines = append(lines, subtitle)

	for _, d := range r.MissingVirtual {
		lines = append(lines, fmt.Sprintf("  🔊 %s", d.Name))
	}
	lines = append(lines, "")

	// Buttons: [Create] [Skip]
	buttons := []string{"Create", "Skip"}
	btnParts := make([]string, 0, len(buttons))
	for i, label := range buttons {
		s := styles.SetupDimValue
		if i == r.ButtonIdx {
			s = styles.SetupReadyHint
		}
		btnParts = append(btnParts, s.Render("["+label+"]"))
	}
	btnLine := strings.Join(btnParts, "  ")
	contentW := overlayW - 4
	btnW := lipgloss.Width(btnLine)
	btnPad := (contentW - btnW) / 2
	if btnPad < 0 {
		btnPad = 0
	}
	lines = append(lines, strings.Repeat(" ", btnPad)+btnLine)

	footer := styles.SetupDimValue.Render("←→: select  enter: confirm  esc: skip")
	footerW := lipgloss.Width(footer)
	footerPad := (contentW - footerW) / 2
	if footerPad < 0 {
		footerPad = 0
	}

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n\n" + strings.Repeat(" ", footerPad) + footer

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(content)
}

// viewLegacy renders the old-style restore overlay (kept for compatibility).
func (r *RestoreOverlay) viewLegacy(overlayW int) string {
	title := styles.SetupColumnTitle.Render("Restore devices?")
	subtitleText := fmt.Sprintf("  Last used with this server (%s):", r.Mode)
	if r.ServerContext {
		subtitleText = fmt.Sprintf("  Last used in this mode (%s):", r.Mode)
	}
	subtitle := styles.SetupDimValue.Render(subtitleText)

	var lines []string
	lines = append(lines, subtitle)

	for _, d := range r.Preset.Devices {
		icon := "🔊"
		if d.Virtual {
			icon = "🔊"
		}
		nameLower := strings.ToLower(d.Name)
		if strings.Contains(nameLower, "mic") || strings.Contains(nameLower, "input") ||
			strings.Contains(nameLower, "capture") || strings.Contains(nameLower, "record") {
			icon = "🎤"
		}
		suffix := ""
		if d.Virtual {
			suffix = " (virtual)"
		}
		lines = append(lines, fmt.Sprintf("  %s %s%s", icon, d.Name, suffix))
	}

	if r.HasVirtual {
		virtualCount := 0
		for _, d := range r.Preset.Devices {
			if d.Virtual {
				virtualCount++
			}
		}
		lines = append(lines, "")
		word := "device"
		if virtualCount > 1 {
			word = "devices"
		}
		lines = append(lines, styles.StatValueWarn.Render(
			fmt.Sprintf("  ⚠ %d virtual %s will be recreated", virtualCount, word)))
	}

	lines = append(lines, "")

	// Buttons
	var buttons []string
	if r.HasVirtual {
		buttons = []string{"Restore all", "Without virtual", "Skip"}
	} else {
		buttons = []string{"Restore", "Skip"}
	}

	btnParts := make([]string, 0, len(buttons))
	for i, label := range buttons {
		s := styles.SetupDimValue
		if i == r.ButtonIdx {
			s = styles.SetupReadyHint
		}
		btnParts = append(btnParts, s.Render("["+label+"]"))
	}
	btnLine := strings.Join(btnParts, "  ")
	contentW := overlayW - 4
	btnW := lipgloss.Width(btnLine)
	btnPad := (contentW - btnW) / 2
	if btnPad < 0 {
		btnPad = 0
	}
	lines = append(lines, strings.Repeat(" ", btnPad)+btnLine)

	footer := styles.SetupDimValue.Render("←→: select  enter: confirm  esc: skip")
	footerW := lipgloss.Width(footer)
	footerPad := (contentW - footerW) / 2
	if footerPad < 0 {
		footerPad = 0
	}

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n\n" + strings.Repeat(" ", footerPad) + footer

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(content)
}

// matchPresetDevices matches preset devices against the current device list.
// Returns matched device rows and a list of unmatched device names.
func matchPresetDevices(preset recent.DevicePreset, inputDevices, outputDevices []deviceRow) (matched []deviceRow, unmatched []string) {
	allDevices := append(append([]deviceRow{}, inputDevices...), outputDevices...)

	// Build name→devices index for fallback matching
	byName := make(map[string][]deviceRow)
	for _, d := range allDevices {
		byName[d.Name] = append(byName[d.Name], d)
	}

	for _, pd := range preset.Devices {
		found := false
		// 1. Exact match: ID + Name + IsInput
		for _, d := range allDevices {
			if d.ID == pd.ID && d.Name == pd.Name && d.IsInput == pd.IsInput {
				matched = append(matched, d)
				found = true
				break
			}
		}
		if found {
			continue
		}
		// 2. Fallback: unique name+type match
		if candidates, ok := byName[pd.Name]; ok {
			var typed []deviceRow
			for _, c := range candidates {
				if c.IsInput == pd.IsInput {
					typed = append(typed, c)
				}
			}
			if len(typed) == 1 {
				matched = append(matched, typed[0])
				continue
			}
		}
		// 3. Not found
		unmatched = append(unmatched, pd.Name)
	}
	return matched, unmatched
}
