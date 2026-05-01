package app

import "math"

const (
	defaultPCMChannels       = 1
	maxPCMContinuityChannels = 2
	leftPCMChannel           = 0
	rightPCMChannel          = 1
)

type pcmFrameStats struct {
	PeakAbs         float32
	ClipSamples     uint64
	NaNSamples      uint64
	InfSamples      uint64
	BoundaryJumpMax float32
	BoundaryJumpL   float32
	BoundaryJumpR   float32
	Sum             [maxPCMContinuityChannels]float64
	SumSquares      [maxPCMContinuityChannels]float64
	Samples         [maxPCMContinuityChannels]uint64
}

type pcmContinuityScanner struct {
	previousTail      []float32
	previousTailValid []bool
}

func scanPCMFrame(frame []float32) pcmFrameStats {
	scanner := pcmContinuityScanner{}
	return scanner.scan(frame, defaultPCMChannels)
}

func (s *pcmContinuityScanner) scan(frame []float32, channels int) pcmFrameStats {
	channels = normalizedPCMChannels(channels)
	s.ensureChannels(channels)
	stats := scanPCMFrameSamples(frame, channels)
	s.recordBoundaryJumps(frame, channels, &stats)
	s.updatePreviousTail(frame, channels)
	return stats
}

func scanPCMFrameSamples(frame []float32, channels int) pcmFrameStats {
	var stats pcmFrameStats
	for i, sample := range frame {
		stats.recordSample(sample, i%channels)
	}
	return stats
}

func (s *pcmFrameStats) recordSample(sample float32, channel int) {
	value := float64(sample)
	if math.IsNaN(value) {
		s.NaNSamples++
		return
	}
	if math.IsInf(value, 0) {
		s.InfSamples++
		return
	}
	s.recordFiniteSample(value, channel)
}

func (s *pcmFrameStats) recordFiniteSample(value float64, channel int) {
	abs := float32(math.Abs(value))
	if abs > s.PeakAbs {
		s.PeakAbs = abs
	}
	if abs >= 1.0 {
		s.ClipSamples++
	}
	if channel < maxPCMContinuityChannels {
		s.recordMoment(value, channel)
	}
}

func (s *pcmFrameStats) recordMoment(value float64, channel int) {
	s.Sum[channel] += value
	s.SumSquares[channel] += value * value
	s.Samples[channel]++
}

func normalizedPCMChannels(channels int) int {
	if channels <= 0 {
		return defaultPCMChannels
	}
	return channels
}

func (s *pcmContinuityScanner) ensureChannels(channels int) {
	if len(s.previousTail) == channels {
		return
	}
	s.previousTail = make([]float32, channels)
	s.previousTailValid = make([]bool, channels)
}

func (s *pcmContinuityScanner) recordBoundaryJumps(frame []float32, channels int, stats *pcmFrameStats) {
	for channel := 0; channel < channels && channel < len(frame); channel++ {
		if !s.previousTailValid[channel] || !isFiniteFloat32(frame[channel]) {
			continue
		}
		jump := float32(math.Abs(float64(frame[channel] - s.previousTail[channel])))
		stats.recordBoundaryJump(channel, jump)
	}
}

func (s *pcmFrameStats) recordBoundaryJump(channel int, jump float32) {
	if jump > s.BoundaryJumpMax {
		s.BoundaryJumpMax = jump
	}
	if channel == leftPCMChannel && jump > s.BoundaryJumpL {
		s.BoundaryJumpL = jump
	}
	if channel == rightPCMChannel && jump > s.BoundaryJumpR {
		s.BoundaryJumpR = jump
	}
}

func (s *pcmContinuityScanner) updatePreviousTail(frame []float32, channels int) {
	completeSamples := len(frame) - len(frame)%channels
	if completeSamples < channels {
		return
	}
	tailStart := completeSamples - channels
	for channel := 0; channel < channels; channel++ {
		sample := frame[tailStart+channel]
		s.previousTail[channel] = sample
		s.previousTailValid[channel] = isFiniteFloat32(sample)
	}
}

