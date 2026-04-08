// Package tui implements the terminal user interface for EchoWarp using Bubble Tea.
// It provides an interactive device selection screen and streaming status display.
package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// StartFunc is called after device selection; it launches the streaming goroutine.
// It receives a stopCh that will be closed when the user requests quit (Ctrl+Q).
// The backend should listen on stopCh, send a graceful stop message, then shut down.
type StartFunc func(cfg config.Config, stopCh <-chan struct{}) (<-chan transport.ConnectionStats, <-chan error, <-chan struct{})

// maxLogs is the maximum number of log lines retained in the TUI model.
const maxLogs = 200

// maxHistoryLen is the number of data points kept for sparkline history (1/sec × 60s).
const maxHistoryLen = 60

// tickInterval is how often the header timer updates.
const tickInterval = time.Second

// spectrumTickInterval is how often the spectrum display updates (~10 fps).
const spectrumTickInterval = 100 * time.Millisecond

// Model represents the TUI application state.
type Model struct {
	screen         Screen
	config         config.Config
	devices        []audio.AudioDevice
	selectedDevice *audio.AudioDevice
	deviceList     list.Model
	spinner        spinner.Model
	stats          transport.ConnectionStats
	err            error
	quitting       bool
	width          int
	height         int
	startTime      time.Time
	paused         bool
	aecActive      bool               // AEC enabled at runtime (toggled via Ctrl+E)
	aecToggleFn    func(enabled bool) // callback to toggle AEC in the audio pipeline
	startFunc      StartFunc
	statsCh        <-chan transport.ConnectionStats
	errCh          <-chan error
	logCh          <-chan LogEntry
	logs           []string // pre-formatted lines, capped at maxLogs

	// Layout & display
	logsVisible     bool
	logScrollOffset int // 0 = auto-scroll, >0 = scrolled up N lines
	deviceName      string

	// Setup screen
	setupModel views.SetupModel

	// Error auto-dismiss timer (UX-11)
	errorTimer time.Time

	// Reconnect tracking
	reconnectCount   int
	reconnectAttempt int // current attempt (0 = first connect)

	// Spectrum analyzer function for playback/decode path (returns frequency band magnitudes, nil if unavailable)
	spectrumFunc  func() []float64
	spectrumBands []float64 // cached bands for current render

	// VU level meter function for playback/decode path (returns per-channel levels, nil if unavailable)
	levelFunc func() []float64
	vuLevels  []float64 // cached levels for current render

	// Capture spectrum analyzer function (returns frequency band magnitudes for capture/outgoing audio)
	captureSpectrumFunc  func() []float64
	captureSpectrumBands []float64 // cached capture bands for current render

	// Capture VU level meter function (returns per-channel levels for capture/outgoing audio)
	captureLevelFunc func() []float64
	captureVULevels  []float64 // cached capture levels for current render

	// Sparkline history (ring buffers, 1 sample/sec)
	jitterHistory []float64
	rttHistory    []float64

	// Summary accumulation
	totalBytesSent   uint64
	totalBytesRecv   uint64
	totalPacketsLost uint32
	jitterSum        float64
	rttSum           float64
	statsCount       int
	logFile          string

	// Instantaneous bitrate (EMA-smoothed)
	prevBytesSent uint64
	prevBytesRecv uint64
	bitrateUp     float64 // kbps, smoothed
	bitrateDown   float64 // kbps, smoothed

	// Multi-client mode
	multiClient      bool
	multiStats       transport.MultiClientStats
	multiStatsCh     <-chan transport.MultiClientStats
	selectedClient   int    // index in client table
	selectedClientID string // ClientID of the selected client (stable across updates)
	cmdCh            chan<- ClientCommand
	bannedIPs        []string
	banListFn        func() []string // returns current banned IPs

	// Per-client history maps (keyed by ClientID)
	perClientJitterHistory map[string][]float64
	perClientRTTHistory    map[string][]float64
	perClientBitrateUp     map[string]float64
	perClientBitrateDown   map[string]float64
	perClientPrevBytesSent map[string]uint64
	perClientPrevBytesRecv map[string]uint64

	// Participants sidebar (client-side, from ChatActionParticipants)
	participantsCh <-chan app.ChatParticipantsPayload
	participants   []string // current online participant nicknames
	maxClients     int      // session max_clients (from participants payload)

	// Pause visibility: tracks which remote participants have paused capture.
	pausedParticipants map[string]bool
	participantPauseCh <-chan app.ParticipantPauseMsg
	conferencePartsCh  <-chan app.ConferenceParticipantsMsg

	// Overlay
	overlay          Overlay
	overlaySelection int

	// Client popup menu
	popupItems         []views.PopupMenuItem
	popupSelectedIndex int
	popupClientID      string
	popupClientNick    string

	// Kick overlay
	kickClientID      string
	kickClientNick    string
	kickReasons       []string
	kickRecentStart   int
	kickCustomStart   int
	kickSelectedIndex int
	kickCustomText    string
	kickCustomEditing bool
	kickFocusButton   int // 0=list, 1=[Kick], 2=[Cancel]

	// Ban overlay
	banClientID        string
	banClientNick      string
	banClientIP        string
	banClientHWID      string
	banCriteriaIP      bool
	banCriteriaNick    bool
	banCriteriaHWID    bool
	banFocusSection    int // 0=criteria, 1=reasons, 2=buttons
	banCriteriaIndex   int // highlighted criterion (0=IP, 1=Nick, 2=HWID)
	banReasons         []string
	banRecentStart     int
	banCustomStart     int
	banSelectedReason  int
	banCustomText      string
	banCustomEditing   bool
	banButtonFocus     int // 0=[Confirm], 1=[Cancel]
	banValidationError string

	// Device controls (per-device mute/volume in streaming screen)
	deviceCmdCh     chan<- DeviceCommand
	deviceStates    []DeviceState // current device states for display
	selectedDevice2 int           // selected device index in device panel (0-based)
	globalMuted     bool
	deviceChangeCh  <-chan DeviceChangeMsg

	// Device overlay state (Ctrl+D)
	deviceOverlaySection int // 0=input, 1=output
	deviceOverlayIndex   int // selected device index within current section

	// Conference mode
	conference         bool
	conferenceStatsCh  <-chan ConferenceStatsPayload
	conferenceStates   []audio.ParticipantState
	participantCmdCh   chan<- ParticipantCommand
	serverMuted        bool
	muteState          *MuteState  // per-participant and mute-all tracking
	serverMuteCh       chan<- bool // sends mute toggle to client app (non-conference mode)
	pauseCh            chan<- bool // sends pause toggle to client app (true=pause, false=resume)
	pauseState         *PauseState // per-device pause tracking (nil if no capture devices)
	allPausedNotified  bool        // tracks whether ActionPause was sent (to avoid duplicate sends)
	recordingOverlay   views.RecordingOverlay
	participantOverlay views.ParticipantOverlay

	// Flash notification (auto-clears after 2s)
	flashMsg   string
	flashTimer time.Time

	// Chat panel
	chatPanel      views.ChatPanel
	chatMsgCh      <-chan app.ChatMessage
	chatSendFn     func(text, to string) // callback to send chat message via app layer
	chatNicknameCh <-chan string         // receives server-assigned nickname after auth

	// Recording state
	isRecording    bool
	recordingMode  views.RecordingMode
	recordingStart time.Time
	recordingCmdCh chan<- RecordingCommand // sends start/stop commands to app layer
	recordingDir   string                  // recording output directory
	recordingFile  string                  // primary recording file name
	recordingSize  uint64                  // total recording size in bytes

	// Graceful shutdown: closed when user presses Ctrl+Q.
	stopCh   chan struct{}
	stopOnce *sync.Once

	// Pre-start probe (client mode: probe before streaming)
	pendingStartCfg *config.Config // non-nil while pre-start probe is in flight

	// Server-stopped reconnect state
	serverStoppedCh       <-chan struct{}     // closed when ActionStop received
	autoReconnect         bool                // from config
	autoReconnectAttempts int                 // from config (0 = infinite)
	reconnectAttemptGS    int                 // current attempt counter (graceful shutdown)
	reconnectCountdown    int                 // seconds until next probe (10..0)
	reconnectProbing      bool                // true while probe is in progress
	criticalChanges       []views.ParamChange // non-empty = overlay visible
	savedCfg              config.Config       // config snapshot for comparison
	reconnectLastError    string              // last probe error message

	// Kicked/banned by server state (client-side only).
	kickedReason    string   // reason provided by server for kick
	bannedReason    string   // reason provided by server for ban
	bannedCriteria  []string // ban criteria (IP, Nickname, HWID)
	kickBanBtnFocus int      // focused button index on kicked/banned screen

	// Guard to prevent multiple spectrum tick chains running simultaneously.
	spectrumTickActive bool

	// Focus area: which section receives arrow key input.
	focusedArea FocusArea
}

