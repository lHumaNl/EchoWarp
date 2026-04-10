package echowarp

import (
	"context"
	"crypto/tls"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// Runner defines the interface for the main execution loop of a Node.
// Implementations (ServerApp, ClientApp) handle the actual streaming workflow.
type Runner interface {
	// Run executes the streaming workflow. It blocks until the context is canceled
	// or a fatal error occurs. Returns nil on graceful shutdown.
	Run(ctx context.Context) error
}

// DeviceCommandReceiver is an optional interface that Runner implementations
// may satisfy to accept device-level control commands (mute / volume) issued
// via the public Node API (Node.SetDeviceMute, Node.SetDeviceVolume).
//
// Runners that do not implement this interface cause the corresponding Node
// methods to return an ErrNotRunning-class error. This keeps the core Runner
// contract minimal while still allowing typed device control without the
// caller knowing the concrete runner type.
type DeviceCommandReceiver interface {
	// HandleDeviceCommand enqueues a device command for processing by the
	// runner. Implementations should return quickly (non-blocking, or with
	// a short timeout) and must be safe to call from any goroutine while
	// the runner is executing Run(ctx).
	HandleDeviceCommand(cmd DeviceCommand) error
}

// RunnerFactory creates a Runner instance based on configuration.
// This function bridges the pkg/echowarp package with internal/app implementations,
// avoiding direct imports that would break the package boundary.
type RunnerFactory func(cfg NodeConfig, logger *slog.Logger, banMgr ban.BanManager, tlsConfig *tls.Config, rateLimiter *auth.IPRateLimiter) (Runner, error)