func isFiniteFloat32(sample float32) bool {
	value := float64(sample)
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (d *clientPlaybackDiagnostics) recordPCMStats(stats pcmFrameStats) {
	updateAtomicFloat32Max(&d.pcmPeakAbsBits, stats.PeakAbs)
	d.pcmClipSamples.Add(stats.ClipSamples)
	d.pcmNaNSamples.Add(stats.NaNSamples)
	d.pcmInfSamples.Add(stats.InfSamples)
	d.pcmMu.Lock()
	d.recordPCMIntervalStatsLocked(stats)
	d.pcmMu.Unlock()
}

func (d *clientPlaybackDiagnostics) recordPCMFrame(frame []float32, channels int) {
	d.pcmMu.Lock()
	stats := d.pcmScanner.scan(frame, channels)
	d.recordPCMIntervalStatsLocked(stats)
	d.pcmMu.Unlock()
	updateAtomicFloat32Max(&d.pcmPeakAbsBits, stats.PeakAbs)
	d.pcmClipSamples.Add(stats.ClipSamples)
	d.pcmNaNSamples.Add(stats.NaNSamples)
	d.pcmInfSamples.Add(stats.InfSamples)
}

func (d *clientPlaybackDiagnostics) recordPCMIntervalStatsLocked(stats pcmFrameStats) {
	updatePCMMax(&d.pcmBoundaryJumpMax, stats.BoundaryJumpMax)
	updatePCMMax(&d.pcmBoundaryJumpL, stats.BoundaryJumpL)
	updatePCMMax(&d.pcmBoundaryJumpR, stats.BoundaryJumpR)
	for channel := range d.pcmSum {
		d.pcmSum[channel] += stats.Sum[channel]
		d.pcmSumSquares[channel] += stats.SumSquares[channel]
		d.pcmSamples[channel] += stats.Samples[channel]
	}
}

func updatePCMMax(target *float32, value float32) {
	if value > *target {
		*target = value
	}
}

func (d *clientPlaybackDiagnostics) pcmSnapshotAndResetInterval() playbackDiagnosticsSnapshot {
	d.pcmMu.Lock()
	defer d.pcmMu.Unlock()
	snapshot := playbackDiagnosticsSnapshot{
		PCMBoundaryJumpMax: d.pcmBoundaryJumpMax,
		PCMBoundaryJumpL:   d.pcmBoundaryJumpL,
		PCMBoundaryJumpR:   d.pcmBoundaryJumpR,
		PCMRMSL:            rms(d.pcmSumSquares[leftPCMChannel], d.pcmSamples[leftPCMChannel]),
		PCMRMSR:            rms(d.pcmSumSquares[rightPCMChannel], d.pcmSamples[rightPCMChannel]),
		PCMDCOffsetL:       average(d.pcmSum[leftPCMChannel], d.pcmSamples[leftPCMChannel]),
		PCMDCOffsetR:       average(d.pcmSum[rightPCMChannel], d.pcmSamples[rightPCMChannel]),
	}
	d.resetPCMIntervalStatsLocked()
	return snapshot
}

func (d *clientPlaybackDiagnostics) resetPCMIntervalStatsLocked() {
	d.pcmBoundaryJumpMax = 0
	d.pcmBoundaryJumpL = 0
	d.pcmBoundaryJumpR = 0
	d.pcmSum = [maxPCMContinuityChannels]float64{}
	d.pcmSumSquares = [maxPCMContinuityChannels]float64{}
	d.pcmSamples = [maxPCMContinuityChannels]uint64{}
}

func rms(sumSquares float64, samples uint64) float64 {
	if samples == 0 {
		return 0
	}
	return math.Sqrt(sumSquares / float64(samples))
}

func average(sum float64, samples uint64) float64 {
	if samples == 0 {
		return 0
	}
	return sum / float64(samples)
}
