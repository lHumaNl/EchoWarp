package audio

import (
	"math"
	"math/cmplx"
	"sync"

	"github.com/madelynnblue/go-dsp/fft"
)

const (
	// DefaultBands is the number of frequency bands for the spectrum display.
	DefaultBands = 16
	// fftSize is the FFT window size (must be power of 2).
	fftSize = 1024
	// smoothingFactor controls exponential smoothing (0 = no smoothing, 1 = frozen).
	smoothingFactor = 0.3
)

// bandRange defines a frequency range for one visual band.
type bandRange struct {
	lo, hi float64 // Hz
}

// SpectrumAnalyzer computes frequency band magnitudes from PCM audio via FFT.
// Thread-safe: Feed() can be called from the audio pipeline goroutine,
// Bands() can be called from the TUI goroutine.
type SpectrumAnalyzer struct {
	mu         sync.Mutex
	bands      []float64 // smoothed magnitudes per band, 0.0–1.0
	ranges     []bandRange
	sampleRate float64
	window     []float64 // Hann window coefficients
	numBands   int
	fftBuf     []float64 // reusable FFT input buffer (fftSize)
	bandBuf    []float64 // reusable band computation buffer (numBands)
}

// NewSpectrumAnalyzer creates a spectrum analyzer with the given sample rate and band count.
func NewSpectrumAnalyzer(sampleRate uint32, numBands int) *SpectrumAnalyzer {
	if numBands <= 0 {
		numBands = DefaultBands
	}

	sr := float64(sampleRate)
	ranges := buildLogBands(numBands, 20.0, sr/2)

	// Pre-compute Hann window
	window := make([]float64, fftSize)
	for i := range window {
		window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}

	return &SpectrumAnalyzer{
		bands:      make([]float64, numBands),
		ranges:     ranges,
		sampleRate: sr,
		window:     window,
		numBands:   numBands,
		fftBuf:     make([]float64, fftSize),
		bandBuf:    make([]float64, numBands),
	}
}

// buildLogBands creates logarithmically spaced frequency band ranges.
func buildLogBands(n int, minHz, maxHz float64) []bandRange {
	ranges := make([]bandRange, n)
	logMin := math.Log10(minHz)
	logMax := math.Log10(maxHz)
	step := (logMax - logMin) / float64(n)
	for i := range ranges {
		ranges[i] = bandRange{
			lo: math.Pow(10, logMin+step*float64(i)),
			hi: math.Pow(10, logMin+step*float64(i+1)),
		}
	}
	return ranges
}

// Feed processes a PCM float32 buffer and updates the frequency bands.
// Called from the audio pipeline. Uses the last fftSize samples.
func (sa *SpectrumAnalyzer) Feed(samples []float32) {
	n := len(samples)
	if n == 0 {
		return
	}

	sa.mu.Lock()
	defer sa.mu.Unlock()

	// Take the last fftSize samples (or zero-pad if fewer)
	input := sa.fftBuf
	for i := range input {
		input[i] = 0
	}
	start := 0
	if n > fftSize {
		start = n - fftSize
	}
	offset := fftSize - min(n, fftSize)
	for i := start; i < n && (i-start+offset) < fftSize; i++ {
		input[i-start+offset] = float64(samples[i]) * sa.window[i-start+offset]
	}

	// FFT
	spectrum := fft.FFTReal(input)

	// Compute magnitudes and map to bands
	freqRes := sa.sampleRate / float64(fftSize)
	halfN := fftSize / 2
	newBands := sa.bandBuf
	for i := range newBands {
		newBands[i] = 0
	}

	for b, r := range sa.ranges {
		binLo := int(r.lo / freqRes)
		binHi := int(r.hi / freqRes)
		if binLo < 1 {
			binLo = 1
		}
		if binHi > halfN {
			binHi = halfN
		}
		if binLo > binHi {
			continue
		}

		var sum float64
		count := 0
		for i := binLo; i <= binHi && i < len(spectrum); i++ {
			mag := cmplx.Abs(spectrum[i])
			sum += mag
			count++
		}
		if count > 0 {
			newBands[b] = sum / float64(count)
		}
	}

	// Normalize: convert to dB scale, map to 0.0–1.0
	// Reference: fftSize/4 — Hann window halves the amplitude of a full-scale sine
	ref := float64(fftSize) / 4
	for i := range newBands {
		if newBands[i] <= 0 {
			continue
		}
		db := 20 * math.Log10(newBands[i]/ref)
		// Map -60dB..0dB → 0.0..1.0
		normalized := (db + 60) / 60
		if normalized < 0 {
			normalized = 0
		}
		if normalized > 1 {
			normalized = 1
		}
		newBands[i] = normalized
	}

	// Exponential smoothing
	for i := range sa.bands {
		sa.bands[i] = sa.bands[i]*smoothingFactor + newBands[i]*(1-smoothingFactor)
	}
}

// Bands returns a copy of the current frequency band magnitudes (0.0–1.0).
func (sa *SpectrumAnalyzer) Bands() []float64 {
	sa.mu.Lock()
	result := make([]float64, len(sa.bands))
	copy(result, sa.bands)
	sa.mu.Unlock()
	return result
}

// NumBands returns the number of frequency bands (immutable after construction).
func (sa *SpectrumAnalyzer) NumBands() int {
	return sa.numBands
}
