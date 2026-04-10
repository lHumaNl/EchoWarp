package echowarp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chatSenderMockRunner implements Runner + ChatSender so Node-level unit
// tests can assert the SendChat command delivery path end-to-end.
type chatSenderMockRunner struct {
	started chan struct{}
	once    sync.Once

	mu   sync.Mutex
	sent []chatCapture

	// failWith, when non-nil, causes SendChat to return this error instead
	// of capturing the call. Used to exercise the "runner returns error"
	// propagation path.
	failWith error
}

type chatCapture struct {
	text string
	to   string
}

func newChatSenderMockRunner() *chatSenderMockRunner {
	return &chatSenderMockRunner{started: make(chan struct{})}
}

func (r *chatSenderMockRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *chatSenderMockRunner) SendChat(text, to string) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, chatCapture{text: text, to: to})
	return nil
}

func (r *chatSenderMockRunner) captured() []chatCapture {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]chatCapture, len(r.sent))
	copy(out, r.sent)
	return out
}

func TestNodeSendChat_DeliversBroadcastToRunner(t *testing.T) {
	runner := newChatSenderMockRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SendChat("hello world", ""))

	// Give the runner goroutine a moment; SendChat is synchronous so it
	// should be immediate, but we still poll for robustness.
	deadline := time.Now().Add(500 * time.Millisecond)
	var captured []chatCapture
	for time.Now().Before(deadline) {
		captured = runner.captured()
		if len(captured) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Len(t, captured, 1, "runner must receive exactly one chat call")
	assert.Equal(t, "hello world", captured[0].text)
	assert.Equal(t, "", captured[0].to, "broadcast must be signaled by empty to")
}

func TestNodeSendChat_DeliversDMToRunner(t *testing.T) {
	runner := newChatSenderMockRunner()
	n := startNodeWithRunner(t, runner, runner.started)

	require.NoError(t, n.SendChat("hi Alice", "Alice"))

	captured := runner.captured()
	require.Len(t, captured, 1)
	assert.Equal(t, "hi Alice", captured[0].text)
	assert.Equal(t, "Alice", captured[0].to)
}

func TestNodeSendChat_EmptyTextRejected(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)

	err = n.SendChat("", "")
	require.Error(t, err)
	// Empty-text path must be rejected before the runner is consulted, so
	// the "not running" message must not appear.
	assert.NotContains(t, err.Error(), "not running")
}

func TestNodeSendChat_NotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	require.NoError(t, err)

	err = n.SendChat("hello", "")
	require.Error(t, err, "must fail when the node has not been started")
}

func TestNodeSendChat_RunnerWithoutSender(t *testing.T) {
	runner := &plainMockRunner{started: make(chan struct{})}
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SendChat("hello", "")
	require.Error(t, err, "runner without ChatSender must be rejected")
}

func TestNodeSendChat_RunnerErrorPropagated(t *testing.T) {
	runner := newChatSenderMockRunner()
	sentinel := errors.New("simulated channel send failure")
	runner.failWith = sentinel
	n := startNodeWithRunner(t, runner, runner.started)

	err := n.SendChat("hello", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel, "SendChat must propagate the runner's error unchanged")
}
