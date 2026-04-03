// Package quality provides real-time connection quality monitoring for WebRTC connections.
// It collects network statistics, calculates quality levels, and provides visual
// representations for terminal user interfaces.
package quality

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// ConnectionQuality represents a snapshot of connection quality metrics.
type ConnectionQuality struct {
	// Latency is the round-trip time measured in milliseconds.
	Latency time.Duration
	// Jitter is the variation in packet arrival times in milliseconds.
	Jitter time.Duration
	// PacketLoss is the percentage of lost packets (0-100).
	PacketLoss float64
	// BytesPerSec is the current throughput in bytes per second.
	BytesPerSec uint64
	// Bitrate is the current bitrate in bits per second.
	Bitrate uint64
	// Quality is the calculated quality level.
	Quality QualityLevel
	// UpdatedAt is when these metrics were collected.
	UpdatedAt time.Time
}

// QualityLevel represents the assessed network quality level.
type QualityLevel int

const (
	// QualityExcellent indicates optimal network conditions.
	// Criteria: latency < 50ms, packet loss < 1%
	QualityExcellent QualityLevel = iota
	// QualityGood indicates good network conditions.
	// Criteria: latency < 100ms, packet loss < 3%
	QualityGood
	// QualityFair indicates acceptable network conditions.
	// Criteria: latency < 200ms, packet loss < 5%
	QualityFair
	// QualityPoor indicates degraded network conditions.
	// Criteria: latency >= 200ms or packet loss >= 5%
	QualityPoor
)

// String returns a human-readable representation of the quality level.
func (q QualityLevel) String() string {
	switch q {
	case QualityExcellent:
		return "Excellent"
	case QualityGood:
		return "Good"
	case QualityFair:
		return "Fair"
	case QualityPoor:
		return "Poor"
	default:
		return "Unknown"
	}
}

// Emoji returns an emoji representation of the quality level.
func (q QualityLevel) Emoji() string {
	switch q {
	case QualityExcellent:
		return "🟢"
	case QualityGood:
		return "🟡"
	case QualityFair:
		return "🟠"
	case QualityPoor:
		return "🔴"
	default:
		return "⚪"
	}
}

// ASCII returns an ASCII bar representation of the quality level.
func (q QualityLevel) ASCII() string {
	switch q {
	case QualityExcellent:
		return "[====]"
	case QualityGood:
		return "[=== ]"
	case QualityFair:
		return "[==  ]"
	case QualityPoor:
		return "[=   ]"
	default:
		return "[    ]"
	}
}

// QualityIndicator monitors WebRTC connection quality in real-time.
// It periodically collects statistics from the peer connection and
// calculates quality levels based on latency, jitter, and packet loss.
//
// Thread-safe: concurrent access to current quality is protected by mutex.
type QualityIndicator struct {
	pc            *webrtc.PeerConnection
	updateChan    chan ConnectionQuality
	current       ConnectionQuality
	mu            sync.RWMutex
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        *slog.Logger
	updateFreq    time.Duration
	lastBytesRecv uint64
	lastBytesSent uint64
	lastUpdate    time.Time
	closeOnce     sync.Once
}

// QualityIndicatorOption configures a QualityIndicator.
type QualityIndicatorOption func(*QualityIndicator)

// WithUpdateFrequency sets how often quality metrics are collected.
// Default is 1 second. Minimum is 100ms.
func WithUpdateFrequency(d time.Duration) QualityIndicatorOption {
	return func(q *QualityIndicator) {
		if d < 100*time.Millisecond {
			d = 100 * time.Millisecond
		}
		q.updateFreq = d
	}
}

// WithLogger sets a custom logger for the quality indicator.
func WithLogger(logger *slog.Logger) QualityIndicatorOption {
	return func(q *QualityIndicator) {
		q.logger = logger
	}
}

// NewQualityIndicator creates a new quality indicator for monitoring a WebRTC connection.
// The peer connection must be valid and non-nil.
func NewQualityIndicator(peerConnection *webrtc.PeerConnection, opts ...QualityIndicatorOption) *QualityIndicator {
	q := &QualityIndicator{
		pc:         peerConnection,
		updateChan: make(chan ConnectionQuality, 10),
		updateFreq: 1 * time.Second,
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(q)
	}

	return q
}

// Start begins monitoring the connection quality.
// Returns a channel that receives quality updates at the configured frequency.
// The channel is closed when Stop() is called or the context is canceled.
func (q *QualityIndicator) Start(ctx context.Context) <-chan ConnectionQuality {
	ctx, cancel := context.WithCancel(ctx)
	q.cancel = cancel

	q.wg.Add(1)
	go q.monitor(ctx)

	return q.updateChan
}

// Stop halts quality monitoring and closes the update channel.
// Safe to call multiple times.
func (q *QualityIndicator) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
}

// GetCurrentQuality returns the most recent quality measurement.
// Returns zero value if no measurements have been taken yet.
func (q *QualityIndicator) GetCurrentQuality() ConnectionQuality {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.current
}

// monitor periodically collects WebRTC statistics and calculates quality.
func (q *QualityIndicator) monitor(ctx context.Context) {
	defer q.wg.Done()
	defer q.closeOnce.Do(func() { close(q.updateChan) })

	ticker := time.NewTicker(q.updateFreq)
	defer ticker.Stop()

	// Initial delay to allow connection to stabilize
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			quality, err := q.collectQuality()
			if err != nil {
				if q.logger != nil {
					q.logger.Debug("failed to collect quality metrics", "error", err)
				}
				continue
			}

			q.mu.Lock()
			q.current = quality
			q.mu.Unlock()

			// Send update (non-blocking)
			select {
			case q.updateChan <- quality:
			default:
				// Channel full, skip this update
			}

			// Update Prometheus metrics
			q.updateMetrics(quality)
		}
	}
}

