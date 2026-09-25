// Package startup implements readiness policy without opening audio devices or connections.
package startup

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// Intent carries non-persistent startup intent separately from audio configuration.
type Intent struct {
	Requested        bool
	Recent           *recent.Server
	Problem          string
	Discover         bool
	DiscoveryTimeout time.Duration
	DeviceName       string
	ExplicitFlags    []string
}

// Prepare returns an independently editable device configuration, or a readiness error.
// A non-nil info must represent a successful probe of cfg.Address and cfg.Port.
// On error, the returned configuration contains any safely applied probe settings.
func Prepare(cfg config.Config, inventory []audio.AudioDevice, saved *recent.Server, info *probe.ProbeServerResult) (config.Config, error) {
	return prepareWithOverrides(cfg, inventory, saved, info, nil)
}

func prepareWithOverrides(cfg config.Config, inventory []audio.AudioDevice, saved *recent.Server, info *probe.ProbeServerResult, flags []string) (config.Config, error) {
	original := cfg
	cfg = cloneConfig(cfg)
	if err := prepareMode(&cfg, saved, info); err != nil {
		return cfg, err
	}
	roles, err := requiredRoles(cfg)
	if err != nil {
		return cfg, err
	}
	if err := prepareDevices(&cfg, inventory, saved, roles); err != nil {
		return cfg, err
	}
	applyExplicitOptions(&cfg, original, flags)
	return cfg, errors.Join(cfg.Validate()...)
}

func prepareMode(cfg *config.Config, saved *recent.Server, info *probe.ProbeServerResult) error {
	if cfg.Mode == config.ModeClient {
		if err := prepareProbe(cfg, saved, info); err != nil {
			return err
		}
	}
	if cfg.StreamMode != "" {
		cfg.SyncFromStreamMode()
	}
	cfg.NormalizeConference()
	return nil
}

func prepareProbe(cfg *config.Config, saved *recent.Server, info *probe.ProbeServerResult) error {
	if strings.TrimSpace(cfg.Address) == "" {
		return errors.New("server address is required")
	}
	if info == nil {
		return errors.New("a successful server probe is required")
	}
	if err := checkSavedServer(saved, info); err != nil {
		return err
	}
	return probe.ApplyProbeToConfig(cfg, info)
}

func checkSavedServer(saved *recent.Server, info *probe.ProbeServerResult) error {
	if saved == nil {
		return nil
	}
	if saved.ServerID != "" && saved.ServerID != info.ServerID {
		return errors.New("server identity changed; review the saved connection")
	}
	mode := effectiveProbeMode(info)
	if saved.LastMode != "" && saved.LastMode != mode {
		return fmt.Errorf("server mode changed from %q to %q; review the saved connection", saved.LastMode, mode)
	}
	return nil
}

func effectiveProbeMode(info *probe.ProbeServerResult) string {
	if info.Mode == "" {
		return string(config.AudioModeNormal)
	}
	return info.Mode
}

func requiredRoles(cfg config.Config) ([]config.DeviceRole, error) {
	if cfg.Mode != config.ModeClient && cfg.Mode != config.ModeServer {
		return nil, fmt.Errorf("invalid startup role %q", cfg.Mode)
	}
	switch cfg.GetAudioMode() {
	case config.AudioModeDuplex:
		return []config.DeviceRole{config.RoleCapture, config.RolePlayback}, nil
	case config.AudioModeConference:
		return conferenceRoles(cfg), nil
	case config.AudioModeNormal, config.AudioModeReverse:
		return []config.DeviceRole{primaryRole(cfg)}, nil
	default:
		return nil, fmt.Errorf("unsupported audio mode %q", cfg.GetAudioMode())
	}
}

func conferenceRoles(cfg config.Config) []config.DeviceRole {
	if cfg.Mode == config.ModeClient {
		return []config.DeviceRole{config.RoleCapture, config.RolePlayback}
	}
	if cfg.ServerMuted {
		return nil
	}
	return []config.DeviceRole{config.RoleCapture}
}

func primaryRole(cfg config.Config) config.DeviceRole {
	if cfg.Mode == config.ModeServer {
		if cfg.GetAudioMode() == config.AudioModeNormal || cfg.GetAudioMode() == config.AudioModeConference {
			return config.RoleCapture
		}
	} else if cfg.GetAudioMode() == config.AudioModeReverse {
		return config.RoleCapture
	}
	return config.RolePlayback
}
