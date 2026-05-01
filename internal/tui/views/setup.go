package views

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/i18n"
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
	SetupOverlayVirtualSinkLifecycle
	SetupOverlayLanguage
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
	multiSelect      map[string]DeviceRoleSet
	unifiedDuplex    bool // true when using unified list (all modes)
	isDuplexMode     bool // true when duplex/conference — two checkbox columns; false — one column
	isConferenceMode bool // true when conference — enables hub mode on server

	// Sectioned device list (Input/Output sections with checkbox columns)
	inputDevices  []deviceRow
	outputDevices []deviceRow
	deviceCursor  int // row index within current section
	inputScroll   int // scroll offset for input section
	outputScroll  int // scroll offset for output section
	deviceColumn  int // 0 = device select, 1 = AGC column

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
	overlay                     SetupOverlay
	summaryOverlay              *SummaryOverlay
	virtualDeviceOverlay        *VirtualDeviceOverlay
	restoreOverlay              *RestoreOverlay
	configSaveOverlay           *ConfigSaveOverlay
	configLoadOverlay           *ConfigLoadOverlay
	virtualSinkLifecycleOverlay *VirtualSinkLifecycleOverlay
	languageOverlay             *LanguageOverlay

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
	virtualMicCreated       bool   // true when pactl sink was created this session
	virtualMicModule        string // PulseAudio module ID for automatic cleanup
	virtualMicManageable    bool   // true when an exact EchoWarp sink can be managed
	virtualMicManagedModule string // PulseAudio module ID for explicit user removal

	// Virtual sink lifecycle preferences (set via overlay after creation)
	virtualSinkOnStop              recent.SinkLifecycle // default: SinkDelete
	virtualSinkOnStart             recent.SinkLifecycle // default: SinkRecreate
	virtualSinkLifecycleConfigured bool                 // true after app-managed lifecycle setup
	pendingVirtualSinkSelection    map[string]bool      // sinks that should be selected after refresh

	// Mix input: maps virtual output device selectKey → set of input device selectKeys
	// whose audio should be mixed into that output.
	mixInputs map[string]map[string]bool

	// refreshDevicesFn re-enumerates audio devices from the OS.
	// Set via WithDeviceRefreshFunc. Used after creating a virtual sink.
	refreshDevicesFn func() ([]list.Item, error)
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
					ServerID:      rs.ServerID,
					Source:        ServerSourceRecent,
					LastConnected: rs.LastConnected,
					ProbeStatus:   ServerProbePending,
				})
			}
			m.serverList.SetEntries(entries)
			m.serverList.SetHasRecent(true)

			// Apply LogLevel from the most recent server entry (recentServers
			// is sorted by LastConnected desc) so user's last choice persists
			// across restarts. Only when cfg.LogLevel is still at default.
			if cfg.LogLevel == "" || cfg.LogLevel == "info" {
				for _, rs := range recentServers {
					if rs.LogLevel == "" {
						continue
					}
					m.cfg.LogLevel = rs.LogLevel
					// "info" is the default — restore it with SourceDefault
					// so the UI doesn't flag it with a ✓ user-set marker.
					src := SourceConfig
					if rs.LogLevel == "info" {
						src = SourceDefault
					}
					for i := range m.AdvancedFields {
						if m.AdvancedFields[i].Key == "log_level" {
							m.AdvancedFields[i].SetValue(rs.LogLevel, src)
							break
						}
					}
					break
				}
			}
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
		// Apply top-level last_mode first (so the current mode is known before restoring
		// per-mode server fields).
		m.restoreLastMode(sp.LastMode)
		// Determine current mode from the (possibly just-updated) Mode field.
		currentMode := ""
		for _, f := range m.Fields {
			if f.Key == "mode" {
				currentMode = modeKeyFromValue(f.Value)
				break
			}
		}
		if currentMode == "" {
			currentMode = "normal"
		}
		if mp := sp.Get(currentMode); mp != nil {
			m.restoreModePreset(*mp)
		}
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
	// Preserve isDuplexMode when already set by applyFieldDependencies from the
	// restored Mode field (duplex/conference presets): overwriting with the
	// caller's isDuplex flag (derived from cfg.Duplex, which is false after a
	// plain restart without CLI) would hide the output section on startup.
	if isDuplex {
		m.isDuplexMode = true
	}
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.HasOutputList = false
	// Pick the initial focused section based on which ones are visible.
	// In reverse mode the input section is hidden, so the cursor must start
	// on Output — otherwise ↑/↓ scroll the invisible inputDevices list and
	// no ▸ cursor is drawn until the user presses →← to re-sync.
	showInput, showOutput := m.visibleSections()
	if showInput {
		m.DeviceSection = SectionInput
	} else if showOutput {
		m.DeviceSection = SectionOutput
	} else {
		m.DeviceSection = SectionInput
	}
	m.deviceCursor = 0
	m.rebuildDeviceGroups()

	// For server mode, auto-restore devices now that devices are populated
	if m.cfg.Mode == config.ModeServer && m.serverPresets != nil {
		m.pendingRestoreCmd = m.tryShowServerRestoreOverlay()
	}

	return m
}

