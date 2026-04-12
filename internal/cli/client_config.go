package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/logging"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

func loadClientConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := loadConfig(cmd, config.ModeClient)
	if err != nil {
		return nil, err
	}

	if cmd.Flags().Changed("address") {
		cfg.Address, _ = cmd.Flags().GetString("address")
	}
	if cmd.Flags().Changed("tls-insecure") {
		cfg.TLSInsecure, _ = cmd.Flags().GetBool("tls-insecure")
	}
	if cmd.Flags().Changed("no-simd-optimization") {
		cfg.NoSIMDOptimization, _ = cmd.Flags().GetBool("no-simd-optimization")
	}
	if cmd.Flags().Changed("no-pool-warmup") {
		cfg.NoPoolWarmup, _ = cmd.Flags().GetBool("no-pool-warmup")
	}
	if cmd.Flags().Changed("nickname") {
		cfg.Nickname, _ = cmd.Flags().GetString("nickname")
	}
	if cmd.Flags().Changed("auto-reconnect") {
		cfg.AutoReconnect, _ = cmd.Flags().GetBool("auto-reconnect")
	}
	if cmd.Flags().Changed("auto-reconnect-attempts") {
		cfg.AutoReconnectAttempts, _ = cmd.Flags().GetInt("auto-reconnect-attempts")
	}
	if cmd.Flags().Changed("record-dir") {
		cfg.RecordDir, _ = cmd.Flags().GetString("record-dir")
	}

	return &cfg, nil
}

func handleClientDiscovery(cmd *cobra.Command, cfg *config.Config) error {
	discover, _ := cmd.Flags().GetBool("discover")
	discoverTimeout, _ := cmd.Flags().GetDuration("discover-timeout")

	if cfg.Address != "" && discover && cmd.Flags().Changed("discover") {
		return fmt.Errorf("cannot use --address and --discover together")
	}

	if cfg.Address != "" || !discover {
		return nil
	}

	noInteractive, _ := cmd.Flags().GetBool("no-interactive")

	// In TUI mode: skip blocking discovery here — TUI handles it asynchronously via Server Picker.
	if !noInteractive {
		return nil
	}

	// --no-interactive: blocking discovery for CLI mode
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	disc := discovery.NewDiscoverer()
	services, err := disc.Discover(ctx, discoverTimeout)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Discovery failed: %v\n", err)
	}

	var servers []discovery.ServiceInfo
	for svc := range services {
		servers = append(servers, svc)
	}

	return selectDiscoveredServer(servers, cfg)
}

func selectDiscoveredServer(servers []discovery.ServiceInfo, cfg *config.Config) error {
	switch len(servers) {
	case 0:
		return fmt.Errorf("no EchoWarp servers found on LAN. Use --address to specify manually")
	case 1:
		setServerAddress(&servers[0], cfg)
		fmt.Printf("Auto-discovered server: %s (%s:%d) auth=%v tls=%v\n",
			servers[0].Name, cfg.Address, cfg.Port, servers[0].AuthReq, servers[0].TLS)
		return nil
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Multiple EchoWarp servers found:\n")
		for _, s := range servers {
			addr := getServerAddress(&s)
			_, _ = fmt.Fprintf(os.Stderr, "  %s (%s:%d) auth=%v tls=%v\n", s.Name, addr, s.Port, s.AuthReq, s.TLS)
		}
		return fmt.Errorf("found %d servers, specify --address explicitly", len(servers))
	}
}

func setServerAddress(s *discovery.ServiceInfo, cfg *config.Config) {
	if len(s.AddrIPv4) > 0 {
		cfg.Address = s.AddrIPv4[0].String()
	} else if len(s.AddrIPv6) > 0 {
		cfg.Address = s.AddrIPv6[0].String()
	}
	cfg.Port = s.Port
}

func getServerAddress(s *discovery.ServiceInfo) string {
	if len(s.AddrIPv4) > 0 {
		return s.AddrIPv4[0].String()
	} else if len(s.AddrIPv6) > 0 {
		return s.AddrIPv6[0].String()
	}
	return ""
}

