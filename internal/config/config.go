// Package config provides configuration management for EchoWarp.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// Mode represents the operational mode of EchoWarp.
type Mode = echowarp.Mode

// Mode constants define whether EchoWarp acts as server or client.
const (
	// ModeServer configures EchoWarp to accept incoming connections.
	ModeServer Mode = echowarp.ModeServer
	// ModeClient configures EchoWarp to connect to a remote server.
	ModeClient Mode = echowarp.ModeClient
)

// AudioMode represents the audio streaming mode (normal/reverse/duplex/conference).
type AudioMode = echowarp.AudioMode

const (
	AudioModeNormal     = echowarp.AudioModeNormal
	AudioModeReverse    = echowarp.AudioModeReverse
	AudioModeDuplex     = echowarp.AudioModeDuplex
	AudioModeConference = echowarp.AudioModeConference
)

// DeviceRole specifies whether a device is used for capture (sending) or playback (receiving).
type DeviceRole = echowarp.DeviceRole

const (
	// RoleCapture means the device captures audio for sending.
	RoleCapture = echowarp.RoleCapture
	// RolePlayback means the device plays back received audio.
	RolePlayback = echowarp.RolePlayback
)

// DeviceType indicates whether a device is an input (microphone) or output (speaker).
type DeviceType = echowarp.DeviceType

const (
	// DeviceInput is a microphone / capture hardware device.
	DeviceInput = echowarp.DeviceInput
	// DeviceOutput is a speaker / playback hardware device.
	DeviceOutput = echowarp.DeviceOutput
)

// DeviceEntry represents a single audio device in a multi-device configuration.
type DeviceEntry = echowarp.DeviceEntry

// TURNServer holds credentials for a TURN relay server.
type TURNServer = transport.TURNServer