// WithDeviceRefreshFunc sets a callback that re-enumerates audio devices from the OS.
// The callback should return list items for all devices (input + output).
func (m SetupModel) WithDeviceRefreshFunc(fn func() ([]list.Item, error)) SetupModel {
	m.refreshDevicesFn = fn
	m.refreshTrackedVirtualSinkAfterEnumeratorInstall()
	return m
}

func (m *SetupModel) refreshTrackedVirtualSinkAfterEnumeratorInstall() {
	if !isLinuxRuntime || !m.hasVirtualMicModuleState() {
		return
	}
	m.refreshDevicesAfterVirtualSinkEnsure()
	m.selectPendingVirtualSink(echowarpSinkName)
}

// WithVirtualSinkLifecycle sets cleanup/startup behavior for the virtual sink.
func (m SetupModel) WithVirtualSinkLifecycle(onStop, onStart recent.SinkLifecycle) SetupModel {
	m.virtualSinkOnStop = onStop
	m.virtualSinkOnStart = onStart
	m.virtualSinkLifecycleConfigured = true
	return m
}

// WithVirtualSinkCreatedForSession records a virtual sink created by this process.
func (m SetupModel) WithVirtualSinkCreatedForSession(moduleID string) SetupModel {
	m.virtualMicCreated = true
	m.virtualMicModule = moduleID
	m.virtualMicManageable = true
	m.virtualMicManagedModule = moduleID
	m.virtualSinkLifecycleConfigured = true
	return m
}

// VirtualMicModule returns the PulseAudio module ID of the virtual mic created
// this session (empty string if none). Used by the TUI to clean up on stop.
func (m SetupModel) VirtualMicModule() string {
	return m.virtualMicModule
}

// VirtualSinkCleanupPlan describes whether and how the app should remove a sink.
type VirtualSinkCleanupPlan struct {
	Delete            bool
	SinkName          string
	ModuleID          string
	AllowNameFallback bool
}

// VirtualSinkCleanupPlan returns the safe cleanup action for the EchoWarp sink.
func (m SetupModel) VirtualSinkCleanupPlan() VirtualSinkCleanupPlan {
	sinkName, shouldDelete := m.virtualSinkCleanupTarget()
	return VirtualSinkCleanupPlan{
		Delete:            shouldDelete,
		SinkName:          sinkName,
		ModuleID:          m.virtualSinkCleanupModuleID(),
		AllowNameFallback: m.virtualMicCreated,
	}
}

// MarkVirtualSinkCleaned clears session-local module state after cleanup.
func (m *SetupModel) MarkVirtualSinkCleaned() {
	m.virtualMicCreated = false
	m.virtualMicModule = ""
	m.syncVirtualMicState()
}

func (m SetupModel) virtualSinkCleanupTarget() (string, bool) {
	if m.virtualSinkLifecycleConfigured || m.virtualMicCreated {
		for _, vs := range m.SelectedVirtualSinkPresets() {
			if vs.ModuleType == "module-null-sink" && vs.SinkName == echowarpSinkName {
				return vs.SinkName, vs.OnStop == recent.SinkDelete
			}
		}
	}
	if !m.hasVirtualSinkCleanupCandidate() {
		return echowarpSinkName, false
	}
	onStop := m.virtualSinkOnStop
	if onStop == "" {
		onStop = recent.SinkDelete
	}
	return echowarpSinkName, onStop == recent.SinkDelete
}

