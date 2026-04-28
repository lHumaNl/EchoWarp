package app

import (
	"context"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// PerClientEncoderConfig configures a per-client encoder goroutine that
// consumes a SharedCaptureHub subscription.
type PerClientEncoderConfig struct {
	SampleRate uint32
	Channels   uint32
	Encoder    EncoderConfig
	// Gain is the per-client volume control. Mute is not applied here —
	// per-client mute is handled upstream via the sendCh mute filter.
	Gain *DeviceGainControl
	// ConfigureEncoder is an optional hook to set server-side Opus options
	// (complexity, DTX, FEC) after the encoder is created and bitrate applied.
	ConfigureEncoder func(*audio.OpusEncoder)
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case samples, ok := <-sub.PCM:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return ewerrors.NewError(ewerrors.ErrInternalState, "per-client encoder: shared capture subscription closed")
			}
			samples = applyPerClientGain(samples, cfg.Gain)
			frames := acc.Write(samples)
			for _, frame := range frames {
				encoded, err := enc.Encode(frame)
				if err != nil {
					logger.Warn("Per-client Opus encode error", "error", err)
					continue
				}
				metrics.AudioBytesSent.Add(float64(len(encoded)))
				if err := safeSend(ctx, sendCh, encoded); err != nil {
					return err
				}
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
