package echowarp

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventBus_Subscribe_ReturnsID(t *testing.T) {
	bus := NewEventBus()
	id := bus.Subscribe(EventConnected, func(e Event) {})
	if id < 0 {
		t.Error("Subscribe should return a valid SubscriptionID")
	}
}

func TestEventBus_SubscribeAll_ReturnsID(t *testing.T) {
	bus := NewEventBus()
	id := bus.SubscribeAll(func(e Event) {})
	if id < 0 {
		t.Error("SubscribeAll should return a valid SubscriptionID")
	}
}

func TestEventBus_Unsubscribe_RemovesListener(t *testing.T) {
	bus := NewEventBus()
	var called int32

	id := bus.Subscribe(EventConnected, func(e Event) {
		atomic.AddInt32(&called, 1)
	})

	removed := bus.Unsubscribe(id)
	if !removed {
		t.Error("Unsubscribe should return true for valid ID")
	}

	bus.EmitConnected("test")
	bus.Shutdown()

	if atomic.LoadInt32(&called) != 0 {
		t.Error("Listener should not be called after unsubscribe")
	}
}

func TestEventBus_Unsubscribe_InvalidID_ReturnsFalse(t *testing.T) {
	bus := NewEventBus()
	removed := bus.Unsubscribe(99999)
	if removed {
		t.Error("Unsubscribe should return false for invalid ID")
	}
}

func TestEventBus_UnsubscribeAll_RemovesListener(t *testing.T) {
	bus := NewEventBus()
	var called int32

	id := bus.SubscribeAll(func(e Event) {
		atomic.AddInt32(&called, 1)
	})

	removed := bus.Unsubscribe(id)
	if !removed {
		t.Error("Unsubscribe should return true for valid ID")
	}

	bus.EmitConnected("test")
	bus.Shutdown()

	if atomic.LoadInt32(&called) != 0 {
		t.Error("Listener should not be called after unsubscribe")
	}
}

func TestEventBus_Emit_CallsAllListeners(t *testing.T) {
	bus := NewEventBus()
	var called int32

	bus.Subscribe(EventConnected, func(e Event) {
		atomic.AddInt32(&called, 1)
	})
	bus.Subscribe(EventConnected, func(e Event) {
		atomic.AddInt32(&called, 1)
	})

	bus.EmitConnected("test")
	bus.Shutdown()

	if atomic.LoadInt32(&called) != 2 {
		t.Errorf("Expected 2 calls, got %d", called)
	}
}

func TestEventBus_Emit_CallsAllListenersForAllEvents(t *testing.T) {
	bus := NewEventBus()
	var called int32

	bus.SubscribeAll(func(e Event) {
		atomic.AddInt32(&called, 1)
	})

	bus.EmitConnected("test")
	bus.EmitDisconnected("reason")
	bus.Shutdown()

	if atomic.LoadInt32(&called) != 2 {
		t.Errorf("Expected 2 calls, got %d", called)
	}
}

func TestEventBus_WorkerPool_LimitsConcurrency(t *testing.T) {
	bus := NewEventBus()
	var maxConcurrent int32
	var current int32
	var wg sync.WaitGroup

	// Use a channel to signal when listener has recorded its concurrency
	listenerDone := make(chan struct{}, 100)

	for i := 0; i < 100; i++ {
		bus.Subscribe(EventConnected, func(e Event) {
			cur := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
					break
				}
			}
			// Signal that we've recorded our concurrency level
			listenerDone <- struct{}{}
			atomic.AddInt32(&current, -1)
			wg.Done()
		})
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
	}
	bus.EmitConnected("test")

	// Wait for all listeners to complete using WaitGroup
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for all listeners to complete")
	}
	bus.Shutdown()

	if maxConcurrent > int32(defaultWorkerPoolSize) {
		t.Errorf("Max concurrent goroutines exceeded limit: got %d, max %d", maxConcurrent, defaultWorkerPoolSize)
	}
}

