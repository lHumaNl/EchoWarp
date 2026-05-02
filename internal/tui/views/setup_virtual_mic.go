package views

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
)

const echowarpSinkName = "EchoWarp"
const echowarpMonitorName = "Monitor of " + echowarpSinkName

var (
	isLinuxRuntime         = runtime.GOOS == "linux"
	createPulseAudioSinkFn = createPulseAudioSink
	removePulseAudioSinkFn = RemovePulseAudioSink
	findPulseAudioModuleFn = FindPulseAudioSinkModule
)

type pulseAudioModule struct {
	ID        string
	Type      string
	Arguments string
}

// openVirtualMicOverlay opens the virtual mic overlay (Linux: create/remove, other: no-op).
func (m SetupModel) openVirtualMicOverlay() (SetupModel, tea.Cmd) {
	if !isLinuxRuntime {
		return m, nil
	}
	m.syncVirtualMicState()
	sinkName := m.defaultVirtualOverlaySinkName()
	devices := m.virtualOverlayDevices()
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(len(devices) > 0, sinkName)
	m.virtualDeviceOverlay.CaptureName = m.virtualOverlayCaptureName(sinkName)
	m.virtualDeviceOverlay.SetDevices(devices)
	m.overlay = SetupOverlayVirtualDevice
	return m, nil
}

func (m SetupModel) virtualOverlayDevices() []VirtualOverlayDevice {
	devices := make([]VirtualOverlayDevice, 0, len(m.trackedVirtualSinks))
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		tracked := m.trackedVirtualSinks[sinkName]
		if tracked.isLiveModuleBacked() {
			devices = append(devices, m.virtualOverlayDevice(tracked.Preset))
		}
	}
	return devices
}

func (m SetupModel) virtualOverlayDevice(vs recent.VirtualSinkPreset) VirtualOverlayDevice {
	owner, removable := m.virtualOverlayDeviceOwnership(vs.SinkName)
	return VirtualOverlayDevice{
		SinkName: vs.SinkName, BaseName: vs.BaseName,
		PlaybackName: virtualSinkPlaybackName(vs), CaptureName: virtualSinkCaptureName(vs),
		Owner: owner, Removable: removable,
	}
}

func (m SetupModel) virtualOverlayDeviceOwnership(sinkName string) (string, bool) {
	device, ok, err := virtualstate.LoadDevice(sinkName)
	if err != nil || !ok {
		return "", true
	}
	return device.Ownership.CreatedBy, !virtualstate.IsOtherRoleOwner(device, m.virtualStateRole())
}

func (m SetupModel) virtualOverlayCaptureName(sinkName string) string {
	if tracked, ok := m.trackedVirtualSinks[sinkName]; ok {
		return virtualSinkCaptureName(tracked.Preset)
	}
	return ""
}

func (m SetupModel) defaultVirtualOverlaySinkName() string {
	if m.virtualMicManageable {
		if sinkName, _, ok := m.firstTrackedManageableVirtualSink(); ok {
			return sinkName
		}
	}
	return echowarpSinkName
}

func (m SetupModel) virtualDeviceOverlayPreset() recent.VirtualSinkPreset {
	if m.virtualDeviceOverlay == nil {
		return *m.defaultVirtualSinkPreset()
	}
	if !m.virtualDeviceOverlay.IsCreateMode() {
		if tracked, ok := m.trackedVirtualSinks[m.virtualDeviceOverlay.SinkName]; ok {
			return tracked.Preset
		}
		return *m.defaultVirtualSinkPreset()
	}
	return m.virtualSinkPresetForBaseName(m.virtualDeviceOverlay.BaseName())
}

// createPulseAudioSink creates a PulseAudio null-sink with the given preset.
// Returns the module ID (for later removal) or an error.
func createPulseAudioSink(vs recent.VirtualSinkPreset) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pactl", pulseAudioLoadModuleArgs(vs)...).Output()
	if err != nil {
		return "", fmt.Errorf("pactl failed: %w", err)
	}
	moduleID := strings.TrimSpace(string(out))
	_ = updatePulseAudioMonitorDescription(vs)
	return moduleID, nil
}

