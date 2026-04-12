package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// HandleDeviceCommands processes device control commands from the TUI and applies
// them to the mixer. It blocks until ctx is canceled or the command channel is closed.
// agcProcessors may be nil if no devices have AGC enabled.
func HandleDeviceCommands(ctx context.Context, cmdCh <-chan DeviceCommand, mixer *audio.AudioMixer, agcProcessors map[uint32]*audio.AGCProcessor, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-cmdCh:
			if !ok {
				return
			}
			sourceID := fmt.Sprintf("device-%d", cmd.DeviceID)
			switch cmd.Action {
			case DeviceToggleMute:
				if mixer == nil {
					logger.Debug("Device mute toggle ignored: no mixer (single-device mode)", "device", cmd.DeviceID)
					continue
				}
				current := mixer.IsSourceMuted(sourceID)
				mixer.SetSourceMuted(sourceID, !current)
				logger.Info("Device mute toggled", "device", cmd.DeviceID, "muted", !current)
			case DeviceGlobalMute:
				if mixer == nil {
					logger.Debug("Global mute toggle ignored: no mixer (single-device mode)")
					continue
				}
				current := mixer.IsGlobalMuted()
				mixer.SetGlobalMute(!current)
				logger.Info("Global mute toggled", "muted", !current)
			case DeviceVolumeUp:
				if mixer == nil {
					logger.Debug("Device volume up ignored: no mixer (single-device mode)", "device", cmd.DeviceID)
					continue
				}
				vol := mixer.GetSourceVolume(sourceID)
				vol = float32(math.Round(float64(vol+0.1)*10) / 10)
				if vol > 1.5 {
					vol = 1.5
				}
				mixer.SetSourceVolume(sourceID, vol)
				logger.Info("Device volume up", "device", cmd.DeviceID, "volume", vol)
			case DeviceVolumeDown:
				if mixer == nil {
					logger.Debug("Device volume down ignored: no mixer (single-device mode)", "device", cmd.DeviceID)
					continue
				}
				vol := mixer.GetSourceVolume(sourceID)
				vol = float32(math.Round(float64(vol-0.1)*10) / 10)
				if vol < 0 {
					vol = 0
				}
				mixer.SetSourceVolume(sourceID, vol)
				logger.Info("Device volume down", "device", cmd.DeviceID, "volume", vol)
			case DeviceToggleAGC:
				if agcProcessors != nil {
					if agcProc, ok := agcProcessors[cmd.DeviceID]; ok {
						newState := !agcProc.IsEnabled()
						agcProc.SetEnabled(newState)
						logger.Info("Device AGC toggled", "device", cmd.DeviceID, "enabled", newState)
					} else {
						logger.Warn("Device AGC toggle: no AGC processor for device", "device", cmd.DeviceID)
					}
				} else {
					logger.Warn("Device AGC toggle: AGC not configured for any device", "device", cmd.DeviceID)
				}
			}
		}
	}
}
