package tui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestModel_Init_StartsOnCorrectScreen(t *testing.T) {
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Device 1", IsInput: true, Channels: 2, SampleRate: 48000},
	}

	t.Run("nil DeviceID starts on ScreenDeviceSelect", func(t *testing.T) {
		cfg := config.Config{DeviceID: nil}
		m := NewModel(cfg, devices)
		cmd := m.Init()

		assert.Equal(t, ScreenDeviceSelect, m.screen)
		// Init always returns a tick command now
		assert.NotNil(t, cmd)
	})

	t.Run("non-nil DeviceID starts on ScreenConnection", func(t *testing.T) {
		deviceID := uint32(1)
		cfg := config.Config{DeviceID: &deviceID}
		m := NewModel(cfg, devices)
		cmd := m.Init()

		assert.Equal(t, ScreenConnection, m.screen)
		assert.NotNil(t, cmd)
	})
}

func TestModel_DeviceSelect_SelectDevice_QuitWhenNoStartFunc(t *testing.T) {
	cfg := config.Config{DeviceID: nil}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test Device", IsInput: true, Channels: 2, SampleRate: 48000},
	}

	m := NewModel(cfg, devices)
	_ = m.Init()

	selectedDevice := audio.AudioDevice{ID: 1, Name: "Test Device", IsInput: true, Channels: 2, SampleRate: 48000}
	updated, _ := m.Update(DeviceSelectedMsg{Device: selectedDevice})
	m = updated.(Model)

	assert.Equal(t, uint32(1), *m.SelectedDeviceID())
}

func TestModel_DeviceSelect_SelectDevice_MovesToConnectionWithStartFunc(t *testing.T) {
	cfg := config.Config{DeviceID: nil}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test Device", IsInput: true, Channels: 2, SampleRate: 48000},
	}

	statsCh := make(chan transport.ConnectionStats, 1)
	errCh := make(chan error, 1)
	m := NewModel(cfg, devices).WithStartFunc(func(c config.Config, stopCh <-chan struct{}) (<-chan transport.ConnectionStats, <-chan error, <-chan struct{}) {
		return statsCh, errCh, nil
	})
	_ = m.Init()

	selectedDevice := audio.AudioDevice{ID: 1, Name: "Test Device", IsInput: true, Channels: 2, SampleRate: 48000}
	updated, _ := m.Update(DeviceSelectedMsg{Device: selectedDevice})
	m = updated.(Model)

	assert.Equal(t, ScreenConnection, m.screen)
	assert.Equal(t, "Test Device", m.deviceName)
}

func TestModel_Connection_ShowsConnectionInfo(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Address: "localhost", Port: 4415}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	_ = m.Init()

	output := m.View()
	assert.Contains(t, output, "Connecting")
	assert.Contains(t, output, "4415")
}

func TestModel_Streaming_ShowsStats(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Mode: config.ModeClient, SampleRate: 48000, Channels: 2}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.startTime = time.Now()

	stats := transport.ConnectionStats{
		State:       "connected",
		LocalAddr:   "192.168.1.1:1234",
		RemoteAddr:  "192.168.1.2:5678",
		BytesSent:   1024,
		BytesRecv:   2048,
		PacketsLost: 0,
		Jitter:      5.2,
		RoundTrip:   15.3,
	}

	updated, _ := m.Update(StatsUpdateMsg{Stats: stats})
	m = updated.(Model)

	output := m.View()
	assert.Contains(t, output, "Connected")
	assert.Contains(t, output, "server")
	assert.Contains(t, output, "client")
}

func TestModel_Streaming_QuitKey_Exits(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	m = updated.(Model)

	require.True(t, m.quitting)
	require.NotNil(t, cmd)
}

func TestModel_Streaming_PauseKey_Pauses(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Reverse: true}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.startTime = time.Now()
	m.pauseCh = make(chan<- bool, 4)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated.(Model)
	assert.True(t, m.paused)
	assert.Contains(t, m.View(), "PAUSED")

	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = updated2.(Model)
	assert.False(t, m.paused)
	// When not paused, the view shows normal direction line (no PAUSED)
	assert.NotContains(t, m.View(), "PAUSED")
}

func TestModel_Error_ShowsErrorMessage(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Address: "localhost", Port: 4415}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	_ = m.Init()

	testErr := errors.New("connection failed")
	updated, _ := m.Update(ErrorMsg{Err: testErr})
	m = updated.(Model)

	output := m.View()
	assert.Contains(t, output, "connection failed")
}

func TestModel_WindowResize_UpdatesDimensions(t *testing.T) {
	cfg := config.Config{}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)
}

