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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/logging"
	"github.com/lHumaNl/echowarp/internal/tui"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// newClientCmd creates the "client" subcommand for starting EchoWarp in client mode.
// In client mode, EchoWarp connects to a server and receives/sends audio.
//
// If --address is not specified and --discover is true (default), searches for
// servers on the local network via mDNS. If multiple servers are found, prompts
// for selection or requires --address to disambiguate.
//
// If no device is specified with --device, an interactive TUI prompts for device selection.
//
// Examples:
//
//	echowarp client --address 192.168.1.100 --password secret
//	echowarp client --discover --discover-timeout 5s
//	echowarp client --address 192.168.1.100  # Direct connect (mode from server probe)
func newClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Start EchoWarp in client mode",
		Long:  "Start EchoWarp as a client, connecting to a server for audio streaming.",
		RunE:  runClient,
	}

	cmd.Flags().StringP("address", "a", "", "Server address (IP or hostname)")
	cmd.Flags().IntP("port", "p", 4415, "Server TCP port")
	cmd.Flags().UintP("device", "d", 0, "Audio device ID (repeatable in multi-device mode)")
	cmd.Flags().UintSlice("capture-device", nil, "Capture device ID (duplex mode, repeatable)")
	cmd.Flags().UintSlice("playback-device", nil, "Playback device ID (duplex mode, repeatable)")
	cmd.Flags().StringP("password", "P", "", "Password for authentication")
	cmd.Flags().Int("max-reconnect", 5, "Max reconnect attempts (0=infinite)")
	cmd.Flags().Bool("auto-reconnect", false, "Auto-reconnect after server shutdown")
	cmd.Flags().Int("auto-reconnect-attempts", 0, "Max auto-reconnect attempts (0=infinite)")
	cmd.Flags().Bool("virtual-mic", false, "Create virtual microphone")
	cmd.Flags().StringSlice("stun-server", nil, "STUN servers")
	cmd.Flags().Bool("tls-insecure", false, "Accept self-signed TLS certificates")
	cmd.Flags().StringP("config", "c", "", "Path to YAML config file")
	cmd.Flags().StringP("save-config", "s", "", "Save current settings to file")
	cmd.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.Flags().String("log-file", "", "Log to file")
	cmd.Flags().Bool("discover", true, "Search for LAN servers")
	cmd.Flags().Duration("discover-timeout", 3*time.Second, "Discovery timeout")
	cmd.Flags().Bool("no-simd-optimization", false, "Disable SIMD audio optimizations (use pure Go)")
	cmd.Flags().Bool("no-pool-warmup", false, "Disable audio buffer pool warmup on startup")
	cmd.Flags().StringP("device-name", "D", "", "Select audio device by name substring (case-insensitive)")
	cmd.Flags().Bool("dry-run", false, "Validate config and check device, then exit without starting")
	cmd.Flags().Bool("loopback", false, "Enable loopback capture of system audio (macOS, requires BlackHole)")
	cmd.Flags().Bool("no-interactive", false, "Disable TUI: requires --device, logs to stdout/file")
	cmd.Flags().Bool("aec", false, "Enable acoustic echo cancellation (duplex mode)")
	cmd.Flags().StringP("nickname", "n", "", "Chat display name (max 20 chars)")
	_ = cmd.Flags().MarkHidden("no-simd-optimization")
	_ = cmd.Flags().MarkHidden("no-pool-warmup")

	applyGroupedUsage(cmd, []flagGroup{
		{"Audio", []string{"device", "device-name", "capture-device", "playback-device", "virtual-mic", "loopback", "aec"}},
		{"Network", []string{"address", "port", "stun-server", "tls-insecure", "discover", "discover-timeout"}},
		{"Security", []string{"password"}},
		{"Chat", []string{"nickname"}},
		{"Config", []string{"config", "save-config"}},
		{"Reconnect", []string{"max-reconnect", "auto-reconnect", "auto-reconnect-attempts"}},
		{"Logging & debug", []string{"log-level", "log-file", "dry-run", "no-interactive"}},
	})

	return cmd
}

func runClient(cmd *cobra.Command, args []string) error {
	cfg, err := loadClientConfig(cmd)
	if err != nil {
		return err
	}

	if deviceName, _ := cmd.Flags().GetString("device-name"); deviceName != "" {
		// Client in normal mode receives audio → output device; in reverse → input device
		var id *uint32
		id, err = resolveDeviceByName(deviceName, cfg.Reverse)
		if err != nil {
			return err
		}
		cfg.DeviceID = id
	}

	if err = handleClientDiscovery(cmd, cfg); err != nil { //nolint:gocritic // avoiding shadow
		return err
	}

	noInteractive, _ := cmd.Flags().GetBool("no-interactive")
	// In TUI mode, address can be empty — user fills it interactively.
	// Only validate config fully for non-interactive mode.
	if noInteractive {
		if err = validateAndSaveConfig(cmd, cfg); err != nil { //nolint:gocritic // avoiding shadow
			return err
		}
	}

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return runDryRun(cfg)
	}

	err = executeClientMode(cmd, cfg)

	if err == nil {
		suggestSaveConfig(cmd, cfg)
	}

	return err
}

