// Package views provides individual TUI screen components for EchoWarp.
package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// ConnectionParams contains all parameters needed to render the connection screen body.
type ConnectionParams struct {
	Spinner          spinner.Model
	IsServer         bool
	Address          string
	Port             int
	Reverse          bool
	Duplex           bool
	Conference       bool
	TLSEnabled       bool
	TLSSelfSigned    bool
	DeviceName       string
	Logs             []string
	ReconnectAttempt int
	MaxAttempts      int
	Width            int
	Height           int
}

// ConnectionView renders the connection/waiting screen body (without header/status bar).
// Shows spinner, session params, and early logs. No centering, no box — content starts at top.
func ConnectionView(p ConnectionParams) string {
	var sections []string

	// Spinner + main message
	spinText := styles.ConnSpinner.Render(p.Spinner.View())
	mainMsg := buildMainMessage(p.IsServer, p.Address, p.Port)

	if p.ReconnectAttempt > 0 {
		attemptInfo := fmt.Sprintf(" (attempt %d", p.ReconnectAttempt)
		if p.MaxAttempts > 0 {
			attemptInfo += fmt.Sprintf("/%d", p.MaxAttempts)
		}
		attemptInfo += ")"
		if p.IsServer {
			mainMsg = fmt.Sprintf("Reconnecting on :%d...%s", p.Port, attemptInfo)
		} else {
			mainMsg = fmt.Sprintf("Reconnecting to %s:%d...%s", p.Address, p.Port, attemptInfo)
		}
	}

	// Session params
	sections = append(sections,
		styles.ConnMessage.Render(fmt.Sprintf("  %s %s", spinText, mainMsg)),
		renderParam("Direction:", buildModeTextFull(p.Reverse, p.Duplex, p.Conference)),
	)
	if p.DeviceName != "" {
		sections = append(sections, renderParam("Device:", p.DeviceName))
	}
	securityText := "AES+DTLS (E2E encryption)"
	if p.TLSEnabled {
		if p.TLSSelfSigned {
			securityText = "TLS (self-signed)"
		} else {
			securityText = "TLS"
		}
	}
	sections = append(sections, renderParam("Security:", securityText))

	// Logs (if any)
	if len(p.Logs) > 0 {
		maxLogLines := p.Height - len(sections) - 2
		if maxLogLines < 3 {
			maxLogLines = 3
		}
		start := 0
		if len(p.Logs) > maxLogLines {
			start = len(p.Logs) - maxLogLines
		}
		panelWidth := p.Width - 2
		if panelWidth < 40 {
			panelWidth = 40
		}
		for _, line := range p.Logs[start:] {
			sections = append(sections, "  "+colorizeLogLine(line, panelWidth))
		}
	}

	return strings.Join(sections, "\n")
}

func renderParam(label, value string) string {
	return fmt.Sprintf("  %s %s",
		styles.ConnParamLabel.Width(12).Render(label),
		styles.ConnParamValue.Render(value),
	)
}

func buildMainMessage(isServer bool, address string, port int) string {
	if isServer {
		return fmt.Sprintf("Waiting for client connection on :%d...", port)
	}
	return fmt.Sprintf("Connecting to %s:%d...", address, port)
}

func buildModeText(reverse bool) string {
	if reverse {
		return "client → server (reverse)"
	}
	return "server → client (normal)"
}

// buildModeTextFull returns a direction string that also handles duplex and conference modes.
func buildModeTextFull(reverse, duplex, conference bool) string {
	if conference {
		return "conference (multi-party)"
	}
	if duplex {
		return "server ↔ client (duplex)"
	}
	return buildModeText(reverse)
}

// ConnectionUpdate handles spinner tick messages for the connection screen.
func ConnectionUpdate(spin spinner.Model, msg tea.Msg) (spinner.Model, tea.Cmd) {
	var cmd tea.Cmd
	spin, cmd = spin.Update(msg)
	return spin, cmd
}
