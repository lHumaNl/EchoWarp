package audio

import (
	"context"
	"log/slog"
)

// AudioProcessor transforms audio samples in a processing pipeline.
// Processors can modify samples, filter silence, or add metadata.
// Returning nil indicates the frame should be dropped (e.g., silence filtering).
type AudioProcessor interface {
	Process(ctx context.Context, samples []float32) ([]float32, error)
	Name() string
}

// ProcessorChain executes multiple AudioProcessors in sequence.
// If any processor returns nil samples, the chain stops and returns nil.
type ProcessorChain struct {
	processors []AudioProcessor
}

// NewProcessorChain creates a chain from the provided processors.
// Processors are executed in the order given.
func NewProcessorChain(processors ...AudioProcessor) *ProcessorChain {
	return &ProcessorChain{processors: processors}
}

// Process runs samples through all processors in sequence.
// Returns nil if any processor returns nil (frame should be dropped).
// Checks ctx between processors for early termination.
func (c *ProcessorChain) Process(ctx context.Context, samples []float32) ([]float32, error) {
	var err error
	for _, p := range c.processors {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		samples, err = p.Process(ctx, samples)
		if err != nil {
			return nil, err
		}
		if samples == nil {
			return nil, nil
		}
	}
	return samples, nil
}

// Name returns the identifier for this processor chain.
func (c *ProcessorChain) Name() string {
	return "chain"
}

// Add appends a processor to the end of the chain.
func (c *ProcessorChain) Add(p AudioProcessor) {
	c.processors = append(c.processors, p)
}

// Processors returns a copy of the processor list.
func (c *ProcessorChain) Processors() []AudioProcessor {
	return c.processors
}

// GainProcessor applies a gain multiplier to audio samples.
// Samples are clamped to [-1.0, 1.0] to prevent clipping artifacts.
type GainProcessor struct {
	gain float64
}

// NewGainProcessor creates a processor with the specified gain multiplier.
// Use values > 1.0 to amplify, < 1.0 to attenuate.
func NewGainProcessor(gain float64) *GainProcessor {
	return &GainProcessor{gain: gain}
}

func (p *GainProcessor) Process(ctx context.Context, samples []float32) ([]float32, error) {
	for i := range samples {
		samples[i] *= float32(p.gain)
		if samples[i] > 1.0 {
			samples[i] = 1.0
		} else if samples[i] < -1.0 {
			samples[i] = -1.0
		}
	}
	return samples, nil
}

func (p *GainProcessor) Name() string { return "gain" }

// SetGain updates the gain multiplier.
func (p *GainProcessor) SetGain(gain float64) {
	p.gain = gain
}

// Gain returns the current gain multiplier.
func (p *GainProcessor) Gain() float64 {
	return p.gain
}

// LoggingProcessor periodically logs audio statistics for debugging.
// Every 100 frames, it logs sample count and peak amplitude.
type LoggingProcessor struct {
	name  string
	log   *slog.Logger
	count int
}

// NewLoggingProcessor creates a processor that logs stats under the given name.
func NewLoggingProcessor(name string, log *slog.Logger) *LoggingProcessor {
	return &LoggingProcessor{name: name, log: log}
}

func (p *LoggingProcessor) Process(ctx context.Context, samples []float32) ([]float32, error) {
	p.count++
	if p.count%100 == 0 && p.log != nil {
		var maxAbs float32
		for _, s := range samples {
			if s < 0 {
				s = -s
			}
			if s > maxAbs {
				maxAbs = s
			}
		}
		p.log.Debug("audio stats",
			"processor", p.name,
			"samples", len(samples),
			"frames", p.count,
			"peak", maxAbs)
	}
	return samples, nil
}

func (p *LoggingProcessor) Name() string { return "logging:" + p.name }

// SilenceProcessor filters out frames with RMS energy below the threshold.
// This reduces bandwidth by not transmitting silence.
type SilenceProcessor struct {
	threshold float64
}

// NewSilenceProcessor creates a filter that drops frames with RMS below threshold.
// Typical threshold values: 0.01 for aggressive filtering, 0.001 for conservative.
func NewSilenceProcessor(threshold float64) *SilenceProcessor {
	return &SilenceProcessor{threshold: threshold}
}

func (p *SilenceProcessor) Process(ctx context.Context, samples []float32) ([]float32, error) {
	if len(samples) == 0 {
		return nil, nil
	}

	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	rms := sum / float64(len(samples))

	if rms < p.threshold*p.threshold {
		return nil, nil
	}
	return samples, nil
}

func (p *SilenceProcessor) Name() string { return "silence_filter" }

// SetThreshold updates the RMS threshold for silence detection.
func (p *SilenceProcessor) SetThreshold(threshold float64) {
	p.threshold = threshold
}

