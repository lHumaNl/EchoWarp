package views

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/i18n"
	"github.com/lHumaNl/echowarp/internal/recent"
)

func TestVirtualMicCreateLabelAndFallback(t *testing.T) {
	assert.Equal(t, "Create Virtual Audio Device ▸", i18n.T("action_create_virtual"))

	m := SetupModel{Fields: []SetupField{NewActionField("virtual_mic", "", "old")}}
	m.updateVirtualMicField(false)

	assert.Equal(t, "Create Virtual Audio Device ▸", m.Fields[0].ActionLabel)
}

func TestVirtualDeviceOverlayCopyUsesUserFriendlyTerms(t *testing.T) {
	view := NewVirtualDeviceOverlay(false, echowarpSinkName).View(80)

	assert.Contains(t, view, "Create Virtual Audio Device")
	assert.Contains(t, view, "audio output named")
	assert.Contains(t, view, "Monitor of EchoWarp")
	assert.Contains(t, view, "[Create]")
	assert.NotContains(t, view, "Virtual Microphone")
	assert.NotContains(t, view, "PulseAudio virtual sink")
}

func TestVirtualSinkLifecycleNavigation(t *testing.T) {
	overlay := NewVirtualSinkLifecycleOverlay()

	assert.Equal(t, 0, overlay.focusGroup)
	assert.Equal(t, recent.SinkRecreate, overlay.OnStart())

	action := overlay.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, VSLifecycleNone, action)
	assert.Equal(t, 1, overlay.focusGroup)

	action = overlay.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, VSLifecycleNone, action)
	assert.Equal(t, 1, overlay.focusGroup)
	assert.Equal(t, recent.SinkKeep, overlay.OnStart())

	action = overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, VSLifecycleSave, action)
}

func TestRestoredFlashLineTruncatesToWidth(t *testing.T) {
	m := SetupModel{
		flashMsg:   "Restored: " + strings.Repeat("Very Long Device Name ", 4),
		flashTimer: time.Now().Add(time.Second),
	}

	line := m.renderFlashLine(32)

	assert.LessOrEqual(t, lipgloss.Width(line), 32)
	assert.Contains(t, line, "…")
}

func TestVirtualSinkCleanupPlanRespectsKeepLifecycle(t *testing.T) {
	m := SetupModel{
		virtualMicCreated: true,
		virtualMicModule:  "42",
		virtualSinkOnStop: recent.SinkKeep,
	}

	plan := m.VirtualSinkCleanupPlan()

	assert.False(t, plan.Delete)
	assert.Equal(t, "42", plan.ModuleID)
	assert.True(t, plan.AllowNameFallback)
}

func TestVirtualSinkCleanupPlanDisallowsNameFallbackForPreExistingSink(t *testing.T) {
	row := deviceRow{ID: 99, Name: echowarpSinkName, IsVirtual: true}
	m := SetupModel{
		outputDevices:      []deviceRow{row},
		multiSelect:        map[string]DeviceRoleSet{row.selectKey(): {Playback: true}},
		virtualSinkOnStop:  recent.SinkDelete,
		virtualSinkOnStart: recent.SinkRecreate,
	}

	plan := m.VirtualSinkCleanupPlan()

	assert.True(t, plan.Delete)
	assert.False(t, plan.AllowNameFallback)
}

func TestFindPulseAudioSinkModuleMatchesOnlyExactEchoWarpSink(t *testing.T) {
	modules := parsePulseAudioModules(strings.Join([]string{
		"11\tmodule-null-sink\tsink_name=EchoWarpExtra sink_properties=device.description=EchoWarp",
		"12\tmodule-null-sink\tsink_name=EchoWarp sink_properties=device.description=EchoWarp",
		"13\tmodule-loopback\tsink_name=EchoWarp",
	}, "\n"))

	var moduleID string
	for _, module := range modules {
		if module.Type == "module-null-sink" && moduleHasSinkName(module.Arguments, echowarpSinkName) {
			moduleID = module.ID
		}
	}

	assert.Equal(t, "12", moduleID)
}