func TestEventBus_DefaultWorkerPoolSize(t *testing.T) {
	bus := NewEventBus()
	if bus.WorkerPoolSize() != defaultWorkerPoolSize {
		t.Errorf("Expected default worker pool size %d, got %d", defaultWorkerPoolSize, bus.WorkerPoolSize())
	}
}

func TestEventBus_CustomWorkerPoolSize(t *testing.T) {
	customSize := 64
	bus := NewEventBus(WithWorkerPoolSize(customSize))
	if bus.WorkerPoolSize() != customSize {
		t.Errorf("Expected custom worker pool size %d, got %d", customSize, bus.WorkerPoolSize())
	}
}

func TestEventBus_InvalidWorkerPoolSize(t *testing.T) {
	bus := NewEventBus(WithWorkerPoolSize(0))
	if bus.WorkerPoolSize() != defaultWorkerPoolSize {
		t.Errorf("Expected default worker pool size %d for invalid input, got %d", defaultWorkerPoolSize, bus.WorkerPoolSize())
	}
}

func TestEventBus_NegativeWorkerPoolSize(t *testing.T) {
	bus := NewEventBus(WithWorkerPoolSize(-1))
	if bus.WorkerPoolSize() != defaultWorkerPoolSize {
		t.Errorf("Expected default worker pool size %d for negative input, got %d", defaultWorkerPoolSize, bus.WorkerPoolSize())
	}
}

func TestEventBus_CustomWorkerPoolSize_LimitsConcurrency(t *testing.T) {
	customSize := 8
	bus := NewEventBus(WithWorkerPoolSize(customSize))
	var maxConcurrent int32
	var current int32
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		bus.Subscribe(EventConnected, func(e Event) {
			cur := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
					break
				}
			}
			// Record concurrency and finish immediately (no artificial delay needed)
			atomic.AddInt32(&current, -1)
			wg.Done()
		})
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
	}
	bus.EmitConnected("test")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for all listeners to complete")
	}
	bus.Shutdown()

	if maxConcurrent > int32(customSize) {
		t.Errorf("Max concurrent goroutines exceeded custom limit: got %d, max %d", maxConcurrent, customSize)
	}
}

func TestEventHandler_CustomWorkerPoolSize(t *testing.T) {
	customSize := 16
	h := NewEventHandler(WithEventHandlerWorkerPoolSize(customSize))
	if h.Bus().WorkerPoolSize() != customSize {
		t.Errorf("Expected custom worker pool size %d, got %d", customSize, h.Bus().WorkerPoolSize())
	}
}

func TestEventHandler_DefaultWorkerPoolSize(t *testing.T) {
	h := NewEventHandler()
	if h.Bus().WorkerPoolSize() != defaultWorkerPoolSize {
		t.Errorf("Expected default worker pool size %d, got %d", defaultWorkerPoolSize, h.Bus().WorkerPoolSize())
	}
}

func TestEventBus_PanicRecovery(t *testing.T) {
	bus := NewEventBus()
	var called int32

	bus.Subscribe(EventConnected, func(e Event) {
		panic("test panic")
	})
	bus.Subscribe(EventConnected, func(e Event) {
		atomic.AddInt32(&called, 1)
	})

	bus.EmitConnected("test")
	bus.Shutdown()

	if atomic.LoadInt32(&called) != 1 {
		t.Error("Other listeners should still be called after panic")
	}
}

