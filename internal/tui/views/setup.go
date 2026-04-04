package views

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

// SetupColumn identifies which column has focus.
type SetupColumn int

const (
	ColumnDevices SetupColumn = iota
	ColumnSettings
)

// SetupOverlay identifies the active overlay on the setup screen.
type SetupOverlay int

const (
	SetupOverlayNone SetupOverlay = iota
	SetupOverlaySummary
	SetupOverlayVirtualDevice
	SetupOverlayRestore
	SetupOverlayConfigSave
	SetupOverlayConfigLoad
)

// SetupDoneMsg is sent when the user confirms setup and is ready to start.
type SetupDoneMsg struct {
	Config config.Config
}

// ValidationDismissMsg is sent after the validation error auto-dismiss timer fires.
type ValidationDismissMsg struct{}

// DiscoveryTickMsg is sent periodically to advance the discovery spinner animation.
type DiscoveryTickMsg struct{}

// FlashDismissMsg is sent after the flash notification auto-dismiss timer fires.
type FlashDismissMsg struct{}

// DeviceSection identifies which device section has focus in duplex mode.
type DeviceSection int

const (
	SectionInput DeviceSection = iota
	SectionOutput
)

// SetupModel holds the state for the setup screen.
type SetupModel struct {
	// Column focus
	activeColumn SetupColumn

	// Left column — device list
	DeviceList list.Model
	IsInput    bool // determines title

	// Duplex mode: second device list (output devices) — legacy, used when not unified
	OutputDeviceList list.Model
	HasOutputList    bool          // true when duplex with two device lists
	DeviceSection    DeviceSection // which device section is focused

	// Unified multi-select mode: Space toggles device selection/roles.
	// Key = device name (stable across re-enumerations), Value = assigned role(s)
	multiSelect             map[string]DeviceRoleSet
	unifiedDuplex           bool // true when using unified list (all modes)
	isDuplexMode            bool // true when duplex/conference — two checkbox columns; false — one column
	isConferenceMode        bool // true when conference — enables hub mode on server
	preConferenceMaxClients int  // saved Max clients value before conference auto-bump

	// Sectioned device list (Input/Output sections with checkbox columns)
	inputDevices  []deviceRow
	outputDevices []deviceRow
	deviceCursor  int // row index within current section
	inputScroll   int // scroll offset for input section
	outputScroll  int // scroll offset for output section

	// Right column — settings fields
	Fields       []SetupField
	FieldCursor  int
	advancedOpen bool

	// Advanced fields (shown when expanded)
	AdvancedFields []SetupField

	// Config reference for building SetupDoneMsg
	cfg config.Config

	// Input mode tracking
	inputMode InputMode

	// Dimensions
	width  int
	height int

	// Overlays
	overlay              SetupOverlay
	summaryOverlay       *SummaryOverlay
	virtualDeviceOverlay *VirtualDeviceOverlay
	restoreOverlay       *RestoreOverlay
	configSaveOverlay    *ConfigSaveOverlay
	configLoadOverlay    *ConfigLoadOverlay

	// Config save/load state
	lastLoadedConfigName string
	loadedClientModes    map[string]interface{} // modes: map from loaded client config

	// Preset restore state (client mode)
	recentServers   []recent.Server // loaded at init for preset lookup
	presetDismissed map[string]bool // per-mode dismiss flag for this session

	// Preset restore state (server mode)
	serverPresets *preset.ServerPresets // loaded at init for server preset lookup

	// Server list (client mode)
	serverList        ServerListModel
	serverListFocused bool // true when server list has focus within ColumnSettings

	// Discovery state
	discoveredServers []discovery.ServiceInfo
	discoveryScanning bool

	// Server probe state (client mode)
	probeAddr   string // "addr:port" currently being probed (for dedup)
	probeStatus string // "", "probing", "ok", "error"
	probeError  string // error message when probeStatus == "error"
	probeResult *ProbeServerResult

	// Validation error feedback (UX-1)
	validationError string
	validationTimer time.Time

	// Discovery spinner animation (UX-5)
	discoveryFrame int

	// Flash notification for clipboard copy (UX-6)
	flashMsg   string
	flashTimer time.Time

	// Pending command from auto-restore at init time (returned by InitCmd)
	pendingRestoreCmd tea.Cmd

	// Virtual mic state (Linux only)
	virtualMicCreated bool   // true when pactl sink was created this session
	virtualMicModule  string // PulseAudio module ID for cleanup

	// Mix input: maps virtual output device selectKey → set of input device selectKeys
	// whose audio should be mixed into that output.
	mixInputs map[string]map[string]bool
}