func (m SetupModel) virtualSinkCleanupModuleID() string {
	if m.virtualMicModule != "" {
		return m.virtualMicModule
	}
	if m.virtualSinkLifecycleConfigured {
		return m.virtualMicManagedModule
	}
	return ""
}

func (m SetupModel) hasVirtualSinkCleanupCandidate() bool {
	if m.virtualMicCreated || m.virtualMicModule != "" {
		return true
	}
	return m.virtualSinkLifecycleConfigured && m.virtualMicManagedModule != ""
}

// SelectedVirtualSinkPresets returns VirtualSinkPreset entries for all currently
// selected output devices that have a virtual sink preset.
func (m SetupModel) SelectedVirtualSinkPresets() []recent.VirtualSinkPreset {
	var result []recent.VirtualSinkPreset
	for _, d := range m.outputDevices {
		if _, ok := m.multiSelect[d.selectKey()]; !ok {
			continue
		}
		if !d.IsVirtual {
			continue
		}
		// Collect from the current device preset.
		devPreset := m.CollectPresetDevices()
		for _, pd := range devPreset.Devices {
			if pd.VirtualSink != nil && pd.Name == d.Name {
				result = append(result, *pd.VirtualSink)
			}
		}
	}
	return result
}

// refreshDevicesFromOS re-enumerates devices and updates the device list.
func (m *SetupModel) refreshDevicesFromOS() {
	if m.refreshDevicesFn == nil {
		return
	}
	items, err := m.refreshDevicesFn()
	if err != nil {
		return
	}
	m.DeviceList.SetItems(items)
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
		addrField := NewTextField("server_address", i18n.T("field_server_address"), cfg.Address, true)
		addrField.SetValidator(ValidateAddress)
		if cfg.Address != "" {
			addrField.Source = SourceCLI
		}
		fields = append(fields, addrField)
	}

	portField := NewNumberField("port", i18n.T("field_port"), cfg.Port, 1, 65535)
	if cfg.Port != 4415 { // non-default
		portField.Source = SourceCLI
	}
	fields = append(fields, portField)

	pw := ""
	if cfg.Password != "" {
		pw = cfg.Password
	}
	pwField := NewPasswordField("password", i18n.T("field_password"), pw)
	if cfg.Password != "" {
		pwField.Source = SourceCLI
	} else if cfg.Mode == config.ModeClient {
		pwField.Hint = i18n.T("hint_server")
		pwField.Hidden = true // hidden until probe says PasswordRequired
	}
	fields = append(fields, pwField)

	// Nickname (client only)
	if cfg.Mode == config.ModeClient {
		nickField := NewTextField("nickname", i18n.T("field_nickname"), cfg.Nickname, false)
		nickField.Hint = i18n.T("hint_nickname")
		if cfg.Nickname != "" {
			nickField.Source = SourceCLI
		}
		fields = append(fields, nickField)
	}

	if cfg.Mode == config.ModeServer {
		mcField := NewNumberField("max_clients", i18n.T("field_max_clients"), cfg.MaxClients, 1, 100)
		if cfg.MaxClients != 1 {
			mcField.Source = SourceCLI
		}
		fields = append(fields, mcField)
	}

	// Mode (server only — client gets mode from probe result)
	if cfg.Mode == config.ModeServer {
		modeOpts := []string{
			i18n.T("mode_normal"),
			i18n.T("mode_reverse"),
			i18n.T("mode_duplex"),
			i18n.T("mode_conference"),
		}
		modeIdx := 0
		if cfg.Conference {
			modeIdx = 3
		} else if cfg.Duplex {
			modeIdx = 2
		} else if cfg.Reverse {
			modeIdx = 1
		}
		modeField := NewToggleField("mode", i18n.T("field_mode"), modeOpts, modeIdx)
		if cfg.Reverse || cfg.Duplex || cfg.Conference {
			modeField.Source = SourceCLI
		}
		fields = append(fields, modeField)
	}

	if cfg.Mode == config.ModeServer {
		// Server: show "Max auth fail" (ban threshold) in main fields
		mafField := NewNumberField("max_auth_fail", i18n.T("field_max_auth_fail"), cfg.MaxFailedAttempts, 0, 100)
		if cfg.MaxFailedAttempts != 5 {
			mafField.Source = SourceCLI
		}
		fields = append(fields, mafField)
	} else {
		// Client: show "Max reconnect" in main fields
		mrField := NewNumberField("max_reconnect", i18n.T("field_max_reconnect"), cfg.MaxReconnectAttempts, 0, 100)
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
		arField := NewToggleField("auto_reconnect", i18n.T("field_auto_reconnect"), arOpts, arIdx)
		arField.Hint = i18n.T("hint_reconnect_after")
		if cfg.AutoReconnect {
			arField.Source = SourceCLI
		}
		fields = append(fields, arField)

		// Reconnect limit (client only)
		attField := NewNumberField("reconnect_limit", i18n.T("field_reconnect_limit"), cfg.AutoReconnectAttempts, 0, 999)
		attField.Hint = i18n.T("hint_reconnect_unlimited")
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
	aecField := NewToggleField("echo_cancellation", i18n.T("field_echo_cancellation"), aecOpts, aecIdx)
	if cfg.AEC {
		aecField.Source = SourceCLI
	}
	fields = append(fields, aecField)

	// Virtual mic (Linux only) — creates PulseAudio null-sink
	if runtime.GOOS == "linux" {
		vmField := NewActionField("virtual_mic", i18n.T("field_virtual_mic"), i18n.T("action_create_virtual"))
		vmField.Hint = i18n.T("hint_pulse_audio")
		fields = append(fields, vmField)
	}

	// Action: Advanced
	fields = append(fields, NewActionField("advanced", "", i18n.T("action_advanced")))

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
	logField := NewSelectField("log_level", i18n.T("field_log_level"), logOpts, logIdx)
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
		tlsField := NewToggleField("tls", i18n.T("field_tls"), tlsOpts, tlsIdx)
		if cfg.IsTLSEnabled() {
			tlsField.Source = SourceCLI
		}
		if cfg.TLSSelfSigned {
			tlsField.Hint = i18n.T("hint_tls_self_signed")
		}
		fields = append(fields, tlsField)
	} else {
		tlsOpts := []string{"off", "on"}
		tlsIdx := 0
		if cfg.TLS {
			tlsIdx = 1
		}
		tlsField := NewToggleField("tls", i18n.T("field_tls"), tlsOpts, tlsIdx)
		if cfg.TLS {
			tlsField.Source = SourceCLI
		} else {
			tlsField.Hint = i18n.T("hint_server")
		}
		fields = append(fields, tlsField)
	}

	// TLS Cert/Key (server only, hidden when TLS is off) — right below TLS toggle
	if cfg.Mode == config.ModeServer {
		tlsOn := cfg.IsTLSEnabled()

		certField := NewTextField("tls_cert", i18n.T("field_tls_cert"), cfg.TLSCert, false)
		if cfg.TLSCert != "" {
			certField.Source = SourceCLI
		}
		certField.Hidden = !tlsOn
		fields = append(fields, certField)

		keyField := NewTextField("tls_key", i18n.T("field_tls_key"), cfg.TLSKey, false)
		if cfg.TLSKey != "" {
			keyField.Source = SourceCLI
		}
		keyField.Hidden = !tlsOn
		fields = append(fields, keyField)
	}

	// Rate limit (server only)
	if cfg.Mode == config.ModeServer {
		rlVal := cfg.RateLimit
		if rlVal == 0 {
			rlVal = 5 // CLI default
		}
		rlField := NewNumberField("rate_limit", i18n.T("field_rate_limit"), rlVal, 0, 1000)
		rlField.Hint = i18n.T("hint_rate_limit")
		if cfg.RateLimit != 0 && cfg.RateLimit != 5 {
			rlField.Source = SourceCLI
		}
		fields = append(fields, rlField)
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
	srField := NewSelectField("sample_rate", i18n.T("field_sample_rate"), srOpts, srIdx)
	if cfg.SampleRate != 48000 {
		srField.Source = SourceCLI
	}
	fields = append(fields, srField)

	chOpts := []string{"stereo", "mono"}
	chIdx := 0 // default stereo (matches config default Channels=2)
	if cfg.Channels == 1 {
		chIdx = 1
	}
	chField := NewToggleField("channels", i18n.T("field_channels"), chOpts, chIdx)
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
	brField := NewSelectField("opus_bitrate", i18n.T("field_opus_bitrate"), brOpts, brIdx)
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
		hwField := NewToggleField("use_simd", i18n.T("field_use_simd"), hwOpts, hwIdx)
		hwField.Hint = i18n.T("hint_simd")
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
		hwidField := NewToggleField("hwid_collection", i18n.T("field_hwid_collection"), hwidOpts, hwidIdx)
		hwidField.Hint = i18n.T("hint_hwid")
		if cfg.HWIDRequired {
			hwidField.Source = SourceCLI
		}
		fields = append(fields, hwidField)
	}

	return fields
}

