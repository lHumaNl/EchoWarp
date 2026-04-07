package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/logging"
	"github.com/lHumaNl/echowarp/internal/tui"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// newServerCmd creates the "server" subcommand for starting EchoWarp in server mode.
// In server mode, EchoWarp listens for incoming client connections and streams audio.
//
// If no device is specified with --device, an interactive TUI prompts for device selection.
// When --max-clients > 1, operates in multi-client mode accepting concurrent connections.
//
// Examples:
//
//	echowarp server --port 8080 --password secret
//	echowarp server --device 1 --max-clients 5
//	echowarp server --reverse --device 2  # Server receives audio
func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start EchoWarp in server mode",
		Long:  "Start EchoWarp as a server, capturing audio and streaming to connected clients.",
		RunE:  runServer,
	}

	cmd.Flags().IntP("port", "p", 4415, "TCP port for signaling")
	cmd.Flags().UintP("device", "d", 0, "Audio device ID (repeatable in multi-device mode)")
	cmd.Flags().UintSlice("capture-device", nil, "Capture device ID (duplex mode, repeatable)")
	cmd.Flags().UintSlice("playback-device", nil, "Playback device ID (duplex mode, repeatable)")
	cmd.Flags().StringP("password", "P", "", "Password for authentication")
	cmd.Flags().StringP("mode", "m", "normal", "Audio mode: normal, reverse, duplex, conference")
	cmd.Flags().BoolP("reverse", "r", false, "Alias for --mode=reverse")
	cmd.Flags().BoolP("duplex", "X", false, "Alias for --mode=duplex")
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
	cmd.Flags().StringP("save-config", "s", "", "Save current settings to file")
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().String("log-file", "", "Log to file")
	cmd.Flags().String("ban-file", "", "Ban list file path")
	cmd.Flags().Bool("no-discovery", false, "Don't publish via mDNS")
	cmd.Flags().StringP("device-name", "D", "", "Select audio device by name substring (case-insensitive)")
	cmd.Flags().String("server-name", "", "Custom server name (defaults to hostname)")
	cmd.Flags().Int("rate-limit", 5, "Max connections per second per IP (0=disabled)")
	cmd.Flags().Bool("no-simd-optimization", false, "Disable SIMD audio optimizations (use pure Go)")
	cmd.Flags().Bool("no-pool-warmup", false, "Disable audio buffer pool warmup on startup")
	cmd.Flags().Int("audio-buffer-frames", 5, "Audio channel buffer size in frames (each frame=20ms). Lower=less latency, higher=more stability")
	cmd.Flags().Bool("dry-run", false, "Validate config and check device, then exit without starting")
	cmd.Flags().Bool("loopback", false, "Enable loopback capture of system audio (macOS, requires BlackHole)")
	cmd.Flags().Bool("no-interactive", false, "Disable TUI: requires --device, logs to stdout/file")
	cmd.Flags().Bool("aec", false, "Enable acoustic echo cancellation (duplex mode)")
	cmd.Flags().Bool("conference", false, "Alias for --mode=conference")
	cmd.Flags().Bool("conf", false, "Alias for --conference")
	_ = cmd.Flags().MarkHidden("conf")
	_ = cmd.Flags().MarkHidden("no-simd-optimization")
	_ = cmd.Flags().MarkHidden("no-pool-warmup")
	cmd.Flags().Bool("server-muted", false, "Server does not contribute audio in conference mode")
	cmd.Flags().String("record", "", "Start recording immediately: mix, tracks, or both (conference mode)")
	cmd.Flags().Bool("hwid-required", false, "Require clients to send hardware ID (for bans)")

	applyGroupedUsage(cmd, []flagGroup{
		{"Audio", []string{"device", "device-name", "capture-device", "playback-device", "sample-rate", "channels", "virtual-mic", "loopback", "aec", "audio-buffer-frames"}},
		{"Network", []string{"port", "stun-server", "tls-cert", "tls-key", "no-discovery", "server-name", "rate-limit"}},
		{"Security", []string{"password", "max-auth-failures", "ban-file", "hwid-required"}},
		{"Conference", []string{"conference", "max-clients", "server-muted", "record"}},
		{"Mode", []string{"reverse", "duplex"}},
		{"Config", []string{"config", "save-config"}},
		{"Logging & debug", []string{"log-level", "log-file", "dry-run", "no-interactive", "max-reconnect"}},
	})

	return cmd
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		return err
	}

	if err = validateAndSaveConfig(cmd, &cfg); err != nil { //nolint:gocritic // avoiding shadow
		return err
	}

	if deviceName, _ := cmd.Flags().GetString("device-name"); deviceName != "" {
		var id *uint32
		id, err = resolveDeviceByName(deviceName, !cfg.Reverse)
		if err != nil {
			return err
		}
		cfg.DeviceID = id
	}

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return runDryRun(&cfg)
	}

	noInteractive, _ := cmd.Flags().GetBool("no-interactive")
	if noInteractive {
		if cfg.DeviceID == nil {
			return fmt.Errorf("--no-interactive requires --device to be specified")
		}
		err = executeServerApp(cmd, &cfg)
	} else {
		err = runServerStreamingTUI(cmd, cfg)
	}

	if err == nil {
		suggestSaveConfig(cmd, &cfg)
	}

	return err
}

