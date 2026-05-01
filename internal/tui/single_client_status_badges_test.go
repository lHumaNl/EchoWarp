package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func newStreamingModel(cfg config.Config, client transport.ClientInfo) Model {
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.startTime = time.Now()
	m.stats = transport.ConnectionStats{State: "connected"}
	m.multiStats = transport.MultiClientStats{Clients: []transport.ClientInfo{client}, MaxClients: 1}
	return m
}

func TestSingleClientNormalServerShowsPeerMutedYou(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, MaxClients: 1}
	m := newStreamingModel(cfg, transport.ClientInfo{ClientID: "client-1", Muted: true})

	assert.Contains(t, m.View(), "[PEER MUTED YOU]")
}

func TestSingleClientReverseServerShowsSourcePaused(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, MaxClients: 1, Reverse: true}
	m := newStreamingModel(cfg, transport.ClientInfo{ClientID: "client-1", Paused: true})

	assert.Contains(t, m.View(), "[SOURCE PAUSED]")
}

func TestSingleClientReverseServerShowsIncomingMuted(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, MaxClients: 1, Reverse: true}
	m := newStreamingModel(cfg, transport.ClientInfo{ClientID: "client-1", MutedIncoming: true})

	assert.Contains(t, m.View(), "[INCOMING MUTED]")
}

func TestSingleClientReverseClientShowsPeerMutedYou(t *testing.T) {
	cfg := config.Config{Mode: config.ModeClient, MaxClients: 1, Reverse: true}
	m := newStreamingModel(cfg, transport.ClientInfo{})
	updated, _ := m.Update(PeerMuteMsg(true))

	assert.Contains(t, updated.(Model).View(), "[PEER MUTED YOU]")
}

func TestSingleClientDuplexCombinesBadges(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, MaxClients: 1, Duplex: true}
	client := transport.ClientInfo{ClientID: "client-1", Muted: true, MutedIncoming: true, Paused: true}
	m := newStreamingModel(cfg, client)
	m.paused = true

	output := m.View()
	assert.Contains(t, output, "[PAUSED]")
	assert.Contains(t, output, "[SOURCE PAUSED]")
	assert.Contains(t, output, "[INCOMING MUTED]")
	assert.Contains(t, output, "[PEER MUTED YOU]")
	assert.Less(t, strings.Index(output, "[PAUSED]"), strings.Index(output, "[SOURCE PAUSED]"))
}
