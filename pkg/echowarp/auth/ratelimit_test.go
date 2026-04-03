package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_DefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultRateLimiterConfig()
	assert.Equal(t, 5, config.MaxAttempts)
	assert.Equal(t, time.Minute, config.Window)
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
}

func TestRateLimiter_CheckAndRecord_AllowsWithinLimit(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     3,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Should allow up to MaxAttempts
	for i := 0; i < config.MaxAttempts; i++ {
		err := rl.CheckAndRecord("192.168.1.1")
		assert.NoError(t, err, "attempt %d should be allowed", i+1)
	}

	// Next attempt should be blocked
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestRateLimiter_CheckAndRecord_BlocksAfterLimit(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          100 * time.Millisecond,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Exhaust attempts
	_ = rl.CheckAndRecord("10.0.0.1")
	_ = rl.CheckAndRecord("10.0.0.1")

	// Should be blocked
	err := rl.CheckAndRecord("10.0.0.1")
	require.Error(t, err)

	var rateLimitErr *RateLimitError
	require.True(t, errors.As(err, &rateLimitErr))
	assert.Equal(t, "10.0.0.1", rateLimitErr.IP)
	assert.Equal(t, 2, rateLimitErr.MaxAttempts)
}

func TestRateLimiter_CheckAndRecord_AllowsAfterWindow(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          50 * time.Millisecond,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Exhaust attempts
	_ = rl.CheckAndRecord("172.16.0.1")
	_ = rl.CheckAndRecord("172.16.0.1")

	// Should be blocked
	err := rl.CheckAndRecord("172.16.0.1")
	require.Error(t, err)

	// Use require.Eventually to wait for window to expire
	require.Eventually(t, func() bool {
		return rl.CheckAndRecord("172.16.0.1") == nil
	}, 200*time.Millisecond, 10*time.Millisecond, "Should be allowed after window expires")
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// IP 1: exhaust attempts
	_ = rl.CheckAndRecord("192.168.1.1")
	_ = rl.CheckAndRecord("192.168.1.1")
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)

	// IP 2: should still be allowed
	err = rl.CheckAndRecord("192.168.1.2")
	assert.NoError(t, err)
	err = rl.CheckAndRecord("192.168.1.2")
	assert.NoError(t, err)
	err = rl.CheckAndRecord("192.168.1.2")
	require.Error(t, err)

	// IP 3: should still be allowed
	err = rl.CheckAndRecord("10.0.0.1")
	assert.NoError(t, err)
}

func TestRateLimiter_IPWithPort(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Record with IP:port format
	_ = rl.CheckAndRecord("192.168.1.1:12345")
	_ = rl.CheckAndRecord("192.168.1.1:54321")

	// Same IP with different port should be blocked
	err := rl.CheckAndRecord("192.168.1.1:99999")
	require.Error(t, err)
}

func TestRateLimiter_RemainingAttempts(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Initially all attempts available
	assert.Equal(t, 5, rl.RemainingAttempts("192.168.1.1"))

	// After one attempt
	_ = rl.CheckAndRecord("192.168.1.1")
	assert.Equal(t, 4, rl.RemainingAttempts("192.168.1.1"))

	// After two more attempts
	_ = rl.CheckAndRecord("192.168.1.1")
	_ = rl.CheckAndRecord("192.168.1.1")
	assert.Equal(t, 2, rl.RemainingAttempts("192.168.1.1"))
}

func TestRateLimiter_IsBlocked(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          500 * time.Millisecond, // Increased window for reliable testing
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Initially not blocked
	assert.False(t, rl.IsBlocked("192.168.1.1"))

	// After first attempt - not blocked yet
	_ = rl.CheckAndRecord("192.168.1.1")
	assert.False(t, rl.IsBlocked("192.168.1.1"))

	// After second attempt - blocked because we reached the limit
	_ = rl.CheckAndRecord("192.168.1.1")
	// Use require.Eventually to wait for blocked state to be set (handles timing)
	require.Eventually(t, func() bool {
		return rl.IsBlocked("192.168.1.1")
	}, 100*time.Millisecond, 10*time.Millisecond, "Should be blocked after reaching limit")

	// Third attempt should be rejected (already blocked)
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)

	// Use require.Eventually to wait for window to expire
	require.Eventually(t, func() bool {
		return !rl.IsBlocked("192.168.1.1")
	}, 700*time.Millisecond, 10*time.Millisecond, "Should be unblocked after window expires")
}

