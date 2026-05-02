package views

import (
	"fmt"
	"math"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
)

const (
	genericPlaybackName        = "Playback"
	genericPlaybackMonitorName = "Monitor of Playback"
)

// DeviceRoleSet tracks assigned roles for a device in multi-select mode.
type DeviceRoleSet struct {
	Capture  bool
	Playback bool
}

type presetDeviceKey struct {
	name    string
	isInput bool
}

// deviceRow holds display info for a device in the sectioned device list.
type deviceRow struct {
	Name            string
	BackendID       string
	VirtualSinkName string
	ID              uint32
	IsInput         bool
	IsVirtual       bool
	IsLoopback      bool
	Channels        uint32
	SampleRate      uint32
	BitDepth        uint32
	Volume          float64
	AGC             bool
}

// selectKey returns a unique key for this device in the multiSelect map,
// distinguishing input/output devices with the same name and ID collisions.
func (d deviceRow) selectKey() string {
	prefix := "O"
	if d.IsInput {
		prefix = "I"
	}
	return fmt.Sprintf("%s:%d:%s", prefix, d.ID, d.Name)
}

// IsVirtualDevice checks if a device name matches known virtual audio devices.
func IsVirtualDevice(name string) bool {
	lower := strings.ToLower(name)
	virtuals := []string{"echowarp", "blackhole", "vb-audio", "cable", "virtual", "loopback", "soundflower", "existential"}
	for _, v := range virtuals {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}

// IsLoopbackDevice checks if a device name indicates a loopback capture device.
func IsLoopbackDevice(name string) bool {
	return strings.Contains(name, "[loopback]")
}

// rebuildDeviceGroups splits DeviceList items into input/output groups.
func (m *SetupModel) rebuildDeviceGroups() {
	m.inputDevices = nil
	m.outputDevices = nil
	aliasContext := m.managedVirtualAliasContext()
	for _, item := range m.DeviceList.Items() {
		type deviceInfo interface {
			FilterValue() string
			DeviceID() uint32
		}
		type inputChecker interface {
			IsInputDevice() bool
		}
		type channelInfo interface {
			DeviceChannels() uint32
			DeviceSampleRate() uint32
		}
		type bitDepthInfo interface {
			DeviceBitDepth() uint32
		}
		type backendInfo interface {
			DeviceBackendID() string
		}
		di, ok := item.(deviceInfo)
		if !ok {
			continue
		}
		name := di.FilterValue()
		id := di.DeviceID()
		isInput := true
		if ic, ok := item.(inputChecker); ok {
			isInput = ic.IsInputDevice()
		}
		var ch, sr, bd uint32
		if ci, ok := item.(channelInfo); ok {
			ch = ci.DeviceChannels()
			sr = ci.DeviceSampleRate()
		}
		if bi, ok := item.(bitDepthInfo); ok {
			bd = bi.DeviceBitDepth()
		}
		backendID := ""
		if bi, ok := item.(backendInfo); ok {
			backendID = bi.DeviceBackendID()
		}
		virtual := m.resolveManagedVirtualDevice(name, backendID, isInput, aliasContext)
		row := deviceRow{
			Name:            virtual.displayName(name),
			BackendID:       backendID,
			VirtualSinkName: virtual.sinkName(),
			ID:              id,
			IsInput:         isInput,
			IsVirtual:       virtual.matched || IsVirtualDevice(name),
			IsLoopback:      IsLoopbackDevice(name),
			Channels:        ch,
			SampleRate:      sr,
			BitDepth:        bd,
			Volume:          1.0,
		}
		if isInput {
			m.inputDevices = append(m.inputDevices, row)
		} else {
			m.outputDevices = append(m.outputDevices, row)
		}
	}
	// Sort: real devices first, virtual devices last
	sortDevices := func(devs []deviceRow) {
		sort.SliceStable(devs, func(i, j int) bool {
			if devs[i].IsVirtual != devs[j].IsVirtual {
				return !devs[i].IsVirtual
			}
			return false
		})
	}
	sortDevices(m.inputDevices)
	sortDevices(m.outputDevices)
	disambiguateDuplicateGenericRows(m.outputDevices, genericPlaybackName)
	disambiguateDuplicateGenericRows(m.inputDevices, genericPlaybackMonitorName)
	m.syncVirtualMicState()
}

type managedVirtualDeviceMatch struct {
	isInput bool
	preset  recent.VirtualSinkPreset
	matched bool
}

func (m managedVirtualDeviceMatch) displayName(fallback string) string {
	if !m.matched {
		return fallback
	}
	return managedVirtualDisplayName(m.isInput, m.preset)
}

func (m managedVirtualDeviceMatch) sinkName() string {
	if !m.matched {
		return ""
	}
	return m.preset.SinkName
}

func (m SetupModel) resolveManagedVirtualDevice(
	name string,
	backendID string,
	isInput bool,
	context managedVirtualAliasContext,
) managedVirtualDeviceMatch {
	if match, ok := matchingManagedVirtualBackendPreset(backendID, isInput, context); ok {
		return managedVirtualDeviceMatch{isInput: isInput, preset: match, matched: true}
	}
	if backendID != "" && isGenericManagedVirtualName(name, isInput) {
		return managedVirtualDeviceMatch{}
	}
	matches := matchingManagedVirtualPresets(name, isInput, context)
	if len(matches) == 1 {
		return managedVirtualDeviceMatch{isInput: isInput, preset: matches[0], matched: true}
	}
	return managedVirtualDeviceMatch{}
}

func matchingManagedVirtualPresets(
	name string,
	isInput bool,
	context managedVirtualAliasContext,
) []recent.VirtualSinkPreset {
	matches := make([]recent.VirtualSinkPreset, 0, 1)
	for _, vs := range context.presets {
		if virtualSinkAliasMatches(name, isInput, vs, context) {
			matches = append(matches, vs)
		}
	}
	return matches
}

func matchingManagedVirtualBackendPreset(
	backendID string,
	isInput bool,
	context managedVirtualAliasContext,
) (recent.VirtualSinkPreset, bool) {
	if backendID == "" {
		return recent.VirtualSinkPreset{}, false
	}
	for _, vs := range context.presets {
		if virtualSinkBackendIDMatches(backendID, isInput, vs) {
			return vs, true
		}
	}
	return recent.VirtualSinkPreset{}, false
}

func virtualSinkBackendIDMatches(backendID string, isInput bool, vs recent.VirtualSinkPreset) bool {
	if backendID == "" || vs.SinkName == "" {
		return false
	}
	if isInput {
		return backendID == virtualSinkMonitorName(vs) || backendID == vs.SinkName+".monitor"
	}
	return backendID == vs.SinkName
}

func isGenericManagedVirtualName(name string, isInput bool) bool {
	if isInput {
		return name == genericPlaybackMonitorName
	}
	return name == genericPlaybackName
}

func (m SetupModel) managedVirtualSinkPresets() []recent.VirtualSinkPreset {
	seen := make(map[string]bool)
	presets := make([]recent.VirtualSinkPreset, 0, len(m.trackedVirtualSinks))
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		presets = appendUniqueVirtualPreset(presets, seen, m.trackedVirtualSinks[sinkName].Preset)
	}
	return appendStateVirtualPresets(presets, seen)
}

