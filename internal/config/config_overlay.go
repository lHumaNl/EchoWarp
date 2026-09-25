package config

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// OverridePresence reports explicit file/environment values, never inherited ones.
type OverridePresence struct {
	StreamMode     bool
	TLS            bool
	Devices        bool
	DeviceID       bool
	InputDeviceID  bool
	OutputDeviceID bool
}

// LoadOver overlays an explicit YAML/JSON file and ECHOWARP_* environment onto base.
// It does not discover files, apply defaults, validate the final configuration, or
// change base.Mode (the command role). Empty values are overrides, not omissions.
// Canonical flat environment names win nested environment aliases; either wins
// file values. Nested file keys win their legacy flat aliases.
func LoadOver(base Config, configPath string) (Config, OverridePresence, error) {
	values, err := readOverlayValues(configPath)
	if err != nil {
		return Config{}, OverridePresence{}, err
	}
	cfg := cloneOverlayBase(base)
	presence, err := applyOverlay(&cfg, values)
	if err != nil {
		return Config{}, OverridePresence{}, err
	}
	if presence.StreamMode {
		cfg.SyncFromStreamMode()
	}
	if err := applyDeviceSelectorPriority(&cfg, values); err != nil {
		return Config{}, OverridePresence{}, err
	}
	return cfg, presence, nil
}

// Config's YAML tags are the canonical flat keys. Only nested aliases need a
// separate table; typed decoding keeps newly added device metadata intact.
var overlayAliases = map[string]string{
	"port": "network.port", "address": "network.address",
	"stun_servers": "network.stun_servers", "turn_servers": "network.turn_servers",
	"device_id": "audio.device_id", "input_device_id": "audio.input_device_id",
	"output_device_id": "audio.output_device_id", "devices": "audio.devices",
	"sample_rate": "audio.sample_rate", "channels": "audio.channels",
	"virtual_mic": "audio.virtual_mic", "loopback": "audio.loopback",
	"audio_buffer_frames": "audio.buffer_frames", "aec": "audio.aec",
	"opus_bitrate": "opus.bitrate", "opus_complexity": "opus.complexity",
	"opus_application": "opus.application", "opus_dtx": "opus.dtx", "opus_fec": "opus.fec",
	"password": "security.password", "password_hash": "security.password_hash",
	"tls_cert": "security.tls.cert", "tls_key": "security.tls.key",
	"tls": "security.tls.enabled", "tls_insecure": "security.tls.insecure",
	"max_failed_attempts": "security.max_auth_failures", "ban_file_path": "security.ban_file",
	"trusted_proxies": "security.trusted_proxies", "rate_limit": "security.rate_limit",
	"max_clients": "connection.max_clients", "max_reconnect_attempts": "connection.max_reconnect_attempts",
	"reconnect_interval_sec": "connection.reconnect_interval_sec",
	"auto_reconnect":         "connection.auto_reconnect", "auto_reconnect_attempts": "connection.auto_reconnect_attempts",
	"log_level": "logging.level", "log_to_file": "logging.to_file", "log_file": "logging.file",
	"no_simd_optimization": "performance.no_simd", "no_pool_warmup": "performance.no_pool_warmup",
}

func applyOverlay(cfg *Config, values map[string]yaml.Node) (OverridePresence, error) {
	var presence OverridePresence
	target := reflect.ValueOf(cfg).Elem()
	for i := range target.NumField() {
		field := target.Type().Field(i)
		key, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if key == "-" || key == "role" || key == "" {
			continue
		}
		present, err := applyOverlayField(target.Field(i), key, values)
		if err != nil {
			return OverridePresence{}, err
		}
		markOverlayPresence(&presence, field.Name, present)
	}
	return presence, nil
}

func applyOverlayField(target reflect.Value, key string, values map[string]yaml.Node) (bool, error) {
	if value, ok := overlayEnv(key); ok {
		if err := decodeOverlayEnv(target, value); err != nil {
			return false, fmt.Errorf("invalid environment value for %s (expected %s)", key, target.Type())
		}
		return true, nil
	}
	nested := overlayAliases[key]
	node, ok := values[nested]
	ok = ok && nested != ""
	if !ok {
		node, ok = values[key]
	}
	if ok {
		if err := decodeOverlayField(target, &node); err != nil {
			return false, fmt.Errorf("invalid config value for %s (expected %s)", key, target.Type())
		}
	}
	return ok, nil
}

func overlayEnv(key string) (string, bool) {
	for _, candidate := range []string{key, overlayAliases[key]} {
		if candidate == "" {
			continue
		}
		name := "ECHOWARP_" + strings.ToUpper(strings.ReplaceAll(candidate, ".", "_"))
		if value, ok := os.LookupEnv(name); ok {
			return value, true
		}
	}
	return "", false
}

func markOverlayPresence(presence *OverridePresence, field string, present bool) {
	flag := reflect.ValueOf(presence).Elem().FieldByName(field)
	if flag.IsValid() {
		flag.SetBool(present)
	}
}

func cloneOverlayBase(base Config) Config {
	base.DeviceID = cloneOverlayID(base.DeviceID)
	base.InputDeviceID = cloneOverlayID(base.InputDeviceID)
	base.OutputDeviceID = cloneOverlayID(base.OutputDeviceID)
	base.Devices = slices.Clone(base.Devices)
	for i := range base.Devices {
		base.Devices[i].MixInputID = cloneOverlayID(base.Devices[i].MixInputID)
	}
	base.STUNServers = slices.Clone(base.STUNServers)
	base.TURNServers = slices.Clone(base.TURNServers)
	base.TrustedProxies = slices.Clone(base.TrustedProxies)
	return base
}

func cloneOverlayID(id *uint32) *uint32 {
	if id == nil {
		return nil
	}
	copyID := *id
	return &copyID
}
