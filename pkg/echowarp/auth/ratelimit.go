// Package auth provides authentication mechanisms for EchoWarp.
// This file implements rate limiting for authentication attempts.
package auth

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// RateLimiterConfig holds configuration for the rate limiter.
type RateLimiterConfig struct {
	// MaxAttempts is the maximum number of authentication attempts allowed per window.
	MaxAttempts int

	// Window is the time window for rate limiting.
	Window time.Duration

	// CleanupInterval is how often to clean up expired entries.
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig returns the default rate limiter configuration.
// Default: 5 attempts per minute per IP.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// RateLimiter implements per-IP rate limiting for authentication attempts.
// Uses a sliding window algorithm for accurate rate limiting.
//
// Thread-safe: Internal state protected by sync.RWMutex.
type RateLimiter struct {
	config  RateLimiterConfig
	entries map[string]*rateLimitEntry
	mu      sync.RWMutex
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// rateLimitEntry tracks authentication attempts for a single IP.
type rateLimitEntry struct {
	attempts  []time.Time
	blockedAt time.Time
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:  config,
		entries: make(map[string]*rateLimitEntry),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	rl.wg.Add(1)
	go rl.cleanupLoop()

	return rl
}

// CheckAndRecord checks if an authentication attempt is allowed and records it.
// Returns an error if the rate limit is exceeded.
// The ipAddr should be the remote IP address (e.g., "192.168.1.1" or "192.168.1.1:12345").
func (rl *RateLimiter) CheckAndRecord(ipAddr string) error {
	// Extract IP from address (handle both "ip" and "ip:port" formats)
	ip := extractIP(ipAddr)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]
	if !exists {
		entry = &rateLimitEntry{
			attempts: make([]time.Time, 0, rl.config.MaxAttempts),
		}
		rl.entries[ip] = entry
	}

	// Remove expired attempts (sliding window)
	windowStart := now.Add(-rl.config.Window)
	validAttempts := make([]time.Time, 0, len(entry.attempts))
	for _, t := range entry.attempts {
		if t.After(windowStart) {
			validAttempts = append(validAttempts, t)
		}
	}
	entry.attempts = validAttempts

	// Check if blocked
	if !entry.blockedAt.IsZero() && now.Sub(entry.blockedAt) < rl.config.Window {
		remaining := rl.config.Window - now.Sub(entry.blockedAt)
		return &RateLimitError{
			IP:          ip,
			MaxAttempts: rl.config.MaxAttempts,
			Window:      rl.config.Window,
			Remaining:   remaining,
		}
	}

	// Check rate limit (before recording)
	if len(entry.attempts) >= rl.config.MaxAttempts {
		entry.blockedAt = now
		remaining := rl.config.Window
		return &RateLimitError{
			IP:          ip,
			MaxAttempts: rl.config.MaxAttempts,
			Window:      rl.config.Window,
			Remaining:   remaining,
		}
	}

	// Record attempt
	entry.attempts = append(entry.attempts, now)

	// Mark as blocked if we just reached the limit
	if len(entry.attempts) >= rl.config.MaxAttempts {
		entry.blockedAt = now
	}

	return nil
}

// RecordFailure records a failed authentication attempt.
// This is an alias for CheckAndRecord when you only want to record without checking.
func (rl *RateLimiter) RecordFailure(ipAddr string) {
	ip := extractIP(ipAddr)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]
	if !exists {
		entry = &rateLimitEntry{
			attempts: make([]time.Time, 0, rl.config.MaxAttempts),
		}
		rl.entries[ip] = entry
	}

	// Remove expired attempts
	windowStart := now.Add(-rl.config.Window)
	validAttempts := make([]time.Time, 0, len(entry.attempts))
	for _, t := range entry.attempts {
		if t.After(windowStart) {
			validAttempts = append(validAttempts, t)
		}
	}
	entry.attempts = validAttempts
	entry.attempts = append(entry.attempts, now)

	// Check if should block
	if len(entry.attempts) >= rl.config.MaxAttempts {
		entry.blockedAt = now
	}
}

// Clear removes the rate limit entry for the given IP.
// Useful after successful authentication.
func (rl *RateLimiter) Clear(ipAddr string) {
	ip := extractIP(ipAddr)

	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, ip)
}

// IsBlocked checks if an IP is currently blocked without recording an attempt.
func (rl *RateLimiter) IsBlocked(ipAddr string) bool {
	ip := extractIP(ipAddr)

	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.entries[ip]
	if !exists {
		return false
	}

	if entry.blockedAt.IsZero() {
		return false
	}

	return time.Since(entry.blockedAt) < rl.config.Window
}

// RemainingAttempts returns the number of remaining attempts for an IP.
func (rl *RateLimiter) RemainingAttempts(ipAddr string) int {
	ip := extractIP(ipAddr)

	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.entries[ip]
	if !exists {
		return rl.config.MaxAttempts
	}

	// Count valid attempts
	now := time.Now()
	windowStart := now.Add(-rl.config.Window)
	count := 0
	for _, t := range entry.attempts {
		if t.After(windowStart) {
			count++
		}
	}

	remaining := rl.config.MaxAttempts - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.wg.Wait()
}

// cleanupLoop periodically removes expired entries.
func (rl *RateLimiter) cleanupLoop() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes expired entries.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.config.Window)

	for ip, entry := range rl.entries {
		// Remove expired attempts
		validAttempts := make([]time.Time, 0, len(entry.attempts))
		for _, t := range entry.attempts {
			if t.After(windowStart) {
				validAttempts = append(validAttempts, t)
			}
		}

		// Remove entry if no valid attempts and not blocked
		if len(validAttempts) == 0 && (entry.blockedAt.IsZero() || entry.blockedAt.Before(windowStart)) {
			delete(rl.entries, ip)
		} else {
			entry.attempts = validAttempts
		}
	}
}

// extractIP extracts the IP address from an address string.
// Handles both "ip" and "ip:port" formats.
func extractIP(addr string) string {
	// Try to parse as IP:port
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Not in IP:port format, assume it's just an IP
		return addr
	}
	return host
}

// RateLimitError represents a rate limit exceeded error.
type RateLimitError struct {
	IP          string
	MaxAttempts int
	Window      time.Duration
	Remaining   time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf(
		"rate limit exceeded for %s: %d attempts allowed per %v, try again in %v",
		e.IP,
		e.MaxAttempts,
		e.Window,
		e.Remaining.Round(time.Second),
	)
}

// Is implements the interface for errors.Is() compatibility.
func (e *RateLimitError) Is(target error) bool {
	_, ok := target.(*RateLimitError)
	return ok
}