// Config holds all configuration parameters for EchoWarp.
// It can be loaded from YAML files, CLI flags, or built programmatically.
// Sensitive fields (Password, PasswordHash, TURN credentials) are redacted
// in SafeConfig() output for safe logging.
type Config struct {
	Mode           Mode      `yaml:"role"`
	StreamMode     AudioMode `yaml:"mode,omitempty"`
	Reverse        bool      `yaml:"-"`
	Duplex         bool      `yaml:"-"`
	Port           int       `yaml:"port"`
	Address        string    `yaml:"address"`
	DeviceID       *uint32   `yaml:"device_id"`
	InputDeviceID  *uint32   `yaml:"input_device_id"`
	OutputDeviceID *uint32   `yaml:"output_device_id"`

	// Devices is the multi-device configuration. When populated, it takes precedence
	// over DeviceID/InputDeviceID/OutputDeviceID. Role is required in duplex mode.
	Devices    []DeviceEntry `yaml:"devices,omitempty"`
	SampleRate uint32        `yaml:"sample_rate"`
	Channels   uint32        `yaml:"channels"`
	VirtualMic bool          `yaml:"virtual_mic"`
	Loopback   bool          `yaml:"loopback"`

	// Runtime-only fields for loopback capture (not persisted to YAML).
	LoopbackOutputDevice string `yaml:"-"`
	LoopbackBlackHole    string `yaml:"-"`

	OpusBitrate     int    `yaml:"opus_bitrate"`
	OpusComplexity  int    `yaml:"opus_complexity"`
	OpusApplication string `yaml:"opus_application"`
	OpusDTX         bool   `yaml:"opus_dtx"`
	OpusFEC         bool   `yaml:"opus_fec"`

	Password     string `yaml:"password,omitempty"`
	PasswordHash string `yaml:"password_hash,omitempty"`

	TLSCert       string `yaml:"tls_cert,omitempty"`
	TLSKey        string `yaml:"tls_key,omitempty"`
	TLS           bool   `yaml:"tls"`
	TLSInsecure   bool   `yaml:"tls_insecure"`
	TLSSelfSigned bool   `yaml:"-" json:"-"` // Runtime-only: true if loaded TLS cert is self-signed.

	STUNServers []string     `yaml:"stun_servers"`
	TURNServers []TURNServer `yaml:"turn_servers,omitempty"`

	MaxClients           int `yaml:"max_clients"`
	MaxReconnectAttempts int `yaml:"max_reconnect_attempts"`
	ReconnectIntervalSec int `yaml:"reconnect_interval_sec"`

	// AutoReconnect enables automatic reconnection after server graceful shutdown (ActionStop).
	// When false (default), client shows idle message and waits for manual ^R.
	AutoReconnect bool `yaml:"auto_reconnect" json:"auto_reconnect"`

	// AutoReconnectAttempts limits the number of auto-reconnect attempts after server shutdown.
	// 0 = infinite (default). Only relevant when AutoReconnect is true.
	AutoReconnectAttempts int `yaml:"auto_reconnect_attempts" json:"auto_reconnect_attempts"`

	MaxFailedAttempts int    `yaml:"max_failed_attempts"`
	BanFilePath       string `yaml:"ban_file_path"`

	LogLevel  string `yaml:"log_level"`
	LogToFile bool   `yaml:"log_to_file"`
	LogFile   string `yaml:"log_file"`

	// TrustedProxies defines IP addresses/CIDRs trusted to send X-Forwarded-For headers.
	// When empty (default), no proxies are trusted and X-Forwarded-For is ignored.
	// Use "localhost" to trust 127.0.0.1 and ::1 for backward compatibility.
	// Examples: ["10.0.0.0/8", "192.168.1.100"]
	TrustedProxies []string `yaml:"trusted_proxies"`

	// AEC enables acoustic echo cancellation in duplex mode.
	// When true, the NLMS adaptive filter removes speaker→mic echo.
	AEC bool `yaml:"aec"`

	// Conference enables conference mode (multi-participant mixing on server).
	// Mutually exclusive with Duplex and Reverse. Implies MaxClients > 1.
	Conference bool `yaml:"-"`

	// ServerMuted when true, the server does not contribute its own audio to the conference mix.
	ServerMuted bool `yaml:"server_muted"`

	// RecordMode starts recording immediately: "mix", "tracks", or "both". Empty = no recording.
	RecordMode string `yaml:"record_mode,omitempty"`

	// RecordDir overrides the default recording output directory.
	// When empty, recordings are stored in ~/Documents/EchoWarp_records.
	RecordDir string `yaml:"record_dir,omitempty"`

	// Nickname is the chat display name (client only). Empty = server assigns "Client-N".
	Nickname string `yaml:"nickname,omitempty" json:"nickname,omitempty"`

	// HWIDRequired when true, server requires clients to send a hardware identifier (for bans).
	// Server only, default false.
	HWIDRequired bool `yaml:"hwid_required" json:"hwid_required"`

	// RateLimit is the maximum number of connections per second per IP (0=disabled).
	RateLimit int `yaml:"rate_limit"`

	NoSIMDOptimization bool `yaml:"no_simd_optimization"`
	NoPoolWarmup       bool `yaml:"no_pool_warmup"`

	// ClientModes stores mode-specific settings loaded from client config files.
	// Key is the AudioMode string (e.g. "conference", "duplex", "normal").
	// Value is raw YAML map data that can be applied via ApplyClientModeData.
	// Only populated when loading client configs with modes: map.
	ClientModes map[string]interface{} `yaml:"-" json:"-"`

	// AudioBufferFrames controls the size of audio channel buffers (in frames).
	// Each frame is 20ms of audio. Higher values increase latency but improve
	// stability on jittery networks. Lower values reduce latency.
	// Default: 5 (~100ms). Higher values improve stability on jittery networks.
	AudioBufferFrames int `yaml:"audio_buffer_frames"`
}

// ── Nested YAML structs ──────────────────────────────────────────────────────

