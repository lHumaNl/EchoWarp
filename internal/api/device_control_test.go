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

// deviceCmdRunner is a test runner that implements both echowarp.Runner and
// echowarp.DeviceCommandReceiver. It blocks until Run's context is canceled
// (keeping the node in Streaming state for the duration of the test) and
// records every device command received on a buffered channel that tests
// can inspect to verify plumbing end-to-end.
type deviceCmdRunner struct {
	started  chan struct{}
	commands chan echowarp.DeviceCommand
	once     sync.Once
}

func newDeviceCmdRunner() *deviceCmdRunner {
	return &deviceCmdRunner{
		started:  make(chan struct{}),
		commands: make(chan echowarp.DeviceCommand, 8),
	}
}

func (r *deviceCmdRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *deviceCmdRunner) HandleDeviceCommand(cmd echowarp.DeviceCommand) error {
	select {
	case r.commands <- cmd:
		return nil
	case <-time.After(100 * time.Millisecond):
		return context.DeadlineExceeded
	}
}

// setupDeviceControlServer wires an APIServer backed by a running Node whose
// runner is a deviceCmdRunner, so tests can assert both the HTTP response
// status and the command that actually reached the runner.
func setupDeviceControlServer(t *testing.T) (*APIServer, *httptest.Server, *deviceCmdRunner, func()) {
	t.Helper()

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	runner := newDeviceCmdRunner()
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
	mux.HandleFunc("POST /api/v1/devices/{id}/mute", apiServer.handleDeviceMute)
	mux.HandleFunc("POST /api/v1/devices/{id}/volume", apiServer.handleDeviceVolume)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return apiServer, ts, runner, cleanup
}

func TestHandleDeviceMuteImplemented(t *testing.T) {
	_, ts, runner, cleanup := setupDeviceControlServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"muted":true}`)
	resp, err := http.Post(ts.URL+"/api/v1/devices/7/mute", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK, not the phase-3 501 stub")

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.EqualValues(t, 7, result["device_id"])
	assert.Equal(t, true, result["muted"])

	// Assert the command actually reached the runner — not just that the
	// handler returned 200 (phase-3 used a 501 stub).
	select {
	case cmd := <-runner.commands:
		assert.Equal(t, echowarp.DeviceActionSetMute, cmd.Action)
		assert.Equal(t, 7, cmd.DeviceID)
		assert.True(t, cmd.Muted)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive device command")
	}
}

func TestHandleDeviceMuteNotRunning(t *testing.T) {
	// Fresh node without Start — runner is nil.
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/devices/{id}/mute", apiServer.handleDeviceMute)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/devices/1/mute", "application/json", bytes.NewBufferString(`{"muted":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "idle node must return 409, not 200")

	var result errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Error)
}

func TestHandleDeviceMuteInvalidID(t *testing.T) {
	_, ts, _, cleanup := setupDeviceControlServer(t)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/devices/notanumber/mute", "application/json", bytes.NewBufferString(`{"muted":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleDeviceVolumeImplemented(t *testing.T) {
	_, ts, runner, cleanup := setupDeviceControlServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"volume":0.5}`)
	resp, err := http.Post(ts.URL+"/api/v1/devices/3/volume", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
	assert.EqualValues(t, 3, result["device_id"])
	assert.InDelta(t, 0.5, result["volume"], 1e-9)

	select {
	case cmd := <-runner.commands:
		assert.Equal(t, echowarp.DeviceActionSetVolume, cmd.Action)
		assert.Equal(t, 3, cmd.DeviceID)
		assert.InDelta(t, 0.5, cmd.Volume, 1e-9)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runner did not receive device command")
	}
}

func TestHandleDeviceVolumeOutOfRange(t *testing.T) {
	_, ts, _, cleanup := setupDeviceControlServer(t)
	defer cleanup()

	cases := []string{`{"volume":2.0}`, `{"volume":-0.1}`, `{"volume":1.6}`}
	for _, body := range cases {
		resp, err := http.Post(ts.URL+"/api/v1/devices/1/volume", "application/json", bytes.NewBufferString(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", body)
		_ = resp.Body.Close()
	}
}

func TestHandleDeviceVolumeInvalidID(t *testing.T) {
	_, ts, _, cleanup := setupDeviceControlServer(t)
	defer cleanup()

	resp, err := http.Post(ts.URL+"/api/v1/devices/-1/volume", "application/json", bytes.NewBufferString(`{"volume":0.5}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
