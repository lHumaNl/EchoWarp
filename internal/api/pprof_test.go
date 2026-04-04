package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func TestPprof_Enabled_Returns200(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "", logger, nil, nil, true)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "pprof")
}

func TestPprof_Disabled_Returns404(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "", logger, nil, nil, false)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestPprof_AuthWithToken_RequiresValidToken(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "secret-token", logger, nil, nil, true)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	t.Run("no_token_returns_401", func(t *testing.T) {
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid_token_returns_401", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://"+actualAddr+"/debug/pprof/", http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer wrong-token")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("valid_token_returns_200", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://"+actualAddr+"/debug/pprof/", http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer secret-token")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestPprof_IndividualEndpoints_Work(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "", logger, nil, nil, true)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	endpoints := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/symbol",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := client.Get("http://" + actualAddr + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestPprof_HeapProfile_Works(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "", logger, nil, nil, true)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("http://" + actualAddr + "/debug/pprof/heap")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, len(body) > 0, "heap profile should not be empty")
}

func TestPprof_GoroutineProfile_Works(t *testing.T) {
	ctx := context.Background()
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	actualAddr := listener.Addr().String()
	listener.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	apiServer := NewAPIServer(node, actualAddr, "", logger, nil, nil, true)

	err = apiServer.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { apiServer.Stop() })

	// Wait for server to be ready to accept connections
	require.Eventually(t, func() bool {
		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Get("http://" + actualAddr + "/debug/pprof/")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return true
	}, 2*time.Second, 10*time.Millisecond, "server should be ready")

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get("http://" + actualAddr + "/debug/pprof/goroutine")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, len(body) > 0, "goroutine profile should not be empty")
}
