package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ParticipantOverlayAction is the action chosen in the participant overlay.
type ParticipantOverlayAction int

const (
	ParticipantOverlayNone ParticipantOverlayAction = iota
	ParticipantOverlayMute
	ParticipantOverlayUnmute
	ParticipantOverlayKick
	ParticipantOverlayBan
)

// ParticipantOverlay is the state for the participant action popup.
type ParticipantOverlay struct {
	Visible       bool
	ParticipantID string
	IsMuted       bool
	IsServer      bool // true if selected participant is the server itself
	Cursor        int
}

// Show opens the overlay for a participant.
func (o *ParticipantOverlay) Show(participantID string, isMuted, isServer bool) {
	o.Visible = true
	o.ParticipantID = participantID
	o.IsMuted = isMuted
	o.IsServer = isServer
	o.Cursor = 0
}

// Hide closes the overlay.
func (o *ParticipantOverlay) Hide() {
	o.Visible = false
}

func (o *ParticipantOverlay) options() []struct {
	label  string
	action ParticipantOverlayAction
} {
	var opts []struct {
		label  string
		action ParticipantOverlayAction
	}

	if o.IsMuted {
		opts = append(opts, struct {
			label  string
			action ParticipantOverlayAction
		}{i18n.T("overlay_participant_unmute"), ParticipantOverlayUnmute})
	} else {
		opts = append(opts, struct {
			label  string
			action ParticipantOverlayAction
		}{i18n.T("overlay_participant_mute"), ParticipantOverlayMute})
	}

	// Server participant can only be muted, not kicked/banned
	if !o.IsServer {
		opts = append(opts,
			struct {
				label  string
				action ParticipantOverlayAction
			}{i18n.T("overlay_participant_kick"), ParticipantOverlayKick},
			struct {
				label  string
				action ParticipantOverlayAction
			}{i18n.T("overlay_participant_ban"), ParticipantOverlayBan},
		)
	}

	return opts
}

// Up moves cursor up.
func (o *ParticipantOverlay) Up() {
	if o.Cursor > 0 {
		o.Cursor--
	}
}

// Down moves cursor down.
func (o *ParticipantOverlay) Down() {
	opts := o.options()
	if o.Cursor < len(opts)-1 {
		o.Cursor++
	}
}

// Confirm returns the selected action.
func (o *ParticipantOverlay) Confirm() ParticipantOverlayAction {
	opts := o.options()
	if o.Cursor >= 0 && o.Cursor < len(opts) {
		return opts[o.Cursor].action
	}
	return ParticipantOverlayNone
}

// Render renders the overlay box.
func (o *ParticipantOverlay) Render(width, height int) string {
	opts := o.options()

	title := fmt.Sprintf(" %s ", o.ParticipantID)
	titleRendered := styles.SetupColumnTitle.Render(title)

	lines := make([]string, 0, 2+len(opts)+2)
	lines = append(lines, titleRendered, "")

	for i, opt := range opts {
		cursor := "  "
		if i == o.Cursor {
			cursor = styles.SelectedItem.Render(styles.CursorGlyph + " ")
		}

		label := opt.label
		// Color destructive actions
		switch opt.action {
		case ParticipantOverlayKick:
			label = styles.StateConnecting.Render(label)
		case ParticipantOverlayBan:
			label = styles.StateDisconnected.Render(label)
		case ParticipantOverlayMute:
			label = styles.StatLabel.Render(label)
		case ParticipantOverlayUnmute:
			label = styles.StatValueGood.Render(label)
		}

		lines = append(lines, cursor+label)
	}

	lines = append(lines, "", styles.SetupDimValue.Render(i18n.T("overlay_participant_help")))

	content := strings.Join(lines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241")).
		Padding(1, 2).
		Width(30)

	box := boxStyle.Render(content)

	// Center on screen
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	padX := (width - boxW) / 2
	padY := (height - boxH) / 2
	if padX < 0 {
		padX = 0
	}
	if padY < 0 {
		padY = 0
	}

	var result strings.Builder
	for i := 0; i < padY; i++ {
		result.WriteString("\n")
	}
	for _, line := range strings.Split(box, "\n") {
		result.WriteString(strings.Repeat(" ", padX))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
