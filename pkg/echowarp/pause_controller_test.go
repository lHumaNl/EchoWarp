package echowarp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// pauseCtrlFakeRunner implements Runner + PauseController and records every
// SetPaused call so tests can assert the delivery path from Node.Pause /
// Node.Resume into the runner.
type pauseCtrlFakeRunner struct {
	started chan struct{}
	once    sync.Once

	mu        sync.Mutex
	calls     []bool
	pauseErr  error // returned from SetPaused(true)
	resumeErr error // returned from SetPaused(false)
}

func newPauseCtrlFakeRunner() *pauseCtrlFakeRunner {
	return &pauseCtrlFakeRunner{started: make(chan struct{})}
}

func (r *pauseCtrlFakeRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *pauseCtrlFakeRunner) SetPaused(paused bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, paused)
	if paused && r.pauseErr != nil {
		return r.pauseErr
	}
	if !paused && r.resumeErr != nil {
		return r.resumeErr
	}
	return nil
}

func (r *pauseCtrlFakeRunner) CallsSnapshot() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]bool, len(r.calls))
	copy(out, r.calls)
	return out
}

var _ Runner = (*pauseCtrlFakeRunner)(nil)
var _ PauseController = (*pauseCtrlFakeRunner)(nil)

// TestNodePause_DeliversToRunner asserts that Node.Pause forwards into the
// runner's SetPaused(true) when the runner implements PauseController, and
// that Node.Resume forwards into SetPaused(false).
func TestNodePause_DeliversToRunner(t *testing.T) {
	runner := newPauseCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.Pause())
	assert.Equal(t, StatusPaused, n.Status())
	assert.True(t, n.IsPaused())

	require.NoError(t, n.Resume())
	assert.Equal(t, StatusStreaming, n.Status())
	assert.False(t, n.IsPaused())

	calls := runner.CallsSnapshot()
	assert.Equal(t, []bool{true, false}, calls)
}

// TestNodePause_RunnerWithoutPauseController asserts back-compat: when the
// runner does not implement PauseController, Node.Pause/Resume still
// succeed (state-only transition) — matching pre-phase-6 tests like
// TestNode_Pause_Success which use a runner without SetPaused.
func TestNodePause_RunnerWithoutPauseController(t *testing.T) {
	runner := &plainMockRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.Pause())
	assert.Equal(t, StatusPaused, n.Status())
	assert.True(t, n.IsPaused())

	require.NoError(t, n.Resume())
	assert.Equal(t, StatusStreaming, n.Status())
	assert.False(t, n.IsPaused())
}

// TestNodePause_RunnerErrorRollsBackState asserts that when the runner's
// SetPaused(true) returns an error, Node.Pause rolls the state machine
// back to Streaming and clears the paused atomic so the node never lands
// in a half-paused state.
func TestNodePause_RunnerErrorRollsBackState(t *testing.T) {
	runner := newPauseCtrlFakeRunner()
	runner.pauseErr = errors.New("mixer refused pause")
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.Pause()
	require.Error(t, err)
	var ew *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ew), "expected EchoWarpError")
	assert.Equal(t, ewerrors.ErrInternalState, ew.Code)

	assert.Equal(t, StatusStreaming, n.Status(), "state must be rolled back")
	assert.False(t, n.IsPaused(), "paused atomic must be rolled back")
}

// TestNodeResume_RunnerErrorRollsBackState asserts that when the runner's
// SetPaused(false) returns an error, Node.Resume rolls the state machine
// back to Paused so the node never lands in a half-streaming state.
func TestNodeResume_RunnerErrorRollsBackState(t *testing.T) {
	runner := newPauseCtrlFakeRunner()
	runner.resumeErr = errors.New("mixer refused resume")
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.Pause())

	err := n.Resume()
	require.Error(t, err)
	var ew *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ew), "expected EchoWarpError")
	assert.Equal(t, ewerrors.ErrInternalState, ew.Code)

	assert.Equal(t, StatusPaused, n.Status(), "state must be rolled back")
	assert.True(t, n.IsPaused(), "paused atomic must be rolled back")
}

// TestFrameCounter_StopsAfterPause asserts that a frame producer driven by
// runner.SetPaused (via Node.Pause) actually stops advancing an outgoing
// frame counter, and that Node.Resume makes it advance again. Exercises
// the public Node.Pause/Resume path — not a direct atomic write.
func TestFrameCounter_StopsAfterPause(t *testing.T) {
	runner := newCountingPauseRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	// Give the producer a moment to advance.
	waitForCounterAdvance(t, &runner.counter)

	require.NoError(t, n.Pause())
	// After Pause, the counter must stabilize.
	before := atomic.LoadUint64(&runner.counter)
	waitUntilCounterStable(t, &runner.counter)
	stable := atomic.LoadUint64(&runner.counter)
	// Allow at most a single inflight increment between Pause and the
	// producer's check (the producer reads the flag per-tick, not under
	// the Pause lock) — but the counter must stop advancing beyond that.
	assert.LessOrEqual(t, stable-before, uint64(1), "counter should not advance while paused")

	require.NoError(t, n.Resume())
	// After Resume, the counter must start advancing again.
	waitForCounterAdvanceFrom(t, &runner.counter, stable)
}

// TestExistingFakesDoNotImplementPauseController guards against accidental
// regressions: if any existing fake runner in this package gains a
// SetPaused method, the runner-without-controller back-compat path tested
// by TestNodePause_RunnerWithoutPauseController silently stops exercising
// the no-interface branch. The assertions below are about what the fakes
// do NOT satisfy.
func TestExistingFakesDoNotImplementPauseController(t *testing.T) {
	var _ Runner = (*plainMockRunner)(nil)
	var ctrl any = (*plainMockRunner)(nil)
	_, ok := ctrl.(PauseController)
	assert.False(t, ok, "plainMockRunner must not implement PauseController")

	var ctrl2 any = (*plainRecRunner)(nil)
	_, ok = ctrl2.(PauseController)
	assert.False(t, ok, "plainRecRunner must not implement PauseController")

	var ctrl3 any = (*plainBanRunner)(nil)
	_, ok = ctrl3.(PauseController)
	assert.False(t, ok, "plainBanRunner must not implement PauseController")

	var ctrl4 any = (*deviceCmdMockRunner)(nil)
	_, ok = ctrl4.(PauseController)
	assert.False(t, ok, "deviceCmdMockRunner must not implement PauseController")

	var ctrl5 any = (*mockRunner)(nil)
	_, ok = ctrl5.(PauseController)
	assert.False(t, ok, "mockRunner must not implement PauseController")

	var ctrl6 any = (*recordingCtrlFakeRunner)(nil)
	_, ok = ctrl6.(PauseController)
	assert.False(t, ok, "recordingCtrlFakeRunner must not implement PauseController")
}
