package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/api"
	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/daemon"
	"github.com/lHumaNl/echowarp/internal/logging"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// newDaemonCmd creates the "daemon" command group for daemon lifecycle management.
// Subcommands: start, stop, status.
func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Daemon management commands",
		Long:  "Commands to start, stop, and check the status of the EchoWarp daemon.",
	}
	cmd.AddCommand(
		newDaemonStartCmd(),
		newDaemonStopCmd(),
		newDaemonStatusCmd(),
	)
	return cmd
}

// newDaemonStartCmd creates the "daemon start" subcommand.
// Starts EchoWarp as a background server process with a PID file.
// Logs are written to a file (default or --log-file).
//
// Examples:
//
//	echowarp daemon start --device 1 --password secret
//	echowarp daemon start --api-port 8080 --api-token mytoken
func newDaemonStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the EchoWarp daemon",
		Long:  "Start the EchoWarp daemon in the background as a server.",
		RunE:  runDaemonStart,
	}

	cmd.Flags().IntP("port", "p", 4415, "TCP port for signaling")
	cmd.Flags().UintP("device", "d", 0, "Audio device ID")
	cmd.Flags().StringP("password", "P", "", "Password for authentication")
	cmd.Flags().BoolP("reverse", "r", false, "Reverse mode: server receives audio")
	cmd.Flags().Int("sample-rate", 48000, "Sample rate")
	cmd.Flags().Int("channels", 1, "Number of channels (1=mono, 2=stereo)")
	cmd.Flags().Int("max-clients", 1, "Maximum number of clients")
	cmd.Flags().Int("max-auth-failures", 5, "Max failed auth attempts before ban (0=disabled)")
	cmd.Flags().Int("max-failed", 5, "")
	_ = cmd.Flags().MarkHidden("max-failed")
	_ = cmd.Flags().MarkDeprecated("max-failed", "use --max-auth-failures instead")
	cmd.Flags().Int("max-reconnect", 5, "Max reconnect attempts (0=infinite)")
	cmd.Flags().Bool("virtual-mic", false, "Create virtual microphone")
	cmd.Flags().StringSlice("stun-server", nil, "STUN servers")
	cmd.Flags().String("tls-cert", "", "TLS certificate path")
	cmd.Flags().String("tls-key", "", "TLS key path")
	cmd.Flags().StringP("config", "c", "", "Path to YAML config file")
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().String("log-file", "", "Log to file (required for daemon)")
	cmd.Flags().String("ban-file", "", "Ban list file path")
	cmd.Flags().Bool("no-discovery", false, "Don't publish via mDNS")
	cmd.Flags().String("server-name", "", "Custom server name (defaults to hostname)")
	cmd.Flags().Int("rate-limit", 5, "Max connections per second per IP (0=disabled)")
	cmd.Flags().String("pid-file", "", "PID file path (default: /tmp/echowarp.pid)")
	cmd.Flags().Int("api-port", 8080, "API server port")
	cmd.Flags().String("api-bind", "127.0.0.1", "API server bind address")
	cmd.Flags().String("api-token", "", "API authentication token")
	cmd.Flags().Bool("enable-pprof", false, "Enable pprof profiling endpoints")
	cmd.Flags().StringSlice("trusted-proxies", nil, "Trusted proxy IPs/CIDRs for X-Forwarded-For processing")
	return cmd
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	d, cfg, logger, err := setupDaemon(cmd)
	if err != nil {
		return err
	}

	if err = d.WritePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer func() { _ = d.RemovePID() }()

	logger.Info("Daemon starting", "pid", os.Getpid(), "pidFile", d.PIDFile())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	flags := getDaemonFlags(cmd)
	rateLimiter := createDaemonRateLimiter(flags.rateLimit)
	setupDaemonDiscovery(flags.noDiscovery, flags.serverName, logger)

	node, err := createNode(cfg, logger, rateLimiter)
	if err != nil {
		return err
	}

	apiServer := startAPIServer(ctx, node, flags, logger)
	defer func() { _ = apiServer.Stop() }()

	if err := node.Start(ctx); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	<-ctx.Done()
	logger.Info("Shutting down...")
	_ = node.Stop()

	logger.Info("Daemon stopped")
	return nil
}

