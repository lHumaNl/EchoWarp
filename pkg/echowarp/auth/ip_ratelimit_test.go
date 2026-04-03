package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewIPRateLimiter_Defaults(t *testing.T) {
	t.Parallel()
	r := NewIPRateLimiter(0)
	defer r.Close()

	if r.maxRate != defaultMaxConnPerSecPerIP {
		t.Errorf("Expected maxRate %d, got %d", defaultMaxConnPerSecPerIP, r.maxRate)
	}
}

func TestNewIPRateLimiter_CustomRate(t *testing.T) {
	t.Parallel()
	r := NewIPRateLimiter(10)
	defer r.Close()

	if r.maxRate != 10 {
		t.Errorf("Expected maxRate 10, got %d", r.maxRate)
	}
}

func TestIPRateLimiter_Allow_UnderLimit(t *testing.T) {
	t.Parallel()
	r := NewIPRateLimiter(3)
	defer r.Close()

	for i := 0; i < 3; i++ {
		if !r.Allow("192.168.1.1") {
			t.Errorf("Expected Allow to return true for request %d", i)
		}
	}
}

func TestIPRateLimiter_Allow_OverLimit(t *testing.T) {
	t.Parallel()
	r := NewIPRateLimiter(2)
	defer r.Close()

	r.Allow("192.168.1.1")
	r.Allow("192.168.1.1")

	if r.Allow("192.168.1.1") {
		t.Error("Expected Allow to return false over limit")
	}
}

func TestIPRateLimiter_Allow_DifferentIPs(t *testing.T) {
	t.Parallel()
	r := NewIPRateLimiter(2)
	defer r.Close()

	r.Allow("192.168.1.1")
	r.Allow("192.168.1.1")

	if !r.Allow("192.168.1.2") {
		t.Error("Expected Allow to return true for different IP")
	}
}

func TestIPRateLimiter_Allow_AfterWindow(t *testing.T) {
	r := NewIPRateLimiter(1)
	r.window = 50 * time.Millisecond
	defer r.Close()

	if !r.Allow("192.168.1.1") {
		t.Error("Expected first Allow to return true")
	}

	if r.Allow("192.168.1.1") {
		t.Error("Expected second Allow to return false")
	}

	// Use require.Eventually to poll for window expiration
	require.Eventually(t, func() bool {
		return r.Allow("192.168.1.1")
	}, 200*time.Millisecond, 10*time.Millisecond, "Expected Allow to return true after window expires")
}

func TestIPRateLimiter_Cleanup_RemovesOldEntries(t *testing.T) {
	r := NewIPRateLimiter(5)
	r.window = 50 * time.Millisecond
	defer r.Close()

	r.Allow("192.168.1.1")

	// Use require.Eventually to wait for entries to expire and be cleaned up
	require.Eventually(t, func() bool {
		r.Cleanup()
		r.mu.Lock()
		entryCount := len(r.entries["192.168.1.1"])
		r.mu.Unlock()
		return entryCount == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Expected old entries to be cleaned up")
}

func TestIPRateLimiter_EvictOldest(t *testing.T) {
	r := NewIPRateLimiter(5)
	r.maxEntries = 2
	defer r.Close()

	r.Allow("192.168.1.1")
	r.Allow("192.168.1.2")

	r.mu.Lock()
	r.evictOldest()
	entryCount := len(r.entries)
	r.mu.Unlock()

	if entryCount >= 2 {
		t.Errorf("Expected entries to be evicted, got %d", entryCount)
	}
}

func TestExtractIP_WithPort(t *testing.T) {
	t.Parallel()
	result := ExtractIP("192.168.1.1:8080")
	if result != "192.168.1.1" {
		t.Errorf("Expected '192.168.1.1', got '%s'", result)
	}
}

func TestExtractIP_InvalidFormat(t *testing.T) {
	t.Parallel()
	result := ExtractIP("invalid")
	if result != "invalid" {
		t.Errorf("Expected 'invalid', got '%s'", result)
	}
}

func TestExtractIP_IPv6(t *testing.T) {
	t.Parallel()
	result := ExtractIP("[::1]:8080")
	if result != "::1" {
		t.Errorf("Expected '::1', got '%s'", result)
	}
}