// modeKeyToDescriptive maps a bare mode key ("normal", "reverse", etc.) to its
// descriptive option string used in the Mode toggle field.
// Computed at call time so that i18n translations are resolved dynamically.
func modeKeyToDescriptiveMap() map[string]string {
	return map[string]string{
		"normal":     i18n.T("mode_normal"),
		"reverse":    i18n.T("mode_reverse"),
		"duplex":     i18n.T("mode_duplex"),
		"conference": i18n.T("mode_conference"),
	}
}

// modeKeyFromValue returns the canonical mode key ("normal", "reverse", "duplex",
// "conference") given a Mode field value. The field value is normally a localized
// descriptive string (e.g. "normale (server → client)" in Italian), so this
// reverse-looks-up the canonical key via modeKeyToDescriptiveMap. Falls back to
// the first whitespace-delimited word when no descriptive match is found — this
// keeps backward compatibility with legacy call sites that pass bare keys.
//
// This helper exists because taking `strings.SplitN(value, " ", 2)[0]` directly
// on a localized descriptive string yields the translated first word
// (e.g. "normale" / "thường"), which then fails preset lookup keyed by the
// canonical English key.
func modeKeyFromValue(v string) string {
	for key, desc := range modeKeyToDescriptiveMap() {
		if desc == v {
			return key
		}
	}
	return strings.SplitN(v, " ", 2)[0]
}

