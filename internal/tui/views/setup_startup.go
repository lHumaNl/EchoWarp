package views

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/config"
)

func (m *SetupModel) SetStartupProblem(message string) tea.Cmd {
	m.validationError = message
	m.validationTimer = time.Now().Add(10 * time.Second)
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg { return ValidationDismissMsg{} })
}

// WithPreparedConfig displays safely resolved startup values without executing
// preset restoration/lifecycle operations or requiring synthetic key presses.
func (m SetupModel) WithPreparedConfig(cfg config.Config, info *ProbeServerResult) SetupModel {
	m.cfg = cfg
	m.pendingRestoreCmd = nil
	m.restoreOverlay = nil
	m.isDuplexMode = cfg.Duplex || cfg.Conference
	m.isConferenceMode = cfg.Conference
	m.probeResult = info
	if info != nil {
		m.probeStatus = "ok"
	}
	// Device/mode values have already been merged; do not apply ClientModes again.
	cfg.ClientModes = nil
	m.loadedClientModes = nil
	m.applyLoadedConfig(cfg)
	for _, entry := range cfg.Devices {
		rows := m.outputDevices
		if entry.Role == config.RoleCapture {
			rows = m.inputDevices
		}
		for i := range rows {
			if rows[i].ID == entry.ID && rows[i].Name == entry.Name {
				rows[i].Volume, rows[i].AGC = entry.Volume, entry.AGC
			}
		}
	}
	return m
}