// InputMode tracks the current keyboard input routing priority.
type InputMode int

const (
	ModeNormal  InputMode = iota
	ModeEditing           // editing a field value
	ModeFilter            // device list filter active
)

// NewSetupModel creates a setup screen model pre-filled from the given config.
func NewSetupModel(cfg config.Config, deviceList list.Model, isInput bool, width, height int) SetupModel {
	fields := buildMainFields(cfg)
	advFields := buildAdvancedFields(cfg)

	m := SetupModel{
		activeColumn:    ColumnDevices,
		DeviceList:      deviceList,
		IsInput:         isInput,
		Fields:          fields,
		AdvancedFields:  advFields,
		cfg:             cfg,
		width:           width,
		height:          height,
		serverList:      NewServerListModel(width/2, height),
		presetDismissed: make(map[string]bool),
	}

	// Load recent servers (client mode only) before mDNS discovery
	if cfg.Mode == config.ModeClient {
		if recentServers, _ := recent.Load(); len(recentServers) > 0 {
			m.recentServers = recentServers
			entries := make([]ServerEntry, 0, len(recentServers))
			for _, rs := range recentServers {
				nick := rs.Hostname
				if nick == "" {
					nick = fmt.Sprintf("%s:%d", rs.Address, rs.Port)
				}
				entries = append(entries, ServerEntry{
					Address:       rs.Address,
					Port:          rs.Port,
					Hostname:      nick,
					Source:        ServerSourceRecent,
					LastConnected: rs.LastConnected,
					ProbeStatus:   ServerProbePending,
				})
			}
			m.serverList.SetEntries(entries)
			m.serverList.SetHasRecent(true)
		}
	}

	// Mark scanning state for client mode (mDNS will start in InitCmd)
	if cfg.Mode == config.ModeClient && cfg.Address == "" {
		m.discoveryScanning = true
		m.serverList.SetScanning(true)
	}

	// Client mode: start with Settings column and server list focused
	if cfg.Mode == config.ModeClient {
		m.activeColumn = ColumnSettings
		if m.serverList.HasEntries() {
			m.serverListFocused = true
		}
	}

	// Load server presets (server mode); overlay is shown later in WithUnifiedDeviceList
	// after devices are populated, so matchPresetDevices can verify device availability.
	if cfg.Mode == config.ModeServer {
		sp := preset.Load()
		m.serverPresets = &sp
	}

	// Apply field dependencies on init
	m.applyFieldDependencies()

	return m
}

// WithOutputDevices adds a second device list for output devices (duplex mode).
func (m SetupModel) WithOutputDevices(outputList list.Model) SetupModel {
	m.OutputDeviceList = outputList
	m.HasOutputList = true
	m.DeviceSection = SectionInput
	return m
}

// WithUnifiedDuplex enables unified multi-select mode for duplex.
// All devices (input + output) are in a single list; Space toggles [C]/[P] roles.
func (m SetupModel) WithUnifiedDuplex(allDevices list.Model) SetupModel {
	m.DeviceList = allDevices
	m.unifiedDuplex = true
	m.isDuplexMode = true
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.HasOutputList = false
	return m
}

// WithUnifiedDeviceList enables unified multi-select mode for all modes.
// In duplex: two checkbox columns (Capture/Playback). In normal/reverse: one column (Select).
func (m SetupModel) WithUnifiedDeviceList(isDuplex bool) SetupModel {
	m.unifiedDuplex = true
	m.isDuplexMode = isDuplex
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.HasOutputList = false
	m.DeviceSection = SectionInput
	m.deviceCursor = 0
	m.rebuildDeviceGroups()

	// For server mode, auto-restore devices now that devices are populated
	if m.cfg.Mode == config.ModeServer && m.serverPresets != nil {
		m.pendingRestoreCmd = m.tryShowServerRestoreOverlay()
	}

	return m
}

