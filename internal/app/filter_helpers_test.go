package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSafeSendOpus_RecoversFromClosedDst verifies that when dst is closed
// during a send (the panic case that caused the server crash on client
// disconnect), safeSendOpus recovers cleanly and signals the caller to exit.
func TestSafeSendOpus_RecoversFromClosedDst(t *testing.T) {
	ctx := context.Background()
	dst := make(chan []byte, 1)
	close(dst)

	// Send on closed channel panics; safeSendOpus must recover.
	done := safeSendOpus(ctx, dst, []byte{1, 2, 3})
	assert.True(t, done, "should signal caller to exit when dst is closed")
}

// TestNewFlagFilterCh_ForwardsAndDrops verifies basic forward/drop behavior.
func TestNewFlagFilterCh_ForwardsAndDrops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dst := make(chan []byte, 4)
	var flag atomic.Bool

	src := newFlagFilterCh(ctx, dst, &flag)

	// flag=false → frames forwarded
	src <- []byte{1}
	select {
	case got := <-dst:
		assert.Equal(t, []byte{1}, got)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected frame to be forwarded when flag=false")
	}

	// flag=true → frames dropped
	flag.Store(true)
	src <- []byte{2}
	select {
	case got := <-dst:
		t.Fatalf("expected no forward when flag=true, got %v", got)
	case <-time.After(50 * time.Millisecond):
		// OK: dropped
	}
}
