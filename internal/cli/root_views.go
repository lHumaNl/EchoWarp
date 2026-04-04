package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/version"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// ─── Shared styles ────────────────────────────────────────────────────────────

var (
	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(18)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	valueGoodStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	valueDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("117")).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color("238"))

	colIDStyle   = lipgloss.NewStyle().Width(5).Foreground(lipgloss.Color("226"))
	colNameStyle = lipgloss.NewStyle().Width(36).Foreground(lipgloss.Color("252"))
	colChStyle   = lipgloss.NewStyle().Width(5).Foreground(lipgloss.Color("117"))
	colRateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	cmdNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Bold(true).
			Width(16)

	cmdDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// ─── Devices view ─────────────────────────────────────────────────────────────

func renderDevicesContent(width int) string {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("Failed to initialize audio: %v", err))
	}
	defer func() { _ = dm.Close() }()

	inputs, err := dm.ListInputDevices()
	if err != nil {
		inputs = nil
	}
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		outputs = nil
	}

	panelW := width - 4
	if panelW < 50 {
		panelW = 50
	}

	var sb strings.Builder
	sb.WriteString(renderDeviceTable("Input Devices (capture)", inputs, panelW))
	sb.WriteString("\n")
	sb.WriteString(renderDeviceTable("Output Devices (playback)", outputs, panelW))

	return sb.String()
}

func renderDeviceTable(title string, devices []audio.AudioDevice, width int) string {
	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render(title) + "\n")

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		tableHeaderStyle.Width(5).Render("ID"),
		tableHeaderStyle.Width(36).Render("Name"),
		tableHeaderStyle.Width(5).Render("Ch"),
		tableHeaderStyle.Render("Sample Rate"),
	)
	sb.WriteString(header + "\n")

	if len(devices) == 0 {
		sb.WriteString(valueDimStyle.Render("  (none found)") + "\n")
	}
	for _, d := range devices {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			colIDStyle.Render(fmt.Sprintf("%d", d.ID)),
			colNameStyle.Render(truncate(d.Name, 35)),
			colChStyle.Render(fmt.Sprintf("%d", d.Channels)),
			colRateStyle.Render(fmt.Sprintf("%d Hz", d.SampleRate)),
		)
		sb.WriteString(row + "\n")
	}

	return panelStyle.Width(width).Render(sb.String()) + "\n"
}

// ─── Config view ──────────────────────────────────────────────────────────────

func renderConfigContent(cfgPath string, width int) string {
	cfg, err := config.LoadWithViper(cfgPath, config.ModeServer)
	if err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("Failed to load config: %v", err))
	}

	panelW := width - 4
	if panelW < 50 {
		panelW = 50
	}

	var sb strings.Builder

	sb.WriteString(renderConfigSection("General", [][]string{
		{"Mode", string(cfg.Mode)},
		{"Log Level", cfg.LogLevel},
		{"Log File", strOrDash(cfg.LogFile)},
	}))
	sb.WriteString("\n")

	sb.WriteString(renderConfigSection("Audio", [][]string{
		{"Sample Rate", fmt.Sprintf("%d Hz", cfg.SampleRate)},
		{"Channels", fmt.Sprintf("%d", cfg.Channels)},
		{"Reverse Mode", boolStr(cfg.Reverse)},
		{"Virtual Mic", boolStr(cfg.VirtualMic)},
		{"Buffer Frames", fmt.Sprintf("%d (%.0f ms)", cfg.AudioBufferFrames, float64(cfg.AudioBufferFrames)*20)},
		{"SIMD", boolStr(!cfg.NoSIMDOptimization)},
		{"Pool Warmup", boolStr(!cfg.NoPoolWarmup)},
	}))
	sb.WriteString("\n")

	sb.WriteString(renderConfigSection("Opus Codec", [][]string{
		{"Bitrate", fmt.Sprintf("%d bps", cfg.OpusBitrate)},
		{"Complexity", fmt.Sprintf("%d / 10", cfg.OpusComplexity)},
		{"Application", cfg.OpusApplication},
		{"DTX", boolStr(cfg.OpusDTX)},
		{"FEC", boolStr(cfg.OpusFEC)},
	}))
	sb.WriteString("\n")

	sb.WriteString(renderConfigSection("Network", [][]string{
		{"Port", fmt.Sprintf("%d", cfg.Port)},
		{"Address", strOrDash(cfg.Address)},
		{"Max Clients", fmt.Sprintf("%d", cfg.MaxClients)},
		{"Max Reconnect", fmt.Sprintf("%d", cfg.MaxReconnectAttempts)},
		{"STUN Servers", strings.Join(cfg.STUNServers, ", ")},
	}))
	sb.WriteString("\n")

	tlsStatus := "disabled"
	if cfg.TLSCert != "" {
		tlsStatus = "enabled"
	}
	sb.WriteString(renderConfigSection("Security", [][]string{
		{"TLS", tlsStatus},
		{"TLS Cert", strOrDash(cfg.TLSCert)},
		{"Password", ternaryStr(cfg.Password != "", "*** (set)", "(not set)")},
		{"Max Auth Failures", fmt.Sprintf("%d", cfg.MaxFailedAttempts)},
	}))

	_ = panelW
	return sb.String()
}

