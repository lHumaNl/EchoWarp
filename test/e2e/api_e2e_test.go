//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lHumaNl/echowarp/internal/api"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func TestE2E_API_StatusEndpoint_ReturnsCurrentState(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       port,
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	resp, err := http.Get(baseURL + "/api/v1/status")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var statusResp struct {
		Status string `json:"status"`
		Uptime int64  `json:"uptime"`
	}
	if err := json.Unmarshal(body, &statusResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	validStatuses := map[string]bool{"idle": true, "connecting": true, "streaming": true, "stopped": true, "reconnecting": true}
	if !validStatuses[statusResp.Status] {
		t.Errorf("invalid status: %s", statusResp.Status)
	}
}

func TestE2E_API_DevicesEndpoint_ListsDevices(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       port,
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	resp, err := http.Get(baseURL + "/api/v1/devices")
	if err != nil {
		t.Fatalf("failed to get devices: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var devices []struct {
		ID      uint32 `json:"id"`
		Name    string `json:"name"`
		IsInput bool   `json:"is_input"`
	}
	if err := json.Unmarshal(body, &devices); err != nil {
		t.Logf("response body: %s", string(body))
	}
}

func TestE2E_API_StartStop_StateTransitions(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       freePort(t),
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	getStatus := func() string {
		resp, err := http.Get(baseURL + "/api/v1/status")
		if err != nil {
			t.Fatalf("failed to get status: %v", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var statusResp struct{ Status string }
		json.Unmarshal(body, &statusResp)
		return statusResp.Status
	}

	if status := getStatus(); status != "idle" && status != "stopped" {
		t.Errorf("expected initial status to be idle or stopped, got %s", status)
	}

	postResp, err := http.Post(baseURL+"/api/v1/start", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to post start: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(postResp.Body)
		t.Errorf("expected status 200 for start, got %d: %s", postResp.StatusCode, string(body))
	}

	postResp2, err := http.Post(baseURL+"/api/v1/start", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to post second start: %v", err)
	}
	defer postResp2.Body.Close()

	if postResp2.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate start, got %d", postResp2.StatusCode)
	}

	postResp3, err := http.Post(baseURL+"/api/v1/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to post stop: %v", err)
	}
	defer postResp3.Body.Close()

	if postResp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(postResp3.Body)
		t.Errorf("expected status 200 for stop, got %d: %s", postResp3.StatusCode, string(body))
	}
}

func TestE2E_API_PauseResume_OnlyWhenStreaming(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       freePort(t),
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	pauseResp, err := http.Post(baseURL+"/api/v1/pause", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to post pause: %v", err)
	}
	defer pauseResp.Body.Close()

	if pauseResp.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 for pause on idle, got %d", pauseResp.StatusCode)
	}

	resumeResp, err := http.Post(baseURL+"/api/v1/resume", "application/json", nil)
	if err != nil {
		t.Fatalf("failed to post resume: %v", err)
	}
	defer resumeResp.Body.Close()

	if resumeResp.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 for resume on idle, got %d", resumeResp.StatusCode)
	}
}

func TestE2E_API_Auth_TokenRequired(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       freePort(t),
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "secret-token", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	resp1, err := http.Get(baseURL + "/api/v1/status")
	if err != nil {
		t.Fatalf("failed to get status without token: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode == http.StatusOK {
		t.Error("expected localhost request to bypass auth, but got success")
	}

	req, _ := http.NewRequest("GET", baseURL+"/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to get status with wrong token: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong token, got %d", resp2.StatusCode)
	}

	req3, _ := http.NewRequest("GET", baseURL+"/api/v1/status", nil)
	req3.Header.Set("Authorization", "Bearer secret-token")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("failed to get status with correct token: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for correct token, got %d", resp3.StatusCode)
	}
}

func TestE2E_API_CORS_Headers(t *testing.T) {
	t.Parallel()

	port := freePort(t)
	bindAddr := "127.0.0.1:" + strconv.Itoa(port)

	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       freePort(t),
		SampleRate: 48000,
		Channels:   1,
	}

	node, err := echowarp.NewNode(cfg)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	apiServer := api.NewAPIServer(node, bindAddr, "", nilLogger(), nil, nil, false)

	ctx := context.Background()
	if err := apiServer.Start(ctx); err != nil {
		t.Fatalf("failed to start API server: %v", err)
	}
	defer apiServer.Stop()

	baseURL := "http://" + bindAddr

	req, _ := http.NewRequest("OPTIONS", baseURL+"/api/v1/status", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send OPTIONS request: %v", err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %s", origin)
	}

	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header to be set")
	}

	if resp.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers header to be set")
	}
}

func nilLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func init() {
	_ = url.QueryEscape("")
	_ = strings.Builder{}
}