// Screen represents the current TUI screen being displayed.
type Screen int

// Screen constants define the possible TUI states.
const (
	ScreenDeviceSelect Screen = iota
	ScreenConnection
	ScreenStreaming
	ScreenServerStopped
	ScreenKicked
	ScreenBanned
)

// FocusArea represents which TUI area currently has keyboard focus.
type FocusArea int

const (
	FocusClientList FocusArea = iota
	FocusChat
	FocusLogs
)

// Overlay represents a modal overlay on top of the main screen.
type Overlay int

const (
	OverlayNone Overlay = iota
	OverlayBanList
	OverlayClientPopup
	OverlayKick
	OverlayBan
	OverlayDevice
)

type deviceItem struct {
	device audio.AudioDevice
}

func (d deviceItem) Title() string {
	return d.device.Name
}

func (d deviceItem) Description() string {
	direction := "Output"
	if d.device.IsInput {
		direction = "Input"
	}
	if strings.Contains(d.device.Name, "[loopback]") {
		direction = "Loopback via BlackHole"
	}
	return fmt.Sprintf("%s | Channels: %d | Rate: %d", direction, d.device.Channels, d.device.SampleRate)
}

func (d deviceItem) FilterValue() string {
	return d.device.Name
}

func (d deviceItem) DeviceID() uint32 {
	return d.device.ID
}

