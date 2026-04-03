package views

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ConfigSaveAction describes the outcome of a key press in the save overlay.
type ConfigSaveAction int

const (
	ConfigSaveNone      ConfigSaveAction = iota
	ConfigSaveDone                       // user confirmed save
	ConfigSaveCancelled                  // user pressed Esc
)

// ConfigSaveMode tracks save overlay state.
type ConfigSaveMode int

const (
	ConfigSaveTyping ConfigSaveMode = iota
)

// ConfigSaveOverlay manages the "Save Config" overlay with a text input,
// optional "Save as profile" checkbox, and existing configs list.
type ConfigSaveOverlay struct {
	nameInput     textinput.Model
	saveAsProfile bool // checkbox state
	focusRow      int  // 0=checkbox, 1=text input, 2=list
	existingList  []config.ConfigEntry
	listCursor    int
	mode          ConfigSaveMode
	cfgMode       config.Mode
	existing      string // name for overwrite detection
	visible       bool
	buildCfg      config.Config // current config for placeholder
}

// NewConfigSaveOverlay creates a save overlay, pre-filling with lastLoadedName if non-empty.
func NewConfigSaveOverlay(lastLoadedName string, cfgMode config.Mode, cfg config.Config) ConfigSaveOverlay {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 64
	ti.Width = 40
	ti.Placeholder = config.DefaultPlaceholder(cfg)
	ti.PlaceholderStyle = lipgloss.NewStyle().Faint(true)
	if lastLoadedName != "" {
		ti.SetValue(lastLoadedName)
	}
	ti.Focus()

	entries, _ := config.ListConfigs(cfgMode)

	return ConfigSaveOverlay{
		nameInput:    ti,
		mode:         ConfigSaveTyping,
		cfgMode:      cfgMode,
		visible:      true,
		buildCfg:     cfg,
		focusRow:     1, // text input focused
		existingList: entries,
	}
}

// Show resets and shows the overlay.
func (o *ConfigSaveOverlay) Show(lastLoadedName string, cfgMode config.Mode, cfg config.Config) {
	o.nameInput.SetValue(lastLoadedName)
	o.nameInput.Placeholder = config.DefaultPlaceholder(cfg)
	o.nameInput.Focus()
	o.mode = ConfigSaveTyping
	o.cfgMode = cfgMode
	o.existing = ""
	o.visible = true
	o.buildCfg = cfg
	o.saveAsProfile = false
	o.focusRow = 1
	o.listCursor = 0
	o.existingList, _ = config.ListConfigs(cfgMode)
}

// Hide hides the overlay.
func (o *ConfigSaveOverlay) Hide() {
	o.visible = false
	o.nameInput.Blur()
}

// effectiveName returns the name that will be used for saving.
func (o *ConfigSaveOverlay) effectiveName() string {
	name := strings.TrimSpace(o.nameInput.Value())
	if name == "" {
		name = o.nameInput.Placeholder
	}
	return name
}

// nameMatchesExisting checks if the effective name matches an existing config file.
func (o *ConfigSaveOverlay) nameMatchesExisting() bool {
	name := o.effectiveName()
	if !isValidConfigName(name) {
		return false
	}
	path := filepath.Join(config.ConfigsDir(o.cfgMode), name+".yaml")
	_, err := os.Stat(path)
	return err == nil
}

// recalcPlaceholder updates placeholder based on saveAsProfile state.
func (o *ConfigSaveOverlay) recalcPlaceholder() {
	if strings.TrimSpace(o.nameInput.Value()) != "" {
		return
	}
	if o.saveAsProfile {
		o.nameInput.Placeholder = "profile"
	} else {
		o.nameInput.Placeholder = config.DefaultPlaceholder(o.buildCfg)
	}
}

// maxFocusRow returns the maximum focusRow index (2 if list has items, 1 otherwise).
// Row 0 (checkbox) is only available in client mode.
func (o *ConfigSaveOverlay) maxFocusRow() int {
	if len(o.existingList) > 0 {
		return 2
	}
	return 1
}

// minFocusRow returns 0 for client mode (checkbox visible), 1 for server mode.
func (o *ConfigSaveOverlay) minFocusRow() int {
	if o.cfgMode == config.ModeClient {
		return 0
	}
	return 1
}

