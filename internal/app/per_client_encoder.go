package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// PerClientEncoderConfig configures a per-client encoder goroutine that
// consumes a SharedCaptureHub subscription.
type PerClientEncoderConfig struct {
	ClientID   string
	Nickname   string
	SampleRate uint32
	Channels   uint32
	Encoder    EncoderConfig
	// Gain is the per-client volume control. Mute is not applied here —
	// per-client mute is handled upstream via the sendCh mute filter.
	Gain *DeviceGainControl
	// ConfigureEncoder is an optional hook to set server-side Opus options
	// (complexity, DTX, FEC) after the encoder is created and bitrate applied.
	ConfigureEncoder func(*audio.OpusEncoder)
	// Stats is optional runtime instrumentation for tests or future metrics export.
	Stats *PerClientEncoderStats
	// InstrumentationInterval controls aggregate lag logging. Defaults to 2s.
	InstrumentationInterval time.Duration
	// HeartbeatInterval controls debug baseline stats logging. Defaults to 5s.
	HeartbeatInterval time.Duration
	// EncodeWarnThreshold logs encode max duration at or above this value.
	EncodeWarnThreshold time.Duration
	// SendWaitWarnThreshold logs safeSend wait max duration at or above this value.
	SendWaitWarnThreshold time.Duration
	// Muted returns the current per-client outgoing mute state when available.
	Muted func() bool
	// Paused returns the current outgoing pause state when available.
	Paused func() bool
}

// runPerClientEncoder consumes PCM frames from sub, applies per-client gain,
// encodes with Opus, and forwards encoded bytes to sendCh. Exits when ctx is
// done, the subscription channel closes, or sendCh is torn down.
//
// A fresh Opus encoder and FrameAccumulator are created per call so each
// client maintains independent encoder state (DTX, FEC, PLC) and frame
// boundaries.
func runPerClientEncoder(
	ctx context.Context,
	sub *CaptureSubscription,
	cfg PerClientEncoderConfig,
	sendCh chan<- []byte,
	logger *slog.Logger,
) error {
	defer sub.Close()

	enc, err := audio.NewOpusEncoder(int(cfg.SampleRate), int(cfg.Channels), cfg.Encoder.Application)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "per-client encoder: create opus encoder")
	}
	if err := enc.SetBitrate(cfg.Encoder.Bitrate); err != nil {
		logger.Warn("Per-client encoder: failed to set opus bitrate", "error", err)
	}
	if cfg.ConfigureEncoder != nil {
		cfg.ConfigureEncoder(enc)
	}

	frameSize := int(cfg.SampleRate) / 50 * int(cfg.Channels)
	acc := audio.NewFrameAccumulator(frameSize)
	monitor := newPerClientEncoderMonitor(cfg, sub, logger)
	monitor.LogActive()
	heartbeat := time.NewTicker(monitor.HeartbeatInterval())
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			monitor.ObserveQueue()
			monitor.LogHeartbeat()
		case samples, ok := <-sub.PCM:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return ewerrors.NewError(ewerrors.ErrInternalState, "per-client encoder: shared capture subscription closed")
			}
			monitor.ObserveQueue()
			samples = applyPerClientGain(samples, cfg.Gain)
			frames := acc.Write(samples)
			for _, frame := range frames {
				encodeStart := time.Now()
				encoded, err := enc.Encode(frame)
				monitor.RecordEncode(time.Since(encodeStart))
				if err != nil {
					logger.Warn("Per-client Opus encode error", "clientID", monitor.clientID, "nickname", monitor.nickname, "error", err)
					continue
				}
				metrics.AudioBytesSent.Add(float64(len(encoded)))
				sendStart := time.Now()
				if err := safeSend(ctx, sendCh, encoded); err != nil {
					monitor.RecordSendWait(time.Since(sendStart))
					monitor.LogCurrentIfUseful()
					return err
				}
				monitor.RecordSendWait(time.Since(sendStart))
				monitor.LogIfDue(time.Now())
			}
		}
	}
}

// applyPerClientGain returns samples with per-client volume applied. Returns
// the input slice unchanged when no scaling is needed; allocates a copy when
// scaling is applied so the hub's shared slice (visible to every subscriber)
// is not mutated.
func applyPerClientGain(samples []float32, gain *DeviceGainControl) []float32 {
	if gain == nil {
		return samples
	}
	g := gain.Gain()
	if g == 1.0 {
		return samples
	}
	scaled := make([]float32, len(samples))
	copy(scaled, samples)
	if g == 0 {
		for i := range scaled {
			scaled[i] = 0
		}
		return scaled
	}
	audio.MixGain(scaled, g)
	if g > 1.0 {
		audio.MixTanh(scaled)
	}
	return scaled
}