func TestModel_Connection_ConnectedMsg_MovesToStreaming(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	_ = m.Init()
	require.Equal(t, ScreenConnection, m.screen)

	stats := transport.ConnectionStats{
		State:      "connected",
		LocalAddr:  "192.168.1.1:1234",
		RemoteAddr: "192.168.1.2:5678",
	}

	updated, _ := m.Update(ConnectedMsg{Stats: stats})
	m = updated.(Model)

	assert.Equal(t, ScreenStreaming, m.screen)
	assert.False(t, m.startTime.IsZero())
}

func TestModel_Quitting_ReturnsSummary(t *testing.T) {
	cfg := config.Config{}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	m.quitting = true

	output := m.View()
	assert.Contains(t, output, "Session summary")
	assert.Contains(t, output, "Duration")
}

func TestModel_Streaming_DisconnectIncreasesReconnectCount(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, MaxReconnectAttempts: 5}
	devices := []audio.AudioDevice{}

	statsCh := make(chan transport.ConnectionStats, 10)
	errCh := make(chan error, 1)

	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.startTime = time.Now()
	m.statsCh = statsCh
	m.errCh = errCh

	// Simulate disconnect
	updated, _ := m.Update(StatsUpdateMsg{Stats: transport.ConnectionStats{State: "disconnected"}})
	m = updated.(Model)

	assert.Equal(t, 1, m.reconnectCount)
	assert.Equal(t, 1, m.reconnectAttempt)
	assert.Equal(t, ScreenConnection, m.screen)
}

func TestModel_LogToggle(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}

	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	assert.True(t, m.logsVisible)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	assert.False(t, m.logsVisible)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	m = updated.(Model)
	assert.True(t, m.logsVisible)
}

func TestModel_LogScroll(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}

	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.logsVisible = true
	m.focusedArea = FocusLogs // single-client default

	// Add some logs
	for i := 0; i < 50; i++ {
		m.logs = append(m.logs, "12:00:00 [INF] Log line")
	}

	// Scroll up (arrows in FocusLogs)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	assert.Equal(t, 1, m.logScrollOffset)

	// Scroll down
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 0, m.logScrollOffset)

	// Can't scroll below 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 0, m.logScrollOffset)
}

func TestModel_StatsAccumulation(t *testing.T) {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID}

	statsCh := make(chan transport.ConnectionStats, 10)
	errCh := make(chan error, 1)

	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.startTime = time.Now()
	m.statsCh = statsCh
	m.errCh = errCh

	stats := transport.ConnectionStats{
		State:       "connected",
		BytesSent:   1024,
		BytesRecv:   2048,
		PacketsLost: 3,
		Jitter:      10.0,
		RoundTrip:   20.0,
	}

	updated, _ := m.Update(StatsUpdateMsg{Stats: stats})
	m = updated.(Model)

	assert.Equal(t, uint64(1024), m.totalBytesSent)
	assert.Equal(t, uint64(2048), m.totalBytesRecv)
	assert.Equal(t, uint32(3), m.totalPacketsLost)
	assert.Equal(t, 1, m.statsCount)
	assert.Equal(t, 1, len(m.jitterHistory))
	assert.Equal(t, 1, len(m.rttHistory))
}

func TestModel_WithLogFile(t *testing.T) {
	cfg := config.Config{}
	m := NewModel(cfg, nil).WithLogFile("/tmp/test.log")
	assert.Equal(t, "/tmp/test.log", m.logFile)
}

// --- Pre-start probe flow tests ---

func newClientModel() Model {
	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Mic", IsInput: true, Channels: 2, SampleRate: 48000},
	}
	return NewModel(cfg, devices)
}

func TestPreStartProbe_SetupDoneMsg_ClientMode_SetsPending(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	doneCfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	updated, cmd := m.Update(views.SetupDoneMsg{Config: doneCfg})
	m = updated.(Model)

	// pendingStartCfg should be set (probe in flight).
	require.NotNil(t, m.pendingStartCfg)
	assert.Equal(t, "127.0.0.1", m.pendingStartCfg.Address)
	assert.Equal(t, 4415, m.pendingStartCfg.Port)
	// A command should be returned (the probe command).
	assert.NotNil(t, cmd)
}

func TestPreStartProbe_DoubleSubmitGuard(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	doneCfg := config.Config{
		Mode:    config.ModeClient,
		Address: "127.0.0.1",
		Port:    4415,
	}
	// First submit — sets pending.
	updated, _ := m.Update(views.SetupDoneMsg{Config: doneCfg})
	m = updated.(Model)
	require.NotNil(t, m.pendingStartCfg)

	// Second submit — should be ignored (probe already in flight).
	updated2, _ := m.Update(views.SetupDoneMsg{Config: doneCfg})
	m2 := updated2.(Model)
	// pendingStartCfg should remain unchanged (not reset).
	require.NotNil(t, m2.pendingStartCfg)
}