// HandleKey processes a key event and returns the resulting action and config name.
func (o *ConfigSaveOverlay) HandleKey(msg tea.KeyMsg) (ConfigSaveAction, string) {
	switch msg.Type {
	case tea.KeyEsc:
		return ConfigSaveCancelled, ""

	case tea.KeyTab:
		// Cycle focus: min -> ... -> max -> min
		o.focusRow++
		if o.focusRow > o.maxFocusRow() {
			o.focusRow = o.minFocusRow()
		}
		o.updateInputFocus()
		return ConfigSaveNone, ""

	case tea.KeyUp:
		if o.focusRow == 2 {
			// In list: move cursor up or leave list
			if o.listCursor > 0 {
				o.listCursor--
			} else {
				o.focusRow = 1
				o.updateInputFocus()
			}
		} else if o.focusRow > o.minFocusRow() {
			o.focusRow--
			o.updateInputFocus()
		}
		return ConfigSaveNone, ""

	case tea.KeyDown:
		if o.focusRow == 2 {
			// In list: move cursor down
			if o.listCursor < len(o.existingList)-1 {
				o.listCursor++
			}
		} else if o.focusRow < o.maxFocusRow() {
			o.focusRow++
			o.updateInputFocus()
		}
		return ConfigSaveNone, ""

	case tea.KeySpace:
		if o.focusRow == 0 && o.cfgMode == config.ModeClient {
			o.saveAsProfile = !o.saveAsProfile
			o.recalcPlaceholder()
			return ConfigSaveNone, ""
		}
		// For text input, fall through to default
		if o.focusRow == 1 {
			o.nameInput, _ = o.nameInput.Update(msg)
			return ConfigSaveNone, ""
		}
		return ConfigSaveNone, ""

	case tea.KeyEnter:
		if o.focusRow == 2 && len(o.existingList) > 0 {
			// Copy selected name to input, move focus to text input
			o.nameInput.SetValue(o.existingList[o.listCursor].Name)
			o.focusRow = 1
			o.updateInputFocus()
			return ConfigSaveNone, ""
		}
		// Save
		name := o.effectiveName()
		if !isValidConfigName(name) {
			return ConfigSaveNone, ""
		}
		// Set existing field for caller info, then proceed with save
		if o.nameMatchesExisting() {
			o.existing = name
		}
		return ConfigSaveDone, name

	default:
		if o.focusRow == 1 {
			o.nameInput, _ = o.nameInput.Update(msg)
		}
		return ConfigSaveNone, ""
	}
}

// updateInputFocus focuses/blurs the text input based on focusRow.
func (o *ConfigSaveOverlay) updateInputFocus() {
	if o.focusRow == 1 {
		o.nameInput.Focus()
	} else {
		o.nameInput.Blur()
	}
}

// View renders the save overlay.
func (o *ConfigSaveOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 50 {
		overlayW = 50
	}
	if overlayW < 35 {
		overlayW = 35
	}

	title := styles.SetupColumnTitle.Render("Save Config")

	var lines []string

	// Checkbox (client mode only)
	if o.cfgMode == config.ModeClient {
		check := "[ ]"
		if o.saveAsProfile {
			check = "[\u00d7]"
		}
		label := check + " Save as profile"
		tabHint := "tab"
		// Pad to right-align tabHint
		pad := overlayW - 6 - len(label) - len(tabHint) // 6 = border padding (2*2) + indent (2)
		if pad < 1 {
			pad = 1
		}
		row := "  " + label + strings.Repeat(" ", pad) + tabHint
		if o.focusRow == 0 {
			row = styles.OverlayActive.Render(row)
		} else {
			row = styles.OverlayDim.Render(row)
		}
		lines = append(lines, row, "")
	}

	// Text input
	var inputLabel string
	if o.focusRow == 1 {
		inputLabel = styles.OverlayActive.Render("  Name: ")
	} else {
		inputLabel = styles.OverlayDim.Render("  Name: ")
	}
	lines = append(lines, inputLabel+o.nameInput.View(), "")

	// Existing configs list
	if len(o.existingList) > 0 {
		for i, entry := range o.existingList {
			cursor := "  "
			if o.focusRow == 2 && i == o.listCursor {
				cursor = styles.CursorGlyph + " "
			}
			nameStr := cursor + entry.Name
			if o.focusRow == 2 && i == o.listCursor {
				nameStr = styles.OverlayActive.Render(nameStr)
			}
			lines = append(lines, nameStr)
			if entry.InfoLine != "" {
				infoStr := "    " + entry.InfoLine
				lines = append(lines, styles.SetupDimValue.Render(infoStr))
			}
		}
		lines = append(lines, "")
	}

	// Bottom hint
	hint := "enter: save"
	if o.nameMatchesExisting() {
		hint = "enter: save (overwrite)"
	}
	hint += "  \u2191\u2193: navigate  esc: cancel"
	lines = append(lines, styles.SetupDimValue.Render("  "+hint))

	content := title + "\n\n" + strings.Join(lines, "\n")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Width(overlayW)

	return border.Render(content)
}

// isValidConfigName checks for non-empty, filesystem-safe name (no /, \, ..; max 64 chars).
func isValidConfigName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return false
	}
	return true
}
