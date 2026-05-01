package views

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(m.virtualMicManageable, echowarpSinkName)
	m.overlay = SetupOverlayVirtualDevice
	return m, nil
}

// createPulseAudioSink creates a PulseAudio null-sink with the given name.
// Returns the module ID (for later removal) or an error.
func createPulseAudioSink(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "pactl", "load-module", "module-null-sink",
		fmt.Sprintf("sink_name=%s", name),
		fmt.Sprintf("sink_properties=device.description=%s", name),
	).Output()
	if err != nil {
		return "", fmt.Errorf("pactl failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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
	monitorName := "Monitor of " + sinkName
	for _, d := range m.inputDevices {
		if d.Name == monitorName && d.IsVirtual {
			m.setSelectedRole(d, true)
			return true
		}
	}
	return false
}

func (m *SetupModel) selectVirtualOutputSink(sinkName string) bool {
	for _, d := range m.outputDevices {
		if d.Name == sinkName && d.IsVirtual {
			m.setSelectedRole(d, false)
			return true
		}
	}
	return false
}

func (m *SetupModel) rememberPendingVirtualSinkSelection(sinkName string) {
	if sinkName == "" {
		return
	}
	if m.pendingVirtualSinkSelection == nil {
		m.pendingVirtualSinkSelection = make(map[string]bool)
	}
	m.pendingVirtualSinkSelection[sinkName] = true
}

func (m *SetupModel) selectPendingVirtualSink(sinkName string) {
	if !m.pendingVirtualSinkSelection[sinkName] {
		return
	}
	if m.autoSelectVirtualDevice(sinkName) {
		delete(m.pendingVirtualSinkSelection, sinkName)
	}
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
	moduleID, found, err := findPulseAudioModuleFn(echowarpSinkName)
	if err == nil && found {
		m.markVirtualSinkFound(moduleID, *m.defaultVirtualSinkPreset())
		return
	}
	if err != nil && m.hasVirtualMicModuleState() {
		m.virtualMicManageable = true
		m.updateVirtualMicField(true)
		return
	}
	if m.hasExactEchoWarpOutput() && m.hasVirtualMicModuleState() {
		m.virtualMicManageable = true
		m.updateVirtualMicField(true)
		return
	}
	m.clearVirtualMicManageState()
	m.updateVirtualMicField(false)
}

func (m SetupModel) hasVirtualMicModuleState() bool {
	return m.virtualMicManagedModule != "" || m.virtualMicModule != ""
}

func (m SetupModel) hasExactEchoWarpOutput() bool {
	for _, d := range m.outputDevices {
		if d.Name == echowarpSinkName {
			return true
		}
	}
	return false
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
	moduleID, err := m.resolveVirtualMicModuleForRemoval()
	if err != nil {
		return err
	}
	if err := removePulseAudioSinkFn(moduleID); err != nil {
		return err
	}
	m.clearVirtualMicManageState()
	m.clearVirtualSinkLifecycleState()
	return nil
}

func (m SetupModel) resolveVirtualMicModuleForRemoval() (string, error) {
	if m.virtualMicManagedModule != "" {
		return m.virtualMicManagedModule, nil
	}
	if m.virtualMicModule != "" {
		return m.virtualMicModule, nil
	}
	moduleID, found, err := findPulseAudioModuleFn(echowarpSinkName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("virtual audio device %q was not found", echowarpSinkName)
	}
	return moduleID, nil
}
