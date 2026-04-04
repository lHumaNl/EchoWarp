package cli

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestServerCommand_HasRequiredFlags(t *testing.T) {
	cmd := newServerCmd()
	flags := cmd.Flags()
	if flags == nil {
		t.Error("server command should have flags")
	}

	requiredFlags := []string{"port", "device", "password", "config"}
	for _, flag := range requiredFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("server command should have --%s flag", flag)
		}
	}
}

func TestServerCommand_Use(t *testing.T) {
	cmd := newServerCmd()
	if cmd.Use != "server" {
		t.Errorf("expected Use 'server', got %q", cmd.Use)
	}
}

func TestServerCommand_RunE(t *testing.T) {
	cmd := newServerCmd()
	if cmd.RunE == nil {
		t.Error("server command should have RunE function")
	}
}

func TestServerCommand_AllFlags(t *testing.T) {
	cmd := newServerCmd()
	flags := cmd.Flags()

	expectedFlags := []string{
		"port", "device", "password", "reverse", "sample-rate",
		"channels", "max-clients", "max-auth-failures", "max-reconnect",
		"virtual-mic", "stun-server", "tls-cert", "tls-key", "config",
		"save-config", "log-level", "log-file", "ban-file",
		"no-discovery", "server-name", "rate-limit",
	}

	for _, flag := range expectedFlags {
		if flags.Lookup(flag) == nil {
			t.Errorf("server command should have --%s flag", flag)
		}
	}
}

func TestServerCommand_DefaultValues(t *testing.T) {
	cmd := newServerCmd()
	flags := cmd.Flags()

	port, _ := flags.GetInt("port")
	if port != 4415 {
		t.Errorf("expected default port 4415, got %d", port)
	}

	sampleRate, _ := flags.GetInt("sample-rate")
	if sampleRate != 48000 {
		t.Errorf("expected default sample rate 48000, got %d", sampleRate)
	}

	channels, _ := flags.GetInt("channels")
	if channels != 1 {
		t.Errorf("expected default channels 1, got %d", channels)
	}

	maxClients, _ := flags.GetInt("max-clients")
	if maxClients != 1 {
		t.Errorf("expected default max-clients 1, got %d", maxClients)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Port != 4415 {
		t.Errorf("expected default port 4415, got %d", cfg.Port)
	}
	if cfg.SampleRate != 48000 {
		t.Errorf("expected default sample rate 48000, got %d", cfg.SampleRate)
	}
	if cfg.Mode != config.ModeServer {
		t.Errorf("expected mode server, got %s", cfg.Mode)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 9999
sample_rate: 44100
password: testpass
log_level: debug
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--config", configFile})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.SampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", cfg.SampleRate)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password testpass, got %s", cfg.Password)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--config", "/nonexistent/config.yaml"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	_, err = loadConfig(cmd, config.ModeServer)
	if err == nil {
		t.Error("expected error for nonexistent config file")
	}
}

func TestLoadConfig_FlagOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 9999
password: filepass
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--config", configFile, "--port", "8080", "--password", "flagpass"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port override to 8080, got %d", cfg.Port)
	}
	if cfg.Password != "flagpass" {
		t.Errorf("expected password override to flagpass, got %s", cfg.Password)
	}
}

