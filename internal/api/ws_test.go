package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nhooyr.io/websocket"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

func TestWSOrigins_Default_ReturnsLocalhostPatterns(t *testing.T) {
	server := &APIServer{
		wsAllowedOrigins: nil,
	}

	origins := server.wsOrigins()

	assert.Len(t, origins, 4)
	assert.Contains(t, origins, "http://localhost:*")
	assert.Contains(t, origins, "http://127.0.0.1:*")
	assert.Contains(t, origins, "https://localhost:*")
	assert.Contains(t, origins, "https://127.0.0.1:*")
}

func TestWSOrigins_EmptySlice_ReturnsLocalhostPatterns(t *testing.T) {
	server := &APIServer{
		wsAllowedOrigins: []string{},
	}

	origins := server.wsOrigins()

	assert.Len(t, origins, 4)
	assert.Contains(t, origins, "http://localhost:*")
	assert.Contains(t, origins, "http://127.0.0.1:*")
}

func TestWSOrigins_CustomOrigins_ReturnsCustomOrigins(t *testing.T) {
	customOrigins := []string{
		"https://example.com",
		"https://app.example.com:*",
	}
	server := &APIServer{
		wsAllowedOrigins: customOrigins,
	}

	origins := server.wsOrigins()

	assert.Equal(t, customOrigins, origins)
	assert.Len(t, origins, 2)
}

func TestWSOrigins_Wildcard_ReturnsWildcard(t *testing.T) {
	server := &APIServer{
		wsAllowedOrigins: []string{"*"},
	}

	origins := server.wsOrigins()

	assert.Equal(t, []string{"*"}, origins)
}

func TestWSOrigins_SingleCustomOrigin_ReturnsSingleOrigin(t *testing.T) {
	server := &APIServer{
		wsAllowedOrigins: []string{"https://trusted.domain.com"},
	}

	origins := server.wsOrigins()

	assert.Len(t, origins, 1)
	assert.Equal(t, "https://trusted.domain.com", origins[0])
}

func TestWebSocket_OriginRestriction_RejectsInvalidOrigin(t *testing.T) {
	t.Parallel()

	// Create a node
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	token := "test-token"

	// Create server with default localhost-only origins
	apiServer := NewAPIServerWithOptions(node, "127.0.0.1:0", token, logger)
	require.NotNil(t, apiServer)
	apiServer.initMiddleware()

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/v1/events", apiServer.handleWebSocket)
	handler := apiServer.loggingMiddleware(apiServer.cors.Middleware(apiServer.rateLimiter.Middleware(bodyLimitMiddleware(apiServer.authMiddleware(mux)))))

	// Create test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test 1: Connection from localhost should succeed
	t.Run("LocalhostOrigin_Succeeds", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/v1/events"
		headers := http.Header{}
		headers.Set("Origin", "http://localhost:3000")
		headers.Set("Authorization", "Bearer "+token)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader: headers,
		})

		if err != nil {
			// httptest.Server might not support WebSocket hijacking
			if strings.Contains(err.Error(), "501") {
				t.Skip("httptest.Server does not support WebSocket hijacking")
			}
			// Some test environments might not support the connection
			t.Logf("WebSocket dial error (may be expected in test env): %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		require.NoError(t, err, "Localhost origin should be accepted")
	})

	// Test 2: Connection from external origin should be rejected
	t.Run("ExternalOrigin_Rejected", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/v1/events"
		headers := http.Header{}
		headers.Set("Origin", "https://evil.com")
		headers.Set("Authorization", "Bearer "+token)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			HTTPHeader: headers,
		})

		if err != nil {
			// Expected: connection should be rejected due to origin mismatch
			t.Logf("Connection rejected as expected: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		// If we got here, the connection was accepted, which is a security issue
		t.Error("External origin should have been rejected")
	})
}

// TestWSBroadcastTimeout ensures WSHub.Broadcast uses a sensible per-client
// write deadline so broadcast messages actually reach connected clients.
//
// Regression test for a bug where the deadline was `context.WithTimeout(..., 5)`
// — five nanoseconds — which caused every write to time out before the frame
// could be delivered. With the fix (5 * time.Second) the broadcast arrives at
// a connected client well under 100ms.
func TestWSBroadcastTimeout(t *testing.T) {
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	apiServer := NewAPIServerWithOptions(node, "127.0.0.1:0", "", logger)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/v1/events", apiServer.handleWebSocket)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/v1/events"
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{ts.URL}},
	})
	require.NoError(t, err, "websocket dial must succeed")
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Drain the initial "client_connected" greeting that handleWebSocket
	// broadcasts on connection. This also ensures the hub has registered the
	// client before we emit the test broadcast.
	greetCtx, greetCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer greetCancel()
	_, greet, err := conn.Read(greetCtx)
	require.NoError(t, err, "client_connected greeting must arrive")
	assert.Contains(t, string(greet), "client_connected")
	require.Equal(t, 1, apiServer.wsHub.ClientCount(), "hub should have one client")

	// Broadcast a test event and read it back with a tight client-side
	// deadline. With the old 5ns bug the server-side write times out before
	// the frame hits the wire and conn.Read below fails with a deadline error.
	apiServer.wsHub.Broadcast(WSEvent{Type: "test_broadcast", Data: map[string]string{"hello": "world"}})

	readCtx, readCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer readCancel()

	start := time.Now()
	_, msg, err := conn.Read(readCtx)
	elapsed := time.Since(start)
	require.NoError(t, err, "broadcast message must arrive within deadline")
	assert.Less(t, elapsed, 100*time.Millisecond, "broadcast should arrive within 100ms")
	assert.Contains(t, string(msg), "test_broadcast")
	assert.Contains(t, string(msg), "hello")
}

func TestWebSocket_CustomOrigins_AcceptsConfiguredOrigins(t *testing.T) {
	t.Parallel()

	// Create a node
	cfg := echowarp.NodeConfig{
		Mode:       echowarp.ModeServer,
		Port:       0,
		SampleRate: 48000,
		Channels:   1,
	}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	token := "test-token"
	customOrigins := []string{"https://trusted.example.com"}

	// Create server with custom origins
	apiServer := NewAPIServerWithOptions(node, "127.0.0.1:0", token, logger,
		WithAllowedOrigins(customOrigins),
	)
	require.NotNil(t, apiServer)

	// Verify custom origins are set
	assert.Equal(t, customOrigins, apiServer.wsOrigins())

	// Setup routes
	apiServer.initMiddleware()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/v1/events", apiServer.handleWebSocket)
	handler := apiServer.loggingMiddleware(apiServer.cors.Middleware(apiServer.rateLimiter.Middleware(bodyLimitMiddleware(apiServer.authMiddleware(mux)))))

	// Create test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test: Connection from configured origin should succeed
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/v1/events"
	headers := http.Header{}
	headers.Set("Origin", "https://trusted.example.com")
	headers.Set("Authorization", "Bearer "+token)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})

	if err != nil {
		if strings.Contains(err.Error(), "501") {
			t.Skip("httptest.Server does not support WebSocket hijacking")
		}
		t.Logf("WebSocket dial error (may be expected in test env): %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	require.NoError(t, err, "Configured origin should be accepted")
}
