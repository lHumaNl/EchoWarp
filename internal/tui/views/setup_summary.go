package views

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// SummaryOverlay shows the full launch configuration before starting.
type SummaryOverlay struct {
	Config     config.Config
	DeviceName string
	Copied     bool // clipboard feedback
}

// NewSummaryOverlay creates a summary overlay from the current config and device.
func NewSummaryOverlay(cfg config.Config, deviceName string) *SummaryOverlay {
	return &SummaryOverlay{
		Config:     cfg,
		DeviceName: deviceName,
	}
}

// Update handles key events. Returns whether the overlay should close, and optional cmd.
func (s *SummaryOverlay) Update(msg tea.KeyMsg) (launch bool, closed bool, cmd tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return false, true, nil
	case tea.KeyEnter:
		return true, true, nil
	case tea.KeyCtrlY:
		clipCmd := BuildCLICommand(s.Config, true)
		copyToClipboard(clipCmd)
		s.Copied = true
		flashCmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg { return FlashDismissMsg{} })
		return false, false, flashCmd
	}
	return false, false, nil
}

// View renders the summary overlay.
func (s *SummaryOverlay) View(width int) string {
	overlayW := width - 4
	if overlayW > 60 {
		overlayW = 60
	}
	if overlayW < 40 {
		overlayW = 40
	}

	cfg := s.Config
	title := styles.SetupColumnTitle.Render("Launch summary")

	labelW := 14
	renderLine := func(label, value string) string {
		return styles.ConnParamLabel.Width(labelW).Render(label+":") + " " + styles.ConnParamValue.Render(value)
	}

	// Mode
	modeStr := "server → client"
	if cfg.Mode == config.ModeClient {
		modeStr = "client → server"
	}
	modeType := cfg.AudioMode()

	var lines []string
	if cfg.Duplex {
		lines = append(lines, renderLine("Mode", "server ↔ client (duplex)"))
	} else {
		lines = append(lines, renderLine("Mode", modeStr+" ("+modeType+")"))
	}
	// Adaptive device display: multi-device or single
	captureDevs := cfg.CaptureDevices()
	playbackDevs := cfg.PlaybackDevices()
	if len(captureDevs)+len(playbackDevs) > 1 {
		if len(captureDevs) > 0 {
			names := make([]string, len(captureDevs))
			for i, d := range captureDevs {
				names[i] = fmt.Sprintf("#%d", d.ID)
				if d.Volume != 1.0 {
					names[i] += fmt.Sprintf(" (%.0f%%)", d.Volume*100)
				}
			}
			lines = append(lines, renderLine("Capture", strings.Join(names, ", ")))
		}
		if len(playbackDevs) > 0 {
			names := make([]string, len(playbackDevs))
			for i, d := range playbackDevs {
				names[i] = fmt.Sprintf("#%d", d.ID)
			}
			lines = append(lines, renderLine("Playback", strings.Join(names, ", ")))
		}
	} else {
		lines = append(lines, renderLine("Device", s.DeviceName))
	}

	if cfg.Mode == config.ModeClient && cfg.Address != "" {
		lines = append(lines, renderLine("Server", cfg.Address))
	}

	lines = append(lines, renderLine("Port", fmt.Sprintf("%d", cfg.Port)))

	tlsStr := "off"
	if cfg.IsTLSEnabled() {
		tlsStr = "on"
	}
	if cfg.TLSInsecure {
		tlsStr = "insecure"
	}
	lines = append(lines, renderLine("TLS", tlsStr))

	pwStr := "(none)"
	if cfg.Password != "" {
		pwStr = "set (●●●●)"
	}
	lines = append(lines, renderLine("Password", pwStr))

	if cfg.Mode == config.ModeServer {
		lines = append(lines, renderLine("Clients", fmt.Sprintf("max %d", cfg.MaxClients)))
	}

	chStr := "mono"
	if cfg.Channels > 1 {
		chStr = "stereo"
	}
	lines = append(lines,
		renderLine("Sample", fmt.Sprintf("%d Hz %s", cfg.SampleRate, chStr)),
		renderLine("Opus", fmt.Sprintf("%d kbps", cfg.OpusBitrate/1000)),
	)

	bufFrames := cfg.EffectiveAudioBufferFrames()
	bufMs := bufFrames * 20
	bufLabel := fmt.Sprintf("%d frames (%dms)", bufFrames, bufMs)
	if cfg.Duplex && bufFrames < cfg.AudioBufferFrames {
		bufLabel += " (auto-reduced for duplex)"
	}
	lines = append(lines, renderLine("Buffer", bufLabel))

	// Bandwidth estimation
	bitrateKbps := cfg.OpusBitrate / 1000
	overheadKbps := 10 // RTP/WebRTC overhead ~10kbps
	perStreamKbps := bitrateKbps + overheadKbps
	if cfg.Duplex {
		lines = append(lines, renderLine("Bandwidth", fmt.Sprintf("↑ %d kbps  ↓ %d kbps (duplex)", perStreamKbps, perStreamKbps)))
	} else {
		dir := "↑"
		if (!cfg.Reverse && cfg.Mode == config.ModeClient) || (cfg.Reverse && cfg.Mode == config.ModeServer) {
			dir = "↓"
		}
		lines = append(lines, renderLine("Bandwidth", fmt.Sprintf("%s ~%d kbps", dir, perStreamKbps)))
	}

	logLvl := cfg.LogLevel
	if logLvl == "" {
		logLvl = "info"
	}
	lines = append(lines, renderLine("Log level", logLvl))

	footer := styles.SetupDimValue.Render("enter: start  esc: back  c: copy cmd")
	if s.Copied {
		footer = styles.StatValueGood.Render("Copied to clipboard ✓")
	}

	content := title + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(overlayW)

	return border.Render(content)
}

