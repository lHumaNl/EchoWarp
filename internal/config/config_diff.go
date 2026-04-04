package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigEntry represents a saved config file in the configs directory.
type ConfigEntry struct {
	Name     string
	Path     string
	ModTime  time.Time
	InfoLine string
}

// ConfigsDir returns the directory for saved configs of the given mode.
// Server: ~/.config/echowarp/configs/server/
// Client: ~/.config/echowarp/configs/client/
func ConfigsDir(mode Mode) string {
	subdir := "server"
	if mode == ModeClient {
		subdir = "client"
	}
	return filepath.Join(EchoWarpDir(), "configs", subdir)
}

// SaveNonDefault saves only non-default config fields to a YAML file.
// For server mode, produces a flat diff. For client mode, produces a modes: map
// structure where base fields (address, port, password, nickname) are at the top
// level and all other settings go under modes:<current_mode>:.
// When saving client config, existing file is read and modes are merged
// (other mode keys are preserved, current mode key is replaced).
func SaveNonDefault(cfg Config, mode Mode, path string) error {
	var m map[string]interface{}
	if mode == ModeClient {
		m = diffClientConfig(cfg, DefaultConfig(), path)
	} else {
		m = diffConfig(cfg, DefaultConfig(), mode)
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal config diff: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	perm := os.FileMode(0600)
	if runtime.GOOS == "windows" {
		perm = 0666
	}

	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// diffClientConfig builds a client config map with modes: map structure.
// Base fields (role, address, port, password, nickname) go at top level.
// Everything else goes under modes:<current_mode>:.
// Reads existing file to merge/preserve other mode entries.
func diffClientConfig(cfg, defaults Config, path string) map[string]interface{} {
	// Read existing file for merge
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &existing)
	}

	root := make(map[string]interface{})
	root["role"] = string(ModeClient)

	// Base fields (always at top level)
	root["address"] = cfg.Address
	if cfg.Port != defaults.Port {
		root["port"] = cfg.Port
	}
	if cfg.Password != "" {
		root["password"] = cfg.Password
	}
	if cfg.Nickname != "" {
		root["nickname"] = cfg.Nickname
	}

	// Build mode-specific settings
	modeSettings := make(map[string]interface{})

	// Audio (devices, virtual_mic, loopback, buffer_frames)
	audio := make(map[string]interface{})
	if len(cfg.Devices) > 0 {
		audio["devices"] = cfg.Devices
	}
	if cfg.VirtualMic != defaults.VirtualMic {
		audio["virtual_mic"] = cfg.VirtualMic
	}
	if cfg.Loopback != defaults.Loopback {
		audio["loopback"] = cfg.Loopback
	}
	if cfg.AudioBufferFrames != defaults.AudioBufferFrames {
		audio["buffer_frames"] = cfg.AudioBufferFrames
	}
	if len(audio) > 0 {
		modeSettings["audio"] = audio
	}

	// Connection
	conn := make(map[string]interface{})
	if cfg.AutoReconnect != defaults.AutoReconnect {
		conn["auto_reconnect"] = cfg.AutoReconnect
	}
	if cfg.AutoReconnectAttempts != defaults.AutoReconnectAttempts {
		conn["auto_reconnect_attempts"] = cfg.AutoReconnectAttempts
	}
	if cfg.MaxReconnectAttempts != defaults.MaxReconnectAttempts {
		conn["max_reconnect_attempts"] = cfg.MaxReconnectAttempts
	}
	if cfg.ReconnectIntervalSec != defaults.ReconnectIntervalSec {
		conn["reconnect_interval_sec"] = cfg.ReconnectIntervalSec
	}
	if len(conn) > 0 {
		modeSettings["connection"] = conn
	}

	// AEC
	if cfg.AEC != defaults.AEC {
		modeSettings["aec"] = cfg.AEC
	}

	// Logging
	log := make(map[string]interface{})
	if cfg.LogLevel != defaults.LogLevel {
		log["level"] = cfg.LogLevel
	}
	if cfg.LogToFile != defaults.LogToFile {
		log["to_file"] = cfg.LogToFile
	}
	if cfg.LogFile != defaults.LogFile {
		log["file"] = cfg.LogFile
	}
	if len(log) > 0 {
		modeSettings["logging"] = log
	}

	// Performance
	perf := make(map[string]interface{})
	if cfg.NoSIMDOptimization != defaults.NoSIMDOptimization {
		perf["no_simd"] = cfg.NoSIMDOptimization
	}
	if cfg.NoPoolWarmup != defaults.NoPoolWarmup {
		perf["no_pool_warmup"] = cfg.NoPoolWarmup
	}
	if len(perf) > 0 {
		modeSettings["performance"] = perf
	}

	// Security (TLS insecure only — cert/key/enabled are server-dictated)
	sec := make(map[string]interface{})
	if cfg.TLSInsecure != defaults.TLSInsecure {
		tls := map[string]interface{}{"insecure": cfg.TLSInsecure}
		sec["tls"] = tls
	}
	if len(sec) > 0 {
		modeSettings["security"] = sec
	}

	// Merge modes map with existing
	existingModes, _ := existing["modes"].(map[string]interface{})
	if existingModes == nil {
		existingModes = make(map[string]interface{})
	}

	modeKey := string(cfg.GetAudioMode())
	if len(modeSettings) > 0 {
		existingModes[modeKey] = modeSettings
	} else {
		// Even if no non-default settings, mark the mode as used (empty entry)
		existingModes[modeKey] = map[string]interface{}{}
	}

	if len(existingModes) > 0 {
		root["modes"] = existingModes
	}

	return root
}

