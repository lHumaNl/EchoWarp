package audio

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyQuality_Excellent(t *testing.T) {
	nq := NetworkQuality{
		RoundTrip:   20,
		LossPercent: 0,
		Jitter:      5,
	}
	level := ClassifyQuality(nq)
	assert.Equal(t, QualityExcellent, level)
}

func TestClassifyQuality_Good(t *testing.T) {
	nq := NetworkQuality{
		RoundTrip:   80,
		LossPercent: 2,
		Jitter:      20,
	}
	level := ClassifyQuality(nq)
	assert.Equal(t, QualityGood, level)
}

func TestClassifyQuality_Fair(t *testing.T) {
	nq := NetworkQuality{
		RoundTrip:   150,
		LossPercent: 4,
		Jitter:      40,
	}
	level := ClassifyQuality(nq)
	assert.Equal(t, QualityFair, level)
}

func TestClassifyQuality_Poor(t *testing.T) {
	nq := NetworkQuality{
		RoundTrip:   300,
		LossPercent: 10,
		Jitter:      80,
	}
	level := ClassifyQuality(nq)
	assert.Equal(t, QualityPoor, level)
}

func TestAdaptive_ExcellentQuality_IncreasesBitrate(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	initialBitrate := ctrl.CurrentBitrate()

	excellent := NetworkQuality{
		RoundTrip:   20,
		LossPercent: 0,
		Jitter:      5,
	}

	for i := 0; i < 10; i++ {
		ctrl.ReportQuality(excellent)
	}

	finalBitrate := ctrl.CurrentBitrate()
	assert.Greater(t, finalBitrate, initialBitrate, "bitrate should increase after excellent quality reports")
}

func TestAdaptive_PoorQuality_DecreasesBitrate(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	initialBitrate := ctrl.CurrentBitrate()

	poor := NetworkQuality{
		RoundTrip:   300,
		LossPercent: 10,
		Jitter:      80,
	}

	for i := 0; i < 10; i++ {
		ctrl.ReportQuality(poor)
	}

	finalBitrate := ctrl.CurrentBitrate()
	assert.Less(t, finalBitrate, initialBitrate, "bitrate should decrease significantly after poor quality reports")
}

func TestAdaptive_Bitrate_NeverExceedsMax(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	excellent := NetworkQuality{
		RoundTrip:   10,
		LossPercent: 0,
		Jitter:      1,
	}

	for i := 0; i < 50; i++ {
		ctrl.ReportQuality(excellent)
	}

	finalBitrate := ctrl.CurrentBitrate()
	assert.LessOrEqual(t, finalBitrate, br.Max, "bitrate should never exceed max")
}

func TestAdaptive_Bitrate_NeverBelowMin(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	poor := NetworkQuality{
		RoundTrip:   500,
		LossPercent: 50,
		Jitter:      200,
	}

	for i := 0; i < 50; i++ {
		ctrl.ReportQuality(poor)
	}

	finalBitrate := ctrl.CurrentBitrate()
	assert.GreaterOrEqual(t, finalBitrate, br.Min, "bitrate should never go below min")
}

func TestAdaptive_GoodQuality_MaintainsBitrate(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	initialBitrate := ctrl.CurrentBitrate()

	good := NetworkQuality{
		RoundTrip:   80,
		LossPercent: 2,
		Jitter:      20,
	}

	for i := 0; i < 10; i++ {
		ctrl.ReportQuality(good)
	}

	finalBitrate := ctrl.CurrentBitrate()
	tolerance := float64(initialBitrate) * 0.15
	diff := float64(abs(finalBitrate - initialBitrate))
	assert.LessOrEqual(t, diff, tolerance, "bitrate should stay approximately the same with good quality")
}

func TestAdaptive_CooldownPreventsRapidChanges(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	ctrl.cooldown = 100 * time.Millisecond

	poor := NetworkQuality{
		RoundTrip:   300,
		LossPercent: 10,
		Jitter:      80,
	}

	bitrate1 := ctrl.ReportQuality(poor)

	time.Sleep(10 * time.Millisecond)
	bitrate2 := ctrl.ReportQuality(poor)

	assert.Equal(t, bitrate1, bitrate2, "rapid reports within cooldown should not change bitrate")

	time.Sleep(150 * time.Millisecond)
	bitrate3 := ctrl.ReportQuality(poor)
	assert.NotEqual(t, bitrate2, bitrate3, "report after cooldown should change bitrate")
}

