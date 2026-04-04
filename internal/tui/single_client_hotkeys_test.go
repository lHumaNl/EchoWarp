package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// newSingleClientServerModel creates a Model simulating a single-client server
// that is connected and streaming.
func newSingleClientServerModel() Model {
	devID := uint32(1)
	cfg := config.Config{
		Mode:       config.ModeServer,
		MaxClients: 1,
		DeviceID:   &devID,
	}
	devices := []audio.AudioDevice{{ID: 1, Name: "Test", IsInput: true, Channels: 2, SampleRate: 48000}}
	cmdCh := make(chan ClientCommand, 16)
	multiStatsCh := make(chan transport.MultiClientStats, 4)

	m := NewModel(cfg, devices).
		WithCommandChannel(cmdCh).
		WithSingleClientStats(multiStatsCh)
	m.screen = ScreenStreaming
	m.stats = transport.ConnectionStats{State: "connected"}
	m.multiStats = transport.MultiClientStats{
		Clients: []transport.ClientInfo{
			{ClientID: "client-1", Nickname: "Alice", RemoteAddr: "10.0.0.1", HWID: "hw-123"},
		},
		MaxClients: 1,
	}
	return m
}

func TestSingleClientInfo_WithMultiStats(t *testing.T) {
	m := newSingleClientServerModel()
	id, nick := m.singleClientInfo()
	assert.Equal(t, "client-1", id)
	assert.Equal(t, "Alice", nick)
}

func TestSingleClientInfo_FallbackToConnected(t *testing.T) {
	m := newSingleClientServerModel()
	m.multiStats.Clients = nil // no multiStats yet
	id, nick := m.singleClientInfo()
	assert.Equal(t, "client-1", id)
	assert.Equal(t, "client-1", nick)
}

func TestSingleClientInfo_NotConnected(t *testing.T) {
	m := newSingleClientServerModel()
	m.multiStats.Clients = nil
	m.stats.State = "new"
	id, nick := m.singleClientInfo()
	assert.Equal(t, "", id)
	assert.Equal(t, "", nick)
}

func TestSingleClient_EnterOpensPopup(t *testing.T) {
	m := newSingleClientServerModel()
	assert.False(t, m.multiClient, "should not be multiClient")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.Equal(t, OverlayClientPopup, m2.overlay)
	assert.Equal(t, "client-1", m2.popupClientID)
	assert.Equal(t, "Alice", m2.popupClientNick)
}

func TestSingleClient_EnterNoPopupWhenDisconnected(t *testing.T) {
	m := newSingleClientServerModel()
	m.multiStats.Clients = nil
	m.stats.State = "new"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(Model)
	assert.Equal(t, OverlayNone, m2.overlay)
}

// Ctrl+K and Ctrl+B hotkeys removed — kick/ban only accessible via popup (Enter → select action).

func TestSingleClient_CtrlK_NoEffect(t *testing.T) {
	m := newSingleClientServerModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	m2 := updated.(Model)
	assert.Equal(t, OverlayNone, m2.overlay)
}

func TestSingleClient_CtrlB_NoEffect(t *testing.T) {
	m := newSingleClientServerModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	m2 := updated.(Model)
	assert.Equal(t, OverlayNone, m2.overlay)
}

func TestWithSingleClientStats_DoesNotSetMultiClient(t *testing.T) {
	ch := make(chan transport.MultiClientStats, 1)
	cfg := config.Config{MaxClients: 1}
	devices := []audio.AudioDevice{{ID: 1, Name: "Test", IsInput: true, Channels: 2, SampleRate: 48000}}
	m := NewModel(cfg, devices).WithSingleClientStats(ch)
	assert.False(t, m.multiClient)
	assert.NotNil(t, m.multiStatsCh)
}
