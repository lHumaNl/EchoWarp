package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
)

// ServerReadyMsg is correlated with the exact stop channel of a launch. Late
// listener events cannot save a newer failed attempt's configuration.
type ServerReadyMsg struct{ Stop <-chan struct{} }

func (m Model) WithServerReadyChannel(ch <-chan ServerReadyMsg) Model {
	m.serverReadyCh = ch
	return m
}

func waitServerReady(ch <-chan ServerReadyMsg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m Model) handleServerReady(msg ServerReadyMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{waitServerReady(m.serverReadyCh)}
	if m.config.Mode == config.ModeServer && !m.quitting && msg.Stop != nil && msg.Stop == m.stopCh && m.serverReadySaved != msg.Stop {
		m.serverReadySaved = msg.Stop
		mode := m.config.AudioMode()
		mp := m.setupModel.CollectModePreset(mode, m.config)
		cmds = append(cmds, saveServerPresetCmd(mp, mode))
	}
	return m, tea.Batch(cmds...)
}
