package app

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// TestPausePropagation_ServerFrameCounterStops asserts that Node.Pause,
// routed through ServerApp.SetPaused (the PauseController adapter), makes
// the server's outgoing frame counter (gated by newServerPauseFilterCh)
// stop advancing — and that Node.Resume makes it advance again.
//
// This exercises the full phase-6 path: Node.Pause → runner.(PauseController).
// SetPaused → s.serverPaused.Store → newServerPauseFilterCh drop branch.
// It does NOT touch the atomic directly and goes through the public
// Node.Pause / Node.Resume API per the acceptance criteria.
func TestPausePropagation_ServerFrameCounterStops(t *testing.T) {
	s := newMinimalServerApp(nil)

	// Wrap the ServerApp in a thin runner that satisfies only the
	// interfaces we need: Runner + PauseController (delegated to
	// s.SetPaused). This keeps the PauseController call path identical
	// (s.SetPaused) but avoids running server.Run() which would bind a
	// TCP listener and try to set up real WebRTC.
	wrapper := &pausableRunner{delegate: s, started: make(chan struct{})}

	node, err := echowarp.NewNode(
		echowarp.NodeConfig{Mode: echowarp.ModeServer, Port: 0},
		echowarp.WithRunnerFactory(pauseTestRunnerFactory(wrapper)),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, node.Start(ctx))
	t.Cleanup(func() { _ = node.Stop() })

	// Wait for Run to start and state to reach Streaming.
	select {
	case <-wrapper.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}
	require.Eventually(t, func() bool {
		return node.Status() == echowarp.StatusStreaming
	}, 2*time.Second, 10*time.Millisecond)

	// Set up newServerPauseFilterCh to observe the frame counter.
	// The filter drops frames when s.serverPaused is true and forwards
	// otherwise — consulted by the audio send pipeline in real runs.
	dst := make(chan []byte, 16)
	src := s.newServerPauseFilterCh(ctx, dst)

	// Producer goroutine: pushes small byte slices at 1ms cadence. The
	// filter returns dropped buffers to audio.PutOpusOutput, so we
	// allocate with cap <= 256 to stay under maxOpusOutputSize and let
	// the pool recycle them safely.
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				buf := make([]byte, 64, 256)
				select {
				case src <- buf:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Consumer goroutine: drains dst and counts surviving frames.
	var counter uint64
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-dst:
				if !ok {
					return
				}
				atomic.AddUint64(&counter, 1)
			}
		}
	}()

	// Phase 1: running — counter must advance.
	waitForAppCounterAdvance(t, &counter, 2*time.Second)

	// Phase 2: Pause via the public Node API (not directly on the
	// atomic). This routes through runner.(PauseController).SetPaused(true)
	// → pausableRunner.SetPaused → ServerApp.SetPaused → s.serverPaused.Store.
	require.NoError(t, node.Pause())
	// Give the producer a tick or two to observe the new flag.
	time.Sleep(10 * time.Millisecond)
	before := atomic.LoadUint64(&counter)
	time.Sleep(50 * time.Millisecond)
	after := atomic.LoadUint64(&counter)
	// The filter may let a single inflight frame through between Pause
	// and the next producer tick; allow a tiny slack.
	assert.LessOrEqual(t, after-before, uint64(2),
		"counter should not advance meaningfully while paused (before=%d after=%d)", before, after)

	// Phase 3: Resume via the public Node API.
	require.NoError(t, node.Resume())
	waitForAppCounterAdvanceFrom(t, &counter, after, 2*time.Second)

	cancel()
	<-producerDone
	<-consumerDone
}

// pausableRunner is the minimal runner needed for the test: it sits
// forever in Run(ctx) and forwards SetPaused to the embedded ServerApp
// adapter. Using a thin wrapper instead of running ServerApp.Run()
// directly keeps the test focused on pause propagation and avoids
// spinning up a TCP listener.
type pausableRunner struct {
	delegate *ServerApp
	started  chan struct{}
}

func (r *pausableRunner) Run(ctx context.Context) error {
	close(r.started)
	<-ctx.Done()
	return nil
}

func (r *pausableRunner) SetPaused(paused bool) error {
	return r.delegate.SetPaused(paused)
}

// pauseTestRunnerFactory returns a RunnerFactory that always returns the
// given pre-built runner. The echowarp.RunnerFactory signature is:
//
//	func(cfg NodeConfig, logger *slog.Logger, banMgr ban.BanManager,
//	     tlsConfig *tls.Config, rateLimiter *auth.IPRateLimiter) (Runner, error)
func pauseTestRunnerFactory(r echowarp.Runner) echowarp.RunnerFactory {
	return func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return r, nil
	}
}

// waitForAppCounterAdvance blocks until the counter advances past its
// current value, or the deadline fires (test-level helper, distinct from
// the one in pkg/echowarp/pause_counter_test.go).
func waitForAppCounterAdvance(t *testing.T, counter *uint64, deadline time.Duration) {
	t.Helper()
	start := atomic.LoadUint64(counter)
	waitForAppCounterAdvanceFrom(t, counter, start, deadline)
}

// waitForAppCounterAdvanceFrom blocks until the counter advances past
// `from`, or the deadline fires.
func waitForAppCounterAdvanceFrom(t *testing.T, counter *uint64, from uint64, deadline time.Duration) {
	t.Helper()
	stop := time.Now().Add(deadline)
	for time.Now().Before(stop) {
		if atomic.LoadUint64(counter) > from {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("counter did not advance past %d within %s (final=%d)", from, deadline, atomic.LoadUint64(counter))
}