type nestedAudioConfig struct {
	DeviceID          *uint32       `yaml:"device_id"        json:"device_id,omitempty"`
	InputDeviceID     *uint32       `yaml:"input_device_id"  json:"input_device_id,omitempty"`
	OutputDeviceID    *uint32       `yaml:"output_device_id" json:"output_device_id,omitempty"`
	Devices           []DeviceEntry `yaml:"devices,omitempty" json:"devices,omitempty"`
	SampleRate        uint32        `yaml:"sample_rate"      json:"sample_rate"`
	Channels          uint32        `yaml:"channels"         json:"channels"`
	VirtualMic        bool          `yaml:"virtual_mic"      json:"virtual_mic"`
	Loopback          bool          `yaml:"loopback"         json:"loopback"`
	AudioBufferFrames int           `yaml:"buffer_frames"    json:"buffer_frames"`
}

type nestedOpusConfig struct {
	Bitrate     int    `yaml:"bitrate"      json:"bitrate"`
	Complexity  int    `yaml:"complexity"   json:"complexity"`
	Application string `yaml:"application"  json:"application"`
	DTX         bool   `yaml:"dtx"          json:"dtx"`
	FEC         bool   `yaml:"fec"          json:"fec"`
}

type nestedTLSConfig struct {
	Cert     string `yaml:"cert,omitempty"  json:"cert,omitempty"`
	Key      string `yaml:"key,omitempty"   json:"key,omitempty"`
	Enabled  bool   `yaml:"enabled"         json:"enabled"`
	Insecure bool   `yaml:"insecure"        json:"insecure"`
}

type nestedSecurityConfig struct {
	Password        string          `yaml:"password,omitempty"      json:"password,omitempty"`
	PasswordHash    string          `yaml:"password_hash,omitempty" json:"password_hash,omitempty"`
	TLS             nestedTLSConfig `yaml:"tls"                     json:"tls"`
	MaxAuthFailures int             `yaml:"max_auth_failures"       json:"max_auth_failures"`
	BanFile         string          `yaml:"ban_file,omitempty"      json:"ban_file,omitempty"`
	TrustedProxies  []string        `yaml:"trusted_proxies"         json:"trusted_proxies,omitempty"`
	RateLimit       int             `yaml:"rate_limit"              json:"rate_limit,omitempty"`
}

type nestedNetworkConfig struct {
	Port        int          `yaml:"port"                    json:"port"`
	Address     string       `yaml:"address"                 json:"address,omitempty"`
	STUNServers []string     `yaml:"stun_servers"            json:"stun_servers,omitempty"`
	TURNServers []TURNServer `yaml:"turn_servers,omitempty"  json:"turn_servers,omitempty"`
}

type nestedConnectionConfig struct {
	MaxClients            int  `yaml:"max_clients"              json:"max_clients"`
	MaxReconnectAttempts  int  `yaml:"max_reconnect_attempts"   json:"max_reconnect_attempts"`
	ReconnectIntervalSec  int  `yaml:"reconnect_interval_sec"   json:"reconnect_interval_sec"`
	AutoReconnect         bool `yaml:"auto_reconnect"           json:"auto_reconnect"`
	AutoReconnectAttempts int  `yaml:"auto_reconnect_attempts"  json:"auto_reconnect_attempts"`
}

type nestedLoggingConfig struct {
	Level  string `yaml:"level"         json:"level"`
	ToFile bool   `yaml:"to_file"       json:"to_file"`
	File   string `yaml:"file,omitempty" json:"file,omitempty"`
}

type nestedPerformanceConfig struct {
	NoSIMD       bool `yaml:"no_simd"        json:"no_simd"`
	NoPoolWarmup bool `yaml:"no_pool_warmup" json:"no_pool_warmup"`
}