func runServerInteractive(_ *cobra.Command, cfg *config.Config) error {
	audio.CleanupOrphanedAggregateDevices()

	dm, err := audio.NewDeviceManager()
	if err != nil {
		return fmt.Errorf("failed to initialize audio: %w", err)
	}
	defer dm.Close() //nolint:errcheck

	// Load all devices (input + output + loopback) for unified TUI.
	// Section visibility is handled by the TUI based on mode.
	devices, err := loadAllDevicesWithLoopback(dm)
	if err != nil {
		return err
	}

	tuiModel := tui.NewModel(*cfg, devices)
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("invalid model type: expected tui.Model, got %T", finalModel)
	}
	if m.Quitting() {
		return nil
	}
	if m.SelectedDeviceID() != nil {
		cfg.DeviceID = m.SelectedDeviceID()
	}
	return nil
}

func executeServerApp(cmd *cobra.Command, cfg *config.Config) error {
	if cfg.NoSIMDOptimization {
		audio.DisableSIMD()
	}

	if !cfg.NoPoolWarmup {
		audio.WarmupPools()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Use server-specific log file if not explicitly set
	logFile := cfg.LogFile
	if logFile == "" {
		logFile = logging.GetDefaultLogFile("server")
	}

	logger, logCloser, err := logging.NewCombinedLogger(cfg.LogLevel, logFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: file logging failed (%v), using console only\n", err)
		logger = logging.NewConsoleLogger(cfg.LogLevel)
		logCloser = logging.NopCloser
	}
	defer logCloser.Close() //nolint:errcheck

	_, _ = fmt.Fprintf(os.Stderr, "Logging to: %s\n", logFile)
	logger.Info("Server logger initialized", "log_file", logFile)

	if cfg.VirtualMic {
		injectVirtualMicDevice(cfg, logger)
	}

	banMgr, err := setupBanManager(cfg, logger)
	if err == nil && banMgr != nil {
		defer banMgr.Close() //nolint:errcheck
	}

	tlsConfig, err := setupTLSConfig(cfg)
	if err != nil {
		return err
	}

	rateLimiter := setupRateLimiter(cmd)
	setupDiscovery(ctx, cmd, cfg, logger)
	startSessionInfoServer(ctx, *cfg, logger)

	serverApp := app.NewServerApp(*cfg, logger, banMgr, tlsConfig, rateLimiter)
	return serverApp.Run(ctx)
}

// runServerStreamingTUI launches the server with a full TUI using an already-parsed config.
// Used when the user runs `echowarp server` without --no-interactive.
func runServerStreamingTUI(cmd *cobra.Command, cfg config.Config) error {
	if cfg.NoSIMDOptimization {
		audio.DisableSIMD()
	}
	if !cfg.NoPoolWarmup {
		audio.WarmupPools()
	}

	audio.CleanupOrphanedAggregateDevices()

	dm, err := audio.NewDeviceManager()
	if err != nil {
		return fmt.Errorf("audio init: %w", err)
	}
	devices, err := listAllDevices(dm)
	if err != nil {
		_ = dm.Close()
		return err
	}
	needsInput := cfg.Duplex || !cfg.Reverse
	devices, loopbackMap := appendLoopbackDevices(dm, devices, needsInput)

	_ = dm.Close()

	statsCh := make(chan transport.ConnectionStats, 4)
	errCh := make(chan error, 4)
	logCh := make(chan tui.LogEntry, 200)
	// Playback (decode/incoming) spectrum and level meter.
	spectrum := audio.NewSpectrumAnalyzer(cfg.SampleRate, audio.DefaultBands)
	levelMeter := audio.NewLevelMeter(cfg.Channels)
	// Capture (outgoing) spectrum and level meter — separate instances for duplex/conference.
	captureSpectrum := audio.NewSpectrumAnalyzer(cfg.SampleRate, audio.DefaultBands)
	captureLevelMeter := audio.NewLevelMeter(cfg.Channels)
	multiStatsCh := make(chan transport.MultiClientStats, 4)
	cmdCh := make(chan tui.ClientCommand, 16)
	appCmdCh := make(chan app.ClientCommand, 16)
	appConferenceStatsCh := make(chan app.ConferenceStatsPayload, 4)
	conferenceStatsCh := make(chan tui.ConferenceStatsPayload, 4)
	participantCmdCh := make(chan tui.ParticipantCommand, 16)
	appParticipantCmdCh := make(chan app.ParticipantCommand, 16)
	recordingCmdCh := make(chan tui.RecordingCommand, 4)
	appRecordingCmdCh := make(chan app.RecordingCommand, 4)
	chatMsgCh := make(chan app.ChatMessage, 32)
	serverPauseCh := make(chan bool, 4)
	var chatServerApp *app.ServerApp

	// clientCount tracks connected clients for session info server probe response.
	var clientCount atomic.Int32

	// banMgrRef is captured by the BanListFunc closure below.
	var banMgrRef ban.BanManager
	// aecRef is captured by the WithAECToggle closure below.
	var aecRef *audio.AECProcessor

	startFunc := tui.StartFunc(func(selectedCfg config.Config, stopCh <-chan struct{}) (<-chan transport.ConnectionStats, <-chan error, <-chan struct{}) {
		// Check if the selected device is a loopback device by looking up the device name.
		// The TUI sets DeviceID based on the device list index. Find the matching device.
		// In conference hub mode, DeviceID may be nil (no devices selected).
		if selectedCfg.DeviceID != nil {
			for _, dev := range devices {
				if dev.ID == *selectedCfg.DeviceID {
					if lb, ok := loopbackMap[dev.Name]; ok {
						selectedCfg.Loopback = true
						selectedCfg.LoopbackOutputDevice = lb.OutputDevice.Name
						selectedCfg.LoopbackBlackHole = lb.BlackHole.Name
					}
					break
				}
			}
		}
		// Also check multi-device list for loopback devices.
		for _, de := range selectedCfg.Devices {
			if lb, ok := loopbackMap[de.Name]; ok {
				selectedCfg.Loopback = true
				selectedCfg.LoopbackOutputDevice = lb.OutputDevice.Name
				selectedCfg.LoopbackBlackHole = lb.BlackHole.Name
				break
			}
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		go func() {
			select {
			case <-stopCh:
				// Delay cancel to let ServerApp handle stopCh and send ActionStop to clients first.
				time.Sleep(500 * time.Millisecond)
				cancel()
			case <-ctx.Done():
			}
		}()
		go func() {
			defer cancel()
			tuiHandler := tui.NewChannelLogHandler(logCh, tui.ParseSlogLevel(selectedCfg.LogLevel))
			logFile := selectedCfg.LogFile
			if logFile == "" {
				logFile = logging.GetDefaultLogFile("server")
			}
			logger, logCloser, logErr := logging.NewTUILogger(selectedCfg.LogLevel, logFile, tuiHandler)
			if logErr != nil {
				logger = slog.New(tuiHandler)
			}
			if logCloser != nil {
				defer logCloser.Close() //nolint:errcheck
			}
			// Redirect the global slog logger so package-level slog.Warn calls (e.g. TLS warning)
			// appear in the TUI log panel instead of raw stderr.
			slog.SetDefault(logger)

			banFilePath := selectedCfg.BanFilePath
			if banFilePath == "" {
				configDir, _ := os.UserConfigDir()
				banFilePath = filepath.Join(configDir, "echowarp", "ban_list.yaml")
			}
			banMgr, _ := ban.NewFileBanManager(selectedCfg.MaxFailedAttempts, banFilePath)
			if banMgr != nil {
				defer banMgr.Close() //nolint:errcheck
				banMgrRef = banMgr
			}

			tlsConfig, _ := setupTLSConfig(&selectedCfg)
			var rateLimiter *auth.IPRateLimiter
			if selectedCfg.RateLimit > 0 {
				rateLimiter = auth.NewIPRateLimiter(selectedCfg.RateLimit)
			}
			setupDiscovery(ctx, cmd, &selectedCfg, logger)
			startSessionInfoServer(ctx, selectedCfg, logger, func() int { return int(clientCount.Load()) })

			serverName, _ := os.Hostname()
			txt := discovery.BuildTXTRecords("2.0.0", serverName, selectedCfg.Password != "", selectedCfg.TLSCert != "", 0, selectedCfg.MaxClients, selectedCfg.AudioMode(), config.ServerID(selectedCfg.Port))
			if pub, pubErr := discovery.NewPublisher(serverName, selectedCfg.Port, txt); pubErr == nil {
				go func() {
					if pubErr2 := pub.Publish(ctx); pubErr2 != nil && ctx.Err() == nil {
						logger.Warn("mDNS publishing error", "error", pubErr2)
					}
				}()
			}

			serverApp := app.NewServerApp(selectedCfg, logger, banMgr, tlsConfig, rateLimiter).
				WithStatsChannels(statsCh, errCh).
				WithStopChannel(stopCh).
				WithSpectrum(spectrum).
				WithLevelMeter(levelMeter).
				WithCaptureSpectrum(captureSpectrum).
				WithCaptureLevelMeter(captureLevelMeter).
				WithChatChannel(chatMsgCh)
			chatServerApp = serverApp
			aecRef = serverApp.AEC()

			serverApp.WithClientCountCallback(func(n int) { clientCount.Store(int32(n)) })

			if selectedCfg.Conference {
				serverApp.WithMultiStatsChannel(multiStatsCh).WithCommandChannel(appCmdCh).
					WithConferenceStatsChannel(appConferenceStatsCh).
					WithParticipantCommandChannel(appParticipantCmdCh).
					WithRecordingCommandChannel(appRecordingCmdCh).
					WithServerPauseChannel(serverPauseCh)
				go bridgeParticipantCommands(ctx, participantCmdCh, appParticipantCmdCh)
				go bridgeRecordingCommands(ctx, recordingCmdCh, appRecordingCmdCh)
				go bridgeConferenceStats(ctx, appConferenceStatsCh, conferenceStatsCh)
			} else {
				serverApp.WithMultiStatsChannel(multiStatsCh).WithCommandChannel(appCmdCh)
				go bridgeClientCommands(ctx, cmdCh, appCmdCh)
			}
			// Wire server pause for non-conference modes with capture (normal, duplex).
			if !selectedCfg.Conference && (!selectedCfg.Reverse || selectedCfg.Duplex) {
				serverApp.WithServerPauseChannel(serverPauseCh)
			}

			if runErr := serverApp.Run(ctx); runErr != nil && ctx.Err() == nil {
				sendError(errCh, runErr, logger)
			}
		}()
		return statsCh, errCh, nil
	})

	srvLogFile := cfg.LogFile
	if srvLogFile == "" {
		srvLogFile = logging.GetDefaultLogFile("server")
	}
	tuiModel := tui.NewModelWithOutputDevices(cfg, devices, nil).
		WithStartFunc(startFunc).
		WithLogChannel(logCh).
		WithLogFile(srvLogFile).
		WithSpectrumFunc(spectrum.Bands).
		WithLevelFunc(levelMeter.Levels).
		WithCaptureSpectrumFunc(captureSpectrum.Bands).
		WithCaptureLevelFunc(captureLevelMeter.Levels).
		WithAECToggle(func(enabled bool) {
			if aecRef != nil {
				aecRef.SetEnabled(enabled)
			}
		}).
		WithChatChannel(chatMsgCh).
		WithChatSendFunc(func(text, to string) {
			if chatServerApp != nil {
				chatServerApp.SendChatMessage(text, to)
			}
		})

	// Always wire multi-client/conference channels — the TUI dynamically enables
	// these modes in SetupDoneMsg based on the user's config selection.
	tuiModel = tuiModel.
		WithCommandChannel(cmdCh).
		WithBanListFunc(func() []string {
			if banMgrRef != nil {
				return banMgrRef.BannedList()
			}
			return nil
		})
	if cfg.Conference {
		tuiModel = tuiModel.
			WithMultiClient(multiStatsCh).
			WithConference(conferenceStatsCh, participantCmdCh, cfg.ServerMuted).
			WithRecordingCommands(recordingCmdCh).
			WithPauseChannel(serverPauseCh)
	} else {
		// Server has capture in normal and duplex modes — enable pause.
		hasCapture := !cfg.Reverse || cfg.Duplex
		if hasCapture {
			tuiModel = tuiModel.WithPauseChannel(serverPauseCh)
		}
		if cfg.MaxClients > 1 {
			tuiModel = tuiModel.WithMultiClient(multiStatsCh)
		} else {
			// Single-client: wire multiStatsCh for client info without enabling multi-client UI.
			tuiModel = tuiModel.WithSingleClientStats(multiStatsCh)
		}
	}
	// Pre-wire channels for dynamic mode switching in TUI setup.
	tuiModel.SetMultiClientChannels(multiStatsCh, conferenceStatsCh, participantCmdCh, recordingCmdCh)
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	_, err = p.Run()
	audio.CleanupOrphanedAggregateDevices()
	return err
}
