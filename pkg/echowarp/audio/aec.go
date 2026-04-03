package audio

import (
	"context"
	"sync"
)

// AECProcessor implements acoustic echo cancellation using an adaptive filter.
// It removes the far-end (reference) signal from the near-end (captured) audio.
//
// Usage:
//  1. Feed the far-end audio (what's being played to the speaker) via FeedReference.
//  2. Process() subtracts the estimated echo from the near-end capture signal.
//
// The adaptive filter uses Normalized Least Mean Squares (NLMS) algorithm.
type AECProcessor struct {
	mu sync.Mutex

	// Adaptive filter state
	filterLen int       // number of taps
	weights   []float32 // adaptive filter coefficients
	refBuf    []float32 // circular buffer of reference samples
	refPos    int       // write position in refBuf

	// NLMS parameters
	stepSize float32 // μ: adaptation rate (0.0–1.0, typical 0.1–0.5)
	regParam float32 // δ: regularization to prevent division by zero

	enabled bool
}

// AECConfig holds configuration for the echo canceller.
type AECConfig struct {
	// FilterLength is the number of filter taps. Longer = more echo tail coverage.
	// At 48kHz, 960 taps = 20ms, 4800 taps = 100ms echo tail.
	FilterLength int
	// StepSize controls adaptation speed. Higher = faster but less stable.
	// Typical range: 0.1–0.5
	StepSize float32
}

// DefaultAECConfig returns a reasonable default for duplex audio at 48kHz.
func DefaultAECConfig() AECConfig {
	return AECConfig{
		FilterLength: 4800, // 100ms at 48kHz — covers typical room echo
		StepSize:     0.3,
	}
}

// NewAECProcessor creates an echo canceller with the given config.
func NewAECProcessor(cfg AECConfig) *AECProcessor {
	if cfg.FilterLength <= 0 {
		cfg.FilterLength = 4800
	}
	if cfg.StepSize <= 0 || cfg.StepSize > 1.0 {
		cfg.StepSize = 0.3
	}
	return &AECProcessor{
		filterLen: cfg.FilterLength,
		weights:   make([]float32, cfg.FilterLength),
		refBuf:    make([]float32, cfg.FilterLength),
		stepSize:  cfg.StepSize,
		regParam:  1e-6,
		enabled:   true,
	}
}

// FeedReference provides far-end audio samples (the signal being played back).
// Must be called synchronously with playback — typically from the playback pipeline.
func (a *AECProcessor) FeedReference(samples []float32) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, s := range samples {
		a.refBuf[a.refPos] = s
		a.refPos = (a.refPos + 1) % a.filterLen
	}
}

// Process removes estimated echo from the near-end capture signal.
func (a *AECProcessor) Process(_ context.Context, samples []float32) ([]float32, error) {
	if !a.enabled || len(samples) == 0 {
		return samples, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// readPos tracks alignment: starts at the oldest sample that was
	// most recently fed (refPos - len(last FeedReference call)),
	// and advances per processed sample.
	// Since FeedReference may have been called with a block of N samples,
	// the first near-end sample aligns with reference at (refPos - N),
	// but we approximate by using (refPos - len(samples)).
	readPos := (a.refPos - len(samples) + a.filterLen*2) % a.filterLen

	for i := range samples {
		// Current reference alignment point
		newest := (readPos + i) % a.filterLen

		// Compute estimated echo: y_hat = w^T * x
		var echoEst float32
		var refPower float32
		for j := 0; j < a.filterLen; j++ {
			idx := (newest - j + a.filterLen) % a.filterLen
			ref := a.refBuf[idx]
			echoEst += a.weights[j] * ref
			refPower += ref * ref
		}

		// Error signal: near-end minus estimated echo
		errSig := samples[i] - echoEst

		// NLMS weight update: w += μ * e * x / (x^T * x + δ)
		norm := a.stepSize / (refPower + a.regParam)
		for j := 0; j < a.filterLen; j++ {
			idx := (newest - j + a.filterLen) % a.filterLen
			a.weights[j] += norm * errSig * a.refBuf[idx]
		}

		samples[i] = errSig
	}

	return samples, nil
}

// Name returns "aec".
func (a *AECProcessor) Name() string { return "aec" }

// SetEnabled enables or disables the AEC processor at runtime.
func (a *AECProcessor) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// IsEnabled returns whether AEC is currently active.
func (a *AECProcessor) IsEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.enabled
}

// Reset clears the adaptive filter state (e.g., after reconnection).
func (a *AECProcessor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.weights {
		a.weights[i] = 0
	}
	for i := range a.refBuf {
		a.refBuf[i] = 0
	}
	a.refPos = 0
}