// nestedConfig is the nested YAML/JSON representation of Config.
// It is used only for serialization/deserialization; internally Config is flat.
type nestedConfig struct {
	Role         Mode                    `yaml:"role"          json:"role"`
	StreamMode   AudioMode               `yaml:"mode"          json:"mode,omitempty"`
	ServerMuted  bool                    `yaml:"server_muted"  json:"server_muted"`
	Nickname     string                  `yaml:"nickname"      json:"nickname,omitempty"`
	HWIDRequired bool                    `yaml:"hwid_required" json:"hwid_required"`
	AEC          bool                    `yaml:"aec"           json:"aec"`
	RecordMode   string                  `yaml:"record_mode"   json:"record_mode,omitempty"`
	Audio        nestedAudioConfig       `yaml:"audio"         json:"audio"`
	Opus         nestedOpusConfig        `yaml:"opus"          json:"opus"`
	Network      nestedNetworkConfig     `yaml:"network"       json:"network"`
	Security     nestedSecurityConfig    `yaml:"security"      json:"security"`
	Connection   nestedConnectionConfig  `yaml:"connection"    json:"connection"`
	Logging      nestedLoggingConfig     `yaml:"logging"       json:"logging"`
	Performance  nestedPerformanceConfig `yaml:"performance"   json:"performance"`
}

// toNested converts a flat Config to the nested YAML representation.
func (c Config) toNested() nestedConfig {
	return nestedConfig{
		Role:         c.Mode,
		StreamMode:   c.GetAudioMode(),
		ServerMuted:  c.ServerMuted,
		Nickname:     c.Nickname,
		HWIDRequired: c.HWIDRequired,
		AEC:          c.AEC,
		RecordMode:   c.RecordMode,
		Audio: nestedAudioConfig{
			DeviceID:          c.DeviceID,
			InputDeviceID:     c.InputDeviceID,
			OutputDeviceID:    c.OutputDeviceID,
			Devices:           c.Devices,
			SampleRate:        c.SampleRate,
			Channels:          c.Channels,
			VirtualMic:        c.VirtualMic,
			Loopback:          c.Loopback,
			AudioBufferFrames: c.AudioBufferFrames,
		},
		Opus: nestedOpusConfig{
			Bitrate:     c.OpusBitrate,
			Complexity:  c.OpusComplexity,
			Application: c.OpusApplication,
			DTX:         c.OpusDTX,
			FEC:         c.OpusFEC,
		},
		Network: nestedNetworkConfig{
			Port:        c.Port,
			Address:     c.Address,
			STUNServers: c.STUNServers,
			TURNServers: c.TURNServers,
		},
		Security: nestedSecurityConfig{
			Password:     c.Password,
			PasswordHash: c.PasswordHash,
			TLS: nestedTLSConfig{
				Cert:     c.TLSCert,
				Key:      c.TLSKey,
				Enabled:  c.TLS,
				Insecure: c.TLSInsecure,
			},
			MaxAuthFailures: c.MaxFailedAttempts,
			BanFile:         c.BanFilePath,
			TrustedProxies:  c.TrustedProxies,
			RateLimit:       c.RateLimit,
		},
		Connection: nestedConnectionConfig{
			MaxClients:            c.MaxClients,
			MaxReconnectAttempts:  c.MaxReconnectAttempts,
			ReconnectIntervalSec:  c.ReconnectIntervalSec,
			AutoReconnect:         c.AutoReconnect,
			AutoReconnectAttempts: c.AutoReconnectAttempts,
		},
		Logging: nestedLoggingConfig{
			Level:  c.LogLevel,
			ToFile: c.LogToFile,
			File:   c.LogFile,
		},
		Performance: nestedPerformanceConfig{
			NoSIMD:       c.NoSIMDOptimization,
			NoPoolWarmup: c.NoPoolWarmup,
		},
	}
}

