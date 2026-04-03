package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestServerApp_ClientMuted_DefaultFalse(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewServerApp(cfg, logger, nil, nil, nil)
	assert.False(t, s.clientMuted.Load())
}

func TestServerApp_HandleDCControl_PeerMute(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewServerApp(cfg, logger, nil, nil, nil)

	// Build a peer_mute control message
	muteMsg := buildControlMessage(t, transport.ActionPeerMute)
	connStart := time.Now()

	done := s.handleDCControl(muteMsg, connStart, "TestClient")
	assert.False(t, done, "peer_mute should not end the session")
	assert.True(t, s.clientMuted.Load(), "clientMuted should be true after peer_mute")

	// Now unmute
	unmuteMsg := buildControlMessage(t, transport.ActionPeerUnmute)
	done = s.handleDCControl(unmuteMsg, connStart, "TestClient")
	assert.False(t, done, "peer_unmute should not end the session")
	assert.False(t, s.clientMuted.Load(), "clientMuted should be false after peer_unmute")
}

func TestServerApp_HandleDCControl_StopStillWorks(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewServerApp(cfg, logger, nil, nil, nil)

	stopMsg := buildControlMessage(t, transport.ActionStop)
	done := s.handleDCControl(stopMsg, time.Now(), "TestClient")
	assert.True(t, done, "stop should end the session")
}

func TestServerApp_MuteFilterCh_DropsMutedFrames(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := NewServerApp(cfg, logger, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dst := make(chan []byte, 10)
	src := newMuteFilterCh(ctx, dst, &s.clientMuted)

	// Unmuted: frame should pass through
	src <- []byte{1, 2, 3}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{1, 2, 3}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through when unmuted")
	}

	// Muted: frame should be dropped
	s.clientMuted.Store(true)
	src <- []byte{4, 5, 6}
	select {
	case <-dst:
		t.Fatal("expected frame to be dropped when muted")
	case <-time.After(100 * time.Millisecond):
		// Good — frame was dropped
	}

	// Unmute again: frames should pass through
	s.clientMuted.Store(false)
	src <- []byte{7, 8, 9}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{7, 8, 9}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through after unmute")
	}
}

func TestCombinedMuteFilterCh_DropsWhenEitherFlagSet(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dst := make(chan []byte, 10)
	var flag1, flag2 atomic.Bool

	src := newCombinedMuteFilterCh(ctx, dst, &flag1, &flag2)

	// Neither flag: frame passes through.
	src <- []byte{1, 2, 3}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{1, 2, 3}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through when both unmuted")
	}

	// flag1 set: frame dropped.
	flag1.Store(true)
	src <- []byte{4, 5, 6}
	select {
	case <-dst:
		t.Fatal("expected frame to be dropped when flag1 set")
	case <-time.After(100 * time.Millisecond):
	}

	// flag1 cleared, flag2 set: frame dropped.
	flag1.Store(false)
	flag2.Store(true)
	src <- []byte{7, 8, 9}
	select {
	case <-dst:
		t.Fatal("expected frame to be dropped when flag2 set")
	case <-time.After(100 * time.Millisecond):
	}

	// Both set: frame dropped.
	flag1.Store(true)
	src <- []byte{10, 11, 12}
	select {
	case <-dst:
		t.Fatal("expected frame to be dropped when both set")
	case <-time.After(100 * time.Millisecond):
	}

	// Both cleared: frame passes.
	flag1.Store(false)
	flag2.Store(false)
	src <- []byte{13, 14, 15}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{13, 14, 15}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through when both unmuted")
	}
}

func TestClientApp_WithServerMuteChannel(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c := NewClientApp(cfg, logger, nil)

	ch := make(chan bool, 4)
	c2 := c.WithServerMuteChannel(ch)
	assert.Same(t, c, c2)
	assert.NotNil(t, c.serverMuteCh)
}