func (d deviceItem) IsInputDevice() bool {
	return d.device.IsInput
}

func (d deviceItem) DeviceChannels() uint32 {
	return d.device.Channels
}

func (d deviceItem) DeviceSampleRate() uint32 {
	return d.device.SampleRate
}

func (d deviceItem) DeviceBitDepth() uint32 {
	return d.device.BitDepth
}

var _ list.DefaultItem = deviceItem{}

// WithStartFunc attaches a StartFunc that will be called after device selection.
func (m Model) WithStartFunc(f StartFunc) Model {
	m.startFunc = f
	return m
}

// WithSpectrumFunc sets a function that returns current FFT spectrum bands.
func (m Model) WithSpectrumFunc(f func() []float64) Model {
	m.spectrumFunc = f
	return m
}

// WithLevelFunc sets a function that returns current per-channel VU levels.
func (m Model) WithLevelFunc(f func() []float64) Model {
	m.levelFunc = f
	return m
}

// WithCaptureSpectrumFunc sets a function that returns current FFT spectrum bands for the capture path.
func (m Model) WithCaptureSpectrumFunc(f func() []float64) Model {
	m.captureSpectrumFunc = f
	return m
}

// WithCaptureLevelFunc sets a function that returns current per-channel VU levels for the capture path.
func (m Model) WithCaptureLevelFunc(f func() []float64) Model {
	m.captureLevelFunc = f
	return m
}

// WithLogChannel attaches a log channel so the TUI can display live log entries.
func (m Model) WithLogChannel(ch <-chan LogEntry) Model {
	m.logCh = ch
	return m
}

// WithMultiClient configures the model for multi-client mode.
func (m Model) WithMultiClient(multiStatsCh <-chan transport.MultiClientStats) Model {
	m.multiClient = true
	m.multiStatsCh = multiStatsCh
	return m
}

// WithSingleClientStats wires the multi-client stats channel for a single-client server.
// Unlike WithMultiClient, it does NOT set m.multiClient = true, so the TUI stays in
// single-client layout while still receiving client info for popup/hotkey commands.
func (m Model) WithSingleClientStats(ch <-chan transport.MultiClientStats) Model {
	m.multiStatsCh = ch
	return m
}

// WithCommandChannel sets the channel for sending commands to the backend.
func (m Model) WithCommandChannel(ch chan<- ClientCommand) Model {
	m.cmdCh = ch
	return m
}

// WithBanListFunc sets a function that returns the current banned IPs list.
func (m Model) WithBanListFunc(fn func() []string) Model {
	m.banListFn = fn
	return m
}

// WithDeviceCommandChannel sets the channel for sending device control commands.
func (m Model) WithDeviceCommandChannel(ch chan<- DeviceCommand) Model {
	m.deviceCmdCh = ch
	return m
}

