package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/probe"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func prepareStartupIntent(cmd *cobra.Command, cfg *config.Config) (startup.Intent, error) {
	intent := startup.Intent{}
	for _, name := range []string{"device", "capture-device", "playback-device", "log-level", "log-file", "max-reconnect", "auto-reconnect", "auto-reconnect-attempts", "aec", "loopback", "virtual-mic", "tls-insecure", "no-simd-optimization", "no-pool-warmup"} {
		if cmd.Flags().Changed(name) {
			intent.ExplicitFlags = append(intent.ExplicitFlags, name)
		}
	}
	for _, name := range []string{"device", "device-name", "capture-device", "playback-device", "address", "config", "mode"} {
		intent.Requested = intent.Requested || cmd.Flags().Changed(name)
	}
	intent.DeviceName, _ = cmd.Flags().GetString("device-name")
	intent.Discover, _ = cmd.Flags().GetBool("discover")
	intent.DiscoveryTimeout, _ = cmd.Flags().GetDuration("discover-timeout")
	if cfg.Conference && cfg.ServerMuted && cmd.Flags().Changed("conference") && cmd.Flags().Changed("server-muted") {
		intent.Requested = true
	}
	useRecent, _ := cmd.Flags().GetBool("recent")
	if !useRecent {
		return intent, nil
	}
	if cfg.Mode == config.ModeServer {
		return prepareServerRecent(cmd, cfg, intent)
	}
	if cmd.Flags().Changed("address") || cmd.Flags().Changed("port") {
		return intent, fmt.Errorf("--recent cannot be combined with --address or --port")
	}
	intent.Requested = true
	saved, err := startup.LatestRecent()
	if err != nil {
		intent.Problem = err.Error()
		cfg.Address = ""
		return intent, nil
	}
	intent.Recent = &saved
	cfg.Address, cfg.Port = saved.Address, saved.Port
	return intent, nil
}

func prepareRecentClientConfig(cmd *cobra.Command, cfg *config.Config, devices []audio.AudioDevice, info *probe.ProbeServerResult, intent startup.Intent) error {
	prepared, _, err := startup.Resolve(context.Background(), *cfg, devices, intent, startup.Network{
		Probe: func(context.Context, string, int) (*probe.ProbeServerResult, error) { return info, nil },
	})
	if err != nil {
		return err
	}
	*cfg = prepared
	// Do not apply probe defaults a second time: that can undo explicit false
	// values such as --tls-insecure=false and replace checked device selection.
	return validateAndSaveConfig(cmd, cfg)
}
