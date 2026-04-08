package views

import (
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
)

// Update handles input for the setup screen.
func (m SetupModel) Update(msg tea.Msg) (SetupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		devH := m.deviceListHeight()
		m.DeviceList.SetSize(m.leftColumnWidth()-2, devH)
		if m.HasOutputList {
			m.OutputDeviceList.SetSize(m.leftColumnWidth()-2, devH)
		}
		return m, nil

	case DiscoveryResultMsg:
		m.discoveryScanning = false
		m.serverList.SetScanning(false)
		if msg.Err == nil {
			m.discoveredServers = msg.Servers
			// Convert discovered servers to ServerEntry list
			var mdnsEntries []ServerEntry
			for _, s := range msg.Servers {
				ip := ""
				if len(s.AddrIPv4) > 0 {
					ip = s.AddrIPv4[0].String()
				} else if len(s.AddrIPv6) > 0 {
					ip = s.AddrIPv6[0].String()
				}
				mdnsEntries = append(mdnsEntries, ServerEntry{
					Address:     ip,
					Port:        s.Port,
					Hostname:    s.Name,
					ServerID:    s.ServerID,
					Source:      ServerSourceMDNS,
					ProbeStatus: ServerProbePending,
				})
			}

			// Merge with existing entries (recent servers), get list of new ones to probe
			newEntries := m.serverList.MergeDiscoveredEntries(mdnsEntries)

			var cmds []tea.Cmd

			// Start probes for newly added entries
			if len(newEntries) > 0 {
				cmds = append(cmds, StartAllProbesCmd(newEntries), tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return ServerProbeSpinnerMsg{} }))
			}

			// No auto-select — user always picks from server list or enters address manually

			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case ValidationDismissMsg:
		if !m.validationTimer.IsZero() && time.Now().After(m.validationTimer) {
			m.validationError = ""
			m.validationTimer = time.Time{}
		}
		return m, nil

	case DiscoveryTickMsg:
		if m.discoveryScanning {
			m.discoveryFrame++
			m.serverList.AdvanceSpinner() // also advance server list spinner for "Scanning..." text
			nextTick := tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return DiscoveryTickMsg{} })
			return m, nextTick
		}
		return m, nil

	case FlashDismissMsg:
		if !m.flashTimer.IsZero() && time.Now().After(m.flashTimer) {
			m.flashMsg = ""
			m.flashTimer = time.Time{}
		}
		return m, nil

	case ProbeDebounceMsg:
		// Only probe if address+port still match (debounce dedup)
		target := msg.Addr + ":" + msg.Port
		if m.probeAddr == target {
			return m, StartProbeCmd(msg.Addr, msg.Port)
		}
		return m, nil

	case ServerSelectedMsg:
		// Fill Address/Port from selected server (SourceDefault so no ✓ until probe succeeds)
		for i := range m.Fields {
			switch m.Fields[i].Key {
			case "server_address":
				m.Fields[i].SetValue(msg.Entry.Address, SourceDefault)
			case "port":
				m.Fields[i].SetValue(fmt.Sprintf("%d", msg.Entry.Port), SourceDefault)
			}
		}
		// If this is a re-select of the same server (already has probe result),
		// force a fresh probe to pick up server-side changes.
		if msg.Entry.ProbeResult != nil {
			// Apply cached result for now (UI stays responsive)
			m.probeResult = msg.Entry.ProbeResult
			m.probeStatus = "probing"
			m.probeError = ""
			var isDuplex bool
			m.Fields, m.AdvancedFields, isDuplex = ApplyProbeResult(m.Fields, m.AdvancedFields, msg.Entry.ProbeResult)
			m.isDuplexMode = isDuplex
			m.isConferenceMode = msg.Entry.ProbeResult.Mode == "conference"
			m.applyFieldDependencies()
			// Always re-probe to get fresh data
			m.probeAddr = msg.Entry.Address + ":" + fmt.Sprintf("%d", msg.Entry.Port)
			return m, StartProbeCmd(msg.Entry.Address, fmt.Sprintf("%d", msg.Entry.Port))
		}
		// Trigger probe
		cmd := m.startProbeIfReady()
		return m, cmd

	case ScanRequestMsg:
		// Re-run mDNS discovery + re-probe all servers
		m.discoveryScanning = true
		m.serverList.SetScanning(true)
		m.serverList.SetAllProbing()
		discoveryTick := tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return DiscoveryTickMsg{} })
		spinnerTick := tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return ServerProbeSpinnerMsg{} })
		var cmds []tea.Cmd
		cmds = append(cmds, StartDiscoveryCmd(3*time.Second), discoveryTick, spinnerTick)
		if m.serverList.HasEntries() {
			cmds = append(cmds, StartAllProbesCmd(m.serverList.Entries()))
		}
		return m, tea.Batch(cmds...)

	case ServerProbeMsg:
		// Parallel probe result for a server in the list
		selectedEntry := m.serverList.UpdateProbeResult(msg.Address, msg.Result, msg.Err)
		if selectedEntry != nil {
			if selectedEntry.ProbeResult != nil {
				// This is the currently selected server — apply probe result to fields
				m.probeResult = selectedEntry.ProbeResult
				m.probeStatus = "ok"
				m.probeError = ""
				var isDuplex bool
				m.Fields, m.AdvancedFields, isDuplex = ApplyProbeResult(m.Fields, m.AdvancedFields, selectedEntry.ProbeResult)
				m.isDuplexMode = isDuplex
				m.isConferenceMode = selectedEntry.ProbeResult.Mode == "conference"
				m.applyFieldDependencies()
				// Try to auto-restore devices for this server + mode
				if restoreCmd := m.tryShowRestoreOverlay(selectedEntry.Address, selectedEntry.Port, selectedEntry.ProbeResult.Mode); restoreCmd != nil {
					return m, restoreCmd
				}
			} else if selectedEntry.ProbeStatus == ServerProbeOffline {
				// Selected server went offline — clear probe result and show hint
				m.probeResult = nil
				m.probeStatus = "error"
				m.probeError = "⚠ server went offline"
				for i := range m.Fields {
					if m.Fields[i].Key == "server_address" {
						m.Fields[i].Hint = "⚠ server went offline"
						break
					}
				}
			}
		}
		return m, nil

	case ServerListExitDownMsg:
		// Server list signaled Down at bottom — move focus to fields
		m.serverListFocused = false
		m.FieldCursor = m.firstVisibleField()
		return m, nil

	case ServerProbeSpinnerMsg:
		// Advance spinner for probing servers
		if m.serverList.HasProbingServers() {
			m.serverList.AdvanceSpinner()
			return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return ServerProbeSpinnerMsg{} })
		}
		return m, nil

	case ProbeServerMsg:
		// Update server list entry status so the list reflects current probe result
		m.serverList.UpdateProbeResult(msg.Addr, msg.Result, msg.Err)

		if msg.Result != nil {
			m.probeStatus = "ok"
			m.probeError = ""
			m.probeResult = msg.Result
			var isDuplex bool
			m.Fields, m.AdvancedFields, isDuplex = ApplyProbeResult(m.Fields, m.AdvancedFields, msg.Result)
			m.isDuplexMode = isDuplex
			m.isConferenceMode = msg.Result.Mode == "conference"
			// Apply stored client mode settings from loaded config
			if len(m.loadedClientModes) > 0 {
				m.applyClientModeSettings(msg.Result.Mode)
			}
			m.applyFieldDependencies()
			// Update address hint
			for i := range m.Fields {
				if m.Fields[i].Key == "server_address" {
					hint := "✓ Server reachable"
					if msg.Result.ServerName != "" {
						hint += " (" + msg.Result.ServerName + ")"
					}
					if msg.Result.CurrentClients >= msg.Result.MaxClients {
						hint += " ⚠ server full (" + fmt.Sprintf("%d/%d", msg.Result.CurrentClients, msg.Result.MaxClients) + ")"
					}
					m.Fields[i].Hint = hint
				}
			}
			// Try to show restore overlay for manually entered server
			var probeAddr, probePort string
			for _, f := range m.Fields {
				if f.Key == "server_address" {
					probeAddr = f.Value
				}
				if f.Key == "port" {
					probePort = f.Value
				}
			}
			if portNum, err := strconv.Atoi(probePort); err == nil {
				if restoreCmd := m.tryShowRestoreOverlay(probeAddr, portNum, msg.Result.Mode); restoreCmd != nil {
					return m, restoreCmd
				}
			}
		} else {
			m.probeStatus = "error"
			m.probeError = "⚠ Server unavailable"
			m.probeResult = nil
			for i := range m.Fields {
				if m.Fields[i].Key == "server_address" {
					m.Fields[i].Hint = m.probeError
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Pass to active component
	if m.activeColumn == ColumnDevices {
		var cmd tea.Cmd
		dl := m.ActiveDeviceList()
		*dl, cmd = dl.Update(msg)
		return m, cmd
	}

	// If editing, forward to the field
	if m.inputMode == ModeEditing {
		fields := m.activeFields()
		if m.FieldCursor < len(fields) {
			cmd := fields[m.FieldCursor].Update(msg)
			m.writeBackFields(fields)
			return m, cmd
		}
	}

	return m, nil
}

func (m SetupModel) handleKey(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	// Priority 1: overlay active
	if m.overlay != SetupOverlayNone {
		return m.handleOverlayKey(msg)
	}

	// Priority 2: editing a field
	if m.inputMode == ModeEditing {
		return m.handleEditingKey(msg)
	}

	// Priority 3: device list filter mode (only for non-unified legacy mode)
	if m.activeColumn == ColumnDevices && !m.unifiedDuplex {
		dl := m.ActiveDeviceList()
		if dl.FilterState() == list.Filtering {
			m.inputMode = ModeFilter
			var cmd tea.Cmd
			*dl, cmd = dl.Update(msg)
			if dl.FilterState() != list.Filtering {
				m.inputMode = ModeNormal
			}
			return m, cmd
		}
	}

	// Priority 4: global shortcuts that should always work regardless of server list focus
	if msg.Type == tea.KeyCtrlS || msg.Type == tea.KeyCtrlL || msg.Type == tea.KeyCtrlH || msg.Type == tea.KeyCtrlY || msg.Type == tea.KeyCtrlE || msg.Type == tea.KeyCtrlW {
		// Fall through to Priority 5 where these are handled
		return m.handleGlobalShortcut(msg)
	}

	// Priority 4b: delegate to server list when focused (client mode)
	if m.activeColumn == ColumnSettings && m.serverListFocused && m.cfg.Mode == config.ModeClient {
		// Left arrow from server list entries (not Scan) → switch to Devices column
		if msg.Type == tea.KeyLeft && !m.serverList.IsScanFocused() {
			if !m.isServerReady() {
				return m, nil
			}
			m.activeColumn = ColumnDevices
			m.serverListFocused = false
			if m.unifiedDuplex {
				showInput, _ := m.visibleSections()
				if showInput {
					m.DeviceSection = SectionInput
				} else {
					m.DeviceSection = SectionOutput
				}
				m.deviceCursor = 0
			}
			return m, nil
		}

		var updatedList ServerListModel
		var cmd tea.Cmd
		updatedList, cmd = m.serverList.Update(msg)
		m.serverList = updatedList

		// Handle messages returned by the server list
		if cmd != nil {
			// Execute the cmd to get the message, then check its type
			// We need to wrap and return it so the Bubble Tea runtime processes it
			return m, cmd
		}
		return m, nil
	}

	// Priority 5: normal mode
	switch msg.Type {
	case tea.KeyTab:
		// Tab in Devices column: cycle Input → Output (duplex modes), skip hidden sections
		if m.activeColumn == ColumnDevices && m.unifiedDuplex {
			showInput, showOutput := m.visibleSections()
			if m.DeviceSection == SectionInput && showOutput {
				m.DeviceSection = SectionOutput
				m.deviceCursor = 0
			} else if m.DeviceSection == SectionOutput && showInput {
				m.DeviceSection = SectionInput
				m.deviceCursor = 0
			}
			return m, nil
		}
		// Tab in Settings or non-duplex Devices: no-op
		return m, nil

	case tea.KeyShiftTab:
		// ShiftTab in Devices column: cycle Output → Input (duplex modes), skip hidden sections
		if m.activeColumn == ColumnDevices && m.unifiedDuplex {
			showInput, _ := m.visibleSections()
			if m.DeviceSection == SectionOutput && showInput {
				m.DeviceSection = SectionInput
				m.deviceCursor = 0
			}
			// If already on Input, ShiftTab is no-op
			return m, nil
		}
		// ShiftTab in Settings or non-duplex Devices: no-op
		return m, nil

	case tea.KeyCtrlUp:
		if m.activeColumn == ColumnDevices && m.DeviceSection == SectionOutput {
			showInput, _ := m.visibleSections()
			if showInput {
				m.DeviceSection = SectionInput
				m.deviceCursor = 0
			}
			return m, nil
		}

	case tea.KeyCtrlDown:
		if m.activeColumn == ColumnDevices && m.DeviceSection == SectionInput {
			_, showOutput := m.visibleSections()
			if showOutput {
				m.DeviceSection = SectionOutput
				m.deviceCursor = 0
			}
			return m, nil
		}

	case tea.KeyCtrlQ, tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyUp:
		if m.activeColumn == ColumnSettings {
			// Check if at first visible field — transition to server list
			firstVisible := m.firstVisibleField()
			if m.FieldCursor <= firstVisible && m.cfg.Mode == config.ModeClient {
				m.serverListFocused = true
				// If no entries, go directly to Scan button
				if !m.serverList.HasEntries() {
					m.serverList.FocusScan()
				}
				return m, nil
			}
			fields := m.activeFields()
			for next := m.FieldCursor - 1; next >= 0; next-- {
				if !fields[next].Hidden {
					m.FieldCursor = next
					break
				}
			}
			return m, nil
		}
		if m.activeColumn == ColumnDevices && m.unifiedDuplex {
			if m.deviceCursor > 0 {
				m.deviceCursor--
			}
			return m, nil
		}

	case tea.KeyDown:
		if m.activeColumn == ColumnSettings {
			fields := m.activeFields()
			for next := m.FieldCursor + 1; next < len(fields); next++ {
				if !fields[next].Hidden {
					m.FieldCursor = next
					break
				}
			}
			return m, nil
		}
		if m.activeColumn == ColumnDevices && m.unifiedDuplex {
			maxIdx := m.currentSectionRowCount() - 1
			if m.deviceCursor < maxIdx {
				m.deviceCursor++
			}
			return m, nil
		}

	case tea.KeyLeft:
		// In Devices column with AGC focus: go back to device select column
		if m.activeColumn == ColumnDevices && m.unifiedDuplex && m.deviceColumn == 1 {
			m.deviceColumn = 0
			return m, nil
		}
		// In Settings column: switch to Devices column
		if m.activeColumn == ColumnSettings {
			// For client mode: block if no server ready
			if m.cfg.Mode == config.ModeClient && !m.isServerReady() {
				return m, nil
			}
			m.activeColumn = ColumnDevices
			m.serverListFocused = false
			if m.unifiedDuplex {
				showInput, _ := m.visibleSections()
				if showInput {
					m.DeviceSection = SectionInput
				} else {
					m.DeviceSection = SectionOutput
				}
				m.deviceCursor = 0
			}
			return m, nil
		}

	case tea.KeyRight:
		// In Devices column (unified): switch to AGC column if device is selected
		if m.activeColumn == ColumnDevices && m.unifiedDuplex && m.deviceColumn == 0 {
			dev := m.currentCursorDevice()
			if dev != nil {
				if _, ok := m.multiSelect[dev.selectKey()]; ok {
					m.deviceColumn = 1
					return m, nil
				}
			}
			// Not selected — go to settings
			m.activeColumn = ColumnSettings
			if m.cfg.Mode == config.ModeClient && m.serverList.HasEntries() {
				m.serverListFocused = true
			}
			return m, nil
		}
		// In Devices column AGC: switch to Settings column
		if m.activeColumn == ColumnDevices && m.unifiedDuplex && m.deviceColumn == 1 {
			m.deviceColumn = 0
			m.activeColumn = ColumnSettings
			if m.cfg.Mode == config.ModeClient && m.serverList.HasEntries() {
				m.serverListFocused = true
			}
			return m, nil
		}
		// In Devices column (non-unified): switch to Settings column
		if m.activeColumn == ColumnDevices {
			m.activeColumn = ColumnSettings
			if m.cfg.Mode == config.ModeClient && m.serverList.HasEntries() {
				m.serverListFocused = true
			}
			return m, nil
		}

	case tea.KeySpace, tea.KeyRunes:
		// On Windows, Space arrives as KeyRunes with rune ' ' instead of KeySpace.
		isSpace := msg.Type == tea.KeySpace || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == ' ')

		// +/- volume adjustment in unified device column
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && m.activeColumn == ColumnDevices && m.unifiedDuplex {
			r := msg.Runes[0]
			if r == '+' || r == '=' || r == '-' {
				return m.handleVolumeAdjust(r == '-')
			}
		}

		if isSpace {
			if m.activeColumn == ColumnDevices && m.unifiedDuplex {
				// AGC column: toggle AGC
				if m.deviceColumn == 1 {
					return m.handleAGCToggle()
				}
				return m.handleMultiSelectToggle()
			}
			if m.activeColumn == ColumnSettings {
				return m.handleFieldActivation()
			}
		}

	case tea.KeyCtrlS:
		saveOverlay := NewConfigSaveOverlay(m.lastLoadedConfigName, m.cfg.Mode, m.BuildConfig())
		m.configSaveOverlay = &saveOverlay
		m.overlay = SetupOverlayConfigSave
		return m, nil

	case tea.KeyCtrlL:
		loadOverlay := NewConfigLoadOverlay(m.cfg.Mode)
		m.configLoadOverlay = &loadOverlay
		m.overlay = SetupOverlayConfigLoad
		return m, nil

	case tea.KeyCtrlH:
		deviceName := m.SelectedDeviceName()
		m.summaryOverlay = NewSummaryOverlay(m.BuildConfig(), deviceName)
		m.overlay = SetupOverlaySummary
		return m, nil

	case tea.KeyCtrlE:
		return m.openVirtualMicOverlay()

	case tea.KeyCtrlW:
		langOverlay := NewLanguageOverlay()
		m.languageOverlay = langOverlay
		m.overlay = SetupOverlayLanguage
		return m, nil

	case tea.KeyCtrlY:
		clipCmd := BuildCLICommand(m.BuildConfig(), true)
		copyToClipboard(clipCmd)
		m.flashMsg = "Copied!"
		m.flashTimer = time.Now().Add(2 * time.Second)
		flashCmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg { return FlashDismissMsg{} })
		return m, flashCmd
	}

	// Forward to device list if in device column (legacy non-unified mode)
	if m.activeColumn == ColumnDevices && !m.unifiedDuplex {
		var cmd tea.Cmd
		dl := m.ActiveDeviceList()
		*dl, cmd = dl.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleGlobalShortcut handles Ctrl+S/L/H/Y/E shortcuts that should work regardless of focus.
func (m SetupModel) handleGlobalShortcut(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlS:
		saveOverlay := NewConfigSaveOverlay(m.lastLoadedConfigName, m.cfg.Mode, m.BuildConfig())
		m.configSaveOverlay = &saveOverlay
		m.overlay = SetupOverlayConfigSave
		return m, nil
	case tea.KeyCtrlL:
		loadOverlay := NewConfigLoadOverlay(m.cfg.Mode)
		m.configLoadOverlay = &loadOverlay
		m.overlay = SetupOverlayConfigLoad
		return m, nil
	case tea.KeyCtrlH:
		deviceName := m.SelectedDeviceName()
		m.summaryOverlay = NewSummaryOverlay(m.BuildConfig(), deviceName)
		m.overlay = SetupOverlaySummary
		return m, nil
	case tea.KeyCtrlY:
		clipCmd := BuildCLICommand(m.BuildConfig(), true)
		copyToClipboard(clipCmd)
		m.flashMsg = "Copied!"
		m.flashTimer = time.Now().Add(2 * time.Second)
		flashCmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg { return FlashDismissMsg{} })
		return m, flashCmd
	case tea.KeyCtrlE:
		return m.openVirtualMicOverlay()
	case tea.KeyCtrlW:
		langOverlay := NewLanguageOverlay()
		m.languageOverlay = langOverlay
		m.overlay = SetupOverlayLanguage
		return m, nil
	}
	return m, nil
}

