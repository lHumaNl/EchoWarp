package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/version"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	"github.com/lHumaNl/echowarp/pkg/echowarp/discovery"
)

// runDryRun validates config, checks audio device availability, prints resolved config, then exits.
func runDryRun(cfg *config.Config) error {
	fmt.Println("--- Dry Run ---")
	fmt.Printf("Mode:        %s\n", cfg.Mode)
	fmt.Printf("Port:        %d\n", cfg.Port)
	fmt.Printf("Sample Rate: %d Hz\n", cfg.SampleRate)
	fmt.Printf("Channels:    %d\n", cfg.Channels)
	fmt.Printf("Audio:       %s\n", cfg.AudioMode())
	fmt.Printf("Log Level:   %s\n", cfg.LogLevel)
	fmt.Printf("SIMD:        %v\n", !cfg.NoSIMDOptimization)
	if cfg.Mode == config.ModeClient {
		fmt.Printf("Server:      %s:%d\n", cfg.Address, cfg.Port)
	}
	if cfg.TLSCert != "" {
		fmt.Printf("TLS Cert:    %s\n", cfg.TLSCert)
	}

	// Check audio device
	dm, err := audio.NewDeviceManager()
	if err != nil {
		fmt.Printf("[FAIL] Audio system: %v\n", err)
		return nil
	}
	defer dm.Close() //nolint:errcheck

	isInput := (cfg.Mode == config.ModeServer && !cfg.Reverse) || (cfg.Mode == config.ModeClient && cfg.Reverse)
	var devices []audio.AudioDevice
	if isInput {
		devices, err = dm.ListInputDevices()
	} else {
		devices, err = dm.ListOutputDevices()
	}
	if err != nil {
		fmt.Printf("[FAIL] Audio devices: %v\n", err)
		return nil
	}

	if cfg.DeviceID != nil {
		found := false
		for _, d := range devices {
			if d.ID == *cfg.DeviceID {
				fmt.Printf("[OK]  Audio device: [%d] %s\n", d.ID, d.Name)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("[FAIL] Audio device ID %d not found\n", *cfg.DeviceID)
		}
	} else {
		fmt.Printf("[OK]  Audio: %d device(s) available\n", len(devices))
	}

	fmt.Println("--- Dry run complete, not starting ---")
	return nil
}

// suggestSaveConfig prints a tip about saving settings when the user hasn't
// already loaded or saved a config file during this run.
func suggestSaveConfig(_ *cobra.Command, _ *config.Config) {
	// Config saving is now done via Ctrl+S in TUI — no CLI tip needed.
}

func loadConfig(cmd *cobra.Command, mode config.Mode) (config.Config, error) {
	configPath, _ := cmd.Flags().GetString("config")

	// Use Viper-backed loading with full priority support:
	// Priority order (highest to lowest):
	// 1. CLI flags (applied via applyFlagOverrides below)
	// 2. Environment variables (ECHOWARP_* - handled by LoadWithViper)
	// 3. Config file (YAML - handled by LoadWithViper)
	// 4. Default values (handled by LoadWithViper)
	cfg, err := config.LoadWithViper(configPath, mode)
	if err != nil {
		return cfg, err
	}

	// Apply CLI flag overrides (highest priority)
	applyFlagOverrides(cmd, &cfg)
	return cfg, nil
}

func applyFlagOverrides(cmd *cobra.Command, cfg *config.Config) {
	overrideIntFlag(cmd, "port", &cfg.Port)
	overrideDeviceFlag(cmd, cfg)
	overrideStringFlag(cmd, "password", &cfg.Password)
	// Audio mode: --mode flag takes precedence, then legacy --reverse/--duplex/--conference
	if cmd.Flags().Changed("mode") {
		modeStr, _ := cmd.Flags().GetString("mode")
		cfg.StreamMode = config.AudioMode(modeStr)
		cfg.SyncFromStreamMode()
	}
	overrideBoolFlag(cmd, "reverse", &cfg.Reverse)
	overrideBoolFlag(cmd, "duplex", &cfg.Duplex)
	overrideSampleRateFlag(cmd, cfg)
	overrideChannelsFlag(cmd, cfg)
	overrideIntFlag(cmd, "max-clients", &cfg.MaxClients)
	overrideIntFlag(cmd, "max-auth-failures", &cfg.MaxFailedAttempts)
	// Legacy alias: --max-failed (hidden, deprecated)
	if cmd.Flags().Lookup("max-failed") != nil {
		overrideIntFlag(cmd, "max-failed", &cfg.MaxFailedAttempts)
	}
	overrideIntFlag(cmd, "max-reconnect", &cfg.MaxReconnectAttempts)
	overrideBoolFlag(cmd, "virtual-mic", &cfg.VirtualMic)
	overrideStringSliceFlag(cmd, "stun-server", &cfg.STUNServers)
	overrideStringFlag(cmd, "tls-cert", &cfg.TLSCert)
	overrideStringFlag(cmd, "tls-key", &cfg.TLSKey)
	overrideStringFlag(cmd, "log-level", &cfg.LogLevel)
	overrideLogFileFlag(cmd, cfg)
	overrideStringFlag(cmd, "ban-file", &cfg.BanFilePath)
	overrideBoolFlag(cmd, "loopback", &cfg.Loopback)
	overrideBoolFlag(cmd, "no-simd-optimization", &cfg.NoSIMDOptimization)
	overrideBoolFlag(cmd, "no-pool-warmup", &cfg.NoPoolWarmup)
	overrideStringSliceFlag(cmd, "trusted-proxies", &cfg.TrustedProxies)
	overrideIntFlag(cmd, "audio-buffer-frames", &cfg.AudioBufferFrames)
	overrideBoolFlag(cmd, "aec", &cfg.AEC)
	overrideBoolFlag(cmd, "conference", &cfg.Conference)
	// Hidden alias --conf
	if cmd.Flags().Changed("conf") {
		cfg.Conference, _ = cmd.Flags().GetBool("conf")
	}
	// Sync bools → StreamMode after all mode-related flags are applied
	if cmd.Flags().Changed("reverse") || cmd.Flags().Changed("duplex") ||
		cmd.Flags().Changed("conference") || cmd.Flags().Changed("conf") {
		cfg.SyncToStreamMode()
	}
	overrideBoolFlag(cmd, "server-muted", &cfg.ServerMuted)
	overrideBoolFlag(cmd, "hwid-required", &cfg.HWIDRequired)
	overrideStringFlag(cmd, "record", &cfg.RecordMode)
	cfg.NormalizeConference()
}

func overrideIntFlag(cmd *cobra.Command, name string, target *int) {
	if cmd.Flags().Changed(name) {
		*target, _ = cmd.Flags().GetInt(name)
	}
}

func overrideStringFlag(cmd *cobra.Command, name string, target *string) {
	if cmd.Flags().Changed(name) {
		*target, _ = cmd.Flags().GetString(name)
	}
}

func overrideBoolFlag(cmd *cobra.Command, name string, target *bool) {
	if cmd.Flags().Changed(name) {
		*target, _ = cmd.Flags().GetBool(name)
	}
}

func overrideStringSliceFlag(cmd *cobra.Command, name string, target *[]string) {
	if cmd.Flags().Changed(name) {
		*target, _ = cmd.Flags().GetStringSlice(name)
	}
}

func overrideDeviceFlag(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("device") {
		val, _ := cmd.Flags().GetUint("device")
		id := uint32(val)
		cfg.DeviceID = &id
	}
	overrideMultiDeviceFlags(cmd, cfg)
}

func overrideMultiDeviceFlags(cmd *cobra.Command, cfg *config.Config) {
	captureChanged := cmd.Flags().Changed("capture-device")
	playbackChanged := cmd.Flags().Changed("playback-device")

	if !captureChanged && !playbackChanged {
		return
	}

	if captureChanged {
		ids, _ := cmd.Flags().GetUintSlice("capture-device")
		for _, id := range ids {
			cfg.Devices = append(cfg.Devices, config.DeviceEntry{
				ID:     uint32(id),
				Role:   config.RoleCapture,
				Volume: 1.0,
			})
		}
	}
	if playbackChanged {
		ids, _ := cmd.Flags().GetUintSlice("playback-device")
		for _, id := range ids {
			cfg.Devices = append(cfg.Devices, config.DeviceEntry{
				ID:     uint32(id),
				Role:   config.RolePlayback,
				Volume: 1.0,
			})
		}
	}
}

func overrideSampleRateFlag(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("sample-rate") {
		val, _ := cmd.Flags().GetInt("sample-rate")
		cfg.SampleRate = uint32(val)
	}
}

func overrideChannelsFlag(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("channels") {
		val, _ := cmd.Flags().GetInt("channels")
		cfg.Channels = uint32(val)
	}
}

func overrideLogFileFlag(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("log-file") {
		cfg.LogFile, _ = cmd.Flags().GetString("log-file")
		cfg.LogToFile = cfg.LogFile != ""
	}
}

// resolveDeviceByName finds an audio device whose name contains the given substring
// (case-insensitive). Returns the device ID if exactly one match is found.
// isInput=true searches input devices; isInput=false searches output devices.
func resolveDeviceByName(name string, isInput bool) (*uint32, error) {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio: %w", err)
	}
	defer dm.Close() //nolint:errcheck

	var devices []audio.AudioDevice
	if isInput {
		devices, err = dm.ListInputDevices()
	} else {
		devices, err = dm.ListOutputDevices()
	}
	if err != nil {
		return nil, err
	}

	var matches []audio.AudioDevice
	nameLower := strings.ToLower(name)
	for _, d := range devices {
		if strings.Contains(strings.ToLower(d.Name), nameLower) {
			matches = append(matches, d)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no audio device matching %q", name)
	case 1:
		id := matches[0].ID
		return &id, nil
	default:
		var names []string
		for _, d := range matches {
			names = append(names, fmt.Sprintf("  [%d] %s", d.ID, d.Name))
		}
		return nil, fmt.Errorf("multiple devices match %q:\n%s\nUse --device to select by ID", name, strings.Join(names, "\n"))
	}
}

func setupBanManager(cfg *config.Config, logger *slog.Logger) (ban.BanManager, error) {
	banFilePath := cfg.BanFilePath
	if banFilePath == "" {
		configDir, _ := os.UserConfigDir()
		banFilePath = filepath.Join(configDir, "echowarp", "ban_list.yaml")
	}
	banMgr, err := ban.NewFileBanManager(cfg.MaxFailedAttempts, banFilePath)
	if err != nil {
		logger.Warn("Failed to initialize ban manager", "error", err)
		return nil, err
	}
	return banMgr, nil
}

func setupTLSConfig(cfg *config.Config) (*tls.Config, error) {
	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert: %w", err)
	}
	cfg.TLSSelfSigned = isCertSelfSigned(cfg.TLSCert)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// isCertSelfSigned checks whether the first certificate in a PEM file is self-signed
// (i.e. Issuer == Subject).
func isCertSelfSigned(certPath string) bool {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return cert.Issuer.String() == cert.Subject.String()
}

func setupRateLimiter(cmd *cobra.Command) *auth.IPRateLimiter {
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	if rateLimit > 0 {
		return auth.NewIPRateLimiter(rateLimit)
	}
	return nil
}

func setupDiscovery(ctx context.Context, cmd *cobra.Command, cfg *config.Config, logger *slog.Logger) {
	noDiscovery, _ := cmd.Flags().GetBool("no-discovery")
	if noDiscovery {
		return
	}

	serverName, _ := cmd.Flags().GetString("server-name")
	if serverName == "" {
		serverName, _ = os.Hostname()
	}

	txt := discovery.BuildTXTRecords("2.0.0", serverName, cfg.Password != "", cfg.TLSCert != "", 0, cfg.MaxClients, cfg.AudioMode(), config.ServerID(cfg.Port))
	pub, err := discovery.NewPublisher(serverName, cfg.Port, txt)
	if err != nil {
		logger.Warn("Failed to start mDNS publisher", "error", err)
		return
	}

	go func() {
		if err := pub.Publish(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("mDNS publishing error", "error", err)
		}
	}()
}

// startSessionInfoServer starts a lightweight HTTP server on port+1 that serves
// GET /api/v1/session/info for client probe in TUI mode (where the full API server is not running).
// The signaling port uses raw TCP, so we need a separate HTTP port for probes.
func startSessionInfoServer(ctx context.Context, cfg config.Config, logger *slog.Logger, clientCountFn ...func() int) {
	infoPort := cfg.Port + 1
	addr := fmt.Sprintf(":%d", infoPort)

	hostname, _ := os.Hostname()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/session/info", func(w http.ResponseWriter, r *http.Request) {
		mode := cfg.AudioMode()
		resp := map[string]any{
			"mode":              mode,
			"tls_required":      cfg.IsTLSEnabled(),
			"tls_self_signed":   cfg.TLSSelfSigned,
			"current_clients":   getClientCount(clientCountFn),
			"max_clients":       cfg.MaxClients,
			"sample_rate":       cfg.SampleRate,
			"channels":          cfg.Channels,
			"opus_bitrate":      cfg.OpusBitrate,
			"server_version":    version.Version,
			"server_name":       hostname,
			"password_required": cfg.Password != "",
			"hwid_required":     cfg.HWIDRequired,
			"server_id":         config.ServerID(cfg.Port),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := &http.Server{Handler: mux}

	listener, err := net.Listen("tcp", addr) //nolint:noctx // session info server uses its own lifecycle management
	if err != nil {
		logger.Warn("Session info server failed to start", "addr", addr, "error", err)
		return
	}

	go func() {
		<-ctx.Done()
		_ = server.Close() //nolint:errcheck
	}()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Warn("Session info server error", "error", err)
		}
	}()

	logger.Info("Session info server started", "addr", addr)
}

func getClientCount(fns []func() int) int {
	if len(fns) > 0 && fns[0] != nil {
		return fns[0]()
	}
	return 0
}
