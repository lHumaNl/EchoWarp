package auth

import (
	"sync"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

const (
	// defaultMaxConnPerSecPerIP is the default rate limit for new connections per IP.
	defaultMaxConnPerSecPerIP = 5
	// ipCleanupInterval is the interval between cleanup of expired rate limit entries.
	ipCleanupInterval = 60 * time.Second
	// DefaultMaxEntries is the maximum number of IPs tracked before LRU eviction.
	DefaultMaxEntries = 10000
	// evictPercent is the percentage of entries to evict when limit is reached.
	evictPercent = 10
)

// IPRateLimiter implements a sliding window rate limiter for IP-based connection limiting.
// It tracks connection attempts per IP and enforces a maximum rate. Thread-safe for
// concurrent use.
//
// Memory Management:
//   - Maximum entries controlled by maxEntries (default 10000)
//   - Automatic LRU eviction when limit reached
//   - Periodic cleanup of expired entries every 60 seconds
//
// Lifecycle:
//
//	rl := NewIPRateLimiter(5) // 5 connections/sec per IP
//	defer rl.Close()           // Stops cleanup goroutine
type IPRateLimiter struct {
	mu         sync.Mutex
	entries    map[string][]time.Time
	maxRate    int
	window     time.Duration
	maxEntries int
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewIPRateLimiter creates a rate limiter with the specified max connections per second per IP.
// If maxPerSecond is <= 0, defaults to 5. Starts a background cleanup goroutine.
func NewIPRateLimiter(maxPerSecond int) *IPRateLimiter {
	if maxPerSecond <= 0 {
		maxPerSecond = defaultMaxConnPerSecPerIP
	}
	r := &IPRateLimiter{
		entries:    make(map[string][]time.Time),
		maxRate:    maxPerSecond,
		window:     time.Second,
		maxEntries: DefaultMaxEntries,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	go r.cleanupLoop()
	return r
}

// Allow checks if a connection from the given IP is permitted under the rate limit.
// Returns true if allowed, false if rate limit exceeded. Thread-safe.
func (r *IPRateLimiter) Allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.entries) >= r.maxEntries {
		r.evictOldest()
	}

	now := time.Now()
	cutoff := now.Add(-r.window)

	timestamps := r.entries[ip]
	n := 0
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			timestamps[n] = ts
			n++
		}
	}
	timestamps = timestamps[:n]

	if len(timestamps) >= r.maxRate {
		r.entries[ip] = timestamps
		metrics.RateLimiterRejections.Inc()
		return false
	}

	r.entries[ip] = append(timestamps, now)
	return true
}

func (r *IPRateLimiter) evictOldest() {
	now := time.Now()
	cutoff := now.Add(-r.window)

	for ip, timestamps := range r.entries {
		n := 0
		for _, ts := range timestamps {
			if ts.After(cutoff) {
				timestamps[n] = ts
				n++
			}
		}
		if n == 0 {
			delete(r.entries, ip)
		} else {
			r.entries[ip] = timestamps[:n]
		}
	}

	if len(r.entries) >= r.maxEntries {
		toRemove := len(r.entries) * evictPercent / 100
		if toRemove < 1 {
			toRemove = 1
		}
		oldestIPs := make([]string, 0, toRemove)
		for ip, timestamps := range r.entries {
			if len(timestamps) > 0 {
				oldestIPs = append(oldestIPs, ip)
				if len(oldestIPs) >= toRemove {
					break
				}
			}
		}
		for _, ip := range oldestIPs {
			delete(r.entries, ip)
		}
	}
}

// Cleanup removes expired entries from the rate limiter. Called automatically every 60 seconds.
func (r *IPRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	for ip, timestamps := range r.entries {
		n := 0
		for _, ts := range timestamps {
			if ts.After(cutoff) {
				timestamps[n] = ts
				n++
			}
		}
		if n == 0 {
			delete(r.entries, ip)
		} else {
			r.entries[ip] = timestamps[:n]
		}
	}
}

func (r *IPRateLimiter) cleanupLoop() {
	defer close(r.doneCh)
	ticker := time.NewTicker(ipCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.Cleanup()
		case <-r.stopCh:
			return
		}
	}
}

// Close stops the background cleanup goroutine. Must be called to prevent goroutine leak.
// Blocks until cleanup goroutine exits.
func (r *IPRateLimiter) Close() {
	close(r.stopCh)
	<-r.doneCh
}

// ExtractIP parses a remote address string (host:port format) and returns just the IP.
// If parsing fails, returns the original string unchanged.
func ExtractIP(remoteAddr string) string {
	return extractIP(remoteAddr)
}
