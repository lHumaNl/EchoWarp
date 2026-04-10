package echowarp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// EventType represents the category of an event in the event bus system.
type EventType int

// Event type constants define all possible events that can be emitted by a Node.
const (
	// EventConnected is emitted when a connection is successfully established.
	EventConnected EventType = iota
	// EventDisconnected is emitted when a connection is closed.
	EventDisconnected
	// EventError is emitted when an error occurs during streaming.
	EventError
	// EventReconnecting is emitted when reconnection attempts begin.
	EventReconnecting
	// EventStatsUpdate is emitted periodically with connection statistics.
	EventStatsUpdate
	// EventClientJoined is emitted when a new client connects (server mode).
	EventClientJoined
	// EventClientLeft is emitted when a client disconnects (server mode).
	EventClientLeft
	// EventChatMessage is emitted when a chat message is received (either
	// from a remote peer in client mode, or from any participant in server
	// mode). System, broadcast, and DM variants all map to this single event
	// type — the Data payload (ChatMessageData) carries the distinguishing
	// fields (From, To, Text).
	EventChatMessage
)

// EventData is a marker interface for typed event payloads.
// Each event type has a corresponding struct that implements this interface.
type EventData interface {
	eventData()
}

// ConnectedData is emitted with EventConnected.
type ConnectedData struct {
	Addr string
}

func (ConnectedData) eventData() {}

// DisconnectedData is emitted with EventDisconnected.
type DisconnectedData struct {
	Reason string
}

func (DisconnectedData) eventData() {}

// ErrorData is emitted with EventError.
type ErrorData struct {
	Err error
}

func (ErrorData) eventData() {}

// ReconnectingData is emitted with EventReconnecting.
type ReconnectingData struct {
	Attempt     int
	MaxAttempts int
}

func (ReconnectingData) eventData() {}

// StatsUpdateData is emitted with EventStatsUpdate.
type StatsUpdateData struct {
	Stats ConnectionStats
}

func (StatsUpdateData) eventData() {}

// ClientJoinedData is emitted with EventClientJoined.
type ClientJoinedData struct {
	ClientID string
	Addr     string
}

func (ClientJoinedData) eventData() {}

// ClientLeftData is emitted with EventClientLeft.
type ClientLeftData struct {
	ClientID string
}

func (ClientLeftData) eventData() {}

// ChatMessageData is emitted with EventChatMessage. It mirrors the minimal
// set of fields a downstream consumer (WebSocket bridge, plugin, etc.) needs
// to render a chat entry, without exposing internal ring-buffer state.
type ChatMessageData struct {
	// From is the sender's display name. Empty for system messages
	// originating from the server itself (e.g. "Server").
	From string `json:"from,omitempty"`
	// To is the direct-message recipient display name. Empty for broadcast
	// messages addressed to all participants.
	To string `json:"to,omitempty"`
	// Text is the plain-text content of the message. May be truncated by
	// the sender if it exceeded the internal per-message length limit.
	Text string `json:"text"`
	// TS is the message timestamp in milliseconds since the Unix epoch, as
	// assigned by the server (or by the local client for outgoing
	// broadcasts).
	TS int64 `json:"ts"`
}

func (ChatMessageData) eventData() {}

// Event represents a notification emitted by the event bus.
type Event struct {
	Type EventType // The category of the event.
	Data EventData // Typed event-specific data.
}

// EventListener is a callback function that receives events.
type EventListener func(event Event)

// SubscriptionID is a unique identifier for an event subscription, used for unsubscription.
type SubscriptionID int64

type subscription struct {
	id       SubscriptionID
	listener EventListener
}

const defaultWorkerPoolSize = 32

const defaultListenerTimeout = 10 * time.Second

type EventBusOption func(*EventBus)

func WithWorkerPoolSize(size int) EventBusOption {
	return func(e *EventBus) {
		if size > 0 {
			e.workerPoolSize = size
		}
	}
}

func WithListenerTimeout(timeout time.Duration) EventBusOption {
	return func(e *EventBus) {
		if timeout > 0 {
			e.listenerTimeout = timeout
		}
	}
}

type EventBus struct {
	mu              sync.RWMutex
	nextID          SubscriptionID
	listeners       map[EventType][]subscription
	all             []subscription
	workerPool      chan struct{}
	workerPoolSize  int
	listenerTimeout time.Duration
	wg              sync.WaitGroup
	logger          *slog.Logger
}

