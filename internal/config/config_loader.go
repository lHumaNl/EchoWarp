package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
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

	const maxSize = 1 << 20
	if stat.Size() > maxSize {
		return cfg, fmt.Errorf("config file too large (max %d bytes)", maxSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		var n nestedConfig
		if err := json.Unmarshal(data, &n); err != nil {
			return cfg, fmt.Errorf("failed to parse JSON config file: %w", err)
		}
		cfg = fromNested(cfg, n)
	default:
		// YAML: try nested, then flat (backward-compatible)
		if isNestedYAML(data) {
			var n nestedConfig
			if err := yaml.Unmarshal(data, &n); err != nil {
				return cfg, fmt.Errorf("failed to parse config file: %w", err)
			}
			cfg = fromNested(cfg, n)
		} else {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("failed to parse config file: %w", err)
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

// LoadWithViper loads configuration with full priority support:
// 1. CLI flags (highest priority - handled separately by cobra)
// 2. Environment variables (ECHOWARP_*)
// 3. Config file (YAML, nested or flat)
// 4. Default values (lowest priority)
//
// Environment variables are mapped as follows:
//   - ECHOWARP_PORT → Port
//   - ECHOWARP_PASSWORD → Password
//   - ECHOWARP_MODE → Mode (server/client)
//   - ECHOWARP_DEVICE_ID → DeviceID
//   - ECHOWARP_SAMPLE_RATE → SampleRate
//   - ECHOWARP_CHANNELS → Channels
//   - ECHOWARP_OPUS_BITRATE → OpusBitrate
//   - ECHOWARP_STUN_SERVERS → STUNServers (comma-separated)
//   - And other fields following the same pattern
//
// If configPath is empty, only env vars and defaults are used.
// The config file format must be YAML (nested or flat).
func LoadWithViper(configPath string, mode Mode) (Config, error) {
	// Start with defaults
	cfg := DefaultConfig()
	cfg.Mode = mode

	v := viper.New()

	// Configure environment variable support
	v.SetEnvPrefix("ECHOWARP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Configure environment variable bindings for all fields
	bindEnvs(v)

	// Load config file if specified
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Helper: check for both nested key and legacy flat key
	isSet := func(nested, flat string) bool {
		return v.IsSet(nested) || v.IsSet(flat)
	}
	getString := func(nested, flat string) string {
		if v.IsSet(nested) {
			return v.GetString(nested)
		}
		return v.GetString(flat)
	}
	getBool := func(nested, flat string) bool {
		if v.IsSet(nested) {
			return v.GetBool(nested)
		}
		return v.GetBool(flat)
	}
	getInt := func(nested, flat string) int {
		if v.IsSet(nested) {
			return v.GetInt(nested)
		}
		return v.GetInt(flat)
	}

	// Apply viper values to config struct, overriding defaults
	if v.IsSet("mode") {
		cfg.StreamMode = AudioMode(v.GetString("mode"))
		cfg.SyncFromStreamMode()
	}
	if v.IsSet("server_muted") {
		cfg.ServerMuted = v.GetBool("server_muted")
	}
	if v.IsSet("nickname") {
		cfg.Nickname = v.GetString("nickname")
	}
	if v.IsSet("hwid_required") {
		cfg.HWIDRequired = v.GetBool("hwid_required")
	}

	if isSet("network.port", "port") {
		cfg.Port = getInt("network.port", "port")
	}
	if isSet("network.address", "address") {
		cfg.Address = getString("network.address", "address")
	}

	if isSet("audio.device_id", "device_id") {
		id := uint32(v.GetUint("audio.device_id"))
		if !v.IsSet("audio.device_id") {
			id = uint32(v.GetUint("device_id"))
		}
		cfg.DeviceID = &id
	}
	// Multi-device: load from audio.devices array
	if v.IsSet("audio.devices") {
		var devEntries []DeviceEntry
		if rawDevices := v.Get("audio.devices"); rawDevices != nil {
			if slice, ok := rawDevices.([]interface{}); ok {
				for _, item := range slice {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					entry := DeviceEntry{Volume: 1.0}
					if id, ok := m["id"]; ok {
						switch v := id.(type) {
						case int:
							entry.ID = uint32(v)
						case float64:
							entry.ID = uint32(v)
						case int64:
							entry.ID = uint32(v)
						}
					}
					if role, ok := m["role"].(string); ok {
						entry.Role = DeviceRole(role)
					}
					if vol, ok := m["volume"]; ok {
						switch v := vol.(type) {
						case float64:
							entry.Volume = v
						case int:
							entry.Volume = float64(v)
						}
					}
					if muted, ok := m["muted"].(bool); ok {
						entry.Muted = muted
					}
					if name, ok := m["name"].(string); ok {
						entry.Name = name
					}
					if typ, ok := m["type"].(string); ok {
						entry.Type = DeviceType(typ)
					}
					devEntries = append(devEntries, entry)
				}
			}
		}
		if len(devEntries) > 0 {
			cfg.Devices = devEntries
		}
	}

	if isSet("audio.sample_rate", "sample_rate") {
		cfg.SampleRate = uint32(getInt("audio.sample_rate", "sample_rate"))
	}
	if isSet("audio.channels", "channels") {
		cfg.Channels = uint32(getInt("audio.channels", "channels"))
	}
	if isSet("audio.virtual_mic", "virtual_mic") {
		cfg.VirtualMic = getBool("audio.virtual_mic", "virtual_mic")
	}
	if isSet("audio.loopback", "loopback") {
		cfg.Loopback = getBool("audio.loopback", "loopback")
	}
	if isSet("audio.buffer_frames", "audio_buffer_frames") {
		cfg.AudioBufferFrames = getInt("audio.buffer_frames", "audio_buffer_frames")
	}

	if isSet("opus.bitrate", "opus_bitrate") {
		cfg.OpusBitrate = getInt("opus.bitrate", "opus_bitrate")
	}
	if isSet("opus.complexity", "opus_complexity") {
		cfg.OpusComplexity = getInt("opus.complexity", "opus_complexity")
	}
	if isSet("opus.application", "opus_application") {
		cfg.OpusApplication = getString("opus.application", "opus_application")
	}
	if isSet("opus.dtx", "opus_dtx") {
		cfg.OpusDTX = getBool("opus.dtx", "opus_dtx")
	}
	if isSet("opus.fec", "opus_fec") {
		cfg.OpusFEC = getBool("opus.fec", "opus_fec")
	}

	if isSet("security.password", "password") {
		cfg.Password = getString("security.password", "password")
	}
	if isSet("security.password_hash", "password_hash") {
		cfg.PasswordHash = getString("security.password_hash", "password_hash")
	}
	if isSet("security.tls.cert", "tls_cert") {
		cfg.TLSCert = getString("security.tls.cert", "tls_cert")
	}
	if isSet("security.tls.key", "tls_key") {
		cfg.TLSKey = getString("security.tls.key", "tls_key")
	}
	if isSet("security.tls.enabled", "tls") {
		cfg.TLS = getBool("security.tls.enabled", "tls")
	}
	if isSet("security.tls.insecure", "tls_insecure") {
		cfg.TLSInsecure = getBool("security.tls.insecure", "tls_insecure")
	}
	if isSet("security.max_auth_failures", "max_failed_attempts") {
		cfg.MaxFailedAttempts = getInt("security.max_auth_failures", "max_failed_attempts")
	}
	if isSet("security.ban_file", "ban_file_path") {
		cfg.BanFilePath = getString("security.ban_file", "ban_file_path")
	}

	// STUN servers: support nested, flat, and comma-separated env var
	stunKey := ""
	if v.IsSet("network.stun_servers") {
		stunKey = "network.stun_servers"
	} else if v.IsSet("stun_servers") {
		stunKey = "stun_servers"
	}
	if stunKey != "" {
		servers := v.GetStringSlice(stunKey)
		if len(servers) == 0 || (len(servers) == 1 && strings.Contains(servers[0], ",")) {
			if stunStr := v.GetString(stunKey); stunStr != "" {
				servers = strings.Split(stunStr, ",")
				for i := range servers {
					servers[i] = strings.TrimSpace(servers[i])
				}
			}
		}
		if len(servers) > 0 {
			cfg.STUNServers = servers
		}
	}

	if isSet("connection.max_clients", "max_clients") {
		cfg.MaxClients = getInt("connection.max_clients", "max_clients")
	}
	if isSet("connection.max_reconnect_attempts", "max_reconnect_attempts") {
		cfg.MaxReconnectAttempts = getInt("connection.max_reconnect_attempts", "max_reconnect_attempts")
	}
	if isSet("connection.reconnect_interval_sec", "reconnect_interval_sec") {
		cfg.ReconnectIntervalSec = getInt("connection.reconnect_interval_sec", "reconnect_interval_sec")
	}
	if isSet("connection.auto_reconnect", "auto_reconnect") {
		cfg.AutoReconnect = getBool("connection.auto_reconnect", "auto_reconnect")
	}
	if isSet("connection.auto_reconnect_attempts", "auto_reconnect_attempts") {
		cfg.AutoReconnectAttempts = getInt("connection.auto_reconnect_attempts", "auto_reconnect_attempts")
	}

	if isSet("logging.level", "log_level") {
		cfg.LogLevel = getString("logging.level", "log_level")
	}
	if isSet("logging.to_file", "log_to_file") {
		cfg.LogToFile = getBool("logging.to_file", "log_to_file")
	}
	if isSet("logging.file", "log_file") {
		cfg.LogFile = getString("logging.file", "log_file")
	}

	if isSet("performance.no_simd", "no_simd_optimization") {
		cfg.NoSIMDOptimization = getBool("performance.no_simd", "no_simd_optimization")
	}
	if isSet("performance.no_pool_warmup", "no_pool_warmup") {
		cfg.NoPoolWarmup = getBool("performance.no_pool_warmup", "no_pool_warmup")
	}

	// Security warnings
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

	return cfg, nil
}

// bindEnvs explicitly binds environment variables that AutomaticEnv might miss.
// Both nested keys and legacy flat keys are bound to the same ECHOWARP_* env vars.
//
//nolint:errcheck // v.BindEnv never fails in practice (only fails for empty key)
func bindEnvs(v *viper.Viper) {
	// Core settings
	_ = v.BindEnv("role", "ECHOWARP_ROLE")
	_ = v.BindEnv("mode", "ECHOWARP_MODE")
	_ = v.BindEnv("server_muted", "ECHOWARP_SERVER_MUTED")

	// Network (nested + flat)
	_ = v.BindEnv("network.port", "ECHOWARP_PORT")
	_ = v.BindEnv("port", "ECHOWARP_PORT")
	_ = v.BindEnv("network.address", "ECHOWARP_ADDRESS")
	_ = v.BindEnv("address", "ECHOWARP_ADDRESS")
	_ = v.BindEnv("network.stun_servers", "ECHOWARP_STUN_SERVERS")
	_ = v.BindEnv("stun_servers", "ECHOWARP_STUN_SERVERS")

	// Audio settings (nested + flat)
	_ = v.BindEnv("audio.device_id", "ECHOWARP_DEVICE_ID")
	_ = v.BindEnv("device_id", "ECHOWARP_DEVICE_ID")
	_ = v.BindEnv("audio.input_device_id", "ECHOWARP_INPUT_DEVICE_ID")
	_ = v.BindEnv("audio.output_device_id", "ECHOWARP_OUTPUT_DEVICE_ID")
	_ = v.BindEnv("audio.sample_rate", "ECHOWARP_SAMPLE_RATE")
	_ = v.BindEnv("sample_rate", "ECHOWARP_SAMPLE_RATE")
	_ = v.BindEnv("audio.channels", "ECHOWARP_CHANNELS")
	_ = v.BindEnv("channels", "ECHOWARP_CHANNELS")
	_ = v.BindEnv("audio.virtual_mic", "ECHOWARP_VIRTUAL_MIC")
	_ = v.BindEnv("virtual_mic", "ECHOWARP_VIRTUAL_MIC")
	_ = v.BindEnv("audio.loopback", "ECHOWARP_LOOPBACK")
	_ = v.BindEnv("loopback", "ECHOWARP_LOOPBACK")
	_ = v.BindEnv("audio.buffer_frames", "ECHOWARP_AUDIO_BUFFER_FRAMES")
	_ = v.BindEnv("audio_buffer_frames", "ECHOWARP_AUDIO_BUFFER_FRAMES")

	// Opus codec settings (nested + flat)
	_ = v.BindEnv("opus.bitrate", "ECHOWARP_OPUS_BITRATE")
	_ = v.BindEnv("opus_bitrate", "ECHOWARP_OPUS_BITRATE")
	_ = v.BindEnv("opus.complexity", "ECHOWARP_OPUS_COMPLEXITY")
	_ = v.BindEnv("opus_complexity", "ECHOWARP_OPUS_COMPLEXITY")
	_ = v.BindEnv("opus.application", "ECHOWARP_OPUS_APPLICATION")
	_ = v.BindEnv("opus_application", "ECHOWARP_OPUS_APPLICATION")
	_ = v.BindEnv("opus.dtx", "ECHOWARP_OPUS_DTX")
	_ = v.BindEnv("opus_dtx", "ECHOWARP_OPUS_DTX")
	_ = v.BindEnv("opus.fec", "ECHOWARP_OPUS_FEC")
	_ = v.BindEnv("opus_fec", "ECHOWARP_OPUS_FEC")

	// Auth/Security settings (nested + flat)
	_ = v.BindEnv("security.password", "ECHOWARP_PASSWORD")
	_ = v.BindEnv("password", "ECHOWARP_PASSWORD")
	_ = v.BindEnv("security.password_hash", "ECHOWARP_PASSWORD_HASH")
	_ = v.BindEnv("password_hash", "ECHOWARP_PASSWORD_HASH")
	_ = v.BindEnv("security.tls.cert", "ECHOWARP_TLS_CERT")
	_ = v.BindEnv("tls_cert", "ECHOWARP_TLS_CERT")
	_ = v.BindEnv("security.tls.key", "ECHOWARP_TLS_KEY")
	_ = v.BindEnv("tls_key", "ECHOWARP_TLS_KEY")
	_ = v.BindEnv("security.tls.enabled", "ECHOWARP_TLS")
	_ = v.BindEnv("tls", "ECHOWARP_TLS")
	_ = v.BindEnv("security.tls.insecure", "ECHOWARP_TLS_INSECURE")
	_ = v.BindEnv("tls_insecure", "ECHOWARP_TLS_INSECURE")
	_ = v.BindEnv("security.max_auth_failures", "ECHOWARP_MAX_FAILED_ATTEMPTS")
	_ = v.BindEnv("max_failed_attempts", "ECHOWARP_MAX_FAILED_ATTEMPTS")
	_ = v.BindEnv("security.ban_file", "ECHOWARP_BAN_FILE_PATH")
	_ = v.BindEnv("ban_file_path", "ECHOWARP_BAN_FILE_PATH")

	// Connection settings (nested + flat)
	_ = v.BindEnv("connection.max_clients", "ECHOWARP_MAX_CLIENTS")
	_ = v.BindEnv("max_clients", "ECHOWARP_MAX_CLIENTS")
	_ = v.BindEnv("connection.max_reconnect_attempts", "ECHOWARP_MAX_RECONNECT_ATTEMPTS")
	_ = v.BindEnv("max_reconnect_attempts", "ECHOWARP_MAX_RECONNECT_ATTEMPTS")
	_ = v.BindEnv("connection.reconnect_interval_sec", "ECHOWARP_RECONNECT_INTERVAL_SEC")
	_ = v.BindEnv("reconnect_interval_sec", "ECHOWARP_RECONNECT_INTERVAL_SEC")

	// Logging settings (nested + flat)
	_ = v.BindEnv("logging.level", "ECHOWARP_LOG_LEVEL")
	_ = v.BindEnv("log_level", "ECHOWARP_LOG_LEVEL")
	_ = v.BindEnv("logging.to_file", "ECHOWARP_LOG_TO_FILE")
	_ = v.BindEnv("log_to_file", "ECHOWARP_LOG_TO_FILE")
	_ = v.BindEnv("logging.file", "ECHOWARP_LOG_FILE")
	_ = v.BindEnv("log_file", "ECHOWARP_LOG_FILE")

	// Chat settings
	_ = v.BindEnv("nickname", "ECHOWARP_NICKNAME")
	_ = v.BindEnv("hwid_required", "ECHOWARP_HWID_REQUIRED")

	// Optimization settings (nested + flat)
	_ = v.BindEnv("performance.no_simd", "ECHOWARP_NO_SIMD_OPTIMIZATION")
	_ = v.BindEnv("no_simd_optimization", "ECHOWARP_NO_SIMD_OPTIMIZATION")
	_ = v.BindEnv("performance.no_pool_warmup", "ECHOWARP_NO_POOL_WARMUP")
	_ = v.BindEnv("no_pool_warmup", "ECHOWARP_NO_POOL_WARMUP")
}