// visibleSections returns which device sections should be shown based on mode.
func (m SetupModel) visibleSections() (showInput, showOutput bool) {
	if m.isDuplexMode {
		return true, true
	}
	isCaptureSide := (m.cfg.Mode == config.ModeServer && !m.cfg.Reverse) ||
		(m.cfg.Mode == config.ModeClient && m.cfg.Reverse)
	if isCaptureSide {
		return true, false
	}
	return false, true
}

// ActiveDeviceList returns the currently focused device list.
func (m *SetupModel) ActiveDeviceList() *list.Model {
	if m.HasOutputList && m.DeviceSection == SectionOutput {
		return &m.OutputDeviceList
	}
	return &m.DeviceList
}

func buildMainFields(cfg config.Config) []SetupField {
	var fields []SetupField

	if cfg.Mode == config.ModeClient {
		addrField := NewTextField("Server address", cfg.Address, true)
		addrField.SetValidator(ValidateAddress)
		if cfg.Address != "" {
			addrField.Source = SourceCLI
		}
		fields = append(fields, addrField)
	}

	portField := NewNumberField("Port", cfg.Port, 1, 65535)
	if cfg.Port != 4415 { // non-default
		portField.Source = SourceCLI
	}
	fields = append(fields, portField)

	pw := ""
	if cfg.Password != "" {
		pw = cfg.Password
	}
	pwField := NewPasswordField("Password", pw)
	if cfg.Password != "" {
		pwField.Source = SourceCLI
	} else if cfg.Mode == config.ModeClient {
		pwField.Hint = "(server)"
		pwField.Hidden = true // hidden until probe says PasswordRequired
	}
	fields = append(fields, pwField)

	// Nickname (client only)
	if cfg.Mode == config.ModeClient {
		nickField := NewTextField("Nickname", cfg.Nickname, false)
		nickField.Hint = "(optional, max 20 chars)"
		if cfg.Nickname != "" {
			nickField.Source = SourceCLI
		}
		fields = append(fields, nickField)
	}

	if cfg.Mode == config.ModeServer {
		mcField := NewNumberField("Max clients", cfg.MaxClients, 1, 100)
		if cfg.MaxClients != 1 {
			mcField.Source = SourceCLI
		}
		fields = append(fields, mcField)
	}

	// Mode (server only — client gets mode from probe result)
	if cfg.Mode == config.ModeServer {
		modeOpts := []string{
			"normal (server → client)",
			"reverse (client → server)",
			"duplex (bidirectional)",
			"conference (multi-user)",
		}
		modeIdx := 0
		if cfg.Conference {
			modeIdx = 3
		} else if cfg.Duplex {
			modeIdx = 2
		} else if cfg.Reverse {
			modeIdx = 1
		}
		modeField := NewToggleField("Mode", modeOpts, modeIdx)
		if cfg.Reverse || cfg.Duplex || cfg.Conference {
			modeField.Source = SourceCLI
		}
		fields = append(fields, modeField)
	}

	if cfg.Mode == config.ModeServer {
		// Server: show "Max auth fail" (ban threshold) in main fields
		mafField := NewNumberField("Max auth fail", cfg.MaxFailedAttempts, 0, 100)
		if cfg.MaxFailedAttempts != 5 {
			mafField.Source = SourceCLI
		}
		fields = append(fields, mafField)
	} else {
		// Client: show "Max reconnect" in main fields
		mrField := NewNumberField("Max reconnect", cfg.MaxReconnectAttempts, 0, 100)
		if cfg.MaxReconnectAttempts != 5 {
			mrField.Source = SourceCLI
		}
		fields = append(fields, mrField)

		// Auto reconnect (client only)
		arOpts := []string{"off", "on"}
		arIdx := 0
		if cfg.AutoReconnect {
			arIdx = 1
		}
		arField := NewToggleField("Auto reconnect", arOpts, arIdx)
		arField.Hint = "(reconnect after server shutdown)"
		if cfg.AutoReconnect {
			arField.Source = SourceCLI
		}
		fields = append(fields, arField)

		// Reconnect limit (client only)
		attField := NewNumberField("Reconnect limit", cfg.AutoReconnectAttempts, 0, 999)
		attField.Hint = "(0 = unlimited)"
		if cfg.AutoReconnectAttempts != 0 {
			attField.Source = SourceCLI
		}
		// Initially hidden if auto-reconnect is off
		attField.Hidden = !cfg.AutoReconnect
		fields = append(fields, attField)
	}

	aecOpts := []string{"off", "on"}
	aecIdx := 0
	if cfg.AEC {
		aecIdx = 1
	}
	aecField := NewToggleField("Echo cancellation", aecOpts, aecIdx)
	if cfg.AEC {
		aecField.Source = SourceCLI
	}
	fields = append(fields, aecField)

	// Virtual mic (Linux only) — creates PulseAudio null-sink
	if runtime.GOOS == "linux" {
		vmField := NewActionField("Virtual mic", "Create ▸")
		vmField.Hint = "(PulseAudio)"
		fields = append(fields, vmField)
	}

	// Action: Advanced
	fields = append(fields, NewActionField("", "Advanced ▸"))

	return fields
}