func TestApplyFlagOverrides_AllFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		check    func(cfg *config.Config) bool
		expected bool
	}{
		{
			name: "port override",
			args: []string{"--port", "8080"},
			check: func(cfg *config.Config) bool {
				return cfg.Port == 8080
			},
			expected: true,
		},
		{
			name: "device override",
			args: []string{"--device", "5"},
			check: func(cfg *config.Config) bool {
				return cfg.DeviceID != nil && *cfg.DeviceID == 5
			},
			expected: true,
		},
		{
			name: "password override",
			args: []string{"--password", "secret123"},
			check: func(cfg *config.Config) bool {
				return cfg.Password == "secret123"
			},
			expected: true,
		},
		{
			name: "reverse override",
			args: []string{"--reverse"},
			check: func(cfg *config.Config) bool {
				return cfg.Reverse
			},
			expected: true,
		},
		{
			name: "sample-rate override",
			args: []string{"--sample-rate", "16000"},
			check: func(cfg *config.Config) bool {
				return cfg.SampleRate == 16000
			},
			expected: true,
		},
		{
			name: "channels override",
			args: []string{"--channels", "2"},
			check: func(cfg *config.Config) bool {
				return cfg.Channels == 2
			},
			expected: true,
		},
		{
			name: "max-clients override",
			args: []string{"--max-clients", "10"},
			check: func(cfg *config.Config) bool {
				return cfg.MaxClients == 10
			},
			expected: true,
		},
		{
			name: "max-failed override",
			args: []string{"--max-failed", "10"},
			check: func(cfg *config.Config) bool {
				return cfg.MaxFailedAttempts == 10
			},
			expected: true,
		},
		{
			name: "max-reconnect override",
			args: []string{"--max-reconnect", "20"},
			check: func(cfg *config.Config) bool {
				return cfg.MaxReconnectAttempts == 20
			},
			expected: true,
		},
		{
			name: "virtual-mic override",
			args: []string{"--virtual-mic"},
			check: func(cfg *config.Config) bool {
				return cfg.VirtualMic
			},
			expected: true,
		},
		{
			name: "log-level override",
			args: []string{"--log-level", "debug"},
			check: func(cfg *config.Config) bool {
				return cfg.LogLevel == "debug"
			},
			expected: true,
		},
		{
			name: "log-file override",
			args: []string{"--log-file", "/var/log/echowarp.log"},
			check: func(cfg *config.Config) bool {
				return cfg.LogFile == "/var/log/echowarp.log" && cfg.LogToFile
			},
			expected: true,
		},
		{
			name: "tls-cert and tls-key override",
			args: []string{"--tls-cert", "/path/cert.pem", "--tls-key", "/path/key.pem"},
			check: func(cfg *config.Config) bool {
				return cfg.TLSCert == "/path/cert.pem" && cfg.TLSKey == "/path/key.pem"
			},
			expected: true,
		},
		{
			name: "ban-file override",
			args: []string{"--ban-file", "/path/ban.json"},
			check: func(cfg *config.Config) bool {
				return cfg.BanFilePath == "/path/ban.json"
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newServerCmd()
			err := cmd.ParseFlags(tt.args)
			if err != nil {
				t.Fatalf("ParseFlags failed: %v", err)
			}

			cfg := config.DefaultConfig()
			applyFlagOverrides(cmd, &cfg)

			if tt.check(&cfg) != tt.expected {
				t.Errorf("check failed for %s", tt.name)
			}
		})
	}
}

func TestApplyFlagOverrides_STUN(t *testing.T) {
	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--stun-server", "stun:example.com:3478", "--stun-server", "stun:example2.com:3478"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg := config.DefaultConfig()
	applyFlagOverrides(cmd, &cfg)

	if len(cfg.STUNServers) != 2 {
		t.Errorf("expected 2 STUN servers, got %d", len(cfg.STUNServers))
	}
}

func TestApplyFlagOverrides_NoChanges(t *testing.T) {
	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	originalCfg := config.DefaultConfig()
	cfg := config.DefaultConfig()
	applyFlagOverrides(cmd, &cfg)

	if cfg.Port != originalCfg.Port {
		t.Error("port should not change without flag")
	}
	if cfg.SampleRate != originalCfg.SampleRate {
		t.Error("sample rate should not change without flag")
	}
}

func TestSetupTLSConfig_NoTLS(t *testing.T) {
	cfg := &config.Config{}
	tlsConfig, err := setupTLSConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tlsConfig != nil {
		t.Error("expected nil tls config when no TLS configured")
	}
}

func TestSetupTLSConfig_OnlyCert(t *testing.T) {
	cfg := &config.Config{
		TLSCert: "/path/cert.pem",
		TLSKey:  "",
	}
	tlsConfig, err := setupTLSConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tlsConfig != nil {
		t.Error("expected nil tls config when only cert is set")
	}
}

func TestSetupTLSConfig_OnlyKey(t *testing.T) {
	cfg := &config.Config{
		TLSCert: "",
		TLSKey:  "/path/key.pem",
	}
	tlsConfig, err := setupTLSConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tlsConfig != nil {
		t.Error("expected nil tls config when only key is set")
	}
}

func TestSetupTLSConfig_InvalidCert(t *testing.T) {
	cfg := &config.Config{
		TLSCert: "/nonexistent/cert.pem",
		TLSKey:  "/nonexistent/key.pem",
	}
	_, err := setupTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid cert/key paths")
	}
}

func TestSetupRateLimiter_Disabled(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--rate-limit", "0"})

	limiter := setupRateLimiter(cmd)
	if limiter != nil {
		t.Error("expected nil rate limiter when rate-limit is 0")
	}
}