func setupDaemon(cmd *cobra.Command) (*daemon.Daemon, config.Config, *slog.Logger, error) {
	pidFile, _ := cmd.Flags().GetString("pid-file")
	d := daemon.New(pidFile)

	if err := checkIfRunning(d); err != nil {
		return nil, config.Config{}, nil, err
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		return nil, config.Config{}, nil, err
	}

	if errs := cfg.Validate(); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", e)
		}
		return nil, config.Config{}, nil, fmt.Errorf("invalid configuration")
	}

	logger, err := setupLogger(cfg)
	if err != nil {
		return nil, config.Config{}, nil, err
	}

	return d, cfg, logger, nil
}

type daemonFlags struct {
	banFilePath    string
	noDiscovery    bool
	serverName     string
	rateLimit      int
	apiPort        int
	apiBind        string
	apiToken       string
	enablePprof    bool
	trustedProxies []string
}

func checkIfRunning(d *daemon.Daemon) error {
	if running, existingPID := d.IsRunning(); running {
		return fmt.Errorf("daemon is already running with PID %d", existingPID)
	}
	return nil
}

func setupLogger(cfg config.Config) (*slog.Logger, error) {
	logFile := cfg.LogFile
	if logFile == "" {
		logFile = filepath.Join(config.EchoWarpDir(), "logs", "daemon.log")
	}

	logger, _, err := logging.NewCombinedLogger(cfg.LogLevel, logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return logger, nil
}

func getDaemonFlags(cmd *cobra.Command) daemonFlags {
	banFilePath, _ := cmd.Flags().GetString("ban-file")
	if banFilePath == "" {
		banFilePath = filepath.Join(config.EchoWarpDir(), "ban_list.yaml")
	}

	return daemonFlags{
		banFilePath:    banFilePath,
		noDiscovery:    getFlagBool(cmd, "no-discovery"),
		serverName:     getFlagString(cmd, "server-name"),
		rateLimit:      getFlagInt(cmd, "rate-limit"),
		apiPort:        getFlagInt(cmd, "api-port"),
		apiBind:        getFlagString(cmd, "api-bind"),
		apiToken:       getFlagString(cmd, "api-token"),
		enablePprof:    getFlagBool(cmd, "enable-pprof"),
		trustedProxies: getFlagStringSlice(cmd, "trusted-proxies"),
	}
}

func getFlagBool(cmd *cobra.Command, name string) bool {
	val, _ := cmd.Flags().GetBool(name)
	return val
}

func getFlagString(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

func getFlagInt(cmd *cobra.Command, name string) int {
	val, _ := cmd.Flags().GetInt(name)
	return val
}

func getFlagStringSlice(cmd *cobra.Command, name string) []string {
	val, _ := cmd.Flags().GetStringSlice(name)
	return val
}

func createDaemonRateLimiter(rateLimit int) *auth.IPRateLimiter {
	if rateLimit > 0 {
		return auth.NewIPRateLimiter(rateLimit)
	}
	return nil
}

func setupDaemonDiscovery(noDiscovery bool, serverName string, logger *slog.Logger) {
	if !noDiscovery {
		name := serverName
		if name == "" {
			name, _ = os.Hostname()
		}
		logger.Info("mDNS discovery enabled", "name", name)
	}
}

func createNode(cfg config.Config, logger *slog.Logger, rateLimiter *auth.IPRateLimiter) (*echowarp.Node, error) {
	nodeCfg := convertToNodeConfig(cfg)
	node, err := echowarp.NewNode(nodeCfg,
		echowarp.WithLogger(logger),
		echowarp.WithRunnerFactory(createRunnerFactory(cfg, logger, rateLimiter)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	return node, nil
}

func startAPIServer(ctx context.Context, node *echowarp.Node, flags daemonFlags, logger *slog.Logger) *api.APIServer {
	apiAddr := fmt.Sprintf("%s:%d", flags.apiBind, flags.apiPort)
	apiServer := api.NewAPIServerWithOptions(node, apiAddr, flags.apiToken, logger,
		api.WithPprof(flags.enablePprof),
		api.WithTrustedProxies(flags.trustedProxies),
	)

	if err := apiServer.Start(ctx); err != nil {
		logger.Error("failed to start API server", "error", err)
		return nil
	}

	logger.Info("API server started", "addr", apiAddr, "pprof", flags.enablePprof)
	return apiServer
}

// newDaemonStopCmd creates the "daemon stop" subcommand.
// Sends SIGTERM to the daemon process via its PID file.
func newDaemonStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the EchoWarp daemon",
		Long:  "Stop the running EchoWarp daemon by sending SIGTERM.",
		RunE:  runDaemonStop,
	}

	cmd.Flags().String("pid-file", "", "PID file path (default: /tmp/echowarp.pid)")

	return cmd
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	pidFile, _ := cmd.Flags().GetString("pid-file")
	d := daemon.New(pidFile)

	running, pid := d.IsRunning()
	if !running {
		fmt.Println("Daemon is not running")
		return nil
	}

	if err := d.Stop(); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	fmt.Printf("Sent SIGTERM to daemon (PID %d)\n", pid)
	return nil
}

// newDaemonStatusCmd creates the "daemon status" subcommand.
// Reports whether the daemon is running and its PID.
func newDaemonStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check daemon status",
		Long:  "Check if the EchoWarp daemon is running and display its PID.",
		RunE:  runDaemonStatus,
	}

	cmd.Flags().String("pid-file", "", "PID file path (default: /tmp/echowarp.pid)")

	return cmd
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	pidFile, _ := cmd.Flags().GetString("pid-file")
	d := daemon.New(pidFile)

	running, pid := d.IsRunning()
	if running {
		fmt.Printf("Daemon is running (PID %d)\n", pid)
		fmt.Printf("PID file: %s\n", d.PIDFile())
	} else {
		fmt.Println("Daemon is not running")
		if _, err := d.ReadPID(); err == nil {
			fmt.Printf("Stale PID file exists: %s\n", d.PIDFile())
		}
	}
	return nil
}

