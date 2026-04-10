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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// participantCmdRunner is a test runner that implements echowarp.Runner plus
// the two optional conference interfaces (ParticipantLister +
// ParticipantCommandReceiver). It blocks in Run until the context is
// canceled so the node stays in Streaming state, and records every received
// participant command on a buffered channel tests can inspect to verify
// plumbing end-to-end.
type participantCmdRunner struct {
	started  chan struct{}
	commands chan echowarp.ParticipantCommand
	once     sync.Once

	mu       sync.Mutex
	snapshot []echowarp.ParticipantInfo
}

func newParticipantCmdRunner(snapshot []echowarp.ParticipantInfo) *participantCmdRunner {
	return &participantCmdRunner{
		started:  make(chan struct{}),
		commands: make(chan echowarp.ParticipantCommand, 8),
		snapshot: snapshot,
	}
}

func (r *participantCmdRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *participantCmdRunner) Participants() []echowarp.ParticipantInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]echowarp.ParticipantInfo, len(r.snapshot))
	copy(out, r.snapshot)
	return out
}

func (r *participantCmdRunner) HandleParticipantCommand(cmd echowarp.ParticipantCommand) error {
	select {
	case r.commands <- cmd:
		return nil
	case <-time.After(100 * time.Millisecond):
		return context.DeadlineExceeded
	}
}

// setupConferenceControlServer wires an APIServer backed by a running Node
// whose runner is a participantCmdRunner, so tests can assert both the HTTP
// response and the command that actually reached the runner.
func setupConferenceControlServer(t *testing.T, snapshot []echowarp.ParticipantInfo) (*APIServer, *httptest.Server, *participantCmdRunner, func()) {
	t.Helper()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	runner := newParticipantCmdRunner(snapshot)
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
	mux.HandleFunc("GET /api/v1/conference/participants", apiServer.handleConferenceParticipants)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/mute", apiServer.handleConferenceParticipantMute)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/kick", apiServer.handleConferenceParticipantKick)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/volume", apiServer.handleConferenceParticipantVolume)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return apiServer, ts, runner, cleanup
}

// setupIdleConferenceServer builds an APIServer whose Node has never been
// started — used for 409 "not running" tests.
func setupIdleConferenceServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/mute", apiServer.handleConferenceParticipantMute)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/kick", apiServer.handleConferenceParticipantKick)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/volume", apiServer.handleConferenceParticipantVolume)
	ts := httptest.NewServer(mux)
	cleanup := func() { ts.Close() }
	return ts, cleanup
}

func TestHandleConferenceParticipants(t *testing.T) {
	snapshot := []echowarp.ParticipantInfo{
		{ID: "alice", Nickname: "Alice", Muted: false, Volume: 1.0},
		{ID: "bob", Nickname: "Bob", Muted: true, Volume: 0.8},
	}
	_, ts, _, cleanup := setupConferenceControlServer(t, snapshot)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/conference/participants")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK, not the phase-3 501 stub")

	var result []echowarp.ParticipantInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Len(t, result, 2)
	assert.Equal(t, "alice", result[0].ID)
	assert.Equal(t, "Alice", result[0].Nickname)
	assert.False(t, result[0].Muted)
	assert.InDelta(t, 1.0, result[0].Volume, 1e-9)
	assert.Equal(t, "bob", result[1].ID)
	assert.True(t, result[1].Muted)
	assert.InDelta(t, 0.8, result[1].Volume, 1e-9)
}

func TestHandleConferenceParticipantMute(t *testing.T) {
	_, ts, runner, cleanup := setupConferenceControlServer(t, nil)
	defer cleanup()

	body := bytes.NewBufferString(`{"muted":true}`)
	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/abc/mute", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK, not the phase-3 501 stub")

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "abc", result["participant_id"])
	assert.Equal(t, true, result["muted"])

	// Assert the command actually reached the runner.
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, echowarp.ParticipantActionMute, cmd.Action)
		assert.Equal(t, "abc", cmd.ID)
		assert.True(t, cmd.Muted)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive participant mute command")
	}
}

func TestHandleConferenceParticipantKick(t *testing.T) {
	_, ts, runner, cleanup := setupConferenceControlServer(t, nil)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/xyz/kick", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "xyz", result["participant_id"])
	assert.Equal(t, true, result["kicked"])

	select {
	case cmd := <-runner.commands:
		assert.Equal(t, echowarp.ParticipantActionKick, cmd.Action)
		assert.Equal(t, "xyz", cmd.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive participant kick command")
	}
}

func TestHandleConferenceParticipantVolume(t *testing.T) {
	_, ts, runner, cleanup := setupConferenceControlServer(t, nil)
	defer cleanup()

	body := bytes.NewBufferString(`{"volume":0.5}`)
	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p1/volume", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "p1", result["participant_id"])
	assert.InDelta(t, 0.5, result["volume"], 1e-9)

	select {
	case cmd := <-runner.commands:
		assert.Equal(t, echowarp.ParticipantActionSetVolume, cmd.Action)
		assert.Equal(t, "p1", cmd.ID)
		assert.InDelta(t, 0.5, cmd.Volume, 1e-9)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive participant volume command")
	}
}

func TestHandleConferenceParticipantVolumeOutOfRange(t *testing.T) {
	_, ts, _, cleanup := setupConferenceControlServer(t, nil)
	defer cleanup()

	cases := []string{`{"volume":2.0}`, `{"volume":-0.1}`, `{"volume":1.6}`}
	for _, body := range cases {
		resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p1/volume", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", body)
		_ = resp.Body.Close()
	}
}

func TestHandleConferenceParticipantMuteNotRunning(t *testing.T) {
	ts, cleanup := setupIdleConferenceServer(t)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/abc/mute", "application/json", bytes.NewBufferString(`{"muted":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "idle node must return 409, not 200")
	var result errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Error)
}

func TestHandleConferenceParticipantVolumeNotRunning(t *testing.T) {
	ts, cleanup := setupIdleConferenceServer(t)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/abc/volume", "application/json", bytes.NewBufferString(`{"volume":0.5}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "idle node must return 409, not 200")
}

func TestHandleConferenceParticipantKickNotRunning(t *testing.T) {
	ts, cleanup := setupIdleConferenceServer(t)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/abc/kick", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "idle node must return 409, not 200")
}

// TestHandleConferenceParticipantsEmptyWhenIdle asserts that listing on an
// idle node returns 200 with an empty array (not 409). The REST contract
// intentionally treats "no participants" and "node not running" the same
// for the read path — callers distinguish via GET /api/v1/status.
func TestHandleConferenceParticipantsEmptyWhenIdle(t *testing.T) {
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/conference/participants", apiServer.handleConferenceParticipants)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/conference/participants")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var result []echowarp.ParticipantInfo
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Len(t, result, 0)
}