func TestAdaptive_Reset_RestoresDefault(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	initialBitrate := ctrl.CurrentBitrate()

	poor := NetworkQuality{
		RoundTrip:   300,
		LossPercent: 10,
		Jitter:      80,
	}

	for i := 0; i < 10; i++ {
		ctrl.ReportQuality(poor)
	}

	changedBitrate := ctrl.CurrentBitrate()
	assert.NotEqual(t, initialBitrate, changedBitrate, "bitrate should have changed after reports")

	require.NoError(t, ctrl.Reset(), "Reset() should not return error")

	resetBitrate := ctrl.CurrentBitrate()
	assert.Equal(t, initialBitrate, resetBitrate, "Reset() should restore initial bitrate")
}

func TestDefaultBitrateRange(t *testing.T) {
	br := DefaultBitrateRange()
	assert.Equal(t, 16000, br.Min, "default min should be 16000")
	assert.Equal(t, 128000, br.Max, "default max should be 128000")
}

func TestClassifyQuality_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		nq       NetworkQuality
		expected QualityLevel
	}{
		{
			name:     "excellent at threshold",
			nq:       NetworkQuality{RoundTrip: 49, LossPercent: 0.9, Jitter: 9},
			expected: QualityExcellent,
		},
		{
			name:     "just above excellent becomes good",
			nq:       NetworkQuality{RoundTrip: 50, LossPercent: 0.9, Jitter: 9},
			expected: QualityGood,
		},
		{
			name:     "good at threshold",
			nq:       NetworkQuality{RoundTrip: 99, LossPercent: 2.9, Jitter: 29},
			expected: QualityGood,
		},
		{
			name:     "just above good becomes fair",
			nq:       NetworkQuality{RoundTrip: 100, LossPercent: 2.9, Jitter: 29},
			expected: QualityFair,
		},
		{
			name:     "fair at threshold",
			nq:       NetworkQuality{RoundTrip: 199, LossPercent: 4.9, Jitter: 49},
			expected: QualityFair,
		},
		{
			name:     "just above fair becomes poor",
			nq:       NetworkQuality{RoundTrip: 200, LossPercent: 4.9, Jitter: 49},
			expected: QualityPoor,
		},
		{
			name:     "poor quality",
			nq:       NetworkQuality{RoundTrip: 300, LossPercent: 10, Jitter: 100},
			expected: QualityPoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := ClassifyQuality(tt.nq)
			assert.Equal(t, tt.expected, level)
		})
	}
}

func TestAdaptive_ConcurrentReportAndRead(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, logger)

	ctrl.cooldown = 0

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			nq := NetworkQuality{RoundTrip: 50, LossPercent: 0.5, Jitter: 5}
			ctrl.ReportQuality(nq)
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			_ = ctrl.CurrentBitrate()
		}()
	}

	wg.Wait()
}

