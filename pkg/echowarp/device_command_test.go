package echowarp

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// deviceCmdMockRunner implements Runner + DeviceCommandReceiver and records
// every received command on a buffered channel so tests can assert plumbing.
type deviceCmdMockRunner struct {
	started  chan struct{}
	commands chan DeviceCommand
	handle   func(DeviceCommand) error
	once     sync.Once
}

func newDeviceCmdMockRunner() *deviceCmdMockRunner {
	return &deviceCmdMockRunner{
		started:  make(chan struct{}),
		commands: make(chan DeviceCommand, 8),
	}
}

func (m *deviceCmdMockRunner) Run(ctx context.Context) error {
	m.once.Do(func() { close(m.started) })
	<-ctx.Done()
	return nil
}

func (m *deviceCmdMockRunner) HandleDeviceCommand(cmd DeviceCommand) error {
	if m.handle != nil {
		return m.handle(cmd)
	}
	m.commands <- cmd
	return nil
}

// plainMockRunner implements only Runner (no DeviceCommandReceiver).
type plainMockRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainMockRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func startNodeWithRunner(t *testing.T, runner Runner, started <-chan struct{}) *Node {
	t.Helper()
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	factory := func(_ NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (Runner, error) {
		return runner, nil
	}
	n, err := NewNode(cfg, WithRunnerFactory(factory))
	require.NoError(t, err)
	go func() { _ = n.Start(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}
	t.Cleanup(func() { _ = n.Stop() })
	return n
}

func TestNodeSetDeviceMute_DeliversToRunner(t *testing.T) {
	runner := newDeviceCmdMockRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetDeviceMute(5, true))
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, DeviceActionSetMute, cmd.Action)
		assert.Equal(t, 5, cmd.DeviceID)
		assert.True(t, cmd.Muted)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive mute command")
	}
}

func TestNodeSetDeviceVolume_DeliversToRunner(t *testing.T) {
	runner := newDeviceCmdMockRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetDeviceVolume(2, 0.75))
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, DeviceActionSetVolume, cmd.Action)
		assert.Equal(t, 2, cmd.DeviceID)
		assert.InDelta(t, 0.75, cmd.Volume, 1e-9)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive volume command")
	}
}

func TestNodeSetDeviceMute_InvalidID(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	err = n.SetDeviceMute(-1, true)
	require.Error(t, err)
}

func TestNodeSetDeviceVolume_OutOfRange(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.SetDeviceVolume(1, -0.01))
	require.Error(t, n.SetDeviceVolume(1, 1.51))
}

func TestNodeSetDeviceMute_NotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	err = n.SetDeviceMute(1, true)
	require.Error(t, err, "must fail when node has not been started")
}

func TestNodeSetDeviceMute_RunnerWithoutReceiver(t *testing.T) {
	runner := &plainMockRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)
	err := n.SetDeviceMute(1, true)
	require.Error(t, err, "runner that does not implement DeviceCommandReceiver must fail")
}
