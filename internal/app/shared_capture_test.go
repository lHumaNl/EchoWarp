package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func testHubLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestSharedCaptureHub_FanOutToMultipleSubscribers — both subscribers receive
// the same PCM frames when the hub dispatches.
func TestSharedCaptureHub_FanOutToMultipleSubscribers(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	subA := h.Subscribe("A")
	defer subA.Close()
	subB := h.Subscribe("B")
	defer subB.Close()
	require.Equal(t, 2, h.SubscriberCount())

	frame := []float32{0.1, 0.2, 0.3}
	h.dispatch(frame)

	select {
	case got := <-subA.PCM:
		assert.Equal(t, frame, got)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subA did not receive frame")
	}
	select {
	case got := <-subB.PCM:
		assert.Equal(t, frame, got)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subB did not receive frame")
	}
}

// TestSharedCaptureHub_SlowSubscriberDoesNotBlock — a full subscriber queue
// drops its oldest queued frame rather than blocking the dispatch.
func TestSharedCaptureHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	slow := h.Subscribe("slow")
	defer slow.Close()

	// Fill slow's channel (cap=4).
	for i := 0; i < 4; i++ {
		h.dispatch([]float32{float32(i)})
	}

	// This dispatch should not block — slow's queue is full.
	done := make(chan struct{})
	go func() {
		h.dispatch([]float32{99})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("dispatch blocked on slow subscriber with full queue")
	}
}

func TestSharedCaptureHub_DropOldestKeepsFreshFrames(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	sub := h.Subscribe("slow")
	defer sub.Close()

	for i := 0; i < 5; i++ {
		h.dispatch([]float32{float32(i)})
	}
	assert.Equal(t, uint64(1), sub.DropCount())
	assert.Equal(t, 4, sub.QueueDepth())
	assert.Equal(t, 4, sub.QueueCapacity())

	for want := 1; want <= 4; want++ {
		select {
		case got := <-sub.PCM:
			assert.Equal(t, []float32{float32(want)}, got)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("missing queued frame %d", want)
		}
	}
}

func TestSharedCaptureHub_SlowSubscriberIsolation_FastSubscriberKeepsFreshFrames(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	slow := h.Subscribe("slow")
	defer slow.Close()
	fast := h.Subscribe("fast")
	defer fast.Close()

	for i := 0; i < 5; i++ {
		frame := []float32{float32(i)}
		h.dispatch(frame)
		assertReceivesFrame(t, fast.PCM, frame)
	}

	assert.Equal(t, uint64(1), slow.DropCount())
	assert.Equal(t, uint64(0), fast.DropCount())
	assert.Equal(t, 4, slow.QueueDepth())
	assert.Equal(t, 0, fast.QueueDepth())

	for want := 1; want <= 4; want++ {
		assertReceivesFrame(t, slow.PCM, []float32{float32(want)})
	}
	assert.Equal(t, 0, slow.QueueDepth())
}

func TestSharedCaptureHub_SubscriberDropLoggingIsThrottled(t *testing.T) {
	t.Parallel()

	logs := &lockedLogBuffer{}
	h := NewSharedCaptureHub(CapturePipelineConfig{}, slog.New(slog.NewJSONHandler(logs, nil)))
	sub := h.Subscribe("slow")
	defer sub.Close()
	fillSharedCaptureQueue(h)

	h.dispatch([]float32{99})
	records := decodeSlogJSONRecords(t, logs.Bytes())
	require.Len(t, records, 1)
	assertSharedCaptureDropLog(t, records[0], float64(1))

	for i := 0; i < 3; i++ {
		h.dispatch([]float32{float32(100 + i)})
	}
	assert.Len(t, decodeSlogJSONRecords(t, logs.Bytes()), 1)

	h.nextDropLogUnixNano.Store(time.Now().Add(-time.Second).UnixNano())
	h.dispatch([]float32{200})
	records = decodeSlogJSONRecords(t, logs.Bytes())
	require.Len(t, records, 2)
	assertSharedCaptureDropLog(t, records[1], float64(4))
}

func TestSharedCaptureHub_RecordingTapOncePerFrameWithSubscribers(t *testing.T) {
	t.Parallel()
	var tapCalls atomic.Int32
	h := NewSharedCaptureHub(CapturePipelineConfig{
		RecordingTap: func([]float32) { tapCalls.Add(1) },
	}, testHubLogger())

	subA := h.Subscribe("A")
	defer subA.Close()
	subB := h.Subscribe("B")
	defer subB.Close()

	frame := h.applyProcessing(context.Background(), []float32{0.25, -0.25})
	h.dispatch(frame)

	assertReceivesFrame(t, subA.PCM, frame)
	assertReceivesFrame(t, subB.PCM, frame)
	assert.Equal(t, int32(1), tapCalls.Load())
}

