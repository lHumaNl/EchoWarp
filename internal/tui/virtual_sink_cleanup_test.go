package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestCleanupResolvesCreatedVirtualSinkByName(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("")
	stub := stubVirtualSinkCleanup(t, "42")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)
	m.cleanupVirtualSinks()

	require.NotNil(t, cmd)
	assert.True(t, m.quitting)
	assert.Equal(t, []string{"EchoWarp"}, stub.findSinkNames)
	assert.Equal(t, []string{"42"}, stub.removedIDs)
}

func TestCleanupSkipsPreExistingVirtualSinkWithoutModuleID(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	stub := stubVirtualSinkCleanup(t, "42")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)

	assert.True(t, m.quitting)
	assert.Empty(t, stub.findSinkNames)
	assert.Empty(t, stub.removedIDs)
}

func TestCleanupUnloadsKnownVirtualSinkModuleID(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("7")
	stub := stubVirtualSinkCleanup(t, "42")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)

	assert.True(t, m.quitting)
	assert.Empty(t, stub.findSinkNames)
	assert.Equal(t, []string{"7"}, stub.removedIDs)
}

func TestCleanupRunsAgainAfterNewSessionVirtualSinkRecorded(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("7")
	stub := stubVirtualSinkCleanup(t, "42")

	m.cleanupVirtualSinks()
	beforePlan := m.setupModel.VirtualSinkCleanupPlan()
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("8")
	m.resetVirtualSinkCleanupLatchIfSessionSinkRecorded(beforePlan)
	m.cleanupVirtualSinks()

	assert.Equal(t, []string{"7", "8"}, stub.removedIDs)
	assert.Empty(t, stub.findSinkNames)
}

func TestDirectQuitKeepsVirtualSinkWhenConfigured(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkKeep)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("7")
	stub := stubVirtualSinkCleanup(t, "42")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)

	assert.True(t, m.quitting)
	assert.Empty(t, stub.findSinkNames)
	assert.Empty(t, stub.removedIDs)
}

func newVirtualSinkCleanupModel(t *testing.T, onStop recent.SinkLifecycle) Model {
	t.Helper()
	devices := []audio.AudioDevice{{ID: 99, Name: "EchoWarp", IsInput: false, Channels: 2, SampleRate: 48000}}
	m := NewModel(config.Config{Mode: config.ModeServer, Reverse: true}, devices)

	setupModel, _ := m.setupModel.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.setupModel = setupModel
	m.setupModel = m.setupModel.WithVirtualSinkLifecycle(onStop, recent.SinkRecreate)
	m.screen = ScreenStreaming
	return m
}

type virtualSinkCleanupStub struct {
	findSinkNames []string
	removedIDs    []string
}

func stubVirtualSinkCleanup(t *testing.T, resolvedID string) *virtualSinkCleanupStub {
	t.Helper()
	oldRemove := removePulseAudioSink
	oldFind := findPulseAudioSinkModule
	stub := &virtualSinkCleanupStub{}

	removePulseAudioSink = func(moduleID string) error {
		stub.removedIDs = append(stub.removedIDs, moduleID)
		return nil
	}
	findPulseAudioSinkModule = func(sinkName string) (string, bool, error) {
		stub.findSinkNames = append(stub.findSinkNames, sinkName)
		return resolvedID, true, nil
	}

	t.Cleanup(func() {
		removePulseAudioSink = oldRemove
		findPulseAudioSinkModule = oldFind
	})
	return stub
}
