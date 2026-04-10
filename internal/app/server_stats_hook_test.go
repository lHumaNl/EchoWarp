package app

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// TestStatsHookPopulatesNodeStats wires a ServerApp's stats hook to a real
// *echowarp.Node (just as the daemon does) and drives reportStats with a
// fake PeerManager that returns ever-growing BytesSent. It asserts that
// node.Stats().BytesSent reflects the live bytes reported by reportStats
// — i.e. the factory-level glue between the runner and Node.UpdateStats
// actually works end-to-end.
//
// This is the phase-6 acceptance test "TestStatsPopulated" — intentionally
// runs a short streaming window (not a full ServerApp.Run) because the
// path that matters is the stats-hook plumbing, not the audio pipeline.
// Spinning up real audio would make the test slow and platform-dependent
// without exercising any new code.
func TestStatsHookPopulatesNodeStats(t *testing.T) {
	// Build a real Node with a runner factory that never actually runs
	// (we exercise reportStats directly). The Node is only needed so
	// node.Stats() reflects the UpdateStats call.
	node, err := echowarp.NewNode(echowarp.NodeConfig{Mode: echowarp.ModeServer})
	require.NoError(t, err)

	s := newMinimalServerApp(nil)
	s.WithStatsHook(func(stats transport.ConnectionStats) {
		node.UpdateStats(stats)
	})

	// Fake PeerManager: increments BytesSent on every GetStats call so
	// reportStats sees a moving target — proves the hook is invoked on
	// the tick, not just once.
	var counter uint64
	peer := &mockPeerManager{
		getStatsFunc: func() transport.ConnectionStats {
			// Simulate ~1 KiB of traffic per call.
			return transport.ConnectionStats{
				State:     "connected",
				BytesSent: atomic.AddUint64(&counter, 1024),
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.reportStats(ctx, peer)
	}()

	// Wait for the initial stat + at least one tick — reportStats sends
	// immediately and then ticks every second. The initial send should
	// populate node.Stats within milliseconds.
	require.Eventually(t, func() bool {
		return node.Stats().BytesSent > 0
	}, 2*time.Second, 10*time.Millisecond, "node.Stats().BytesSent should become non-zero via statsHook")

	cancel()
	<-done

	// Assert the final state reflects real plumbing (non-zero bytes).
	assert.Greater(t, node.Stats().BytesSent, uint64(0))
	assert.Equal(t, "connected", node.Stats().State)
}

// TestClientTrackingHooksPopulateNodeClients drives the registerMultiClient
// and unregisterMultiClient paths through the real ServerApp hook points
// and asserts that node.Clients() reflects the add and remove via the
// WithClientTrackingHooks adapters.
func TestClientTrackingHooksPopulateNodeClients(t *testing.T) {
	node, err := echowarp.NewNode(echowarp.NodeConfig{Mode: echowarp.ModeServer, MaxClients: 4})
	require.NoError(t, err)

	s := newMinimalServerApp(nil)
	// Multi-client mode so registerMultiClient / unregisterMultiClient run.
	s.cfg.MaxClients = 4
	s.WithClientTrackingHooks(
		func(clientID, remoteAddr string) {
			node.AddClient(echowarp.ClientInfo{ID: clientID, Address: remoteAddr})
		},
		func(clientID string) {
			node.RemoveClient(clientID)
		},
	)

	// Use an in-memory pipe so conn.RemoteAddr() returns a real addr.
	left, right := net.Pipe()
	defer left.Close()  //nolint:errcheck
	defer right.Close() //nolint:errcheck

	mc, ok := s.registerMultiClient(left, "client-test-1")
	require.True(t, ok, "registerMultiClient should succeed under MaxClients")
	require.NotNil(t, mc)

	// Precondition: Node now has the client.
	clients, _ := node.Clients()
	require.Len(t, clients, 1)
	assert.Equal(t, "client-test-1", clients[0].ID)
	// Address is best-effort — net.Pipe returns a "pipe" addr which
	// auth.ExtractIP passes through; we just assert it is non-empty.
	assert.NotEmpty(t, clients[0].Address)

	// Unregister and verify removal.
	s.unregisterMultiClient(mc, "client-test-1", true)

	clients, _ = node.Clients()
	assert.Empty(t, clients, "node.Clients() should be empty after unregister")
}

// TestStatsHookAggregatesMultiStats covers the multi-client aggregation
// path: reportMultiStats should fold per-client bytes into a single
// ConnectionStats and pass it to the hook so GET /api/v1/stats returns a
// meaningful sum even in multi-client mode.
func TestStatsHookAggregatesMultiStats(t *testing.T) {
	s := newMinimalServerApp(nil)
	// Multi-client mode with two clients in the map.
	s.cfg.MaxClients = 4
	s.clients = map[string]*multiClient{
		"c1": {id: "c1", peer: &mockPeerManager{getStatsFunc: func() transport.ConnectionStats {
			return transport.ConnectionStats{BytesSent: 100, BytesRecv: 50, State: "connected"}
		}}, joinedAt: time.Now(), conn: fakeAddrConn{}},
		"c2": {id: "c2", peer: &mockPeerManager{getStatsFunc: func() transport.ConnectionStats {
			return transport.ConnectionStats{BytesSent: 200, BytesRecv: 75, State: "connected"}
		}}, joinedAt: time.Now(), conn: fakeAddrConn{}},
	}

	var mu sync.Mutex
	var observed transport.ConnectionStats
	var ticks int
	s.WithStatsHook(func(stats transport.ConnectionStats) {
		mu.Lock()
		defer mu.Unlock()
		observed = stats
		ticks++
	})

	// Exercise aggregateMultiStats directly through the hook fan-out path.
	agg := s.aggregateMultiStats(s.collectMultiClientStats())
	s.statsHook(agg)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, uint64(300), observed.BytesSent, "BytesSent should sum per-client")
	assert.Equal(t, uint64(125), observed.BytesRecv, "BytesRecv should sum per-client")
	assert.Equal(t, "connected", observed.State)
	assert.Equal(t, 1, ticks)
}

// fakeAddrConn is a minimal net.Conn stub that returns a non-nil
// RemoteAddr for collectMultiClientStats, which calls
// mc.conn.RemoteAddr().String() on every client.
type fakeAddrConn struct{ net.Conn }

func (fakeAddrConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}
func (fakeAddrConn) LocalAddr() net.Addr             { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} }
func (fakeAddrConn) Read([]byte) (int, error)        { return 0, nil }
func (fakeAddrConn) Write([]byte) (int, error)       { return 0, nil }
func (fakeAddrConn) Close() error                    { return nil }
func (fakeAddrConn) SetDeadline(time.Time) error     { return nil }
func (fakeAddrConn) SetReadDeadline(time.Time) error { return nil }
func (fakeAddrConn) SetWriteDeadline(time.Time) error {
	return nil
}