// Threshold returns the current silence detection threshold.
func (p *SilenceProcessor) Threshold() float64 {
	return p.threshold
}

// NormalizeProcessor scales audio to reach a target peak level.
// This ensures consistent volume across different sources.
type NormalizeProcessor struct {
	targetLevel float64
}

// NewNormalizeProcessor creates a normalizer targeting the specified peak level.
// targetLevel: 0.5-0.9 is typical; 1.0 may cause clipping in downstream processing.
func NewNormalizeProcessor(targetLevel float64) *NormalizeProcessor {
	return &NormalizeProcessor{targetLevel: targetLevel}
}

func (p *NormalizeProcessor) Process(ctx context.Context, samples []float32) ([]float32, error) {
	if len(samples) == 0 {
		return samples, nil
	}

	var maxAbs float32
	for _, s := range samples {
		if s < 0 {
			s = -s
		}
		if s > maxAbs {
			maxAbs = s
		}
	}

	if maxAbs < 0.001 {
		return samples, nil
	}

	scale := float32(p.targetLevel) / maxAbs
	for i := range samples {
		samples[i] *= scale
	}

	return samples, nil
}

func (p *NormalizeProcessor) Name() string { return "normalize" }

// SetTargetLevel updates the target peak level for normalization.
func (p *NormalizeProcessor) SetTargetLevel(level float64) {
	p.targetLevel = level
}

// TargetLevel returns the current target peak level.
func (p *NormalizeProcessor) TargetLevel() float64 {
	return p.targetLevel
}

// ProcessingCapturer wraps an AudioCapturer and applies a processing chain
// to all captured samples before they are delivered to the output channel.
type ProcessingCapturer struct {
	capturer   AudioCapturer
	processors *ProcessorChain
}

// NewProcessingCapturer creates a capturer that processes samples through
// the provided processors before output.
func NewProcessingCapturer(capturer AudioCapturer, processors ...AudioProcessor) *ProcessingCapturer {
	return &ProcessingCapturer{
		capturer:   capturer,
		processors: NewProcessorChain(processors...),
	}
}

func (c *ProcessingCapturer) Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error {
	processedCh := make(chan []float32, 16)

	go func() {
		defer close(processedCh)
		for {
			select {
			case <-ctx.Done():
				return
			case samples, ok := <-processedCh:
				if !ok {
					return
				}
				processed, err := c.processors.Process(ctx, samples)
				if err != nil {
					continue
				}
				if processed != nil {
					select {
					case outCh <- processed:
					default:
					}
				}
			}
		}
	}()

	return c.capturer.Start(ctx, deviceID, processedCh)
}

func (c *ProcessingCapturer) SampleRate() uint32 {
	return c.capturer.SampleRate()
}

func (c *ProcessingCapturer) Channels() uint32 {
	return c.capturer.Channels()
}

func (c *ProcessingCapturer) Close() error {
	return c.capturer.Close()
}

// AddProcessor appends a processor to the processing chain.
func (c *ProcessingCapturer) AddProcessor(p AudioProcessor) {
	c.processors.Add(p)
}

// Processors returns the underlying processor chain for inspection.
func (c *ProcessingCapturer) Processors() *ProcessorChain {
	return c.processors
}

// ProcessingPlayer wraps an AudioPlayer and applies a processing chain
// to all samples before they are played to the output device.
type ProcessingPlayer struct {
	player     AudioPlayer
	processors *ProcessorChain
}

// NewProcessingPlayer creates a player that processes samples through
// the provided processors before playback.
func NewProcessingPlayer(player AudioPlayer, processors ...AudioProcessor) *ProcessingPlayer {
	return &ProcessingPlayer{
		player:     player,
		processors: NewProcessorChain(processors...),
	}
}

func (p *ProcessingPlayer) Start(ctx context.Context, deviceID uint32, inCh <-chan []float32) error {
	processedCh := make(chan []float32, 16)

	go func() {
		defer close(processedCh)
		for {
			select {
			case <-ctx.Done():
				return
			case samples, ok := <-inCh:
				if !ok {
					return
				}
				processed, err := p.processors.Process(ctx, samples)
				if err != nil {
					continue
				}
				if processed != nil {
					select {
					case processedCh <- processed:
					default:
					}
				}
			}
		}
	}()

	return p.player.Start(ctx, deviceID, processedCh)
}

func (p *ProcessingPlayer) Close() error {
	return p.player.Close()
}

// AddProcessor appends a processor to the processing chain.
func (p *ProcessingPlayer) AddProcessor(processor AudioProcessor) {
	p.processors.Add(processor)
}

// Processors returns the underlying processor chain for inspection.
func (p *ProcessingPlayer) Processors() *ProcessorChain {
	return p.processors
}
