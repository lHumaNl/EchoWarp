package app

import (
	"context"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// EncoderConfig holds Opus encoder parameters.
type EncoderConfig struct {
	Bitrate     int    // Target bitrate in bits per second (e.g., 64000).
	Complexity  int    // Encoder complexity 0-10 (higher = better quality, more CPU).
	DTX         bool   // Enable discontinuous transmission for silence.
	FEC         bool   // Enable in-band forward error correction.
	Application string // Opus application mode: "voip", "audio", or "lowdelay".
}

// CapturePipelineConfig configures an audio capture pipeline.
type CapturePipelineConfig struct {
	SampleRate        uint32        // Audio sample rate in Hz.
	Channels          uint32        // Number of audio channels (1=mono, 2=stereo).
	DeviceID          uint32        // Audio device ID to capture from.
	EncoderConfig     EncoderConfig // Opus encoder configuration.
	AudioBufferFrames int           // PCM channel buffer size in frames.

	// Loopback capture (macOS only).
	IsLoopback           bool   // If true, create an aggregate device for loopback capture.
	LoopbackOutputDevice string // Name of the output device (speakers) for the aggregate.
	LoopbackBlackHole    string // Name of the BlackHole input device for capture.

	// AEC: acoustic echo cancellation processor (optional, injected externally).
	AEC *audio.AECProcessor

	// AGC: automatic gain control processor (optional, injected externally).
	AGC *audio.AGCProcessor

	// Spectrum analyzer fed from capture PCM (optional).
	Spectrum *audio.SpectrumAnalyzer
	// Level meter fed from capture PCM (optional).
	LevelMeter *audio.LevelMeter

	// RecordingTap is called with each captured PCM frame (after AEC/AGC)
	// for non-conference recording. May be nil.
	RecordingTap func([]float32)

	// GainControl provides atomic volume/mute for single-device mode (no mixer).
	GainControl *DeviceGainControl
}

// CapturePipeline captures PCM audio from a device, accumulates frames to the
// correct size, encodes them with Opus, and sends them to a channel.
// The configureEncoder hook allows subtype-specific encoder configuration.
type CapturePipeline struct {
	cfg              CapturePipelineConfig
	logger           *slog.Logger
	configureEncoder func(enc *audio.OpusEncoder)
}

// NewCapturePipeline creates a capture pipeline with the given configuration.
func NewCapturePipeline(cfg CapturePipelineConfig, logger *slog.Logger) *CapturePipeline {
	p := &CapturePipeline{
		cfg:    cfg,
		logger: logger,
	}
	p.configureEncoder = p.defaultConfigureEncoder
	return p
}

// Run starts the capture pipeline and blocks until the context is canceled or
// a fatal error occurs. Captured and encoded audio frames are sent to sendCh.
func (p *CapturePipeline) Run(ctx context.Context, sendCh chan<- []byte) error {
	deviceID := p.cfg.DeviceID

	// Loopback: create loopback session and override capture device.
	var capturerOpts []audio.CapturerOption
	if p.cfg.IsLoopback {
		session, loopErr := audio.NewLoopbackSession(p.cfg.LoopbackOutputDevice, p.cfg.LoopbackBlackHole)
		if loopErr != nil {
			return ewerrors.Wrap(loopErr, ewerrors.ErrAggregateDeviceCreate, "create loopback session")
		}
		defer func() { _ = session.Close() }() //nolint:errcheck
		deviceID = session.CaptureDeviceID()
		if session.IsNativeLoopback() {
			capturerOpts = append(capturerOpts, audio.WithLoopbackMode())
		}
		p.logger.Info("Loopback capture active",
			"output", p.cfg.LoopbackOutputDevice,
			"nativeLoopback", session.IsNativeLoopback(),
			"captureDevice", deviceID)
	}

	capturer, err := audio.NewCapturer(p.cfg.SampleRate, p.cfg.Channels, capturerOpts...)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "create audio capturer")
	}
	defer func() { _ = capturer.Close() }() //nolint:errcheck

	enc, err := audio.NewOpusEncoder(int(p.cfg.SampleRate), int(p.cfg.Channels), p.cfg.EncoderConfig.Application)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "create opus encoder")
	}

	p.configureEncoder(enc)

	frameSize := int(p.cfg.SampleRate) / 50 * int(p.cfg.Channels)
	acc := audio.NewFrameAccumulator(frameSize)

	bufSize := p.cfg.AudioBufferFrames
	if bufSize <= 0 {
		bufSize = 5
	}
	pcmCh := make(chan []float32, bufSize)

	go func() {
		if err := capturer.Start(ctx, deviceID, pcmCh); err != nil && ctx.Err() == nil {
			p.logger.Error("Capture error", "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case samples := <-pcmCh:
			// Apply AEC if enabled: remove echo from captured audio.
			if p.cfg.AEC != nil && p.cfg.AEC.IsEnabled() {
				processed, aecErr := p.cfg.AEC.Process(ctx, samples)
				if aecErr == nil {
					samples = processed
				}
			}
			// Apply AGC if enabled: normalize volume levels.
			if p.cfg.AGC != nil {
				processed, agcErr := p.cfg.AGC.Process(ctx, samples)
				if agcErr == nil {
					samples = processed
				}
			}
			// Apply device gain/mute (single-device mode without mixer).
			if p.cfg.GainControl != nil {
				if p.cfg.GainControl.IsMuted() {
					for i := range samples {
						samples[i] = 0
					}
				} else if gain := p.cfg.GainControl.Gain(); gain != 1.0 {
					audio.MixGain(samples, gain)
					// Soft-clip when amplifying to avoid hard-clip distortion.
					if gain > 1.0 {
						audio.MixTanh(samples)
					}
				}
			}
			// Recording tap: feed post-AEC/AGC PCM to the recorder.
			if p.cfg.RecordingTap != nil {
				tapCopy := make([]float32, len(samples))
				copy(tapCopy, samples)
				p.cfg.RecordingTap(tapCopy)
			}
			if p.cfg.Spectrum != nil {
				p.cfg.Spectrum.Feed(samples)
			}
			if p.cfg.LevelMeter != nil {
				p.cfg.LevelMeter.Feed(samples)
			}
			frames := acc.Write(samples)
			for _, frame := range frames {
				encoded, err := enc.Encode(frame)
				if err != nil {
					p.logger.Warn("Opus encode error", "error", err)
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

func (p *CapturePipeline) defaultConfigureEncoder(enc *audio.OpusEncoder) {
	if err := enc.SetBitrate(p.cfg.EncoderConfig.Bitrate); err != nil {
		p.logger.Warn("Failed to set opus bitrate", "error", err)
	}
}

// NewServerCapturePipeline creates a server-side capture pipeline with full Opus configuration.
// Servers control audio quality settings including complexity, DTX, and FEC.
func NewServerCapturePipeline(cfg CapturePipelineConfig, logger *slog.Logger) *CapturePipeline {
	p := NewCapturePipeline(cfg, logger)
	p.configureEncoder = func(enc *audio.OpusEncoder) {
		p.defaultConfigureEncoder(enc)
		if err := enc.SetComplexity(p.cfg.EncoderConfig.Complexity); err != nil {
			p.logger.Warn("Failed to set opus complexity", "error", err)
		}
		if err := enc.SetDTX(p.cfg.EncoderConfig.DTX); err != nil {
			p.logger.Warn("Failed to set opus DTX", "error", err)
		}
		if err := enc.SetInBandFEC(p.cfg.EncoderConfig.FEC); err != nil {
			p.logger.Warn("Failed to set opus FEC", "error", err)
		}
	}
	return p
}

// NewClientCapturePipeline creates a client-side capture pipeline with minimal Opus configuration.
// Clients only set bitrate; other encoder settings are received from the server.
func NewClientCapturePipeline(cfg CapturePipelineConfig, logger *slog.Logger) *CapturePipeline {
	return NewCapturePipeline(cfg, logger)
}

// safeSend sends data to ch, returning ctx.Err() on context cancellation
// or a closed-channel error if the peer has already been torn down.
func safeSend(ctx context.Context, ch chan<- []byte, data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed by peer.Close() before context was canceled.
			err = context.Canceled
		}
	}()
	select {
	case ch <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
