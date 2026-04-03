// Package styles provides consolidated Lip Gloss styling definitions for the TUI.
// All TUI styles are defined here — no inline styles in views.
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// ── Brand & Layout ──────────────────────────────────────────────────────────

var (
	// AppTitle styles the "EchoWarp" brand text in the header.
	AppTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	// ModeServer styles the "SERVER" mode badge.
	ModeServer = lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true)

	// ModeClient styles the "CLIENT" mode badge.
	ModeClient = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)

	// Version styles the version text (faint).
	Version = lipgloss.NewStyle().Faint(true)

	// Separator renders a horizontal line between sections.
	Separator = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	// StatusBarStyle styles the bottom status bar.
	StatusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Help styles help key hints.
	Help = lipgloss.NewStyle().Faint(true)

	// Header styles section headers (used by device list title).
	Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).MarginBottom(1)
)

// ── State indicators ────────────────────────────────────────────────────────

var (
	StateConnected    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	StateDisconnected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	StateConnecting   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	StateDefault      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
)

// ── Streaming stats ─────────────────────────────────────────────────────────

var (
	StatLabel      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	StatValueGood  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StatValueWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	StatValueError = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	Direction        = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	DirectionReverse = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	Paused           = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
)

// ── Logs ────────────────────────────────────────────────────────────────────

var (
	LogTime       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	LogInfo       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	LogWarn       = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	LogError      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	LogDebug      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	LogLevelInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	LogLevelWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	LogLevelError = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	LogLevelDebug = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ScrollHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Faint(true)
	FocusedLabel  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
)

// ── Device select ───────────────────────────────────────────────────────────

var (
	SelectedItem  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	ActiveSpinner = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	DeviceTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Padding(0, 1)
)

// ── Setup screen ────────────────────────────────────────────────────────────

var (
	// SetupDimValue styles default (unchanged) field values.
	SetupDimValue = lipgloss.NewStyle().Faint(true)

	// SetupCLIValue styles values from CLI flags (cyan).
	SetupCLIValue = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	// SetupAutoValue styles auto-discovered values (dim cyan).
	SetupAutoValue = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Faint(true)

	// SetupRequired styles required but empty fields (yellow underline).
	SetupRequired = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Underline(true)

	// SetupActionLabel styles action items like [Advanced ▸].
	SetupActionLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// SetupModeIndicator styles the mode badge shown under device status.
	SetupModeIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)

	// SetupReadyHint styles the "Ready to start" hint (green dim).
	SetupReadyHint = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Faint(true)

	// SetupErrorHint styles error hints (red).
	SetupErrorHint = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	// SetupColumnTitle styles the active column title (bold).
	SetupColumnTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	// SetupColumnTitleDim styles the inactive column title.
	SetupColumnTitleDim = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// SetupSeparator styles the vertical column separator.
	SetupSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// ── Error ───────────────────────────────────────────────────────────────────

var Error = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("255")).
	Background(lipgloss.Color("196")).
	Padding(0, 1)

// ── Connection screen ───────────────────────────────────────────────────────

var (
	ConnSpinner    = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	ConnMessage    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ConnParamLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ConnParamValue = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// ── Cursor ──────────────────────────────────────────────────────────────────

// CursorGlyph is the unified selection indicator used across all views.
const CursorGlyph = "▸"

// ── Table data ──────────────────────────────────────────────────────────────

// StatValueNeutral styles neutral data values in tables (not good/bad).
var StatValueNeutral = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

// ── Overlay ─────────────────────────────────────────────────────────────────

var (
	OverlayActive = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	OverlayDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	OverlayBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 2)
	OverlayBorderDanger = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("196")).
				Padding(1, 2)
)

// ── Flash notification ──────────────────────────────────────────────────────

var FlashSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

// ── Master-Detail layout ──────────────────────────────────────────────────

var (
	// ColumnSeparator styles the vertical │ between master and detail columns.
	ColumnSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// ClientListItem styles a client entry in the master list.
	ClientListItem = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// DetailHeader styles the combined "Clients N/M  ▸ Nick — IP" line.
	DetailHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)

	// AggregateTraffic styles the aggregate traffic numbers.
	AggregateTraffic = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// ParticipantBullet styles the bullet "•" in participant sidebar.
	ParticipantBullet = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// ── Empty state ─────────────────────────────────────────────────────────────

var EmptyState = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Faint(true)

// ── TLS badge ───────────────────────────────────────────────────────────────

var (
	TLSEnabled   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	TLSDisabled  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	TLSEncrypted = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// Recording indicator — red for active recording.
	RecordingIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// ── Server-stopped screen ──────────────────────────────────────────────────

var (
	// ServerStoppedTitle styles the main status text on the server-stopped screen (orange).
	ServerStoppedTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))

	// ServerStoppedHint styles hint text on the server-stopped screen (gray).
	ServerStoppedHint = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	// ReconnectCountdown styles the countdown timer value (cyan).
	ReconnectCountdown = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

// ── Chat panel ─────────────────────────────────────────────────────────────

var (
	// ChatTitle styles the "Chat" panel header.
	ChatTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

	// ChatTimestamp styles the [HH:MM] prefix.
	ChatTimestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// ChatNickSelf styles the sender's own nickname (light blue).
	ChatNickSelf = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	// ChatNickOther styles other users' nicknames (accent color).
	ChatNickOther = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	// ChatNickServer styles the "Server" nickname (bold).
	ChatNickServer = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)

	// ChatSystem styles system messages (gray, italic).
	ChatSystem = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	// ChatText styles normal message text.
	ChatText = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// ChatInputActive styles the input line border when focused.
	ChatInputActive = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	// ChatInputInactive styles the input placeholder when not focused.
	ChatInputInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Faint(true)

	// ChatNewIndicator styles the "N new messages" indicator.
	ChatNewIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// ChatDMTag styles the "DM" label in direct messages (purple/magenta).
	ChatDMTag = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)

	// ChatDMNick styles the recipient/sender nickname in DM messages.
	ChatDMNick = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))

	// ChatDMText styles the text content of direct messages.
	ChatDMText = lipgloss.NewStyle().Foreground(lipgloss.Color("219"))
)
