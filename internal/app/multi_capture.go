package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// MultiCapturePipeline captures audio from multiple devices simultaneously,
// mixes them into a single stream, encodes with Opus, and sends to a channel.
type MultiCapturePipeline struct {
	devices          []config.DeviceEntry
	sampleRate       uint32
	channels         uint32
	encoderCfg       EncoderConfig
	bufferFrames     int
	logger           *slog.Logger
	configureEncoder func(enc *audio.OpusEncoder)
	mixer            *audio.AudioMixer
	spectrum         *audio.SpectrumAnalyzer
	levelMeter       *audio.LevelMeter
	agcProcessors    map[uint32]*audio.AGCProcessor
	recordingTap     func([]float32)
}

// MultiCapturePipelineConfig holds configuration for a multi-device capture pipeline.
type MultiCapturePipelineConfig struct {
	Devices       []config.DeviceEntry
	SampleRate    uint32
	Channels      uint32
	EncoderConfig EncoderConfig
	BufferFrames  int
	Normalize     bool // if true, apply 1/sqrt(N) normalization
	Spectrum      *audio.SpectrumAnalyzer
	LevelMeter    *audio.LevelMeter

	// AGCProcessors maps deviceID → AGCProcessor for per-device AGC.
	// Entries may be nil; only devices with AGC enabled have processors.
	AGCProcessors map[uint32]*audio.AGCProcessor

	// RecordingTap is called with each mixed PCM frame for non-conference recording. May be nil.
	RecordingTap func([]float32)
}

// NewMultiCapturePipeline creates a pipeline that captures from multiple devices.
func NewMultiCapturePipeline(cfg MultiCapturePipelineConfig, logger *slog.Logger) *MultiCapturePipeline {
	frameSize := int(cfg.SampleRate) / 50 // 20ms frames
	mixer := audio.NewAudioMixer(audio.MixerConfig{
		SampleRate:   cfg.SampleRate,
		Channels:     int(cfg.Channels),
		FrameSize:    frameSize,
		BufferFrames: 3,
	})
	mixer.SetNormalize(cfg.Normalize)

	p := &MultiCapturePipeline{
		devices:       cfg.Devices,
		sampleRate:    cfg.SampleRate,
		channels:      cfg.Channels,
		encoderCfg:    cfg.EncoderConfig,
		bufferFrames:  cfg.BufferFrames,
		logger:        logger,
		mixer:         mixer,
		spectrum:      cfg.Spectrum,
		levelMeter:    cfg.LevelMeter,
		agcProcessors: cfg.AGCProcessors,
		recordingTap:  cfg.RecordingTap,
	}
	p.configureEncoder = p.defaultConfigureEncoder
	return p
}

// Mixer returns the underlying AudioMixer for external volume/mute control.
func (p *MultiCapturePipeline) Mixer() *audio.AudioMixer {
	return p.mixer
}

// AGCProcessors returns the per-device AGC processor map for runtime toggle.
func (p *MultiCapturePipeline) AGCProcessors() map[uint32]*audio.AGCProcessor {
	return p.agcProcessors
}

// Run starts capturing from all devices, mixing, encoding and sending.
// Blocks until ctx is canceled.
func (p *MultiCapturePipeline) Run(ctx context.Context, sendCh chan<- []byte) error {
	if len(p.devices) == 0 {
		return ewerrors.NewError(ewerrors.ErrDeviceNotFound, "no capture devices configured")
	}

	// Single device: use simple capture pipeline for zero overhead
	if len(p.devices) == 1 {
		return p.runSingle(ctx, sendCh, p.devices[0])
	}

	return p.runMulti(ctx, sendCh)
}

func (p *MultiCapturePipeline) runSingle(ctx context.Context, sendCh chan<- []byte, dev config.DeviceEntry) error {
	var agc *audio.AGCProcessor
	if p.agcProcessors != nil {
		agc = p.agcProcessors[dev.ID]
	}
	pipeline := NewCapturePipeline(CapturePipelineConfig{
		SampleRate:        p.sampleRate,
		Channels:          p.channels,
		DeviceID:          dev.ID,
		EncoderConfig:     p.encoderCfg,
		AudioBufferFrames: p.bufferFrames,
		Spectrum:          p.spectrum,
		LevelMeter:        p.levelMeter,
		AGC:               agc,
		RecordingTap:      p.recordingTap,
	}, p.logger)
	pipeline.configureEncoder = p.configureEncoder
	return pipeline.Run(ctx, sendCh)
}