// WithServerStoppedChannel sets the channel that will be closed when the server
// sends ActionStop. The TUI listens on this to transition to screenServerStopped.
func (m Model) WithServerStoppedChannel(ch <-chan struct{}) Model {
	m.serverStoppedCh = ch
	return m
}

// WithAECToggle sets the callback invoked when the user toggles AEC during streaming.
func (m Model) WithAECToggle(fn func(enabled bool)) Model {
	m.aecToggleFn = fn
	return m
}

// WithDeviceStates sets the initial device states for display.
// Also initializes pauseState for capture devices (Role == "capture").
func (m Model) WithDeviceStates(states []DeviceState) Model {
	m.deviceStates = states
	var captureIDs []string
	for _, ds := range states {
		if ds.Role == "capture" {
			captureIDs = append(captureIDs, fmt.Sprintf("%d", ds.ID))
		}
	}
	if len(captureIDs) > 0 {
		m.pauseState = NewPauseState(captureIDs)
	}
	return m
}

// WithDeviceChanges sets the channel for receiving device hot-plug events.
func (m Model) WithDeviceChanges(ch <-chan DeviceChangeMsg) Model {
	m.deviceChangeCh = ch
	return m
}

// WithServerMuteChannel sets the channel for sending server mute toggle requests to the client app.
func (m Model) WithServerMuteChannel(ch chan<- bool) Model {
	m.serverMuteCh = ch
	return m
}

// WithPauseChannel sets the channel for sending pause toggle requests to the client app.
func (m Model) WithPauseChannel(ch chan<- bool) Model {
	m.pauseCh = ch
	return m
}

// WithConference configures conference mode with stats and command channels.
func (m Model) WithConference(statsCh <-chan ConferenceStatsPayload, cmdCh chan<- ParticipantCommand, serverMuted bool) Model {
	m.conference = true
	m.conferenceStatsCh = statsCh
	m.participantCmdCh = cmdCh
	m.serverMuted = serverMuted
	m.muteState = NewMuteState()
	return m
}

// WithRecordingCommands sets the channel for sending recording start/stop commands to the app layer.
func (m Model) WithRecordingCommands(ch chan<- RecordingCommand) Model {
	m.recordingCmdCh = ch
	return m
}

// WithParticipantsChannel sets the channel for receiving participant list updates from the chat layer.
func (m Model) WithParticipantsChannel(ch <-chan app.ChatParticipantsPayload) Model {
	m.participantsCh = ch
	return m
}

// WithChatChannel sets the channel for receiving chat messages from the app layer.
func (m Model) WithChatChannel(ch <-chan app.ChatMessage) Model {
	m.chatMsgCh = ch
	return m
}

// WithChatSendFunc sets the callback for sending chat messages to the app layer.
func (m Model) WithChatSendFunc(fn func(text, to string)) Model {
	m.chatSendFn = fn
	return m
}

// WithParticipantPauseChannel sets the channel for receiving participant pause state changes.
func (m Model) WithParticipantPauseChannel(ch <-chan app.ParticipantPauseMsg) Model {
	m.participantPauseCh = ch
	m.pausedParticipants = make(map[string]bool)
	return m
}

// WithConferenceParticipantsChannel sets the channel for receiving full participant list updates.
func (m Model) WithConferenceParticipantsChannel(ch <-chan app.ConferenceParticipantsMsg) Model {
	m.conferencePartsCh = ch
	if m.pausedParticipants == nil {
		m.pausedParticipants = make(map[string]bool)
	}
	return m
}

// WithChatNicknameChannel sets the channel for receiving the server-assigned nickname.
func (m Model) WithChatNicknameChannel(ch <-chan string) Model {
	m.chatNicknameCh = ch
	return m
}

// SetMultiClientChannels pre-wires multi-client and conference channels without
// enabling the modes. The modes are enabled dynamically in SetupDoneMsg based on
// the user's config selection. This is needed for TUI-driven flows where the user
// selects conference/multi-client mode interactively.
func (m *Model) SetMultiClientChannels(
	multiStatsCh <-chan transport.MultiClientStats,
	conferenceStatsCh <-chan ConferenceStatsPayload,
	participantCmdCh chan<- ParticipantCommand,
	recordingCmdCh chan<- RecordingCommand,
) {
	m.multiStatsCh = multiStatsCh
	m.conferenceStatsCh = conferenceStatsCh
	m.participantCmdCh = participantCmdCh
	m.recordingCmdCh = recordingCmdCh
}

