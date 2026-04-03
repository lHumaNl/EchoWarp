package views

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func baseConferenceParams() ConferenceParams {
	return ConferenceParams{
		Stats: transport.MultiClientStats{
			MaxClients: 4,
			Clients: []transport.ClientInfo{
				{ClientID: "Client-1", Nickname: "Client-1", RemoteAddr: "10.0.0.1:5000", Duration: "20s", State: "connected", BytesSent: 1000, Jitter: 1.0, RoundTrip: 2.0},
				{ClientID: "Client-2", Nickname: "Client-2", RemoteAddr: "10.0.0.2:5000", Duration: "11s", State: "connected", BytesSent: 500, Jitter: 0.5, RoundTrip: 1.0},
			},
		},
		IsServer:        true,
		HubMode:         false,
		SelectedIndex:   1, // first client (server is 0)
		DeviceName:      "Mic MacBook",
		Width:           120,
		Height:          30,
		ClientQualities: []QualityLevel{QualityExcellent, QualityGood},
		SelectedStats: transport.ConnectionStats{
			State: "connected", RemoteAddr: "10.0.0.1:5000",
			BytesSent: 1000, Jitter: 1.0, RoundTrip: 2.0,
		},
		SelectedQuality: QualityExcellent,
	}
}

func TestConferenceView_ServerNonHub_ShowsServerInList(t *testing.T) {
	p := baseConferenceParams()
	result := ConferenceView(p)

	assert.Contains(t, result, "server (me)")
	assert.Contains(t, result, "Client-1")
	assert.Contains(t, result, "Client-2")
	assert.Contains(t, result, "conference mode")
	assert.NotContains(t, result, "conference mode (hub)")
}

func TestConferenceView_ServerHub_NoServerInList(t *testing.T) {
	p := baseConferenceParams()
	p.HubMode = true
	p.ServerMuted = true
	p.SelectedIndex = 0 // first client
	result := ConferenceView(p)

	assert.NotContains(t, result, "server (me)")
	assert.Contains(t, result, "conference mode (hub)")
	assert.Contains(t, result, "Client-1")
}

func TestConferenceView_Client_ShowsOnlineParticipants(t *testing.T) {
	p := ConferenceParams{
		Stats: transport.MultiClientStats{
			MaxClients: 4,
			Clients: []transport.ClientInfo{
				{ClientID: "Client-2", Nickname: "Client-2", RemoteAddr: "10.0.0.2:5000", Duration: "11s"},
			},
		},
		IsServer:           false,
		SelectedIndex:      2,
		Width:              120,
		Height:             30,
		OnlineParticipants: []string{"server", "Client-1", "Client-2"},
		MyNickname:         "Client-1",
		MaxClients:         4,
		ClientQualities:    []QualityLevel{QualityGood},
	}
	result := ConferenceView(p)

	assert.Contains(t, result, "server")
	assert.Contains(t, result, "Client-1 (me)")
	assert.Contains(t, result, "Client-2")
	assert.Contains(t, result, "conference mode")
}

func TestConferenceParticipantCount_ServerNonHub(t *testing.T) {
	p := baseConferenceParams()
	count := ConferenceParticipantCount(p)
	// server + 2 clients = 3
	assert.Equal(t, 3, count)
}

func TestConferenceParticipantCount_ServerHub(t *testing.T) {
	p := baseConferenceParams()
	p.HubMode = true
	count := ConferenceParticipantCount(p)
	// only 2 clients (no server)
	assert.Equal(t, 2, count)
}

func TestConferenceView_ServerEntry_NoNetworkStats(t *testing.T) {
	p := baseConferenceParams()
	p.SelectedIndex = 0 // server (me) selected
	result := ConferenceView(p)

	assert.Contains(t, result, "Local participant")
}

func TestConferenceView_AggregateTraffic(t *testing.T) {
	p := baseConferenceParams()
	p.AggBytesSent = 263000
	p.AggBytesRecv = 128000
	p.AggBitrateUp = 134.0
	p.AggBitrateDown = 64.0
	result := ConferenceView(p)

	assert.Contains(t, result, "↑")
	assert.Contains(t, result, "↓")
}

func TestConferenceView_DetailHeader_ParticipantsCount(t *testing.T) {
	p := baseConferenceParams()
	p.SelectedIndex = 1 // Client-1

	result := ConferenceView(p)
	// Participants 3/5 (2 clients + 1 server / max 4 + 1 server)
	assert.Contains(t, result, "Participants 3/5")
}

