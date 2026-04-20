package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func (m SetupModel) tryStart() (SetupModel, tea.Cmd) {
	// Client mode: block start without successful probe
	if m.cfg.Mode == config.ModeClient && m.probeResult == nil {
		m.validationError = "Server not reachable — check address and port"
		m.validationTimer = time.Now().Add(5 * time.Second)
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
	}
	// Validate required fields
	allFields := m.allFieldsFlat()
	for _, f := range allFields {
		var errMsg string
		if f.Required && f.Value == "" {
			errMsg = f.Label + " is required"
		} else if !f.IsValid() {
			errMsg = f.Label + ": " + f.lastResult.Message
		}
		if errMsg != "" {
			m.validationError = errMsg
			m.validationTimer = time.Now().Add(5 * time.Second)
			m.activeColumn = ColumnSettings
			// Find the field in activeFields, opening advanced if needed
			isAdvanced := false
			for _, af := range m.AdvancedFields {
				if af.Key == f.Key {
					isAdvanced = true
					break
				}
			}
			if isAdvanced {
				m.advancedOpen = true
			}
			for idx, af := range m.activeFields() {
				if af.Key == f.Key {
					m.FieldCursor = idx
					break
				}
			}
			dismissCmd := tea.Tick(5*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
			return m, dismissCmd
		}
	}

	finalCfg := m.BuildConfig()

	// In unified mode, build Devices from multiSelect
	if m.unifiedDuplex {
		// Conference server hub mode: no devices needed
		isHub := m.isConferenceMode && m.cfg.Mode == config.ModeServer && len(m.multiSelect) == 0
		if len(m.multiSelect) == 0 && !isHub {
			m.validationError = "Select at least one device"
			m.validationTimer = time.Now().Add(5 * time.Second)
			dismissCmd := tea.Tick(5*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
			return m, dismissCmd
		}
		if isHub {
			finalCfg.ServerMuted = true
		}
		finalCfg.Devices = nil
		allDevs := make([]deviceRow, len(m.inputDevices), len(m.inputDevices)+len(m.outputDevices))
		copy(allDevs, m.inputDevices)
		allDevs = append(allDevs, m.outputDevices...)
		for _, dev := range allDevs {
			_, ok := m.multiSelect[dev.selectKey()]
			if !ok {
				continue
			}
			devType := echowarp.DeviceOutput
			if dev.IsInput {
				devType = echowarp.DeviceInput
			}
			// Role determined by device section: Input=Capture, Output=Playback
			role := config.RolePlayback
			if dev.IsInput {
				role = config.RoleCapture
			}
			finalCfg.Devices = append(finalCfg.Devices, config.DeviceEntry{
				ID: dev.ID, Name: dev.Name, Type: devType, Role: role, Volume: dev.Volume, AGC: dev.AGC,
			})
		}

		if m.isDuplexMode && !isHub {
			// Conference server: any device combination is fine (capture-only, playback-only, or both).
			// Regular duplex requires at least one capture and one playback.
			if !m.isConferenceMode || m.cfg.Mode != config.ModeServer {
				hasCap, hasPlay := false, false
				for key := range m.multiSelect {
					if strings.HasPrefix(key, "I:") {
						hasCap = true
					} else {
						hasPlay = true
					}
				}
				if !hasCap || !hasPlay {
					m.validationError = "Select at least one [C] capture and one [P] playback device"
					m.validationTimer = time.Now().Add(5 * time.Second)
					dismissCmd := tea.Tick(5*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
					return m, dismissCmd
				}
			}
		} else if len(finalCfg.Devices) == 0 && !isHub {
			m.validationError = "Select at least one device"
			m.validationTimer = time.Now().Add(5 * time.Second)
			dismissCmd := tea.Tick(5*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
			return m, dismissCmd
		}

		// For single-device backward compatibility, also set DeviceID
		if len(finalCfg.Devices) == 1 {
			id := finalCfg.Devices[0].ID
			finalCfg.DeviceID = &id
		}
	}

	return m, func() tea.Msg { return SetupDoneMsg{Config: finalCfg} }
}

// BuildConfig constructs a config.Config from the current field values.
func (m SetupModel) BuildConfig() config.Config {
	cfg := m.cfg
	for _, f := range m.Fields {
		switch f.Key {
		case "server_address":
			cfg.Address = f.Value
		case "port":
			cfg.Port = f.IntValue()
		case "password":
			cfg.Password = f.Value
		case "max_clients":
			cfg.MaxClients = f.IntValue()
		case "mode":
			modeKey := modeKeyFromValue(f.Value)
			cfg.StreamMode = config.AudioMode(modeKey)
			cfg.SyncFromStreamMode()
		case "max_reconnect":
			cfg.MaxReconnectAttempts = f.IntValue()
		case "max_auth_fail":
			cfg.MaxFailedAttempts = f.IntValue()
		case "nickname":
			cfg.Nickname = f.Value
		case "echo_cancellation":
			cfg.AEC = f.Value == "on"
		case "auto_reconnect":
			cfg.AutoReconnect = f.Value == "on"
		case "reconnect_limit":
			cfg.AutoReconnectAttempts = f.IntValue()
		}
	}
	// Client mode: set mode from probe result (no Mode field in client fields)
	if m.cfg.Mode == config.ModeClient && m.probeResult != nil {
		cfg.StreamMode = config.AudioMode(m.probeResult.Mode)
		if cfg.StreamMode == "" {
			cfg.StreamMode = config.AudioModeNormal
		}
		cfg.SyncFromStreamMode()
	}

	// Populate Devices from multiSelect (unified device list)
	if m.unifiedDuplex && len(m.multiSelect) > 0 {
		cfg.Devices = nil
		allDevs := make([]deviceRow, 0, len(m.inputDevices)+len(m.outputDevices))
		allDevs = append(allDevs, m.inputDevices...)
		allDevs = append(allDevs, m.outputDevices...)
		for _, dev := range allDevs {
			_, ok := m.multiSelect[dev.selectKey()]
			if !ok {
				continue
			}
			devType := echowarp.DeviceOutput
			if dev.IsInput {
				devType = echowarp.DeviceInput
			}
			// Role determined by device section: Input=Capture, Output=Playback
			role := config.RolePlayback
			if dev.IsInput {
				role = config.RoleCapture
			}
			entry := config.DeviceEntry{
				ID: dev.ID, Name: dev.Name, Type: devType, Role: role, Volume: dev.Volume, AGC: dev.AGC,
			}
			// Attach mix input for virtual output devices.
			if !dev.IsInput && dev.IsVirtual {
				if mixSet, ok := m.mixInputs[dev.selectKey()]; ok {
					for _, inp := range m.inputDevices {
						if mixSet[inp.selectKey()] {
							id := inp.ID
							entry.MixInputID = &id
							entry.MixInputName = inp.Name
							break // one mix input per output device
						}
					}
				}
			}
			cfg.Devices = append(cfg.Devices, entry)
		}
	}

	for _, f := range m.AdvancedFields {
		switch f.Key {
		case "log_level":
			cfg.LogLevel = f.Value
		case "tls":
			switch f.Value {
			case "on":
				cfg.TLS = true
			default:
				cfg.TLS = false
			}
		case "sample_rate":
			if v := parseSampleRate(f.Value); v > 0 {
				cfg.SampleRate = v
			}
		case "channels":
			if f.Value == "mono" {
				cfg.Channels = 1
			} else {
				cfg.Channels = 2
			}
		case "opus_bitrate":
			if v := parseBitrate(f.Value); v > 0 {
				cfg.OpusBitrate = v
			}
		case "tls_cert":
			cfg.TLSCert = f.Value
		case "tls_key":
			cfg.TLSKey = f.Value
		case "use_simd":
			cfg.NoSIMDOptimization = f.Value == "off"
		case "hwid_collection":
			cfg.HWIDRequired = f.Value == "on"
		case "rate_limit":
			cfg.RateLimit = f.IntValue()
		}
	}
	return cfg
}

// applyLoadedConfig applies a loaded config to the setup fields and device selection.
// Only fields that differ from defaults get SourceConfig; all others reset to SourceDefault.
// For client configs with modes: map, only base fields are applied immediately;
// mode-specific settings (devices, aec, etc.) are stored and applied after probe.
func (m *SetupModel) applyLoadedConfig(cfg config.Config) {
	defaults := config.DefaultConfig()

	// Helper: set field value with SourceConfig if different from default value, else SourceDefault.
	setField := func(fields []SetupField, key, value, defaultValue string) {
		for i := range fields {
			if fields[i].Key == key {
				if value != defaultValue {
					fields[i].SetValue(value, SourceConfig)
				} else {
					fields[i].SetValue(value, SourceDefault)
				}
				return
			}
		}
	}

	isClient := m.cfg.Mode == config.ModeClient
	hasClientModes := isClient && len(cfg.ClientModes) > 0

	// Store client modes for later application after probe
	if hasClientModes {
		m.loadedClientModes = cfg.ClientModes
	}

	// Base fields (always applied)
	setField(m.Fields, "server_address", cfg.Address, defaults.Address)
	setField(m.Fields, "port", fmt.Sprintf("%d", cfg.Port), fmt.Sprintf("%d", defaults.Port))
	setField(m.Fields, "password", cfg.Password, defaults.Password)
	setField(m.Fields, "nickname", cfg.Nickname, defaults.Nickname)

	// For client with modes: map, skip mode-specific fields (applied after probe)
	if !hasClientModes {
		m.applyAllConfigFields(cfg, defaults, setField)
	}

	m.applyFieldDependencies()
}

// applyAllConfigFields applies all config fields (used for server mode and legacy client configs).
func (m *SetupModel) applyAllConfigFields(cfg config.Config, defaults config.Config, setField func([]SetupField, string, string, string)) {
	setField(m.Fields, "max_clients", fmt.Sprintf("%d", cfg.MaxClients), fmt.Sprintf("%d", defaults.MaxClients))
	setField(m.Fields, "max_reconnect", fmt.Sprintf("%d", cfg.MaxReconnectAttempts), fmt.Sprintf("%d", defaults.MaxReconnectAttempts))
	setField(m.Fields, "max_auth_fail", fmt.Sprintf("%d", cfg.MaxFailedAttempts), fmt.Sprintf("%d", defaults.MaxFailedAttempts))

	arVal := "off"
	if cfg.AutoReconnect {
		arVal = "on"
	}
	arDef := "off"
	if defaults.AutoReconnect {
		arDef = "on"
	}
	setField(m.Fields, "auto_reconnect", arVal, arDef)
	setField(m.Fields, "reconnect_limit", fmt.Sprintf("%d", cfg.AutoReconnectAttempts), fmt.Sprintf("%d", defaults.AutoReconnectAttempts))

	aecVal := "off"
	if cfg.AEC {
		aecVal = "on"
	}
	setField(m.Fields, "echo_cancellation", aecVal, "off")

	// Mode field (server only)
	modeVal := "normal (server → client)"
	if cfg.Conference {
		modeVal = "conference (multi-user)"
	} else if cfg.Duplex {
		modeVal = "duplex (bidirectional)"
	} else if cfg.Reverse {
		modeVal = "reverse (client → server)"
	}
	modeDef := "normal (server → client)"
	setField(m.Fields, "mode", modeVal, modeDef)

	// Advanced fields
	setField(m.AdvancedFields, "log_level", cfg.LogLevel, defaults.LogLevel)

	tlsVal := "off"
	if cfg.TLS {
		tlsVal = "on"
	}
	setField(m.AdvancedFields, "tls", tlsVal, "off")
	setField(m.AdvancedFields, "tls_cert", cfg.TLSCert, defaults.TLSCert)
	setField(m.AdvancedFields, "tls_key", cfg.TLSKey, defaults.TLSKey)
	setField(m.AdvancedFields, "sample_rate", FormatSampleRate(cfg.SampleRate), FormatSampleRate(defaults.SampleRate))

	chVal := "mono"
	if cfg.Channels >= 2 {
		chVal = "stereo"
	}
	chDef := "mono"
	if defaults.Channels >= 2 {
		chDef = "stereo"
	}
	setField(m.AdvancedFields, "channels", chVal, chDef)
	setField(m.AdvancedFields, "opus_bitrate", FormatBitrate(cfg.OpusBitrate), FormatBitrate(defaults.OpusBitrate))

	simdVal := "on"
	if cfg.NoSIMDOptimization {
		simdVal = "off"
	}
	setField(m.AdvancedFields, "use_simd", simdVal, "on")

	hwidVal := "off"
	if cfg.HWIDRequired {
		hwidVal = "on"
	}
	setField(m.AdvancedFields, "hwid_collection", hwidVal, "off")

	setField(m.AdvancedFields, "rate_limit", fmt.Sprintf("%d", cfg.RateLimit), fmt.Sprintf("%d", defaults.RateLimit))

	// Apply device selection from loaded config.
	m.applyDeviceSelection(cfg.Devices)
}

// applyClientModeSettings applies mode-specific settings from the stored loadedClientModes
// after the server probe has determined the current mode.
func (m *SetupModel) applyClientModeSettings(serverMode string) {
	if len(m.loadedClientModes) == 0 {
		return
	}

	modeData, ok := m.loadedClientModes[serverMode]
	if !ok {
		return
	}

	// Apply mode settings to a temporary config, then use it to set fields
	modeCfg := config.DefaultConfig()
	modeCfg.ApplyClientModeData(modeData)
	defaults := config.DefaultConfig()

	setField := func(fields []SetupField, key, value, defaultValue string) {
		for i := range fields {
			if fields[i].Key == key {
				if value != defaultValue {
					fields[i].SetValue(value, SourceConfig)
				} else {
					fields[i].SetValue(value, SourceDefault)
				}
				return
			}
		}
	}

	// Connection fields
	arVal := "off"
	if modeCfg.AutoReconnect {
		arVal = "on"
	}
	setField(m.Fields, "auto_reconnect", arVal, "off")
	if modeCfg.AutoReconnectAttempts != defaults.AutoReconnectAttempts {
		setField(m.Fields, "reconnect_limit", fmt.Sprintf("%d", modeCfg.AutoReconnectAttempts), fmt.Sprintf("%d", defaults.AutoReconnectAttempts))
	}
	if modeCfg.MaxReconnectAttempts != defaults.MaxReconnectAttempts {
		setField(m.Fields, "max_reconnect", fmt.Sprintf("%d", modeCfg.MaxReconnectAttempts), fmt.Sprintf("%d", defaults.MaxReconnectAttempts))
	}

	// AEC
	aecVal := "off"
	if modeCfg.AEC {
		aecVal = "on"
	}
	setField(m.Fields, "echo_cancellation", aecVal, "off")

	// Advanced fields
	if modeCfg.LogLevel != defaults.LogLevel {
		setField(m.AdvancedFields, "log_level", modeCfg.LogLevel, defaults.LogLevel)
	}
	if modeCfg.NoSIMDOptimization {
		setField(m.AdvancedFields, "use_simd", "off", "on")
	}

	// Apply device selection
	m.applyDeviceSelection(modeCfg.Devices)

	// Clear loaded modes after application
	m.loadedClientModes = nil
}

// applyLoadedProfile applies a loaded profile to the setup fields.
// Only nickname and modes are applied — address/port/password are not touched.
// If a probe result already exists, mode settings are applied immediately.
func (m *SetupModel) applyLoadedProfile(cfg config.Config) {
	// Apply nickname if set
	if cfg.Nickname != "" {
		for i := range m.Fields {
			if m.Fields[i].Key == "nickname" {
				m.Fields[i].SetValue(cfg.Nickname, SourceConfig)
				break
			}
		}
	}

	// Store client modes for later application after probe
	if len(cfg.ClientModes) > 0 {
		m.loadedClientModes = cfg.ClientModes
	}

	// If probe already done, apply mode settings immediately
	if m.probeResult != nil && len(m.loadedClientModes) > 0 {
		m.applyClientModeSettings(m.probeResult.Mode)
	}

	m.applyFieldDependencies()
}

// applyDeviceSelection applies device entries to the multiSelect map.
// Device IDs can overlap between input and output, so match by Type+ID first,
// then fall back to exact Name match if ID changed (e.g. after reboot).
func (m *SetupModel) applyDeviceSelection(devices []config.DeviceEntry) {
	if !m.unifiedDuplex || len(devices) == 0 {
		return
	}
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.mixInputs = make(map[string]map[string]bool)
	// Build lookups: by ID and by Name, separated by device type.
	inputByID := make(map[uint32]string, len(m.inputDevices))
	inputByName := make(map[string]int)
	for _, d := range m.inputDevices {
		inputByID[d.ID] = d.Name
		inputByName[d.Name]++
	}
	outputByID := make(map[uint32]string, len(m.outputDevices))
	outputByName := make(map[string]int)
	for _, d := range m.outputDevices {
		outputByID[d.ID] = d.Name
		outputByName[d.Name]++
	}

	for _, de := range devices {
		var name string
		var ok bool
		isInput := false

		var byID map[uint32]string
		var byName map[string]int
		switch de.Type {
		case config.DeviceInput:
			byID = inputByID
			byName = inputByName
			isInput = true
		case config.DeviceOutput:
			byID = outputByID
			byName = outputByName
		default:
			switch de.Role {
			case config.RoleCapture:
				byID = inputByID
				byName = inputByName
				isInput = true
			case config.RolePlayback:
				byID = outputByID
				byName = outputByName
			default:
				byID = inputByID
				byName = inputByName
				isInput = true
			}
		}

		name, ok = byID[de.ID]
		if !ok {
			if de.Type == "" {
				if de.Role != config.RoleCapture && de.Role != config.RolePlayback {
					name, ok = outputByID[de.ID]
					if ok {
						isInput = false
					}
				}
			}
		}

		if !ok && de.Name != "" {
			if byName[de.Name] == 1 {
				name = de.Name
				ok = true
			}
		}

		if !ok {
			continue
		}
		key := deviceRow{Name: name, ID: de.ID, IsInput: isInput}.selectKey()
		roles := m.multiSelect[key]
		if de.Role == config.RoleCapture {
			roles.Capture = true
		} else if de.Role == config.RolePlayback {
			roles.Playback = true
		} else if !m.isDuplexMode {
			roles.Capture = true
		}
		m.multiSelect[key] = roles

		// Restore mix input for virtual output devices.
		if !isInput && de.MixInputID != nil {
			m.restoreMixInput(key, *de.MixInputID, de.MixInputName)
		}
	}
}

// restoreMixInput finds a matching input device by ID (fallback: name) and adds it to mixInputs.
func (m *SetupModel) restoreMixInput(outputKey string, mixID uint32, mixName string) {
	for _, inp := range m.inputDevices {
		if inp.ID == mixID || (mixName != "" && inp.Name == mixName) {
			if m.mixInputs == nil {
				m.mixInputs = make(map[string]map[string]bool)
			}
			if m.mixInputs[outputKey] == nil {
				m.mixInputs[outputKey] = make(map[string]bool)
			}
			m.mixInputs[outputKey][inp.selectKey()] = true
			return
		}
	}
}
