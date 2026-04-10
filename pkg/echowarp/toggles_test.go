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

// muteCtrlFakeRunner implements Runner + MuteController so node-level
// unit tests can assert the SetMuted delivery path without spinning up
// a real ClientApp. Kept separate from the device/recording/ban fakes
// so the "runner does not implement MuteController" fallback path can
// be exercised via a runner that deliberately lacks the interface.
type muteCtrlFakeRunner struct {
	started chan struct{}
	once    sync.Once

	mu          sync.Mutex
	calls       int
	lastMuted   bool
	injectedErr error
	stateMuted  atomic.Bool
}

func newMuteCtrlFakeRunner() *muteCtrlFakeRunner {
	return &muteCtrlFakeRunner{started: make(chan struct{})}
}

func (r *muteCtrlFakeRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *muteCtrlFakeRunner) SetMuted(muted bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.injectedErr != nil {
		return r.injectedErr
	}
	r.lastMuted = muted
	r.stateMuted.Store(muted)
	return nil
}

var _ Runner = (*muteCtrlFakeRunner)(nil)
var _ MuteController = (*muteCtrlFakeRunner)(nil)

// discoveryPubFakeRunner implements Runner + DiscoveryPublisher.
type discoveryPubFakeRunner struct {
	started chan struct{}
	once    sync.Once

	mu          sync.Mutex
	calls       int
	lastEnabled bool
	injectedErr error
	publishing  atomic.Bool
}

func newDiscoveryPubFakeRunner() *discoveryPubFakeRunner {
	return &discoveryPubFakeRunner{started: make(chan struct{})}
}

func (r *discoveryPubFakeRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *discoveryPubFakeRunner) SetDiscoveryPublish(enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.injectedErr != nil {
		return r.injectedErr
	}
	r.lastEnabled = enabled
	r.publishing.Store(enabled)
	return nil
}

var _ Runner = (*discoveryPubFakeRunner)(nil)
var _ DiscoveryPublisher = (*discoveryPubFakeRunner)(nil)

// plainToggleRunner implements Runner only — used to prove both
// type-assertion miss paths.
type plainToggleRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainToggleRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

// ---------------------------------------------------------------------------
// Node.SetMuted — delivery / error paths
// ---------------------------------------------------------------------------

func TestNodeSetMuted_DeliversTrueToRunner(t *testing.T) {
	runner := newMuteCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetMuted(true))
	runner.mu.Lock()
	assert.Equal(t, 1, runner.calls)
	assert.True(t, runner.lastMuted)
	runner.mu.Unlock()
	assert.True(t, runner.stateMuted.Load())
}

func TestNodeSetMuted_DeliversFalseToRunner(t *testing.T) {
	runner := newMuteCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetMuted(true))
	require.NoError(t, n.SetMuted(false))
	runner.mu.Lock()
	assert.Equal(t, 2, runner.calls)
	assert.False(t, runner.lastMuted)
	runner.mu.Unlock()
	assert.False(t, runner.stateMuted.Load())
}

func TestNodeSetMuted_Idempotent(t *testing.T) {
	runner := newMuteCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetMuted(true))
	require.NoError(t, n.SetMuted(true))
	runner.mu.Lock()
	// The Node wrapper does not short-circuit on repeated calls — the
	// runner sees both. Idempotence is a runner-side contract.
	assert.Equal(t, 2, runner.calls)
	runner.mu.Unlock()
}

func TestNodeSetMuted_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeClient, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	err = node.SetMuted(true)
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeSetMuted_RunnerWithoutController_ErrInternalState(t *testing.T) {
	runner := &plainToggleRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SetMuted(true)
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeSetMuted_PropagatesRunnerError(t *testing.T) {
	runner := newMuteCtrlFakeRunner()
	runner.injectedErr = errors.New("hardware failure")
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SetMuted(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hardware failure")
}

// ---------------------------------------------------------------------------
// Node.SetDiscoveryPublish — delivery / error paths
// ---------------------------------------------------------------------------

func TestNodeSetDiscoveryPublish_DeliversTrueToRunner(t *testing.T) {
	runner := newDiscoveryPubFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetDiscoveryPublish(true))
	runner.mu.Lock()
	assert.Equal(t, 1, runner.calls)
	assert.True(t, runner.lastEnabled)
	runner.mu.Unlock()
	assert.True(t, runner.publishing.Load())
}

func TestNodeSetDiscoveryPublish_DeliversFalseToRunner(t *testing.T) {
	runner := newDiscoveryPubFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetDiscoveryPublish(true))
	require.NoError(t, n.SetDiscoveryPublish(false))
	runner.mu.Lock()
	assert.Equal(t, 2, runner.calls)
	assert.False(t, runner.lastEnabled)
	runner.mu.Unlock()
	assert.False(t, runner.publishing.Load())
}

func TestNodeSetDiscoveryPublish_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	err = node.SetDiscoveryPublish(true)
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeSetDiscoveryPublish_RunnerWithoutPublisher_ErrInternalState(t *testing.T) {
	runner := &plainToggleRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SetDiscoveryPublish(true)
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeSetDiscoveryPublish_PropagatesRunnerError(t *testing.T) {
	runner := newDiscoveryPubFakeRunner()
	runner.injectedErr = errors.New("port already bound")
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SetDiscoveryPublish(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port already bound")
}
