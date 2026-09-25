package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EchoWarpDir returns the base directory for all EchoWarp files.
// It uses ECHOWARP_CONFIG_DIR env var if set (useful for tests),
// otherwise returns ~/.config/echowarp.
// Falls back to ./.echowarp if the home directory cannot be determined.
func EchoWarpDir() string {
	if dir := os.Getenv("ECHOWARP_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".echowarp")
	}
	return filepath.Join(home, ".config", "echowarp")
}

// ── LoadFromFile ──────────────────────────────────────────────────────────────

// LoadFromFile reads configuration from a YAML file. Missing fields retain
// their default values from DefaultConfig(). Supports both nested and flat
// YAML formats; nested format is tried first for forward compatibility.
//
// Security checks:
//   - File size limited to 1MB to prevent OOM
//   - Warns if file permissions are too permissive (should be 0600)
//   - Warns if TLSInsecure is enabled
//
// Returns an error if the file cannot be read or parsed.
func LoadFromFile(path string) (Config, error) {
	cfg := DefaultConfig()

	stat, err := os.Stat(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	data, err := readBoundedConfig(path)
	if err != nil {
		return cfg, err
	}

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		var n nestedConfig
		if err := json.Unmarshal(data, &n); err != nil {
			return cfg, fmt.Errorf("failed to parse JSON config file %q", path)
		}
		cfg = fromNested(cfg, n)
	default:
		// YAML: try nested, then flat (backward-compatible)
		if isNestedYAML(data) {
			var n nestedConfig
			if err := yaml.Unmarshal(data, &n); err != nil {
				return cfg, fmt.Errorf("failed to parse config file %q", path)
			}
			cfg = fromNested(cfg, n)
		} else {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("failed to parse config file %q", path)
			}
		}
	}

	if mode := stat.Mode(); mode.Perm()&0077 != 0 {
		fmt.Fprintf(os.Stderr, "Warning: config file %s has insecure permissions: %o (should be 0600)\n", path, mode.Perm())
	}

	if cfg.TLSInsecure {
		fmt.Fprintln(os.Stderr, "Warning: TLS certificate verification is DISABLED. This is insecure and should only be used for testing!")
	}

	// Parse client modes: map if present
	if cfg.Mode == ModeClient {
		loadClientModes(&cfg, data, ext)
	}

	return cfg, nil
}

// loadClientModes parses the modes: map from raw config data and stores it in cfg.ClientModes.
func loadClientModes(cfg *Config, data []byte, ext string) {
	if ext == ".json" {
		return
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	modesRaw, ok := raw["modes"]
	if !ok {
		return
	}
	modesMap, ok := modesRaw.(map[string]interface{})
	if !ok || len(modesMap) == 0 {
		return
	}
	cfg.ClientModes = modesMap
}

// ── SaveToFile ────────────────────────────────────────────────────────────────

const yamlHeader = "# EchoWarp configuration file\n# See: https://github.com/lHumaNl/echowarp\n\n"

// SaveToFile writes the configuration to a YAML file in nested format.
// Creates parent directories with 0700 permissions.
// The file is written with 0600 permissions (owner read/write only).
func (c Config) SaveToFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))

	var out []byte
	switch ext {
	case ".json":
		jsonData, err := json.MarshalIndent(c.toNested(), "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		out = append(jsonData, '\n') //nolint:gocritic // appendAssign: jsonData is intentionally not reused
	default:
		yamlData, err := yaml.Marshal(c.toNested())
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		out = append([]byte(yamlHeader), yamlData...)
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ── LoadWithViper ─────────────────────────────────────────────────────────────

// LoadWithViper loads defaults, an optional explicit YAML/JSON file, and
// ECHOWARP_* environment overrides, in increasing priority. CLI flags are
// applied separately by Cobra. The mode argument fixes the command role;
// ECHOWARP_MODE selects the audio streaming mode, not the role.
// The historical name is retained for callers; LoadOver is the shared loader.
func LoadWithViper(configPath string, mode Mode) (Config, error) {
	cfg := DefaultConfig()
	cfg.Mode = mode
	switch mode {
	case ModeClient:
		cfg.AutoReconnectAttempts = defaultClientReconnectAttempts
	case ModeServer:
		cfg.RateLimit = DefaultServerRateLimit
	}
	cfg, _, err := LoadOver(cfg, configPath)
	if err != nil {
		return Config{}, err
	}
	warnLoadedConfig(cfg, configPath)
	return cfg, nil
}

const defaultClientReconnectAttempts = 5

// DefaultServerRateLimit matches the established server CLI connection limit.
const DefaultServerRateLimit = 5

func warnLoadedConfig(cfg Config, configPath string) {
	if configPath != "" {
		if stat, err := os.Stat(configPath); err == nil {
			if fileMode := stat.Mode(); fileMode.Perm()&0077 != 0 {
				fmt.Fprintf(os.Stderr, "Warning: config file %s has insecure permissions: %o (should be 0600)\n", configPath, fileMode.Perm())
			}
		}
	}
	if cfg.TLSInsecure {
		fmt.Fprintln(os.Stderr, "Warning: TLS certificate verification is DISABLED. This is insecure and should only be used for testing!")
	}
}