func TestEventBus_Shutdown_WaitsForListeners(t *testing.T) {
	bus := NewEventBus()
	var completed int32
	listenerDone := make(chan struct{})
	listenerStarted := make(chan struct{})

	bus.Subscribe(EventConnected, func(e Event) {
		close(listenerStarted)
		<-listenerDone
		atomic.AddInt32(&completed, 1)
	})

	bus.EmitConnected("test")

	select {
	case <-listenerStarted:
		// Listener is now running and waiting on listenerDone
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for listener to start")
	}
	close(listenerDone)

	shutdownDone := make(chan struct{})
	go func() {
		bus.Shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		if atomic.LoadInt32(&completed) != 1 {
			t.Error("Shutdown should wait for all listeners to complete")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for shutdown to complete")
	}
}

func TestEventHandler_SetOnConnected(t *testing.T) {
	h := NewEventHandler()
	var receivedAddr string
	h.SetOnConnected(func(addr string) {
		receivedAddr = addr
	})

	h.Bus().EmitConnected("192.168.1.1:8080")
	h.Bus().Shutdown()

	if receivedAddr != "192.168.1.1:8080" {
		t.Errorf("Expected addr '192.168.1.1:8080', got '%s'", receivedAddr)
	}
}

func TestEventHandler_SetOnDisconnected(t *testing.T) {
	h := NewEventHandler()
	var receivedReason string
	h.SetOnDisconnected(func(reason string) {
		receivedReason = reason
	})

	h.Bus().EmitDisconnected("connection lost")
	h.Bus().Shutdown()

	if receivedReason != "connection lost" {
		t.Errorf("Expected reason 'connection lost', got '%s'", receivedReason)
	}
}

func TestEventHandler_SetOnError(t *testing.T) {
	h := NewEventHandler()
	var receivedErr error
	h.SetOnError(func(err error) {
		receivedErr = err
	})

	testErr := error(&testError{msg: "test error"})
	h.Bus().EmitError(testErr)
	h.Bus().Shutdown()

	if receivedErr == nil || receivedErr.Error() != "test error" {
		t.Errorf("Expected error 'test error', got '%v'", receivedErr)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestEventBus_ListenerTimeout(t *testing.T) {
	bus := NewEventBus(WithListenerTimeout(100 * time.Millisecond))
	var called int32
	listenerStarted := make(chan struct{})

	bus.Subscribe(EventConnected, func(e Event) {
		close(listenerStarted)
		time.Sleep(5 * time.Second) // Intentional: listener blocks longer than timeout
		atomic.AddInt32(&called, 1)
	})

	bus.EmitConnected("test")

	// Wait for listener to start (deterministic sync)
	<-listenerStarted

	// Use require.Eventually to wait for timeout to occur (called should remain 0)
	require.Eventually(t, func() bool {
		return bus.ActiveWorkers() == 0
	}, 300*time.Millisecond, 10*time.Millisecond, "Worker should be released after timeout")

	if atomic.LoadInt32(&called) != 0 {
		t.Error("Blocking listener should not have completed, but timeout should have occurred")
	}
	bus.Shutdown()
}

func TestEventBus_ListenerCompletesBeforeTimeout(t *testing.T) {
	bus := NewEventBus(WithListenerTimeout(1 * time.Second))
	var called int32
	done := make(chan struct{})

	bus.Subscribe(EventConnected, func(e Event) {
		atomic.AddInt32(&called, 1)
		close(done)
	})

	bus.EmitConnected("test")

	// Wait for listener to complete (deterministic sync)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for listener to complete")
	}

	bus.Shutdown()

	if atomic.LoadInt32(&called) != 1 {
		t.Errorf("Expected listener to complete, got %d calls", called)
	}
}

func TestEventBus_ListenerTimeoutOption(t *testing.T) {
	customTimeout := 5 * time.Second
	bus := NewEventBus(WithListenerTimeout(customTimeout))

	if bus.listenerTimeout != customTimeout {
		t.Errorf("Expected listener timeout %v, got %v", customTimeout, bus.listenerTimeout)
	}
}

func TestEventBus_DefaultListenerTimeout(t *testing.T) {
	bus := NewEventBus()

	if bus.listenerTimeout != defaultListenerTimeout {
		t.Errorf("Expected default listener timeout %v, got %v", defaultListenerTimeout, bus.listenerTimeout)
	}
}

func TestEventBus_InvalidListenerTimeout(t *testing.T) {
	bus := NewEventBus(WithListenerTimeout(0))

	if bus.listenerTimeout != defaultListenerTimeout {
		t.Errorf("Expected default listener timeout %v for invalid input, got %v", defaultListenerTimeout, bus.listenerTimeout)
	}
}

func TestEventBus_NegativeListenerTimeout(t *testing.T) {
	bus := NewEventBus(WithListenerTimeout(-1 * time.Second))

	if bus.listenerTimeout != defaultListenerTimeout {
		t.Errorf("Expected default listener timeout %v for negative input, got %v", defaultListenerTimeout, bus.listenerTimeout)
	}
}

func TestEventBus_PoolExhaustion(t *testing.T) {
	// This test verifies that when all workers are occupied with blocking listeners,
	// new events are queued. After the listener timeout, workers are released
	// even though the blocking listener goroutines continue running, allowing
	// queued events to be processed.

	poolSize := 2
	timeout := 200 * time.Millisecond
	bus := NewEventBus(
		WithWorkerPoolSize(poolSize),
		WithListenerTimeout(timeout),
	)

	// Create a channel to block listeners
	blockCh := make(chan struct{})
	var blockingCompleted int32
	workersOccupied := make(chan struct{}, poolSize)

	// Subscribe 2 blocking listeners (matches pool size)
	bus.Subscribe(EventConnected, func(e Event) {
		workersOccupied <- struct{}{}
		<-blockCh
		atomic.AddInt32(&blockingCompleted, 1)
	})
	bus.Subscribe(EventConnected, func(e Event) {
		workersOccupied <- struct{}{}
		<-blockCh
		atomic.AddInt32(&blockingCompleted, 1)
	})

	// Emit the event to start both blocking listeners
	bus.EmitConnected("test")

	// Wait for both workers to be occupied using channel signals
	for i := 0; i < poolSize; i++ {
		select {
		case <-workersOccupied:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for worker %d to start", i+1)
		}
	}

	// Verify both workers are occupied
	if bus.ActiveWorkers() != poolSize {
		t.Errorf("Expected %d active workers after emit, got %d", poolSize, bus.ActiveWorkers())
	}

	// Subscribe a non-blocking listener to a different event
	var nonBlockingCalled int32
	bus.Subscribe(EventDisconnected, func(e Event) {
		atomic.AddInt32(&nonBlockingCalled, 1)
	})

	// Emit different event - this will block waiting for a worker
	// The emit call itself blocks until timeout releases a worker
	emitDone := make(chan struct{})
	go func() {
		bus.EmitDisconnected("reason")
		close(emitDone)
	}()

	// The emit should block because workers are occupied
	select {
	case <-emitDone:
		t.Error("Emit should have blocked waiting for worker")
	case <-time.After(100 * time.Millisecond):
		// Good - emit is blocked
	}

	// Wait for timeout to release workers using the emitDone channel
	select {
	case <-emitDone:
		// Good - emit completed after timeout
	case <-time.After(timeout + 200*time.Millisecond):
		t.Error("Emit should have completed after timeout")
	}

	// Use require.Eventually to wait for non-blocking listener to complete
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&nonBlockingCalled) == 1
	}, 200*time.Millisecond, 10*time.Millisecond, "Non-blocking listener should have been called after timeout")

	// Unblock the listeners (their inner goroutines are still running)
	close(blockCh)

	// Use require.Eventually to wait for blocking listeners to complete
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&blockingCompleted) == 2
	}, 200*time.Millisecond, 10*time.Millisecond, "Blocking listeners should complete")

	bus.Shutdown()

	// Verify blocking listeners completed
	if atomic.LoadInt32(&blockingCompleted) != 2 {
		t.Errorf("Expected 2 blocking listeners to complete, got %d", atomic.LoadInt32(&blockingCompleted))
	}
}
