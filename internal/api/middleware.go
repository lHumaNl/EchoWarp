package api

import (
	"crypto/hmac"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// authMiddleware validates authentication for API requests.
// If a token is configured, requires Bearer token in Authorization header.
// If no token is configured, restricts access to localhost only.
// publicPaths are endpoints that bypass authentication.
// Clients probe these before knowing the password.
var publicPaths = map[string]bool{
	"/healthz":             true,
	"/readyz":              true,
	"/api/v1/session/info": true,
}

func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		if s.token != "" {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if !hmac.Equal([]byte(token), []byte(s.token)) {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
		} else {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if host != "127.0.0.1" && host != "::1" && host != "localhost" {
				writeError(w, http.StatusUnauthorized, "access denied from remote address")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs all HTTP requests with method, path, status, and duration.
func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration,
			"remote", r.RemoteAddr,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before delegating to the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write ensures statusCode is set if WriteHeader was not called.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// maxRequestBodySize limits request bodies to 1MB to prevent DoS attacks.
const maxRequestBodySize = 1 << 20

// CORSMiddleware handles Cross-Origin Resource Sharing for browser clients.
// By default, allows localhost origins. Use "*" to allow all origins (insecure).
type CORSMiddleware struct {
	allowedOrigins map[string]bool
	allowAll       bool
}

// NewCORSMiddleware creates a CORS middleware with the specified allowed origins.
// If empty, defaults to localhost only. "*" allows all origins.
func NewCORSMiddleware(allowedOrigins []string) *CORSMiddleware {
	m := &CORSMiddleware{
		allowedOrigins: make(map[string]bool),
	}
	if len(allowedOrigins) == 0 {
		m.allowedOrigins["http://localhost"] = true
		m.allowedOrigins["http://127.0.0.1"] = true
		return m
	}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			m.allowAll = true
			break
		}
		m.allowedOrigins[origin] = true
	}
	return m
}

// Middleware returns an HTTP middleware that adds CORS headers.
// Handles preflight OPTIONS requests automatically.
func (m *CORSMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowed := m.allowAll
		if !allowed && origin != "" {
			if m.allowedOrigins[origin] {
				allowed = true
			} else if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
				allowed = true
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// bodyLimitMiddleware limits request body size to prevent memory exhaustion.
func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

// APIRateLimiter implements a sliding window rate limiter for API requests.
// Limits requests per IP within a time window.
type APIRateLimiter struct {
	mu       sync.Mutex
	requests map[string]*rateLimiterEntry
	limit    int
	window   time.Duration
	checker  *trustedProxyChecker
}

type rateLimiterEntry struct {
	count   int
	resetAt time.Time
}

// NewAPIRateLimiter creates a rate limiter with the specified limit and window.
func NewAPIRateLimiter(limit int, window time.Duration) *APIRateLimiter {
	return &APIRateLimiter{
		requests: make(map[string]*rateLimiterEntry),
		limit:    limit,
		window:   window,
	}
}

// NewAPIRateLimiterWithChecker creates a rate limiter with trusted proxy support.
func NewAPIRateLimiterWithChecker(limit int, window time.Duration, checker *trustedProxyChecker) *APIRateLimiter {
	return &APIRateLimiter{
		requests: make(map[string]*rateLimiterEntry),
		limit:    limit,
		window:   window,
		checker:  checker,
	}
}

// Middleware returns an HTTP middleware that enforces rate limiting.
// Sets X-RateLimit-* headers. Returns 429 when limit exceeded.
func (rl *APIRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIPWithChecker(r, rl.checker)

		rl.mu.Lock()
		entry, exists := rl.requests[ip]
		now := time.Now()

		if !exists || now.After(entry.resetAt) {
			entry = &rateLimiterEntry{
				count:   0,
				resetAt: now.Add(rl.window),
			}
			rl.requests[ip] = entry
		}

		entry.count++
		remaining := rl.limit - entry.count
		rl.mu.Unlock()

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(0, remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(entry.resetAt.Unix(), 10))

		if entry.count > rl.limit {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// trustedProxyChecker checks if a remote IP is in the trusted proxies list.
// Supports IP addresses, CIDR notation, and the special "localhost" value.
type trustedProxyChecker struct {
	nets []*net.IPNet
	ips  map[string]bool
}

// newTrustedProxyChecker creates a checker from a list of trusted proxy specifications.
// Each spec can be:
//   - "localhost" - matches 127.0.0.1 and ::1
//   - An IP address (e.g., "192.168.1.1")
//   - A CIDR range (e.g., "10.0.0.0/8")
func newTrustedProxyChecker(specs []string) *trustedProxyChecker {
	c := &trustedProxyChecker{
		ips: make(map[string]bool),
	}

	for _, spec := range specs {
		switch {
		case spec == "localhost":
			c.ips["127.0.0.1"] = true
			c.ips["::1"] = true
		case strings.Contains(spec, "/"):
			// CIDR notation
			_, ipNet, err := net.ParseCIDR(spec)
			if err == nil {
				c.nets = append(c.nets, ipNet)
			}
		default:
			// Plain IP address
			ip := net.ParseIP(spec)
			if ip != nil {
				c.ips[ip.String()] = true
			}
		}
	}

	return c
}

// isTrusted checks if the given IP address is in the trusted list.
func (c *trustedProxyChecker) isTrusted(remoteIP string) bool {
	// Check direct IP match
	if c.ips[remoteIP] {
		return true
	}

	// Parse the remote IP for CIDR matching
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}

	// Check CIDR ranges
	for _, ipNet := range c.nets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// extractClientIP extracts the real client IP from a request, handling X-Forwarded-For
// only when the request comes from a trusted proxy.
func extractClientIPWithChecker(r *http.Request, checker *trustedProxyChecker) string {
	if checker != nil && r.Header.Get("X-Forwarded-For") != "" {
		remoteIP := extractIP(r.RemoteAddr)
		if checker.isTrusted(remoteIP) {
			xff := r.Header.Get("X-Forwarded-For")
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return extractIP(r.RemoteAddr)
}

// extractIP parses host:port and returns just the IP portion.
func extractIP(remoteAddr string) string {
	ip, _, _ := net.SplitHostPort(remoteAddr) //nolint:errcheck
	if ip == "" {
		return remoteAddr
	}
	return ip
}

// SecurityHeadersMiddleware adds OWASP-recommended security headers to all responses:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - X-XSS-Protection: 1; mode=block
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: restrictive defaults
//   - Content-Security-Policy: default-src 'self'
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}
