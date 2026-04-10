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

// recordingMockRunner implements echowarp.Runner + RecordingController
// so handler tests can observe every call reaching the runner without
// spinning up a real ServerApp + ConferenceRecorder (which would need
// an audio device and a writable Documents directory). The mock lets
// each test inject start/stop errors independently and read back the
// captured mode to assert the handler forwarded it verbatim.
type recordingMockRunner struct {
	started chan struct{}
	once    sync.Once

	mu           sync.Mutex
	capturedMode echowarp.RecordingMode
	startCalled  int
	stopCalled   int
	statusCalled int

	startErr  error
	stopErr   error
	stopRes   echowarp.RecordingResult
	curStatus echowarp.RecordingStatus
}

func newRecordingMockRunner() *recordingMockRunner {
	return &recordingMockRunner{started: make(chan struct{})}
}

func (r *recordingMockRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *recordingMockRunner) StartRecording(mode echowarp.RecordingMode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startCalled++
	if r.startErr != nil {
		return r.startErr
	}
	r.capturedMode = mode
	r.curStatus = echowarp.RecordingStatus{
		Active:    true,
		Mode:      mode,
		StartedAt: time.Now(),
	}
	return nil
}

func (r *recordingMockRunner) StopRecording() (echowarp.RecordingResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalled++
	if r.stopErr != nil {
		return echowarp.RecordingResult{Files: []string{}}, r.stopErr
	}
	r.curStatus = echowarp.RecordingStatus{}
	return r.stopRes, nil
}

func (r *recordingMockRunner) RecordingStatus() echowarp.RecordingStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCalled++
	return r.curStatus
}

func (r *recordingMockRunner) captured() echowarp.RecordingMode {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.capturedMode
}

// plainRecordingRunner implements echowarp.Runner only — exercises the
// "runner does not implement RecordingController" fallback path.
type plainRecordingRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainRecordingRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func setupRecordingServer(t *testing.T) (*httptest.Server, *recordingMockRunner, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := newRecordingMockRunner()
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
	mux.HandleFunc("POST /api/v1/recording/start", apiServer.handleRecordingStart)
	mux.HandleFunc("POST /api/v1/recording/stop", apiServer.handleRecordingStop)
	mux.HandleFunc("GET /api/v1/recording/status", apiServer.handleRecordingStatus)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, runner, cleanup
}

func setupRecordingServerIdle(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/recording/start", apiServer.handleRecordingStart)
	mux.HandleFunc("POST /api/v1/recording/stop", apiServer.handleRecordingStop)
	mux.HandleFunc("GET /api/v1/recording/status", apiServer.handleRecordingStatus)
	ts := httptest.NewServer(mux)
	return ts, func() { ts.Close() }
}

func setupRecordingServerPlainRunner(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := &plainRecordingRunner{started: make(chan struct{})}
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
	mux.HandleFunc("POST /api/v1/recording/start", apiServer.handleRecordingStart)
	mux.HandleFunc("POST /api/v1/recording/stop", apiServer.handleRecordingStop)
	mux.HandleFunc("GET /api/v1/recording/status", apiServer.handleRecordingStatus)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, cleanup
}

func postRecording(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	return resp
}

func TestHandleRecordingStartValid(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"mix"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "started", body["status"])
	assert.Equal(t, "mix", body["mode"])
	assert.Equal(t, echowarp.RecordingModeMix, runner.captured())
}

func TestHandleRecordingStartTracks(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"tracks"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, echowarp.RecordingModeTracks, runner.captured())
}

func TestHandleRecordingStartBoth(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"both"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, echowarp.RecordingModeBoth, runner.captured())
}

func TestHandleRecordingStartInvalidMode(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"bogus"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	// Runner must not have received the call (pre-check failed).
	runner.mu.Lock()
	assert.Zero(t, runner.startCalled)
	runner.mu.Unlock()
}

func TestHandleRecordingStartMissingMode(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	runner.mu.Lock()
	assert.Zero(t, runner.startCalled)
	runner.mu.Unlock()
}

