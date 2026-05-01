package app

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

const (
	playbackDiagnosticsInterval = 2 * time.Second
	pumpLate25Milliseconds      = 25
	pumpLate40Milliseconds      = 40
	unsetDiagnosticMinimum      = -1
)

type playbackDiagnosticsSnapshot struct {
	JBDepth             int64
	JBDepthMin          int64
	JBDepthMax          int64
	JBUnderrunsTotal    uint64
	JBDropsTotal        uint64
	PlayerDropsTotal    uint64
	PlaybackChLen       int64
	PlaybackChCap       int64
	PumpTickGapMax      time.Duration
	PumpLate25MSTotal   uint64
	PumpLate40MSTotal   uint64
	PCMPeakAbs          float32
	PCMClipSamplesTotal uint64
	PCMNaNSamplesTotal  uint64
	PCMInfSamplesTotal  uint64
	PCMBoundaryJumpMax  float32
	PCMBoundaryJumpL    float32
	PCMBoundaryJumpR    float32
	PCMRMSL             float64
	PCMRMSR             float64
	PCMDCOffsetL        float64
	PCMDCOffsetR        float64
}

type silenceFillCounter interface {
	SilenceFills() uint64
}

type playbackDiagnosticsProvider interface {
	PlaybackDiagnostics() audio.PlayerDiagnosticsSnapshot
}

type playerDiagnosticsPrevious struct {
	silenceFills uint64
	zeroSamples  uint64
	partialFills uint64
	fullFills    uint64
}

type clientPlaybackDiagnostics struct {
	jbDepth             atomic.Int64
	jbDepthMin          atomic.Int64
	jbDepthMax          atomic.Int64
	jbUnderruns         atomic.Uint64
	jbDrops             atomic.Uint64
	playerDrops         atomic.Uint64
	playbackChLen       atomic.Int64
	playbackChCap       int64
	lastPumpTickNano    atomic.Int64
	pumpTickGapMaxNanos atomic.Uint64
	pumpLate25MS        atomic.Uint64
	pumpLate40MS        atomic.Uint64
	pcmPeakAbsBits      atomic.Uint32
	pcmClipSamples      atomic.Uint64
	pcmNaNSamples       atomic.Uint64
	pcmInfSamples       atomic.Uint64
	pcmMu               sync.Mutex
	pcmScanner          pcmContinuityScanner
	pcmBoundaryJumpMax  float32
	pcmBoundaryJumpL    float32
	pcmBoundaryJumpR    float32
	pcmSum              [maxPCMContinuityChannels]float64
	pcmSumSquares       [maxPCMContinuityChannels]float64
	pcmSamples          [maxPCMContinuityChannels]uint64
}

func newClientPlaybackDiagnostics(playbackChCap int) *clientPlaybackDiagnostics {
	d := &clientPlaybackDiagnostics{playbackChCap: int64(playbackChCap)}
	d.jbDepthMin.Store(unsetDiagnosticMinimum)
	return d
}

func (d *clientPlaybackDiagnostics) recordPumpTick(now time.Time) {
	prev := d.lastPumpTickNano.Swap(now.UnixNano())
	if prev == 0 || now.UnixNano() <= prev {
		return
	}
	d.recordPumpGap(time.Duration(now.UnixNano() - prev))
}

func (d *clientPlaybackDiagnostics) recordPumpGap(gap time.Duration) {
	updateAtomicUint64Max(&d.pumpTickGapMaxNanos, uint64(gap))
	if gap >= time.Duration(pumpLate25Milliseconds)*time.Millisecond {
		d.pumpLate25MS.Add(1)
	}
	if gap >= time.Duration(pumpLate40Milliseconds)*time.Millisecond {
		d.pumpLate40MS.Add(1)
	}
}

func (d *clientPlaybackDiagnostics) recordBufferState(jbDepth, playbackChLen int) {
	depth := int64(jbDepth)
	d.jbDepth.Store(depth)
	d.playbackChLen.Store(int64(playbackChLen))
	updateAtomicInt64Min(&d.jbDepthMin, depth)
	updateAtomicInt64Max(&d.jbDepthMax, depth)
}

func (d *clientPlaybackDiagnostics) recordJitterUnderrun() {
	d.jbUnderruns.Add(1)
}

func (d *clientPlaybackDiagnostics) recordJitterDrops(count uint64) {
	d.jbDrops.Store(count)
}

func (d *clientPlaybackDiagnostics) recordPlayerDrop() {
	d.playerDrops.Add(1)
}