func updatePulseAudioMonitorDescription(vs recent.VirtualSinkPreset) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pactl", pulseAudioUpdateMonitorArgs(vs)...).Run()
}

func pulseAudioLoadModuleArgs(vs recent.VirtualSinkPreset) []string {
	return []string{
		"load-module",
		"module-null-sink",
		fmt.Sprintf("sink_name=%s", vs.SinkName),
		fmt.Sprintf("sink_properties=device.description=%s", pulseAudioQuotedValue(virtualSinkPlaybackName(vs))),
	}
}

func pulseAudioUpdateMonitorArgs(vs recent.VirtualSinkPreset) []string {
	return []string{
		"update-source-proplist",
		virtualSinkMonitorName(vs),
		fmt.Sprintf("device.description=%s", pulseAudioQuotedValue(virtualSinkCaptureName(vs))),
	}
}

func pulseAudioQuotedValue(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

// RemovePulseAudioSink unloads a PulseAudio module by ID.
func RemovePulseAudioSink(moduleID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pactl", "unload-module", moduleID).Run()
}

// FindPulseAudioSinkModule returns the module ID for an exact null-sink name.
func FindPulseAudioSinkModule(sinkName string) (string, bool, error) {
	modules, err := listPulseAudioModules()
	if err != nil {
		return "", false, err
	}
	for _, module := range modules {
		if module.Type == "module-null-sink" && moduleHasSinkName(module.Arguments, sinkName) {
			return module.ID, true, nil
		}
	}
	return "", false, nil
}

func listPulseAudioModules() ([]pulseAudioModule, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pactl", "list", "short", "modules").Output()
	if err != nil {
		return nil, fmt.Errorf("pactl list modules failed: %w", err)
	}
	return parsePulseAudioModules(string(out)), nil
}

func parsePulseAudioModules(output string) []pulseAudioModule {
	var modules []pulseAudioModule
	for _, line := range strings.Split(output, "\n") {
		if module, ok := parsePulseAudioModuleLine(line); ok {
			modules = append(modules, module)
		}
	}
	return modules
}

func parsePulseAudioModuleLine(line string) (pulseAudioModule, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), "\t", 4)
	if len(parts) < 3 {
		parts = strings.Fields(line)
	}
	if len(parts) < 3 {
		return pulseAudioModule{}, false
	}
	return pulseAudioModule{ID: parts[0], Type: parts[1], Arguments: parts[2]}, true
}

func moduleHasSinkName(arguments, sinkName string) bool {
	for _, arg := range strings.Fields(arguments) {
		value, ok := strings.CutPrefix(arg, "sink_name=")
		if ok && strings.Trim(value, "\"'") == sinkName {
			return true
		}
	}
	return false
}

// updateVirtualMicField updates the Virtual mic field label and hint.
func (m *SetupModel) updateVirtualMicField(created bool) {
	for i := range m.Fields {
		if m.Fields[i].Key == "virtual_mic" {
			if created {
				m.Fields[i].ActionLabel = "EchoWarp ✓"
				m.Fields[i].Hint = ""
			} else {
				m.Fields[i].ActionLabel = "Create Virtual Audio Device ▸"
				m.Fields[i].Hint = "(PulseAudio)"
			}
			break
		}
	}
}

// autoSelectVirtualDevice finds the newly created virtual device and selects it.
func (m *SetupModel) autoSelectVirtualDevice(name string) bool {
	if m.multiSelect == nil {
		m.multiSelect = make(map[string]DeviceRoleSet)
	}
	showInput, showOutput := m.visibleSections()
	selected := false
	if showInput {
		selected = m.selectVirtualInputMonitor(name) || selected
	}
	if showOutput {
		selected = m.selectVirtualOutputSink(name) || selected
	}
	return selected
}

func (m *SetupModel) selectVirtualInputMonitor(sinkName string) bool {
	vs := m.virtualSinkPresetBySinkName(sinkName)
	for _, d := range m.inputDevices {
		if d.IsVirtual && stringSetContains(captureAliases(vs), d.Name) {
			m.setSelectedRole(d, true)
			return true
		}
	}
	return false
}

