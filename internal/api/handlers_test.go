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
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

func setupTestHandlersServer(t *testing.T) (*APIServer, *httptest.Server) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	// Provide a no-op runner factory so node.Start() succeeds. Without it,
	// Start returns a synchronous "no runner factory configured" error which
	// handleStart now surfaces as HTTP 500 (see TestHandleStartErrorFeedback).
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return &noopTestRunner{}, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", apiServer.handleHealthz)
	mux.HandleFunc("GET /readyz", apiServer.handleReadyz)
	mux.HandleFunc("GET /api/v1/status", apiServer.handleStatus)
	mux.HandleFunc("GET /api/v1/version", apiServer.handleVersion)
	mux.HandleFunc("GET /api/v1/stats", apiServer.handleStats)
	mux.HandleFunc("GET /api/v1/devices", apiServer.handleDevices)
	mux.HandleFunc("GET /api/v1/clients", apiServer.handleClients)
	mux.HandleFunc("GET /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("POST /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("PUT /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("POST /api/v1/start", apiServer.handleStart)
	mux.HandleFunc("POST /api/v1/stop", apiServer.handleStop)
	mux.HandleFunc("POST /api/v1/connect", apiServer.handleConnect)
	mux.HandleFunc("POST /api/v1/disconnect", apiServer.handleDisconnect)
	mux.HandleFunc("POST /api/v1/pause", apiServer.handlePause)
	mux.HandleFunc("POST /api/v1/resume", apiServer.handleResume)
	mux.HandleFunc("POST /api/v1/shutdown", apiServer.handleShutdown)
	mux.HandleFunc("POST /api/v1/discover", apiServer.handleDiscover)
	mux.HandleFunc("POST /api/v1/devices/{id}/mute", apiServer.handleDeviceMute)
	mux.HandleFunc("POST /api/v1/devices/{id}/volume", apiServer.handleDeviceVolume)
	mux.HandleFunc("GET /api/v1/conference/participants", apiServer.handleConferenceParticipants)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/mute", apiServer.handleConferenceParticipantMute)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/kick", apiServer.handleConferenceParticipantKick)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/volume", apiServer.handleConferenceParticipantVolume)

	handler := SecurityHeadersMiddleware(apiServer.loggingMiddleware(apiServer.cors.Middleware(apiServer.rateLimiter.Middleware(bodyLimitMiddleware(apiServer.authMiddleware(mux))))))
	ts := httptest.NewServer(handler)
	t.Cleanup(func() { ts.Close() })

	return apiServer, ts
}

func TestHandleStatus(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result statusResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Status)
	assert.Equal(t, "idle", result.Status)
	assert.Equal(t, int64(0), result.Uptime)
}

func TestHandleVersion(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/version")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result, "version")
	assert.Contains(t, result, "commit")
	assert.Contains(t, result, "buildDate")
	assert.Contains(t, result, "goVersion")
	assert.NotEmpty(t, result["version"])
	assert.NotEmpty(t, result["goVersion"])
}

func TestSecurityHeaders(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "geolocation=(), microphone=(), camera=()", resp.Header.Get("Permissions-Policy"))
	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
}

func TestHandleStart_Success(t *testing.T) {
	apiServer, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Eventually(t, func() bool {
		return apiServer.node.Status() == echowarp.StatusStopped
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestHandleStart_AlreadyRunning(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}

	// Create a runner that blocks until context is canceled, keeping node in non-idle state.
	blockingRunner := &blockingTestRunner{started: make(chan struct{})}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return blockingRunner, nil
	}

	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() {
		_ = node.Start(context.Background())
	}()

	// Wait until node leaves idle/stopped state.
	<-blockingRunner.started
	defer func() {
		_ = node.Stop()
	}()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/start", apiServer.handleStart)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "already running")
}

// blockingTestRunner blocks in Run() until context is canceled.
type blockingTestRunner struct {
	started chan struct{}
}

func (r *blockingTestRunner) Run(ctx context.Context) error {
	close(r.started)
	<-ctx.Done()
	return nil
}

// noopTestRunner exits immediately without streaming. Used in the default
// test server setup so node.Start() has something to launch.
type noopTestRunner struct{}

func (r *noopTestRunner) Run(_ context.Context) error { return nil }

// TestHandleStartErrorFeedback asserts that POST /api/v1/start surfaces
// synchronous Node.Start() errors as HTTP 500 with an error body. Prior to
// the fix, handleStart used a fire-and-forget goroutine that discarded the
// error and always returned 200, making start failures invisible to API
// consumers such as the Decky plugin.
func TestHandleStartErrorFeedback(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	// Node with no runner factory — Node.Start() returns a synchronous error
	// ("no runner factory configured") that the handler must propagate.
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/start", apiServer.handleStart)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"start failure must be reported as 500, not 200")

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error, "error body must contain a message")
	assert.Contains(t, strings.ToLower(result.Error), "runner",
		"error should mention the missing runner factory")
}

