package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ConfigLoadAction describes the outcome of a key press in the load overlay.
type ConfigLoadAction int

const (
	ConfigLoadNone      ConfigLoadAction = iota
	ConfigLoadSelected                   // user selected a config (Enter)
	ConfigLoadAsProfile                  // user selected to load as profile (Shift+Enter)
	ConfigLoadDeleted                    // user deleted a config
	ConfigLoadCancelled                  // user pressed Esc
)

// ConfigLoadMode tracks load overlay state.
type ConfigLoadMode int

const (
	ConfigLoadBrowse        ConfigLoadMode = iota
	ConfigLoadConfirmDelete                // confirming deletion
)

// ConfigLoadOverlay manages the "Load Config" overlay with a list of saved configs.
type ConfigLoadOverlay struct {
	entries       []config.ConfigEntry
	selectedIndex int
	mode          ConfigLoadMode
	cfgMode       config.Mode
	deleteName    string // name being confirmed for deletion
	visible       bool
	loadAsProfile bool // Tab toggle: load selected entry as profile (ignore base fields)
}

// NewConfigLoadOverlay creates a load overlay populated with saved configs for the given mode.
func NewConfigLoadOverlay(cfgMode config.Mode) ConfigLoadOverlay {
	entries, _ := config.ListConfigs(cfgMode)
	return ConfigLoadOverlay{
		entries:       entries,
		selectedIndex: 0,
		mode:          ConfigLoadBrowse,
		cfgMode:       cfgMode,
		visible:       true,
	}
}

// Show refreshes the list and shows the overlay.
func (o *ConfigLoadOverlay) Show(cfgMode config.Mode) {
	o.entries, _ = config.ListConfigs(cfgMode)
	o.selectedIndex = 0
	o.mode = ConfigLoadBrowse
	o.cfgMode = cfgMode
	o.deleteName = ""
	o.visible = true
}

// Hide hides the overlay.
func (o *ConfigLoadOverlay) Hide() {
	o.visible = false
}

// HandleKey processes a key event and returns the resulting action and selected entry.
func (o *ConfigLoadOverlay) HandleKey(msg tea.KeyMsg) (ConfigLoadAction, *config.ConfigEntry) {
	switch o.mode {
	case ConfigLoadBrowse:
		switch msg.Type {
		case tea.KeyEsc:
			return ConfigLoadCancelled, nil
		case tea.KeyUp:
			if o.selectedIndex > 0 {
				o.selectedIndex--
			}
		case tea.KeyDown:
			if o.selectedIndex < len(o.entries)-1 {
				o.selectedIndex++
			}
		case tea.KeyTab:
			// Toggle load mode (client only)
			if o.cfgMode == config.ModeClient {
				o.loadAsProfile = !o.loadAsProfile
			}
		case tea.KeyEnter:
			if len(o.entries) > 0 && o.selectedIndex < len(o.entries) {
				entry := o.entries[o.selectedIndex]
				if o.loadAsProfile {
					return ConfigLoadAsProfile, &entry
				}
				return ConfigLoadSelected, &entry
			}
		case tea.KeyDelete, tea.KeyCtrlD:
			if len(o.entries) > 0 && o.selectedIndex < len(o.entries) {
				o.deleteName = o.entries[o.selectedIndex].Name
				o.mode = ConfigLoadConfirmDelete
			}
		}

	case ConfigLoadConfirmDelete:
		switch msg.Type {
		case tea.KeyEnter:
			_ = config.DeleteConfig(o.cfgMode, o.deleteName)
			deletedName := o.deleteName
			o.entries, _ = config.ListConfigs(o.cfgMode)
			if o.selectedIndex >= len(o.entries) && o.selectedIndex > 0 {
				o.selectedIndex = len(o.entries) - 1
			}
			o.mode = ConfigLoadBrowse
			o.deleteName = ""
			return ConfigLoadDeleted, &config.ConfigEntry{Name: deletedName}
		case tea.KeyEsc:
			o.mode = ConfigLoadBrowse
			o.deleteName = ""
		}
	}

	return ConfigLoadNone, nil
}

// View renders the load overlay.
func (o *ConfigLoadOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 65 {
		overlayW = 65
	}
	if overlayW < 35 {
		overlayW = 35
	}

	titleText := "Load Config"
	if o.cfgMode == config.ModeClient {
		titleText = "Load Config / Profile"
	}
	title := styles.SetupColumnTitle.Render(titleText)

	var lines []string

	// Segmented toggle for client mode: Config / Profile
	if o.cfgMode == config.ModeClient && o.mode != ConfigLoadConfirmDelete {
		configLabel := "Config"
		profileLabel := "Profile"
		if o.loadAsProfile {
			configLabel = styles.SetupDimValue.Render("  " + configLabel + "  ")
			profileLabel = styles.SetupColumnTitle.Render(" ▸ " + profileLabel + " ")
		} else {
			configLabel = styles.SetupColumnTitle.Render(" ▸ " + configLabel + " ")
			profileLabel = styles.SetupDimValue.Render("  " + profileLabel + "  ")
		}
		lines = append(lines, "  "+configLabel+profileLabel+"          tab")
		if o.loadAsProfile {
			lines = append(lines, styles.SetupDimValue.Render("  ignore address/port/password"))
		} else {
			lines = append(lines, styles.SetupDimValue.Render("  apply all settings"))
		}
		lines = append(lines, "")
	}

	if o.mode == ConfigLoadConfirmDelete {
		lines = append(lines, styles.StatValueWarn.Render("  Delete \""+o.deleteName+"\"?"), "", styles.SetupDimValue.Render("  Enter: confirm / Esc: cancel"))
	} else if len(o.entries) == 0 {
		emptyMsg := "  No saved configs"
		if o.cfgMode == config.ModeClient {
			emptyMsg = "  No saved configs or profiles"
		}
		lines = append(lines, styles.SetupDimValue.Render(emptyMsg))
	} else {
		for i, e := range o.entries {
			cursor := "  "
			if i == o.selectedIndex {
				cursor = "▸ "
			}
			nameStyle := styles.SetupDimValue
			if i == o.selectedIndex {
				nameStyle = styles.SetupColumnTitle
			}
			lines = append(lines, cursor+nameStyle.Render(e.Name))
			if e.InfoLine != "" {
				lines = append(lines, "    "+styles.SetupDimValue.Render(e.InfoLine))
			}
		}
	}

	// Bottom hint
	loadHint := "enter: load"
	if o.loadAsProfile {
		loadHint = "enter: load as profile"
	} else if o.mode == ConfigLoadBrowse && len(o.entries) > 0 && o.selectedIndex < len(o.entries) {
		if strings.HasPrefix(o.entries[o.selectedIndex].InfoLine, "📋") {
			loadHint = "enter: load profile"
		}
	}
	lines = append(lines, "", styles.SetupDimValue.Render("  ↑↓: browse  "+loadHint+"  ^D: delete  esc: cancel"))

	content := title + "\n\n" + strings.Join(lines, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(content)
}
