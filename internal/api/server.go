package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

// ServerOption configures an APIServer during construction.
type ServerOption func(*APIServer)

// APIServer provides a REST API and WebSocket endpoint for remote EchoWarp control.
// It wraps a Node and exposes its functionality via HTTP endpoints.
//
// Endpoints:
//
//	GET  /healthz              - Health check (always returns 200)
//	GET  /readyz               - Readiness check (200 if node ready, 503 otherwise)
//	GET  /api/v1/session/info  - Public server capabilities (no auth required)
//	GET  /api/v1/status        - Current node status and uptime
//	GET  /api/v1/version       - Version, commit, and build info
//	GET  /api/v1/stats         - Connection statistics
//	GET  /api/v1/devices       - Available audio devices
//	GET  /api/v1/clients       - Connected clients (server mode)
//	GET  /api/v1/config        - Current configuration (secrets redacted)
//	POST /api/v1/config        - Update configuration
//	POST /api/v1/start         - Start streaming
//	POST /api/v1/stop          - Stop streaming
//	POST /api/v1/pause         - Pause streaming
//	POST /api/v1/resume        - Resume streaming
//	POST /api/v1/shutdown      - Graceful shutdown
//	POST /api/v1/discover      - Trigger mDNS discovery and return results
//	POST /api/v1/devices/{id}/mute   - Toggle device mute (stub — 501)
//	POST /api/v1/devices/{id}/volume - Set device volume (stub — 501)
//	GET  /api/v1/conference/participants              - List participants (stub — 501)
//	POST /api/v1/conference/participants/{id}/mute   - Mute participant (stub — 501)
//	POST /api/v1/conference/participants/{id}/kick   - Kick participant (stub — 501)
//	POST /api/v1/conference/participants/{id}/volume - Set participant volume (stub — 501)
//	GET  /ws/v1/events         - WebSocket event stream
//	GET  /metrics              - Prometheus metrics
//
// Security:
//   - Bearer token authentication (if token configured)
//   - Localhost-only access (if no token configured)
//   - Rate limiting: 100 requests/minute per IP
//   - Request body limit: 1MB
//   - Security headers (OWASP recommended)
type APIServer struct {
	node                *echowarp.Node
	bindAddr            string
	token               string
	server              *http.Server
	logger              *slog.Logger
	wsHub               *WSHub
	cors                *CORSMiddleware
	rateLimiter         *APIRateLimiter
	corsOrigins         []string
	wsAllowedOrigins    []string
	enablePprof         bool
	trustedProxyChecker *trustedProxyChecker
}

// WithAllowedOrigins sets custom WebSocket origin patterns.
// If not called, defaults to localhost-only patterns for security.
// Use this to allow connections from specific domains in production deployments.
//
// Example:
//
//	server := NewAPIServer(node, addr, token, logger,
//	    WithAllowedOrigins([]string{"https://example.com", "https://app.example.com:*"}),
//	)
func WithAllowedOrigins(origins []string) ServerOption {
	return func(s *APIServer) {
		s.wsAllowedOrigins = origins
	}
}

// WithCORSOrigins sets custom CORS origins for the API server.
// If not called, defaults to localhost-only for security.
func WithCORSOrigins(origins []string) ServerOption {
	return func(s *APIServer) {
		s.corsOrigins = origins
	}
}

// WithPprof enables pprof endpoints for profiling.
func WithPprof(enable bool) ServerOption {
	return func(s *APIServer) {
		s.enablePprof = enable
	}
}

// WithTrustedProxies sets the list of trusted proxy IPs/CIDRs for X-Forwarded-For processing.
// When set, requests from these IPs will have their X-Forwarded-For header processed to
// extract the real client IP for rate limiting and logging.
//
// Special values:
//   - "localhost" - trusts 127.0.0.1 and ::1
//   - CIDR notation (e.g., "10.0.0.0/8") - trusts entire network range
//   - Plain IP (e.g., "192.168.1.100") - trusts specific IP
//
// If not called, X-Forwarded-For is ignored (no proxies trusted).
func WithTrustedProxies(proxies []string) ServerOption {
	return func(s *APIServer) {
		s.trustedProxyChecker = newTrustedProxyChecker(proxies)
	}
}

// NewAPIServer creates an API server for the given node.
// bindAddr specifies the listen address (e.g., "127.0.0.1:8080").
// token enables bearer token authentication; if empty, localhost-only access is enforced.
// corsOrigins specifies allowed CORS origins; empty defaults to localhost.
func NewAPIServer(node *echowarp.Node, bindAddr string, token string, logger *slog.Logger, corsOrigins []string, wsAllowedOrigins []string, enablePprof bool) *APIServer {
	return &APIServer{
		node:             node,
		bindAddr:         bindAddr,
		token:            token,
		logger:           logger,
		wsHub:            NewWSHub(logger),
		corsOrigins:      corsOrigins,
		wsAllowedOrigins: wsAllowedOrigins,
		enablePprof:      enablePprof,
	}
}

