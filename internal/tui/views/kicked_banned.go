package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// KickedViewParams contains parameters for rendering the kicked screen.
type KickedViewParams struct {
	Reason         string
	SelectedButton int // 0=Reconnect, 1=Settings, 2=Quit
	Width          int
	Height         int
}

// BannedViewParams contains parameters for rendering the banned screen.
type BannedViewParams struct {
	Reason         string
	Criteria       []string
	SelectedButton int // 0=Settings, 1=Quit
	Width          int
	Height         int
}

var (
	kickBanTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	kickBanLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	btnActive    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	btnInactive  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
)

// RenderKickedView renders the kicked-by-server screen.
func RenderKickedView(p KickedViewParams) string {
	var b strings.Builder

	b.WriteString(kickBanTitle.Render(i18n.T("kicked_title")))
	b.WriteString("\n")
	if p.Reason != "" {
		b.WriteString(kickBanLabel.Render(i18n.Tf("kickban_label_reason", p.Reason)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	buttons := []string{i18n.T("kickban_btn_reconnect"), i18n.T("kickban_btn_settings"), i18n.T("kickban_btn_quit")}
	b.WriteString(renderButtons(buttons, p.SelectedButton))

	maxW := p.Width - 8
	if maxW < 30 {
		maxW = 30
	}
	return styles.OverlayBorderDanger.MaxWidth(maxW).Render(b.String())
}

// RenderBannedView renders the banned-by-server screen.
func RenderBannedView(p BannedViewParams) string {
	var b strings.Builder

	b.WriteString(kickBanTitle.Render(i18n.T("banned_title")))
	b.WriteString("\n")
	if p.Reason != "" {
		b.WriteString(kickBanLabel.Render(i18n.Tf("kickban_label_reason", p.Reason)))
		b.WriteString("\n")
	}
	if len(p.Criteria) > 0 {
		b.WriteString(kickBanLabel.Render(i18n.Tf("kickban_label_criteria", strings.Join(p.Criteria, ", "))))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	buttons := []string{i18n.T("kickban_btn_settings"), i18n.T("kickban_btn_quit")}
	b.WriteString(renderButtons(buttons, p.SelectedButton))

	maxW := p.Width - 8
	if maxW < 30 {
		maxW = 30
	}
	return styles.OverlayBorderDanger.MaxWidth(maxW).Render(b.String())
}

// renderButtons renders a row of buttons with the selected one highlighted.
func renderButtons(labels []string, selected int) string {
	var parts []string
	for i, label := range labels {
		text := fmt.Sprintf("[%s]", label)
		if i == selected {
			parts = append(parts, btnActive.Render(text))
		} else {
			parts = append(parts, btnInactive.Render(text))
		}
	}
	return strings.Join(parts, "  ")
}
