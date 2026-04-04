package audio

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// BitrateRange defines the minimum and maximum bitrate for adaptive streaming.
type BitrateRange struct {
	Min int
	Max int
}

// DefaultBitrateRange returns the recommended bitrate range for voice communication:
// 16 kbps minimum for intelligible speech, 128 kbps maximum for high quality.
func DefaultBitrateRange() BitrateRange {
	return BitrateRange{Min: 16000, Max: 128000}
}

func (br BitrateRange) Validate() error {
	if br.Min <= 0 || br.Max <= 0 {
		return errors.New("bitrate range values must be positive")
	}
	if br.Min > br.Max {
		return errors.New("bitrate min cannot exceed max")
	}
	return nil
}

// NetworkQuality contains measured network metrics used for bitrate adaptation.
type NetworkQuality struct {
	LossPercent float64
	Jitter      float64
	RoundTrip   float64
}

// QualityLevel represents the assessed network quality level.
type QualityLevel int

const (
	// QualityExcellent indicates optimal network conditions (<50ms RTT, <1% loss).
	QualityExcellent QualityLevel = iota
	// QualityGood indicates good network conditions (<100ms RTT, <3% loss).
	QualityGood
	// QualityFair indicates acceptable network conditions (<200ms RTT, <5% loss).
	QualityFair
	// QualityPoor indicates degraded network conditions requiring bitrate reduction.
	QualityPoor
)

// AdaptiveBitrateController adjusts Opus encoder bitrate based on network conditions.
// It uses a smoothing window of recent quality measurements and applies a cooldown
// period between adjustments to prevent oscillation.
//
// Thread-safe: concurrent ReportQuality calls are protected by internal mutex.
type AdaptiveBitrateController struct {
	mu             sync.Mutex
	encoder        *OpusEncoder
	bitrateRange   BitrateRange
	currentBitrate int
	history        []NetworkQuality
	maxHistory     int
	cooldown       time.Duration
	lastAdjust     time.Time
	logger         *slog.Logger
}

// NewAdaptiveBitrateController creates a controller for the given encoder.
// If the bitrate range is invalid, defaults are used.
// Logger can be nil to disable debug logging.
func NewAdaptiveBitrateController(enc *OpusEncoder, br BitrateRange, logger *slog.Logger) *AdaptiveBitrateController {
	if err := br.Validate(); err != nil {
		if logger != nil {
			logger.Warn("invalid bitrate range, using defaults", "error", err, "provided", br)
		}
		br = DefaultBitrateRange()
	}
	startBitrate := (br.Min + br.Max) / 2
	return &AdaptiveBitrateController{
		encoder:        enc,
		bitrateRange:   br,
		currentBitrate: startBitrate,
		history:        make([]NetworkQuality, 0, 10),
		maxHistory:     10,
		cooldown:       2 * time.Second,
		logger:         logger,
	}
}

// ReportQuality submits network metrics and returns the current bitrate.
// Bitrate adjustments are made based on averaged quality over the history window,
// subject to a 2-second cooldown between changes.
func (c *AdaptiveBitrateController) ReportQuality(nq NetworkQuality) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.addToHistory(nq)

	if time.Since(c.lastAdjust) < c.cooldown {
		return c.currentBitrate
	}

	avg := c.computeAverageQuality()
	level := ClassifyQuality(avg)
	newBitrate := c.calculateNewBitrate(level)

	return c.applyNewBitrate(newBitrate, level)
}

func (c *AdaptiveBitrateController) addToHistory(nq NetworkQuality) {
	c.history = append(c.history, nq)
	if len(c.history) > c.maxHistory {
		c.history = c.history[1:]
	}
}

func (c *AdaptiveBitrateController) calculateNewBitrate(level QualityLevel) int {
	newBitrate := c.currentBitrate
	switch level {
	case QualityExcellent:
		newBitrate = int(float64(c.currentBitrate) * 1.10)
		if newBitrate > c.bitrateRange.Max {
			newBitrate = c.bitrateRange.Max
		}
	case QualityFair:
		newBitrate = int(float64(c.currentBitrate) * 0.85)
		if newBitrate < c.bitrateRange.Min {
			newBitrate = c.bitrateRange.Min
		}
	case QualityPoor:
		newBitrate = int(float64(c.currentBitrate) * 0.70)
		if newBitrate < c.bitrateRange.Min {
			newBitrate = c.bitrateRange.Min
		}
	}
	return newBitrate
}

func (c *AdaptiveBitrateController) applyNewBitrate(newBitrate int, level QualityLevel) int {
	if newBitrate == c.currentBitrate || c.encoder == nil {
		return c.currentBitrate
	}

	if err := c.encoder.SetBitrate(newBitrate); err != nil {
		if c.logger != nil {
			c.logger.Warn("failed to set bitrate", "error", err, "bitrate", newBitrate)
		}
		return c.currentBitrate
	}

	c.currentBitrate = newBitrate
	c.lastAdjust = time.Now()
	if c.logger != nil {
		c.logger.Debug("adjusted bitrate",
			"quality", level.String(),
			"new_bitrate", newBitrate,
		)
	}
	return c.currentBitrate
}

func (c *AdaptiveBitrateController) computeAverageQuality() NetworkQuality {
	if len(c.history) == 0 {
		return NetworkQuality{}
	}

	var sum NetworkQuality
	for _, nq := range c.history {
		sum.LossPercent += nq.LossPercent
		sum.Jitter += nq.Jitter
		sum.RoundTrip += nq.RoundTrip
	}

	n := float64(len(c.history))
	return NetworkQuality{
		LossPercent: sum.LossPercent / n,
		Jitter:      sum.Jitter / n,
		RoundTrip:   sum.RoundTrip / n,
	}
}

// CurrentBitrate returns the currently configured encoder bitrate in bps.
func (c *AdaptiveBitrateController) CurrentBitrate() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentBitrate
}

// Reset returns bitrate to the midpoint of the range and clears history.
func (c *AdaptiveBitrateController) Reset() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	newBitrate := (c.bitrateRange.Min + c.bitrateRange.Max) / 2

	if c.encoder != nil {
		if err := c.encoder.SetBitrate(newBitrate); err != nil {
			if c.logger != nil {
				c.logger.Warn("failed to reset bitrate", "error", err)
			}
			return err
		}
	}

	c.currentBitrate = newBitrate
	c.history = c.history[:0]
	c.lastAdjust = time.Time{}
	return nil
}

// ClassifyQuality categorizes network metrics into a quality level.
// Thresholds are based on typical VoIP requirements:
//   - Excellent: <50ms RTT, <1% loss, <10ms jitter
//   - Good: <100ms RTT, <3% loss, <30ms jitter
//   - Fair: <200ms RTT, <5% loss, <50ms jitter
//   - Poor: worse than fair
func ClassifyQuality(nq NetworkQuality) QualityLevel {
	switch {
	case nq.RoundTrip < 50 && nq.LossPercent < 1 && nq.Jitter < 10:
		return QualityExcellent
	case nq.RoundTrip < 100 && nq.LossPercent < 3 && nq.Jitter < 30:
		return QualityGood
	case nq.RoundTrip < 200 && nq.LossPercent < 5 && nq.Jitter < 50:
		return QualityFair
	default:
		return QualityPoor
	}
}

// String returns the human-readable name of the quality level.
func (q QualityLevel) String() string {
	switch q {
	case QualityExcellent:
		return "excellent"
	case QualityGood:
		return "good"
	case QualityFair:
		return "fair"
	case QualityPoor:
		return "poor"
	default:
		return "unknown"
	}
}
