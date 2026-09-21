package cli

import (
	"fmt"

	"github.com/lHumaNl/echowarp/internal/config"
)

type audioDeviceRequirements struct {
	capture  bool
	playback bool
}

type audioDeviceSelection struct {
	capture  *uint32
	playback *uint32
}

func prepareNonInteractiveAudioConfig(cfg *config.Config) error {
	requirements := nonInteractiveDeviceRequirements(*cfg)
	if !requirements.capture && !requirements.playback {
		return nil
	}

	selection := resolveAudioDeviceSelection(*cfg, requirements)
	if requirements.capture && selection.capture == nil && requirements.playback && selection.playback == nil {
		return fmt.Errorf("--no-interactive requires capture and playback devices for %s mode; use --capture-device and --playback-device", cfg.GetAudioMode())
	}
	if requirements.capture && selection.capture == nil {
		return fmt.Errorf("--no-interactive requires a capture device for %s mode; use --device or --capture-device", cfg.GetAudioMode())
	}
	if requirements.playback && selection.playback == nil {
		return fmt.Errorf("--no-interactive requires a playback device for %s mode; use --device or --playback-device", cfg.GetAudioMode())
	}

	normalizeNonInteractiveDeviceFields(cfg, selection)
	return nil
}

func nonInteractiveDeviceRequirements(cfg config.Config) audioDeviceRequirements {
	switch cfg.GetAudioMode() {
	case config.AudioModeReverse:
		if cfg.Mode == config.ModeServer {
			return audioDeviceRequirements{playback: true}
		}
		return audioDeviceRequirements{capture: true}
	case config.AudioModeDuplex:
		return audioDeviceRequirements{capture: true, playback: true}
	case config.AudioModeConference:
		if cfg.Mode == config.ModeServer {
			if cfg.ServerMuted {
				return audioDeviceRequirements{}
			}
			return audioDeviceRequirements{capture: true}
		}
		return audioDeviceRequirements{capture: true, playback: true}
	default:
		if cfg.Mode == config.ModeServer {
			return audioDeviceRequirements{capture: true}
		}
		return audioDeviceRequirements{playback: true}
	}
}

func resolveAudioDeviceSelection(cfg config.Config, requirements audioDeviceRequirements) audioDeviceSelection {
	if len(cfg.Devices) > 0 {
		return resolveExplicitDevices(cfg.Devices, requirements)
	}

	selection := audioDeviceSelection{
		capture:  cloneDeviceID(cfg.InputDeviceID),
		playback: cloneDeviceID(cfg.OutputDeviceID),
	}

	if cfg.DeviceID == nil {
		return selection
	}

	// A lone legacy DeviceID has always represented whichever device roles the
	// selected mode needs, including both directions in legacy duplex configs.
	if cfg.InputDeviceID == nil && cfg.OutputDeviceID == nil {
		if requirements.capture {
			selection.capture = cloneDeviceID(cfg.DeviceID)
		}
		if requirements.playback {
			selection.playback = cloneDeviceID(cfg.DeviceID)
		}
		return selection
	}

	// In a single-direction mode, keep DeviceID as a backward-compatible
	// fallback when the role-specific field for that direction is absent.
	if requirements.capture && !requirements.playback && selection.capture == nil {
		selection.capture = cloneDeviceID(cfg.DeviceID)
	}
	if requirements.playback && !requirements.capture && selection.playback == nil {
		selection.playback = cloneDeviceID(cfg.DeviceID)
	}
	return selection
}

func resolveExplicitDevices(devices []config.DeviceEntry, requirements audioDeviceRequirements) audioDeviceSelection {
	var selection audioDeviceSelection
	for i := range devices {
		deviceID := devices[i].ID
		switch devices[i].Role {
		case config.RoleCapture:
			if selection.capture == nil {
				selection.capture = &deviceID
			}
		case config.RolePlayback:
			if selection.playback == nil {
				selection.playback = &deviceID
			}
		default:
			if requirements.capture && !requirements.playback && selection.capture == nil {
				selection.capture = &deviceID
			}
			if requirements.playback && !requirements.capture && selection.playback == nil {
				selection.playback = &deviceID
			}
		}
	}
	return selection
}

func normalizeNonInteractiveDeviceFields(cfg *config.Config, selection audioDeviceSelection) {
	if selection.capture != nil {
		cfg.InputDeviceID = cloneDeviceID(selection.capture)
	}
	if selection.playback != nil {
		cfg.OutputDeviceID = cloneDeviceID(selection.playback)
	}

	// Conference does not set Duplex, so its legacy input/output fields are not
	// included by EffectiveDevices. Materialize role entries for the capture path.
	if cfg.GetAudioMode() == config.AudioModeConference && len(cfg.Devices) == 0 &&
		selection.capture != nil && selection.playback != nil && *selection.capture != *selection.playback {
		cfg.Devices = []config.DeviceEntry{
			{ID: *selection.capture, Role: config.RoleCapture, Volume: 1.0},
			{ID: *selection.playback, Role: config.RolePlayback, Volume: 1.0},
		}
	}

	primary := selection.capture
	if cfg.Mode == config.ModeClient && (cfg.GetAudioMode() == config.AudioModeNormal ||
		cfg.GetAudioMode() == config.AudioModeDuplex || cfg.GetAudioMode() == config.AudioModeConference) {
		primary = selection.playback
	}
	if cfg.Mode == config.ModeServer && (cfg.GetAudioMode() == config.AudioModeReverse ||
		cfg.GetAudioMode() == config.AudioModeDuplex) {
		primary = selection.playback
	}
	if primary != nil {
		cfg.DeviceID = cloneDeviceID(primary)
	}
}

func cloneDeviceID(id *uint32) *uint32 {
	if id == nil {
		return nil
	}
	cloned := *id
	return &cloned
}
