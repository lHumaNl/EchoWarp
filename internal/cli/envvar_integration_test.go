package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lHumaNl/echowarp/internal/config"
)

// TestEnvVarIntegration_Server tests all supported environment variables for server mode.
// This test demonstrates the complete environment variable support across all CLI commands.
func TestEnvVarIntegration_Server(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		check    func(cfg *config.Config) bool
	}{
		{
			name:     "ECHOWARP_PORT",
			envVar:   "ECHOWARP_PORT",
			envValue: "9999",
			check: func(cfg *config.Config) bool {
				return cfg.Port == 9999
			},
		},
		{
			name:     "ECHOWARP_PASSWORD",
			envVar:   "ECHOWARP_PASSWORD",
			envValue: "testpass123",
			check: func(cfg *config.Config) bool {
				return cfg.Password == "testpass123"
			},
		},
		{
			name:     "ECHOWARP_MODE",
			envVar:   "ECHOWARP_MODE",
			envValue: "reverse",
			check: func(cfg *config.Config) bool {
				return cfg.Reverse == true
			},
		},
		{
			name:     "ECHOWARP_SAMPLE_RATE",
			envVar:   "ECHOWARP_SAMPLE_RATE",
			envValue: "16000",
			check: func(cfg *config.Config) bool {
				return cfg.SampleRate == 16000
			},
		},
		{
			name:     "ECHOWARP_CHANNELS",
			envVar:   "ECHOWARP_CHANNELS",
			envValue: "2",
			check: func(cfg *config.Config) bool {
				return cfg.Channels == 2
			},
		},
		{
			name:     "ECHOWARP_VIRTUAL_MIC",
			envVar:   "ECHOWARP_VIRTUAL_MIC",
			envValue: "true",
			check: func(cfg *config.Config) bool {
				return cfg.VirtualMic == true
			},
		},
		{
			name:     "ECHOWARP_LOG_LEVEL",
			envVar:   "ECHOWARP_LOG_LEVEL",
			envValue: "debug",
			check: func(cfg *config.Config) bool {
				return cfg.LogLevel == "debug"
			},
		},
		{
			name:     "ECHOWARP_OPUS_BITRATE",
			envVar:   "ECHOWARP_OPUS_BITRATE",
			envValue: "96000",
			check: func(cfg *config.Config) bool {
				return cfg.OpusBitrate == 96000
			},
		},
		{
			name:     "ECHOWARP_OPUS_COMPLEXITY",
			envVar:   "ECHOWARP_OPUS_COMPLEXITY",
			envValue: "10",
			check: func(cfg *config.Config) bool {
				return cfg.OpusComplexity == 10
			},
		},
		{
			name:     "ECHOWARP_MAX_CLIENTS",
			envVar:   "ECHOWARP_MAX_CLIENTS",
			envValue: "15",
			check: func(cfg *config.Config) bool {
				return cfg.MaxClients == 15
			},
		},
		{
			name:     "ECHOWARP_MAX_RECONNECT_ATTEMPTS",
			envVar:   "ECHOWARP_MAX_RECONNECT_ATTEMPTS",
			envValue: "20",
			check: func(cfg *config.Config) bool {
				return cfg.MaxReconnectAttempts == 20
			},
		},
		{
			name:     "ECHOWARP_MAX_FAILED_ATTEMPTS",
			envVar:   "ECHOWARP_MAX_FAILED_ATTEMPTS",
			envValue: "10",
			check: func(cfg *config.Config) bool {
				return cfg.MaxFailedAttempts == 10
			},
		},
		{
			name:     "ECHOWARP_NO_SIMD_OPTIMIZATION",
			envVar:   "ECHOWARP_NO_SIMD_OPTIMIZATION",
			envValue: "true",
			check: func(cfg *config.Config) bool {
				return cfg.NoSIMDOptimization == true
			},
		},
		{
			name:     "ECHOWARP_NO_POOL_WARMUP",
			envVar:   "ECHOWARP_NO_POOL_WARMUP",
			envValue: "true",
			check: func(cfg *config.Config) bool {
				return cfg.NoPoolWarmup == true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.envVar, tt.envValue)
			defer os.Unsetenv(tt.envVar)

			cmd := newServerCmd()
			_ = cmd.ParseFlags([]string{})

			cfg, err := loadConfig(cmd, config.ModeServer)
			if err != nil {
				t.Fatalf("loadConfig failed: %v", err)
			}

			if !tt.check(&cfg) {
				t.Errorf("Environment variable %s was not applied correctly", tt.name)
			}
		})
	}
}

