package echowarp

import (
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// Option is a functional option for configuring a Node.
// Options are applied during Node construction via NewNode.
type Option func(*Node)

// WithLogger sets a custom structured logger for the Node.
// If nil is provided, the default slog logger is retained.
func WithLogger(logger *slog.Logger) Option {
	return func(n *Node) {
		if logger != nil {
			n.logger = logger
		}
	}
}

// WithBanManager sets a ban manager for tracking failed authentication attempts
// and blocking malicious IPs. If nil, no ban tracking is performed.
func WithBanManager(bm ban.BanManager) Option {
	return func(n *Node) {
		n.banMgr = bm
	}
}

// WithEventHandler sets an event handler for receiving async notifications
// about connection state, errors, and statistics updates.
func WithEventHandler(h *EventHandler) Option {
	return func(n *Node) {
		n.events = h
	}
}

// WithRunnerFactory sets the factory function that creates a Runner (ServerApp or ClientApp)
// based on the configuration. This is required for the Node to function.
func WithRunnerFactory(factory RunnerFactory) Option {
	return func(n *Node) {
		n.runnerFactory = factory
	}
}