// fromNested converts a nested YAML representation back to a flat Config,
// merging into the provided base (typically DefaultConfig()).
func fromNested(base Config, n nestedConfig) Config {
	base.Mode = n.Role
	base.ServerMuted = n.ServerMuted
	base.Nickname = n.Nickname
	base.HWIDRequired = n.HWIDRequired
	base.AEC = n.AEC
	base.RecordMode = n.RecordMode
	if n.StreamMode != "" {
		base.StreamMode = n.StreamMode
		base.SyncFromStreamMode()
	}

	// Audio
	base.DeviceID = n.Audio.DeviceID
	base.InputDeviceID = n.Audio.InputDeviceID
	base.OutputDeviceID = n.Audio.OutputDeviceID
	base.Devices = n.Audio.Devices
	if n.Audio.SampleRate != 0 {
		base.SampleRate = n.Audio.SampleRate
	}
	if n.Audio.Channels != 0 {
		base.Channels = n.Audio.Channels
	}
	base.VirtualMic = n.Audio.VirtualMic
	base.Loopback = n.Audio.Loopback
	if n.Audio.AudioBufferFrames != 0 {
		base.AudioBufferFrames = n.Audio.AudioBufferFrames
	}

	// Opus
	if n.Opus.Bitrate != 0 {
		base.OpusBitrate = n.Opus.Bitrate
	}
	if n.Opus.Complexity != 0 {
		base.OpusComplexity = n.Opus.Complexity
	}
	if n.Opus.Application != "" {
		base.OpusApplication = n.Opus.Application
	}
	base.OpusDTX = n.Opus.DTX
	base.OpusFEC = n.Opus.FEC

	// Network
	if n.Network.Port != 0 {
		base.Port = n.Network.Port
	}
	base.Address = n.Network.Address
	if len(n.Network.STUNServers) > 0 {
		base.STUNServers = n.Network.STUNServers
	}
	base.TURNServers = n.Network.TURNServers

	// Security
	base.Password = n.Security.Password
	base.PasswordHash = n.Security.PasswordHash
	base.TLSCert = n.Security.TLS.Cert
	base.TLSKey = n.Security.TLS.Key
	base.TLS = n.Security.TLS.Enabled
	base.TLSInsecure = n.Security.TLS.Insecure
	if n.Security.MaxAuthFailures != 0 {
		base.MaxFailedAttempts = n.Security.MaxAuthFailures
	}
	base.BanFilePath = n.Security.BanFile
	base.TrustedProxies = n.Security.TrustedProxies
	base.RateLimit = n.Security.RateLimit

	// Connection
	if n.Connection.MaxClients != 0 {
		base.MaxClients = n.Connection.MaxClients
	}
	if n.Connection.MaxReconnectAttempts != 0 {
		base.MaxReconnectAttempts = n.Connection.MaxReconnectAttempts
	}
	if n.Connection.ReconnectIntervalSec != 0 {
		base.ReconnectIntervalSec = n.Connection.ReconnectIntervalSec
	}
	base.AutoReconnect = n.Connection.AutoReconnect
	if n.Connection.AutoReconnectAttempts != 0 {
		base.AutoReconnectAttempts = n.Connection.AutoReconnectAttempts
	}

	// Logging
	if n.Logging.Level != "" {
		base.LogLevel = n.Logging.Level
	}
	base.LogToFile = n.Logging.ToFile
	base.LogFile = n.Logging.File

	// Performance
	base.NoSIMDOptimization = n.Performance.NoSIMD
	base.NoPoolWarmup = n.Performance.NoPoolWarmup

	return base
}

// isNestedYAML returns true if the raw YAML bytes contain nested keys
// (i.e. any of the top-level section keys are present).
func isNestedYAML(data []byte) bool {
	// Quick probe: unmarshal into a map and check for section keys
	var probe map[string]interface{}
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return false
	}
	nestedSections := []string{"audio", "opus", "network", "security", "connection", "logging", "performance"}
	for _, sec := range nestedSections {
		if _, ok := probe[sec]; ok {
			return true
		}
	}
	return false
}

// ── DefaultConfig ─────────────────────────────────────────────────────────────