func (m *SetupModel) selectVirtualOutputSink(sinkName string) bool {
	vs := m.virtualSinkPresetBySinkName(sinkName)
	for _, d := range m.outputDevices {
		if d.IsVirtual && stringSetContains(playbackAliases(vs), d.Name) {
			m.setSelectedRole(d, false)
			return true
		}
	}
	return false
}

func (m *SetupModel) virtualSinkPresetBySinkName(sinkName string) recent.VirtualSinkPreset {
	if tracked, ok := m.trackedVirtualSinks[sinkName]; ok {
		return tracked.Preset
	}
	for _, vs := range m.managedVirtualSinkPresets() {
		if vs.SinkName == sinkName {
			return vs
		}
	}
	return recent.VirtualSinkPreset{SinkName: sinkName}
}

func (m *SetupModel) setSelectedRole(d deviceRow, capture bool) {
	key := d.selectKey()
	role := m.multiSelect[key]
	if capture {
		role.Capture = true
	} else {
		role.Playback = true
	}
	m.multiSelect[key] = role
}

func (m *SetupModel) syncVirtualMicState() {
	if !isLinuxRuntime {
		return
	}
	tracked := m.refreshVirtualMicStateFromTrackedSinks()
	found, err := m.discoverManageableVirtualSink()
	if found {
		return
	}
	if tracked && (err != nil || m.refreshVirtualMicStateFromTrackedSinks()) {
		return
	}
	if err != nil && m.hasVirtualMicModuleState() {
		m.virtualMicManageable = true
		m.updateVirtualMicField(true)
		return
	}
	if m.hasKnownVirtualOutput() && m.hasVirtualMicModuleState() {
		m.virtualMicManageable = true
		m.updateVirtualMicField(true)
		return
	}
	m.clearVirtualMicManageState()
	m.updateVirtualMicField(false)
}

func (m *SetupModel) discoverManageableVirtualSink() (bool, error) {
	foundAny := false
	for _, vs := range m.virtualSinkDiscoveryPresets() {
		if m.hasModuleBackedTrackedVirtualSink(vs.SinkName) {
			foundAny = true
			continue
		}
		moduleID, found, err := findPulseAudioModuleFn(vs.SinkName)
		if err != nil {
			return foundAny, err
		}
		if !found {
			continue
		}
		if err := m.trackDiscoveredVirtualSink(moduleID, vs); err != nil {
			m.clearVirtualMicManageState()
			return false, nil
		}
		foundAny = true
	}
	return foundAny, nil
}

func (m SetupModel) hasModuleBackedTrackedVirtualSink(sinkName string) bool {
	tracked, ok := m.trackedVirtualSinks[sinkName]
	return ok && tracked.isLiveModuleBacked()
}

func (tracked trackedVirtualSink) isLiveModuleBacked() bool {
	return tracked.Manageable && tracked.ModuleID != "" && tracked.LiveConfirmed
}

func (m *SetupModel) trackDiscoveredVirtualSink(moduleID string, vs recent.VirtualSinkPreset) error {
	m.markVirtualSinkFound(moduleID, vs)
	return m.persistFoundVirtualSink(moduleID, vs, virtualSinkEnsureOptions{})
}

func (m SetupModel) hasVirtualMicModuleState() bool {
	return m.virtualMicManagedModule != "" || m.virtualMicModule != ""
}

func (m SetupModel) hasKnownVirtualOutput() bool {
	knownNames := m.virtualSinkOutputNames()
	for _, d := range m.outputDevices {
		if knownNames[d.Name] {
			return true
		}
	}
	return false
}

func (m SetupModel) virtualSinkOutputNames() map[string]bool {
	names := make(map[string]bool)
	for _, vs := range m.virtualSinkDiscoveryPresets() {
		for _, name := range uniqueStrings(virtualSinkPlaybackName(vs), vs.SinkName) {
			names[name] = true
		}
	}
	return names
}