func (d *clientPlaybackDiagnostics) snapshotAndResetInterval() playbackDiagnosticsSnapshot {
	minDepth := d.jbDepthMin.Swap(unsetDiagnosticMinimum)
	if minDepth == unsetDiagnosticMinimum {
		minDepth = d.jbDepth.Load()
	}
	pcmInterval := d.pcmSnapshotAndResetInterval()
	return playbackDiagnosticsSnapshot{
		JBDepth:             d.jbDepth.Load(),
		JBDepthMin:          minDepth,
		JBDepthMax:          d.jbDepthMax.Swap(0),
		JBUnderrunsTotal:    d.jbUnderruns.Load(),
		JBDropsTotal:        d.jbDrops.Load(),
		PlayerDropsTotal:    d.playerDrops.Load(),
		PlaybackChLen:       d.playbackChLen.Load(),
		PlaybackChCap:       d.playbackChCap,
		PumpTickGapMax:      time.Duration(d.pumpTickGapMaxNanos.Swap(0)),
		PumpLate25MSTotal:   d.pumpLate25MS.Load(),
		PumpLate40MSTotal:   d.pumpLate40MS.Load(),
		PCMPeakAbs:          math.Float32frombits(d.pcmPeakAbsBits.Swap(0)),
		PCMClipSamplesTotal: d.pcmClipSamples.Load(),
		PCMNaNSamplesTotal:  d.pcmNaNSamples.Load(),
		PCMInfSamplesTotal:  d.pcmInfSamples.Load(),
		PCMBoundaryJumpMax:  pcmInterval.PCMBoundaryJumpMax,
		PCMBoundaryJumpL:    pcmInterval.PCMBoundaryJumpL,
		PCMBoundaryJumpR:    pcmInterval.PCMBoundaryJumpR,
		PCMRMSL:             pcmInterval.PCMRMSL,
		PCMRMSR:             pcmInterval.PCMRMSR,
		PCMDCOffsetL:        pcmInterval.PCMDCOffsetL,
		PCMDCOffsetR:        pcmInterval.PCMDCOffsetR,
	}
}

func recordPumpTick(diagnostics *clientPlaybackDiagnostics) {
	if diagnostics != nil {
		diagnostics.recordPumpTick(time.Now())
	}
}

func recordJitterUnderrun(diagnostics *clientPlaybackDiagnostics, jb *audio.JitterBuffer) {
	if diagnostics == nil {
		return
	}
	diagnostics.recordJitterUnderrun()
	diagnostics.recordJitterDrops(jb.DropCount())
}

func recordPlayerDrop(diagnostics *clientPlaybackDiagnostics) {
	if diagnostics != nil {
		diagnostics.recordPlayerDrop()
	}
}

func recordPlaybackDiagnostics(diagnostics *clientPlaybackDiagnostics, jb *audio.JitterBuffer, playbackCh chan<- []float32, frame []float32, channels int) {
	if diagnostics == nil {
		return
	}
	diagnostics.recordBufferState(jb.Depth(), len(playbackCh))
	diagnostics.recordJitterDrops(jb.DropCount())
	diagnostics.recordPCMFrame(frame, channels)
}

func updateAtomicUint64Max(target *atomic.Uint64, value uint64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}

func updateAtomicInt64Max(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}

func updateAtomicInt64Min(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if current != unsetDiagnosticMinimum && value >= current {
			return
		}
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

func updateAtomicFloat32Max(target *atomic.Uint32, value float32) {
	for {
		currentBits := target.Load()
		current := math.Float32frombits(currentBits)
		if value <= current || target.CompareAndSwap(currentBits, math.Float32bits(value)) {
			return
		}
	}
}

func milliseconds(duration time.Duration) int64 {
	return duration.Milliseconds()
}

func logPlayerSilenceFills(ctx context.Context, logger *slog.Logger, player silenceFillCounter, diagnostics *clientPlaybackDiagnostics) {
	ticker := time.NewTicker(playbackDiagnosticsInterval)
	defer ticker.Stop()
	var previous playerDiagnosticsPrevious
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logPlayerDiagnosticsTick(logger, player, diagnostics, &previous)
		}
	}
}

func logPlayerSilenceDelta(logger *slog.Logger, player silenceFillCounter, prev *uint64) {
	cur := player.SilenceFills()
	delta := cur - *prev
	*prev = cur
	if delta > 0 {
		logger.Info("Player silence-fill events", "in_last_2s", delta, "total", cur)
	}
}

func logPlayerDiagnosticsTick(logger *slog.Logger, player silenceFillCounter, diagnostics *clientPlaybackDiagnostics, previous *playerDiagnosticsPrevious) {
	playerSnapshot := playerDiagnosticsSnapshot(player)
	logSilenceFillWarning(logger, playerSnapshot, previous)
	if diagnostics == nil {
		updatePreviousPlayerDiagnostics(previous, playerSnapshot)
		return
	}
	logAudioPlaybackDiagnostics(logger, diagnostics.snapshotAndResetInterval(), playerSnapshot, previous)
	updatePreviousPlayerDiagnostics(previous, playerSnapshot)
}

