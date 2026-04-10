package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// chatSendRunner implements echowarp.Runner + echowarp.ChatSender so chat
// handler tests can assert both the HTTP response and the command payload
// that actually reached the runner.
type chatSendRunner struct {
	started chan struct{}
	once    sync.Once

	mu   sync.Mutex
	sent []chatSendCapture
}

type chatSendCapture struct {
	text string
	to   string
}

func newChatSendRunner() *chatSendRunner {
	return &chatSendRunner{started: make(chan struct{})}
}

func (r *chatSendRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *chatSendRunner) SendChat(text, to string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, chatSendCapture{text: text, to: to})
	return nil
}

func (r *chatSendRunner) captured() []chatSendCapture {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]chatSendCapture, len(r.sent))
	copy(out, r.sent)
	return out
}

// plainRunner implements only echowarp.Runner — used to exercise the
// "runner does not implement ChatSender" path that must return 500.
type plainChatRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainChatRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

// setupChatServer builds an APIServer backed by a running Node whose runner
// implements ChatSender. Returns the test HTTP server plus the runner so
// tests can introspect captured commands.
func setupChatServer(t *testing.T) (*httptest.Server, *chatSendRunner, func()) {
	t.Helper()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	runner := newChatSendRunner()
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return runner, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() { _ = node.Start(context.Background()) }()
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start in time")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/chat/send", apiServer.handleChatSend)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, runner, cleanup
}

// setupIdleChatServer wires an APIServer whose Node has never been started —
// used for 409 "not running" tests.
func setupIdleChatServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/chat/send", apiServer.handleChatSend)
	ts := httptest.NewServer(mux)
	return ts, func() { ts.Close() }
}

// setupChatServerPlainRunner wires an APIServer whose runner does NOT
// implement ChatSender. Used for the 500 (ErrInternalState) path.
func setupChatServerPlainRunner(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := &plainChatRunner{started: make(chan struct{})}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return runner, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() { _ = node.Start(context.Background()) }()
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start in time")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/chat/send", apiServer.handleChatSend)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, cleanup
}

func TestHandleChatSend(t *testing.T) {
	ts, runner, cleanup := setupChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"hello","to":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "", result["to"])

	captured := runner.captured()
	require.Len(t, captured, 1, "runner must have received exactly one chat send")
	assert.Equal(t, "hello", captured[0].text)
	assert.Equal(t, "", captured[0].to, "broadcast must carry empty to")
}

func TestHandleChatSendDM(t *testing.T) {
	ts, runner, cleanup := setupChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"hi Alice","to":"Alice"}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "Alice", result["to"])

	captured := runner.captured()
	require.Len(t, captured, 1)
	assert.Equal(t, "hi Alice", captured[0].text)
	assert.Equal(t, "Alice", captured[0].to)
}

func TestHandleChatSendEmptyText(t *testing.T) {
	ts, runner, cleanup := setupChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"","to":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Contains(t, strings.ToLower(result.Error), "text", "error must mention the text field")

	// Nothing must have reached the runner.
	assert.Empty(t, runner.captured(), "empty text must be rejected before reaching the runner")
}

func TestHandleChatSendWhitespaceOnlyText(t *testing.T) {
	ts, runner, cleanup := setupChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"   \t\n","to":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, runner.captured())
}

func TestHandleChatSendNotRunning(t *testing.T) {
	ts, cleanup := setupIdleChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"hello","to":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "idle node must return 409, not 200")

	var result errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Error)
}

func TestHandleChatSendRunnerWithoutSender(t *testing.T) {
	ts, cleanup := setupChatServerPlainRunner(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"text":"hello","to":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"runner without ChatSender must surface as 500 (ErrInternalState)")
}

func TestHandleChatSendInvalidJSON(t *testing.T) {
	ts, _, cleanup := setupChatServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{not json`)
	resp, err := http.Post(ts.URL+"/api/v1/chat/send", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestChatReceivedViaWS verifies the incoming path: when a Runner emits a
// chat event on the Node's EventBus, the WS bridge forwards it to connected
// WebSocket clients as a "chat_message" WSEvent with a ChatMessageData
// payload. This is the API-level counterpart of the chat receive flow
// exercised at the Node level by pkg/echowarp chat_test.go.
func TestChatReceivedViaWS(t *testing.T) {
	bus := echowarp.NewEventBus()
	defer bus.Shutdown()

	hub := &fakeBroadcaster{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bridgeDone := make(chan struct{})
	go func() {
		BridgeEventsToWS(ctx, bus, hub, logger)
		close(bridgeDone)
	}()
	// Let the bridge register its subscriptions.
	time.Sleep(20 * time.Millisecond)

	bus.EmitChatMessage("Alice", "", "hello everyone", 1234567890)

	events := hub.waitFor(t, 1, 2*time.Second)
	require.Len(t, events, 1)
	assert.Equal(t, "chat_message", events[0].Type)

	data, ok := events[0].Data.(echowarp.ChatMessageData)
	require.True(t, ok, "chat_message payload must be ChatMessageData")
	assert.Equal(t, "Alice", data.From)
	assert.Equal(t, "", data.To)
	assert.Equal(t, "hello everyone", data.Text)
	assert.Equal(t, int64(1234567890), data.TS)

	cancel()
	select {
	case <-bridgeDone:
	case <-time.After(1 * time.Second):
		t.Fatal("bridge did not return after context cancellation")
	}
}
