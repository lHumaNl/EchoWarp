package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// DeviceRoleSet tracks assigned roles for a device in multi-select mode.
type DeviceRoleSet struct {
	Capture  bool
	Playback bool
}

// deviceRow holds display info for a device in the sectioned device list.
type deviceRow struct {
	Name       string
	ID         uint32
	IsInput    bool
	IsVirtual  bool
	IsLoopback bool
	Channels   uint32
	SampleRate uint32
	BitDepth   uint32
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

// displayNameFromKey extracts the human-readable device name from a selectKey.
func displayNameFromKey(key string) string {
	// Key format: "I:123:Device Name" or "O:456:Device Name"
	parts := strings.SplitN(key, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return key
}

// IsVirtualDevice checks if a device name matches known virtual audio devices.
func IsVirtualDevice(name string) bool {
	lower := strings.ToLower(name)
	virtuals := []string{"blackhole", "vb-audio", "cable", "virtual", "loopback", "soundflower", "existential"}
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
		row := deviceRow{
			Name:       name,
			ID:         id,
			IsInput:    isInput,
			IsVirtual:  IsVirtualDevice(name),
			IsLoopback: IsLoopbackDevice(name),
			Channels:   ch,
			SampleRate: sr,
			BitDepth:   bd,
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

// currentSectionRowCount returns the number of navigable rows in the current section.
// For output section, this includes mix sub-items under selected virtual outputs.
func (m SetupModel) currentSectionRowCount() int {
	if m.DeviceSection == SectionOutput {
		return len(m.buildOutputRows())
	}
	return len(m.inputDevices)
}
