package tui

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func startupTestModel(t *testing.T, client bool) Model {
	t.Helper()
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Devices = []config.DeviceEntry{{ID: 0, Role: config.RoleCapture, Volume: 1}}
	if client {
		cfg.Mode = config.ModeClient
		cfg.Address = "example.org"
		cfg.Devices[0].Role = config.RolePlayback
	}
	m := NewModelWithOutputDevices(cfg, []audio.AudioDevice{{ID: 0, Name: "Mic", IsInput: true}, {ID: 0, Name: "Speakers"}}, nil, startup.Intent{Requested: true})
	m.startupNetwork = startup.Network{Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return &probe.ProbeServerResult{Mode: "normal", SampleRate: 48000, Channels: 2}, nil
	}}
	return m
}

func runFiniteStartupCommands(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var messages []tea.Msg
		for _, child := range batch {
			messages = append(messages, runFiniteStartupCommands(child)...)
		}
		return messages
	}
	return []tea.Msg{msg}
}

func TestStartupUsesSharedTransitionOnce(t *testing.T) {
	for _, client := range []bool{false, true} {
		t.Run(map[bool]string{false: "server", true: "client"}[client], func(t *testing.T) {
			m := startupTestModel(t, client)
			calls := 0
			var receivedStop <-chan struct{}
			m = m.WithStartFunc(func(cfg config.Config, stop <-chan struct{}) (<-chan transport.ConnectionStats, <-chan error, <-chan struct{}) {
				calls++
				receivedStop = stop
				require.Equal(t, uint32(0), *cfg.DeviceID)
				require.NotNil(t, stop)
				return nil, nil, nil
			})
			updated, cmd := m.Update(startupBeginMsg{})
			m = updated.(Model)
			prepared := cmd().(startupPreparedMsg)
			require.NoError(t, prepared.err)
			updated, cmd = m.Update(prepared)
			m = updated.(Model)
			require.Equal(t, ScreenConnection, m.screen)
			require.NotNil(t, m.stopCh)
			require.NotNil(t, m.stopOnce)
			require.Len(t, m.deviceStates, 1)
			runFiniteStartupCommands(cmd)
			require.Equal(t, 1, calls)
			_, cmd = m.Update(prepared)
			require.Nil(t, cmd)
			_, cmd = m.Update(startupBeginMsg{})
			require.Nil(t, cmd)
			_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
			require.NotNil(t, cmd)
			select {
			case <-receivedStop:
			default:
				t.Fatal("quit did not close the stop channel received by StartFunc")
			}
		})
	}
}

func TestStartupUserInputCancelsStaleResult(t *testing.T) {
	m := startupTestModel(t, true)
	updated, cmd := m.Update(startupBeginMsg{})
	m = updated.(Model)
	prepared := cmd().(startupPreparedMsg)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd = m.Update(prepared)
	m = updated.(Model)
	require.Nil(t, cmd)
	require.Equal(t, ScreenDeviceSelect, m.screen)
	require.False(t, m.startupPending)
}

func TestStartupUnavailableStaysInSetup(t *testing.T) {
	m := startupTestModel(t, true)
	m.startupNetwork.Probe = func(context.Context, string, int) (*probe.ProbeServerResult, error) {
		return nil, errors.New("offline")
	}
	updated, cmd := m.Update(startupBeginMsg{})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	require.Equal(t, ScreenDeviceSelect, m.screen)
	require.Contains(t, m.View(), "offline")
	_, cmd = m.Update(startupBeginMsg{})
	require.Nil(t, cmd)
}

func TestStartupRecentHistoryWaitsForConnection(t *testing.T) {
	m := startupTestModel(t, true)
	m.screen = ScreenConnection
	stats, errs := make(chan transport.ConnectionStats), make(chan error)
	close(stats)
	close(errs)
	updated, cmd := m.Update(streamingStartedMsg{statsCh: stats, errCh: errs})
	m = updated.(Model)
	runFiniteStartupCommands(cmd)
	history, err := recent.Load()
	require.NoError(t, err)
	require.Empty(t, history)
	updated, cmd = m.Update(StatsUpdateMsg{Stats: transport.ConnectionStats{State: "connecting"}})
	m = updated.(Model)
	runFiniteStartupCommands(cmd)
	history, _ = recent.Load()
	require.Empty(t, history)
	updated, cmd = m.Update(StatsUpdateMsg{Stats: transport.ConnectionStats{State: "connected"}})
	m = updated.(Model)
	runFiniteStartupCommands(cmd)
	history, err = recent.Load()
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, "normal", history[0].LastMode)
	require.True(t, m.recentSaved)
}