func (p *MultiCapturePipeline) runMulti(ctx context.Context, sendCh chan<- []byte) error {
	// Create Opus encoder for the mixed output
	enc, err := audio.NewOpusEncoder(int(p.sampleRate), int(p.channels), p.encoderCfg.Application)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "create opus encoder for multi-capture")
	}
	p.configureEncoder(enc)

	frameSize := int(p.sampleRate) / 50 * int(p.channels)
	acc := audio.NewFrameAccumulator(frameSize)

	// Start per-device capture goroutines
	var wg sync.WaitGroup
	captureCtx, captureCancel := context.WithCancel(ctx)
	defer captureCancel()

	for _, dev := range p.devices {
		devCh := make(chan []float32, 5)
		sourceID := fmt.Sprintf("device-%d", dev.ID)
		p.mixer.AddSourceWithVolume(sourceID, devCh, float32(dev.Volume))
		if dev.Muted {
			p.mixer.SetSourceMuted(sourceID, true)
		}

		var agc *audio.AGCProcessor
		if p.agcProcessors != nil {
			agc = p.agcProcessors[dev.ID]
		}

		wg.Add(1)
		go func(deviceID uint32, ch chan<- []float32, agcProc *audio.AGCProcessor) {
			defer wg.Done()
			defer close(ch)
			p.captureDevice(captureCtx, deviceID, ch, agcProc)
		}(dev.ID, devCh, agc)
	}

	// Start mixer
	go func() {
		if err := p.mixer.Run(captureCtx); err != nil && captureCtx.Err() == nil {
			p.logger.Error("Mixer error", "error", err)
		}
	}()

	// Read mixed output → encode → send
	for {
		select {
		case <-ctx.Done():
			captureCancel()
			wg.Wait()
			return ctx.Err()
		case mixed, ok := <-p.mixer.Output():
			if !ok {
				return nil
			}
			if p.spectrum != nil {
				p.spectrum.Feed(mixed)
			}
			if p.levelMeter != nil {
				p.levelMeter.Feed(mixed)
			}
			if p.recordingTap != nil {
				tapCopy := make([]float32, len(mixed))
				copy(tapCopy, mixed)
				p.recordingTap(tapCopy)
			}
			frames := acc.Write(mixed)
			p.mixer.PutMixedFrame(mixed)
			for _, frame := range frames {
				encoded, encErr := enc.Encode(frame)
				if encErr != nil {
					p.logger.Warn("Opus encode error", "error", encErr)
					continue
				}
				metrics.AudioBytesSent.Add(float64(len(encoded)))
				select {
				case sendCh <- encoded:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

func (p *MultiCapturePipeline) captureDevice(ctx context.Context, deviceID uint32, ch chan<- []float32, agc *audio.AGCProcessor) {
	capturer, err := audio.NewCapturer(p.sampleRate, p.channels)
	if err != nil {
		p.logger.Error("Failed to create capturer", "device", deviceID, "error", err)
		return
	}
	defer func() { _ = capturer.Close() }() //nolint:errcheck

	bufSize := p.bufferFrames
	if bufSize <= 0 {
		bufSize = 5
	}
	pcmCh := make(chan []float32, bufSize)

	go func() {
		if err := capturer.Start(ctx, deviceID, pcmCh); err != nil && ctx.Err() == nil {
			p.logger.Error("Capture error", "device", deviceID, "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case samples, ok := <-pcmCh:
			if !ok {
				return
			}
			// Apply per-device AGC before feeding into mixer.
			if agc != nil {
				if processed, err := agc.Process(ctx, samples); err == nil {
					samples = processed
				}
			}
			select {
			case ch <- samples:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (p *MultiCapturePipeline) defaultConfigureEncoder(enc *audio.OpusEncoder) {
	if err := enc.SetBitrate(p.encoderCfg.Bitrate); err != nil {
		p.logger.Warn("Failed to set opus bitrate", "error", err)
	}
}

// NewServerMultiCapturePipeline creates a server-side multi-capture pipeline with full Opus config.
func NewServerMultiCapturePipeline(cfg MultiCapturePipelineConfig, logger *slog.Logger) *MultiCapturePipeline {
	p := NewMultiCapturePipeline(cfg, logger)
	p.configureEncoder = func(enc *audio.OpusEncoder) {
		p.defaultConfigureEncoder(enc)
		if err := enc.SetComplexity(p.encoderCfg.Complexity); err != nil {
			p.logger.Warn("Failed to set opus complexity", "error", err)
		}
		if err := enc.SetDTX(p.encoderCfg.DTX); err != nil {
			p.logger.Warn("Failed to set opus DTX", "error", err)
		}
		if err := enc.SetInBandFEC(p.encoderCfg.FEC); err != nil {
			p.logger.Warn("Failed to set opus FEC", "error", err)
		}
	}
	return p
}
