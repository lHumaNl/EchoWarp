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

// RunnerFactory creates a Runner instance based on configuration.
// This function bridges the pkg/echowarp package with internal/app implementations,
// avoiding direct imports that would break the package boundary.
type RunnerFactory func(cfg NodeConfig, logger *slog.Logger, banMgr ban.BanManager, tlsConfig *tls.Config, rateLimiter *auth.IPRateLimiter) (Runner, error)
