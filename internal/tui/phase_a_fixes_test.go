package tui

// Tests for Phase A bug fixes:
//   Bug 4: Server in reverse mode — pauseCh should be nil after proceedWithStart
//   Bug 5: Client in reverse mode — serverMuteCh should be nil after proceedWithStart
//   Bug 7: Mute toggle does NOT set flashMsg; kick/ban DO set flashMsg

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// makeReverseModel creates a model in pure reverse mode with pauseCh and serverMuteCh wired.
func makeReverseModel() Model {
	cfg := config.Config{Reverse: true, Duplex: false, Conference: false, Mode: config.ModeServer}
	m := NewModel(cfg, nil)
	m.pauseCh = make(chan bool, 4)
	m.serverMuteCh = make(chan bool, 4)
	return m
}

// TestBug4_ReversePauseCh_NilAfterProceedWithStart verifies that pauseCh is nil for a
// reverse-only server after proceedWithStart (no capture device in reverse mode).
func TestBug4_ReversePauseCh_NilAfterProceedWithStart(t *testing.T) {
	t.Parallel()
	m := makeReverseModel()
	assert.NotNil(t, m.pauseCh, "pre-condition: pauseCh was wired")

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.Nil(t, got.pauseCh, "pauseCh must be nil in reverse-only server mode")
}

// TestBug4_ReverseClient_PauseCh_Preserved verifies that pauseCh is kept for reverse client.
func TestBug4_ReverseClient_PauseCh_Preserved(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Reverse: true, Mode: config.ModeClient}
	m := NewModel(cfg, nil)
	m.pauseCh = make(chan bool, 4)
	m.serverMuteCh = make(chan bool, 4)

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.NotNil(t, got.pauseCh, "pauseCh must be kept for reverse client (captures audio)")
	assert.Nil(t, got.serverMuteCh, "serverMuteCh must be nil for reverse client (no playback)")
}

// TestBug4_NormalMode_PauseCh_Preserved verifies that pauseCh is kept for normal mode.
func TestBug4_NormalMode_PauseCh_Preserved(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Reverse: false}
	m := NewModel(cfg, nil)
	m.pauseCh = make(chan bool, 4)

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.NotNil(t, got.pauseCh, "pauseCh must be kept in normal server mode")
}

// TestBug4_DuplexMode_PauseCh_Preserved verifies that pauseCh is kept for duplex mode.
func TestBug4_DuplexMode_PauseCh_Preserved(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Reverse: true, Duplex: true}
	m := NewModel(cfg, nil)
	m.pauseCh = make(chan bool, 4)

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.NotNil(t, got.pauseCh, "pauseCh must be kept in duplex mode")
}

// TestBug5_ReverseServerMuteCh_NilAfterProceedWithStart verifies that serverMuteCh is nil
// for reverse-only client (server sends no audio to client in reverse mode).
func TestBug5_ReverseServerMuteCh_NilAfterProceedWithStart(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Reverse: true, Mode: config.ModeClient}
	m := NewModel(cfg, nil)
	m.pauseCh = make(chan bool, 4)
	m.serverMuteCh = make(chan bool, 4)

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.Nil(t, got.serverMuteCh, "serverMuteCh must be nil in reverse-only client mode")
	assert.NotNil(t, got.pauseCh, "pauseCh must be kept for reverse client")
}

// TestBug5_NormalMode_ServerMuteCh_Preserved verifies serverMuteCh is kept for normal mode.
func TestBug5_NormalMode_ServerMuteCh_Preserved(t *testing.T) {
	t.Parallel()
	cfg := config.Config{Reverse: false}
	m := NewModel(cfg, nil)
	m.serverMuteCh = make(chan bool, 4)

	result, _ := m.proceedWithStart(nil)
	got := result.(Model)

	assert.NotNil(t, got.serverMuteCh, "serverMuteCh must be kept in normal client mode")
}

// makeStreamingModel builds a minimal streaming model with cmdCh and one connected client.
func makeStreamingModel() Model {
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, MaxClients: 2}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test", Channels: 1, SampleRate: 48000},
	}
	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.multiClient = true
	m.cmdCh = make(chan ClientCommand, 16)
	m.multiStats = transport.MultiClientStats{
		Clients: []transport.ClientInfo{
			{ClientID: "client-1", Nickname: "Alice", MutedOutgoing: false, MutedIncoming: false},
		},
	}
	m.selectedClient = 0
	return m
}

// TestBug7_MuteOutgoing_NoFlash verifies Ctrl+O (mute outgoing) does NOT set flashMsg.
func TestBug7_MuteOutgoing_NoFlash(t *testing.T) {
	t.Parallel()
	m := makeStreamingModel()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	got := result.(Model)

	assert.Empty(t, got.flashMsg, "mute outgoing toggle must NOT set flashMsg")
}

// TestBug7_MuteIncoming_NoFlash verifies Ctrl+M (mute incoming in multi-client) does NOT set flashMsg.
func TestBug7_MuteIncoming_NoFlash(t *testing.T) {
	t.Parallel()
	m := makeStreamingModel()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := result.(Model)

	assert.Empty(t, got.flashMsg, "mute incoming toggle must NOT set flashMsg")
}

// TestBug7_PopupMuteOutgoing_NoFlash verifies dispatchPopupAction mute_outgoing does NOT set flashMsg.
func TestBug7_PopupMuteOutgoing_NoFlash(t *testing.T) {
	t.Parallel()
	m := makeStreamingModel()
	m.popupClientID = "client-1"
	m.popupClientNick = "Alice"

	result, _ := m.dispatchPopupAction("mute_outgoing")
	got := result.(Model)

	assert.Empty(t, got.flashMsg, "popup mute_outgoing must NOT set flashMsg")
}

// TestBug7_PopupMuteIncoming_NoFlash verifies dispatchPopupAction mute_incoming does NOT set flashMsg.
func TestBug7_PopupMuteIncoming_NoFlash(t *testing.T) {
	t.Parallel()
	m := makeStreamingModel()
	m.popupClientID = "client-1"
	m.popupClientNick = "Alice"

	result, _ := m.dispatchPopupAction("mute_incoming")
	got := result.(Model)

	assert.Empty(t, got.flashMsg, "popup mute_incoming must NOT set flashMsg")
}

// TestBug7_Kick_SetsFlash verifies executeKick DOES set flashMsg.
func TestBug7_Kick_SetsFlash(t *testing.T) {
	t.Parallel()
	origFunc := reasonsFilePathFunc
	reasonsFilePathFunc = func() string { return filepath.Join(t.TempDir(), "reasons.json") }
	defer func() { reasonsFilePathFunc = origFunc }()

	m := makeStreamingModel()
	m.kickClientID = "client-1"
	m.kickClientNick = "Alice"

	result, _ := m.executeKick("spam")
	got := result.(Model)

	assert.Contains(t, got.flashMsg, "kicked", "kick must set a flash message")
}

// TestBug7_Ban_SetsFlash verifies executeBan DOES set flashMsg.
func TestBug7_Ban_SetsFlash(t *testing.T) {
	t.Parallel()
	origFunc := reasonsFilePathFunc
	reasonsFilePathFunc = func() string { return filepath.Join(t.TempDir(), "reasons.json") }
	defer func() { reasonsFilePathFunc = origFunc }()

	m := makeStreamingModel()
	m.banClientID = "client-1"
	m.banClientNick = "Alice"
	m.banCriteriaIP = true // at least one criterion required

	result, _ := m.executeBan("spam")
	got := result.(Model)

	assert.Contains(t, got.flashMsg, "banned", "ban action must set a flash message")
}