// applyFieldDependencies updates field requirements based on current values.
func (m *SetupModel) applyFieldDependencies() {
	// Find TLS field value (now in AdvancedFields)
	tlsValue := "off"
	for _, f := range m.AdvancedFields {
		if f.Key == "tls" {
			tlsValue = f.Value
			break
		}
	}

	// Update TLS Cert/Key visibility and requirements in advanced fields
	for i := range m.AdvancedFields {
		if m.AdvancedFields[i].Key == "tls_cert" || m.AdvancedFields[i].Key == "tls_key" {
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
		if f.Key == "mode" {
			modeKey = modeKeyFromValue(f.Value)
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

	// Echo cancellation is only useful in duplex/conference modes
	for i := range m.Fields {
		if m.Fields[i].Key == "echo_cancellation" {
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
		if f.Key == "auto_reconnect" {
			autoReconnectOn = f.Value == "on"
			break
		}
	}
	for i := range m.Fields {
		if m.Fields[i].Key == "reconnect_limit" {
			m.Fields[i].Hidden = !autoReconnectOn
			break
		}
	}

	// Auto-open advanced if TLS is on and cert/key are empty
	if tlsValue == "on" && m.cfg.Mode == config.ModeServer {
		certEmpty := true
		keyEmpty := true
		for _, f := range m.AdvancedFields {
			if f.Key == "tls_cert" && f.Value != "" {
				certEmpty = false
			}
			if f.Key == "tls_key" && f.Value != "" {
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
			if f.Key == "server_address" {
				addr = f.Value
			}
			if f.Key == "port" {
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
			if f.Key == "advanced" {
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
			if f.Key == "advanced" {
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
			for _, d := range m.inputDevices {
				if roles := m.multiSelect[d.selectKey()]; roles.Capture {
					caps = append(caps, d.Name)
				}
			}
			for _, d := range m.outputDevices {
				if roles := m.multiSelect[d.selectKey()]; roles.Playback {
					plays = append(plays, d.Name)
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
		rows := m.selectedVisibleRows()
		names := make([]string, 0, len(rows))
		for _, d := range rows {
			names = append(names, d.Name)
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