func (m SetupModel) virtualSinkDiscoveryPresets() []recent.VirtualSinkPreset {
	seen := make(map[string]bool)
	presets := make([]recent.VirtualSinkPreset, 0, len(m.trackedVirtualSinks)+1)
	addPreset := func(vs recent.VirtualSinkPreset) {
		if vs.SinkName == "" || seen[vs.SinkName] {
			return
		}
		seen[vs.SinkName] = true
		presets = append(presets, vs)
	}
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		addPreset(m.trackedVirtualSinks[sinkName].Preset)
	}
	for _, device := range m.virtualStateDiscoveryDevices() {
		addPreset(virtualSinkPresetFromStateDevice(device))
	}
	addPreset(*m.defaultVirtualSinkPreset())
	return presets
}

func (m SetupModel) virtualStateDiscoveryDevices() []virtualstate.Device {
	state, err := virtualstate.Load()
	if err != nil {
		return nil
	}
	devices := make([]virtualstate.Device, 0, len(state.Devices))
	for _, device := range state.Devices {
		if isDiscoverableStateDevice(device) {
			devices = append(devices, device)
		}
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].SinkName < devices[j].SinkName })
	return devices
}
func isDiscoverableStateDevice(device virtualstate.Device) bool {
	return (device.ModuleType == "" || device.ModuleType == virtualstate.ModuleNullSink) &&
		device.State.Desired != virtualstate.DesiredAbsent
}

func (m SetupModel) sortedTrackedVirtualSinkNames() []string {
	names := make([]string, 0, len(m.trackedVirtualSinks))
	for sinkName := range m.trackedVirtualSinks {
		names = append(names, sinkName)
	}
	sort.Strings(names)
	return names
}

func (m SetupModel) firstTrackedManageableVirtualSink() (string, trackedVirtualSink, bool) {
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		tracked := m.trackedVirtualSinks[sinkName]
		if tracked.isLiveModuleBacked() {
			return sinkName, tracked, true
		}
	}
	return "", trackedVirtualSink{}, false
}

func (m *SetupModel) refreshVirtualMicStateFromTrackedSinks() bool {
	_, tracked, ok := m.firstTrackedManageableVirtualSink()
	if !ok {
		return false
	}
	m.refreshSessionVirtualMicModule()
	m.virtualMicManageable = true
	m.virtualMicManagedModule = tracked.ModuleID
	m.virtualSinkOnStop = tracked.Preset.OnStop
	m.virtualSinkOnStart = tracked.Preset.OnStart
	m.virtualSinkLifecycleConfigured = true
	m.updateVirtualMicField(true)
	return true
}

func (m *SetupModel) refreshSessionVirtualMicModule() {
	m.virtualMicCreated = false
	m.virtualMicModule = ""
	for _, sinkName := range m.sortedTrackedVirtualSinkNames() {
		tracked := m.trackedVirtualSinks[sinkName]
		if tracked.CreatedThisSession && tracked.ModuleID != "" {
			m.virtualMicCreated = true
			m.virtualMicModule = tracked.ModuleID
			return
		}
	}
}

func (m *SetupModel) clearVirtualMicManageState() {
	m.virtualMicCreated = false
	m.virtualMicModule = ""
	m.virtualMicManageable = false
	m.virtualMicManagedModule = ""
}

func (m *SetupModel) clearVirtualSinkLifecycleState() {
	m.virtualSinkOnStop = ""
	m.virtualSinkOnStart = ""
	m.virtualSinkLifecycleConfigured = false
}

func (m *SetupModel) removeManagedVirtualMic() error {
	return m.removeManagedVirtualSink(echowarpSinkName)
}

func (m *SetupModel) removeManagedVirtualSink(sinkName string) error {
	if err := m.ensureVirtualSinkRemovalAllowed(sinkName); err != nil {
		return err
	}
	moduleID, err := m.resolveVirtualSinkModuleForRemoval(sinkName)
	if err != nil {
		return err
	}
	if err := removePulseAudioSinkFn(moduleID); err != nil {
		return err
	}
	if err := virtualstate.DeleteDevice(sinkName); err != nil {
		return fmt.Errorf("persist virtual audio device removal: %w", err)
	}
	delete(m.trackedVirtualSinks, sinkName)
	if !m.refreshVirtualMicStateFromTrackedSinks() {
		m.clearVirtualMicManageState()
		m.clearVirtualSinkLifecycleState()
	}
	return nil
}