// BuildCLICommand generates the CLI command equivalent of the config.
func BuildCLICommand(cfg config.Config, maskPassword bool) string {
	var parts []string

	if cfg.Mode == config.ModeServer {
		parts = append(parts, "echowarp server")
	} else {
		parts = append(parts, "echowarp client")
	}

	if cfg.Mode == config.ModeClient && cfg.Address != "" {
		parts = append(parts, "--address", cfg.Address)
	}

	if cfg.Port != 4415 {
		parts = append(parts, "--port", fmt.Sprintf("%d", cfg.Port))
	}

	if cfg.Password != "" {
		pw := "\"***\""
		if !maskPassword {
			pw = fmt.Sprintf("%q", cfg.Password)
		}
		parts = append(parts, "--password", pw)
	}

	if cfg.Mode == config.ModeServer {
		if cfg.TLSCert != "" {
			parts = append(parts, "--tls-cert", cfg.TLSCert)
		}
		if cfg.TLSKey != "" {
			parts = append(parts, "--tls-key", cfg.TLSKey)
		}
	} else if cfg.TLS {
		parts = append(parts, "--tls")
		if cfg.TLSInsecure {
			parts = append(parts, "--tls-insecure")
		}
	}

	if cfg.Duplex {
		parts = append(parts, "--duplex")
	} else if cfg.Reverse {
		parts = append(parts, "--reverse")
	}

	// Multi-device flags
	captureDevs := cfg.CaptureDevices()
	playbackDevs := cfg.PlaybackDevices()
	for _, d := range captureDevs {
		parts = append(parts, "--capture-device", fmt.Sprintf("%d", d.ID))
	}
	for _, d := range playbackDevs {
		parts = append(parts, "--playback-device", fmt.Sprintf("%d", d.ID))
	}

	if cfg.Mode == config.ModeServer && cfg.MaxClients != 1 {
		parts = append(parts, "--max-clients", fmt.Sprintf("%d", cfg.MaxClients))
	}

	if cfg.SampleRate != 0 && cfg.SampleRate != 48000 {
		parts = append(parts, "--sample-rate", fmt.Sprintf("%d", cfg.SampleRate))
	}

	if cfg.Channels > 1 {
		parts = append(parts, "--channels", "2")
	}

	if cfg.MaxReconnectAttempts != 0 && cfg.MaxReconnectAttempts != 5 {
		parts = append(parts, "--max-reconnect", fmt.Sprintf("%d", cfg.MaxReconnectAttempts))
	}

	if cfg.AudioBufferFrames != 0 && cfg.AudioBufferFrames != 5 {
		parts = append(parts, "--audio-buffer-frames", fmt.Sprintf("%d", cfg.AudioBufferFrames))
	}

	if cfg.LogLevel != "" && cfg.LogLevel != "info" {
		parts = append(parts, "--log-level", cfg.LogLevel)
	}

	return strings.Join(parts, " ")
}

// copyToClipboard attempts to copy text to the system clipboard.
func copyToClipboard(text string) {
	copyCommands := []struct {
		name string
		args []string
	}{
		{"pbcopy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
	}

	for _, c := range copyCommands {
		if _, err := exec.LookPath(c.name); err == nil {
			cmd := exec.CommandContext(context.Background(), c.name, c.args...)
			cmd.Stdin = strings.NewReader(text)
			_ = cmd.Run() //nolint:errcheck
			return
		}
	}
}
