package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// HandleDeviceCommands processes device control commands from the TUI and applies
// them to the mixer. It blocks until ctx is canceled or the command channel is closed.
// aec may be nil if echo cancellation is not enabled.
func HandleDeviceCommands(ctx context.Context, cmdCh <-chan DeviceCommand, mixer *audio.AudioMixer, logger *slog.Logger) {
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
				current := mixer.IsSourceMuted(sourceID)
				mixer.SetSourceMuted(sourceID, !current)
				logger.Info("Device mute toggled", "device", cmd.DeviceID, "muted", !current)
			case DeviceGlobalMute:
				current := mixer.IsGlobalMuted()
				mixer.SetGlobalMute(!current)
				logger.Info("Global mute toggled", "muted", !current)
			case DeviceVolumeUp:
				vol := mixer.GetSourceVolume(sourceID)
				vol += 0.1
				if vol > 2.0 {
					vol = 2.0
				}
				mixer.SetSourceVolume(sourceID, vol)
				logger.Info("Device volume up", "device", cmd.DeviceID, "volume", vol)
			case DeviceVolumeDown:
				vol := mixer.GetSourceVolume(sourceID)
				vol -= 0.1
				if vol < 0 {
					vol = 0
				}
				mixer.SetSourceVolume(sourceID, vol)
				logger.Info("Device volume down", "device", cmd.DeviceID, "volume", vol)
			}
		}
	}
}
