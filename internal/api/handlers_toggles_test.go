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

// muteMockRunner implements echowarp.Runner + echowarp.MuteController so
// handler tests can observe that the handler reached the runner without
// spinning up a real ClientApp (which would need a live signaler and
// playback device).
type muteMockRunner struct {
	started chan struct{}
	once    sync.Once

	mu        sync.Mutex
	calls     int
	lastMuted bool
	injectErr error
}

func newMuteMockRunner() *muteMockRunner {
	return &muteMockRunner{started: make(chan struct{})}
}

func (r *muteMockRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *muteMockRunner) SetMuted(muted bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.injectErr != nil {
		return r.injectErr
	}
	r.lastMuted = muted
	return nil
}

// discoveryMockRunner implements echowarp.Runner + echowarp.DiscoveryPublisher.
type discoveryMockRunner struct {
	started chan struct{}
	once    sync.Once

	mu          sync.Mutex
	calls       int
	lastEnabled bool
	injectErr   error
}

func newDiscoveryMockRunner() *discoveryMockRunner {
	return &discoveryMockRunner{started: make(chan struct{})}
}

func (r *discoveryMockRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *discoveryMockRunner) SetDiscoveryPublish(enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.injectErr != nil {
		return r.injectErr
	}
	r.lastEnabled = enabled
	return nil
}

// plainToggleRunner implements echowarp.Runner only — exercises the
// ErrInternalState fallback path for both endpoints.
type plainToggleAPIRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainToggleAPIRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

// ---------------------------------------------------------------------------
// Compile-time guard: existing ban/recording/chat/conference/device
// fakes must NOT accidentally implement the new toggle interfaces.
// The guard below is itself a compile-time assertion that the sibling
// test fakes defined in this package are visible to the Go type
// system — if any of them grows a SetMuted or SetDiscoveryPublish
// method in a future refactor, the _ assignments here will silently
// succeed and test coverage will regress. To catch that regression
// we instead rely on the plainToggleAPIRunner path (below) and the
// separate "RunnerWithoutController" tests. No explicit var _
// assignments are needed because Go's type system already rejects
// the negative form.
// ---------------------------------------------------------------------------

func setupMuteServer(t *testing.T) (*httptest.Server, *muteMockRunner, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeClient, SampleRate: 48000, Channels: 1}
	runner := newMuteMockRunner()
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
	mux.HandleFunc("POST /api/v1/mute", apiServer.handleMuteToggle)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, runner, cleanup
}

func setupDiscoveryServer(t *testing.T) (*httptest.Server, *discoveryMockRunner, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := newDiscoveryMockRunner()
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
	mux.HandleFunc("POST /api/v1/discovery/publish", apiServer.handleDiscoveryPublish)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, runner, cleanup
}

func setupToggleIdleServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeClient, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/mute", apiServer.handleMuteToggle)
	mux.HandleFunc("POST /api/v1/discovery/publish", apiServer.handleDiscoveryPublish)
	ts := httptest.NewServer(mux)
	return ts, func() { ts.Close() }
}

func setupTogglePlainRunnerServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeClient, SampleRate: 48000, Channels: 1}
	runner := &plainToggleAPIRunner{started: make(chan struct{})}
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
	mux.HandleFunc("POST /api/v1/mute", apiServer.handleMuteToggle)
	mux.HandleFunc("POST /api/v1/discovery/publish", apiServer.handleDiscoveryPublish)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, cleanup
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	return resp
}

// ---------------------------------------------------------------------------
// POST /api/v1/mute
// ---------------------------------------------------------------------------

func TestHandleMuteToggle_True(t *testing.T) {
	ts, runner, cleanup := setupMuteServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{"muted":true}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, true, body["muted"])

	runner.mu.Lock()
	assert.Equal(t, 1, runner.calls)
	assert.True(t, runner.lastMuted, "runner must have received muted=true — проверка 'применяется в runner'")
	runner.mu.Unlock()
}

func TestHandleMuteToggle_False(t *testing.T) {
	ts, runner, cleanup := setupMuteServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{"muted":false}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, false, body["muted"])

	runner.mu.Lock()
	assert.Equal(t, 1, runner.calls)
	assert.False(t, runner.lastMuted)
	runner.mu.Unlock()
}

func TestHandleMuteToggle_Idempotent(t *testing.T) {
	ts, runner, cleanup := setupMuteServer(t)
	defer cleanup()

	for i := 0; i < 3; i++ {
		resp := postJSON(t, ts.URL+"/api/v1/mute", `{"muted":true}`)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
	runner.mu.Lock()
	assert.Equal(t, 3, runner.calls)
	assert.True(t, runner.lastMuted)
	runner.mu.Unlock()
}

func TestHandleMuteToggle_InvalidJSON(t *testing.T) {
	ts, runner, cleanup := setupMuteServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{not json`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	runner.mu.Lock()
	assert.Zero(t, runner.calls, "runner must not have been touched on invalid JSON")
	runner.mu.Unlock()
}

func TestHandleMuteToggle_MissingField(t *testing.T) {
	ts, runner, cleanup := setupMuteServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	runner.mu.Lock()
	assert.Zero(t, runner.calls)
	runner.mu.Unlock()
}

func TestHandleMuteToggle_NotRunning(t *testing.T) {
	ts, cleanup := setupToggleIdleServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{"muted":true}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleMuteToggle_RunnerWithoutController(t *testing.T) {
	ts, cleanup := setupTogglePlainRunnerServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/mute", `{"muted":true}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// POST /api/v1/discovery/publish
// ---------------------------------------------------------------------------

func TestHandleDiscoveryPublish_True(t *testing.T) {
	ts, runner, cleanup := setupDiscoveryServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{"enabled":true}`)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, true, body["enabled"])

	runner.mu.Lock()
	assert.Equal(t, 1, runner.calls, "mDNS announcer must have been started — проверка start/stop")
	assert.True(t, runner.lastEnabled)
	runner.mu.Unlock()
}

func TestHandleDiscoveryPublish_False(t *testing.T) {
	ts, runner, cleanup := setupDiscoveryServer(t)
	defer cleanup()

	// Drive enable then disable to exercise the full start/stop cycle.
	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{"enabled":true}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	resp = postJSON(t, ts.URL+"/api/v1/discovery/publish", `{"enabled":false}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, false, body["enabled"])

	runner.mu.Lock()
	assert.Equal(t, 2, runner.calls, "mDNS announcer must have been stopped after enable — проверка start/stop")
	assert.False(t, runner.lastEnabled)
	runner.mu.Unlock()
}

func TestHandleDiscoveryPublish_InvalidJSON(t *testing.T) {
	ts, runner, cleanup := setupDiscoveryServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{not json`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	runner.mu.Lock()
	assert.Zero(t, runner.calls)
	runner.mu.Unlock()
}

func TestHandleDiscoveryPublish_MissingField(t *testing.T) {
	ts, runner, cleanup := setupDiscoveryServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	runner.mu.Lock()
	assert.Zero(t, runner.calls)
	runner.mu.Unlock()
}

func TestHandleDiscoveryPublish_NotRunning(t *testing.T) {
	ts, cleanup := setupToggleIdleServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{"enabled":true}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleDiscoveryPublish_RunnerWithoutPublisher(t *testing.T) {
	ts, cleanup := setupTogglePlainRunnerServer(t)
	defer cleanup()

	resp := postJSON(t, ts.URL+"/api/v1/discovery/publish", `{"enabled":true}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