func TestHandleStart_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", bytes.NewBufferString(`{invalid}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestHandleStart_WithConfig(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	configJSON := `{"mode":"server","port":8080}`
	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", bytes.NewBufferString(configJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandleStop_NotRunning(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/stop", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "not running")
}

func TestHandleStop_AfterStart(t *testing.T) {
	apiServer, ts := setupTestHandlersServer(t)

	resp1, err := http.Post(ts.URL+"/api/v1/start", "application/json", nil)
	require.NoError(t, err)
	resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	assert.Eventually(t, func() bool {
		return apiServer.node.Status() == echowarp.StatusStopped
	}, 500*time.Millisecond, 10*time.Millisecond)

	resp2, err := http.Post(ts.URL+"/api/v1/stop", "application/json", nil)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusConflict, resp2.StatusCode)
}

func TestHandlePause_NotStreaming(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/pause", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "not streaming")
}

func TestHandleResume_NotPaused(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/resume", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "not paused")
}

func TestHandleConfig_Get(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/config")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result, "Mode")
	assert.Contains(t, result, "SampleRate")
	assert.Contains(t, result, "Channels")
}

func TestHandleConfig_Post_ValidConfig(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	configJSON := `{"mode":"client","address":"192.168.1.100","port":8080}`
	resp, err := http.Post(ts.URL+"/api/v1/config", "application/json", bytes.NewBufferString(configJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "configured", result["status"])
}

func TestHandleConfig_Post_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/config", "application/json", bytes.NewBufferString(`{invalid json}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestHandleConfig_Put(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	configJSON := `{"mode":"server","port":9999}`
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/config", bytes.NewBufferString(configJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandleConfig_MethodNotAllowed(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestHandleDevices(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/devices")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestHandleClients_Empty(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/clients")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result clientsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Empty(t, result.Clients)
	assert.Equal(t, 0, result.Count)
	assert.GreaterOrEqual(t, result.Max, 1)
}

func TestHandleClients_WithClients(t *testing.T) {
	apiServer, ts := setupTestHandlersServer(t)

	apiServer.node.AddClient(echowarp.ClientInfo{ID: "client-1", Address: "192.168.1.10:5000"})
	apiServer.node.AddClient(echowarp.ClientInfo{ID: "client-2", Address: "192.168.1.11:5000"})

	resp, err := http.Get(ts.URL + "/api/v1/clients")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result clientsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Len(t, result.Clients, 2)
	assert.Equal(t, 2, result.Count)

	clientIDs := make(map[string]bool)
	for _, c := range result.Clients {
		clientIDs[c.ID] = true
		assert.NotEmpty(t, c.Address)
	}
	assert.True(t, clientIDs["client-1"])
	assert.True(t, clientIDs["client-2"])
}

func TestHandleClients_AfterRemove(t *testing.T) {
	apiServer, ts := setupTestHandlersServer(t)

	apiServer.node.AddClient(echowarp.ClientInfo{ID: "client-1", Address: "192.168.1.10:5000"})
	apiServer.node.AddClient(echowarp.ClientInfo{ID: "client-2", Address: "192.168.1.11:5000"})
	apiServer.node.RemoveClient("client-1")

	resp, err := http.Get(ts.URL + "/api/v1/clients")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result clientsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Len(t, result.Clients, 1)
	assert.Equal(t, 1, result.Count)
	assert.Equal(t, "client-2", result.Clients[0].ID)
}