// TestEnvVarIntegration_Client tests all supported environment variables for client mode.
func TestEnvVarIntegration_Client(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		check    func(cfg *config.Config) bool
	}{
		{
			name:     "ECHOWARP_ADDRESS",
			envVar:   "ECHOWARP_ADDRESS",
			envValue: "192.168.1.100",
			check: func(cfg *config.Config) bool {
				return cfg.Address == "192.168.1.100"
			},
		},
		{
			name:     "ECHOWARP_TLS",
			envVar:   "ECHOWARP_TLS",
			envValue: "true",
			check: func(cfg *config.Config) bool {
				return cfg.TLS == true
			},
		},
		{
			name:     "ECHOWARP_TLS_INSECURE",
			envVar:   "ECHOWARP_TLS_INSECURE",
			envValue: "true",
			check: func(cfg *config.Config) bool {
				return cfg.TLSInsecure == true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tt.envVar, tt.envValue)
			defer os.Unsetenv(tt.envVar)

			cmd := newClientCmd()
			_ = cmd.ParseFlags([]string{})

			cfg, err := loadClientConfig(cmd)
			if err != nil {
				t.Fatalf("loadClientConfig failed: %v", err)
			}

			if !tt.check(cfg) {
				t.Errorf("Environment variable %s was not applied correctly", tt.name)
			}
		})
	}
}

// TestEnvVarIntegration_Daemon tests environment variables work for daemon commands.
func TestEnvVarIntegration_Daemon(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "daemon.pid")

	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_PASSWORD", "testpass")
	os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_PASSWORD")
		os.Unsetenv("ECHOWARP_LOG_LEVEL")
	}()

	cmd := newDaemonStartCmd()
	_ = cmd.ParseFlags([]string{"--pid-file", pidFile})

	_, cfg, _, err := setupDaemon(cmd)
	if err != nil {
		t.Fatalf("setupDaemon failed: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("expected port 9999 from env var, got %d", cfg.Port)
	}
	if cfg.Password != "testpass" {
		t.Errorf("expected password 'testpass' from env var, got %s", cfg.Password)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level 'debug' from env var, got %s", cfg.LogLevel)
	}
}

// TestEnvVarIntegration_STUNCommaSeparated tests comma-separated STUN servers from env var.
func TestEnvVarIntegration_STUNCommaSeparated(t *testing.T) {
	os.Setenv("ECHOWARP_STUN_SERVERS", "stun:server1.com:3478,stun:server2.com:3478")
	defer os.Unsetenv("ECHOWARP_STUN_SERVERS")

	cmd := newServerCmd()
	_ = cmd.ParseFlags([]string{})

	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Note: Viper may include empty strings when parsing comma-separated values
	// Filter out non-empty values
	var validServers []string
	for _, s := range cfg.STUNServers {
		if s != "" {
			validServers = append(validServers, s)
		}
	}

	if len(validServers) != 2 {
		t.Errorf("expected 2 STUN servers, got %d: %v", len(validServers), validServers)
		return
	}

	expected := []string{"stun:server1.com:3478", "stun:server2.com:3478"}
	for i, srv := range validServers {
		if srv != expected[i] {
			t.Errorf("STUN server[%d] = %q, want %q", i, srv, expected[i])
		}
	}
}