func TestRateLimiter_Clear(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Exhaust attempts
	_ = rl.CheckAndRecord("192.168.1.1")
	_ = rl.CheckAndRecord("192.168.1.1")
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)

	// Clear the entry
	rl.Clear("192.168.1.1")

	// Should be allowed again
	err = rl.CheckAndRecord("192.168.1.1")
	assert.NoError(t, err)
}

func TestRateLimiter_RecordFailure(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Record failures without checking
	rl.RecordFailure("192.168.1.1")
	rl.RecordFailure("192.168.1.1")

	// Should be blocked
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)
}

func TestRateLimiter_Cleanup(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     2,
		Window:          50 * time.Millisecond,
		CleanupInterval: 30 * time.Millisecond,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Record some attempts
	_ = rl.CheckAndRecord("192.168.1.1")
	_ = rl.CheckAndRecord("192.168.1.2")

	// Use require.Eventually to wait for cleanup and window to expire
	require.Eventually(t, func() bool {
		rl.cleanup()
		rl.mu.RLock()
		entryCount := len(rl.entries)
		rl.mu.RUnlock()
		return entryCount == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Entries should be cleared after cleanup")
}

func TestRateLimiter_SlidingWindow(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     3,
		Window:          100 * time.Millisecond,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Make 2 attempts
	_ = rl.CheckAndRecord("192.168.1.1")
	_ = rl.CheckAndRecord("192.168.1.1")

	// Use require.Eventually to wait for partial window expiry (1 attempt should be within limit)
	require.Eventually(t, func() bool {
		// Third attempt should succeed (old attempts expired)
		return rl.CheckAndRecord("192.168.1.1") == nil
	}, 200*time.Millisecond, 10*time.Millisecond, "Third attempt should succeed after partial window expires")

	// Fourth should fail (3 per window)
	err := rl.CheckAndRecord("192.168.1.1")
	require.Error(t, err)

	// Use require.Eventually to wait for more attempts to expire
	require.Eventually(t, func() bool {
		// Should have 2 or more remaining (most attempts expired)
		return rl.RemainingAttempts("192.168.1.1") >= 2
	}, 200*time.Millisecond, 10*time.Millisecond, "Should have remaining attempts after window expires")
}

func TestRateLimitError_Error(t *testing.T) {
	t.Parallel()

	err := &RateLimitError{
		IP:          "192.168.1.1",
		MaxAttempts: 5,
		Window:      time.Minute,
		Remaining:   30 * time.Second,
	}

	assert.Contains(t, err.Error(), "192.168.1.1")
	assert.Contains(t, err.Error(), "5")
	assert.Contains(t, err.Error(), "30s")
}

func TestRateLimitError_Is(t *testing.T) {
	t.Parallel()

	err1 := &RateLimitError{IP: "192.168.1.1"}
	err2 := &RateLimitError{IP: "10.0.0.1"}

	// errors.Is should match RateLimitError types
	assert.True(t, errors.Is(err1, &RateLimitError{}))
	assert.True(t, errors.Is(err2, &RateLimitError{}))
	assert.False(t, errors.Is(err1, errors.New("other error")))
}

func TestExtractIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.1.1:8080", "192.168.1.1"},
		{"[::1]:8080", "::1"},
		{"::1", "::1"},
		{"10.0.0.1:12345", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractIP(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	t.Parallel()

	config := RateLimiterConfig{
		MaxAttempts:     100,
		Window:          time.Minute,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	// Concurrent access from multiple goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			ip := "192.168.1.1"
			for j := 0; j < 10; j++ {
				_ = rl.CheckAndRecord(ip)
				_ = rl.RemainingAttempts(ip)
				_ = rl.IsBlocked(ip)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have correct count
	assert.Equal(t, 0, rl.RemainingAttempts("192.168.1.1"))
}