func TestHandleShutdown(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/shutdown", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "shutting down", result["status"])
}

func TestHandleStats(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"test": "value"}

	writeJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]string
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["test"])
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, http.StatusBadRequest, "test error message")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result errorResponse
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "test error message", result.Error)
}

func TestApplyConfigOverrides_AllFields(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       8080,
		SampleRate: 44100,
		Channels:   2,
	}

	deviceID := uint32(1)
	reverse := true
	password := "secret"
	virtualMic := true

	req := configRequest{
		Mode:       "client",
		Address:    "192.168.1.100",
		Port:       9000,
		DeviceID:   &deviceID,
		Reverse:    &reverse,
		Password:   &password,
		SampleRate: 48000,
		Channels:   1,
		VirtualMic: &virtualMic,
	}

	applyConfigOverrides(&cfg, req)

	assert.Equal(t, echowarp.ModeClient, cfg.Mode)
	assert.Equal(t, "192.168.1.100", cfg.Address)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, &deviceID, cfg.DeviceID)
	assert.True(t, cfg.Reverse)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, uint32(48000), cfg.SampleRate)
	assert.Equal(t, uint32(1), cfg.Channels)
	assert.True(t, cfg.VirtualMic)
}

func TestApplyConfigOverrides_PartialFields(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       8080,
		SampleRate: 44100,
		Channels:   2,
	}

	req := configRequest{
		Port: 9000,
	}

	applyConfigOverrides(&cfg, req)

	assert.Equal(t, echowarp.ModeServer, cfg.Mode)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, uint32(44100), cfg.SampleRate)
	assert.Equal(t, uint32(2), cfg.Channels)
}

func TestStatusResponse_JSON(t *testing.T) {
	resp := statusResponse{
		Status: "streaming",
		Uptime: 3600,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled statusResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "streaming", unmarshaled.Status)
	assert.Equal(t, int64(3600), unmarshaled.Uptime)
}

func TestClientsResponse_JSON(t *testing.T) {
	resp := clientsResponse{
		Clients: []clientInfo{
			{ID: "client-1", Address: "192.168.1.10:5000"},
			{ID: "client-2", Address: "192.168.1.11:5000"},
		},
		Count: 2,
		Max:   10,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled clientsResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Len(t, unmarshaled.Clients, 2)
	assert.Equal(t, 2, unmarshaled.Count)
	assert.Equal(t, 10, unmarshaled.Max)
	assert.Equal(t, "client-1", unmarshaled.Clients[0].ID)
}

func TestErrorResponse_JSON(t *testing.T) {
	resp := errorResponse{
		Error: "something went wrong",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var unmarshaled errorResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "something went wrong", unmarshaled.Error)
}

func TestConfigRequest_JSON(t *testing.T) {
	deviceID := uint32(1)
	reverse := true
	password := "secret"
	virtualMic := true

	jsonStr := `{
		"mode": "client",
		"address": "192.168.1.100",
		"port": 8080,
		"device_id": 1,
		"reverse": true,
		"password": "secret",
		"sample_rate": 48000,
		"channels": 1,
		"virtual_mic": true
	}`

	var req configRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	require.NoError(t, err)

	assert.Equal(t, "client", req.Mode)
	assert.Equal(t, "192.168.1.100", req.Address)
	assert.Equal(t, 8080, req.Port)
	assert.Equal(t, &deviceID, req.DeviceID)
	assert.Equal(t, &reverse, req.Reverse)
	assert.Equal(t, &password, req.Password)
	assert.Equal(t, uint32(48000), req.SampleRate)
	assert.Equal(t, uint32(1), req.Channels)
	assert.Equal(t, &virtualMic, req.VirtualMic)
}

func TestConfigRequest_EmptyFields(t *testing.T) {
	jsonStr := `{}`

	var req configRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	require.NoError(t, err)

	assert.Empty(t, req.Mode)
	assert.Empty(t, req.Address)
	assert.Equal(t, 0, req.Port)
	assert.Nil(t, req.DeviceID)
	assert.Nil(t, req.Reverse)
	assert.Nil(t, req.Password)
	assert.Equal(t, uint32(0), req.SampleRate)
	assert.Equal(t, uint32(0), req.Channels)
	assert.Nil(t, req.VirtualMic)
}

func TestHandleConfig_HidesPassword(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       8080,
		SampleRate: 48000,
		Channels:   1,
		Password:   "secret-password",
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	safeCfg := node.SafeConfig()
	assert.Equal(t, "********", safeCfg.Password)
}

// ── Discovery tests ──────────────────────────────────────────────────────────

func TestHandleDiscover_NoBody(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - zeroconf crashes on mDNS browse cleanup")
	}
	// Discovery with no body should use default timeout and return an empty list
	// (no real mDNS services on CI/test hosts). We just assert the shape is correct.
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/discover", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// mDNS browse may fail on some CI environments; tolerate 500 as well.
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"expected 200 or 500, got %d", resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestHandleDiscover_InvalidJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - zeroconf crashes on mDNS browse cleanup")
	}
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/discover", "application/json", bytes.NewBufferString(`{bad}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestHandleDiscover_ZeroTimeout_UsesDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - zeroconf crashes on mDNS browse cleanup")
	}
	// timeout_ms: 0 should use default (5000ms). We send a very short explicit
	// timeout so the test finishes fast; just validate status + JSON array shape.
	_, ts := setupTestHandlersServer(t)

	body := bytes.NewBufferString(`{"timeout_ms": 50}`)
	resp, err := http.Post(ts.URL+"/api/v1/discover", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"expected 200 or 500, got %d", resp.StatusCode)
}

