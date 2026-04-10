package api

import (
	"context"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

// wsBroadcaster is the minimal subset of WSHub used by BridgeEventsToWS.
// Declared as an interface to allow fake implementations in tests.
type wsBroadcaster interface {
	Broadcast(event WSEvent)
}

// BridgeEventsToWS subscribes to lifecycle events on the given EventBus and
// re-broadcasts them to all connected WebSocket clients of the given hub.
//
// The bridge runs until ctx is canceled, at which point it unsubscribes from
// the EventBus and returns. If bus or hub is nil, the function returns
// immediately.
//
// Mapped event types (EventBus → WSEvent.Type):
//
//	EventConnected     → "connected"
//	EventDisconnected  → "disconnected"
//	EventError         → "error"
//	EventStatsUpdate   → "stats_updated"
//	EventClientJoined  → "client_joined"
//	EventClientLeft    → "client_left"
//
// The "status_changed" event type listed in the spec maps to
// EventConnected/EventDisconnected — there is no dedicated EventBus type for
// it, so callers receiving "connected"/"disconnected" over WS should treat
// them as status transitions.
func BridgeEventsToWS(ctx context.Context, bus *echowarp.EventBus, hub wsBroadcaster, logger *slog.Logger) {
	if bus == nil || hub == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	handler := func(wsType string) echowarp.EventListener {
		return func(e echowarp.Event) {
			hub.Broadcast(WSEvent{Type: wsType, Data: e.Data})
		}
	}

	subs := []echowarp.SubscriptionID{
		bus.Subscribe(echowarp.EventConnected, handler("connected")),
		bus.Subscribe(echowarp.EventDisconnected, handler("disconnected")),
		bus.Subscribe(echowarp.EventError, func(e echowarp.Event) {
			// Error events carry an error interface; marshal a string form so
			// JSON clients get a readable payload instead of an empty object.
			data := map[string]string{}
			if ed, ok := e.Data.(echowarp.ErrorData); ok && ed.Err != nil {
				data["error"] = ed.Err.Error()
			}
			hub.Broadcast(WSEvent{Type: "error", Data: data})
		}),
		bus.Subscribe(echowarp.EventStatsUpdate, handler("stats_updated")),
		bus.Subscribe(echowarp.EventClientJoined, handler("client_joined")),
		bus.Subscribe(echowarp.EventClientLeft, handler("client_left")),
	}

	logger.Debug("events → WS bridge started", "subscriptions", len(subs))

	<-ctx.Done()

	for _, id := range subs {
		bus.Unsubscribe(id)
	}
	logger.Debug("events → WS bridge stopped")
}