func TestConferenceView_DetailHeader_HubParticipantsCount(t *testing.T) {
	p := baseConferenceParams()
	p.HubMode = true
	p.ServerMuted = true
	p.SelectedIndex = 0

	result := ConferenceView(p)
	// Hub: only clients in count
	assert.Contains(t, result, "Participants 2/4")
}

func TestConferenceView_NarrowFallback(t *testing.T) {
	p := baseConferenceParams()
	p.Width = 40 // below minTerminalWidth=50
	result := ConferenceView(p)

	// Should still render without panics
	assert.Contains(t, result, "conference mode")
	assert.Contains(t, result, "Participants")
}

func TestConferenceView_Paused(t *testing.T) {
	p := baseConferenceParams()
	p.Paused = true
	result := ConferenceView(p)

	assert.Contains(t, result, "[PAUSED]")
}

func TestConferenceView_EmptyClients(t *testing.T) {
	p := baseConferenceParams()
	p.Stats.Clients = nil
	p.ClientQualities = nil
	p.SelectedIndex = 0
	result := ConferenceView(p)

	// Server (me) still shows, detail shows local participant
	assert.Contains(t, result, "server (me)")
}

func TestBuildConferenceList_ServerNonHub(t *testing.T) {
	p := baseConferenceParams()
	entries := buildConferenceList(p)

	assert.Len(t, entries, 3)
	assert.True(t, entries[0].isServer)
	assert.Equal(t, "server (me)", entries[0].label)
	assert.Equal(t, 0, entries[1].clientIdx)
	assert.Equal(t, 1, entries[2].clientIdx)
}

func TestBuildConferenceList_ServerHub(t *testing.T) {
	p := baseConferenceParams()
	p.HubMode = true
	entries := buildConferenceList(p)

	assert.Len(t, entries, 2)
	assert.False(t, entries[0].isServer)
}

func TestBuildConferenceList_Client(t *testing.T) {
	p := ConferenceParams{
		Stats: transport.MultiClientStats{
			Clients: []transport.ClientInfo{
				{ClientID: "Client-2", Nickname: "Client-2"},
			},
		},
		IsServer:           false,
		OnlineParticipants: []string{"server", "Me", "Client-2"},
		MyNickname:         "Me",
	}
	entries := buildConferenceList(p)

	assert.Len(t, entries, 3)
	assert.True(t, entries[0].isServer)
	assert.Equal(t, "Me (me)", entries[1].label)
	assert.Equal(t, "Client-2", entries[2].label)
	assert.Equal(t, 0, entries[2].clientIdx)
}

func TestConferenceView_MasterDetailLayout(t *testing.T) {
	p := baseConferenceParams()
	p.SelectedIndex = 1
	result := ConferenceView(p)

	// Should contain the column separator
	assert.True(t, strings.Contains(result, "│"), "should contain column separator │")
}

func TestConferenceView_PausedParticipant_ShowsBadge(t *testing.T) {
	p := baseConferenceParams()
	p.Participants = []ConferenceParticipant{
		{ID: "Client-1"},
		{ID: "Client-2"},
	}
	p.PausedParticipants = map[string]bool{
		"Client-1": true,
	}
	result := ConferenceView(p)

	assert.Contains(t, result, "⏸", "should show pause badge for paused participant")
}

func TestConferenceView_NoPausedParticipants_NoBadge(t *testing.T) {
	p := baseConferenceParams()
	p.Participants = []ConferenceParticipant{
		{ID: "Client-1"},
		{ID: "Client-2"},
	}
	p.PausedParticipants = nil
	result := ConferenceView(p)

	assert.NotContains(t, result, "⏸", "should not show pause badge when no one is paused")
}

func TestConferenceView_MutedAndPaused_ShowsBothBadges(t *testing.T) {
	p := baseConferenceParams()
	p.Participants = []ConferenceParticipant{
		{ID: "Client-1"},
	}
	p.MutedParticipants = map[string]bool{"Client-1": true}
	p.PausedParticipants = map[string]bool{"Client-1": true}
	result := ConferenceView(p)

	assert.Contains(t, result, "🔇", "should show muted badge")
	assert.Contains(t, result, "⏸", "should show pause badge")
}