func validateAndSaveConfig(cmd *cobra.Command, cfg *config.Config) error {
	errs := cfg.Validate()
	if len(errs) > 0 {
		for _, e := range errs {
			_, _ = fmt.Fprintf(os.Stderr, "Config error: %v\n", e)
		}
		return fmt.Errorf("invalid configuration")
	}

	savePath, _ := cmd.Flags().GetString("save-config")
	if savePath != "" {
		if err := cfg.SaveToFile(savePath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("Configuration saved to %s\n", savePath)
	}

	return nil
}

func executeClientMode(cmd *cobra.Command, cfg *config.Config) error {
	noInteractive, _ := cmd.Flags().GetBool("no-interactive")
	if noInteractive {
		if cfg.Address == "" {
			return fmt.Errorf("--no-interactive requires --address to be specified (or use interactive mode with --discover for LAN server discovery)")
		}
		if cfg.DeviceID == nil {
			return fmt.Errorf("--no-interactive requires --device to be specified")
		}
		return runClientDirect(cfg)
	}
	return runClientStreamingTUI(cmd, *cfg)
}

func runClientInteractive(_ *cobra.Command, cfg *config.Config) error {
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

	tuiModel := tui.NewModel(*cfg, devices).
		WithDeviceEnumerator(dm)
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

// runClientStreamingTUI launches the client with a full TUI using an already-parsed config.
// Used when the user runs `echowarp client` without --no-interactive.
func runClientStreamingTUI(_ *cobra.Command, cfg config.Config) error {
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
	defer dm.Close() //nolint:errcheck
	devices, err := listAllDevices(dm)
	if err != nil {
		return err
	}
	// Always load loopback devices — mode may change after probe (reverse/duplex).
	devices, loopbackMap := appendLoopbackDevices(dm, devices, true)

	statsCh := make(chan transport.ConnectionStats, 4)
	errCh := make(chan error, 4)
	logCh := make(chan tui.LogEntry, 200)
	recordingCmdCh := make(chan tui.RecordingCommand, 4)
	appRecordingCmdCh := make(chan app.RecordingCommand, 4)
	serverMuteCh := make(chan bool, 4)
	pauseCh := make(chan bool, 4)
	deviceCmdCh := make(chan tui.DeviceCommand, 16)
	// Playback (decode/incoming) spectrum and level meter.
	spectrum := audio.NewSpectrumAnalyzer(cfg.SampleRate, audio.DefaultBands)
	levelMeter := audio.NewLevelMeter(cfg.Channels)
	// Capture (outgoing) spectrum and level meter — separate instances for duplex/conference.
	captureSpectrum := audio.NewSpectrumAnalyzer(cfg.SampleRate, audio.DefaultBands)
	captureLevelMeter := audio.NewLevelMeter(cfg.Channels)
	chatMsgCh := make(chan app.ChatMessage, 32)
	chatNicknameCh := make(chan string, 1)
	participantsCh := make(chan app.ChatParticipantsPayload, 4)
	participantPauseCh := make(chan app.ParticipantPauseMsg, 16)
	conferencePartsCh := make(chan app.ConferenceParticipantsMsg, 4)
	var chatClientApp *app.ClientApp

	serverStoppedCh := make(chan struct{})

	startFunc := tui.StartFunc(func(selectedCfg config.Config, stopCh <-chan struct{}) (<-chan transport.ConnectionStats, <-chan error, <-chan struct{}) {
		// Create a fresh serverStoppedCh for each connection cycle.
		serverStoppedCh = make(chan struct{})

		// Check if the selected device is a loopback device.
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
		// Cancel context when TUI requests graceful stop.
		go func() {
			select {
			case <-stopCh:
				cancel()
			case <-ctx.Done():
			}
		}()
		go func() {
			defer cancel()
			tuiHandler := tui.NewChannelLogHandler(logCh, tui.ParseSlogLevel(selectedCfg.LogLevel))
			logFile := selectedCfg.LogFile
			if logFile == "" {
				logFile = logging.GetDefaultLogFile("client")
			}
			logger, logCloser, logErr := logging.NewTUILogger(selectedCfg.LogLevel, logFile, tuiHandler)
			if logErr != nil {
				logger = slog.New(tuiHandler)
			}
			if logCloser != nil {
				defer logCloser.Close() //nolint:errcheck
			}
			// Redirect global slog so package-level slog.Warn calls appear in TUI log panel.
			slog.SetDefault(logger)

			// Auto-discover server if address not set.
			if selectedCfg.Address == "" {
				disc := discovery.NewDiscoverer()
				discCtx, discCancel := context.WithTimeout(ctx, 3*time.Second)
				services, _ := disc.Discover(discCtx, 3*time.Second)
				discCancel()
				var servers []discovery.ServiceInfo
				for svc := range services {
					servers = append(servers, svc)
				}
				if len(servers) == 1 {
					if len(servers[0].AddrIPv4) > 0 {
						selectedCfg.Address = servers[0].AddrIPv4[0].String()
					}
					selectedCfg.Port = servers[0].Port
				}
			}

			var tlsConfig *tls.Config
			if selectedCfg.TLS {
				tlsConfig = &tls.Config{InsecureSkipVerify: selectedCfg.TLSInsecure, MinVersion: tls.VersionTLS12} //nolint:gosec
			}

			clientApp := app.NewClientApp(selectedCfg, logger, tlsConfig).
				WithStatsChannels(statsCh, errCh).
				WithStopChannel(stopCh).
				WithSpectrum(spectrum).
				WithLevelMeter(levelMeter).
				WithCaptureSpectrum(captureSpectrum).
				WithCaptureLevelMeter(captureLevelMeter).
				WithRecordingCommandChannel(appRecordingCmdCh).
				WithChatChannel(chatMsgCh).
				WithChatNicknameChannel(chatNicknameCh).
				WithParticipantsChannel(participantsCh).
				WithServerStoppedChannel(serverStoppedCh).
				WithParticipantPauseChannel(participantPauseCh).
				WithConferenceParticipantsChannel(conferencePartsCh)
			// Wire pause only for modes with capture (reverse/duplex/conference).
			hasCapture := selectedCfg.Reverse || selectedCfg.Duplex || selectedCfg.Conference
			if hasCapture {
				clientApp.WithPauseChannel(pauseCh)
			}
			// Wire server mute only for modes that receive audio (normal/duplex/conference).
			hasPlayback := !selectedCfg.Reverse || selectedCfg.Duplex || selectedCfg.Conference
			if hasPlayback {
				clientApp.WithServerMuteChannel(serverMuteCh)
			}
			chatClientApp = clientApp

			// Bridge TUI recording commands → app recording commands.
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case cmd, ok := <-recordingCmdCh:
						if !ok {
							return
						}
						appCmd := app.RecordingCommand{
							Start:          cmd.Start,
							Mode:           audio.RecordingMode(cmd.Mode),
							LocalDeviceIDs: cmd.LocalDeviceIDs,
							RemoteIDs:      cmd.RemoteIDs,
						}
						select {
						case appRecordingCmdCh <- appCmd:
						case <-ctx.Done():
							return
						}
					}
				}
			}()

			// TODO: wire deviceCmdCh to HandleDeviceCommands with the mixer and AGC processors
			// once ClientApp exposes them. For now, drain the channel so the TUI doesn't block.
			go drainDeviceCommands(ctx, deviceCmdCh, logger)

			runErr := app.RunWithReconnect(
				ctx, logger,
				selectedCfg.MaxReconnectAttempts,
				time.Duration(selectedCfg.ReconnectIntervalSec)*time.Second,
				clientApp.Run,
			)
			if runErr != nil && ctx.Err() == nil {
				sendError(errCh, runErr, logger)
			}
		}()
		return statsCh, errCh, serverStoppedCh
	})

	cliLogFile := cfg.LogFile
	if cliLogFile == "" {
		cliLogFile = logging.GetDefaultLogFile("client")
	}
	tuiModel := tui.NewModelWithOutputDevices(cfg, devices, nil).
		WithDeviceEnumerator(dm).
		WithStartFunc(startFunc).
		WithLogChannel(logCh).
		WithLogFile(cliLogFile).
		WithRecordingCommands(recordingCmdCh).
		WithSpectrumFunc(spectrum.Bands).
		WithLevelFunc(levelMeter.Levels).
		WithCaptureSpectrumFunc(captureSpectrum.Bands).
		WithCaptureLevelFunc(captureLevelMeter.Levels).
		WithChatChannel(chatMsgCh).
		WithChatSendFunc(func(text, to string) {
			if chatClientApp != nil {
				_ = chatClientApp.SendChatMessage(text, to)
			}
		}).
		WithChatNicknameChannel(chatNicknameCh).
		WithParticipantsChannel(participantsCh).
		WithServerStoppedChannel(serverStoppedCh).
		WithParticipantPauseChannel(participantPauseCh).
		WithConferenceParticipantsChannel(conferencePartsCh).
		WithDeviceCommandChannel(deviceCmdCh)
	// Wire both channels unconditionally — mode may change after probe in TUI setup.
	tuiModel = tuiModel.WithPauseChannel(pauseCh).WithServerMuteChannel(serverMuteCh)
	p := tea.NewProgram(tuiModel, tea.WithAltScreen())
	_, err = p.Run()
	audio.CleanupOrphanedAggregateDevices()
	return err
}