func TestHandleRecordingStartInvalidJSON(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{not json`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	runner.mu.Lock()
	assert.Zero(t, runner.startCalled)
	runner.mu.Unlock()
}

func TestHandleRecordingStartNotRunning(t *testing.T) {
	ts, cleanup := setupRecordingServerIdle(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"mix"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleRecordingStartRunnerWithoutController(t *testing.T) {
	ts, cleanup := setupRecordingServerPlainRunner(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"mix"}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleRecordingStop(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	// Pre-seed a Stop result on the mock so we can assert the handler
	// forwards every field.
	runner.mu.Lock()
	runner.stopRes = echowarp.RecordingResult{
		Duration:   3 * time.Second,
		DurationMs: 3000,
		Size:       4242,
		Files:      []string{"/tmp/mix.wav", "/tmp/participant-1.wav"},
	}
	runner.mu.Unlock()

	resp := postRecording(t, ts.URL+"/api/v1/recording/stop", `{}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var result echowarp.RecordingResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, int64(3000), result.DurationMs)
	assert.Equal(t, int64(4242), result.Size)
	require.Len(t, result.Files, 2)
	assert.Equal(t, "/tmp/mix.wav", result.Files[0])
	assert.Equal(t, "/tmp/participant-1.wav", result.Files[1])
}

func TestHandleRecordingStopIdempotent(t *testing.T) {
	// Stop when nothing is active — the adapter returns a zero-value
	// result and the handler maps that to 200 with an empty Files
	// slice (not null). Matches the Node contract documented in
	// echowarp.go.
	ts, _, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/stop", `{}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var result echowarp.RecordingResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotNil(t, result.Files, "Files must be [] not null")
	assert.Empty(t, result.Files)
	assert.Zero(t, result.DurationMs)
	assert.Zero(t, result.Size)
}

func TestHandleRecordingStopNotRunning(t *testing.T) {
	ts, cleanup := setupRecordingServerIdle(t)
	defer cleanup()

	resp := postRecording(t, ts.URL+"/api/v1/recording/stop", `{}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleRecordingStatusActive(t *testing.T) {
	ts, runner, cleanup := setupRecordingServer(t)
	defer cleanup()

	// Drive the mock into an "active" state via the handler.
	resp := postRecording(t, ts.URL+"/api/v1/recording/start", `{"mode":"mix"}`)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	statusResp, err := http.Get(ts.URL + "/api/v1/recording/status")
	require.NoError(t, err)
	defer statusResp.Body.Close()

	assert.Equal(t, http.StatusOK, statusResp.StatusCode)
	var status echowarp.RecordingStatus
	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&status))
	assert.True(t, status.Active)
	assert.Equal(t, echowarp.RecordingModeMix, status.Mode)
	// Mock ran StartRecording once, Status at least once.
	runner.mu.Lock()
	assert.Equal(t, 1, runner.startCalled)
	assert.GreaterOrEqual(t, runner.statusCalled, 1)
	runner.mu.Unlock()
}

func TestHandleRecordingStatusInactive(t *testing.T) {
	ts, _, cleanup := setupRecordingServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/recording/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var status echowarp.RecordingStatus
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	assert.False(t, status.Active)
	assert.Empty(t, status.Mode)
	assert.Zero(t, status.DurationMs)
}

func TestHandleRecordingStatusEmptyWhenIdle(t *testing.T) {
	// Idempotent GET — must return 200 + inactive even when node
	// never started. Matches the BanList/Participants convention.
	ts, cleanup := setupRecordingServerIdle(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/recording/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var status echowarp.RecordingStatus
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	assert.False(t, status.Active)
}

func TestHandleRecordingStatusRunnerWithoutController(t *testing.T) {
	// Runner lacks RecordingController — GET still 200 + inactive.
	ts, cleanup := setupRecordingServerPlainRunner(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/recording/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var status echowarp.RecordingStatus
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&status))
	assert.False(t, status.Active)
}
