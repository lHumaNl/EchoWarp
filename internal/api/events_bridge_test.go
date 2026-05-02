package api

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

// fakeBroadcaster captures WSEvent values for assertion in tests.
type fakeBroadcaster struct {
	mu     sync.Mutex
	events []WSEvent
}

func (f *fakeBroadcaster) Broadcast(e WSEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
}

func (f *fakeBroadcaster) snapshot() []WSEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]WSEvent, len(f.events))
	copy(out, f.events)
	return out
}

func (f *fakeBroadcaster) waitFor(t *testing.T, n int, timeout time.Duration) []WSEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(f.snapshot()) >= n {
			return f.snapshot()
		}
		time.Sleep(5 * time.Millisecond)
	}
	return f.snapshot()
}

// TestEventBusBridge verifies that BridgeEventsToWS forwards each supported
// EventBus event type to the WSHub with the correct string type identifier
// and preserves the original event data payload.
func TestEventBusBridge(t *testing.T) {
	bus := echowarp.NewEventBus()
	defer bus.Shutdown()

	hub := &fakeBroadcaster{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bridgeDone := make(chan struct{})
	go func() {
		BridgeEventsToWS(ctx, bus, hub, logger)
		close(bridgeDone)
	}()

	// Give the bridge a moment to register its subscriptions before emitting.
	time.Sleep(20 * time.Millisecond)

	bus.EmitConnected("10.0.0.1:1234")
	bus.EmitDisconnected("peer reset")
	bus.EmitError(errors.New("boom"))
	bus.EmitStatsUpdate(echowarp.ConnectionStats{})
	bus.EmitClientJoined("client-1", "10.0.0.2:5678")
	bus.EmitClientLeft("client-1")
	bus.EmitChatMessage("Alice", "Bob", "psst", 42)

	events := hub.waitFor(t, 7, 2*time.Second)
	require.Len(t, events, 7, "all seven events should be forwarded to the hub")

	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	assert.ElementsMatch(t,
		[]string{"connected", "disconnected", "error", "stats_updated", "client_joined", "client_left", "chat_message"},
		types,
	)

	// Verify data mapping for a couple of representative events.
	for _, e := range events {
		switch e.Type {
		case "connected":
			d, ok := e.Data.(echowarp.ConnectedData)
			require.True(t, ok, "connected event data must be ConnectedData")
			assert.Equal(t, "10.0.0.1:1234", d.Addr)
		case "client_joined":
			d, ok := e.Data.(echowarp.ClientJoinedData)
			require.True(t, ok, "client_joined event data must be ClientJoinedData")
			assert.Equal(t, "client-1", d.ClientID)
			assert.Equal(t, "10.0.0.2:5678", d.Addr)
		case "error":
			d, ok := e.Data.(map[string]string)
			require.True(t, ok, "error event data must be map[string]string")
			assert.Equal(t, "boom", d["error"])
		case "chat_message":
			d, ok := e.Data.(echowarp.ChatMessageData)
			require.True(t, ok, "chat_message event data must be ChatMessageData")
			assert.Equal(t, "Alice", d.From)
			assert.Equal(t, "Bob", d.To)
			assert.Equal(t, "psst", d.Text)
			assert.Equal(t, int64(42), d.TS)
		}
	}

	// Cancellation unsubscribes and returns from the goroutine.
	cancel()
	select {
	case <-bridgeDone:
	case <-time.After(1 * time.Second):
		t.Fatal("BridgeEventsToWS did not return after ctx cancellation")
	}
}

// TestEventBusBridge_NilInputs ensures the bridge tolerates nil bus/hub.
func TestEventBusBridge_NilInputs(t *testing.T) {
	// Should not panic and should return promptly.
	done := make(chan struct{})
	go func() {
		BridgeEventsToWS(context.Background(), nil, &fakeBroadcaster{}, nil)
		BridgeEventsToWS(context.Background(), echowarp.NewEventBus(), nil, nil)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("nil input case should return immediately")
	}
}
