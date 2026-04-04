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

// removePulseAudioSink unloads a PulseAudio module by ID.
func removePulseAudioSink(moduleID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pactl", "unload-module", moduleID).Run()
}

// updateVirtualMicField updates the Virtual mic field label and hint.
func (m *SetupModel) updateVirtualMicField(created bool) {
	for i := range m.Fields {
		if m.Fields[i].Label == "Virtual mic" {
			if created {
				m.Fields[i].ActionLabel = "EchoWarp ✓"
				m.Fields[i].Hint = ""
			} else {
				m.Fields[i].ActionLabel = "Create ▸"
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
