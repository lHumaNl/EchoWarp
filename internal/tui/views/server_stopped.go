package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ServerStoppedParams contains the parameters for rendering the server-stopped screen.
type ServerStoppedParams struct {
	AutoReconnect  bool
	Attempt        int
	MaxAttempts    int    // 0 = infinite
	Countdown      int    // seconds until next probe
	Probing        bool   // probe in progress
	LastError      string // last probe error message (empty if none)
	SelectedButton int    // 0=Reconnect, 1=Settings, 2=Quit
	Width          int
	Height         int
}

// ServerStoppedView renders the idle/auto-reconnect view shown after the server
// sends ActionStop (graceful shutdown).
func ServerStoppedView(p ServerStoppedParams) string {
	var b strings.Builder

	title := styles.ServerStoppedTitle.Render("Server has shut down.")
	b.WriteString("\n  " + title + "\n")

	if p.AutoReconnect && p.Probing {
		b.WriteString("\n  " + styles.ServerStoppedHint.Render("Probing server...") + "\n")
	} else if p.AutoReconnect {
		attemptStr := fmt.Sprintf("attempt %d", p.Attempt)
		if p.MaxAttempts > 0 {
			attemptStr = fmt.Sprintf("attempt %d/%d", p.Attempt, p.MaxAttempts)
		}
		countdownStr := styles.ReconnectCountdown.Render(fmt.Sprintf("%ds", p.Countdown))
		b.WriteString("\n  " + styles.ServerStoppedHint.Render(
			fmt.Sprintf("Waiting for server... (%s, next in %s)", attemptStr, countdownStr),
		) + "\n")
		if p.LastError != "" {
			b.WriteString("  " + styles.ServerStoppedHint.Render("Last error: "+p.LastError) + "\n")
		}
	} else if p.MaxAttempts > 0 && p.Attempt >= p.MaxAttempts {
		b.WriteString("\n  " + styles.ServerStoppedHint.Render("Max reconnect attempts reached.") + "\n")
	}

	b.WriteString("\n  " + renderButtons([]string{"Reconnect", "Settings", "Quit"}, p.SelectedButton) + "\n")
	return b.String()
}

// CriticalChangesOverlayParams contains parameters for rendering the critical changes overlay.
type CriticalChangesOverlayParams struct {
	Changes []ParamChange
	Width   int
	Height  int
}

// CriticalChangesOverlay renders a bordered overlay listing all critical parameter changes.
func CriticalChangesOverlay(p CriticalChangesOverlayParams) string {
	_ = p.Height // reserved for future use
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("Server settings changed:"))
	b.WriteString("\n")
	for _, c := range p.Changes {
		line := fmt.Sprintf("  • %s: %s → %s", c.Field, c.OldValue, c.NewValue)
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.ServerStoppedHint.Render("Enter: open setup    Esc: quit"))

	maxW := p.Width - 8
	if maxW < 30 {
		maxW = 30
	}
	return styles.OverlayBorderDanger.MaxWidth(maxW).Render(b.String())
}
