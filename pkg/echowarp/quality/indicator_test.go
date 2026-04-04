package quality

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityLevel_String(t *testing.T) {
	tests := []struct {
		level    QualityLevel
		expected string
	}{
		{QualityExcellent, "Excellent"},
		{QualityGood, "Good"},
		{QualityFair, "Fair"},
		{QualityPoor, "Poor"},
		{QualityLevel(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestQualityLevel_Emoji(t *testing.T) {
	tests := []struct {
		level    QualityLevel
		expected string
	}{
		{QualityExcellent, "🟢"},
		{QualityGood, "🟡"},
		{QualityFair, "🟠"},
		{QualityPoor, "🔴"},
		{QualityLevel(99), "⚪"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.Emoji())
		})
	}
}

func TestQualityLevel_ASCII(t *testing.T) {
	tests := []struct {
		level    QualityLevel
		expected string
	}{
		{QualityExcellent, "[====]"},
		{QualityGood, "[=== ]"},
		{QualityFair, "[==  ]"},
		{QualityPoor, "[=   ]"},
		{QualityLevel(99), "[    ]"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.ASCII())
		})
	}
}

func TestNewQualityIndicator(t *testing.T) {
	// Create a mock peer connection (nil for testing)
	qi := NewQualityIndicator(nil)
	assert.NotNil(t, qi)
	assert.NotNil(t, qi.updateChan)
	assert.Equal(t, time.Second, qi.updateFreq)
}

func TestNewQualityIndicator_WithOptions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	qi := NewQualityIndicator(nil,
		WithUpdateFrequency(500*time.Millisecond),
		WithLogger(logger),
	)
	assert.NotNil(t, qi)
	assert.Equal(t, 500*time.Millisecond, qi.updateFreq)
	assert.NotNil(t, qi.logger)
}

func TestNewQualityIndicator_MinUpdateFrequency(t *testing.T) {
	qi := NewQualityIndicator(nil, WithUpdateFrequency(10*time.Millisecond))
	assert.Equal(t, 100*time.Millisecond, qi.updateFreq, "Should enforce minimum update frequency of 100ms")
}

func TestQualityIndicator_GetCurrentQuality(t *testing.T) {
	qi := NewQualityIndicator(nil)
	quality := qi.GetCurrentQuality()
	assert.Equal(t, ConnectionQuality{}, quality, "Should return zero value when no measurements taken")
}

func TestQualityIndicator_Stop(t *testing.T) {
	qi := NewQualityIndicator(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	_ = qi.Start(ctx)

	// Stop should not panic
	qi.Stop()
	qi.Stop() // Safe to call multiple times
}

func TestQualityIndicator_Start_ContextCancellation(t *testing.T) {
	qi := NewQualityIndicator(nil, WithUpdateFrequency(100*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())

	updateCh := qi.Start(ctx)

	// Cancel after a short time
	time.AfterFunc(200*time.Millisecond, cancel)

	// Wait for channel to close
	select {
	case _, ok := <-updateCh:
		if !ok {
			// Channel closed as expected
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Update channel should have been closed")
	}
}

func TestQualityIndicator_collectQuality_NilConnection(t *testing.T) {
	qi := NewQualityIndicator(nil)
	_, err := qi.collectQuality()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "peer connection is nil")
}

func TestQualityIndicator_collectQuality_WithRealConnection(t *testing.T) {
	// Create a real WebRTC peer connection for integration test
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer pc.Close()

	qi := NewQualityIndicator(pc)
	quality, err := qi.collectQuality()

	// Connection is not established, but function should not error
	// It may return zero values or incomplete stats
	assert.NoError(t, err)
	assert.NotNil(t, quality)
	assert.False(t, quality.UpdatedAt.IsZero())
}

func TestQualityIndicator_calculateQualityLevel(t *testing.T) {
	tests := []struct {
		name        string
		latency     time.Duration
		packetLoss  float64
		expectedLvl QualityLevel
	}{
		{"Excellent - perfect", 20 * time.Millisecond, 0.1, QualityExcellent},
		{"Excellent - boundary", 49 * time.Millisecond, 0.9, QualityExcellent},
		{"Good - low", 50 * time.Millisecond, 0.5, QualityGood},
		{"Good - high", 99 * time.Millisecond, 2.9, QualityGood},
		{"Fair - low", 100 * time.Millisecond, 1.5, QualityFair},
		{"Fair - high", 199 * time.Millisecond, 4.9, QualityFair},
		{"Poor - latency", 200 * time.Millisecond, 1.0, QualityPoor},
		{"Poor - packet loss", 50 * time.Millisecond, 5.0, QualityPoor},
		{"Poor - both bad", 300 * time.Millisecond, 10.0, QualityPoor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qi := NewQualityIndicator(nil)
			level := qi.calculateQualityLevel(tt.latency, tt.packetLoss)
			assert.Equal(t, tt.expectedLvl, level)
		})
	}
}

func TestQualityIndicator_FormatForTUI(t *testing.T) {
	qi := NewQualityIndicator(nil)
	quality := ConnectionQuality{
		Latency:    50 * time.Millisecond,
		PacketLoss: 1.5,
		Jitter:     10 * time.Millisecond,
		Quality:    QualityGood,
		UpdatedAt:  time.Now(),
	}

	result := qi.FormatForTUI(quality)
	assert.Contains(t, result, "🟡")
	assert.Contains(t, result, "Good")
	assert.Contains(t, result, "50ms")
	assert.Contains(t, result, "1.5%")
	assert.Contains(t, result, "10ms")
}

func TestQualityIndicator_FormatCompact(t *testing.T) {
	qi := NewQualityIndicator(nil)
	quality := ConnectionQuality{
		Latency:    30 * time.Millisecond,
		PacketLoss: 0.5,
		Quality:    QualityExcellent,
		UpdatedAt:  time.Now(),
	}

	result := qi.FormatCompact(quality)
	assert.Contains(t, result, "🟢")
	assert.Contains(t, result, "[====]")
	assert.Contains(t, result, "30ms")
	assert.Contains(t, result, "0.5%")
}

func TestQualityIndicator_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create real peer connections
	pc1, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer pc1.Close()

	pc2, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	defer pc2.Close()

	// Create quality indicator for first peer
	qi := NewQualityIndicator(pc1, WithUpdateFrequency(100*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	updateCh := qi.Start(ctx)
	defer qi.Stop()

	// Collect at least one update
	select {
	case quality := <-updateCh:
		assert.NotNil(t, quality)
		assert.False(t, quality.UpdatedAt.IsZero())
		t.Logf("Quality update: %s", qi.FormatCompact(quality))
	case <-time.After(2 * time.Second):
		t.Log("No quality updates received (expected for non-connected peers)")
	}
}

func TestMaxInt32(t *testing.T) {
	assert.Equal(t, int32(5), maxInt32(5, 3))
	assert.Equal(t, int32(5), maxInt32(3, 5))
	assert.Equal(t, int32(-1), maxInt32(-1, -5))
	assert.Equal(t, int32(0), maxInt32(-1, 0))
}