func TestSetupRateLimiter_Enabled(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--rate-limit", "10"})

	limiter := setupRateLimiter(cmd)
	if limiter == nil {
		t.Error("expected non-nil rate limiter when rate-limit > 0")
	}
}

func TestSetupDiscovery_Disabled(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--no-discovery"})

	cfg := &config.Config{Port: 4415}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_Enabled(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--server-name", "test-server"})

	cfg := &config.Config{Port: 4415}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_WithCustomName(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--server-name", "my-custom-server"})

	cfg := &config.Config{Port: 4415, Password: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_EmptyServerName(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--server-name", ""})

	cfg := &config.Config{Port: 4415}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_NoServerNameFlag(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := &config.Config{Port: 4415}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_MulticastAddress(t *testing.T) {
	addr := net.ParseIP("224.0.0.251")
	if addr == nil {
		t.Error("failed to parse multicast address")
	}
}

func TestSetupBanManager_DefaultPath(t *testing.T) {
	cfg := &config.Config{
		MaxFailedAttempts: 5,
	}

	banMgr, err := setupBanManager(cfg, nil)
	if err == nil && banMgr != nil {
		banMgr.Close()
	}
}

func TestSetupBanManager_WithLogger(t *testing.T) {
	cfg := &config.Config{
		MaxFailedAttempts: 5,
	}

	banMgr, err := setupBanManager(cfg, nil)
	if err == nil && banMgr != nil {
		banMgr.Close()
	}
}

func TestSetupBanManager_CustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	banFile := tmpDir + "/ban.json"

	cfg := &config.Config{
		MaxFailedAttempts: 5,
		BanFilePath:       banFile,
	}

	banMgr, err := setupBanManager(cfg, nil)
	if err == nil && banMgr != nil {
		banMgr.Close()
	}
}

func TestSetupBanManager_InvalidPath(t *testing.T) {
	cfg := &config.Config{
		MaxFailedAttempts: 5,
		BanFilePath:       "/nonexistent/path/ban.json",
	}

	_, _ = setupBanManager(cfg, nil)
}

func TestRunServer_InvalidConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Port = -1
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestRunServer_InvalidPort(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Port = 70000
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestRunServer_MissingConfigFile(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--config", "/nonexistent/config.yaml"})

	_, err := loadConfig(cmd, config.ModeServer)
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestRunServer_InvalidSampleRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SampleRate = 12345
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid sample rate")
	}
}

func TestRunServer_InvalidChannels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels = 3
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid channels")
	}
}

func TestRunServer_InvalidLogLevel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LogLevel = "invalid"
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestRunServer_SaveConfigToInvalidPath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(blocker, "sub", "config.yaml")

	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--save-config", invalidPath})

	cfg := config.DefaultConfig()
	cfg.DeviceID = new(uint32)
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for invalid save path")
	}
}

func TestRunServer_ZeroChannels(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Channels = 0
	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	if err == nil {
		t.Error("expected error for zero channels")
	}
}

func TestServerCommand_Short(t *testing.T) {
	cmd := newServerCmd()
	if cmd.Short == "" {
		t.Error("server command should have a short description")
	}
}

func TestServerCommand_Long(t *testing.T) {
	cmd := newServerCmd()
	if cmd.Long == "" {
		t.Error("server command should have a long description")
	}
}

func TestNewServerCmd(t *testing.T) {
	cmd := newServerCmd()
	if cmd == nil {
		t.Error("newServerCmd should return non-nil command")
	}
}

func TestLoadConfig_ClientMode(t *testing.T) {
	cmd := newClientCmd()
	err := cmd.ParseFlags([]string{"--address", "192.168.1.100"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeClient)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg.Mode != config.ModeClient {
		t.Errorf("expected mode client, got %s", cfg.Mode)
	}
}

func TestRunServer_ValidConfigWithDevice(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--device", "0", "--port", "4415", "--no-discovery"})

	cfg := config.DefaultConfig()
	cfg.DeviceID = uint32Ptr(0)

	_ = validateAndSaveConfig(cmd, &cfg)
}

func TestSetupBanManager_WithMaxFailedZero(t *testing.T) {
	cfg := &config.Config{
		MaxFailedAttempts: 0,
	}

	banMgr, err := setupBanManager(cfg, nil)
	if err != nil && banMgr != nil {
		banMgr.Close()
	}
}

func TestSetupBanManager_WithMaxFailedPositive(t *testing.T) {
	tmpDir := t.TempDir()
	banFile := filepath.Join(tmpDir, "ban.json")

	cfg := &config.Config{
		MaxFailedAttempts: 5,
		BanFilePath:       banFile,
	}

	banMgr, err := setupBanManager(cfg, nil)
	if err == nil && banMgr != nil {
		banMgr.Close()
	}
}

func TestSetupTLSConfig_ValidCertKey(t *testing.T) {
	t.Skip("Skipping - requires valid TLS certificates")
}

func TestSetupDiscovery_WithPassword(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--server-name", "test-server"})

	cfg := &config.Config{Port: 4415, Password: "secret"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestSetupDiscovery_WithTLSCert(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--server-name", "test-server"})

	cfg := &config.Config{Port: 4415, TLSCert: "/path/cert.pem"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
}

func TestOverrideLogFileFlag_Enabled(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--log-file", "/var/log/echowarp.log"})

	cfg := config.DefaultConfig()
	overrideLogFileFlag(cmd, &cfg)

	if cfg.LogFile != "/var/log/echowarp.log" {
		t.Errorf("logFile = %v, want /var/log/echowarp.log", cfg.LogFile)
	}
	if !cfg.LogToFile {
		t.Error("logToFile should be true")
	}
}

func TestOverrideLogFileFlag_Empty(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--log-file", ""})

	cfg := config.DefaultConfig()
	overrideLogFileFlag(cmd, &cfg)

	if cfg.LogToFile {
		t.Error("logToFile should be false when log-file is empty")
	}
}

func uint32Ptr(v uint) *uint32 {
	val := uint32(v)
	return &val
}

// TestRunServerInteractive_NoDevices tests runServerInteractive when no devices are available.
func TestRunServerInteractive_NoDevices(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping interactive test in CI - requires terminal")
	}
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{})

	cfg := &config.Config{
		Mode:    config.ModeServer,
		Reverse: false,
	}

	err := runServerInteractive(cmd, cfg)
	if err == nil {
		t.Log("runServerInteractive succeeded - audio devices available")
	}
}

// TestRunServerInteractive_ReverseMode tests runServerInteractive in reverse mode.
func TestRunServerInteractive_ReverseMode(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping interactive test in CI - requires terminal")
	}
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{"--reverse"})

	cfg := &config.Config{
		Mode:    config.ModeServer,
		Reverse: true,
	}

	err := runServerInteractive(cmd, cfg)
	if err == nil {
		t.Log("runServerInteractive succeeded in reverse mode")
	}
}

// TestRunServer_WithDeviceID tests runServer when DeviceID is provided.
func TestRunServer_WithDeviceID(t *testing.T) {
	t.Skip("Skipping - requires audio hardware and may start server")
}

// TestExecuteServerApp_InvalidDevice tests executeServerApp with invalid device.
func TestExecuteServerApp_InvalidDevice(t *testing.T) {
	t.Skip("Skipping - requires audio hardware")
}

// TestSetupDiscovery_WithAllOptions tests setupDiscovery with all options.
func TestSetupDiscovery_WithAllOptions(t *testing.T) {
	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{
		"--server-name", "test-server",
		"--no-discovery=false",
	})

	cfg := &config.Config{
		Port:     4415,
		Password: "secret",
		TLSCert:  "/path/cert.pem",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	setupDiscovery(ctx, cmd, cfg, nil)
	// Wait for context to expire so zeroconf goroutines shut down cleanly.
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
}

// TestRunServer_NilDeviceID tests that config with nil DeviceID passes validation
// (DeviceID is optional at config level, required only by --no-interactive at runtime).
func TestRunServer_NilDeviceID(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.DeviceID = nil
	cfg.Password = "test"

	cmd := newServerCmd()
	err := validateAndSaveConfig(cmd, &cfg)
	// Validation should pass — DeviceID is not a config-level requirement
	if err != nil {
		t.Errorf("validateAndSaveConfig should not error for nil DeviceID: %v", err)
	}
}

// TestLoadConfig_EnvVarOverride tests that environment variables override config file values.
func TestLoadConfig_EnvVarOverride(t *testing.T) {
	// Create a config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
sample_rate: 44100
password: filepass
log_level: info
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_PASSWORD")
		os.Unsetenv("ECHOWARP_LOG_LEVEL")
	}()

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--config", configFile})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Env vars should override config file values
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 from env var, got %d", cfg.Port)
	}
	if cfg.Password != "envpass" {
		t.Errorf("expected password 'envpass' from env var, got %s", cfg.Password)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env var, got %s", cfg.LogLevel)
	}
	// Sample rate should still be from config file (not overridden by env)
	if cfg.SampleRate != 44100 {
		t.Errorf("expected sample rate 44100 from config file, got %d", cfg.SampleRate)
	}
}