// clientProbedFields are fields that the server dictates at connection time.
// These are excluded from client config saves.
var clientProbedFields = map[string]bool{
	"sample_rate":      true,
	"channels":         true,
	"opus_bitrate":     true,
	"opus_complexity":  true,
	"opus_application": true,
	"opus_dtx":         true,
	"opus_fec":         true,
	"max_clients":      true,
	"mode":             true,
	"server_muted":     true,
}

// diffConfig builds a nested YAML map containing only fields that differ from defaults.
func diffConfig(cfg, defaults Config, mode Mode) map[string]interface{} {
	root := make(map[string]interface{})
	isClient := mode == ModeClient

	// Role is always included (server/client).
	root["role"] = string(mode)

	// Helper to check if a field is client-probed (and thus excluded for client).
	probed := func(name string) bool {
		return isClient && clientProbedFields[name]
	}

	// ── Audio mode ──
	audioMode := cfg.GetAudioMode()
	if !probed("mode") && audioMode != AudioModeNormal {
		root["mode"] = string(audioMode)
	}
	if !probed("server_muted") && cfg.ServerMuted != defaults.ServerMuted {
		root["server_muted"] = cfg.ServerMuted
	}

	// ── Network ──
	net := make(map[string]interface{})
	if cfg.Port != defaults.Port {
		net["port"] = cfg.Port
	}
	if isClient {
		// Address always included for client.
		net["address"] = cfg.Address
	} else if cfg.Address != defaults.Address {
		net["address"] = cfg.Address
	}
	if !sliceEqual(cfg.STUNServers, defaults.STUNServers) {
		net["stun_servers"] = cfg.STUNServers
	}
	if len(cfg.TURNServers) > 0 {
		net["turn_servers"] = cfg.TURNServers
	}
	if len(net) > 0 {
		root["network"] = net
	}

	// ── Audio ──
	audio := make(map[string]interface{})
	if !probed("sample_rate") && cfg.SampleRate != defaults.SampleRate {
		audio["sample_rate"] = cfg.SampleRate
	}
	if !probed("channels") && cfg.Channels != defaults.Channels {
		audio["channels"] = cfg.Channels
	}
	if cfg.VirtualMic != defaults.VirtualMic {
		audio["virtual_mic"] = cfg.VirtualMic
	}
	if cfg.Loopback != defaults.Loopback {
		audio["loopback"] = cfg.Loopback
	}
	if cfg.AudioBufferFrames != defaults.AudioBufferFrames {
		audio["buffer_frames"] = cfg.AudioBufferFrames
	}
	// Devices are always included if any are present.
	if len(cfg.Devices) > 0 {
		audio["devices"] = cfg.Devices
	}
	if len(audio) > 0 {
		root["audio"] = audio
	}

	// ── Opus ──
	if !isClient {
		opus := make(map[string]interface{})
		if !probed("opus_bitrate") && cfg.OpusBitrate != defaults.OpusBitrate {
			opus["bitrate"] = cfg.OpusBitrate
		}
		if !probed("opus_complexity") && cfg.OpusComplexity != defaults.OpusComplexity {
			opus["complexity"] = cfg.OpusComplexity
		}
		if !probed("opus_application") && cfg.OpusApplication != defaults.OpusApplication {
			opus["application"] = cfg.OpusApplication
		}
		if !probed("opus_dtx") && cfg.OpusDTX != defaults.OpusDTX {
			opus["dtx"] = cfg.OpusDTX
		}
		if !probed("opus_fec") && cfg.OpusFEC != defaults.OpusFEC {
			opus["fec"] = cfg.OpusFEC
		}
		if len(opus) > 0 {
			root["opus"] = opus
		}
	}

	// ── Security ──
	sec := make(map[string]interface{})
	if cfg.Password != defaults.Password {
		sec["password"] = cfg.Password
	}
	if cfg.PasswordHash != defaults.PasswordHash {
		sec["password_hash"] = cfg.PasswordHash
	}
	tls := make(map[string]interface{})
	if cfg.TLSCert != defaults.TLSCert {
		tls["cert"] = cfg.TLSCert
	}
	if cfg.TLSKey != defaults.TLSKey {
		tls["key"] = cfg.TLSKey
	}
	if cfg.TLS != defaults.TLS {
		tls["enabled"] = cfg.TLS
	}
	if cfg.TLSInsecure != defaults.TLSInsecure {
		tls["insecure"] = cfg.TLSInsecure
	}
	if len(tls) > 0 {
		sec["tls"] = tls
	}
	if cfg.MaxFailedAttempts != defaults.MaxFailedAttempts {
		sec["max_auth_failures"] = cfg.MaxFailedAttempts
	}
	if cfg.BanFilePath != defaults.BanFilePath {
		sec["ban_file"] = cfg.BanFilePath
	}
	if len(cfg.TrustedProxies) > 0 {
		sec["trusted_proxies"] = cfg.TrustedProxies
	}
	if len(sec) > 0 {
		root["security"] = sec
	}

	// ── Connection ──
	conn := make(map[string]interface{})
	if !probed("max_clients") && cfg.MaxClients != defaults.MaxClients {
		conn["max_clients"] = cfg.MaxClients
	}
	if cfg.MaxReconnectAttempts != defaults.MaxReconnectAttempts {
		conn["max_reconnect_attempts"] = cfg.MaxReconnectAttempts
	}
	if cfg.ReconnectIntervalSec != defaults.ReconnectIntervalSec {
		conn["reconnect_interval_sec"] = cfg.ReconnectIntervalSec
	}
	if cfg.AutoReconnect != defaults.AutoReconnect {
		conn["auto_reconnect"] = cfg.AutoReconnect
	}
	if cfg.AutoReconnectAttempts != defaults.AutoReconnectAttempts {
		conn["auto_reconnect_attempts"] = cfg.AutoReconnectAttempts
	}
	if len(conn) > 0 {
		root["connection"] = conn
	}

	// ── Logging ──
	log := make(map[string]interface{})
	if cfg.LogLevel != defaults.LogLevel {
		log["level"] = cfg.LogLevel
	}
	if cfg.LogToFile != defaults.LogToFile {
		log["to_file"] = cfg.LogToFile
	}
	if cfg.LogFile != defaults.LogFile {
		log["file"] = cfg.LogFile
	}
	if len(log) > 0 {
		root["logging"] = log
	}

	// ── Performance ──
	perf := make(map[string]interface{})
	if cfg.NoSIMDOptimization != defaults.NoSIMDOptimization {
		perf["no_simd"] = cfg.NoSIMDOptimization
	}
	if cfg.NoPoolWarmup != defaults.NoPoolWarmup {
		perf["no_pool_warmup"] = cfg.NoPoolWarmup
	}
	if len(perf) > 0 {
		root["performance"] = perf
	}

	// ── Other top-level fields ──
	if cfg.Nickname != defaults.Nickname {
		root["nickname"] = cfg.Nickname
	}
	if cfg.HWIDRequired != defaults.HWIDRequired {
		root["hwid_required"] = cfg.HWIDRequired
	}
	if cfg.AEC != defaults.AEC {
		root["aec"] = cfg.AEC
	}
	if cfg.RecordMode != defaults.RecordMode {
		root["record_mode"] = cfg.RecordMode
	}

	return root
}

