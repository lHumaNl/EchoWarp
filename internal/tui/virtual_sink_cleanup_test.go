package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/internal/virtualstate"
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
	assert.Equal(t, []string{"EchoWarp"}, stub.findSinkNames)
	assert.Equal(t, []string{"42"}, stub.removedIDs)
}

func TestCleanupRunsAgainAfterNewSessionVirtualSinkRecorded(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("7")
	stub := stubVirtualSinkCleanup(t, "42")

	m.cleanupVirtualSinks()
	beforePlans := m.setupModel.VirtualSinkCleanupPlans()
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("8")
	m.resetVirtualSinkCleanupLatchIfSessionSinkRecorded(beforePlans)
	m.cleanupVirtualSinks()

	assert.Equal(t, []string{"42", "42"}, stub.removedIDs)
	assert.Equal(t, []string{"EchoWarp", "EchoWarp"}, stub.findSinkNames)
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

func TestClientShutdownDoesNotDeleteServerOwnedVirtualSink(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	policy := virtualstate.DevicePolicy{OnStop: recent.SinkDelete, OnStart: recent.SinkRecreate}
	require.NoError(t, virtualstate.UpsertPresent("EchoWarp", "Monitor of EchoWarp", "42", virtualstate.RoleServer, policy))
	devices := []audio.AudioDevice{{ID: 99, Name: "EchoWarp", IsInput: false, Channels: 2, SampleRate: 48000}}
	m := NewModel(config.Config{Mode: config.ModeClient, Reverse: true}, devices)
	m.setupModel = m.setupModel.WithVirtualSinkLifecycle(recent.SinkDelete, recent.SinkRecreate)
	m.screen = ScreenStreaming
	stub := stubVirtualSinkCleanup(t, "42")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)

	assert.True(t, m.quitting)
	assert.Empty(t, stub.findSinkNames)
	assert.Empty(t, stub.removedIDs)
}

func TestCleanupSkipsRemovalWhenExactSinkLookupDoesNotFindCurrentSink(t *testing.T) {
	m := newVirtualSinkCleanupModel(t, recent.SinkDelete)
	m.setupModel = m.setupModel.WithVirtualSinkCreatedForSession("stale-module")
	stub := stubVirtualSinkCleanupLookup(t, map[string]virtualSinkCleanupLookupResult{})

	m.cleanupVirtualSinks()

	assert.Equal(t, []string{"EchoWarp"}, stub.findSinkNames)
	assert.Empty(t, stub.removedIDs)
}

func TestCleanupLatchStaysRetryableAfterPartialPlanFailure(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	m := Model{}
	stub := stubVirtualSinkCleanupLookup(t, map[string]virtualSinkCleanupLookupResult{
		"good": {moduleID: "41", found: true},
		"bad":  {moduleID: "42", found: true},
	})
	stub.removeErrByID = map[string]error{"42": errors.New("remove failed")}
	plans := []views.VirtualSinkCleanupPlan{
		{Delete: true, SinkName: "good"},
		{Delete: true, SinkName: "bad"},
	}

	cleaned := m.cleanupVirtualSinkPlans(plans)

	assert.False(t, cleaned)
	assert.Equal(t, []string{"41", "42"}, stub.removedIDs)
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
	removeErrByID map[string]error
}

type virtualSinkCleanupLookupResult struct {
	moduleID string
	found    bool
}

func stubVirtualSinkCleanup(t *testing.T, resolvedID string) *virtualSinkCleanupStub {
	t.Helper()
	return stubVirtualSinkCleanupLookup(t, map[string]virtualSinkCleanupLookupResult{
		"EchoWarp": {moduleID: resolvedID, found: true},
	})
}

func stubVirtualSinkCleanupLookup(t *testing.T, resolved map[string]virtualSinkCleanupLookupResult) *virtualSinkCleanupStub {
	t.Helper()
	oldRemove := removePulseAudioSink
	oldFind := findPulseAudioSinkModule
	stub := &virtualSinkCleanupStub{}

	removePulseAudioSink = func(moduleID string) error {
		stub.removedIDs = append(stub.removedIDs, moduleID)
		if err := stub.removeErrByID[moduleID]; err != nil {
			return err
		}
		return nil
	}
	findPulseAudioSinkModule = func(sinkName string) (string, bool, error) {
		stub.findSinkNames = append(stub.findSinkNames, sinkName)
		result, ok := resolved[sinkName]
		if !ok {
			return "", false, nil
		}
		return result.moduleID, result.found, nil
	}

	t.Cleanup(func() {
		removePulseAudioSink = oldRemove
		findPulseAudioSinkModule = oldFind
	})
	return stub
}
