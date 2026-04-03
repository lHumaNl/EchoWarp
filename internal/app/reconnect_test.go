package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestRunWithReconnect_SuccessOnFirstTry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var callCount int32

	err := RunWithReconnect(ctx, testLogger(), 5, 100*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestRunWithReconnect_RetriesOnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var callCount int32

	err := RunWithReconnect(ctx, testLogger(), 3, 50*time.Millisecond, func(ctx context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestRunWithReconnect_MaxAttemptsExceeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var callCount int32

	err := RunWithReconnect(ctx, testLogger(), 3, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return errors.New("persistent error")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max reconnect attempts")
	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
}

func TestRunWithReconnect_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount int32
	started := make(chan struct{})

	go func() {
		<-started
		cancel()
	}()

	err := RunWithReconnect(ctx, testLogger(), 0, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		select {
		case started <- struct{}{}:
		default:
		}
		return errors.New("error")
	})

	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunWithReconnect_ExponentialBackoff(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var timestamps []time.Time
	var callCount int32

	start := time.Now()
	_ = RunWithReconnect(ctx, testLogger(), 4, 50*time.Millisecond, func(ctx context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		timestamps = append(timestamps, time.Now())
		if count >= 4 {
			return nil
		}
		return errors.New("error")
	})

	require.GreaterOrEqual(t, len(timestamps), 3)

	for i := 1; i < len(timestamps)-1; i++ {
		delay := timestamps[i+1].Sub(timestamps[i])
		t.Logf("Delay %d: %v", i, delay)
	}

	totalTime := timestamps[len(timestamps)-1].Sub(start)
	t.Logf("Total time: %v", totalTime)
	assert.Greater(t, totalTime, 300*time.Millisecond, "backoff should add noticeable delay")
}

func TestRunWithReconnect_InfiniteRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var callCount int32

	err := RunWithReconnect(ctx, testLogger(), 0, 5*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return errors.New("error")
	})

	assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(5))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCircuitBreaker_Closed_AllowsAll(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(3, 1*time.Second)
	var callCount int32

	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func() error {
			atomic.AddInt32(&callCount, 1)
			return nil
		})
		assert.NoError(t, err)
	}

	assert.Equal(t, int32(5), atomic.LoadInt32(&callCount))
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	t.Parallel()
	cb := NewCircuitBreaker(3, 1*time.Second)
	var callCount int32

	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func() error {
			atomic.AddInt32(&callCount, 1)
			return errors.New("failure")
		})
		if i < 3 {
			assert.Error(t, err)
			assert.NotEqual(t, ErrCircuitOpen, err)
		} else {
			assert.Equal(t, ErrCircuitOpen, err)
		}
	}

	assert.Equal(t, int32(3), atomic.LoadInt32(&callCount))
	assert.Equal(t, StateOpen, cb.State())
	assert.Equal(t, 3, cb.FailureCount())
}

func TestCircuitBreaker_HalfOpen_AfterTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	const timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	err := cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	err = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout using timer channel (deterministic channel-based synchronization)
	timer := time.NewTimer(timeout)
	<-timer.C

	var executed bool
	err = cb.Execute(context.Background(), func() error {
		executed = true
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, executed)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_ClosesAfterSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	const timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	err := cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	err = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout using timer channel (deterministic channel-based synchronization)
	timer := time.NewTimer(timeout)
	<-timer.C

	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.FailureCount())
}

func TestCircuitBreaker_HalfOpen_ReopensOnFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	const timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	err := cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	err = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)

	assert.Equal(t, StateOpen, cb.State())

	// Wait for timeout using timer channel (deterministic channel-based synchronization)
	timer := time.NewTimer(timeout)
	<-timer.C

	err = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_StateChange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	const timeout = 100 * time.Millisecond
	cb := NewCircuitBreaker(2, timeout)

	var stateChanges []CircuitState
	cb.SetOnStateChange(func(old, new CircuitState) {
		stateChanges = append(stateChanges, new)
	})

	_ = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("failure")
	})

	// Wait for timeout using timer channel (deterministic channel-based synchronization)
	timer := time.NewTimer(timeout)
	<-timer.C

	_ = cb.Execute(context.Background(), func() error {
		return nil
	})

	assert.Equal(t, []CircuitState{StateOpen, StateHalfOpen, StateClosed}, stateChanges)
}

func TestRunWithReconnectAndCircuitBreaker_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	var callCount int32

	err := RunWithReconnectAndCircuitBreaker(ctx, testLogger(), 0, 10*time.Millisecond, cb, func(ctx context.Context) error {
		count := atomic.AddInt32(&callCount, 1)
		if count < 5 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_StateString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.state.String())
	}
}

func TestRunWithReconnect_NoReconnectOnKicked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var callCount int32

	kickErr := ewerrors.NewError(ewerrors.ErrClientKicked, "kicked by server")
	err := RunWithReconnect(ctx, testLogger(), 5, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return kickErr
	})

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "should NOT retry after kicked error")
	require.Error(t, err)
	assert.True(t, ewerrors.IsErrorCode(err, ewerrors.ErrClientKicked))
}

func TestRunWithReconnect_NoReconnectOnBanned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var callCount int32

	banErr := ewerrors.NewError(ewerrors.ErrClientBannedByAdmin, "banned by server")
	err := RunWithReconnect(ctx, testLogger(), 5, 10*time.Millisecond, func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return banErr
	})

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "should NOT retry after banned error")
	require.Error(t, err)
	assert.True(t, ewerrors.IsErrorCode(err, ewerrors.ErrClientBannedByAdmin))
}