func buildAdvancedFields(cfg config.Config) []SetupField {
	var fields []SetupField

	// Log level
	logOpts := []string{"debug", "info", "warn", "error"}
	logIdx := 1 // default info
	for i, opt := range logOpts {
		if opt == cfg.LogLevel {
			logIdx = i
			break
		}
	}
	logField := NewSelectField("Log level", logOpts, logIdx)
	if cfg.LogLevel != "" && cfg.LogLevel != "info" {
		logField.Source = SourceCLI
	}
	fields = append(fields, logField)

	// TLS
	if cfg.Mode == config.ModeServer {
		tlsOpts := []string{"off", "on"}
		tlsIdx := 0
		if cfg.IsTLSEnabled() {
			tlsIdx = 1
		}
		tlsField := NewToggleField("TLS", tlsOpts, tlsIdx)
		if cfg.IsTLSEnabled() {
			tlsField.Source = SourceCLI
		}
		if cfg.TLSSelfSigned {
			tlsField.Hint = "⚠ self-signed"
		}
		fields = append(fields, tlsField)
	} else {
		tlsOpts := []string{"off", "on"}
		tlsIdx := 0
		if cfg.TLS {
			tlsIdx = 1
		}
		tlsField := NewToggleField("TLS", tlsOpts, tlsIdx)
		if cfg.TLS {
			tlsField.Source = SourceCLI
		} else {
			tlsField.Hint = "(server)"
		}
		fields = append(fields, tlsField)
	}

	// TLS Cert/Key (server only, hidden when TLS is off) — right below TLS toggle
	if cfg.Mode == config.ModeServer {
		tlsOn := cfg.IsTLSEnabled()

		certField := NewTextField("TLS Cert", cfg.TLSCert, false)
		if cfg.TLSCert != "" {
			certField.Source = SourceCLI
		}
		certField.Hidden = !tlsOn
		fields = append(fields, certField)

		keyField := NewTextField("TLS Key", cfg.TLSKey, false)
		if cfg.TLSKey != "" {
			keyField.Source = SourceCLI
		}
		keyField.Hidden = !tlsOn
		fields = append(fields, keyField)
	}

	srOpts := []string{"48 kHz", "24 kHz", "16 kHz", "8 kHz"}
	srIdx := 0
	srDisplay := FormatSampleRate(cfg.SampleRate)
	for i, opt := range srOpts {
		if opt == srDisplay {
			srIdx = i
			break
		}
	}
	srField := NewSelectField("Sample rate", srOpts, srIdx)
	if cfg.SampleRate != 48000 {
		srField.Source = SourceCLI
	}
	fields = append(fields, srField)

	chOpts := []string{"stereo", "mono"}
	chIdx := 0 // default stereo (matches config default Channels=2)
	if cfg.Channels == 1 {
		chIdx = 1
	}
	chField := NewToggleField("Channels", chOpts, chIdx)
	if cfg.Channels != 2 {
		chField.Source = SourceCLI
	}
	fields = append(fields, chField)

	brOpts := []string{"16 kbps", "32 kbps", "64 kbps", "96 kbps", "128 kbps", "256 kbps", "510 kbps"}
	brIdx := 2 // default 64 kbps
	brDisplay := FormatBitrate(cfg.OpusBitrate)
	for i, opt := range brOpts {
		if opt == brDisplay {
			brIdx = i
			break
		}
	}
	brField := NewSelectField("Opus bitrate", brOpts, brIdx)
	if cfg.OpusBitrate != 64000 {
		brField.Source = SourceCLI
	}
	fields = append(fields, brField)

	// Hardware acceleration (SIMD) — only shown when CPU supports it
	if audio.CPUHasSIMD() {
		hwOpts := []string{"on", "off"}
		hwIdx := 0
		if cfg.NoSIMDOptimization {
			hwIdx = 1
		}
		hwField := NewToggleField("Use SIMD", hwOpts, hwIdx)
		hwField.Hint = "(accelerate audio mixing for 2+ streams)"
		if cfg.NoSIMDOptimization {
			hwField.Source = SourceCLI
		}
		fields = append(fields, hwField)
	}

	// HWID collection toggle (server only)
	if cfg.Mode == config.ModeServer {
		hwidOpts := []string{"off", "on"}
		hwidIdx := 0
		if cfg.HWIDRequired {
			hwidIdx = 1
		}
		hwidField := NewToggleField("HWID collection", hwidOpts, hwidIdx)
		hwidField.Hint = "(collect device ID for bans)"
		if cfg.HWIDRequired {
			hwidField.Source = SourceCLI
		}
		fields = append(fields, hwidField)
	}

	return fields
}