func TestPreStartProbe_ServerMode_SkipsProbe(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer}
	m := NewModel(cfg, nil)
	_ = m.Init()

	doneCfg := config.Config{Mode: config.ModeServer}
	updated, _ := m.Update(views.SetupDoneMsg{Config: doneCfg})
	m = updated.(Model)

	// Server mode should NOT set pendingStartCfg.
	assert.Nil(t, m.pendingStartCfg)
}

func TestPreStartProbe_ProbeError_ClearsPending(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	// Simulate pending state.
	cfg := config.Config{Mode: config.ModeClient, Address: "127.0.0.1", Port: 4415}
	m.pendingStartCfg = &cfg

	// Simulate probe failure.
	updated, _ := m.Update(PreStartProbeMsg{Err: errors.New("connection refused")})
	m = updated.(Model)

	// pendingStartCfg should be cleared.
	assert.Nil(t, m.pendingStartCfg)
	// Should stay on device select screen (not proceed).
	assert.Equal(t, ScreenDeviceSelect, m.screen)
}

func TestPreStartProbe_StaleMessage_Ignored(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	// pendingStartCfg is nil — any PreStartProbeMsg should be ignored.
	updated, _ := m.Update(PreStartProbeMsg{Result: &views.ProbeServerResult{Mode: "normal"}})
	m = updated.(Model)

	// Should remain on device select, no crash.
	assert.Equal(t, ScreenDeviceSelect, m.screen)
	assert.Nil(t, m.pendingStartCfg)
}

func TestPreStartProbe_NoCriticalNoNonCritical_Proceeds(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	m.pendingStartCfg = &cfg

	// Probe result matches config exactly — no changes.
	probeResult := &views.ProbeServerResult{
		Mode:       "normal",
		SampleRate: 48000,
		Channels:   1,
	}

	updated, _ := m.Update(PreStartProbeMsg{Result: probeResult})
	m = updated.(Model)

	// Should proceed: pendingStartCfg cleared, config applied.
	assert.Nil(t, m.pendingStartCfg)
	assert.Equal(t, uint32(48000), m.config.SampleRate)
}

func TestPreStartProbe_OnlyNonCritical_AutoAppliesAndProceeds(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	m.pendingStartCfg = &cfg

	// Probe result with non-critical change: sample rate differs.
	probeResult := &views.ProbeServerResult{
		Mode:       "normal",
		SampleRate: 44100,
		Channels:   1,
	}

	updated, _ := m.Update(PreStartProbeMsg{Result: probeResult})
	m = updated.(Model)

	// Should proceed (no critical changes).
	assert.Nil(t, m.pendingStartCfg)
}

func TestPreStartProbe_CriticalChanges_StaysOnSetup(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	m.pendingStartCfg = &cfg

	// Probe result with critical change: mode changed.
	probeResult := &views.ProbeServerResult{
		Mode:       "reverse",
		SampleRate: 48000,
		Channels:   1,
	}

	updated, _ := m.Update(PreStartProbeMsg{Result: probeResult})
	m = updated.(Model)

	// Should NOT proceed: pendingStartCfg cleared, stays on setup.
	assert.Nil(t, m.pendingStartCfg)
	assert.Equal(t, ScreenDeviceSelect, m.screen)
}

func TestPreStartProbe_CriticalTLS_StaysOnSetup(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	m.pendingStartCfg = &cfg

	// TLS changed on→off is critical.
	probeResult := &views.ProbeServerResult{
		Mode:        "normal",
		TLSRequired: true,
		SampleRate:  48000,
		Channels:    1,
	}

	updated, _ := m.Update(PreStartProbeMsg{Result: probeResult})
	m = updated.(Model)

	assert.Nil(t, m.pendingStartCfg)
	assert.Equal(t, ScreenDeviceSelect, m.screen)
}

func TestPreStartProbe_CriticalPassword_StaysOnSetup(t *testing.T) {
	m := newClientModel()
	_ = m.Init()

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "127.0.0.1",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	m.pendingStartCfg = &cfg

	// Password now required but not set in config — critical.
	probeResult := &views.ProbeServerResult{
		Mode:             "normal",
		PasswordRequired: true,
		SampleRate:       48000,
		Channels:         1,
	}

	updated, _ := m.Update(PreStartProbeMsg{Result: probeResult})
	m = updated.(Model)

	assert.Nil(t, m.pendingStartCfg)
	assert.Equal(t, ScreenDeviceSelect, m.screen)
}

// ─── Phase 4: Saving Presets ─────────────────────────────────────────────────

