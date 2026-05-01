package app

import (
	"bytes"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestAudioPCMDiagnosticsScan(t *testing.T) {
	stats := scanPCMFrame([]float32{
		0.5,
		-1.0,
		1.2,
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	})

	assert.InDelta(t, 1.2, stats.PeakAbs, 0.0001)
	assert.Equal(t, uint64(2), stats.ClipSamples)
	assert.Equal(t, uint64(1), stats.NaNSamples)
	assert.Equal(t, uint64(2), stats.InfSamples)
}

func TestPCMContinuityScanFirstFrameHasNoBoundaryJump(t *testing.T) {
	scanner := pcmContinuityScanner{}
	stats := scanner.scan([]float32{0.1, -0.2, 0.3, -0.4}, 2)

	assert.Zero(t, stats.BoundaryJumpMax)
	assert.Zero(t, stats.BoundaryJumpL)
	assert.Zero(t, stats.BoundaryJumpR)
}

func TestPCMContinuityScanBoundaryJumpStereo(t *testing.T) {
	scanner := pcmContinuityScanner{}
	_ = scanner.scan([]float32{0.1, -0.2, 0.3, -0.4}, 2)
	stats := scanner.scan([]float32{0.8, 0.1, -0.2, 0.2}, 2)

	assert.InDelta(t, 0.5, stats.BoundaryJumpMax, 0.0001)
	assert.InDelta(t, 0.5, stats.BoundaryJumpL, 0.0001)
	assert.InDelta(t, 0.5, stats.BoundaryJumpR, 0.0001)
}

func TestPCMContinuityScanRMSAndDCValues(t *testing.T) {
	diagnostics := newClientPlaybackDiagnostics(5)
	diagnostics.recordPCMFrame([]float32{0.2, -0.4, 0.6, -0.8}, 2)
	diagnostics.recordPCMFrame([]float32{0.1, 0.5, -0.3, 0.7}, 2)

	snapshot := diagnostics.snapshotAndResetInterval()

	assert.InDelta(t, 1.3, snapshot.PCMBoundaryJumpMax, 0.0001)
	assert.InDelta(t, 0.5, snapshot.PCMBoundaryJumpL, 0.0001)
	assert.InDelta(t, 1.3, snapshot.PCMBoundaryJumpR, 0.0001)
	assert.InDelta(t, math.Sqrt(0.125), snapshot.PCMRMSL, 0.0001)
	assert.InDelta(t, math.Sqrt(0.385), snapshot.PCMRMSR, 0.0001)
	assert.InDelta(t, 0.15, snapshot.PCMDCOffsetL, 0.0001)
	assert.InDelta(t, 0.0, snapshot.PCMDCOffsetR, 0.0001)
}

func TestPCMContinuityScanHandlesMono(t *testing.T) {
	scanner := pcmContinuityScanner{}
	_ = scanner.scan([]float32{0.25, -0.25}, 1)
	stats := scanner.scan([]float32{0.75, 0.5}, 1)

	assert.InDelta(t, 1.0, stats.BoundaryJumpMax, 0.0001)
	assert.InDelta(t, 1.0, stats.BoundaryJumpL, 0.0001)
	assert.Zero(t, stats.BoundaryJumpR)
	assert.Equal(t, uint64(2), stats.Samples[leftPCMChannel])
	assert.Equal(t, uint64(0), stats.Samples[rightPCMChannel])
}

func TestPCMContinuityScanNaNInfBehavior(t *testing.T) {
	scanner := pcmContinuityScanner{}
	first := scanner.scan([]float32{float32(math.NaN()), float32(math.Inf(1)), 0.5, -0.5}, 2)
	second := scanner.scan([]float32{0.25, float32(math.Inf(-1))}, 2)

	assert.Equal(t, uint64(1), first.NaNSamples)
	assert.Equal(t, uint64(1), first.InfSamples)
	assert.Equal(t, uint64(1), first.Samples[leftPCMChannel])
	assert.Equal(t, uint64(1), first.Samples[rightPCMChannel])
	assert.InDelta(t, 0.25, second.BoundaryJumpMax, 0.0001)
	assert.InDelta(t, 0.25, second.BoundaryJumpL, 0.0001)
	assert.Zero(t, second.BoundaryJumpR)
	assert.Equal(t, uint64(1), second.InfSamples)
	assert.False(t, math.IsNaN(average(second.Sum[leftPCMChannel], second.Samples[leftPCMChannel])))
}

func TestPlaybackDiagnosticsSnapshotAndReset(t *testing.T) {
	diagnostics := newClientPlaybackDiagnostics(5)
	recordDeterministicPumpSamples(diagnostics)

	snapshot := diagnostics.snapshotAndResetInterval()

	assert.Equal(t, int64(6), snapshot.JBDepth)
	assert.Equal(t, int64(1), snapshot.JBDepthMin)
	assert.Equal(t, int64(6), snapshot.JBDepthMax)
	assert.Equal(t, int64(5), snapshot.PlaybackChCap)
	assert.Equal(t, uint64(1), snapshot.JBUnderrunsTotal)
	assert.Equal(t, uint64(1), snapshot.PlayerDropsTotal)
	assert.Equal(t, uint64(7), snapshot.JBDropsTotal)
	assert.Equal(t, 44*time.Millisecond, snapshot.PumpTickGapMax)
	assert.Equal(t, uint64(2), snapshot.PumpLate25MSTotal)
	assert.Equal(t, uint64(1), snapshot.PumpLate40MSTotal)
	assert.InDelta(t, 0.75, snapshot.PCMPeakAbs, 0.0001)
	assert.Equal(t, uint64(2), snapshot.PCMClipSamplesTotal)
	assert.Equal(t, uint64(1), snapshot.PCMNaNSamplesTotal)
	assert.Equal(t, uint64(1), snapshot.PCMInfSamplesTotal)
}

func TestPlaybackDiagnosticsIntervalFieldsReset(t *testing.T) {
	diagnostics := newClientPlaybackDiagnostics(5)
	recordDeterministicPumpSamples(diagnostics)
	_ = diagnostics.snapshotAndResetInterval()

	diagnostics.recordBufferState(4, 1)
	snapshot := diagnostics.snapshotAndResetInterval()

	assert.Equal(t, int64(4), snapshot.JBDepthMin)
	assert.Equal(t, int64(4), snapshot.JBDepthMax)
	assert.Zero(t, snapshot.PumpTickGapMax)
	assert.Zero(t, snapshot.PCMPeakAbs)
	assert.Zero(t, snapshot.PCMBoundaryJumpMax)
	assert.Zero(t, snapshot.PCMRMSL)
}

func TestAudioPlaybackDiagnosticsLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	diagnostics := newClientPlaybackDiagnostics(5)
	diagnostics.recordBufferState(2, 1)
	diagnostics.recordJitterUnderrun()
	diagnostics.recordJitterDrops(7)
	diagnostics.recordPlayerDrop()
	diagnostics.recordPumpGap(45 * time.Millisecond)
	diagnostics.recordPCMStats(pcmFrameStats{
		PeakAbs:         0.5,
		ClipSamples:     2,
		NaNSamples:      1,
		InfSamples:      1,
		BoundaryJumpMax: 0.3,
		BoundaryJumpL:   0.2,
		BoundaryJumpR:   0.3,
		Sum:             [maxPCMContinuityChannels]float64{1, -1},
		SumSquares:      [maxPCMContinuityChannels]float64{0.5, 2},
		Samples:         [maxPCMContinuityChannels]uint64{2, 2},
	})
	player := fakeDiagnosticsPlayer{snapshot: audio.PlayerDiagnosticsSnapshot{
		SilenceFillsTotal:   3,
		ZeroFilledSamples:   12,
		PartialSilenceFills: 2,
		FullSilenceFills:    1,
		CallbackCount:       4,
		CallbackGapMax:      30 * time.Millisecond,
		CallbackLate25MS:    1,
		CallbackLate40MS:    1,
	}}
	previous := &playerDiagnosticsPrevious{silenceFills: 1, zeroSamples: 4}

	logPlayerDiagnosticsTick(logger, player, diagnostics, previous)
	logOutput := buf.String()

	assert.True(t, strings.Contains(logOutput, "Audio playback diagnostics"))
	assert.True(t, strings.Contains(logOutput, "silence_fills_delta=2"))
	assert.True(t, strings.Contains(logOutput, "zero_filled_samples_delta=8"))
	assert.True(t, strings.Contains(logOutput, "jb_underruns_total=1"))
	assert.True(t, strings.Contains(logOutput, "jb_drops_total=7"))
	assert.True(t, strings.Contains(logOutput, "player_drops_total=1"))
	assert.True(t, strings.Contains(logOutput, "pump_late_25ms_total=1"))
	assert.True(t, strings.Contains(logOutput, "pump_late_40ms_total=1"))
	assert.True(t, strings.Contains(logOutput, "pcm_clip_samples_total=2"))
	assert.True(t, strings.Contains(logOutput, "pcm_nan_samples_total=1"))
	assert.True(t, strings.Contains(logOutput, "pcm_inf_samples_total=1"))
	assert.True(t, strings.Contains(logOutput, "pcm_boundary_jump_max="))
	assert.True(t, strings.Contains(logOutput, "pcm_boundary_jump_l="))
	assert.True(t, strings.Contains(logOutput, "pcm_boundary_jump_r="))
	assert.True(t, strings.Contains(logOutput, "pcm_rms_l="))
	assert.True(t, strings.Contains(logOutput, "pcm_rms_r="))
	assert.True(t, strings.Contains(logOutput, "pcm_dc_offset_l="))
	assert.True(t, strings.Contains(logOutput, "pcm_dc_offset_r="))
	assert.True(t, strings.Contains(logOutput, "callback_count_total=4"))
	assert.True(t, strings.Contains(logOutput, "callback_late_25ms_total=1"))
	assert.True(t, strings.Contains(logOutput, "callback_late_40ms_total=1"))
	assert.False(t, strings.Contains(logOutput, "jb_underruns="))
	assert.False(t, strings.Contains(logOutput, "pcm_clip_samples="))
	assert.False(t, strings.Contains(logOutput, "callback_count="))
	assert.Equal(t, uint64(3), previous.silenceFills)
}

