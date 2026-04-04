//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestE2E_Config_LoadSave_Roundtrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	original := config.Config{
		Mode:            config.ModeServer,
		Port:            9999,
		Password:        "test_pass",
		SampleRate:      48000,
		Channels:        2,
		OpusBitrate:     96000,
		OpusComplexity:  8,
		OpusApplication: "audio",
		OpusDTX:         true,
		OpusFEC:         true,
		STUNServers: []string{
			"stun:custom.stun.com:3478",
		},
		MaxClients:           5,
		MaxReconnectAttempts: 10,
		ReconnectIntervalSec: 2,
		LogLevel:             "debug",
	}

	if err := original.SaveToFile(configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("config file has insecure permissions: %o", info.Mode().Perm())
	}

	loaded, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Mode != original.Mode {
		t.Errorf("mode mismatch: got %s, want %s", loaded.Mode, original.Mode)
	}
	if loaded.Port != original.Port {
		t.Errorf("port mismatch: got %d, want %d", loaded.Port, original.Port)
	}
	if loaded.Password != original.Password {
		t.Errorf("password mismatch: got %s, want %s", loaded.Password, original.Password)
	}
	if loaded.SampleRate != original.SampleRate {
		t.Errorf("sample rate mismatch: got %d, want %d", loaded.SampleRate, original.SampleRate)
	}
	if loaded.Channels != original.Channels {
		t.Errorf("channels mismatch: got %d, want %d", loaded.Channels, original.Channels)
	}
	if loaded.OpusBitrate != original.OpusBitrate {
		t.Errorf("opus bitrate mismatch: got %d, want %d", loaded.OpusBitrate, original.OpusBitrate)
	}
	if len(loaded.STUNServers) != len(original.STUNServers) {
		t.Errorf("STUN servers count mismatch: got %d, want %d", len(loaded.STUNServers), len(original.STUNServers))
	} else if len(loaded.STUNServers) > 0 && loaded.STUNServers[0] != original.STUNServers[0] {
		t.Errorf("STUN server mismatch: got %s, want %s", loaded.STUNServers[0], original.STUNServers[0])
	}

	loaded.Password = "new_password"
	loaded.Port = 8888

	if err := loaded.SaveToFile(configPath); err != nil {
		t.Fatalf("failed to save modified config: %v", err)
	}

	reloaded, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if reloaded.Password != "new_password" {
		t.Errorf("password not updated: got %s", reloaded.Password)
	}
	if reloaded.Port != 8888 {
		t.Errorf("port not updated: got %d", reloaded.Port)
	}
}

func TestE2E_Config_Validate_InvalidValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name:    "invalid port zero",
			config:  config.Config{Port: 0},
			wantErr: true,
		},
		{
			name:    "invalid port too high",
			config:  config.Config{Port: 70000},
			wantErr: true,
		},
		{
			name:    "client mode without address",
			config:  config.Config{Mode: config.ModeClient, Address: ""},
			wantErr: true,
		},
		{
			name:    "invalid sample rate",
			config:  config.Config{SampleRate: 44100},
			wantErr: true,
		},
		{
			name:    "invalid channels",
			config:  config.Config{Channels: 3},
			wantErr: true,
		},
		{
			name:    "invalid log level",
			config:  config.Config{LogLevel: "trace"},
			wantErr: true,
		},
		{
			name:    "negative reconnect attempts",
			config:  config.Config{MaxReconnectAttempts: -1},
			wantErr: true,
		},
		{
			name:    "negative max failed attempts",
			config:  config.Config{MaxFailedAttempts: -1},
			wantErr: true,
		},
		{
			name: "valid config",
			config: config.Config{
				Mode:       config.ModeServer,
				Port:       4415,
				SampleRate: 48000,
				Channels:   1,
				LogLevel:   "info",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.config.Validate()
			hasErr := len(errs) > 0
			if hasErr != tc.wantErr {
				if tc.wantErr {
					t.Errorf("expected validation errors, got none")
				} else {
					t.Errorf("unexpected validation errors: %v", errs)
				}
			}
		})
	}
}

func TestE2E_Config_Partial_MergesWithDefaults(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "partial_config.yaml")

	partialYAML := "port: 9999\n"
	if err := os.WriteFile(configPath, []byte(partialYAML), 0600); err != nil {
		t.Fatalf("failed to write partial config: %v", err)
	}

	loaded, err := config.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load partial config: %v", err)
	}

	if loaded.Port != 9999 {
		t.Errorf("port should be 9999 from file, got %d", loaded.Port)
	}

	defaultCfg := config.DefaultConfig()
	if loaded.SampleRate != defaultCfg.SampleRate {
		t.Errorf("sample rate should be default %d, got %d", defaultCfg.SampleRate, loaded.SampleRate)
	}
	if loaded.Channels != defaultCfg.Channels {
		t.Errorf("channels should be default %d, got %d", defaultCfg.Channels, loaded.Channels)
	}
	if len(loaded.STUNServers) != len(defaultCfg.STUNServers) {
		t.Errorf("STUN servers should have %d default entries, got %d", len(defaultCfg.STUNServers), len(loaded.STUNServers))
	}
}