// TestEnvVarIntegration_FullPriorityChain tests the complete priority chain:
// CLI flags > environment variables > config file > defaults
func TestEnvVarIntegration_FullPriorityChain(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `port: 8080
sample_rate: 24000
channels: 2
max_clients: 5
log_level: warn
password: filepass
`
	_ = os.WriteFile(configFile, []byte(configContent), 0600)

	// Set environment variables
	os.Setenv("ECHOWARP_PORT", "9999")
	os.Setenv("ECHOWARP_SAMPLE_RATE", "16000")
	os.Setenv("ECHOWARP_PASSWORD", "envpass")
	defer func() {
		os.Unsetenv("ECHOWARP_PORT")
		os.Unsetenv("ECHOWARP_SAMPLE_RATE")
		os.Unsetenv("ECHOWARP_PASSWORD")
	}()

	// Test 1: No CLI overrides - env vars should override config file
	t.Run("env_overrides_config", func(t *testing.T) {
		cmd := newServerCmd()
		_ = cmd.ParseFlags([]string{"--config", configFile})

		cfg, err := loadConfig(cmd, config.ModeServer)
		if err != nil {
			t.Fatalf("loadConfig failed: %v", err)
		}

		// Port: env var (9999) should override config file (8080)
		if cfg.Port != 9999 {
			t.Errorf("port = %d, want 9999 from env var", cfg.Port)
		}
		// Sample rate: env var (16000) should override config file (24000)
		if cfg.SampleRate != 16000 {
			t.Errorf("sample_rate = %d, want 16000 from env var", cfg.SampleRate)
		}
		// Password: env var (envpass) should override config file (filepass)
		if cfg.Password != "envpass" {
			t.Errorf("password = %s, want 'envpass' from env var", cfg.Password)
		}
		// Channels: config file (2) should be used (no env var)
		if cfg.Channels != 2 {
			t.Errorf("channels = %d, want 2 from config file", cfg.Channels)
		}
	})

	// Test 2: CLI flags override everything
	t.Run("cli_overrides_all", func(t *testing.T) {
		cmd := newServerCmd()
		_ = cmd.ParseFlags([]string{
			"--config", configFile,
			"--port", "7777",
			"--password", "clipass",
		})

		cfg, err := loadConfig(cmd, config.ModeServer)
		if err != nil {
			t.Fatalf("loadConfig failed: %v", err)
		}

		// Port: CLI (7777) should override env var (9999) and config file (8080)
		if cfg.Port != 7777 {
			t.Errorf("port = %d, want 7777 from CLI flag", cfg.Port)
		}
		// Password: CLI (clipass) should override env var (envpass) and config file (filepass)
		if cfg.Password != "clipass" {
			t.Errorf("password = %s, want 'clipass' from CLI flag", cfg.Password)
		}
		// Sample rate: env var (16000) should still override config file (24000)
		if cfg.SampleRate != 16000 {
			t.Errorf("sample_rate = %d, want 16000 from env var", cfg.SampleRate)
		}
	})

	// Test 3: No config file - env vars override defaults
	t.Run("env_overrides_defaults", func(t *testing.T) {
		cmd := newServerCmd()
		_ = cmd.ParseFlags([]string{})

		cfg, err := loadConfig(cmd, config.ModeServer)
		if err != nil {
			t.Fatalf("loadConfig failed: %v", err)
		}

		// Port: env var (9999) should override default (4415)
		if cfg.Port != 9999 {
			t.Errorf("port = %d, want 9999 from env var", cfg.Port)
		}
		// Sample rate: env var (16000) should override default (48000)
		if cfg.SampleRate != 16000 {
			t.Errorf("sample_rate = %d, want 16000 from env var", cfg.SampleRate)
		}
		// Channels: default (2) should be used (no env var)
		if cfg.Channels != 2 {
			t.Errorf("channels = %d, want 2 from defaults", cfg.Channels)
		}
	})
}

// TestEnvVarIntegration_DocumentationExample tests the examples from documentation.
// This ensures the documented behavior matches actual implementation.
func TestEnvVarIntegration_DocumentationExample(t *testing.T) {
	// Example 1: ECHOWARP_PORT=9999 echowarp server
	t.Run("port_override", func(t *testing.T) {
		os.Setenv("ECHOWARP_PORT", "9999")
		defer os.Unsetenv("ECHOWARP_PORT")

		cmd := newServerCmd()
		_ = cmd.ParseFlags([]string{})

		cfg, err := loadConfig(cmd, config.ModeServer)
		if err != nil {
			t.Fatalf("loadConfig failed: %v", err)
		}

		if cfg.Port != 9999 {
			t.Errorf("Expected port 9999, got %d", cfg.Port)
		}
	})

	// Example 2: ECHOWARP_PASSWORD=secret echowarp client
	t.Run("password_override", func(t *testing.T) {
		os.Setenv("ECHOWARP_PASSWORD", "secret")
		os.Setenv("ECHOWARP_ADDRESS", "192.168.1.1")
		defer func() {
			os.Unsetenv("ECHOWARP_PASSWORD")
			os.Unsetenv("ECHOWARP_ADDRESS")
		}()

		cmd := newClientCmd()
		_ = cmd.ParseFlags([]string{})

		cfg, err := loadClientConfig(cmd)
		if err != nil {
			t.Fatalf("loadClientConfig failed: %v", err)
		}

		if cfg.Password != "secret" {
			t.Errorf("Expected password 'secret', got %s", cfg.Password)
		}
	})

	// Example 3: ECHOWARP_LOG_LEVEL=debug echowarp daemon start
	t.Run("log_level_override", func(t *testing.T) {
		tmpDir := t.TempDir()
		pidFile := filepath.Join(tmpDir, "daemon.pid")

		os.Setenv("ECHOWARP_LOG_LEVEL", "debug")
		defer os.Unsetenv("ECHOWARP_LOG_LEVEL")

		cmd := newDaemonStartCmd()
		_ = cmd.ParseFlags([]string{"--pid-file", pidFile})

		_, cfg, _, err := setupDaemon(cmd)
		if err != nil {
			t.Fatalf("setupDaemon failed: %v", err)
		}

		if cfg.LogLevel != "debug" {
			t.Errorf("Expected log level 'debug', got %s", cfg.LogLevel)
		}
	})
}