func renderConfigSection(title string, rows [][]string) string {
	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render(title) + "\n")
	for _, row := range rows {
		line := lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render(row[0]+":"),
			valueStyle.Render(row[1]),
		)
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// ─── Help view ────────────────────────────────────────────────────────────────

func renderHelpContent(width int) string {
	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render(fmt.Sprintf("EchoWarp v%s — Command Reference", version.Version)) + "\n\n")

	commands := [][]string{
		{"server", "Start as audio server"},
		{"client", "Connect to a server"},
		{"config init", "Create config file with defaults"},
		{"config show", "Show current configuration"},
		{"devices", "List audio input/output devices"},
		{"doctor", "Run audio & network diagnostics"},
		{"daemon", "Run as background daemon"},
		{"update", "Check for updates"},
		{"version", "Show version"},
		{"completion", "Generate shell completions"},
	}

	sb.WriteString(sectionTitleStyle.Render("Subcommands") + "\n")
	for _, cmd := range commands {
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left,
			cmdNameStyle.Render(cmd[0]),
			cmdDescStyle.Render(cmd[1]),
		) + "\n")
	}

	sb.WriteString("\n" + sectionTitleStyle.Render("Common Flags") + "\n")
	flags := [][]string{
		{"--config, -c", "Path to YAML config file"},
		{"--device, -d", "Audio device ID (see: echowarp devices)"},
		{"--device-name, -D", "Select device by name substring"},
		{"--password", "Authentication password"},
		{"--port, -p", "TCP port (default: 4415)"},
		{"--address, -a", "Server address (client mode)"},
		{"--reverse, -r", "Reverse audio direction (client → server)"},
		{"--duplex", "Bidirectional audio (server ↔ client)"},
		{"--conference", "Multi-user conference mode"},
		{"--discover", "Auto-discover servers on LAN"},
		{"--max-clients", "Max simultaneous clients (server)"},
		{"--tls", "Enable TLS for signaling"},
		{"--dry-run", "Validate config and exit"},
		{"--log-level", "Log verbosity: debug, info, warn, error"},
	}
	flagNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Width(24)
	flagDescStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	for _, f := range flags {
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left,
			flagNameStyle.Render(f[0]),
			flagDescStyle.Render(f[1]),
		) + "\n")
	}

	sb.WriteString("\n" + sectionTitleStyle.Render("Examples") + "\n")
	exStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	examples := []string{
		"echowarp server --password secret",
		"echowarp client --discover",
		"echowarp client --address 192.168.1.100 --duplex",
		"echowarp server --conference --max-clients 8",
		"echowarp config init",
		"echowarp doctor",
	}
	for _, ex := range examples {
		sb.WriteString(exStyle.Render("  $ "+ex) + "\n")
	}

	_ = width
	return sb.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func strOrDash(s string) string {
	if s == "" {
		return valueDimStyle.Render("—")
	}
	return s
}

func boolStr(b bool) string {
	if b {
		return valueGoodStyle.Render("yes")
	}
	return valueDimStyle.Render("no")
}

func ternaryStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