func (m SetupModel) ensureVirtualSinkRemovalAllowed(sinkName string) error {
	device, ok, err := virtualstate.LoadDevice(sinkName)
	if err != nil {
		return fmt.Errorf("load virtual audio device state: %w", err)
	}
	if ok && virtualstate.IsOtherRoleOwner(device, m.virtualStateRole()) {
		return otherRoleVirtualMicRemoveError(sinkName, device.Ownership.CreatedBy)
	}
	return nil
}

func otherRoleVirtualMicRemoveError(sinkName, owner string) error {
	return virtualSinkOwnershipError{sinkName: sinkName, owner: owner, reason: "is owned by"}
}

func isVirtualSinkOwnershipError(err error) bool {
	var ownershipErr virtualSinkOwnershipError
	return errors.As(err, &ownershipErr)
}

type virtualSinkOwnershipError struct {
	sinkName string
	owner    string
	reason   string
}

func (e virtualSinkOwnershipError) Error() string {
	return fmt.Sprintf("%s virtual audio device %s %s", e.sinkName, e.reason, e.owner)
}

func (m *SetupModel) persistVirtualSinkPresent(moduleID string, vs recent.VirtualSinkPreset) error {
	policy := virtualstate.DevicePolicy{OnStop: vs.OnStop, OnStart: vs.OnStart}
	metadata := virtualstate.DeviceMetadata{
		ID: vs.ID, BaseName: vs.BaseName, PlaybackName: vs.PlaybackName,
		CaptureName: vs.CaptureName, SessionID: m.ensureVirtualSessionID(),
	}
	return virtualstate.UpsertPresentWithMetadata(
		vs.SinkName, virtualSinkMonitorName(vs), moduleID, m.virtualStateRole(), policy, metadata,
	)
}

func (m *SetupModel) persistVirtualSinkPolicy() error {
	if m.virtualMicManagedModule == "" && m.virtualMicModule == "" {
		return nil
	}
	vs := m.pendingLifecycleSink
	if vs.SinkName == "" {
		vs = *m.defaultVirtualSinkPreset()
	}
	return m.persistVirtualSinkPresent(m.virtualSinkCleanupModuleID(), vs)
}

func (m *SetupModel) applyPendingVirtualSinkLifecycle() {
	if m.pendingLifecycleSink.SinkName == "" {
		return
	}
	m.pendingLifecycleSink.OnStop = m.virtualSinkOnStop
	m.pendingLifecycleSink.OnStart = m.virtualSinkOnStart
	tracked := m.trackedVirtualSinks[m.pendingLifecycleSink.SinkName]
	tracked.Preset = m.pendingLifecycleSink
	m.trackedVirtualSinks[m.pendingLifecycleSink.SinkName] = tracked
}

func (m SetupModel) virtualSinkCreatedFlashMessage() string {
	vs := m.pendingLifecycleSink
	if vs.SinkName == "" {
		vs = *m.defaultVirtualSinkPreset()
	}
	return "✓ Virtual audio device created — " + virtualSinkPlaybackName(vs)
}

func (m SetupModel) resolveVirtualSinkModuleForRemoval(sinkName string) (string, error) {
	moduleID, found, err := findPulseAudioModuleFn(sinkName)
	if err != nil {
		return "", err
	}
	if found {
		return moduleID, nil
	}
	if tracked, ok := m.trackedVirtualSinks[sinkName]; ok && tracked.ModuleID != "" {
		return "", fmt.Errorf("virtual audio device %q was not found for module %s", sinkName, tracked.ModuleID)
	}
	if m.virtualMicManagedModule != "" || m.virtualMicModule != "" {
		moduleID = firstNonEmpty(m.virtualMicManagedModule, m.virtualMicModule)
		return "", fmt.Errorf("virtual audio device %q was not found for module %s", sinkName, moduleID)
	}
	if !found {
		return "", fmt.Errorf("virtual audio device %q was not found", sinkName)
	}
	return "", fmt.Errorf("virtual audio device %q was not found", sinkName)
}

func (m *SetupModel) ensureVirtualSessionID() string {
	if m.virtualSessionID == "" {
		m.virtualSessionID = virtualstate.NewSessionID()
	}
	return m.virtualSessionID
}
