package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

// loopbackTestDeviceItem implements all interfaces used by rebuildDeviceGroups.
type loopbackTestDeviceItem struct {
	name       string
	id         uint32
	isInput    bool
	channels   uint32
	sampleRate uint32
}

func (m loopbackTestDeviceItem) FilterValue() string      { return m.name }
func (m loopbackTestDeviceItem) Title() string            { return m.name }
func (m loopbackTestDeviceItem) Description() string      { return "" }
func (m loopbackTestDeviceItem) DeviceID() uint32         { return m.id }
func (m loopbackTestDeviceItem) IsInputDevice() bool      { return m.isInput }
func (m loopbackTestDeviceItem) DeviceChannels() uint32   { return m.channels }
func (m loopbackTestDeviceItem) DeviceSampleRate() uint32 { return m.sampleRate }

func TestLoopbackDevice_IsLoopback_Flag(t *testing.T) {
	items := []list.Item{
		loopbackTestDeviceItem{name: "Built-in Mic", id: 1, isInput: true, channels: 1, sampleRate: 44100},
		loopbackTestDeviceItem{name: "Speakers [loopback]", id: 2, isInput: true, channels: 2, sampleRate: 44100},
	}
	deviceList := list.New(items, list.NewDefaultDelegate(), 80, 20)
	cfg := newTestConfig(config.ModeServer)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)
	m = m.WithUnifiedDeviceList(false)
	m.rebuildDeviceGroups()

	require.Len(t, m.inputDevices, 2)
	assert.False(t, m.inputDevices[0].IsLoopback, "regular mic should not be loopback")
	assert.True(t, m.inputDevices[1].IsLoopback, "device with [loopback] should be marked loopback")
	assert.True(t, m.inputDevices[1].IsVirtual, "loopback devices are also virtual")
}

func TestLoopbackDevice_RenderWithIcon(t *testing.T) {
	cfg := newTestConfig(config.ModeServer)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)
	m = m.WithUnifiedDeviceList(true)
	m.inputDevices = []deviceRow{
		{Name: "Built-in Mic", ID: 1, IsInput: true},
		{Name: "Speakers [loopback]", ID: 2, IsInput: true, IsLoopback: true, IsVirtual: true, Channels: 2, SampleRate: 44100},
	}
	m.multiSelect = make(map[string]DeviceRoleSet)
	m.activeColumn = ColumnDevices
	m.DeviceSection = SectionInput

	view := m.renderDeviceSection(SectionInput, m.inputDevices, 100, 20)
	assert.Contains(t, view, "🔄", "loopback device should render with 🔄 prefix")
	// Regular device should NOT have loopback icon
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Built-in Mic") {
			assert.NotContains(t, line, "🔄", "regular device should not have loopback icon")
		}
	}
}

func TestLoopbackDevice_AppearsInInputSection(t *testing.T) {
	items := []list.Item{
		loopbackTestDeviceItem{name: "Built-in Mic", id: 1, isInput: true, channels: 1, sampleRate: 48000},
		loopbackTestDeviceItem{name: "Headphones", id: 3, isInput: false, channels: 2, sampleRate: 48000},
		loopbackTestDeviceItem{name: "Speakers [loopback]", id: 2, isInput: true, channels: 2, sampleRate: 44100},
	}
	deviceList := list.New(items, list.NewDefaultDelegate(), 80, 20)
	cfg := newTestConfig(config.ModeServer)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)
	m = m.WithUnifiedDeviceList(true)
	m.rebuildDeviceGroups()

	// Loopback device must be in input section
	foundInInput := false
	for _, d := range m.inputDevices {
		if strings.Contains(d.Name, "[loopback]") {
			foundInInput = true
			assert.True(t, d.IsInput, "loopback device should be input")
			assert.True(t, d.IsLoopback, "loopback device should have IsLoopback=true")
		}
	}
	assert.True(t, foundInInput, "loopback device should appear in input section")

	// Should NOT be in output section
	for _, d := range m.outputDevices {
		assert.False(t, strings.Contains(d.Name, "[loopback]"), "loopback device should not be in output section")
	}
}

func TestBuildConfig_LoopbackDevice_SetsConfigFlags(t *testing.T) {
	// The loopback config flags (Loopback, LoopbackOutputDevice, LoopbackBlackHole)
	// are set in the CLI startFunc closure, not in BuildConfig, because the TUI
	// doesn't have access to the loopbackMap. This test verifies that loopback
	// devices are included in the Devices list built by tryStart/BuildConfig,
	// so the CLI layer can detect them by name.
	cfg := newTestConfig(config.ModeServer)
	deviceList := list.New(nil, list.NewDefaultDelegate(), 80, 20)
	m := NewSetupModel(cfg, deviceList, true, 120, 40)
	m = m.WithUnifiedDeviceList(true)
	m.isDuplexMode = false
	m.inputDevices = []deviceRow{
		{Name: "Built-in Mic", ID: 1, IsInput: true},
		{Name: "Speakers [loopback]", ID: 2, IsInput: true, IsLoopback: true, IsVirtual: true, Channels: 2, SampleRate: 44100},
	}
	m.outputDevices = nil
	m.multiSelect = map[string]DeviceRoleSet{
		m.inputDevices[1].selectKey(): {Capture: true},
	}

	builtCfg := m.BuildConfig()
	require.Len(t, builtCfg.Devices, 1, "should have one device entry")
	assert.Contains(t, builtCfg.Devices[0].Name, "[loopback]", "device name should contain [loopback] for CLI layer detection")
}

func TestIsLoopbackDevice(t *testing.T) {
	assert.True(t, IsLoopbackDevice("Speakers [loopback]"))
	assert.True(t, IsLoopbackDevice("Headphones [loopback]"))
	assert.False(t, IsLoopbackDevice("Built-in Mic"))
	assert.False(t, IsLoopbackDevice("BlackHole 2ch"))
}