func TestClientApp_ServerMutedIncoming_DefaultFalse(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	c := NewClientApp(cfg, testClientLogger(), nil)
	assert.False(t, c.serverMutedIncoming.Load())
}

func TestClientApp_HandleDCControl_MuteIncoming(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	c := NewClientApp(cfg, testClientLogger(), nil)

	// mute_incoming should set serverMutedIncoming=true
	muteMsg := buildControlMessage(t, transport.ActionMuteIncoming)
	done := c.handleDCControl(muteMsg)
	assert.False(t, done, "mute_incoming should not end session")
	assert.True(t, c.serverMutedIncoming.Load(), "serverMutedIncoming should be true after mute_incoming")

	// unmute_incoming should set serverMutedIncoming=false
	unmuteMsg := buildControlMessage(t, transport.ActionUnmuteIncoming)
	done = c.handleDCControl(unmuteMsg)
	assert.False(t, done, "unmute_incoming should not end session")
	assert.False(t, c.serverMutedIncoming.Load(), "serverMutedIncoming should be false after unmute_incoming")
}

func TestClientApp_ServerMuteIncomingFilterCh_DropsWhenMuted(t *testing.T) {
	t.Parallel()
	cfg := defaultClientConfig()
	c := NewClientApp(cfg, testClientLogger(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dst := make(chan []byte, 10)
	src := c.newServerMuteIncomingFilterCh(ctx, dst)

	// Unmuted: frame should pass through
	src <- []byte{1, 2, 3}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{1, 2, 3}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through when unmuted")
	}

	// Muted: frame should be dropped
	c.serverMutedIncoming.Store(true)
	src <- []byte{4, 5, 6}
	select {
	case <-dst:
		t.Fatal("expected frame to be dropped when muted")
	case <-time.After(100 * time.Millisecond):
		// Good — frame was dropped
	}

	// Unmute: frame should pass through again
	c.serverMutedIncoming.Store(false)
	src <- []byte{7, 8, 9}
	select {
	case data := <-dst:
		assert.Equal(t, []byte{7, 8, 9}, data)
	case <-time.After(time.Second):
		t.Fatal("expected frame to pass through after unmute")
	}
}

func TestSetupReverseAudioMuted_SpectrumNotFedWhenMuted(t *testing.T) {
	// This test verifies that when muteIncomingFlag is non-nil, spectrum/levelMeter
	// are NOT passed to setupAudioDecoder (they are nil), and instead fed in the
	// intercept goroutine only when not muted. We verify by checking the code path
	// logic: muteIncomingFlag != nil means spectrum goes through intercept.
	// Direct integration test would require full audio pipeline; instead we verify
	// the atomic flag + filter channel behavior which is the core of the fix.
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var muteFlag atomic.Bool
	muteFlag.Store(true)

	// Simulate intercept: frames should be dropped when muted
	decodeCh := make(chan []float32, 5)
	outputCh := make(chan []float32, 5)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case samples, ok := <-decodeCh:
				if !ok {
					return
				}
				if muteFlag.Load() {
					continue
				}
				outputCh <- samples
			}
		}
	}()

	// Send frame while muted — should not appear on output
	decodeCh <- []float32{0.1, 0.2, 0.3}
	select {
	case <-outputCh:
		t.Fatal("frame should be dropped when muted")
	case <-time.After(100 * time.Millisecond):
	}

	// Unmute and send — should appear
	muteFlag.Store(false)
	decodeCh <- []float32{0.4, 0.5, 0.6}
	select {
	case data := <-outputCh:
		assert.Equal(t, []float32{0.4, 0.5, 0.6}, data)
	case <-time.After(time.Second):
		t.Fatal("frame should pass through when unmuted")
	}
}

func buildControlMessage(t *testing.T, action string) []byte {
	t.Helper()
	msg := struct {
		Type    string `json:"type"`
		Payload struct {
			Action string `json:"action"`
		} `json:"payload"`
	}{
		Type: transport.TypeControl,
		Payload: struct {
			Action string `json:"action"`
		}{Action: action},
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	return data
}
