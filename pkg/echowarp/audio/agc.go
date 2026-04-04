package audio

import (
	"context"
	"math"
	"sync"
)

// AGCProcessor implements automatic gain control targeting a specific dBFS level.
// It smoothly adjusts gain to maintain consistent volume across different sources.
//
// Uses a simple attack/release envelope follower with configurable target level.
type AGCProcessor struct {
	mu sync.Mutex

	targetDBFS   float64 // target level in dBFS (e.g., -18.0)
	maxGain      float64 // maximum gain multiplier to prevent noise amplification
	attackCoeff  float64 // how fast to reduce gain (fast, for loud signals)
	releaseCoeff float64 // how fast to increase gain (slow, for quiet signals)

	currentGain float64 // current gain multiplier
	enabled     bool
}

// AGCConfig holds configuration for the automatic gain control.
type AGCConfig struct {
	TargetDBFS float64 // target RMS level in dBFS (typical: -18.0)
	MaxGain    float64 // maximum gain to apply (typical: 20.0 = +26dB)
	AttackMs   float64 // attack time in ms (fast gain reduction, typical: 5.0)
	ReleaseMs  float64 // release time in ms (slow gain increase, typical: 50.0)
	SampleRate uint32  // for computing coefficients
}

// DefaultAGCConfig returns a standard voice AGC config at 48kHz.
func DefaultAGCConfig() AGCConfig {
	return AGCConfig{
		TargetDBFS: -18.0,
		MaxGain:    20.0,
		AttackMs:   5.0,
		ReleaseMs:  50.0,
		SampleRate: 48000,
	}
}

// NewAGCProcessor creates an AGC processor with the given config.
func NewAGCProcessor(cfg AGCConfig) *AGCProcessor {
	if cfg.MaxGain <= 0 {
		cfg.MaxGain = 20.0
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 48000
	}
	// Time constant: coeff = 1 - exp(-1 / (timeMs * sampleRate / 1000))
	// For frame-based processing with 20ms frames:
	frameRate := float64(cfg.SampleRate) / 960.0 // ~50 fps at 48kHz
	attack := 1.0 - math.Exp(-1.0/(cfg.AttackMs*frameRate/1000.0))
	release := 1.0 - math.Exp(-1.0/(cfg.ReleaseMs*frameRate/1000.0))

	return &AGCProcessor{
		targetDBFS:   cfg.TargetDBFS,
		maxGain:      cfg.MaxGain,
		attackCoeff:  attack,
		releaseCoeff: release,
		currentGain:  1.0,
		enabled:      true,
	}
}

// Process applies automatic gain control to the samples.
func (a *AGCProcessor) Process(_ context.Context, samples []float32) ([]float32, error) {
	if !a.enabled || len(samples) == 0 {
		return samples, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Compute RMS of frame
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	rms := math.Sqrt(sum / float64(len(samples)))

	if rms < 1e-10 {
		// Silence — don't adjust gain
		return samples, nil
	}

	// Convert to dBFS
	rmsDBFS := 20.0 * math.Log10(rms)

	// Desired gain to reach target
	desiredGainDB := a.targetDBFS - rmsDBFS
	desiredGain := math.Pow(10.0, desiredGainDB/20.0)

	// Clamp to max gain
	if desiredGain > a.maxGain {
		desiredGain = a.maxGain
	}
	if desiredGain < 1.0/a.maxGain {
		desiredGain = 1.0 / a.maxGain
	}

	// Smooth gain transition using attack/release
	if desiredGain < a.currentGain {
		a.currentGain += a.attackCoeff * (desiredGain - a.currentGain)
	} else {
		a.currentGain += a.releaseCoeff * (desiredGain - a.currentGain)
	}

	// Apply gain with clipping protection
	gain := float32(a.currentGain)
	for i := range samples {
		samples[i] *= gain
		if samples[i] > 1.0 {
			samples[i] = 1.0
		} else if samples[i] < -1.0 {
			samples[i] = -1.0
		}
	}

	return samples, nil
}

// Name returns "agc".
func (a *AGCProcessor) Name() string { return "agc" }

// SetEnabled enables or disables AGC at runtime.
func (a *AGCProcessor) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// IsEnabled returns whether AGC is active.
func (a *AGCProcessor) IsEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enabled
}

// CurrentGainDB returns the current gain in dB.
func (a *AGCProcessor) CurrentGainDB() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return 20.0 * math.Log10(a.currentGain)
}
