package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// VolumeController abstracts per-source volume/mute operations so
// HandleDeviceCommands works identically with an AudioMixer (multi-device)
// and a DeviceGainControl (single-device).
type VolumeController interface {
	GetSourceVolume(sourceID string) float32
	SetSourceVolume(sourceID string, vol float32)
	IsSourceMuted(sourceID string) bool
	SetSourceMuted(sourceID string, muted bool)
	IsGlobalMuted() bool
	SetGlobalMute(muted bool)
}

// DeviceGainControl provides atomic gain/mute for single-device mode where no
// AudioMixer exists. The capture pipeline reads these atomics every frame.
// It implements VolumeController by ignoring the sourceID parameter.
type DeviceGainControl struct {
	gain        atomic.Uint32 // float32 bits via math.Float32bits/frombits
	muted       atomic.Bool
	globalMuted atomic.Bool
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

// IsMuted returns the current mute state (per-source OR global).
func (g *DeviceGainControl) IsMuted() bool {
	return g.muted.Load() || g.globalMuted.Load()
}

// SetMuted sets the per-source mute state atomically.
func (g *DeviceGainControl) SetMuted(m bool) {
	g.muted.Store(m)
}

// --- VolumeController interface (ignores sourceID for single-device) ---

// GetSourceVolume implements VolumeController.
func (g *DeviceGainControl) GetSourceVolume(_ string) float32 { return g.Gain() }

// SetSourceVolume implements VolumeController.
func (g *DeviceGainControl) SetSourceVolume(_ string, vol float32) { g.SetGain(vol) }

// IsSourceMuted implements VolumeController.
func (g *DeviceGainControl) IsSourceMuted(_ string) bool { return g.muted.Load() }

// SetSourceMuted implements VolumeController.
func (g *DeviceGainControl) SetSourceMuted(_ string, muted bool) { g.SetMuted(muted) }

// IsGlobalMuted implements VolumeController.
func (g *DeviceGainControl) IsGlobalMuted() bool { return g.globalMuted.Load() }

// SetGlobalMute implements VolumeController.
func (g *DeviceGainControl) SetGlobalMute(muted bool) { g.globalMuted.Store(muted) }

// HandleDeviceCommands processes device control commands from the TUI and applies
// them to the VolumeController (mixer or gain control). It blocks until ctx is
// canceled or the command channel is closed.
// ctrl may be nil; when nil, volume/mute commands are silently skipped.
// agcProcessors may be nil if no devices have AGC enabled.
func HandleDeviceCommands(ctx context.Context, cmdCh <-chan DeviceCommand, ctrl VolumeController, agcProcessors map[uint32]*audio.AGCProcessor, logger *slog.Logger) {
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
				if ctrl == nil {
					continue
				}
				current := ctrl.IsSourceMuted(sourceID)
				ctrl.SetSourceMuted(sourceID, !current)
				logger.Info("Device mute toggled", "device", cmd.DeviceID, "muted", !current)
			case DeviceGlobalMute:
				if ctrl == nil {
					continue
				}
				current := ctrl.IsGlobalMuted()
				ctrl.SetGlobalMute(!current)
				logger.Info("Global mute toggled", "muted", !current)
			case DeviceVolumeUp:
				if ctrl == nil {
					continue
				}
				vol := ctrl.GetSourceVolume(sourceID)
				vol = float32(math.Round(float64(vol+0.1)*10) / 10)
				if vol > 1.5 {
					vol = 1.5
				}
				ctrl.SetSourceVolume(sourceID, vol)
				logger.Info("Device volume up", "device", cmd.DeviceID, "volume", vol)
			case DeviceVolumeDown:
				if ctrl == nil {
					continue
				}
				vol := ctrl.GetSourceVolume(sourceID)
				vol = float32(math.Round(float64(vol-0.1)*10) / 10)
				if vol < 0 {
					vol = 0
				}
				ctrl.SetSourceVolume(sourceID, vol)
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