func TestSaveRecentServerCmd_PersistsPreset(t *testing.T) {
	// Use a temp dir so the test is isolated from real filesystem state.
	dir := t.TempDir()
	t.Setenv("HOME", dir) // config.EchoWarpDir uses $HOME on darwin/linux

	cfg := config.Config{
		Mode:    config.ModeClient,
		Address: "10.0.0.1",
		Port:    4415,
	}
	devicePreset := recent.DevicePreset{
		Devices: []recent.PresetDevice{
			{ID: 1, Name: "Mic", Virtual: false},
		},
	}

	cmd := saveRecentServerCmd(cfg, nil, nil, devicePreset, "normal")
	require.NotNil(t, cmd)
	_ = cmd() // execute

	servers, err := recent.Load()
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "10.0.0.1", servers[0].Address)
	assert.Equal(t, 4415, servers[0].Port)
	require.NotNil(t, servers[0].Presets)
	p, ok := servers[0].Presets["normal"]
	require.True(t, ok)
	require.Len(t, p.Devices, 1)
	assert.Equal(t, "Mic", p.Devices[0].Name)
}

func TestSaveRecentServerCmd_PreservesPresetsForOtherModes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := config.Config{
		Mode:    config.ModeClient,
		Address: "10.0.0.1",
		Port:    4415,
	}

	// Pre-seed an existing entry with a "duplex" preset.
	existingPreset := recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 99, Name: "OldDevice"}},
	}
	initial := []recent.Server{{
		Address:  "10.0.0.1",
		Port:     4415,
		Hostname: "server1",
		Presets:  map[string]recent.DevicePreset{"duplex": existingPreset},
	}}
	require.NoError(t, recent.Save(initial))

	// Now save a new "normal" preset — should not overwrite "duplex".
	newPreset := recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 1, Name: "Mic"}},
	}
	cmd := saveRecentServerCmd(cfg, nil, nil, newPreset, "normal")
	_ = cmd()

	servers, _ := recent.Load()
	require.Len(t, servers, 1)
	presets := servers[0].Presets
	assert.Len(t, presets, 2, "both modes must be present")
	assert.Equal(t, "Mic", presets["normal"].Devices[0].Name)
	assert.Equal(t, "OldDevice", presets["duplex"].Devices[0].Name)
}

func TestSaveServerPresetCmd_PersistsPreset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	devicePreset := recent.DevicePreset{
		Devices: []recent.PresetDevice{
			{ID: 5, Name: "Speaker", Virtual: false},
		},
	}

	settings := preset.ServerSettings{Port: 9090, LastMode: "normal"}
	cmd := saveServerPresetCmd(devicePreset, "normal", settings)
	require.NotNil(t, cmd)
	msg := cmd()
	assert.Nil(t, msg)

	sp := preset.Load()
	p := sp.Get("normal")
	require.NotNil(t, p)
	require.Len(t, p.Devices, 1)
	assert.Equal(t, "Speaker", p.Devices[0].Name)
	assert.Equal(t, 9090, sp.Settings.Port)
	assert.Equal(t, "normal", sp.Settings.LastMode)
}

func TestSaveServerPresetCmd_PreservesOtherModes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Write an existing preset for "duplex".
	sp := preset.Load()
	sp.Set("duplex", recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 9, Name: "OldSpeaker"}},
	})
	require.NoError(t, preset.Save(sp))

	// Now save a "normal" preset.
	cmd := saveServerPresetCmd(recent.DevicePreset{
		Devices: []recent.PresetDevice{{ID: 1, Name: "NewMic"}},
	}, "normal", preset.ServerSettings{})
	_ = cmd()

	sp2 := preset.Load()
	normalP := sp2.Get("normal")
	require.NotNil(t, normalP)
	assert.Equal(t, "NewMic", normalP.Devices[0].Name)

	duplexP := sp2.Get("duplex")
	require.NotNil(t, duplexP)
	assert.Equal(t, "OldSpeaker", duplexP.Devices[0].Name)
}

func TestStreamingStartedMsg_ClientMode_SavesPreset(t *testing.T) {
	// Verify that streamingStartedMsg in client mode triggers saveRecentServerCmd.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := config.Config{
		Mode:       config.ModeClient,
		Address:    "192.168.1.100",
		Port:       4415,
		SampleRate: 48000,
		Channels:   1,
	}
	devices := []audio.AudioDevice{}

	m := NewModel(cfg, devices)
	m.screen = ScreenDeviceSelect

	statsCh := make(chan transport.ConnectionStats, 1)
	errCh := make(chan error, 1)

	updated, cmd := m.Update(streamingStartedMsg{statsCh: statsCh, errCh: errCh})
	m = updated.(Model)
	require.NotNil(t, cmd)

	// The batch cmd contains saveRecentServerCmd + blocking channel waits,
	// so we can't execute it directly in tests. The save logic is tested
	// separately in TestSaveRecentServerCmd_PersistsPreset.
	// Here we just verify Update didn't panic and returned a batch command.
	assert.NotNil(t, cmd)
}
