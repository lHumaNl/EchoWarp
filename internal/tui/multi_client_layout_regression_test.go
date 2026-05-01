package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestProceedWithStart_ClientNormalMaxClientsKeepsSingleClientLayout(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Mode: config.ModeClient, MaxClients: 4}, nil)
	updated, _ := m.proceedWithStart(nil)
	got := updated.(Model)

	assert.False(t, got.multiClient)
	assert.False(t, got.conference)
}

func TestProceedWithStart_ServerNormalMaxClientsEnablesMultiClientLayout(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Mode: config.ModeServer, MaxClients: 4}, nil)
	updated, _ := m.proceedWithStart(nil)
	got := updated.(Model)

	assert.True(t, got.multiClient)
	assert.False(t, got.conference)
}

func TestProceedWithStart_ConferenceEnablesConferenceLayout(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Mode: config.ModeClient, Conference: true, MaxClients: 4}, nil)
	updated, _ := m.proceedWithStart(nil)
	got := updated.(Model)

	assert.True(t, got.multiClient)
	assert.True(t, got.conference)
}

func TestClientNormalStreamingViewUsesStatsAndParticipantSidebar(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Mode: config.ModeClient, MaxClients: 4, SampleRate: 48000, Channels: 2}, nil)
	m.screen = ScreenStreaming
	m.startTime = time.Now()
	m.stats = transport.ConnectionStats{
		State:      "connected",
		RemoteAddr: "10.0.0.2:4415",
		BytesSent:  4096,
		BytesRecv:  8192,
		Jitter:     2.5,
		RoundTrip:  12.5,
	}
	m.participants = []string{"Alice", "Bob"}
	m.maxClients = 4

	output := m.View()

	assert.Contains(t, output, "10.0.0.2:4415")
	assert.Contains(t, output, "Online (2/4)")
	assert.Contains(t, output, "Jitter")
	assert.NotContains(t, output, "Waiting for clients")
	assert.NotContains(t, output, "No clients")
	assert.NotContains(t, output, "0/0 clients")
}

func TestParticipantsUpdateMsgUpdatesParticipantsAndMaxClients(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Mode: config.ModeClient}, nil)
	updated, _ := m.Update(ParticipantsUpdateMsg{
		Participants: []string{"Alice", "Bob"},
		MaxClients:   4,
	})
	got := updated.(Model)

	assert.Equal(t, []string{"Alice", "Bob"}, got.participants)
	assert.Equal(t, 4, got.maxClients)
}
