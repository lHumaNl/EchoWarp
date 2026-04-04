package app

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/app/mocks"
	"github.com/lHumaNl/echowarp/internal/config"
)

func TestRemoteAddrConn_ImplementsNetConn(t *testing.T) {
	rac := &remoteAddrConn{addr: "10.0.0.5:12345"}
	var _ net.Conn = rac // compile-time check

	assert.Equal(t, "10.0.0.5:12345", rac.RemoteAddr().String())
	assert.Equal(t, "tcp", rac.RemoteAddr().Network())
	assert.NoError(t, rac.Close())
}

func TestRemoteAddrConn_CloseCallsCloseFn(t *testing.T) {
	called := false
	rac := &remoteAddrConn{
		addr:    "1.2.3.4:5678",
		closeFn: func() error { called = true; return nil },
	}
	err := rac.Close()
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProcessCommands_SingleClient_Kick(t *testing.T) {
	cmdCh := make(chan ClientCommand, 4)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s := &ServerApp{
		cfg:     config.Config{MaxClients: 1},
		logger:  logger,
		clients: make(map[string]*multiClient),
		cmdCh:   cmdCh,
	}

	mockPeer := mocks.NewMockPeer()
	s.clients["client-1"] = &multiClient{
		id:       "client-1",
		nickname: "TestUser",
		conn:     &remoteAddrConn{addr: "192.168.1.1:9999"},
		peer:     mockPeer,
		joinedAt: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.processCommands(ctx)

	cmdCh <- ClientCommand{Action: ActionKick, ClientID: "client-1", Reason: "test kick"}
	time.Sleep(100 * time.Millisecond)

	assert.True(t, mockPeer.IsClosed(), "peer should be closed after kick")
}

func TestProcessCommands_SingleClient_MuteOutgoing(t *testing.T) {
	cmdCh := make(chan ClientCommand, 4)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s := &ServerApp{
		cfg:     config.Config{MaxClients: 1},
		logger:  logger,
		clients: make(map[string]*multiClient),
		cmdCh:   cmdCh,
	}

	mockPeer := mocks.NewMockPeer()
	mc := &multiClient{
		id:       "client-1",
		nickname: "TestUser",
		conn:     &remoteAddrConn{addr: "192.168.1.1:9999"},
		peer:     mockPeer,
		joinedAt: time.Now(),
	}
	s.clients["client-1"] = mc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.processCommands(ctx)

	cmdCh <- ClientCommand{Action: ActionMuteOutgoing, ClientID: "client-1"}
	time.Sleep(100 * time.Millisecond)
	assert.True(t, mc.mutedOutgoing.Load(), "client should be muted outgoing after toggle")

	cmdCh <- ClientCommand{Action: ActionMuteOutgoing, ClientID: "client-1"}
	time.Sleep(100 * time.Millisecond)
	assert.False(t, mc.mutedOutgoing.Load(), "client should be unmuted outgoing after second toggle")
}

func TestProcessCommands_SingleClient_MuteIncoming(t *testing.T) {
	cmdCh := make(chan ClientCommand, 4)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s := &ServerApp{
		cfg:     config.Config{MaxClients: 1},
		logger:  logger,
		clients: make(map[string]*multiClient),
		cmdCh:   cmdCh,
	}

	mockPeer := mocks.NewMockPeer()
	mc := &multiClient{
		id:       "client-1",
		nickname: "TestUser",
		conn:     &remoteAddrConn{addr: "192.168.1.1:9999"},
		peer:     mockPeer,
		joinedAt: time.Now(),
	}
	s.clients["client-1"] = mc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.processCommands(ctx)

	cmdCh <- ClientCommand{Action: ActionMuteIncoming, ClientID: "client-1"}
	time.Sleep(100 * time.Millisecond)
	assert.True(t, mc.mutedIncoming.Load(), "client should be muted incoming after toggle")
}

func TestCollectMultiClientStats_SingleClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	s := &ServerApp{
		cfg:     config.Config{MaxClients: 1},
		logger:  logger,
		clients: make(map[string]*multiClient),
	}

	mockPeer := mocks.NewMockPeer()
	s.clients["client-1"] = &multiClient{
		id:       "client-1",
		nickname: "Alice",
		conn:     &remoteAddrConn{addr: "10.0.0.1:4415"},
		peer:     mockPeer,
		joinedAt: time.Now(),
	}

	stats := s.collectMultiClientStats()
	require.Len(t, stats.Clients, 1)
	assert.Equal(t, "client-1", stats.Clients[0].ClientID)
	assert.Equal(t, "Alice", stats.Clients[0].Nickname)
	assert.Equal(t, "10.0.0.1", stats.Clients[0].RemoteAddr)
	assert.Equal(t, 1, stats.MaxClients)
}
