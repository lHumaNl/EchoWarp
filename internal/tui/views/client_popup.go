package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// PopupMenuItem represents a single item in the client popup menu.
type PopupMenuItem struct {
	Label    string // display label (e.g. "Mute outgoing")
	Hotkey   string // hotkey hint (e.g. "Ctrl+O")
	Action   string // identifier for dispatching (e.g. "mute_outgoing")
	IsToggle bool   // true for mute items
	IsActive bool   // when IsToggle && IsActive, prefix changes to "Unmute"
}

// ClientPopupParams holds all data needed to render the client popup.
type ClientPopupParams struct {
	ClientNickname string
	Items          []PopupMenuItem
	SelectedIndex  int
	Width          int
}

// RenderClientPopup renders a bordered popup box with menu items and hotkey hints.
func RenderClientPopup(p ClientPopupParams) string {
	if len(p.Items) == 0 {
		return ""
	}

	popupWidth := p.Width
	if popupWidth <= 0 {
		popupWidth = 34
	}

	// Calculate inner width (subtract border + padding: 2 border + 4 padding = 6)
	innerWidth := popupWidth - 6
	if innerWidth < 20 {
		innerWidth = 20
	}

	var lines []string
	for i, item := range p.Items {
		label := item.Label
		if item.IsToggle && item.IsActive {
			// Replace "Mute" prefix with "Unmute"
			if strings.HasPrefix(label, "Mute") {
				label = "Unmute" + label[len("Mute"):]
			}
		}

		cursor := "  "
		if i == p.SelectedIndex {
			cursor = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}

		hotkey := item.Hotkey
		// Right-align hotkey: available space = innerWidth - len(cursor visible) - len(label)
		// cursor visible width is 2
		labelWidth := lipgloss.Width(cursor) + lipgloss.Width(label)
		hotkeyWidth := lipgloss.Width(hotkey)
		padding := innerWidth - labelWidth - hotkeyWidth
		if padding < 1 {
			padding = 1
		}

		hotkeyStyled := styles.OverlayDim.Render(hotkey)
		line := fmt.Sprintf("%s%s%s%s", cursor, label, strings.Repeat(" ", padding), hotkeyStyled)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	// Build the box with title in border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(0, 2).
		Width(popupWidth)

	box := boxStyle.Render(content)

	// Inject title into top border line
	title := fmt.Sprintf(" %s ", p.ClientNickname)
	titleStyled := styles.OverlayActive.Render(title)

	boxLines := strings.Split(box, "\n")
	if len(boxLines) > 0 {
		topBorder := boxLines[0]
		// Replace characters after the first corner to inject title
		topRunes := []rune(topBorder)
		if len(topRunes) > 3 {
			// Insert title after "╭─"
			prefix := string(topRunes[:2]) // "╭─" (first 2 runes)
			suffix := ""
			titlePlainLen := lipgloss.Width(titleStyled)
			remainLen := lipgloss.Width(topBorder) - 2 - titlePlainLen
			if remainLen > 0 {
				suffix = strings.Repeat("─", remainLen-1) + string(topRunes[len(topRunes)-1])
			}
			// Re-color the border parts
			borderColor := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			boxLines[0] = borderColor.Render(prefix) + titleStyled + borderColor.Render(suffix)
		}
	}

	return strings.Join(boxLines, "\n")
}
