package audio

import (
	"math"
	"sync"
)

const (
	// levelSmoothingFactor controls exponential smoothing for VU levels (0 = instant, 1 = frozen).
	levelSmoothingFactor = 0.3
)

// LevelMeter computes per-channel RMS levels from interleaved PCM audio.
// Thread-safe: Feed() from the audio goroutine, Levels() from the TUI goroutine.
type LevelMeter struct {
	mu       sync.Mutex
	levels   []float64 // smoothed RMS per channel, 0.0–1.0
	channels int
	rmsBuf   []float64 // reusable RMS accumulator
	countBuf []int     // reusable sample count per channel
}

// NewLevelMeter creates a level meter for the given number of channels.
func NewLevelMeter(channels uint32) *LevelMeter {
	ch := int(channels)
	if ch < 1 {
		ch = 1
	}
	return &LevelMeter{
		levels:   make([]float64, ch),
		channels: ch,
		rmsBuf:   make([]float64, ch),
		countBuf: make([]int, ch),
	}
}

// Feed processes interleaved PCM float32 samples and updates per-channel RMS levels.
func (lm *LevelMeter) Feed(samples []float32) {
	n := len(samples)
	if n == 0 {
		return
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	ch := lm.channels
	rms := lm.rmsBuf
	counts := lm.countBuf
	for i := range rms {
		rms[i] = 0
	}
	for i := range counts {
		counts[i] = 0
	}

	for i := 0; i < n; i++ {
		c := i % ch
		v := float64(samples[i])
		rms[c] += v * v
		counts[c]++
	}

	for c := 0; c < ch; c++ {
		if counts[c] > 0 {
			rms[c] = math.Sqrt(rms[c] / float64(counts[c]))
			// Convert to dB, map -60dB..0dB → 0.0..1.0
			if rms[c] > 0 {
				db := 20 * math.Log10(rms[c])
				rms[c] = (db + 60) / 60
				if rms[c] < 0 {
					rms[c] = 0
				}
				if rms[c] > 1 {
					rms[c] = 1
				}
			}
		}
	}

	for c := 0; c < ch; c++ {
		lm.levels[c] = lm.levels[c]*levelSmoothingFactor + rms[c]*(1-levelSmoothingFactor)
	}
}

// Levels returns a copy of the current per-channel levels (0.0–1.0).
func (lm *LevelMeter) Levels() []float64 {
	lm.mu.Lock()
	result := make([]float64, len(lm.levels))
	copy(result, lm.levels)
	lm.mu.Unlock()
	return result
}

// Channels returns the number of channels.
func (lm *LevelMeter) Channels() int {
	return lm.channels
}