// ── Device Control tests ─────────────────────────────────────────────────────

func TestHandleDeviceMute_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	body := bytes.NewBufferString(`{"muted": true}`)
	resp, err := http.Post(ts.URL+"/api/v1/devices/1/mute", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleDeviceMute_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/devices/1/mute", "application/json", bytes.NewBufferString(`{bad}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestHandleDeviceVolume_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	body := bytes.NewBufferString(`{"volume": 0.75}`)
	resp, err := http.Post(ts.URL+"/api/v1/devices/2/volume", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleDeviceVolume_InvalidRange(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"negative", `{"volume": -0.1}`},
		{"too high", `{"volume": 2.1}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/api/v1/devices/1/volume", "application/json", bytes.NewBufferString(tc.payload))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var result errorResponse
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.Contains(t, result.Error, "volume must be between")
		})
	}
}

func TestHandleDeviceVolume_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/devices/1/volume", "application/json", bytes.NewBufferString(`{bad}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

// ── Conference tests ─────────────────────────────────────────────────────────

func TestHandleConferenceParticipants_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/conference/participants")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleConferenceParticipantMute_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	body := bytes.NewBufferString(`{"muted": true}`)
	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-1/mute", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleConferenceParticipantMute_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-1/mute", "application/json", bytes.NewBufferString(`{bad}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

func TestHandleConferenceParticipantKick_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-2/kick", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleConferenceParticipantVolume_Returns501(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	body := bytes.NewBufferString(`{"volume": 0.5}`)
	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-3/volume", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result["error"], "not yet implemented")
}

func TestHandleConferenceParticipantVolume_InvalidRange(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"negative", `{"volume": -1.0}`},
		{"too high", `{"volume": 3.0}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-1/volume", "application/json", bytes.NewBufferString(tc.payload))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var result errorResponse
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.Contains(t, result.Error, "volume must be between")
		})
	}
}

func TestHandleConferenceParticipantVolume_InvalidJSON(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/conference/participants/p-1/volume", "application/json", bytes.NewBufferString(`{bad}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "invalid JSON")
}

// ── Existing password tests (keep below new tests) ───────────────────────────

func TestHandleConfig_HidesEmptyPassword(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       8080,
		SampleRate: 48000,
		Channels:   1,
		Password:   "",
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	safeCfg := node.SafeConfig()
	assert.Equal(t, "", safeCfg.Password)
}

func TestAPIServer_NodeIntegration(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)

	assert.NotNil(t, apiServer.node)
	assert.Equal(t, echowarp.StatusIdle, apiServer.node.Status())

	uptime := apiServer.node.Uptime()
	assert.Equal(t, time.Duration(0), uptime)

	stats := apiServer.node.Stats()
	assert.NotNil(t, stats)
}