// ConfigInfoLine returns a short summary string for display in config lists.
func ConfigInfoLine(cfg Config, mode Mode) string {
	defaults := DefaultConfig()
	var parts []string

	if mode == ModeServer {
		if cfg.Port != defaults.Port {
			parts = append(parts, fmt.Sprintf("port: %d", cfg.Port))
		}
		parts = append(parts, fmt.Sprintf("devices: %d", len(cfg.Devices)))
		if am := cfg.GetAudioMode(); am != AudioModeNormal {
			parts = append(parts, string(am))
		}
		if cfg.AEC {
			parts = append(parts, "aec")
		}
		if cfg.TLS {
			parts = append(parts, "tls")
		}
	} else {
		// Client mode.
		if cfg.Address == "" {
			// Profile
			parts = append(parts, "📋 profile")
			if cfg.Nickname != "" {
				parts = append(parts, fmt.Sprintf("nick: %s", cfg.Nickname))
			}
		} else {
			// Config with address
			parts = append(parts, fmt.Sprintf("📡 %s:%d", cfg.Address, cfg.Port))
			if cfg.Password != "" {
				parts = append(parts, "🔑")
			}
			if cfg.AutoReconnect {
				parts = append(parts, "🔄")
			}
		}

		// Mode info from ClientModes
		modeNames := make([]string, 0, len(cfg.ClientModes))
		for k := range cfg.ClientModes {
			modeNames = append(modeNames, k)
		}
		sort.Strings(modeNames)

		if len(modeNames) == 1 {
			modeName := modeNames[0]
			captureCount, playbackCount := countDevicesInMode(cfg.ClientModes[modeName])
			parts = append(parts, modeName)
			if captureCount > 0 || playbackCount > 0 {
				parts = append(parts, fmt.Sprintf("🎤%d", captureCount), fmt.Sprintf("🔊%d", playbackCount))
			}
		} else if len(modeNames) > 1 {
			parts = append(parts, "modes: "+strings.Join(modeNames, ", "))
		}
	}

	return strings.Join(parts, " | ")
}