// modeKeyToDescriptive maps a bare mode key ("normal", "reverse", etc.) to its
// descriptive option string used in the Mode toggle field.
var modeKeyToDescriptive = map[string]string{
	"normal":     "normal (server → client)",
	"reverse":    "reverse (client → server)",
	"duplex":     "duplex (bidirectional)",
	"conference": "conference (multi-user)",
}

// applyFieldDependencies updates field requirements based on current values.
func (m *SetupModel) applyFieldDependencies() {
	// Find TLS field value (now in AdvancedFields)
	tlsValue := "off"
	for _, f := range m.AdvancedFields {
		if f.Label == "TLS" {
			tlsValue = f.Value
			break
		}
	}

	// Update TLS Cert/Key visibility and requirements in advanced fields
	for i := range m.AdvancedFields {
		if m.AdvancedFields[i].Label == "TLS Cert" || m.AdvancedFields[i].Label == "TLS Key" {
			if tlsValue == "on" {
				m.AdvancedFields[i].Hidden = false
				m.AdvancedFields[i].Required = true
				if m.AdvancedFields[i].Value == "" {
					m.AdvancedFields[i].Hint = "⚠ required when TLS is on"
				} else {
					m.AdvancedFields[i].Hint = ""
				}
			} else {
				m.AdvancedFields[i].Hidden = true
				m.AdvancedFields[i].Required = false
				m.AdvancedFields[i].Hint = ""
				m.AdvancedFields[i].Value = ""
			}
		}
	}

	// If cursor is on a hidden TLS Cert/Key field, move to TLS field
	if m.advancedOpen {
		fields := m.activeFields()
		if m.FieldCursor < len(fields) && fields[m.FieldCursor].Hidden {
			for j := m.FieldCursor - 1; j >= 0; j-- {
				if !fields[j].Hidden {
					m.FieldCursor = j
					break
				}
			}
		}
	}

	// Update isDuplexMode/isConferenceMode and cfg flags from Mode field (server)
	// or from probe result (client).
	var modeKey string
	for _, f := range m.Fields {
		if f.Label == "Mode" {
			modeKey = strings.SplitN(f.Value, " ", 2)[0]
			break
		}
	}
	// Client has no Mode field — derive from probe result
	if modeKey == "" && m.probeResult != nil {
		modeKey = m.probeResult.Mode
	}
	if modeKey != "" {
		m.isConferenceMode = modeKey == "conference"
		m.isDuplexMode = modeKey == "duplex" || m.isConferenceMode
		m.cfg.Reverse = modeKey == "reverse"
		m.cfg.Duplex = modeKey == "duplex"
		m.cfg.Conference = modeKey == "conference"
		// If current DeviceSection is now hidden, move to the visible one
		showInput, showOutput := m.visibleSections()
		if m.DeviceSection == SectionInput && !showInput && showOutput {
			m.DeviceSection = SectionOutput
			m.deviceCursor = 0
		} else if m.DeviceSection == SectionOutput && !showOutput && showInput {
			m.DeviceSection = SectionInput
			m.deviceCursor = 0
		}
	}

	// Conference requires at least 2 clients; restore default when leaving conference.
	for i := range m.Fields {
		if m.Fields[i].Label == "Max clients" {
			if m.isConferenceMode && m.Fields[i].IntValue() < 2 {
				m.preConferenceMaxClients = m.Fields[i].IntValue()
				m.Fields[i].SetValue("2", SourceDefault)
			} else if !m.isConferenceMode && m.preConferenceMaxClients > 0 {
				m.Fields[i].SetValue(fmt.Sprintf("%d", m.preConferenceMaxClients), SourceDefault)
				m.preConferenceMaxClients = 0
			}
			break
		}
	}

	// Echo cancellation is only useful in duplex/conference modes
	for i := range m.Fields {
		if m.Fields[i].Label == "Echo cancellation" {
			m.Fields[i].Hidden = !m.isDuplexMode
			if m.Fields[i].Hidden {
				m.Fields[i].Value = "off"
				m.Fields[i].optIndex = 0
			}
			break
		}
	}

	// Auto reconnect: show/hide Reconnect limit field
	autoReconnectOn := false
	for _, f := range m.Fields {
		if f.Label == "Auto reconnect" {
			autoReconnectOn = f.Value == "on"
			break
		}
	}
	for i := range m.Fields {
		if m.Fields[i].Label == "Reconnect limit" {
			m.Fields[i].Hidden = !autoReconnectOn
			break
		}
	}

	// Auto-open advanced if TLS is on and cert/key are empty
	if tlsValue == "on" && m.cfg.Mode == config.ModeServer {
		certEmpty := true
		keyEmpty := true
		for _, f := range m.AdvancedFields {
			if f.Label == "TLS Cert" && f.Value != "" {
				certEmpty = false
			}
			if f.Label == "TLS Key" && f.Value != "" {
				keyEmpty = false
			}
		}
		if certEmpty || keyEmpty {
			m.advancedOpen = true
		}
	}
}

