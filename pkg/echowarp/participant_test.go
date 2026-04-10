package echowarp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// participantCmdMockRunner implements Runner + ParticipantLister +
// ParticipantCommandReceiver so Node-level unit tests can assert the command
// delivery path end-to-end.
type participantCmdMockRunner struct {
	started  chan struct{}
	commands chan ParticipantCommand
	once     sync.Once

	mu       sync.Mutex
	snapshot []ParticipantInfo
}

func newParticipantCmdMockRunner(snapshot []ParticipantInfo) *participantCmdMockRunner {
	return &participantCmdMockRunner{
		started:  make(chan struct{}),
		commands: make(chan ParticipantCommand, 8),
		snapshot: snapshot,
	}
}

func (m *participantCmdMockRunner) Run(ctx context.Context) error {
	m.once.Do(func() { close(m.started) })
	<-ctx.Done()
	return nil
}

func (m *participantCmdMockRunner) Participants() []ParticipantInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ParticipantInfo, len(m.snapshot))
	copy(out, m.snapshot)
	return out
}

func (m *participantCmdMockRunner) HandleParticipantCommand(cmd ParticipantCommand) error {
	m.commands <- cmd
	return nil
}

func TestNodeMuteParticipant_DeliversToRunner(t *testing.T) {
	runner := newParticipantCmdMockRunner(nil)
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.MuteParticipant("alice", true))
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, ParticipantActionMute, cmd.Action)
		assert.Equal(t, "alice", cmd.ID)
		assert.True(t, cmd.Muted)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive mute command")
	}
}

func TestNodeKickParticipant_DeliversToRunner(t *testing.T) {
	runner := newParticipantCmdMockRunner(nil)
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.KickParticipant("bob"))
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, ParticipantActionKick, cmd.Action)
		assert.Equal(t, "bob", cmd.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive kick command")
	}
}

func TestNodeSetParticipantVolume_DeliversToRunner(t *testing.T) {
	runner := newParticipantCmdMockRunner(nil)
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SetParticipantVolume("carol", 0.75))
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, ParticipantActionSetVolume, cmd.Action)
		assert.Equal(t, "carol", cmd.ID)
		assert.InDelta(t, 0.75, cmd.Volume, 1e-9)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive volume command")
	}
}

func TestNodeParticipants_ReturnsSnapshot(t *testing.T) {
	snapshot := []ParticipantInfo{
		{ID: "alice", Nickname: "Alice", Muted: false, Volume: 1.0},
		{ID: "bob", Nickname: "Bob", Muted: true, Volume: 0.5},
	}
	runner := newParticipantCmdMockRunner(snapshot)
	n := startNodeWithRunner(t, runner, runner.started)

	got := n.Participants()
	require.Len(t, got, 2)
	assert.Equal(t, "alice", got[0].ID)
	assert.Equal(t, "Alice", got[0].Nickname)
	assert.InDelta(t, 1.0, got[0].Volume, 1e-9)
	assert.Equal(t, "bob", got[1].ID)
	assert.True(t, got[1].Muted)
}

func TestNodeMuteParticipant_InvalidID(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.MuteParticipant("", true), "empty id must be rejected")
}

func TestNodeKickParticipant_InvalidID(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.KickParticipant(""), "empty id must be rejected")
}

func TestNodeSetParticipantVolume_InvalidID(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.SetParticipantVolume("", 0.5))
}

func TestNodeSetParticipantVolume_OutOfRange(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.SetParticipantVolume("alice", -0.01))
	require.Error(t, n.SetParticipantVolume("alice", 1.51))
}

func TestNodeMuteParticipant_NotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	require.Error(t, n.MuteParticipant("alice", true), "must fail when node has not been started")
}

func TestNodeParticipants_NotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)
	got := n.Participants()
	assert.NotNil(t, got, "must return non-nil empty slice, not nil")
	assert.Len(t, got, 0)
}

func TestNodeMuteParticipant_RunnerWithoutReceiver(t *testing.T) {
	runner := &plainMockRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)
	err := n.MuteParticipant("alice", true)
	require.Error(t, err, "runner that does not implement ParticipantCommandReceiver must fail")
}

func TestNodeParticipants_RunnerWithoutLister(t *testing.T) {
	runner := &plainMockRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)
	got := n.Participants()
	assert.NotNil(t, got)
	assert.Len(t, got, 0, "runner without ParticipantLister must yield empty slice")
}