func playerDiagnosticsSnapshot(player silenceFillCounter) audio.PlayerDiagnosticsSnapshot {
	provider, ok := player.(playbackDiagnosticsProvider)
	if ok {
		return provider.PlaybackDiagnostics()
	}
	return audio.PlayerDiagnosticsSnapshot{SilenceFillsTotal: player.SilenceFills()}
}

func logSilenceFillWarning(logger *slog.Logger, snapshot audio.PlayerDiagnosticsSnapshot, previous *playerDiagnosticsPrevious) {
	delta := snapshot.SilenceFillsTotal - previous.silenceFills
	if delta > 0 {
		logger.Info("Player silence-fill events", "in_last_2s", delta, "total", snapshot.SilenceFillsTotal)
	}
}

func logAudioPlaybackDiagnostics(logger *slog.Logger, pump playbackDiagnosticsSnapshot, player audio.PlayerDiagnosticsSnapshot, previous *playerDiagnosticsPrevious) {
	if !logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	logger.Debug("Audio playback diagnostics", audioPlaybackDiagnosticFields(pump, player, previous)...)
}

func audioPlaybackDiagnosticFields(pump playbackDiagnosticsSnapshot, player audio.PlayerDiagnosticsSnapshot, previous *playerDiagnosticsPrevious) []any {
	fields := make([]any, 0, 70)
	fields = appendPumpDiagnosticFields(fields, pump)
	fields = appendPlayerDiagnosticFields(fields, player, previous)
	return appendPCMDiagnosticFields(fields, pump)
}

func appendPumpDiagnosticFields(fields []any, pump playbackDiagnosticsSnapshot) []any {
	return append(fields,
		"jb_depth", pump.JBDepth,
		"jb_depth_min", pump.JBDepthMin,
		"jb_depth_max", pump.JBDepthMax,
		"jb_underruns_total", pump.JBUnderrunsTotal,
		"jb_drops_total", pump.JBDropsTotal,
		"player_drops_total", pump.PlayerDropsTotal,
		"playback_ch_len", pump.PlaybackChLen,
		"playback_ch_cap", pump.PlaybackChCap,
		"pump_tick_gap_max_ms", milliseconds(pump.PumpTickGapMax),
		"pump_late_25ms_total", pump.PumpLate25MSTotal,
		"pump_late_40ms_total", pump.PumpLate40MSTotal,
	)
}

func appendPlayerDiagnosticFields(fields []any, player audio.PlayerDiagnosticsSnapshot, previous *playerDiagnosticsPrevious) []any {
	return append(fields,
		"callback_count_total", player.CallbackCount,
		"callback_first_framecount", player.FirstCallbackFrameCount,
		"callback_gap_max_ms", milliseconds(player.CallbackGapMax),
		"callback_late_25ms_total", player.CallbackLate25MS,
		"callback_late_40ms_total", player.CallbackLate40MS,
		"silence_fills_delta", player.SilenceFillsTotal-previous.silenceFills,
		"silence_fills_total", player.SilenceFillsTotal,
		"zero_filled_samples_delta", player.ZeroFilledSamples-previous.zeroSamples,
		"partial_silence_fills_delta", player.PartialSilenceFills-previous.partialFills,
		"full_silence_fills_delta", player.FullSilenceFills-previous.fullFills,
	)
}

func appendPCMDiagnosticFields(fields []any, pump playbackDiagnosticsSnapshot) []any {
	return append(fields,
		"pcm_peak_abs", pump.PCMPeakAbs,
		"pcm_clip_samples_total", pump.PCMClipSamplesTotal,
		"pcm_nan_samples_total", pump.PCMNaNSamplesTotal,
		"pcm_inf_samples_total", pump.PCMInfSamplesTotal,
		"pcm_boundary_jump_max", pump.PCMBoundaryJumpMax,
		"pcm_boundary_jump_l", pump.PCMBoundaryJumpL,
		"pcm_boundary_jump_r", pump.PCMBoundaryJumpR,
		"pcm_rms_l", pump.PCMRMSL,
		"pcm_rms_r", pump.PCMRMSR,
		"pcm_dc_offset_l", pump.PCMDCOffsetL,
		"pcm_dc_offset_r", pump.PCMDCOffsetR,
	)
}

func updatePreviousPlayerDiagnostics(previous *playerDiagnosticsPrevious, snapshot audio.PlayerDiagnosticsSnapshot) {
	previous.silenceFills = snapshot.SilenceFillsTotal
	previous.zeroSamples = snapshot.ZeroFilledSamples
	previous.partialFills = snapshot.PartialSilenceFills
	previous.fullFills = snapshot.FullSilenceFills
}