// collectQuality gathers WebRTC statistics and calculates quality level.
func (q *QualityIndicator) collectQuality() (ConnectionQuality, error) {
	if q.pc == nil {
		return ConnectionQuality{}, fmt.Errorf("peer connection is nil")
	}

	stats := q.pc.GetStats()

	quality := ConnectionQuality{
		UpdatedAt: time.Now(),
	}

	var currentRTT float64
	var currentJitter float64
	var packetsLost int32
	var packetsReceived uint32
	var bytesRecv uint64
	var bytesSent uint64

	// Parse WebRTC stats by iterating through the stats map
	for _, stat := range stats {
		switch s := stat.(type) {
		case webrtc.ICECandidatePairStats:
			// Only use the nominated/active candidate pair
			if s.Nominated && s.State == webrtc.StatsICECandidatePairStateSucceeded {
				// RTT from candidate pair (convert seconds to milliseconds)
				if s.CurrentRoundTripTime > 0 {
					currentRTT = s.CurrentRoundTripTime * 1000
				}
				bytesRecv = s.BytesReceived
				bytesSent = s.BytesSent
			}

		case webrtc.InboundRTPStreamStats:
			// Jitter and packet loss from inbound RTP (convert seconds to milliseconds)
			if s.Kind == "audio" {
				currentJitter = s.Jitter * 1000
				packetsLost = s.PacketsLost
				packetsReceived = s.PacketsReceived
			}

		case webrtc.OutboundRTPStreamStats:
			// Could add outbound stats if needed
		}
	}

	// Calculate metrics
	quality.Latency = time.Duration(currentRTT) * time.Millisecond
	quality.Jitter = time.Duration(currentJitter) * time.Millisecond

	// Calculate packet loss percentage
	// Note: packetsLost can be negative if more packets received than sent
	if packetsReceived > 0 {
		totalPackets := float64(packetsReceived) + float64(maxInt32(0, packetsLost))
		if totalPackets > 0 {
			quality.PacketLoss = (float64(maxInt32(0, packetsLost)) / totalPackets) * 100
		}
	}

	// Calculate throughput
	if !q.lastUpdate.IsZero() {
		elapsed := time.Since(q.lastUpdate).Seconds()
		if elapsed > 0 {
			bytesDiff := (bytesRecv - q.lastBytesRecv) + (bytesSent - q.lastBytesSent)
			quality.BytesPerSec = uint64(float64(bytesDiff) / elapsed)
			quality.Bitrate = quality.BytesPerSec * 8
		}
	}

	q.lastBytesRecv = bytesRecv
	q.lastBytesSent = bytesSent
	q.lastUpdate = quality.UpdatedAt

	// Calculate quality level
	quality.Quality = q.calculateQualityLevel(quality.Latency, quality.PacketLoss)

	return quality, nil
}

// calculateQualityLevel determines the quality level based on latency and packet loss.
func (q *QualityIndicator) calculateQualityLevel(latency time.Duration, packetLoss float64) QualityLevel {
	latencyMs := latency.Milliseconds()

	// Check for poor conditions first
	if latencyMs >= 200 || packetLoss >= 5 {
		return QualityPoor
	}

	// Then check for fair conditions
	if latencyMs >= 100 || packetLoss >= 3 {
		return QualityFair
	}

	// Check for good conditions
	if latencyMs >= 50 || packetLoss >= 1 {
		return QualityGood
	}

	// Excellent conditions
	return QualityExcellent
}

// updateMetrics updates Prometheus metrics with the current quality data.
func (q *QualityIndicator) updateMetrics(quality ConnectionQuality) {
	// Update audio latency metric
	metrics.AudioLatency.Observe(quality.Latency.Seconds())

	// Update connection quality metrics
	metrics.ConnectionLatency.Set(float64(quality.Latency.Milliseconds()))
	metrics.ConnectionJitter.Set(float64(quality.Jitter.Milliseconds()))
	metrics.ConnectionPacketLoss.Set(quality.PacketLoss)
	metrics.ConnectionBitrate.Set(float64(quality.Bitrate))
	metrics.ConnectionQualityLevel.Set(float64(quality.Quality))
}

// FormatForTUI returns a formatted string suitable for TUI display.
// Includes quality emoji, level name, and key metrics.
func (q *QualityIndicator) FormatForTUI(quality ConnectionQuality) string {
	return fmt.Sprintf("%s %s | Latency: %dms | Loss: %.1f%% | Jitter: %dms",
		quality.Quality.Emoji(),
		quality.Quality.String(),
		quality.Latency.Milliseconds(),
		quality.PacketLoss,
		quality.Jitter.Milliseconds(),
	)
}

// FormatCompact returns a compact single-line representation.
func (q *QualityIndicator) FormatCompact(quality ConnectionQuality) string {
	return fmt.Sprintf("%s %s %dms/%.1f%%",
		quality.Quality.Emoji(),
		quality.Quality.ASCII(),
		quality.Latency.Milliseconds(),
		quality.PacketLoss,
	)
}

// maxInt32 returns the maximum of two int32 values.
func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