func appendStateVirtualPresets(
	presets []recent.VirtualSinkPreset,
	seen map[string]bool,
) []recent.VirtualSinkPreset {
	state, err := virtualstate.Load()
	if err != nil {
		return presets
	}
	for _, device := range state.Devices {
		if stateDeviceClassifiesVirtual(device) {
			presets = appendUniqueVirtualPreset(presets, seen, virtualSinkPresetFromStateDevice(device))
		}
	}
	return presets
}

func appendUniqueVirtualPreset(
	presets []recent.VirtualSinkPreset,
	seen map[string]bool,
	vs recent.VirtualSinkPreset,
) []recent.VirtualSinkPreset {
	if vs.SinkName == "" || seen[vs.SinkName] {
		return presets
	}
	seen[vs.SinkName] = true
	return append(presets, vs)
}

func stateDeviceClassifiesVirtual(device virtualstate.Device) bool {
	if device.State.Desired == virtualstate.DesiredAbsent {
		return false
	}
	return device.ModuleType == "" || device.ModuleType == virtualstate.ModuleNullSink
}

func virtualSinkAliasMatches(
	name string,
	isInput bool,
	vs recent.VirtualSinkPreset,
	context managedVirtualAliasContext,
) bool {
	if isInput {
		return captureAliasMatches(name, vs, context)
	}
	return playbackAliasMatches(name, vs, context)
}

func managedVirtualDisplayName(isInput bool, vs recent.VirtualSinkPreset) string {
	if isInput {
		return virtualSinkCaptureName(vs)
	}
	return virtualSinkPlaybackName(vs)
}

func playbackAliases(vs recent.VirtualSinkPreset) []string {
	return uniqueStrings(virtualSinkPlaybackName(vs), vs.SinkName)
}

func playbackAliasMatches(name string, vs recent.VirtualSinkPreset, context managedVirtualAliasContext) bool {
	return stringSetContains(playbackAliases(vs), name) ||
		truncatedPlaybackAliasMatches(name, vs, context)
}

func truncatedPlaybackAliasMatches(
	name string,
	vs recent.VirtualSinkPreset,
	context managedVirtualAliasContext,
) bool {
	if name != genericPlaybackName || virtualSinkPlaybackName(vs) == name {
		return false
	}
	if context.deviceNameCounts[genericPlaybackName] > 1 {
		return false
	}
	return context.moduleBackedSinks[vs.SinkName] || captureAliasInDeviceList(vs, context.deviceNames)
}