// DefaultConfig returns a Config with sensible defaults:
//   - Port: 4415
//   - Sample rate: 48000 Hz mono
//   - Opus: 64kbps, complexity 5, VOIP mode, DTX and FEC enabled
//   - STUN: Google and Cloudflare public servers
//   - MaxClients: 1 (single-client mode)
//   - MaxReconnectAttempts: 5
//   - LogLevel: info
func DefaultConfig() Config {
	return Config{
		Port:            4415,
		SampleRate:      48000,
		Channels:        2,
		OpusBitrate:     64000,
		OpusComplexity:  5,
		OpusApplication: "voip",
		OpusDTX:         true,
		OpusFEC:         true,
		STUNServers: []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun.cloudflare.com:3478",
		},
		MaxClients:           1,
		MaxReconnectAttempts: 5,
		ReconnectIntervalSec: 3,
		MaxFailedAttempts:    5,
		LogLevel:             "info",
		AudioBufferFrames:    0, // 0 = auto (mode-based default in EffectiveAudioBufferFrames)
	}
}

// ── IsTLSEnabled ─────────────────────────────────────────────────────────────

// EffectiveRecordDir returns the recording output directory.
// If RecordDir is set, it is returned as-is. Otherwise falls back to
// ~/Documents/EchoWarp_records.
func (c Config) EffectiveRecordDir() string {
	if c.RecordDir != "" {
		return c.RecordDir
	}
	homeDir, _ := os.UserHomeDir() //nolint:errcheck
	return filepath.Join(homeDir, "Documents", "EchoWarp_records")
}

// EffectiveAudioBufferFrames returns the mode-based jitter buffer target depth.
// If AudioBufferFrames was explicitly set (non-zero), it is used as an override.
// Otherwise, defaults are chosen per mode to balance latency vs. resilience:
//   - duplex/conference: 3 frames (60ms) — low latency for interactive audio
//   - normal/reverse:    5 frames (100ms) — more headroom for one-way streaming
func (c Config) EffectiveAudioBufferFrames() int {
	if c.AudioBufferFrames > 0 {
		return c.AudioBufferFrames
	}
	if c.Duplex || c.Conference {
		return 3
	}
	return 5
}

// GetAudioMode returns the current audio mode as AudioMode type.
// If StreamMode is set, returns it directly. Otherwise computes from legacy booleans.
func (c Config) GetAudioMode() AudioMode {
	if c.StreamMode != "" {
		return c.StreamMode
	}
	if c.Conference {
		return AudioModeConference
	}
	if c.Duplex {
		return AudioModeDuplex
	}
	if c.Reverse {
		return AudioModeReverse
	}
	return AudioModeNormal
}

// AudioMode returns a human-readable string for the current audio direction mode.
func (c Config) AudioMode() string {
	return string(c.GetAudioMode())
}

// SyncFromStreamMode populates Reverse/Duplex/Conference booleans from StreamMode.
func (c *Config) SyncFromStreamMode() {
	c.Reverse = c.StreamMode == AudioModeReverse
	c.Duplex = c.StreamMode == AudioModeDuplex
	c.Conference = c.StreamMode == AudioModeConference
}

// SyncToStreamMode populates StreamMode from Reverse/Duplex/Conference booleans.
func (c *Config) SyncToStreamMode() {
	c.StreamMode = c.GetAudioMode()
}

// NormalizeConference adjusts settings for conference mode.
// If Conference is true and MaxClients < 2, sets MaxClients to 8.
func (c *Config) NormalizeConference() {
	if c.Conference && c.MaxClients < 2 {
		c.MaxClients = 8
	}
}

