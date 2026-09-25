package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/startup"
)

type startupBeginMsg struct{}
type startupPreparedMsg struct {
	id     uint64
	config config.Config
	probe  *probe.ProbeServerResult
	err    error
}

func (m Model) beginStartup() (tea.Model, tea.Cmd) {
	if m.startupAttempted || !m.startupIntent.Requested || m.screen != ScreenDeviceSelect || m.quitting {
		return m, nil
	}
	m.startupAttempted, m.startupPending = true, true
	m.startupID++
	id := m.startupID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.startupCancel = cancel
	cfg, devices, intent := m.config, m.devices, m.startupIntent
	network := m.startupNetwork
	if network.Probe == nil {
		network = startup.DefaultNetwork()
	}
	return m, func() tea.Msg {
		defer cancel()
		prepared, info, err := startup.Resolve(ctx, cfg, devices, intent, network)
		return startupPreparedMsg{id: id, config: prepared, probe: info, err: err}
	}
}

func (m Model) finishStartup(msg startupPreparedMsg) (tea.Model, tea.Cmd) {
	if !m.startupPending || m.startupID != msg.id || m.screen != ScreenDeviceSelect || m.quitting {
		return m, nil
	}
	m.startupPending = false
	m.startupCancel = nil
	m.config = msg.config
	m.setupModel = m.setupModel.WithPreparedConfig(msg.config, msg.probe)
	if msg.err != nil {
		flash := m.setupModel.SetStartupProblem("Automatic start paused: " + msg.err.Error())
		return m, tea.Batch(flash, m.setupModel.InitCmd())
	}
	// Resolve already performed the pre-start probe and validation. Reuse the
	// normal transition, including device states and owned stop channel.
	return m.proceedWithStart(nil)
}

func (m *Model) cancelStartup() {
	if m.startupCancel != nil {
		m.startupCancel()
		m.startupCancel = nil
	}
	m.startupPending = false
	m.startupID++
}