// InitCmd returns the initial command for the setup screen (e.g. start discovery or probe).
func (m SetupModel) InitCmd() tea.Cmd {
	var cmds []tea.Cmd

	// Include pending restore command from WithUnifiedDeviceList (server mode auto-restore)
	if m.pendingRestoreCmd != nil {
		cmds = append(cmds, m.pendingRestoreCmd)
	}

	if m.cfg.Mode != config.ModeClient {
		if len(cmds) > 0 {
			return tea.Batch(cmds...)
		}
		return nil
	}

	// Always start mDNS discovery
	if m.cfg.Address == "" {
		discoveryTick := tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return DiscoveryTickMsg{} })
		cmds = append(cmds, StartDiscoveryCmd(3*time.Second), discoveryTick)
	}

	// Probe any existing entries (recent servers loaded at construction time)
	if m.serverList.HasEntries() {
		cmds = append(cmds, StartAllProbesCmd(m.serverList.Entries()), tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return ServerProbeSpinnerMsg{} }))
	}

	// If address already set (from CLI) — also probe immediately via the original path
	if m.cfg.Address != "" {
		var addr, port string
		for _, f := range m.Fields {
			if f.Label == "Server address" {
				addr = f.Value
			}
			if f.Label == "Port" {
				port = f.Value
			}
		}
		if addr != "" && port != "" {
			cmds = append(cmds, StartProbeCmd(addr, port))
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// activeFields returns the currently navigable fields slice.
// Hidden fields are included (for correct index mapping in writeBackFields)
// but skipped during rendering and cursor navigation.
func (m *SetupModel) activeFields() []SetupField {
	if m.advancedOpen {
		combined := make([]SetupField, 0, len(m.Fields)+len(m.AdvancedFields))
		for _, f := range m.Fields {
			combined = append(combined, f)
			if f.Type == FieldAction && strings.Contains(f.ActionLabel, "Advanced") {
				combined = append(combined, m.AdvancedFields...)
			}
		}
		return combined
	}
	return m.Fields
}

// firstVisibleField returns the index of the first non-hidden field.
func (m *SetupModel) firstVisibleField() int {
	fields := m.activeFields()
	for i, f := range fields {
		if !f.Hidden {
			return i
		}
	}
	return 0
}

// allFieldsFlat returns all fields for validation (main + advanced).
func (m *SetupModel) allFieldsFlat() []SetupField {
	result := make([]SetupField, 0, len(m.Fields)+len(m.AdvancedFields))
	result = append(result, m.Fields...)
	result = append(result, m.AdvancedFields...)
	return result
}

// writeBackFields writes the modified field slice back to the model.
func (m *SetupModel) writeBackFields(fields []SetupField) {
	if m.advancedOpen {
		mainIdx := 0
		advIdx := 0
		inAdvanced := false
		for _, f := range fields {
			if f.Type == FieldAction && strings.Contains(f.ActionLabel, "Advanced") {
				m.Fields[mainIdx] = f
				mainIdx++
				inAdvanced = true
				continue
			}
			if inAdvanced && advIdx < len(m.AdvancedFields) {
				m.AdvancedFields[advIdx] = f
				advIdx++
				if advIdx >= len(m.AdvancedFields) {
					inAdvanced = false
				}
				continue
			}
			if mainIdx < len(m.Fields) {
				m.Fields[mainIdx] = f
				mainIdx++
			}
		}
	} else {
		copy(m.Fields, fields)
	}
}

// renderNarrowTabBar renders a [Devices] | Settings or Devices | [Settings] tab bar.
// Returns empty string when width >= 80 (wide mode uses two-column layout instead).
// deviceStatusHint returns the device selection status shown under "Audio Devices" header.
// modeIndicator returns a styled mode description shown under the device status hint.
func (m SetupModel) leftColumnWidth() int {
	half := m.width / 2
	if half < 30 {
		return 30
	}
	return half
}

func (m SetupModel) bodyHeight() int {
	h := m.height - 2
	if h < 10 {
		return 10
	}
	return h
}

// Width returns the current width.
func (m SetupModel) Width() int { return m.width }

// Height returns the current height.
func (m SetupModel) Height() int { return m.height }

// SetFlash sets a flash notification message on the setup screen.
// The message auto-clears after the given duration.
func (m *SetupModel) SetFlash(msg string, d time.Duration) tea.Cmd {
	m.flashMsg = msg
	m.flashTimer = time.Now().Add(d)
	return tea.Tick(d, func(time.Time) tea.Msg { return FlashDismissMsg{} })
}

// SetSize updates dimensions.
func (m *SetupModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// deviceListHeight returns the height for each device list.
// In duplex mode, the available height is split between two lists.
func (m SetupModel) deviceListHeight() int {
	h := m.bodyHeight() - 4
	if m.HasOutputList {
		// Split: title+sep for each section = 4 lines overhead, remaining split in half
		h = (m.bodyHeight() - 8) / 2
		if h < 3 {
			h = 3
		}
	}
	return h
}

// SelectedDeviceName returns the name(s) of selected device(s) for display.
func (m SetupModel) SelectedDeviceName() string {
	if m.unifiedDuplex {
		if m.isDuplexMode {
			var caps, plays []string
			for key := range m.multiSelect {
				name := displayNameFromKey(key)
				if strings.HasPrefix(key, "I:") {
					caps = append(caps, name)
				} else {
					plays = append(plays, name)
				}
			}
			if len(caps) > 0 && len(plays) > 0 {
				return strings.Join(caps, ", ") + " → " + strings.Join(plays, ", ")
			}
			if len(caps) > 0 {
				return strings.Join(caps, ", ")
			}
			return ""
		}
		// Non-duplex: just list selected names
		var names []string
		for key := range m.multiSelect {
			names = append(names, displayNameFromKey(key))
		}
		return strings.Join(names, ", ")
	}
	inputName := ""
	if item, ok := m.DeviceList.SelectedItem().(interface{ FilterValue() string }); ok {
		inputName = item.FilterValue()
	}
	if m.HasOutputList {
		outputName := ""
		if item, ok := m.OutputDeviceList.SelectedItem().(interface{ FilterValue() string }); ok {
			outputName = item.FilterValue()
		}
		if inputName != "" && outputName != "" {
			return inputName + " → " + outputName
		}
	}
	return inputName
}

// isServerReady returns true if the client has a reachable server
// (selected from list with successful probe, or manually entered with successful probe).
func (m SetupModel) isServerReady() bool {
	return m.probeResult != nil
}

// ProbeResult returns the last successful probe result (may be nil).
func (m SetupModel) ProbeResult() *ProbeServerResult {
	return m.probeResult
}

// SelectedServer returns the currently selected server entry (may be nil).
func (m SetupModel) SelectedServer() *ServerEntry {
	return m.serverList.SelectedEntry()
}