func TestAdaptive_NilLogger_NoPanic(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(enc, br, nil)

	nq := NetworkQuality{RoundTrip: 50, LossPercent: 0.5, Jitter: 5}

	assert.NotPanics(t, func() {
		ctrl.ReportQuality(nq)
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestBitrateRange_Validate_Valid(t *testing.T) {
	br := BitrateRange{Min: 16000, Max: 128000}
	assert.NoError(t, br.Validate())
}

func TestBitrateRange_Validate_ZeroMin(t *testing.T) {
	br := BitrateRange{Min: 0, Max: 128000}
	assert.Error(t, br.Validate())
}

func TestBitrateRange_Validate_ZeroMax(t *testing.T) {
	br := BitrateRange{Min: 16000, Max: 0}
	assert.Error(t, br.Validate())
}

func TestBitrateRange_Validate_NegativeMin(t *testing.T) {
	br := BitrateRange{Min: -1000, Max: 128000}
	assert.Error(t, br.Validate())
}

func TestBitrateRange_Validate_MinExceedsMax(t *testing.T) {
	br := BitrateRange{Min: 200000, Max: 128000}
	assert.Error(t, br.Validate())
}

func TestQualityLevel_String_AllLevels(t *testing.T) {
	tests := []struct {
		level    QualityLevel
		expected string
	}{
		{QualityExcellent, "excellent"},
		{QualityGood, "good"},
		{QualityFair, "fair"},
		{QualityPoor, "poor"},
		{QualityLevel(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestAdaptiveBitrateController_NilEncoder(t *testing.T) {
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(nil, br, nil)

	assert.NotNil(t, ctrl)

	initial := ctrl.CurrentBitrate()
	assert.Equal(t, (br.Min+br.Max)/2, initial)

	ctrl.ReportQuality(NetworkQuality{RoundTrip: 20, LossPercent: 0, Jitter: 5})

	assert.Equal(t, initial, ctrl.CurrentBitrate())
}

func TestAdaptiveBitrateController_Reset_NilEncoder(t *testing.T) {
	br := BitrateRange{Min: 16000, Max: 128000}
	ctrl := NewAdaptiveBitrateController(nil, br, nil)

	err := ctrl.Reset()
	assert.NoError(t, err)
}

func TestAdaptiveBitrateController_InvalidBitrateRange_UsesDefault(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	invalidBr := BitrateRange{Min: 0, Max: 0}
	ctrl := NewAdaptiveBitrateController(enc, invalidBr, nil)

	defaultBr := DefaultBitrateRange()
	expected := (defaultBr.Min + defaultBr.Max) / 2
	assert.Equal(t, expected, ctrl.CurrentBitrate())
}

func TestAdaptive_FairQuality_DecreasesBitrate(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	initialBitrate := ctrl.CurrentBitrate()

	fair := NetworkQuality{
		RoundTrip:   150,
		LossPercent: 4,
		Jitter:      40,
	}

	for i := 0; i < 10; i++ {
		ctrl.ReportQuality(fair)
	}

	finalBitrate := ctrl.CurrentBitrate()
	assert.Less(t, finalBitrate, initialBitrate, "bitrate should decrease after fair quality reports")
}

func TestAdaptive_ComputeAverageQuality_Empty(t *testing.T) {
	ctrl := NewAdaptiveBitrateController(nil, BitrateRange{Min: 16000, Max: 128000}, nil)
	avg := ctrl.computeAverageQuality()
	assert.Equal(t, NetworkQuality{}, avg)
}

func TestAdaptive_ReportQuality_HistoryLimit(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	for i := 0; i < 20; i++ {
		ctrl.ReportQuality(NetworkQuality{RoundTrip: float64(i), LossPercent: 0, Jitter: 0})
	}

	assert.LessOrEqual(t, len(ctrl.history), ctrl.maxHistory)
}

func BenchmarkAdaptiveBitrate_ReportQuality(b *testing.B) {
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	nq := NetworkQuality{RoundTrip: 50, LossPercent: 1.5, Jitter: 15}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl.ReportQuality(nq)
	}
}

func BenchmarkAdaptiveBitrate_ReportQuality_WithCooldown(b *testing.B) {
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)

	nq := NetworkQuality{RoundTrip: 50, LossPercent: 1.5, Jitter: 15}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl.ReportQuality(nq)
	}
}

func BenchmarkAdaptiveBitrate_ReportQuality_NoEncoder(b *testing.B) {
	ctrl := NewAdaptiveBitrateController(nil, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	nq := NetworkQuality{RoundTrip: 50, LossPercent: 1.5, Jitter: 15}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl.ReportQuality(nq)
	}
}

func BenchmarkClassifyQuality(b *testing.B) {
	qualities := []NetworkQuality{
		{RoundTrip: 20, LossPercent: 0.5, Jitter: 5},
		{RoundTrip: 80, LossPercent: 2, Jitter: 20},
		{RoundTrip: 150, LossPercent: 4, Jitter: 40},
		{RoundTrip: 300, LossPercent: 10, Jitter: 80},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyQuality(qualities[i%len(qualities)])
	}
}

func BenchmarkAdaptiveBitrate_ComputeAverageQuality(b *testing.B) {
	ctrl := &AdaptiveBitrateController{
		history:    make([]NetworkQuality, 10),
		maxHistory: 10,
	}
	for i := range ctrl.history {
		ctrl.history[i] = NetworkQuality{RoundTrip: 50, LossPercent: 1, Jitter: 10}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl.computeAverageQuality()
	}
}

func BenchmarkAdaptiveBitrate_FullCycle(b *testing.B) {
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	qualities := []NetworkQuality{
		{RoundTrip: 20, LossPercent: 0.5, Jitter: 5},
		{RoundTrip: 80, LossPercent: 2, Jitter: 20},
		{RoundTrip: 150, LossPercent: 4, Jitter: 40},
		{RoundTrip: 300, LossPercent: 10, Jitter: 80},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl.ReportQuality(qualities[i%len(qualities)])
	}
}

func BenchmarkAdaptiveBitrate_Concurrent(b *testing.B) {
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	ctrl := NewAdaptiveBitrateController(enc, BitrateRange{Min: 16000, Max: 128000}, nil)
	ctrl.cooldown = 0

	nq := NetworkQuality{RoundTrip: 50, LossPercent: 1.5, Jitter: 15}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctrl.ReportQuality(nq)
		}
	})
}