func captureAliasMatches(name string, vs recent.VirtualSinkPreset, context managedVirtualAliasContext) bool {
	return stringSetContains(captureAliases(vs), name) ||
		truncatedCaptureAliasMatches(name, vs, context)
}

func truncatedCaptureAliasMatches(
	name string,
	vs recent.VirtualSinkPreset,
	context managedVirtualAliasContext,
) bool {
	if name != genericPlaybackMonitorName {
		return false
	}
	if context.deviceNameCounts[genericPlaybackMonitorName] > 1 {
		return false
	}
	return context.moduleBackedSinks[vs.SinkName] || playbackAliasInDeviceList(vs, context.deviceNames)
}

func disambiguateDuplicateGenericRows(devices []deviceRow, genericName string) {
	if countGenericRows(devices, genericName) < 2 {
		return
	}
	index := 1
	for i := range devices {
		if devices[i].Name == genericName && !devices[i].IsVirtual {
			devices[i].Name = fmt.Sprintf("%s #%d", genericName, index)
			index++
		}
	}
}

func countGenericRows(devices []deviceRow, genericName string) int {
	count := 0
	for _, device := range devices {
		if device.Name == genericName && !device.IsVirtual {
			count++
		}
	}
	return count
}

func playbackAliasInDeviceList(vs recent.VirtualSinkPreset, deviceNames map[string]bool) bool {
	for _, alias := range playbackAliases(vs) {
		if deviceNames[alias] {
			return true
		}
	}
	return false
}

func captureAliasInDeviceList(vs recent.VirtualSinkPreset, deviceNames map[string]bool) bool {
	for _, alias := range captureAliases(vs) {
		if deviceNames[alias] {
			return true
		}
	}
	return false
}

func captureAliases(vs recent.VirtualSinkPreset) []string {
	return uniqueStrings(
		virtualSinkCaptureName(vs), virtualSinkMonitorName(vs),
		"Monitor of "+virtualSinkPlaybackName(vs), "Monitor of "+vs.SinkName,
	)
}

func stringSetContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// handleMultiSelectToggle cycles the selected device through roles:
// none → [C] → [P] → [C+P] → none
// For mix sub-items in the output section, toggles the mix input instead.
func (m SetupModel) handleMultiSelectToggle() (SetupModel, tea.Cmd) {
	// Output section: use expanded row list (includes mix sub-items).
	if m.DeviceSection == SectionOutput {
		rows := m.buildOutputRows()
		if m.deviceCursor < 0 || m.deviceCursor >= len(rows) {
			return m, nil
		}
		row := rows[m.deviceCursor]
		if row.isMixItem {
			m.toggleMixInput(row.parentKey, row.device.selectKey())
			return m, nil
		}
		// Regular output device toggle
		key := row.device.selectKey()
		current := m.multiSelect[key]
		if current.Playback {
			current.Playback = false
		} else {
			current.Playback = true
		}
		if current.Capture || current.Playback {
			m.multiSelect[key] = current
		} else {
			delete(m.multiSelect, key)
		}
		m.cleanupMixInputs()
		return m, nil
	}

	// Input section: standard toggle.
	devices := m.currentSectionDevices()
	if m.deviceCursor < 0 || m.deviceCursor >= len(devices) {
		return m, nil
	}
	key := devices[m.deviceCursor].selectKey()
	current := m.multiSelect[key]

	if current.Capture || current.Playback {
		current.Capture = false
		current.Playback = false
	} else {
		current.Capture = true
	}

	if current.Capture || current.Playback {
		m.multiSelect[key] = current
	} else {
		delete(m.multiSelect, key)
	}
	return m, nil
}

// currentSectionDevices returns the device list for the currently focused section.
func (m SetupModel) currentSectionDevices() []deviceRow {
	if m.DeviceSection == SectionOutput {
		return m.outputDevices
	}
	return m.inputDevices
}

func (m SetupModel) selectedVisibleRows() []deviceRow {
	showInput, showOutput := m.visibleSections()
	rows := make([]deviceRow, 0, len(m.multiSelect))
	if showInput {
		rows = append(rows, m.selectedInputRows()...)
	}
	if showOutput {
		rows = append(rows, m.selectedOutputRows()...)
	}
	return rows
}

func (m SetupModel) selectedInputRows() []deviceRow {
	var rows []deviceRow
	for _, d := range m.inputDevices {
		if roles := m.multiSelect[d.selectKey()]; roles.Capture || roles.Playback {
			rows = append(rows, d)
		}
	}
	return rows
}