func (m SetupModel) handleOverlayKey(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	switch m.overlay {
	case SetupOverlaySummary:
		if m.summaryOverlay != nil {
			launch, closed, overlayCmd := m.summaryOverlay.Update(msg)
			if launch {
				m.overlay = SetupOverlayNone
				m.summaryOverlay = nil
				return m.tryStart()
			}
			if closed {
				m.overlay = SetupOverlayNone
				m.summaryOverlay = nil
			}
			if overlayCmd != nil {
				// Propagate flash cmd and set flash state
				m.flashMsg = "Copied!"
				m.flashTimer = time.Now().Add(2 * time.Second)
				return m, overlayCmd
			}
		}

	case SetupOverlayVirtualDevice:
		if m.virtualDeviceOverlay != nil {
			action := m.virtualDeviceOverlay.Update(msg)
			switch action {
			case VirtualActionCreate:
				moduleID, err := createPulseAudioSink("EchoWarp")
				if err != nil {
					m.virtualDeviceOverlay.Error = err.Error()
					return m, nil
				}
				m.virtualMicCreated = true
				m.virtualMicModule = moduleID
				m.virtualDeviceOverlay = nil
				// Update field label
				m.updateVirtualMicField(true)
				// Re-enumerate devices from OS (PulseAudio needs time to register)
				time.Sleep(200 * time.Millisecond)
				m.refreshDevicesFromOS()
				// Refresh device list and auto-select
				m.rebuildDeviceGroups()
				m.autoSelectVirtualDevice("EchoWarp")
				// Show lifecycle options overlay
				m.virtualSinkLifecycleOverlay = NewVirtualSinkLifecycleOverlay()
				m.overlay = SetupOverlayVirtualSinkLifecycle
				return m, nil
			case VirtualActionRemove:
				if m.virtualMicModule != "" {
					_ = RemovePulseAudioSink(m.virtualMicModule)
				}
				m.virtualMicCreated = false
				m.virtualMicModule = ""
				m.overlay = SetupOverlayNone
				m.virtualDeviceOverlay = nil
				m.updateVirtualMicField(false)
				// Re-enumerate devices from OS after sink removal
				time.Sleep(200 * time.Millisecond)
				m.refreshDevicesFromOS()
				m.rebuildDeviceGroups()
				flashCmd := m.SetFlash("Virtual mic removed", 3*time.Second)
				return m, flashCmd
			case VirtualActionCancel:
				m.overlay = SetupOverlayNone
				m.virtualDeviceOverlay = nil
			}
		}

	case SetupOverlayConfigSave:
		if m.configSaveOverlay != nil {
			action, name := m.configSaveOverlay.HandleKey(msg)
			switch action {
			case ConfigSaveDone:
				path := filepath.Join(config.ConfigsDir(m.cfg.Mode), name+".yaml")
				m.overlay = SetupOverlayNone
				saveAsProfile := m.configSaveOverlay.saveAsProfile
				m.configSaveOverlay = nil
				cfg := m.BuildConfig()
				var saveErr error
				if saveAsProfile {
					saveErr = config.SaveProfile(cfg, path)
				} else {
					saveErr = config.SaveNonDefault(cfg, m.cfg.Mode, path)
				}
				if saveErr != nil {
					flashCmd := m.SetFlash("Save failed: "+saveErr.Error(), 5*time.Second)
					return m, flashCmd
				}
				label := "Config saved: "
				if saveAsProfile {
					label = "Profile saved: "
				}
				flashCmd := m.SetFlash(label+name, 3*time.Second)
				return m, flashCmd
			case ConfigSaveCancelled:
				m.overlay = SetupOverlayNone
				m.configSaveOverlay = nil
			}
		}

	case SetupOverlayConfigLoad:
		if m.configLoadOverlay != nil {
			action, entry := m.configLoadOverlay.HandleKey(msg)
			switch action {
			case ConfigLoadSelected, ConfigLoadAsProfile:
				if entry != nil {
					if loadedCfg, err := config.LoadFromFile(entry.Path); err == nil {
						asProfile := action == ConfigLoadAsProfile || config.IsProfile(loadedCfg)
						if asProfile {
							m.applyLoadedProfile(loadedCfg)
						} else {
							m.applyLoadedConfig(loadedCfg)
						}
						m.lastLoadedConfigName = entry.Name
						m.overlay = SetupOverlayNone
						m.configLoadOverlay = nil

						label := "Loaded: "
						if asProfile {
							label = "Loaded profile: "
						}
						flashCmd := m.SetFlash(label+entry.Name, 3*time.Second)

						// Client mode: trigger probe to verify server (only for configs, not profiles)
						// For profiles: probe only if address/port already filled
						if m.cfg.Mode == config.ModeClient {
							probeCmd := m.startProbeIfReady()
							if probeCmd != nil {
								return m, tea.Batch(flashCmd, probeCmd)
							}
						}
						return m, flashCmd
					}
				}
				m.overlay = SetupOverlayNone
				m.configLoadOverlay = nil
			case ConfigLoadDeleted:
				if entry != nil {
					flashCmd := m.SetFlash("Deleted: "+entry.Name, 3*time.Second)
					return m, flashCmd
				}
			case ConfigLoadCancelled:
				m.overlay = SetupOverlayNone
				m.configLoadOverlay = nil
			}
		}

	case SetupOverlayVirtualSinkLifecycle:
		if m.virtualSinkLifecycleOverlay != nil {
			action := m.virtualSinkLifecycleOverlay.Update(msg)
			switch action {
			case VSLifecycleSave:
				m.virtualSinkOnStop = m.virtualSinkLifecycleOverlay.OnStop()
				m.virtualSinkOnStart = m.virtualSinkLifecycleOverlay.OnStart()
				m.overlay = SetupOverlayNone
				m.virtualSinkLifecycleOverlay = nil
				flashCmd := m.SetFlash("✓ Virtual mic created — EchoWarp", 3*time.Second)
				return m, flashCmd
			case VSLifecycleCancel:
				// Use defaults
				m.virtualSinkOnStop = recent.SinkDelete
				m.virtualSinkOnStart = recent.SinkRecreate
				m.overlay = SetupOverlayNone
				m.virtualSinkLifecycleOverlay = nil
				flashCmd := m.SetFlash("✓ Virtual mic created — EchoWarp", 3*time.Second)
				return m, flashCmd
			}
		}

	case SetupOverlayLanguage:
		if m.languageOverlay != nil {
			action := m.languageOverlay.Update(msg)
			switch action {
			case LanguageActionSelected:
				m.overlay = SetupOverlayNone
				m.languageOverlay = nil
				// Fields use i18n.T() labels — rebuild to pick up new translations
				m.Fields = buildMainFields(m.cfg)
				m.AdvancedFields = buildAdvancedFields(m.cfg)
				return m, nil
			case LanguageActionCancel:
				m.overlay = SetupOverlayNone
				m.languageOverlay = nil
			}
		}

	case SetupOverlayRestore:
		if m.restoreOverlay != nil {
			action := m.restoreOverlay.Update(msg)
			switch action {
			case RestoreActionAll:
				m.overlay = SetupOverlayNone
				if m.restoreOverlay.VirtualOnly {
					// TODO: trigger virtual device creation for m.restoreOverlay.MissingVirtual
					m.restoreOverlay = nil
				} else {
					cmd := m.applyRestore(m.restoreOverlay.Preset, false)
					m.restoreOverlay = nil
					return m, cmd
				}
			case RestoreActionWithoutVirtual:
				m.overlay = SetupOverlayNone
				cmd := m.applyRestore(m.restoreOverlay.Preset, true)
				m.restoreOverlay = nil
				return m, cmd
			case RestoreActionSkip:
				if m.presetDismissed == nil {
					m.presetDismissed = make(map[string]bool)
				}
				m.presetDismissed[m.restoreOverlay.Mode] = true
				m.overlay = SetupOverlayNone
				m.restoreOverlay = nil
			}
		}
	}

	return m, nil
}