func runClientDirect(cfg *config.Config) error {
	if cfg.NoSIMDOptimization {
		audio.DisableSIMD()
	}

	if !cfg.NoPoolWarmup {
		audio.WarmupPools()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Use client-specific log file if not explicitly set
	logFile := cfg.LogFile
	if logFile == "" {
		logFile = logging.GetDefaultLogFile("client")
	}

	logger, logCloser, err := logging.NewCombinedLogger(cfg.LogLevel, logFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: file logging failed (%v), using console only\n", err)
		logger = logging.NewConsoleLogger(cfg.LogLevel)
		logCloser = logging.NopCloser
	}
	defer logCloser.Close() //nolint:errcheck

	_, _ = fmt.Fprintf(os.Stderr, "Logging to: %s\n", logFile)
	logger.Info("Client logger initialized", "log_file", logFile)

	// Probe server to get authoritative config (mode, sample_rate, channels, etc.)
	probeResult, probeErr := probe.ProbeServer(cfg.Address, cfg.Port)
	if probeErr != nil {
		logger.Error("Server not reachable", "address", cfg.Address, "port", cfg.Port, "error", probeErr)
		return fmt.Errorf("server not reachable at %s:%d: %w", cfg.Address, cfg.Port, probeErr)
	}
	if err := probe.ApplyProbeToConfig(cfg, probeResult); err != nil {
		return err
	}
	logger.Info("Server config applied",
		"mode", probeResult.Mode,
		"sample_rate", probeResult.SampleRate,
		"channels", probeResult.Channels,
		"opus_bitrate", probeResult.OpusBitrate,
		"tls", probeResult.TLSRequired)

	if cfg.VirtualMic {
		injectVirtualMicDevice(cfg, logger)
	}

	if cfg.TLSInsecure {
		logger.Warn("TLS certificate verification is DISABLED. This is insecure and should only be used for testing!")
	}

	var tlsConfig *tls.Config
	if cfg.TLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecure,
			MinVersion:         tls.VersionTLS12,
		}
	}

	serverStoppedCh := make(chan struct{})
	clientApp := app.NewClientApp(*cfg, logger, tlsConfig).
		WithServerStoppedChannel(serverStoppedCh)

	runErr := app.RunWithReconnect(
		ctx, logger,
		cfg.MaxReconnectAttempts,
		time.Duration(cfg.ReconnectIntervalSec)*time.Second,
		clientApp.Run,
	)

	// Check if the server sent ActionStop (graceful shutdown).
	select {
	case <-serverStoppedCh:
		if cfg.AutoReconnect {
			logger.Info("Server has shut down, waiting to reconnect...")
			return runClientAutoReconnectLoop(ctx, logger, cfg)
		}
		logger.Info("Server has shut down")
		return nil
	default:
		return runErr
	}
}

// runClientAutoReconnectLoop implements CLI auto-reconnect after server graceful shutdown.
// Uses exponential backoff: 2s, 4s, 8s, 16s, max 30s.
// On critical param change: logs error and exits (no interactive prompt).
func runClientAutoReconnectLoop(ctx context.Context, logger *slog.Logger, cfg *config.Config) error {
	maxAttempts := cfg.AutoReconnectAttempts // 0 = infinite
	attempt := 0
	backoff := 2 * time.Second
	const maxBackoff = 30 * time.Second

	for {
		attempt++
		if maxAttempts > 0 && attempt > maxAttempts {
			logger.Error("Max auto-reconnect attempts reached", "max", maxAttempts)
			return fmt.Errorf("max auto-reconnect attempts (%d) reached", maxAttempts)
		}

		logger.Info("Waiting before reconnect", "attempt", attempt, "delay", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff: 2s, 4s, 8s, 16s, 30s, 30s, ...
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Probe server.
		probeResult, err := probe.ProbeServer(cfg.Address, cfg.Port)
		if err != nil {
			logger.Warn("Probe failed, will retry", "error", err, "attempt", attempt)
			continue
		}

		// Check critical changes.
		changes := probe.CompareServerParams(*cfg, probeResult)
		hasCritical := false
		for _, ch := range changes {
			if ch.Critical {
				hasCritical = true
				logger.Error("Critical server config change",
					"field", ch.Field, "old", ch.OldValue, "new", ch.NewValue)
			}
		}
		if hasCritical {
			return fmt.Errorf("server configuration changed critically, manual reconfiguration required")
		}

		// Apply non-critical changes.
		_ = probe.ApplyProbeToConfig(cfg, probeResult)

		// Rebuild TLS config in case probe changed it.
		var tlsConfig *tls.Config
		if cfg.TLS {
			tlsConfig = &tls.Config{
				InsecureSkipVerify: cfg.TLSInsecure, //nolint:gosec
				MinVersion:         tls.VersionTLS12,
			}
		}

		// Reconnect.
		logger.Info("Server available, reconnecting...", "attempt", attempt)
		serverStoppedCh := make(chan struct{})
		clientApp := app.NewClientApp(*cfg, logger, tlsConfig).
			WithServerStoppedChannel(serverStoppedCh)

		runErr := app.RunWithReconnect(ctx, logger,
			cfg.MaxReconnectAttempts,
			time.Duration(cfg.ReconnectIntervalSec)*time.Second,
			clientApp.Run)

		select {
		case <-serverStoppedCh:
			// Server stopped again — reset backoff and loop.
			logger.Info("Server has shut down again")
			backoff = 2 * time.Second
			continue
		default:
			return runErr
		}
	}
}
