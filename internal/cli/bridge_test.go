package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeChanTransform(t *testing.T) {
	src := make(chan int, 4)
	dst := make(chan string, 4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridgeChanTransform(ctx, src, dst, func(i int) string {
		return string(rune('A' + i))
	})

	// Send values.
	src <- 0
	src <- 1
	src <- 2

	// Receive transformed values.
	assert.Equal(t, "A", recv(t, dst))
	assert.Equal(t, "B", recv(t, dst))
	assert.Equal(t, "C", recv(t, dst))
}

func TestBridgeChanTransform_CloseSrc(t *testing.T) {
	src := make(chan int, 1)
	dst := make(chan string, 1)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		bridgeChanTransform(ctx, src, dst, func(i int) string { return "" })
		close(done)
	}()

	close(src)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bridge did not exit after src close")
	}

	// dst should be closed.
	_, ok := <-dst
	require.False(t, ok, "dst should be closed")
}

func TestBridgeChanTransform_ContextCancel(t *testing.T) {
	src := make(chan int)
	dst := make(chan string, 1)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		bridgeChanTransform(ctx, src, dst, func(i int) string { return "" })
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("bridge did not exit after context cancel")
	}
}

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for value")
		var zero T
		return zero
	}
}
