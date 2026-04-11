package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// KickOverlayParams contains the state needed to render the kick overlay.
type KickOverlayParams struct {
	ClientNickname   string
	Reasons          []string // full list: "(no reason)", predefined..., separator, recent..., separator, "Custom reason..."
	RecentStartIndex int      // index where recent reasons start (-1 if none)
	CustomStartIndex int      // index of "Custom reason..." item
	SelectedIndex    int
	CustomText       string
	CustomEditing    bool
	FocusButton      int // 0=list, 1=[Kick], 2=[Cancel]
	Width            int
	Height           int
}

// RenderKickOverlay renders the kick reason overlay centered on screen.
func RenderKickOverlay(p KickOverlayParams) string {
	title := styles.AppTitle.Render(i18n.Tf("overlay_kick_title", p.ClientNickname))

	var lines []string
	for i, reason := range p.Reasons {
		// Separator before recent section
		if p.RecentStartIndex > 0 && i == p.RecentStartIndex {
			lines = append(lines, styles.Help.Render(i18n.T("overlay_kick_recent_sep")))
		}
		// Separator before custom
		if i == p.CustomStartIndex {
			lines = append(lines, styles.Help.Render(i18n.T("overlay_kick_custom_sep")))
		}

		prefix := "  "
		if i == p.SelectedIndex && p.FocusButton == 0 {
			prefix = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}

		if p.CustomEditing && i == p.CustomStartIndex {
			// Show text input field
			cursor := "█"
			inputText := p.CustomText + cursor
			lines = append(lines, prefix+styles.StatValueNeutral.Render(i18n.T("overlay_kick_custom_label"))+inputText)
		} else {
			lines = append(lines, prefix+reason)
		}
	}

	// Buttons
	kickBtn := i18n.T("overlay_kick_btn")
	cancelBtn := i18n.T("overlay_cancel_btn")
	if p.FocusButton == 1 {
		kickBtn = styles.SelectedItem.Render(styles.CursorGlyph + i18n.T("overlay_kick_btn_selected"))
	}
	if p.FocusButton == 2 {
		cancelBtn = styles.SelectedItem.Render(styles.CursorGlyph + i18n.T("overlay_cancel_btn_selected"))
	}
	buttons := kickBtn + "  " + cancelBtn

	help := styles.Help.Render(i18n.T("overlay_kick_help"))

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n\n" + buttons + "\n\n" + help

	boxWidth := 52
	if p.Width < boxWidth+6 {
		boxWidth = p.Width - 6
	}
	if boxWidth < 30 {
		boxWidth = 30
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