// EffectiveDevices returns the resolved device list. If Devices is populated, it is returned
// as-is. Otherwise, legacy fields (DeviceID, InputDeviceID, OutputDeviceID) are migrated
// into DeviceEntry slice with appropriate roles based on mode.
func (c Config) EffectiveDevices() []DeviceEntry {
	if len(c.Devices) > 0 {
		return c.Devices
	}
	// Migration from legacy single-device fields
	var entries []DeviceEntry
	if c.Duplex {
		if c.InputDeviceID != nil {
			entries = append(entries, DeviceEntry{ID: *c.InputDeviceID, Role: RoleCapture, Volume: 1.0})
		}
		if c.OutputDeviceID != nil {
			entries = append(entries, DeviceEntry{ID: *c.OutputDeviceID, Role: RolePlayback, Volume: 1.0})
		}
	}
	if c.DeviceID != nil && len(entries) == 0 {
		entries = append(entries, DeviceEntry{ID: *c.DeviceID, Volume: 1.0})
	}
	return entries
}

// CaptureDevices returns only capture-role devices from EffectiveDevices.
// In non-duplex mode, all devices without an explicit role are treated as capture (normal)
// or playback (reverse).
func (c Config) CaptureDevices() []DeviceEntry {
	devices := c.EffectiveDevices()
	var result []DeviceEntry
	for _, d := range devices {
		if d.Role == RoleCapture || (d.Role == "" && !c.Reverse) {
			result = append(result, d)
		}
	}
	return result
}

// PlaybackDevices returns only playback-role devices from EffectiveDevices.
func (c Config) PlaybackDevices() []DeviceEntry {
	devices := c.EffectiveDevices()
	var result []DeviceEntry
	for _, d := range devices {
		if d.Role == RolePlayback || (d.Role == "" && c.Reverse) {
			result = append(result, d)
		}
	}
	return result
}

// NormalizeDeviceVolumes ensures all devices have a valid volume (default 1.0).
func (c *Config) NormalizeDeviceVolumes() {
	for i := range c.Devices {
		if c.Devices[i].Volume <= 0 {
			c.Devices[i].Volume = 1.0
		}
	}
}

// ApplyClientModeData applies mode-specific settings from a raw YAML map
// (as stored in ClientModes) onto the config. Fields not present in the mode
// data retain their current values.
func (c *Config) ApplyClientModeData(modeData interface{}) {
	if modeData == nil {
		return
	}
	data, err := yaml.Marshal(modeData)
	if err != nil {
		return
	}
	var n nestedConfig
	if err := yaml.Unmarshal(data, &n); err != nil {
		return
	}
	// Apply only non-zero/non-empty fields from the mode data
	if len(n.Audio.Devices) > 0 {
		c.Devices = n.Audio.Devices
	}
	if n.Audio.VirtualMic {
		c.VirtualMic = true
	}
	if n.Audio.Loopback {
		c.Loopback = true
	}
	if n.Audio.AudioBufferFrames != 0 {
		c.AudioBufferFrames = n.Audio.AudioBufferFrames
	}
	if n.AEC {
		c.AEC = true
	}
	if n.Connection.AutoReconnect {
		c.AutoReconnect = true
	}
	if n.Connection.AutoReconnectAttempts != 0 {
		c.AutoReconnectAttempts = n.Connection.AutoReconnectAttempts
	}
	if n.Connection.MaxReconnectAttempts != 0 {
		c.MaxReconnectAttempts = n.Connection.MaxReconnectAttempts
	}
	if n.Connection.ReconnectIntervalSec != 0 {
		c.ReconnectIntervalSec = n.Connection.ReconnectIntervalSec
	}
	if n.Logging.Level != "" {
		c.LogLevel = n.Logging.Level
	}
	if n.Logging.ToFile {
		c.LogToFile = true
	}
	if n.Logging.File != "" {
		c.LogFile = n.Logging.File
	}
	if n.Performance.NoSIMD {
		c.NoSIMDOptimization = true
	}
	if n.Performance.NoPoolWarmup {
		c.NoPoolWarmup = true
	}
	if n.Security.TLS.Insecure {
		c.TLSInsecure = true
	}
}

// IsTLSEnabled returns true if both TLS certificate and key paths are configured.
func (c Config) IsTLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}