func recordDeterministicPumpSamples(diagnostics *clientPlaybackDiagnostics) {
	diagnostics.recordBufferState(3, 2)
	diagnostics.recordBufferState(1, 4)
	diagnostics.recordBufferState(6, 0)
	diagnostics.recordPumpTick(timeAt(time.Second))
	diagnostics.recordPumpTick(timeAt(time.Second + 26*time.Millisecond))
	diagnostics.recordPumpTick(timeAt(time.Second + 70*time.Millisecond))
	diagnostics.recordPCMStats(pcmFrameStats{
		PeakAbs:     0.75,
		ClipSamples: 2,
		NaNSamples:  1,
		InfSamples:  1,
	})
	diagnostics.recordJitterUnderrun()
	diagnostics.recordPlayerDrop()
	diagnostics.recordJitterDrops(7)
}

type fakeDiagnosticsPlayer struct {
	snapshot audio.PlayerDiagnosticsSnapshot
}

func (p fakeDiagnosticsPlayer) SilenceFills() uint64 {
	return p.snapshot.SilenceFillsTotal
}

func (p fakeDiagnosticsPlayer) PlaybackDiagnostics() audio.PlayerDiagnosticsSnapshot {
	return p.snapshot
}

func timeAt(offset time.Duration) time.Time {
	return time.Unix(0, offset.Nanoseconds())
}
