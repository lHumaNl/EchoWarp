package probe

import (
	"fmt"

	"github.com/lHumaNl/echowarp/internal/config"
)

// ParamChange describes a single parameter difference between client config and server probe result.
type ParamChange struct {
	Field    string // "Mode", "TLS", "HWID", "Password", "SampleRate", etc.
	OldValue string
	NewValue string
	Critical bool
}

// configModeString returns the mode key string corresponding to config flags.
func configModeString(cfg config.Config) string {
	switch {
	case cfg.Conference:
		return "conference"
	case cfg.Duplex:
		return "duplex"
	case cfg.Reverse:
		return "reverse"
	default:
		return "normal"
	}
}

// CompareServerParams compares the current client config with a probe result
// and returns a list of parameter changes, marking each as critical or not.
func CompareServerParams(cfg config.Config, probe *ProbeServerResult) []ParamChange {
	var changes []ParamChange

	// Mode comparison (critical: any change).
	oldMode := configModeString(cfg)
	newMode := probe.Mode
	if newMode == "" {
		newMode = "normal"
	}
	if oldMode != newMode {
		changes = append(changes, ParamChange{
			Field:    "Mode",
			OldValue: oldMode,
			NewValue: newMode,
			Critical: true,
		})
	}

	// TLS comparison (critical: any change).
	oldTLS := cfg.TLS || cfg.TLSCert != "" || cfg.TLSKey != ""
	newTLS := probe.TLSRequired
	if oldTLS != newTLS {
		changes = append(changes, ParamChange{
			Field:    "TLS",
			OldValue: boolToOnOff(oldTLS),
			NewValue: boolToOnOff(newTLS),
			Critical: true,
		})
	}

	// HWID comparison.
	// off→on = critical; on→off = non-critical.
	oldHWID := cfg.HWIDRequired
	newHWID := probe.HWIDRequired
	if oldHWID != newHWID {
		changes = append(changes, ParamChange{
			Field:    "HWID",
			OldValue: boolToOnOff(oldHWID),
			NewValue: boolToOnOff(newHWID),
			Critical: !oldHWID && newHWID, // off→on is critical
		})
	}

	// Password comparison.
	// none→required = critical; required→none = critical.
	oldHasPassword := cfg.Password != ""
	newHasPassword := probe.PasswordRequired
	if oldHasPassword != newHasPassword {
		changes = append(changes, ParamChange{
			Field:    "Password",
			OldValue: passwordStatus(oldHasPassword),
			NewValue: passwordStatus(newHasPassword),
			Critical: true,
		})
	}

	// Non-critical: sample_rate.
	if probe.SampleRate > 0 && cfg.SampleRate != probe.SampleRate {
		changes = append(changes, ParamChange{
			Field:    "SampleRate",
			OldValue: fmt.Sprintf("%d", cfg.SampleRate),
			NewValue: fmt.Sprintf("%d", probe.SampleRate),
			Critical: false,
		})
	}

	// Non-critical: channels.
	if probe.Channels > 0 && cfg.Channels != probe.Channels {
		changes = append(changes, ParamChange{
			Field:    "Channels",
			OldValue: fmt.Sprintf("%d", cfg.Channels),
			NewValue: fmt.Sprintf("%d", probe.Channels),
			Critical: false,
		})
	}

	// Non-critical: opus_bitrate.
	if probe.OpusBitrate > 0 && cfg.OpusBitrate != probe.OpusBitrate {
		changes = append(changes, ParamChange{
			Field:    "OpusBitrate",
			OldValue: fmt.Sprintf("%d", cfg.OpusBitrate),
			NewValue: fmt.Sprintf("%d", probe.OpusBitrate),
			Critical: false,
		})
	}

	// Non-critical: max_clients.
	// Skip the change if the old value is the default (1) — the first mDNS probe
	// doesn't carry MaxClients so the client defaults to 1, causing a spurious
	// "1 -> N" log on every connect. Only report when the user had an explicit
	// non-default value that now differs.
	if probe.MaxClients > 0 && cfg.MaxClients != probe.MaxClients && cfg.MaxClients != 1 {
		changes = append(changes, ParamChange{
			Field:    "MaxClients",
			OldValue: fmt.Sprintf("%d", cfg.MaxClients),
			NewValue: fmt.Sprintf("%d", probe.MaxClients),
			Critical: false,
		})
	}

	return changes
}

func boolToOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func passwordStatus(hasPassword bool) string {
	if hasPassword {
		return "required"
	}
	return "none"
}