// WithEventBusLogger sets a structured logger for EventBus internal messages (panics, timeouts).
// If not set, slog.Default() is used.
func WithEventBusLogger(logger *slog.Logger) EventBusOption {
	return func(e *EventBus) {
		if logger != nil {
			e.logger = logger
		}
	}
}

func NewEventBus(opts ...EventBusOption) *EventBus {
	e := &EventBus{
		listeners:       make(map[EventType][]subscription),
		workerPoolSize:  defaultWorkerPoolSize,
		listenerTimeout: defaultListenerTimeout,
		logger:          slog.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}
	e.workerPool = make(chan struct{}, e.workerPoolSize)
	metrics.EventBusWorkerPoolSize.Set(float64(e.workerPoolSize))
	return e
}

func (b *EventBus) WorkerPoolSize() int {
	return b.workerPoolSize
}

func (b *EventBus) ActiveWorkers() int {
	return len(b.workerPool)
}

// Subscribe registers a listener for a specific event type.
// Returns a SubscriptionID that can be used to unsubscribe.
func (b *EventBus) Subscribe(eventType EventType, listener EventListener) SubscriptionID {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	b.listeners[eventType] = append(b.listeners[eventType], subscription{id: id, listener: listener})
	return id
}

// SubscribeAll registers a listener that receives all events regardless of type.
// Returns a SubscriptionID that can be used to unsubscribe.
func (b *EventBus) SubscribeAll(listener EventListener) SubscriptionID {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	b.all = append(b.all, subscription{id: id, listener: listener})
	return id
}

// Unsubscribe removes a listener by its subscription ID.
// Returns true if the subscription was found and removed, false otherwise.
func (b *EventBus) Unsubscribe(id SubscriptionID) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for et, subs := range b.listeners {
		for i, sub := range subs {
			if sub.id == id {
				b.listeners[et] = append(subs[:i], subs[i+1:]...)
				return true
			}
		}
	}
	for i, sub := range b.all {
		if sub.id == id {
			b.all = append(b.all[:i], b.all[i+1:]...)
			return true
		}
	}
	return false
}

// Emit dispatches an event to all registered listeners asynchronously.
// Each listener runs in its own goroutine via the worker pool.
// Panics in listeners are recovered and logged.
// If a listener exceeds the timeout, its context is canceled and the worker is released.
func (b *EventBus) Emit(event Event) {
	b.mu.RLock()
	specificListeners := b.listeners[event.Type]
	allTargets := make([]subscription, 0, len(specificListeners)+len(b.all))
	allTargets = append(allTargets, specificListeners...)
	allTargets = append(allTargets, b.all...)
	b.mu.RUnlock()

	for _, sub := range allTargets {
		b.workerPool <- struct{}{}
		b.wg.Add(1)
		metrics.EventBusWorkersActive.Inc()
		go func(l EventListener, e Event) {
			defer func() {
				metrics.EventBusWorkersActive.Dec()
				<-b.workerPool
				b.wg.Done()
				if r := recover(); r != nil {
					b.logger.Error("EventBus: panic in listener", "recover", r)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), b.listenerTimeout)
			defer cancel()

			done := make(chan struct{})
			go func() {
				defer func() {
					if r := recover(); r != nil {
						b.logger.Error("EventBus: panic in listener", "recover", r)
					}
					close(done)
				}()
				l(e)
			}()

			select {
			case <-done:
			case <-ctx.Done():
				b.logger.Warn("EventBus: listener timeout", "timeout", b.listenerTimeout, "event_type", e.Type)
			}
		}(sub.listener, event)
	}
}

// Shutdown waits for all pending event handlers to complete, with a 30-second timeout.
// After timeout, logs a warning and returns even if handlers are still running.
func (b *EventBus) Shutdown() {
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		b.logger.Warn("EventBus: shutdown timeout, some workers may still be running", "timeout", "30s")
	}
}

// EmitConnected emits a connection event with the remote address.
func (b *EventBus) EmitConnected(addr string) {
	b.Emit(Event{Type: EventConnected, Data: ConnectedData{Addr: addr}})
}

// EmitDisconnected emits a disconnection event with the reason.
func (b *EventBus) EmitDisconnected(reason string) {
	b.Emit(Event{Type: EventDisconnected, Data: DisconnectedData{Reason: reason}})
}

// EmitError emits an error event.
func (b *EventBus) EmitError(err error) {
	b.Emit(Event{Type: EventError, Data: ErrorData{Err: err}})
}