func TestSharedCaptureHub_RunKeepsRunningAfterStartSuccess(t *testing.T) {
	t.Parallel()
	h := newFakeCaptureHub(&fakeSharedCapturer{})
	ctx, cancel := context.WithCancel(context.Background())
	runDone := runSharedCaptureHub(ctx, h)

	require.NoError(t, h.WaitReady(ctx))
	assertRunStillActive(t, runDone)

	cancel()
	require.ErrorIs(t, <-runDone, context.Canceled)
}

func TestSharedCaptureHub_RunDeliversPCMAfterStartSuccess(t *testing.T) {
	t.Parallel()
	frame := []float32{0.125, -0.25}
	fake := &fakeSharedCapturer{start: sendFrameOnStart(frame)}
	h := newFakeCaptureHub(fake)
	sub := h.Subscribe("client-1")
	ctx, cancel := context.WithCancel(context.Background())
	runDone := runSharedCaptureHub(ctx, h)

	require.NoError(t, h.WaitReady(ctx))
	assertReceivesFrame(t, sub.PCM, frame)
	assertSubscriptionOpen(t, sub.PCM)

	cancel()
	require.ErrorIs(t, <-runDone, context.Canceled)
}

func TestSharedCaptureHub_RunClosesSubscriptionOnContextCancel(t *testing.T) {
	t.Parallel()
	h := newFakeCaptureHub(&fakeSharedCapturer{})
	sub := h.Subscribe("client-1")
	ctx, cancel := context.WithCancel(context.Background())
	runDone := runSharedCaptureHub(ctx, h)

	require.NoError(t, h.WaitReady(ctx))
	cancel()
	require.ErrorIs(t, <-runDone, context.Canceled)
	assertSubscriptionClosed(t, sub.PCM)
}

func TestSharedCaptureHub_RunClosesSubscriptionOnFatalPCMClose(t *testing.T) {
	t.Parallel()
	fake := &fakeSharedCapturer{start: closePCMOnStart}
	h := newFakeCaptureHub(fake)
	sub := h.Subscribe("client-1")
	ctx := context.Background()
	runDone := runSharedCaptureHub(ctx, h)

	err := <-runDone
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PCM channel closed")
	assertSubscriptionClosed(t, sub.PCM)
}

func TestSharedCaptureHub_RunPropagatesStartupError(t *testing.T) {
	t.Parallel()
	startErr := errors.New("start failed")
	fake := &fakeSharedCapturer{start: failCaptureStart(startErr)}
	h := newFakeCaptureHub(fake)
	sub := h.Subscribe("client-1")

	err := h.Run(context.Background())
	require.ErrorIs(t, err, startErr)
	assertSubscriptionClosed(t, sub.PCM)
}

func TestSharedCaptureHub_FirstSubscriberCloseKeepsHubUsable(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	first := h.Subscribe("first")
	second := h.Subscribe("second")
	defer second.Close()

	first.Close()
	replacement := h.Subscribe("replacement")
	defer replacement.Close()
	h.dispatch([]float32{42})

	assertReceivesFrame(t, second.PCM, []float32{42})
	assertReceivesFrame(t, replacement.PCM, []float32{42})
	assert.Equal(t, 2, h.SubscriberCount())
}

func assertReceivesFrame(t *testing.T, ch <-chan []float32, want []float32) {
	t.Helper()
	select {
	case got := <-ch:
		assert.Equal(t, want, got)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber did not receive frame")
	}
}

func fillSharedCaptureQueue(h *SharedCaptureHub) {
	for i := 0; i < 4; i++ {
		h.dispatch([]float32{float32(i)})
	}
}

func assertSharedCaptureDropLog(t *testing.T, record map[string]any, drops float64) {
	t.Helper()
	assert.Equal(t, "Shared capture subscriber drops", record["msg"])
	assert.Equal(t, "slow", record["clientID"])
	assert.Equal(t, drops, record["subscriber_drops"])
	assert.Equal(t, float64(4), record["sub_queue_cap"])
}

type fakeSharedCapturer struct {
	start  func(context.Context, uint32, chan<- []float32) error
	closed atomic.Bool
}

func (f *fakeSharedCapturer) Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error {
	if f.start == nil {
		return nil
	}
	return f.start(ctx, deviceID, outCh)
}

func (f *fakeSharedCapturer) Close() error {
	f.closed.Store(true)
	return nil
}

func newFakeCaptureHub(c *fakeSharedCapturer) *SharedCaptureHub {
	h := NewSharedCaptureHub(CapturePipelineConfig{SampleRate: 48000, Channels: 1}, testHubLogger())
	h.newCapturer = func(uint32, uint32, ...audio.CapturerOption) (sharedCapturer, error) {
		return c, nil
	}
	return h
}