// NewAPIServerWithOptions creates an API server with functional options.
// This is the recommended way to create an APIServer as it provides better
// extensibility and clearer configuration.
//
// Basic usage:
//
//	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "secret-token", logger)
//
// With custom origins:
//
//	server := NewAPIServerWithOptions(node, "127.0.0.1:8080", "secret-token", logger,
//	    WithAllowedOrigins([]string{"https://example.com"}),
//	    WithCORSOrigins([]string{"https://example.com"}),
//	    WithPprof(true),
//	)
func NewAPIServerWithOptions(node *echowarp.Node, bindAddr string, token string, logger *slog.Logger, opts ...ServerOption) *APIServer {
	server := &APIServer{
		node:     node,
		bindAddr: bindAddr,
		token:    token,
		logger:   logger,
		wsHub:    NewWSHub(logger),
	}

	// Apply all options
	for _, opt := range opts {
		opt(server)
	}

	return server
}

// WSHub returns the WebSocket hub used for broadcasting events to clients.
// Exposed so external components (such as the EventBus → WS bridge) can push
// events without going through an HTTP round trip.
func (s *APIServer) WSHub() *WSHub {
	return s.wsHub
}

// serverID returns the server UUID for the listening port, or "" if not yet generated.
func (s *APIServer) serverID() string {
	_, portStr, err := net.SplitHostPort(s.bindAddr)
	if err != nil {
		return ""
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ""
	}
	return config.ServerID(port)
}

func (s *APIServer) initMiddleware() {
	s.cors = NewCORSMiddleware(s.corsOrigins)
	s.rateLimiter = NewAPIRateLimiterWithChecker(100, time.Minute, s.trustedProxyChecker)
}

// Start begins listening for HTTP requests. It returns after the server
// is ready to accept connections, or immediately if an error occurs.
// Use Stop() for graceful shutdown.
func (s *APIServer) Start(ctx context.Context) error {
	s.initMiddleware()

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	handler := s.buildHandlerChain(mux)
	s.server = s.createHTTPServer(handler)

	return s.startListening()
}

// registerRoutes registers all API routes on the given mux.
func (s *APIServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("GET /api/v1/session/info", s.handleSessionInfo)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/version", s.handleVersion)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/devices", s.handleDevices)
	mux.HandleFunc("GET /api/v1/clients", s.handleClients)
	mux.HandleFunc("GET /api/v1/config", s.handleConfig)
	mux.HandleFunc("POST /api/v1/config", s.handleConfig)
	mux.HandleFunc("PUT /api/v1/config", s.handleConfig)
	mux.HandleFunc("POST /api/v1/start", s.handleStart)
	mux.HandleFunc("POST /api/v1/stop", s.handleStop)
	mux.HandleFunc("POST /api/v1/pause", s.handlePause)
	mux.HandleFunc("POST /api/v1/resume", s.handleResume)
	mux.HandleFunc("POST /api/v1/shutdown", s.handleShutdown)
	mux.HandleFunc("POST /api/v1/discover", s.handleDiscover)
	mux.HandleFunc("POST /api/v1/devices/{id}/mute", s.handleDeviceMute)
	mux.HandleFunc("POST /api/v1/devices/{id}/volume", s.handleDeviceVolume)
	mux.HandleFunc("GET /api/v1/conference/participants", s.handleConferenceParticipants)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/mute", s.handleConferenceParticipantMute)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/kick", s.handleConferenceParticipantKick)
	mux.HandleFunc("POST /api/v1/conference/participants/{id}/volume", s.handleConferenceParticipantVolume)
	mux.HandleFunc("GET /ws/v1/events", s.handleWebSocket)
	mux.Handle("GET /metrics", promhttp.Handler())

	if s.enablePprof {
		s.registerPprofRoutes(mux)
	}
}

// registerPprofRoutes registers pprof profiling endpoints.
func (s *APIServer) registerPprofRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	mux.Handle("GET /debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("GET /debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("GET /debug/pprof/block", pprof.Handler("block"))
	mux.Handle("GET /debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("GET /debug/pprof/mutex", pprof.Handler("mutex"))
}

// buildHandlerChain creates the middleware chain wrapping the mux.
func (s *APIServer) buildHandlerChain(mux *http.ServeMux) http.Handler {
	return SecurityHeadersMiddleware(
		s.loggingMiddleware(
			s.cors.Middleware(
				s.rateLimiter.Middleware(
					bodyLimitMiddleware(
						s.authMiddleware(mux))))))
}

// createHTTPServer creates the HTTP server with standard timeouts.
func (s *APIServer) createHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         s.bindAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// startListening binds the listener first, then serves in a goroutine.
// Returns only after the listener is ready (or on bind error).
func (s *APIServer) startListening() error {
	ln, err := net.Listen("tcp", s.bindAddr) //nolint:noctx
	if err != nil {
		return fmt.Errorf("api server listen: %w", err)
	}

	s.logger.Info("API server started", "addr", ln.Addr().String())

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("API server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the server with a 5-second timeout.
// Also closes all WebSocket connections.
func (s *APIServer) Stop() error {
	if s.server == nil {
		return nil
	}
	s.wsHub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.logger.Info("API server stopping")
	return s.server.Shutdown(ctx)
}