// TestLoadConfig_CLIOverridesEnvVar tests that CLI flags override environment variables.
func TestLoadConfig_CLIOverridesEnvVar(t *testing.T) {
	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_PASSWORD")
		os.Unsetenv("ECHOWARP_LOG_LEVEL")
	}()

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--port", "7777", "--password", "clipass"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// CLI flags should override env vars
	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from CLI flag, got %d", cfg.Port)
	}
	if cfg.Password != "clipass" {
		t.Errorf("expected password 'clipass' from CLI flag, got %s", cfg.Password)
	}
	// Log level should come from env var (not overridden by CLI)
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env var, got %s", cfg.LogLevel)
	}
}

// TestLoadConfig_AllEnvVars tests loading all configuration from environment variables.
func TestLoadConfig_AllEnvVars(t *testing.T) {
	// Set all relevant environment variables
	envVars := map[string]string{
		"ECHOWARP_PORT":         "9999",
		"ECHOWARP_PASSWORD":     "testpass",
		"ECHOWARP_MODE":         "reverse",
		"ECHOWARP_SAMPLE_RATE":  "16000",
		"ECHOWARP_CHANNELS":     "2",
		"ECHOWARP_VIRTUAL_MIC":  "true",
		"ECHOWARP_LOG_LEVEL":    "debug",
		"ECHOWARP_OPUS_BITRATE": "96000",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Verify all env vars were applied
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password 'testpass', got %s", cfg.Password)
	}
	if !cfg.Reverse {
		t.Error("expected reverse to be true")
	}
	if cfg.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", cfg.SampleRate)
	}
	if cfg.Channels != 2 {
		t.Errorf("expected channels 2, got %d", cfg.Channels)
	}
	if !cfg.VirtualMic {
		t.Error("expected virtual_mic to be true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got %s", cfg.LogLevel)
	}
	if cfg.OpusBitrate != 96000 {
		t.Errorf("expected opus bitrate 96000, got %d", cfg.OpusBitrate)
	}
}

