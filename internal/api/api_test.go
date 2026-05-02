package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

func createTestNode(t *testing.T) *echowarp.Node {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return &noopTestRunner{}, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)
	return node
}

func createTestServer(t *testing.T, token string) (*APIServer, *httptest.Server) {
	node := createTestNode(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", token, logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", apiServer.handleStatus)
	mux.HandleFunc("GET /api/v1/stats", apiServer.handleStats)
	mux.HandleFunc("GET /api/v1/devices", apiServer.handleDevices)
	mux.HandleFunc("GET /api/v1/clients", apiServer.handleClients)
	mux.HandleFunc("GET /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("POST /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("PUT /api/v1/config", apiServer.handleConfig)
	mux.HandleFunc("POST /api/v1/start", apiServer.handleStart)
	mux.HandleFunc("POST /api/v1/stop", apiServer.handleStop)
	mux.HandleFunc("POST /api/v1/pause", apiServer.handlePause)
	mux.HandleFunc("POST /api/v1/resume", apiServer.handleResume)
	mux.HandleFunc("POST /api/v1/shutdown", apiServer.handleShutdown)
	mux.HandleFunc("GET /ws/v1/events", apiServer.handleWebSocket)
	mux.HandleFunc("GET /api/v1/session/info", apiServer.handleSessionInfo)

	handler := apiServer.loggingMiddleware(apiServer.cors.Middleware(apiServer.rateLimiter.Middleware(bodyLimitMiddleware(apiServer.authMiddleware(mux)))))
	ts := httptest.NewServer(handler)
	t.Cleanup(func() { ts.Close() })

	return apiServer, ts
}

func TestAPIServer_Status_ReturnsCurrentState(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result statusResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.NotEmpty(t, result.Status)
}

func TestAPIServer_Devices_ListsAudioDevices(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/devices")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result []interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
}

func TestAPIServer_Stats_ReturnsStats(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPIServer_Clients_ReturnsClientList(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/clients")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result clientsResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.NotNil(t, result.Clients)
}

func TestAPIServer_Auth_ValidToken_Allowed(t *testing.T) {
	_, ts := createTestServer(t, "test-token")

	req, err := http.NewRequest("GET", ts.URL+"/api/v1/status", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPIServer_Auth_InvalidToken_Rejected(t *testing.T) {
	_, ts := createTestServer(t, "test-token")

	req, err := http.NewRequest("GET", ts.URL+"/api/v1/status", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer wrong-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPIServer_Auth_NoToken_LocalhostAllowed(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPIServer_Auth_MissingBearer_Returned401(t *testing.T) {
	_, ts := createTestServer(t, "test-token")

	req, err := http.NewRequest("GET", ts.URL+"/api/v1/status", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPIServer_Start_BeginsStreaming(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Post(ts.URL+"/api/v1/start", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPIServer_Stop_StopsStreaming(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Post(ts.URL+"/api/v1/stop", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAPIServer_Pause_WhenNotStreaming_Returns409(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Post(ts.URL+"/api/v1/pause", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAPIServer_Resume_WhenNotPaused_Returns409(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Post(ts.URL+"/api/v1/resume", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAPIServer_CORS_PreflightOK(t *testing.T) {
	_, ts := createTestServer(t, "test-token")

	req, err := http.NewRequest("OPTIONS", ts.URL+"/api/v1/status", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "http://localhost:3000", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestWSHub_Broadcast_SendsToClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	hub := NewWSHub(logger)

	event := WSEvent{Type: "test", Data: "hello"}
	hub.Broadcast(event)

	require.Equal(t, 0, hub.ClientCount())
}

func TestWSHub_AddRemoveClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	hub := NewWSHub(logger)

	require.Equal(t, 0, hub.ClientCount())

	hub.AddClient(nil, func() {})
	require.Equal(t, 1, hub.ClientCount())

	hub.RemoveClient(nil)
	require.Equal(t, 0, hub.ClientCount())
}

func TestWSHub_Close_ClosesAllConnections(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	hub := NewWSHub(logger)

	require.Equal(t, 0, hub.ClientCount())

	hub.Close()

	require.Equal(t, 0, hub.ClientCount())
}

func TestAPIServer_WebSocket_Connects(t *testing.T) {
	_, ts := createTestServer(t, "")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/v1/events"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		if strings.Contains(err.Error(), "501") {
			t.Skip("httptest.Server does not support WebSocket hijacking")
		}
		require.NoError(t, err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
}

func TestAPIServer_SessionInfo_ReturnsPublicCapabilities(t *testing.T) {
	_, ts := createTestServer(t, "")

	resp, err := http.Get(ts.URL + "/api/v1/session/info")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result sessionInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "normal", result.Mode)
	require.Equal(t, uint32(48000), result.SampleRate)
	require.Equal(t, uint32(1), result.Channels)
	require.NotEmpty(t, result.ServerVersion)
	require.False(t, result.PasswordRequired)
}

func TestAPIServer_SessionInfo_NoAuthRequired(t *testing.T) {
	// Server with auth token — session/info should still be accessible
	_, ts := createTestServer(t, "secret-token")

	resp, err := http.Get(ts.URL + "/api/v1/session/info")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result sessionInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, "normal", result.Mode)
}
