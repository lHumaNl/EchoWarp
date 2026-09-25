package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/recent"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func loadServerStartup(cmd *cobra.Command) (config.Config, startup.Intent, error) {
	cfg, err := loadConfig(cmd, config.ModeServer)
	if err != nil {
		return cfg, startup.Intent{}, err
	}
	intent, err := prepareStartupIntent(cmd, &cfg)
	return cfg, intent, err
}

func prepareRecentServerHeadless(cmd *cobra.Command, cfg *config.Config, intent startup.Intent) error {
	var devices []audio.AudioDevice
	var loopbacks map[string]audio.LoopbackDevice
	if !cfg.Conference || !cfg.ServerMuted {
		dm, err := audio.NewDeviceManager()
		if err != nil {
			return err
		}
		defer dm.Close() //nolint:errcheck
		devices, err = listAllDevices(dm)
		if err != nil {
			return err
		}
		devices, loopbacks = appendLoopbackDevices(dm, devices, cfg.Duplex || !cfg.Reverse)
	}
	prepared, _, err := startup.Resolve(context.Background(), *cfg, devices, intent, startup.Network{})
	if err != nil {
		return err
	}
	*cfg = prepared
	for _, device := range cfg.Devices {
		if loopback, ok := loopbacks[device.Name]; ok {
			cfg.Loopback = true
			cfg.LoopbackOutputDevice = loopback.OutputDevice.Name
			cfg.LoopbackBlackHole = loopback.BlackHole.Name
		}
	}
	return validateAndSaveConfig(cmd, cfg)
}

func prepareServerRecent(cmd *cobra.Command, cfg *config.Config, intent startup.Intent) (startup.Intent, error) {
	intent.Requested = true
	path, _ := cmd.Flags().GetString("config")
	selectors, presence, err := config.LoadOver(config.Config{Mode: config.ModeServer}, path)
	if err != nil {
		return intent, err
	}
	profiles := preset.Load()
	mode := profiles.LastMode
	if presence.StreamMode {
		mode = string(selectors.StreamMode)
	}
	mode = serverModeFlag(cmd, mode)
	if !validServerMode(mode) {
		intent.Problem = "No valid saved server mode; choose --mode and configure a profile"
		return intent, nil
	}
	mp := profiles.Get(mode)
	if mp == nil {
		intent.Problem = fmt.Sprintf("No saved server profile for %s", mode)
		return intent, nil
	}
	if mp.Version > 1 {
		intent.Problem = "Server profile was written by a newer version"
		return intent, nil
	}
	base, err := loadServerBase()
	if err != nil {
		return intent, err
	}
	merged := preset.Apply(base, mode, *mp)
	merged, presence, err = config.LoadOver(merged, path)
	if err != nil {
		return intent, err
	}
	// An explicit TLS=false is an intentional override, never a downgrade on error.
	if presence.TLS && !merged.TLS {
		merged.TLSCert, merged.TLSKey = "", ""
	}
	applyFlagOverrides(cmd, &merged)
	merged.StreamMode = config.AudioMode(mode)
	merged.SyncFromStreamMode()
	merged.NormalizeConference()
	if cmd.Flags().Changed("tls-cert") || cmd.Flags().Changed("tls-key") {
		merged.TLS = merged.TLSCert != "" || merged.TLSKey != ""
	} else if !presence.TLS && (merged.TLSCert != "" || merged.TLSKey != "") {
		merged.TLS = true
	}
	saved := recent.DevicePreset{Devices: append([]recent.PresetDevice(nil), mp.Devices...), VirtualSinks: mp.VirtualSinks}
	if presence.Devices && len(merged.Devices) == 0 {
		saved = recent.DevicePreset{} // Explicit [] means no implicit saved selection.
	} else {
		kept := make([]recent.PresetDevice, 0, len(saved.Devices))
		captureIDs, _ := cmd.Flags().GetUintSlice("capture-device")
		playbackIDs, _ := cmd.Flags().GetUintSlice("playback-device")
		clearCapture := cmd.Flags().Changed("capture-device") && len(captureIDs) == 0
		clearPlayback := cmd.Flags().Changed("playback-device") && len(playbackIDs) == 0
		for _, device := range saved.Devices {
			if presence.DeviceID || (device.IsInput && (presence.InputDeviceID || clearCapture)) || (!device.IsInput && (presence.OutputDeviceID || clearPlayback)) {
				continue
			}
			kept = append(kept, device)
		}
		saved.Devices = kept
	}
	// Reuse safe named-device restoration; no credentials go into client history.
	intent.Recent = &recent.Server{Presets: map[string]recent.DevicePreset{mode: saved}}
	*cfg = merged
	return intent, nil
}

func loadServerBase() (config.Config, error) {
	base := config.DefaultConfig()
	base.Mode = config.ModeServer
	base.RateLimit = config.DefaultServerRateLimit
	for _, name := range []string{"config.yaml", "config.yml"} {
		path := filepath.Join(config.EchoWarpDir(), name)
		if _, err := os.Stat(path); err == nil {
			loaded, _, loadErr := config.LoadOver(base, path)
			return loaded, loadErr
		} else if !os.IsNotExist(err) {
			return base, fmt.Errorf("cannot access base server configuration")
		}
	}
	return base, nil
}

func serverModeFlag(cmd *cobra.Command, fallback string) string {
	if cmd.Flags().Changed("mode") {
		mode, _ := cmd.Flags().GetString("mode")
		return mode
	}
	for _, entry := range []struct{ flag, mode string }{{"conference", "conference"}, {"conf", "conference"}, {"duplex", "duplex"}, {"reverse", "reverse"}} {
		if cmd.Flags().Changed(entry.flag) {
			enabled, _ := cmd.Flags().GetBool(entry.flag)
			if enabled {
				return entry.mode
			}
			fallback = "normal"
		}
	}
	return fallback
}

func validServerMode(mode string) bool {
	switch config.AudioMode(mode) {
	case config.AudioModeNormal, config.AudioModeReverse, config.AudioModeDuplex, config.AudioModeConference:
		return true
	default:
		return false
	}
}
