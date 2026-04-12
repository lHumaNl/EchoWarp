package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// DeviceGainControl provides atomic gain/mute for single-device mode where no
// AudioMixer exists. The capture pipeline reads these atomics every frame.
type DeviceGainControl struct {
	gain  atomic.Uint32 // float32 bits via math.Float32bits/frombits
	muted atomic.Bool
}

// NewDeviceGainControl creates a gain control initialized to the given volume.
func NewDeviceGainControl(initialVolume float32) *DeviceGainControl {
	g := &DeviceGainControl{}
	g.SetGain(initialVolume)
	return g
}

// Gain returns the current gain factor.
func (g *DeviceGainControl) Gain() float32 {
	return math.Float32frombits(g.gain.Load())
}

// SetGain sets the gain factor atomically.
func (g *DeviceGainControl) SetGain(v float32) {
	g.gain.Store(math.Float32bits(v))
}

// IsMuted returns the current mute state.
func (g *DeviceGainControl) IsMuted() bool {
	return g.muted.Load()
}

// SetMuted sets the mute state atomically.
func (g *DeviceGainControl) SetMuted(m bool) {
	g.muted.Store(m)
}

// HandleDeviceCommands processes device control commands from the TUI and applies
// them to the mixer (multi-device) or gain control (single-device). It blocks
// until ctx is canceled or the command channel is closed.
// agcProcessors may be nil if no devices have AGC enabled.
// gainCtl may be nil; when non-nil it is used as fallback when mixer is nil.
func HandleDeviceCommands(ctx context.Context, cmdCh <-chan DeviceCommand, mixer *audio.AudioMixer, agcProcessors map[uint32]*audio.AGCProcessor, gainCtl *DeviceGainControl, logger *slog.Logger) {
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
				if mixer != nil {
					current := mixer.IsSourceMuted(sourceID)
					mixer.SetSourceMuted(sourceID, !current)
					logger.Info("Device mute toggled", "device", cmd.DeviceID, "muted", !current)
				} else if gainCtl != nil {
					current := gainCtl.IsMuted()
					gainCtl.SetMuted(!current)
					logger.Info("Device mute toggled (gain control)", "device", cmd.DeviceID, "muted", !current)
				}
			case DeviceGlobalMute:
				if mixer != nil {
					current := mixer.IsGlobalMuted()
					mixer.SetGlobalMute(!current)
					logger.Info("Global mute toggled", "muted", !current)
				} else if gainCtl != nil {
					current := gainCtl.IsMuted()
					gainCtl.SetMuted(!current)
					logger.Info("Global mute toggled (gain control)", "muted", !current)
				}
			case DeviceVolumeUp:
				if mixer != nil {
					vol := mixer.GetSourceVolume(sourceID)
					vol = float32(math.Round(float64(vol+0.1)*10) / 10)
					if vol > 1.5 {
						vol = 1.5
					}
					mixer.SetSourceVolume(sourceID, vol)
					logger.Info("Device volume up", "device", cmd.DeviceID, "volume", vol)
				} else if gainCtl != nil {
					vol := gainCtl.Gain()
					vol = float32(math.Round(float64(vol+0.1)*10) / 10)
					if vol > 1.5 {
						vol = 1.5
					}
					gainCtl.SetGain(vol)
					logger.Info("Device volume up (gain control)", "device", cmd.DeviceID, "volume", vol)
				}
			case DeviceVolumeDown:
				if mixer != nil {
					vol := mixer.GetSourceVolume(sourceID)
					vol = float32(math.Round(float64(vol-0.1)*10) / 10)
					if vol < 0 {
						vol = 0
					}
					mixer.SetSourceVolume(sourceID, vol)
					logger.Info("Device volume down", "device", cmd.DeviceID, "volume", vol)
				} else if gainCtl != nil {
					vol := gainCtl.Gain()
					vol = float32(math.Round(float64(vol-0.1)*10) / 10)
					if vol < 0 {
						vol = 0
					}
					gainCtl.SetGain(vol)
					logger.Info("Device volume down (gain control)", "device", cmd.DeviceID, "volume", vol)
				}
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
