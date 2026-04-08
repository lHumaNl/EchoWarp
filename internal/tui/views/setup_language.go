package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// LanguageAction represents the result of a language overlay interaction.
type LanguageAction int

const (
	LanguageActionNone     LanguageAction = iota
	LanguageActionSelected                // user confirmed a language
	LanguageActionCancel                  // user pressed Esc
)

// languageOption pairs a language code with its display name.
type languageOption struct {
	Code    i18n.Language
	Display string
}

// knownLanguages maps language codes to native display names.
var knownLanguages = map[i18n.Language]string{
	i18n.English: "English",
	i18n.Russian: "Русский",
}

// LanguageOverlay lets the user pick a UI language.
type LanguageOverlay struct {
	options  []languageOption
	cursor   int
	selected i18n.Language // language that was active when overlay opened
}

// NewLanguageOverlay creates the overlay, pre-selecting the current language.
func NewLanguageOverlay() *LanguageOverlay {
	cur := i18n.CurrentLanguage()
	langs := i18n.AvailableLanguages()

	opts := make([]languageOption, 0, len(langs))
	curIdx := 0
	for idx, l := range langs {
		display, ok := knownLanguages[l]
		if !ok {
			display = string(l)
		}
		opts = append(opts, languageOption{Code: l, Display: display})
		if l == cur {
			curIdx = idx
		}
	}

	return &LanguageOverlay{
		options:  opts,
		cursor:   curIdx,
		selected: cur,
	}
}

// Update handles key events and returns the resulting action.
func (o *LanguageOverlay) Update(msg tea.KeyMsg) LanguageAction {
	switch msg.Type {
	case tea.KeyEsc:
		return LanguageActionCancel
	case tea.KeyUp:
		if o.cursor > 0 {
			o.cursor--
		}
	case tea.KeyDown:
		if o.cursor < len(o.options)-1 {
			o.cursor++
		}
	case tea.KeyEnter:
		lang := o.options[o.cursor].Code
		i18n.SetLanguage(lang)
		_ = i18n.SaveLanguage(lang)
		return LanguageActionSelected
	}
	return LanguageActionNone
}

// View renders the overlay.
func (o *LanguageOverlay) View(width int) string {
	overlayW := 34
	if overlayW > width-4 {
		overlayW = width - 4
	}
	if overlayW < 20 {
		overlayW = 20
	}

	title := i18n.T("lang_overlay_title")
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(overlayW)

	lines := make([]string, 0, len(o.options))
	for idx, opt := range o.options {
		bullet := "● "
		if opt.Code != o.selected {
			bullet = "○ "
		}
		line := bullet + opt.Display
		if idx == o.cursor {
			line = styles.SelectedItem.Render(line)
		}
		lines = append(lines, line)
	}

	hint := i18n.T("lang_overlay_hint")
	body := strings.Join(lines, "\n") + "\n\n" + styles.SetupDimValue.Render(hint)

	content := styles.SetupColumnTitle.Render(title) + "\n\n" + body
	return border.Render(content)
}