// ListConfigs reads saved configs for the given mode and returns them sorted
// by modification time (most recent first).
func ListConfigs(mode Mode) ([]ConfigEntry, error) {
	dir := ConfigsDir(mode)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read configs directory: %w", err)
	}

	var result []ConfigEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(e.Name(), ext)
		p := filepath.Join(dir, e.Name())

		// Load to generate info line.
		cfg, loadErr := LoadFromFile(p)
		infoLine := ""
		if loadErr == nil {
			infoLine = ConfigInfoLine(cfg, mode)
		}

		result = append(result, ConfigEntry{
			Name:     name,
			Path:     p,
			ModTime:  info.ModTime(),
			InfoLine: infoLine,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.After(result[j].ModTime)
	})

	return result, nil
}

// DeleteConfig removes a saved config file for the given mode and name.
// Tries .yaml first, then .yml for backward compatibility.
func DeleteConfig(mode Mode, name string) error {
	dir := ConfigsDir(mode)
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to delete config %q: %w", name, err)
			}
			return nil
		}
	}
	return fmt.Errorf("config %q not found", name)
}

// IsProfile returns true if the config represents a client profile (no address).
func IsProfile(cfg Config) bool {
	return cfg.Mode == ModeClient && cfg.Address == ""
}

// sanitizeFilename replaces characters unsafe for filenames with underscores.
// Dots are kept (valid in filenames). ".." is replaced with "_" for security.
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "..", "_")
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	s = replacer.Replace(s)
	s = strings.Trim(s, "_")
	return s
}

// DefaultPlaceholder returns the default filename (without extension) for saving a config.
func DefaultPlaceholder(cfg Config) string {
	if cfg.Address != "" {
		return sanitizeFilename(cfg.Address + "_" + strconv.Itoa(cfg.Port))
	}
	if IsProfile(cfg) {
		return "profile"
	}
	return "config"
}

// SaveProfile saves a client profile (without address/port/password) to a YAML file.
// Like SaveNonDefault but uses diffProfileConfig to exclude base connection fields.
func SaveProfile(cfg Config, path string) error {
	m := diffProfileConfig(cfg, DefaultConfig(), path)

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal profile config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	perm := os.FileMode(0600)
	if runtime.GOOS == "windows" {
		perm = 0666
	}

	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}

