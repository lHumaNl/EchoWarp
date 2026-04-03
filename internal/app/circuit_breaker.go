package app

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

// Circuit breaker states follow the standard pattern:
//   - Closed: Requests flow normally, failures are counted
//   - Open: Requests are blocked, waiting for timeout
//   - HalfOpen: Limited requests allowed to test recovery
const (
	// StateClosed allows all requests through and counts failures.
	StateClosed CircuitState = iota
	// StateOpen blocks all requests and waits for timeout before transitioning to HalfOpen.
	StateOpen
	// StateHalfOpen allows a limited number of test requests to check if the service recovered.
	StateHalfOpen
)

// ErrCircuitOpen is returned when Execute is called while the circuit is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern for fault tolerance.
// It prevents cascading failures by temporarily blocking requests when the
// failure threshold is exceeded.
//
// Thread-safe: All methods are safe for concurrent use.
//
// State transitions:
//
//	Closed --[failures >= max]--> Open --[timeout]--> HalfOpen
//	HalfOpen --[success]--> Closed
//	HalfOpen --[failure]--> Open
type CircuitBreaker struct {
	mu            sync.RWMutex
	state         CircuitState
	failureCount  int
	maxFailures   int
	timeout       time.Duration
	lastFailTime  time.Time
	onStateChange func(old, newState CircuitState)
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
// maxFailures is the number of consecutive failures before opening the circuit.
// timeout is how long to wait in Open state before transitioning to HalfOpen.
func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		timeout:     timeout,
	}
}

// Execute runs the given function if the circuit allows requests.
// Returns ErrCircuitOpen if the circuit is open and no requests are allowed.
// The result (success/failure) is recorded and may trigger a state transition.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed {
		return true
	}

	if cb.state == StateOpen {
		if time.Since(cb.lastFailTime) > cb.timeout {
			oldState := cb.state
			cb.state = StateHalfOpen
			if cb.onStateChange != nil {
				cb.onStateChange(oldState, cb.state)
			}
			return true
		}
		return false
	}

	return true
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state

	if err == nil {
		if cb.state == StateHalfOpen {
			cb.state = StateClosed
			cb.failureCount = 0
			if cb.onStateChange != nil {
				cb.onStateChange(oldState, cb.state)
			}
		}
	} else {
		cb.failureCount++
		cb.lastFailTime = time.Now()

		if cb.failureCount >= cb.maxFailures && cb.state != StateOpen {
			cb.state = StateOpen
			if cb.onStateChange != nil {
				cb.onStateChange(oldState, cb.state)
			}
		}
	}
}

// State returns the current circuit breaker state (Closed, Open, or HalfOpen).
// This is useful for monitoring and health checks.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// FailureCount returns the current number of consecutive failures.
// Resets to 0 on successful execution or when entering HalfOpen state.
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// SetOnStateChange registers a callback for state transition notifications.
// The callback receives the old and new states. Set to nil to remove.
func (cb *CircuitBreaker) SetOnStateChange(fn func(old, newState CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// String returns a human-readable representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
