package app

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestServerListeningHookOnlyAfterBind(t *testing.T) {
	for _, maxClients := range []int{1, 2} {
		t.Run(string(rune('0'+maxClients)), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Port = 0
			cfg.MaxClients = maxClients
			ready := make(chan struct{}, 1)
			server := NewServerApp(cfg, testAppLogger(), nil, nil, nil).WithListeningHook(func() { ready <- struct{}{} })
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- server.Run(ctx) }()
			select {
			case <-ready:
			case <-time.After(time.Second):
				t.Fatal("listener never became ready")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("server did not stop")
			}
			require.Empty(t, ready, "listener hook must fire once")
		})
	}
}

func TestServerOccupiedPortDoesNotReportReady(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	for _, maxClients := range []int{1, 2} {
		cfg := config.DefaultConfig()
		cfg.Port = port
		cfg.MaxClients = maxClients
		var calls atomic.Int32
		server := NewServerApp(cfg, testAppLogger(), nil, nil, nil).WithListeningHook(func() { calls.Add(1) })
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		err = server.Run(ctx)
		cancel()
		require.Error(t, err)
		require.NotErrorIs(t, err, context.DeadlineExceeded, "bind failure must return, not silently retry or wait forever")
		require.Zero(t, calls.Load())
	}
}
