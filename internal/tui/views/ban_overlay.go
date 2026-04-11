package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// BanOverlayParams contains the state needed to render the ban overlay.
type BanOverlayParams struct {
	ClientNickname string

	// Ban criteria checkboxes
	IP            string
	Nickname      string
	HWID          string
	HWIDAvailable bool // whether to show the HWID row
	CriteriaIP    bool
	CriteriaNick  bool
	CriteriaHWID  bool

	// Reason selection (same structure as kick)
	Reasons          []string // full list: "(no reason)", predefined..., separator, recent..., separator, "Custom reason..."
	RecentStartIndex int      // index where recent reasons start (-1 if none)
	CustomStartIndex int      // index of "Custom reason..." item
	SelectedReason   int
	CustomText       string
	CustomEditing    bool

	// Navigation
	FocusSection    int // 0=criteria, 1=reasons, 2=buttons
	CriteriaIndex   int // which criterion is highlighted (0=IP, 1=Nick, 2=HWID)
	ButtonFocus     int // 0=[Confirm], 1=[Cancel]
	ValidationError string

	Width  int
	Height int
}

// RenderBanOverlay renders the ban criteria + reason overlay centered on screen.
func RenderBanOverlay(p BanOverlayParams) string {
	title := styles.AppTitle.Render(i18n.Tf("overlay_ban_title", p.ClientNickname))

	// --- Criteria section ---
	criteriaHeader := styles.StatValueNeutral.Render(i18n.T("overlay_ban_by"))
	type criterion struct {
		checked bool
		label   string
		value   string
		suffix  string
	}
	criteria := []criterion{
		{p.CriteriaIP, i18n.T("overlay_ban_crit_ip"), p.IP, ""},
		{p.CriteriaNick, i18n.T("overlay_ban_crit_nickname"), p.Nickname, ""},
	}
	if p.HWIDAvailable {
		hwid := p.HWID
		if len(hwid) > 12 {
			hwid = hwid[:4] + "..." + hwid[len(hwid)-4:]
		}
		criteria = append(criteria, criterion{p.CriteriaHWID, i18n.T("overlay_ban_crit_hwid"), hwid, i18n.T("overlay_ban_hwid_fallback")})
	}

	criteriaLines := make([]string, 0, len(criteria))
	for i, c := range criteria {
		check := "[ ]"
		if c.checked {
			check = "[x]"
		}
		prefix := "  "
		if p.FocusSection == 0 && i == p.CriteriaIndex {
			prefix = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}
		line := fmt.Sprintf("%s %s  %s", check, c.label, c.value)
		if c.suffix != "" {
			line += styles.Help.Render(c.suffix)
		}
		criteriaLines = append(criteriaLines, prefix+line)
	}

	// --- Reason section ---
	reasonHeader := styles.StatValueNeutral.Render(i18n.T("overlay_ban_reason_header"))
	var reasonLines []string
	for i, reason := range p.Reasons {
		if p.RecentStartIndex > 0 && i == p.RecentStartIndex {
			reasonLines = append(reasonLines, styles.Help.Render(i18n.T("overlay_kick_recent_sep")))
		}
		if i == p.CustomStartIndex {
			reasonLines = append(reasonLines, styles.Help.Render(i18n.T("overlay_kick_custom_sep")))
		}

		prefix := "  "
		if p.FocusSection == 1 && i == p.SelectedReason {
			prefix = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}

		if p.CustomEditing && i == p.CustomStartIndex {
			cursor := "█"
			inputText := p.CustomText + cursor
			reasonLines = append(reasonLines, prefix+styles.StatValueNeutral.Render(i18n.T("overlay_kick_custom_label"))+inputText)
		} else {
			reasonLines = append(reasonLines, prefix+reason)
		}
	}

	// --- Buttons ---
	confirmBtn := i18n.T("overlay_ban_confirm_btn")
	cancelBtn := i18n.T("overlay_cancel_btn")
	if p.FocusSection == 2 && p.ButtonFocus == 0 {
		confirmBtn = styles.SelectedItem.Render(styles.CursorGlyph + i18n.T("overlay_ban_confirm_btn_selected"))
	}
	if p.FocusSection == 2 && p.ButtonFocus == 1 {
		cancelBtn = styles.SelectedItem.Render(styles.CursorGlyph + i18n.T("overlay_cancel_btn_selected"))
	}
	buttons := confirmBtn + "  " + cancelBtn

	// --- Validation error ---
	var validationLine string
	if p.ValidationError != "" {
		validationLine = "\n" + styles.StatValueError.Render(p.ValidationError)
	}

	help := styles.Help.Render(i18n.T("overlay_ban_help"))

	content := title + "\n\n" +
		criteriaHeader + "\n" + strings.Join(criteriaLines, "\n") + "\n\n" +
		reasonHeader + "\n" + strings.Join(reasonLines, "\n") + "\n\n" +
		buttons + validationLine + "\n\n" + help

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