func runSharedCaptureHub(ctx context.Context, h *SharedCaptureHub) <-chan error {
	runDone := make(chan error, 1)
	go func() { runDone <- h.Run(ctx) }()
	return runDone
}

func assertRunStillActive(t *testing.T, runDone <-chan error) {
	t.Helper()
	select {
	case err := <-runDone:
		t.Fatalf("hub exited after successful start: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func sendFrameOnStart(frame []float32) func(context.Context, uint32, chan<- []float32) error {
	return func(ctx context.Context, _ uint32, outCh chan<- []float32) error {
		go func() {
			select {
			case outCh <- frame:
			case <-ctx.Done():
			}
		}()
		return nil
	}
}

func closePCMOnStart(_ context.Context, _ uint32, outCh chan<- []float32) error {
	close(outCh)
	return nil
}

func failCaptureStart(err error) func(context.Context, uint32, chan<- []float32) error {
	return func(context.Context, uint32, chan<- []float32) error {
		return err
	}
}

func assertSubscriptionOpen(t *testing.T, ch <-chan []float32) {
	t.Helper()
	select {
	case _, ok := <-ch:
		require.True(t, ok, "subscription closed immediately after startup")
	default:
	}
}

func assertSubscriptionClosed(t *testing.T, ch <-chan []float32) {
	t.Helper()
	select {
	case _, ok := <-ch:
		require.False(t, ok, "subscription channel should be closed")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("subscription channel did not close")
	}
}

// TestSharedCaptureHub_SubscribeUnsubscribe — dynamic join/leave doesn't leak
// goroutines or panic.
func TestSharedCaptureHub_SubscribeUnsubscribe(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	for i := 0; i < 5; i++ {
		sub := h.Subscribe("client")
		assert.Equal(t, 1, h.SubscriberCount(), "re-subscribing same ID replaces prior")
		sub.Close()
		assert.Equal(t, 0, h.SubscriberCount())
	}

	// Double-Close is idempotent.
	sub := h.Subscribe("x")
	sub.Close()
	sub.Close()
	assert.Equal(t, 0, h.SubscriberCount())
}

// TestSharedCaptureHub_ReplaceSubscriber — re-subscribing with same clientID
// closes the prior channel so its consumer exits cleanly.
func TestSharedCaptureHub_ReplaceSubscriber(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	subA := h.Subscribe("dup")
	done := make(chan struct{})
	go func() {
		for range subA.PCM {
		}
		close(done)
	}()

	subB := h.Subscribe("dup")
	// subA's channel should have been closed on re-subscribe.
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("prior subscription channel was not closed on re-subscribe")
	}
	subB.Close()
}

// TestApplyPerClientGain — scales samples and leaves hub slice untouched.
func TestApplyPerClientGain(t *testing.T) {
	t.Parallel()
	orig := []float32{0.2, -0.2, 0.4, -0.4}

	// nil gain → same slice
	out := applyPerClientGain(orig, nil)
	assert.Equal(t, &orig[0], &out[0], "nil gain must return same slice")

	// gain == 1.0 → same slice (no copy)
	g := NewDeviceGainControl(1.0)
	out = applyPerClientGain(orig, g)
	assert.Equal(t, &orig[0], &out[0], "gain=1.0 must return same slice (no copy)")

	// gain == 0.5 → new slice, scaled
	g.SetGain(0.5)
	out = applyPerClientGain(orig, g)
	assert.NotEqual(t, &orig[0], &out[0], "gain!=1 must allocate a copy")
	assert.Equal(t, []float32{0.2, -0.2, 0.4, -0.4}, orig, "input must not be mutated")
	for i, v := range out {
		assert.InDelta(t, orig[i]*0.5, v, 1e-6)
	}

	// gain == 0 → zeros
	g.SetGain(0)
	out = applyPerClientGain(orig, g)
	for _, v := range out {
		assert.Equal(t, float32(0), v)
	}
}

// TestSharedCaptureHub_ConcurrentSubscribeAndDispatch — subscribe/unsubscribe
// and dispatch concurrently without panics or data races.
func TestSharedCaptureHub_ConcurrentSubscribeAndDispatch(t *testing.T) {
	t.Parallel()
	h := NewSharedCaptureHub(CapturePipelineConfig{}, testHubLogger())

	var stop atomic.Bool
	var wg sync.WaitGroup

	// Dispatcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		frame := []float32{1, 2, 3}
		for !stop.Load() {
			h.dispatch(frame)
		}
	}()

	// Subscribe/unsubscribe churn
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			sub := h.Subscribe("churn")
			// drain a bit
			select {
			case <-sub.PCM:
			default:
			}
			sub.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}