func (m SetupModel) handleEditingKey(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	fields := m.activeFields()
	if m.FieldCursor >= len(fields) {
		return m, nil
	}
	f := &fields[m.FieldCursor]

	switch msg.Type {
	case tea.KeyEnter:
		if f.ConfirmEditing() {
			m.inputMode = ModeNormal
			m.applyFieldDependencies()
		}
		m.writeBackFields(fields)
		// Trigger server probe on address or port change (client mode)
		if m.cfg.Mode == config.ModeClient && (f.Key == "server_address" || f.Key == "port") {
			cmd := m.startProbeIfReady()
			return m, cmd
		}
		return m, nil
	case tea.KeyEsc:
		f.CancelEditing()
		m.inputMode = ModeNormal
		m.writeBackFields(fields)
		return m, nil
	default:
		cmd := f.Update(msg)
		m.writeBackFields(fields)
		return m, cmd
	}
}

func (m SetupModel) handleEnter() (SetupModel, tea.Cmd) {
	if m.activeColumn == ColumnDevices {
		return m.tryStart()
	}

	// Settings column
	return m.handleFieldActivation()
}

func (m SetupModel) handleFieldActivation() (SetupModel, tea.Cmd) {
	fields := m.activeFields()
	if m.FieldCursor >= len(fields) {
		return m, nil
	}
	f := &fields[m.FieldCursor]

	// Client mode: server-driven fields are locked until probe succeeds
	if m.cfg.Mode == config.ModeClient && m.probeResult == nil {
		clientLocalFields := map[string]bool{
			"server_address": true, "port": true,
			"max_reconnect": true,
			"log_level":     true, "echo_cancellation": true,
			"use_simd": true,
		}
		if !clientLocalFields[f.Key] && f.Type != FieldAction {
			return m, nil
		}
	}

	switch f.Type {
	case FieldText, FieldNumber:
		// Block editing for fields locked by server probe
		if f.Source == SourceAuto && f.Key != "server_address" {
			return m, nil
		}
		// Password not required by server — block editing
		if f.Key == "password" && f.Hint == "(not required)" {
			return m, nil
		}
		f.StartEditing()
		m.inputMode = ModeEditing
		m.writeBackFields(fields)
		return m, nil

	case FieldToggle:
		// Fields set by server probe are read-only
		if f.Source == SourceAuto {
			return m, nil
		}
		f.Toggle()
		m.writeBackFields(fields)
		m.applyFieldDependencies()
		// Server mode: clear device selection and restore preset after mode change
		if f.Key == "mode" && m.cfg.Mode == config.ModeServer {
			// Always clear device selections when switching modes —
			// carrying over devices from another mode is incorrect.
			m.multiSelect = make(map[string]DeviceRoleSet)
			// Clear stale flash from previous mode's preset restore
			m.flashMsg = ""
			if restoreCmd := m.tryShowServerRestoreOverlay(); restoreCmd != nil {
				return m, restoreCmd
			}
		}
		return m, nil

	case FieldSelect:
		// Fields set by server probe are read-only
		if f.Source == SourceAuto {
			return m, nil
		}
		// Cycle like toggle
		f.optIndex = (f.optIndex + 1) % len(f.Options)
		f.Value = f.Options[f.optIndex]
		f.Source = SourceUser
		m.writeBackFields(fields)
		return m, nil

	case FieldAction:
		if f.Key == "advanced" {
			m.advancedOpen = !m.advancedOpen
			f.ActionExpanded = m.advancedOpen
			m.writeBackFields(fields)
			return m, nil
		}
		if f.Key == "virtual_mic" {
			m.virtualDeviceOverlay = NewVirtualDeviceOverlay(m.virtualMicCreated, "EchoWarp")
			m.overlay = SetupOverlayVirtualDevice
			return m, nil
		}
		return m, nil
	}

	return m, nil
}