// WithDeviceEnumerator sets a device enumerator for refreshing the device list
// after creating/removing virtual audio sinks.
func (m Model) WithDeviceEnumerator(enum audio.DeviceEnumerator) Model {
	m.setupModel = m.setupModel.WithDeviceRefreshFunc(func() ([]list.Item, error) {
		inputs, err := enum.ListInputDevices()
		if err != nil {
			return nil, err
		}
		outputs, err := enum.ListOutputDevices()
		if err != nil {
			return nil, err
		}
		all := make([]list.Item, 0, len(inputs)+len(outputs))
		for _, dev := range inputs {
			all = append(all, deviceItem{device: dev})
		}
		for _, dev := range outputs {
			all = append(all, deviceItem{device: dev})
		}
		return all, nil
	})
	return m
}

// WithLogFile sets the log file path for display in the exit summary.
func (m Model) WithLogFile(path string) Model {
	m.logFile = path
	return m
}

// NewModel creates a new TUI model with the given configuration and device list.
func NewModel(cfg config.Config, devices []audio.AudioDevice) Model {
	return NewModelWithOutputDevices(cfg, devices, nil)
}

// NewModelWithOutputDevices creates a TUI model with separate input and output device lists (for duplex).
func NewModelWithOutputDevices(cfg config.Config, devices []audio.AudioDevice, _ []audio.AudioDevice) Model {
	items := make([]list.Item, len(devices))
	for i, dev := range devices {
		items[i] = deviceItem{device: dev}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.SelectedItem
	delegate.Styles.NormalTitle = lipgloss.NewStyle()

	deviceList := list.New(items, delegate, 0, 0)
	deviceList.Title = "Select Audio Device"
	deviceList.Styles.Title = styles.Header
	deviceList.SetShowStatusBar(false)
	deviceList.SetFilteringEnabled(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.ActiveSpinner

	initialScreen := ScreenConnection
	if cfg.DeviceID == nil {
		initialScreen = ScreenDeviceSelect
	}

	// Determine if input device based on mode (used as fallback hint).
	isInput := cfg.Duplex || (cfg.Mode == config.ModeServer && !cfg.Reverse) || (cfg.Mode != config.ModeServer && cfg.Reverse)

	setupModel := views.NewSetupModel(cfg, deviceList, isInput, 80, 24)

	// Always use unified multi-select device list — all devices (input + output) shown regardless of mode.
	// In duplex: Space cycles [C]/[P]/[C+P]. In normal/reverse: Space toggles [✓].
	setupModel = setupModel.WithUnifiedDeviceList(cfg.Duplex)

	// Determine chat nickname: server is always "Server", client uses config or empty.
	chatNick := cfg.Nickname
	if cfg.Mode == config.ModeServer {
		chatNick = "Server"
	}

	m := Model{
		screen:      initialScreen,
		config:      cfg,
		devices:     devices,
		deviceList:  deviceList,
		spinner:     sp,
		setupModel:  setupModel,
		chatPanel:   views.NewChatPanel(chatNick),
		width:       80,
		height:      24,
		logsVisible: true,
	}
	// Chat is visible by default; Ctrl+T toggles it.
	m.chatPanel.ToggleVisible()
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{doTick(), doSpectrumTick()}

	// Start mDNS discovery for client mode setup
	if m.screen == ScreenDeviceSelect {
		if initCmd := m.setupModel.InitCmd(); initCmd != nil {
			cmds = append(cmds, initCmd)
		}
	}

	if m.screen == ScreenConnection {
		cmds = append(cmds, m.spinner.Tick)
		if m.startFunc != nil && m.config.DeviceID != nil {
			cfg := m.config
			startFunc := m.startFunc
			m.stopCh = make(chan struct{})
			m.stopOnce = &sync.Once{} //nolint:govet,staticcheck // stopOnce is used in other methods
			stopCh := m.stopCh
			cmds = append(cmds, func() tea.Msg {
				statsCh, errCh, srvStoppedCh := startFunc(cfg, stopCh)
				return streamingStartedMsg{statsCh: statsCh, errCh: errCh, serverStoppedCh: srvStoppedCh}
			})
			if m.logCh != nil {
				cmds = append(cmds, waitForLog(m.logCh))
			}
		}
	}

	return tea.Batch(cmds...)
}