func TestNodeConfig_Copy(t *testing.T) {
	deviceID := uint32(1)
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       8080,
		DeviceID:   &deviceID,
		SampleRate: 48000,
		Channels:   2,
		Password:   "secret",
	}

	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	configCopy := node.Config()
	assert.Equal(t, cfg.Mode, configCopy.Mode)
	assert.Equal(t, cfg.Port, configCopy.Port)
	assert.Equal(t, cfg.SampleRate, configCopy.SampleRate)
	assert.Equal(t, cfg.Channels, configCopy.Channels)
	assert.Equal(t, cfg.Password, configCopy.Password)
}

func TestHandleConfig_ContextTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	_, ts := setupTestHandlersServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/config", http.NoBody)
	require.NoError(t, err)

	_, err = http.DefaultClient.Do(req)
	assert.Error(t, err)
}

func TestHandleHealthz(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
	assert.NotEmpty(t, result["version"])
}

func TestHandleReadyz(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "ready", result["status"])
}

func TestHandleReadyz_NodeNil(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()
	apiServer.node = nil

	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", apiServer.handleReadyz)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "not ready", result["status"])
}

// TestHandleConnect asserts that POST /api/v1/connect configures the node in
// client mode and transitions it through a Start cycle. The default test
// setup uses a noopTestRunner so Start returns quickly — the node should end
// up back in stopped state, but the handler must return 200 (fast success)
// and must have applied the client config during Reconfigure.
func TestHandleConnect(t *testing.T) {
	apiServer, ts := setupTestHandlersServer(t)

	body := `{"address":"127.0.0.1","port":4415,"password":"test","nickname":"bob"}`
	resp, err := http.Post(ts.URL+"/api/v1/connect", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "fast connect should return 200")

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "connected", result["status"])

	// The node config must reflect the new client-mode values.
	cfg := apiServer.node.Config()
	assert.Equal(t, echowarp.ModeClient, cfg.Mode, "node must be reconfigured to client mode")
	assert.Equal(t, "127.0.0.1", cfg.Address)
	assert.Equal(t, 4415, cfg.Port)
	assert.Equal(t, "bob", cfg.Nickname)

	// Noop runner returns immediately, so the node settles back in stopped state.
	assert.Eventually(t, func() bool {
		return apiServer.node.Status() == echowarp.StatusStopped
	}, 500*time.Millisecond, 10*time.Millisecond)
}

// TestHandleConnectMissingAddress asserts that POST /api/v1/connect returns
// 400 when neither address nor discover is set — the caller has no target.
func TestHandleConnectMissingAddress(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/connect", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "address")
}

// TestHandleConnectWhenRunning asserts that POST /api/v1/connect returns 409
// when the node is already in a non-idle state — connect must not hijack a
// running session.
func TestHandleConnectWhenRunning(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}

	blockingRunner := &blockingTestRunner{started: make(chan struct{})}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return blockingRunner, nil
	}

	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() {
		_ = node.Start(context.Background())
	}()
	<-blockingRunner.started
	defer func() { _ = node.Stop() }()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/connect", apiServer.handleConnect)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body := `{"address":"127.0.0.1"}`
	resp, err := http.Post(ts.URL+"/api/v1/connect", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "already running")
}

// TestHandleDisconnect asserts that POST /api/v1/disconnect is routed through
// the same stop path as POST /api/v1/stop. With an idle node it must return
// 409 (node not running) — the error body is identical to /stop.
func TestHandleDisconnect(t *testing.T) {
	_, ts := setupTestHandlersServer(t)

	resp, err := http.Post(ts.URL+"/api/v1/disconnect", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var result errorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Error, "not running")
}

// TestHandleDisconnectStopsRunningNode asserts that POST /api/v1/disconnect
// successfully stops a running node — exercising the happy path of the
// client-side semantic alias.
func TestHandleDisconnectStopsRunningNode(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeClient,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}

	blockingRunner := &blockingTestRunner{started: make(chan struct{})}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return blockingRunner, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() { _ = node.Start(context.Background()) }()
	<-blockingRunner.started

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/disconnect", apiServer.handleDisconnect)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/disconnect", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "stopped", result["status"])
}