func (m SetupModel) selectedOutputRows() []deviceRow {
	var rows []deviceRow
	for _, d := range m.outputDevices {
		if roles := m.multiSelect[d.selectKey()]; roles.Capture || roles.Playback {
			rows = append(rows, d)
		}
	}
	return rows
}

func (m SetupModel) selectedVisibleCounts() (captureCount, playbackCount int) {
	showInput, showOutput := m.visibleSections()
	if showInput {
		captureCount = len(m.selectedInputRows())
	}
	if showOutput {
		playbackCount = len(m.selectedOutputRows())
	}
	return captureCount, playbackCount
}

// currentSectionRowCount returns the number of navigable rows in the current section.
// For output section, this includes mix sub-items under selected virtual outputs.
func (m SetupModel) currentSectionRowCount() int {
	if m.DeviceSection == SectionOutput {
		return len(m.buildOutputRows())
	}
	return len(m.inputDevices)
}

// currentCursorDevice returns a pointer to the deviceRow at the current cursor position,
// or nil if the cursor is out of range or points to a mix sub-item.
func (m *SetupModel) currentCursorDevice() *deviceRow {
	if m.DeviceSection == SectionOutput {
		rows := m.buildOutputRows()
		if m.deviceCursor >= 0 && m.deviceCursor < len(rows) && !rows[m.deviceCursor].isMixItem {
			// Find the actual device in outputDevices slice (mutable)
			key := rows[m.deviceCursor].device.selectKey()
			for i := range m.outputDevices {
				if m.outputDevices[i].selectKey() == key {
					return &m.outputDevices[i]
				}
			}
		}
		return nil
	}
	if m.deviceCursor >= 0 && m.deviceCursor < len(m.inputDevices) {
		return &m.inputDevices[m.deviceCursor]
	}
	return nil
}

// handleVolumeAdjust changes the volume of the currently selected device by ±0.1.
func (m SetupModel) handleVolumeAdjust(decrease bool) (SetupModel, tea.Cmd) {
	dev := m.currentCursorDevice()
	if dev == nil {
		return m, nil
	}
	if _, ok := m.multiSelect[dev.selectKey()]; !ok {
		return m, nil
	}
	step := 0.1
	if decrease {
		step = -0.1
	}
	dev.Volume = math.Round((dev.Volume+step)*10) / 10
	if dev.Volume < 0 {
		dev.Volume = 0
	}
	if dev.Volume > 1.5 {
		dev.Volume = 1.5
	}
	return m, nil
}

// handleAGCToggle toggles AGC on the currently selected device.
func (m SetupModel) handleAGCToggle() (SetupModel, tea.Cmd) {
	dev := m.currentCursorDevice()
	if dev == nil {
		return m, nil
	}
	if _, ok := m.multiSelect[dev.selectKey()]; !ok {
		return m, nil
	}
	dev.AGC = !dev.AGC
	return m, nil
}

// restoreVolumeAGCFromPreset applies Volume and AGC from preset devices
// to the matching deviceRow entries in inputDevices/outputDevices.
func (m *SetupModel) restoreVolumeAGCFromPreset(preset recent.DevicePreset, matched []deviceRow) {
	// Build map from matched device name+isInput → preset device for fast lookup
	presetByKey := make(map[presetDeviceKey]recent.PresetDevice, len(preset.Devices))
	for _, pd := range preset.Devices {
		presetByKey[presetDeviceKey{pd.Name, pd.IsInput}] = pd
	}

	applyToSlice := func(devices []deviceRow) {
		for i := range devices {
			for _, md := range matched {
				if devices[i].selectKey() == md.selectKey() {
					if pd, ok := presetDeviceForMatchedRow(preset, presetByKey, md); ok {
						if pd.Volume > 0 {
							devices[i].Volume = pd.Volume
						}
						devices[i].AGC = pd.AGC
					}
					break
				}
			}
		}
	}
	applyToSlice(m.inputDevices)
	applyToSlice(m.outputDevices)
}

func presetDeviceForMatchedRow(
	preset recent.DevicePreset,
	byKey map[presetDeviceKey]recent.PresetDevice,
	row deviceRow,
) (recent.PresetDevice, bool) {
	// Virtual device runtime IDs and generic names can collide, so prefer
	// persisted virtual sink identity before the legacy name fallback.
	for _, pd := range preset.Devices {
		if presetDeviceMatchesVirtualIdentity(pd, row) {
			return pd, true
		}
	}
	if pd, ok := byKey[presetDeviceKey{row.Name, row.IsInput}]; ok {
		return pd, true
	}
	return recent.PresetDevice{}, false
}

func presetDeviceMatchesVirtualIdentity(pd recent.PresetDevice, row deviceRow) bool {
	if pd.IsInput != row.IsInput || (!pd.Virtual && pd.VirtualSink == nil) {
		return false
	}
	return virtualPresetIdentityMatches(pd, row)
}
