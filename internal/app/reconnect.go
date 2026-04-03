package app

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

const maxReconnectDelay = 60 * time.Second

// RunWithReconnect executes runFn with automatic reconnection on failure.
// It implements exponential backoff with a 60-second maximum delay.
//
// Parameters:
//   - ctx: Context for cancellation
//   - logger: Logger for reconnection messages
//   - maxAttempts: Maximum reconnection attempts (0 = infinite)
//   - baseInterval: Initial delay, doubles on each retry
//   - runFn: Function to execute (typically ClientApp.Run)
//
// Returns nil on successful completion, or the last error after maxAttempts exceeded.
func RunWithReconnect(
	ctx context.Context,
	logger *slog.Logger,
	maxAttempts int,
	baseInterval time.Duration,
	runFn func(ctx context.Context) error,
) error {
	return RunWithReconnectAndCircuitBreaker(ctx, logger, maxAttempts, baseInterval, nil, runFn)
}

// RunWithReconnectAndCircuitBreaker combines reconnection logic with circuit breaker
// protection. The circuit breaker prevents cascading failures when the remote
// service is unavailable.
//
// Parameters:
//   - ctx: Context for cancellation
//   - logger: Logger for reconnection messages
//   - maxAttempts: Maximum reconnection attempts (0 = infinite)
//   - baseInterval: Initial delay, doubles on each retry up to 60s
//   - circuitBreaker: Optional circuit breaker (nil to disable)
//   - runFn: Function to execute (typically ClientApp.Run)
//
// Returns nil on successful completion, ErrCircuitOpen if circuit is open,
// or the last error after maxAttempts exceeded.
func RunWithReconnectAndCircuitBreaker(
	ctx context.Context,
	logger *slog.Logger,
	maxAttempts int,
	baseInterval time.Duration,
	circuitBreaker *CircuitBreaker,
	runFn func(ctx context.Context) error,
) error {
	attempt := 0

	for {
		err := executeWithOptionalCircuitBreaker(ctx, circuitBreaker, runFn)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Do not reconnect if kicked or banned — these are terminal states.
		if ewerrors.IsErrorCode(err, ewerrors.ErrClientKicked) || ewerrors.IsErrorCode(err, ewerrors.ErrClientBannedByAdmin) {
			return err
		}

		// Reset backoff if connection was established but then dropped (context.Canceled
		// from the session sub-context, not the parent context which we already checked above).
		if errors.Is(err, context.Canceled) {
			attempt = 0
		}
		attempt++
		if shouldStop := handleReconnectError(logger, circuitBreaker, err, maxAttempts, attempt); shouldStop {
			return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "max reconnect attempts exceeded").
				WithContext("max_attempts", maxAttempts).
				WithContext("last_attempt", attempt)
		}

		delay := calculateBackoffDelay(baseInterval, attempt)
		logReconnectAttempt(logger, attempt, maxAttempts, delay, err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// executeWithOptionalCircuitBreaker runs the function with or without circuit breaker.
func executeWithOptionalCircuitBreaker(ctx context.Context, cb *CircuitBreaker, runFn func(ctx context.Context) error) error {
	if cb != nil {
		return cb.Execute(ctx, func() error { return runFn(ctx) })
	}
	return runFn(ctx)
}

// calculateBackoffDelay computes exponential backoff delay capped at 60 seconds.
func calculateBackoffDelay(baseInterval time.Duration, attempt int) time.Duration {
	delay := float64(baseInterval) * math.Pow(2, float64(attempt-1))
	if delay > float64(maxReconnectDelay) {
		delay = float64(maxReconnectDelay)
	}
	return time.Duration(delay)
}

// handleReconnectError logs circuit breaker state and checks if max attempts exceeded.
func handleReconnectError(logger *slog.Logger, cb *CircuitBreaker, err error, maxAttempts, attempt int) bool {
	if errors.Is(err, ErrCircuitOpen) && cb != nil {
		logger.Warn("Circuit breaker is open, waiting before retry",
			"attempt", attempt, "circuit_state", cb.State().String())
	}
	return maxAttempts > 0 && attempt >= maxAttempts
}

// logReconnectAttempt logs the reconnection attempt with appropriate detail.
func logReconnectAttempt(logger *slog.Logger, attempt, maxAttempts int, delay time.Duration, err error) {
	args := []any{"attempt", attempt, "delay", delay, "error", err}
	if maxAttempts > 0 {
		args = append(args, "max_attempts", maxAttempts)
	}
	logger.Warn("Connection lost, reconnecting", args...)
}
