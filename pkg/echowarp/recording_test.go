package echowarp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// recordingCtrlFakeRunner implements Runner + RecordingController so
// node-level unit tests can assert the Start/Stop/Status delivery path.
// Separate from the other fake runners in this package so the "no
// RecordingController" fallback path exercised by plainBanRunner stays
// distinct — a fake that also implements BanManager/ChatSender would
// accidentally satisfy multiple optional interfaces and confuse
// intent-specific tests.
type recordingCtrlFakeRunner struct {
	started chan struct{}
	once    sync.Once

	mu           sync.Mutex
	modeCaptured RecordingMode
	startCalls   int
	stopCalls    int

	startErr error
	stopErr  error
	stopRes  RecordingResult

	status RecordingStatus
}

func newRecordingCtrlFakeRunner() *recordingCtrlFakeRunner {
	return &recordingCtrlFakeRunner{started: make(chan struct{})}
}

func (r *recordingCtrlFakeRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *recordingCtrlFakeRunner) StartRecording(mode RecordingMode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startCalls++
	if r.startErr != nil {
		return r.startErr
	}
	r.modeCaptured = mode
	r.status = RecordingStatus{
		Active:    true,
		Mode:      mode,
		StartedAt: time.Now(),
	}
	return nil
}

func (r *recordingCtrlFakeRunner) StopRecording() (RecordingResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalls++
	if r.stopErr != nil {
		return RecordingResult{Files: []string{}}, r.stopErr
	}
	r.status = RecordingStatus{}
	return r.stopRes, nil
}

func (r *recordingCtrlFakeRunner) RecordingStatus() RecordingStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

var _ Runner = (*recordingCtrlFakeRunner)(nil)
var _ RecordingController = (*recordingCtrlFakeRunner)(nil)

// plainRecRunner implements Runner only — used to prove the
// RecordingController type assertion miss path.
type plainRecRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainRecRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func TestIsValidRecordingMode(t *testing.T) {
	assert.True(t, IsValidRecordingMode("mix"))
	assert.True(t, IsValidRecordingMode("tracks"))
	assert.True(t, IsValidRecordingMode("both"))
	assert.False(t, IsValidRecordingMode(""))
	assert.False(t, IsValidRecordingMode("bogus"))
	assert.False(t, IsValidRecordingMode("MIX")) // case-sensitive by design
}

func TestNodeStartRecording_DeliversMixToRunner(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.StartRecording("mix"))
	runner.mu.Lock()
	assert.Equal(t, 1, runner.startCalls)
	assert.Equal(t, RecordingModeMix, runner.modeCaptured)
	runner.mu.Unlock()
}

func TestNodeStartRecording_DeliversTracksToRunner(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.StartRecording("tracks"))
	runner.mu.Lock()
	assert.Equal(t, RecordingModeTracks, runner.modeCaptured)
	runner.mu.Unlock()
}

func TestNodeStartRecording_DeliversBothToRunner(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.StartRecording("both"))
	runner.mu.Lock()
	assert.Equal(t, RecordingModeBoth, runner.modeCaptured)
	runner.mu.Unlock()
}

func TestNodeStartRecording_InvalidMode_ErrConfigValidation(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.StartRecording("bogus")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr), "expected EchoWarpError, got %T", err)
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
	// Runner must not have been touched.
	runner.mu.Lock()
	assert.Zero(t, runner.startCalls)
	runner.mu.Unlock()
}

func TestNodeStartRecording_EmptyMode_ErrConfigValidation(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.StartRecording("")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
}

func TestNodeStartRecording_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	err = node.StartRecording("mix")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeStartRecording_RunnerWithoutController_ErrInternalState(t *testing.T) {
	runner := &plainRecRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.StartRecording("mix")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeStartRecording_PropagatesRunnerError(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	runner.startErr = errors.New("disk full")
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.StartRecording("mix")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestNodeStopRecording_DeliversToRunner(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	runner.stopRes = RecordingResult{
		Duration:   2 * time.Second,
		DurationMs: 2000,
		Size:       1024,
		Files:      []string{"/tmp/mix.wav"},
	}
	n := startNodeWithRunner(t, runner, runner.started)

	result, err := n.StopRecording()
	require.NoError(t, err)
	assert.Equal(t, int64(2000), result.DurationMs)
	assert.Equal(t, int64(1024), result.Size)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "/tmp/mix.wav", result.Files[0])

	runner.mu.Lock()
	assert.Equal(t, 1, runner.stopCalls)
	runner.mu.Unlock()
}

func TestNodeStopRecording_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	_, err = node.StopRecording()
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeStopRecording_RunnerWithoutController_ErrInternalState(t *testing.T) {
	runner := &plainRecRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	_, err := n.StopRecording()
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeRecordingStatus_NotRunning_Inactive(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	status := node.RecordingStatus()
	assert.False(t, status.Active)
	assert.Empty(t, status.Mode)
}

func TestNodeRecordingStatus_RunnerWithoutController_Inactive(t *testing.T) {
	runner := &plainRecRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	status := n.RecordingStatus()
	assert.False(t, status.Active)
}

func TestNodeRecordingStatus_DeliveryToRunner(t *testing.T) {
	runner := newRecordingCtrlFakeRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	// Before any start — status is inactive.
	status := n.RecordingStatus()
	assert.False(t, status.Active)

	// Drive into active state.
	require.NoError(t, n.StartRecording("mix"))
	status = n.RecordingStatus()
	assert.True(t, status.Active)
	assert.Equal(t, RecordingModeMix, status.Mode)

	// And back to inactive after Stop.
	_, err := n.StopRecording()
	require.NoError(t, err)
	status = n.RecordingStatus()
	assert.False(t, status.Active)
}

// Sanity check — ensure none of the other test fakes in this package
// accidentally gained a RecordingController implementation. Compile-
// time assertions: if any of these compile, we have a regression
// (e.g. noopTestRunner sprouting a StartRecording method). The test
// is about what the fakes do NOT satisfy.
func TestExistingFakesDoNotImplementRecordingController(t *testing.T) {
	var _ Runner = (*plainBanRunner)(nil)
	// plainBanRunner must not satisfy RecordingController.
	var ctrl any = (*plainBanRunner)(nil)
	_, ok := ctrl.(RecordingController)
	assert.False(t, ok, "plainBanRunner must not implement RecordingController")

	var ctrl2 any = (*banMgrFakeRunner)(nil)
	_, ok = ctrl2.(RecordingController)
	assert.False(t, ok, "banMgrFakeRunner must not implement RecordingController")

	var ctrl3 any = (*chatSenderMockRunner)(nil)
	_, ok = ctrl3.(RecordingController)
	assert.False(t, ok, "chatSenderMockRunner must not implement RecordingController")
}
