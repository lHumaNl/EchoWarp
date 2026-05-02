package echowarp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingPauseRunner implements Runner + PauseController and runs a
// background "audio send" goroutine that increments an atomic counter on
// every tick, gated by a paused flag. Tests use it to observe Pause /
// Resume propagation in a real frame-counter-style setup without needing
// any actual audio hardware.
type countingPauseRunner struct {
	started chan struct{}
	once    sync.Once

	paused  atomic.Bool
	counter uint64 // incremented on every unpaused tick
}

func newCountingPauseRunner() *countingPauseRunner {
	return &countingPauseRunner{started: make(chan struct{})}
}

func (r *countingPauseRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	// Use a small tick so tests stay fast. Frame budget: one "frame" per
	// millisecond — enough resolution for a ~50ms observation window.
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if r.paused.Load() {
				continue
			}
			atomic.AddUint64(&r.counter, 1)
		}
	}
}

func (r *countingPauseRunner) SetPaused(paused bool) error {
	r.paused.Store(paused)
	return nil
}

var _ Runner = (*countingPauseRunner)(nil)
var _ PauseController = (*countingPauseRunner)(nil)

// waitForCounterAdvance blocks until the counter advances past its current
// value or the deadline fires.
func waitForCounterAdvance(t *testing.T, counter *uint64) {
	t.Helper()
	start := atomic.LoadUint64(counter)
	waitForCounterAdvanceFrom(t, counter, start)
}

// waitForCounterAdvanceFrom blocks until the counter advances past `from`
// or the deadline fires (2s).
func waitForCounterAdvanceFrom(t *testing.T, counter *uint64, from uint64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadUint64(counter) > from {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("counter did not advance past %d within 2s (final=%d)", from, atomic.LoadUint64(counter))
}

// waitUntilCounterStable waits until the counter has not advanced for at
// least 20ms, or until the 500ms deadline fires. Used to assert that a
// pause has taken effect.
func waitUntilCounterStable(t *testing.T, counter *uint64) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	last := atomic.LoadUint64(counter)
	stableSince := time.Now()
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		cur := atomic.LoadUint64(counter)
		if cur != last {
			last = cur
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= 20*time.Millisecond {
			return
		}
	}
	// Not strictly fatal — the caller may still assert the counter did
	// not advance beyond a small inflight allowance. Just return.
}