func convertToNodeConfig(cfg config.Config) echowarp.NodeConfig {
	nodeCfg := echowarp.NodeConfig{
		Mode:            cfg.Mode,
		Reverse:         cfg.Reverse,
		Duplex:          cfg.Duplex,
		Conference:      cfg.Conference,
		Port:            cfg.Port,
		Address:         cfg.Address,
		DeviceID:        cfg.DeviceID,
		SampleRate:      cfg.SampleRate,
		Channels:        cfg.Channels,
		VirtualMic:      cfg.VirtualMic,
		OpusBitrate:     cfg.OpusBitrate,
		OpusComplexity:  cfg.OpusComplexity,
		OpusApplication: cfg.OpusApplication,
		OpusDTX:         cfg.OpusDTX,
		OpusFEC:         cfg.OpusFEC,
		Password:        cfg.Password,
		TLSCert:         cfg.TLSCert,
		TLSKey:          cfg.TLSKey,
		TLS:             cfg.IsTLSEnabled(),
		TLSInsecure:     cfg.TLSInsecure,
		TLSSelfSigned:   cfg.TLSSelfSigned,
		MaxClients:      cfg.MaxClients,
		Nickname:        cfg.Nickname,
		HWIDRequired:    cfg.HWIDRequired,
	}

	nodeCfg.STUNServers = append(nodeCfg.STUNServers, cfg.STUNServers...)

	nodeCfg.TURNServers = append(nodeCfg.TURNServers, cfg.TURNServers...)

	return nodeCfg
}

func createRunnerFactory(cfg config.Config, logger *slog.Logger, rateLimiter *auth.IPRateLimiter) echowarp.RunnerFactory {
	return func(_ echowarp.NodeConfig, _ *slog.Logger, banMgr ban.BanManager, tlsConf *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return app.NewServerApp(cfg, logger, banMgr, tlsConf, rateLimiter), nil
	}
}
