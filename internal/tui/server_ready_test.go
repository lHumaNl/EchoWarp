package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestServerProfileSavedOnlyForMatchingListener(t *testing.T) {
	t.Setenv("ECHOWARP_CONFIG_DIR", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Conference = true
	cfg.ServerMuted = true
	cfg.Port = 5544
	cfg.MaxClients = 8
	m := NewModelWithOutputDevices(cfg, nil, nil, startup.Intent{Requested: true})
	m.screen = ScreenConnection
	m.stopCh = make(chan struct{})
	stats := make(chan transport.ConnectionStats)
	errs := make(chan error)
	close(stats)
	close(errs)
	updated, cmd := m.Update(streamingStartedMsg{statsCh: stats, errCh: errs})
	m = updated.(Model)
	runFiniteStartupCommands(cmd)
	require.Empty(t, preset.Load().Presets, "attempted startup must not overwrite previous profile")
	updated, cmd = m.Update(ServerReadyMsg{Stop: make(chan struct{})})
	m = updated.(Model)
	runFiniteStartupCommands(cmd)
	require.Empty(t, preset.Load().Presets, "stale listener event must not save current config")
	_, cmd = m.Update(ServerReadyMsg{Stop: m.stopCh})
	runFiniteStartupCommands(cmd)
	saved := preset.Load()
	require.Equal(t, "conference", saved.LastMode)
	require.True(t, saved.Get("conference").ServerMuted)
	require.Equal(t, 5544, saved.Get("conference").Port)
	require.Equal(t, 1, saved.Get("conference").Version)
}