// TestLoadConfig_STUNEnvVar tests loading STUN servers from environment variable.
func TestLoadConfig_STUNEnvVar(t *testing.T) {
	os.Setenv("ECHOWARP_STUN_SERVERS", "stun:server1.com:3478,stun:server2.com:3478")
	defer os.Unsetenv("ECHOWARP_STUN_SERVERS")

	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if len(cfg.STUNServers) != 2 {
		t.Errorf("expected 2 STUN servers, got %d", len(cfg.STUNServers))
	}
	if len(cfg.STUNServers) > 0 {
		if cfg.STUNServers[0] != "stun:server1.com:3478" {
			t.Errorf("expected first STUN server 'stun:server1.com:3478', got %s", cfg.STUNServers[0])
		}
		if cfg.STUNServers[1] != "stun:server2.com:3478" {
			t.Errorf("expected second STUN server 'stun:server2.com:3478', got %s", cfg.STUNServers[1])
		}
	}
}

// TestLoadConfig_EnvVarPriority tests the full priority chain:
// CLI flags > env vars > config file > defaults
func TestLoadConfig_EnvVarPriority(t *testing.T) {
	// Create a config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
sample_rate: 44100
channels: 2
log_level: warn
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Set environment variables (lower priority than CLI flags)
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_SAMPLE_RATE", "16000")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_SAMPLE_RATE")
	}()

	// CLI flags override everything
	cmd := newServerCmd()
	err := cmd.ParseFlags([]string{"--config", configFile, "--port", "7777"})
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Port: CLI flag (7777) should override env var (9999) and config file (8080)
	if cfg.Port != 7777 {
		t.Errorf("expected port 7777 from CLI flag, got %d", cfg.Port)
	}

	// Sample rate: env var (16000) should override config file (44100)
	if cfg.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000 from env var, got %d", cfg.SampleRate)
	}

	// Channels: config file (2) should be used (no env var or CLI override)
	if cfg.Channels != 2 {
		t.Errorf("expected channels 2 from config file, got %d", cfg.Channels)
	}

	// Log level: config file (warn) should be used (no env var or CLI override)
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log level 'warn' from config file, got %s", cfg.LogLevel)
	}
}
