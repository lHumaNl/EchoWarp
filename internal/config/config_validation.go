package config

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Validate checks the configuration for errors. Returns a slice of errors,
// empty if valid. Validates:
//   - Port range (1-65535)
//   - Address required for client mode
//   - Sample rate (8000, 12000, 16000, 24000, or 48000)
//   - Channels (1 or 2)
//   - Log level (debug, info, warn, error)
//   - Path traversal in file paths
func (c Config) Validate() []error {
	var errs []error

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, NewInvalidConfigError("port", "must be 1-65535", c.Port))
	}

	if c.Mode == ModeClient && c.Address == "" {
		errs = append(errs, NewInvalidConfigError("address", "required for client mode", nil))
	}

	validSampleRates := map[uint32]bool{8000: true, 12000: true, 16000: true, 24000: true, 48000: true}
	if !validSampleRates[c.SampleRate] {
		errs = append(errs, NewInvalidConfigError("sample_rate", "must be one of 8000, 12000, 16000, 24000, 48000", c.SampleRate))
	}

	if c.Channels != 1 && c.Channels != 2 {
		errs = append(errs, NewInvalidConfigError("channels", "must be 1 or 2", c.Channels))
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		errs = append(errs, NewInvalidConfigError("log_level", "must be debug, info, warn, or error", c.LogLevel))
	}

	if c.Duplex && c.Reverse {
		errs = append(errs, NewInvalidConfigError("duplex+reverse", "duplex and reverse cannot be used together", nil))
	}
	if c.Conference && c.Duplex {
		errs = append(errs, NewInvalidConfigError("conference+duplex", "conference and duplex cannot be used together", nil))
	}
	if c.Conference && c.Reverse {
		errs = append(errs, NewInvalidConfigError("conference+reverse", "conference and reverse cannot be used together", nil))
	}

	// Validate Devices entries
	for i, d := range c.Devices {
		if d.Role != "" && d.Role != RoleCapture && d.Role != RolePlayback {
			errs = append(errs, NewInvalidConfigError(fmt.Sprintf("devices[%d].role", i), "must be \"capture\" or \"playback\"", d.Role))
		}
		if d.Volume < 0 || d.Volume > 2.0 {
			errs = append(errs, NewInvalidConfigError(fmt.Sprintf("devices[%d].volume", i), "must be 0.0-2.0", d.Volume))
		}
	}
	if c.Duplex && len(c.Devices) > 0 {
		hasCap, hasPlay := false, false
		for _, d := range c.Devices {
			if d.Role == RoleCapture {
				hasCap = true
			}
			if d.Role == RolePlayback {
				hasPlay = true
			}
		}
		if !hasCap {
			errs = append(errs, NewInvalidConfigError("devices", "duplex mode requires at least one capture device", nil))
		}
		if !hasPlay {
			errs = append(errs, NewInvalidConfigError("devices", "duplex mode requires at least one playback device", nil))
		}
	}

	if c.MaxReconnectAttempts < 0 {
		errs = append(errs, NewInvalidConfigError("max_reconnect_attempts", "must be >= 0", c.MaxReconnectAttempts))
	}

	if c.AudioBufferFrames < 0 || c.AudioBufferFrames > 200 {
		errs = append(errs, NewInvalidConfigError("audio_buffer_frames", "must be 0-200 (0 = auto)", c.AudioBufferFrames))
	}

	if c.MaxFailedAttempts < 0 {
		errs = append(errs, NewInvalidConfigError("max_failed_attempts", "must be >= 0", c.MaxFailedAttempts))
	}

	if err := validateFilePath(c.TLSCert, "tls_cert"); err != nil {
		errs = append(errs, err)
	}
	if err := validateFilePath(c.TLSKey, "tls_key"); err != nil {
		errs = append(errs, err)
	}
	if err := validateFilePath(c.BanFilePath, "ban_file_path"); err != nil {
		errs = append(errs, err)
	}

	// Validate trusted proxies format
	for _, proxy := range c.TrustedProxies {
		if err := validateTrustedProxy(proxy); err != nil {
			errs = append(errs, NewInvalidConfigError("trusted_proxies", fmt.Sprintf("invalid proxy %q: %v", proxy, err), proxy))
		}
	}

	return errs
}

func validateFilePath(path, fieldName string) error {
	if path == "" {
		return nil
	}

	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return NewInvalidConfigError(fieldName, "path traversal detected", path)
	}

	if !filepath.IsAbs(clean) {
		_, err := filepath.Abs(clean)
		if err != nil {
			return NewInvalidConfigError(fieldName, fmt.Sprintf("invalid path: %v", err), path)
		}
	}

	return nil
}

// validateTrustedProxy validates a single trusted proxy entry.
// Accepts IP addresses (e.g., "192.168.1.1"), CIDR notation (e.g., "10.0.0.0/8"),
// or the special value "localhost" which expands to 127.0.0.1 and ::1.
func validateTrustedProxy(proxy string) error {
	if proxy == "" {
		return fmt.Errorf("empty proxy value")
	}

	// Special case: "localhost" is allowed as a shorthand
	if proxy == "localhost" {
		return nil
	}

	// Try CIDR notation first
	if strings.Contains(proxy, "/") {
		_, _, err := net.ParseCIDR(proxy)
		if err != nil {
			return fmt.Errorf("invalid CIDR notation: %w", err)
		}
		return nil
	}

	// Try plain IP address
	ip := net.ParseIP(proxy)
	if ip == nil {
		return fmt.Errorf("invalid IP address")
	}

	return nil
}

// ── SafeConfig / SafeString ───────────────────────────────────────────────────

// SafeConfig returns a copy with sensitive fields redacted for safe logging.
// Password, PasswordHash, and TURN credentials are replaced with "***REDACTED***".
func (c Config) SafeConfig() Config {
	safe := c
	if safe.Password != "" {
		safe.Password = "***REDACTED***"
	}
	if safe.PasswordHash != "" {
		safe.PasswordHash = "***REDACTED***"
	}
	for i := range safe.TURNServers {
		if safe.TURNServers[i].Credential != "" {
			safe.TURNServers[i].Credential = "***REDACTED***"
		}
	}
	return safe
}

// SafeString returns the nested YAML representation of SafeConfig().
// Useful for logging configuration without exposing secrets.
func (c Config) SafeString() string {
	safe := c.SafeConfig()
	b, err := yaml.Marshal(safe.toNested())
	if err != nil {
		return "Config{...}"
	}
	return string(b)
}
