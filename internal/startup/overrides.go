package startup

import (
	"fmt"
	"slices"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// Legacy config entries may omit Role/Type. Resolve their direction after probe
// and enumeration before allowing explicit role flags to replace them.
func overrideLegacyDevices(cfg config.Config, inventory []audio.AudioDevice, flags []string) (config.Config, error) {
	capture, playback := slices.Contains(flags, "capture-device"), slices.Contains(flags, "playback-device")
	if !capture && !playback {
		return cfg, nil
	}
	roles, err := requiredRoles(cfg)
	if err != nil || len(roles) == 0 {
		return cfg, err
	}
	kept := make([]config.DeviceEntry, 0, len(cfg.Devices))
	for _, entry := range cfg.Devices {
		if entry.Role == "" && entry.Type == "" {
			if capture && playback {
				continue
			}
			role, err := entryRole(entry, inventory, roles)
			if err != nil {
				return cfg, fmt.Errorf("cannot safely apply device override: %w", err)
			}
			if role == config.RoleCapture && capture || role == config.RolePlayback && playback {
				continue
			}
		}
		kept = append(kept, entry)
	}
	cfg.Devices = kept
	return cfg, nil
}

// Mode-specific config is lower priority than explicitly supplied CLI values,
// including false and zero; server-authoritative audio format remains untouched.
func applyExplicitOptions(dst *config.Config, src config.Config, flags []string) {
	for _, flag := range flags {
		switch flag {
		case "log-level":
			dst.LogLevel = src.LogLevel
		case "log-file":
			dst.LogFile = src.LogFile
		case "max-reconnect":
			dst.MaxReconnectAttempts = src.MaxReconnectAttempts
		case "auto-reconnect":
			dst.AutoReconnect = src.AutoReconnect
		case "auto-reconnect-attempts":
			dst.AutoReconnectAttempts = src.AutoReconnectAttempts
		case "aec":
			dst.AEC = src.AEC
		case "loopback":
			dst.Loopback = src.Loopback
		case "virtual-mic":
			dst.VirtualMic = src.VirtualMic
		case "tls-insecure":
			dst.TLSInsecure = src.TLSInsecure
		case "no-simd-optimization":
			dst.NoSIMDOptimization = src.NoSIMDOptimization
		case "no-pool-warmup":
			dst.NoPoolWarmup = src.NoPoolWarmup
		}
	}
}