// diffProfileConfig builds a client profile map — like diffClientConfig but WITHOUT
// address, port, password at root level. Keeps role, nickname, and modes.
func diffProfileConfig(cfg, defaults Config, path string) map[string]interface{} {
	// Read existing file for merge
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, &existing)
	}

	root := make(map[string]interface{})
	root["role"] = string(ModeClient)

	// Nickname (no address/port/password)
	if cfg.Nickname != "" {
		root["nickname"] = cfg.Nickname
	}

	// Build mode-specific settings (same as diffClientConfig)
	modeSettings := make(map[string]interface{})

	// Audio
	audio := make(map[string]interface{})
	if len(cfg.Devices) > 0 {
		audio["devices"] = cfg.Devices
	}
	if cfg.VirtualMic != defaults.VirtualMic {
		audio["virtual_mic"] = cfg.VirtualMic
	}
	if cfg.Loopback != defaults.Loopback {
		audio["loopback"] = cfg.Loopback
	}
	if cfg.AudioBufferFrames != defaults.AudioBufferFrames {
		audio["buffer_frames"] = cfg.AudioBufferFrames
	}
	if len(audio) > 0 {
		modeSettings["audio"] = audio
	}

	// Connection
	conn := make(map[string]interface{})
	if cfg.AutoReconnect != defaults.AutoReconnect {
		conn["auto_reconnect"] = cfg.AutoReconnect
	}
	if cfg.AutoReconnectAttempts != defaults.AutoReconnectAttempts {
		conn["auto_reconnect_attempts"] = cfg.AutoReconnectAttempts
	}
	if cfg.MaxReconnectAttempts != defaults.MaxReconnectAttempts {
		conn["max_reconnect_attempts"] = cfg.MaxReconnectAttempts
	}
	if cfg.ReconnectIntervalSec != defaults.ReconnectIntervalSec {
		conn["reconnect_interval_sec"] = cfg.ReconnectIntervalSec
	}
	if len(conn) > 0 {
		modeSettings["connection"] = conn
	}

	// AEC
	if cfg.AEC != defaults.AEC {
		modeSettings["aec"] = cfg.AEC
	}

	// Logging
	log := make(map[string]interface{})
	if cfg.LogLevel != defaults.LogLevel {
		log["level"] = cfg.LogLevel
	}
	if cfg.LogToFile != defaults.LogToFile {
		log["to_file"] = cfg.LogToFile
	}
	if cfg.LogFile != defaults.LogFile {
		log["file"] = cfg.LogFile
	}
	if len(log) > 0 {
		modeSettings["logging"] = log
	}

	// Performance
	perf := make(map[string]interface{})
	if cfg.NoSIMDOptimization != defaults.NoSIMDOptimization {
		perf["no_simd"] = cfg.NoSIMDOptimization
	}
	if cfg.NoPoolWarmup != defaults.NoPoolWarmup {
		perf["no_pool_warmup"] = cfg.NoPoolWarmup
	}
	if len(perf) > 0 {
		modeSettings["performance"] = perf
	}

	// Security (TLS insecure only)
	sec := make(map[string]interface{})
	if cfg.TLSInsecure != defaults.TLSInsecure {
		tls := map[string]interface{}{"insecure": cfg.TLSInsecure}
		sec["tls"] = tls
	}
	if len(sec) > 0 {
		modeSettings["security"] = sec
	}

	// Merge modes map with existing
	existingModes, _ := existing["modes"].(map[string]interface{})
	if existingModes == nil {
		existingModes = make(map[string]interface{})
	}

	modeKey := string(cfg.GetAudioMode())
	if len(modeSettings) > 0 {
		existingModes[modeKey] = modeSettings
	} else {
		existingModes[modeKey] = map[string]interface{}{}
	}

	if len(existingModes) > 0 {
		root["modes"] = existingModes
	}

	return root
}

// countDevicesInMode counts capture and playback devices in a mode's data map.
// It looks for audio.devices ([]interface{} of maps with "role" key).
func countDevicesInMode(modeData interface{}) (capture, playback int) {
	m, ok := modeData.(map[string]interface{})
	if !ok {
		return 0, 0
	}
	audioRaw, ok := m["audio"]
	if !ok {
		return 0, 0
	}
	audioMap, ok := audioRaw.(map[string]interface{})
	if !ok {
		return 0, 0
	}
	devicesRaw, ok := audioMap["devices"]
	if !ok {
		return 0, 0
	}
	devices, ok := devicesRaw.([]interface{})
	if !ok {
		return 0, 0
	}
	for _, d := range devices {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := dm["role"].(string)
		switch role {
		case "capture":
			capture++
		case "playback":
			playback++
		}
	}
	return capture, playback
}

// sliceEqual compares two string slices for equality.
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