// EmitReconnecting emits a reconnection event with attempt information.
func (b *EventBus) EmitReconnecting(attempt, maxAttempts int) {
	b.Emit(Event{Type: EventReconnecting, Data: ReconnectingData{Attempt: attempt, MaxAttempts: maxAttempts}})
}

// EmitStatsUpdate emits a statistics update event.
func (b *EventBus) EmitStatsUpdate(stats ConnectionStats) {
	b.Emit(Event{Type: EventStatsUpdate, Data: StatsUpdateData{Stats: stats}})
}

// EmitClientJoined emits a client join event with client ID and address.
func (b *EventBus) EmitClientJoined(clientID, addr string) {
	b.Emit(Event{Type: EventClientJoined, Data: ClientJoinedData{ClientID: clientID, Addr: addr}})
}

// EmitClientLeft emits a client leave event with the client ID.
func (b *EventBus) EmitClientLeft(clientID string) {
	b.Emit(Event{Type: EventClientLeft, Data: ClientLeftData{ClientID: clientID}})
}

// EmitChatMessage emits a chat message event. Used by Runner implementations
// to surface incoming (remote) chat traffic to API/WS subscribers.
func (b *EventBus) EmitChatMessage(from, to, text string, ts int64) {
	b.Emit(Event{Type: EventChatMessage, Data: ChatMessageData{
		From: from,
		To:   to,
		Text: text,
		TS:   ts,
	}})
}

// EventHandler provides convenience methods for subscribing to specific event types.
// It wraps an EventBus and provides type-safe callback registration.
type EventHandler struct {
	bus *EventBus
}

type EventHandlerOption func(*EventHandler)

func WithEventHandlerWorkerPoolSize(size int) EventHandlerOption {
	return func(h *EventHandler) {
		h.bus = NewEventBus(WithWorkerPoolSize(size))
	}
}

func NewEventHandler(opts ...EventHandlerOption) *EventHandler {
	h := &EventHandler{bus: NewEventBus()}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *EventHandler) Bus() *EventBus {
	return h.bus
}

// SetOnConnected registers a callback for connection events.
// The callback receives the remote address string.
func (h *EventHandler) SetOnConnected(fn func(addr string)) {
	h.bus.Subscribe(EventConnected, func(e Event) {
		if d, ok := e.Data.(ConnectedData); ok {
			fn(d.Addr)
		}
	})
}

// SetOnDisconnected registers a callback for disconnection events.
// The callback receives the disconnection reason string.
func (h *EventHandler) SetOnDisconnected(fn func(reason string)) {
	h.bus.Subscribe(EventDisconnected, func(e Event) {
		if d, ok := e.Data.(DisconnectedData); ok {
			fn(d.Reason)
		}
	})
}

// SetOnReconnecting registers a callback for reconnection events.
// The callback receives the current attempt number and max attempts.
func (h *EventHandler) SetOnReconnecting(fn func(attempt, maxAttempts int)) {
	h.bus.Subscribe(EventReconnecting, func(e Event) {
		if d, ok := e.Data.(ReconnectingData); ok {
			fn(d.Attempt, d.MaxAttempts)
		}
	})
}

// SetOnError registers a callback for error events.
func (h *EventHandler) SetOnError(fn func(err error)) {
	h.bus.Subscribe(EventError, func(e Event) {
		if d, ok := e.Data.(ErrorData); ok {
			fn(d.Err)
		}
	})
}

// SetOnStatsUpdate registers a callback for periodic statistics updates.
func (h *EventHandler) SetOnStatsUpdate(fn func(stats ConnectionStats)) {
	h.bus.Subscribe(EventStatsUpdate, func(e Event) {
		if d, ok := e.Data.(StatsUpdateData); ok {
			fn(d.Stats)
		}
	})
}

// SetOnClientJoined registers a callback for client connection events (server mode).
func (h *EventHandler) SetOnClientJoined(fn func(clientID, addr string)) {
	h.bus.Subscribe(EventClientJoined, func(e Event) {
		if d, ok := e.Data.(ClientJoinedData); ok {
			fn(d.ClientID, d.Addr)
		}
	})
}

// SetOnClientLeft registers a callback for client disconnection events (server mode).
func (h *EventHandler) SetOnClientLeft(fn func(clientID string)) {
	h.bus.Subscribe(EventClientLeft, func(e Event) {
		if d, ok := e.Data.(ClientLeftData); ok {
			fn(d.ClientID)
		}
	})
}
