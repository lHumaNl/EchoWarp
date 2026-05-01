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

type pulseAudioModule struct {
	ID        string
	Type      string
	Arguments string
}

// openVirtualMicOverlay opens the virtual mic overlay (Linux: create/remove, other: no-op).
func (m SetupModel) openVirtualMicOverlay() (SetupModel, tea.Cmd) {
	if runtime.GOOS != "linux" {
		return m, nil
	}
	m.virtualDeviceOverlay = NewVirtualDeviceOverlay(m.virtualMicCreated, echowarpSinkName)
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
func (m *SetupModel) autoSelectVirtualDevice(name string) {
	if m.multiSelect == nil {
		m.multiSelect = make(map[string]DeviceRoleSet)
	}
	for _, d := range m.outputDevices {
		if strings.Contains(d.Name, name) && d.IsVirtual {
			key := d.selectKey()
			role := m.multiSelect[key]
			role.Playback = true
			m.multiSelect[key] = role
		}
	}
}
