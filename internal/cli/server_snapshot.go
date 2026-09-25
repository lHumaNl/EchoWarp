package cli

import (
	"log/slog"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/preset"
	"github.com/lHumaNl/echowarp/internal/startup"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func saveHeadlessServerSnapshot(cfg config.Config, logger *slog.Logger) {
	var devices []audio.AudioDevice
	if !cfg.Conference || !cfg.ServerMuted {
		dm, err := audio.NewDeviceManager()
		if err != nil {
			logger.Warn("Cannot snapshot server audio devices")
			return
		}
		defer dm.Close() //nolint:errcheck
		devices, err = listAllDevices(dm)
		if err != nil {
			logger.Warn("Cannot snapshot server audio devices")
			return
		}
		devices, _ = appendLoopbackDevices(dm, devices, cfg.Duplex || !cfg.Reverse)
	}
	resolved, err := startup.Prepare(cfg, devices, nil, nil)
	if err == nil {
		err = preset.SaveSnapshot(resolved, startup.PresetFromConfig(resolved))
	}
	if err != nil {
		logger.Warn("Cannot save server profile", "error", err)
	}
}
