package audio

import (
	"context"
	"math"
)

// ResamplerProcessor converts between different sample rates and channel counts.
// It uses linear interpolation for upsampling and decimation with a simple
// low-pass filter for downsampling. Channel conversion supports mono→stereo
// (duplication) and stereo→mono (averaging). The processor maintains fractional
// position state between Process() calls for smooth interpolation.
type ResamplerProcessor struct {
	srcRate     uint32
	dstRate     uint32
	srcChannels uint32
	dstChannels uint32

	// fractional position state for smooth interpolation across calls
	fracPos float64
}

// NewResamplerProcessor creates a new resampler that converts from srcRate/srcChannels
// to dstRate/dstChannels.
func NewResamplerProcessor(srcRate, dstRate, srcChannels, dstChannels uint32) *ResamplerProcessor {
	return &ResamplerProcessor{
		srcRate:     srcRate,
		dstRate:     dstRate,
		srcChannels: srcChannels,
		dstChannels: dstChannels,
	}
}

// NeedsResampling returns true if either sample rate or channel count differs.
func (r *ResamplerProcessor) NeedsResampling() bool {
	return r.srcRate != r.dstRate || r.srcChannels != r.dstChannels
}

// Name returns "resampler".
func (r *ResamplerProcessor) Name() string {
	return "resampler"
}

// Process resamples the rate first, then converts channels.
// Returns samples unchanged when no conversion is needed.
func (r *ResamplerProcessor) Process(_ context.Context, samples []float32) ([]float32, error) {
	if !r.NeedsResampling() {
		return samples, nil
	}

	resampled := r.resampleRate(samples)
	converted := r.convertChannels(resampled)
	return converted, nil
}

// resampleRate converts sample rate using linear interpolation (up) or
// decimation with low-pass filter (down). Maintains fractional position state.
func (r *ResamplerProcessor) resampleRate(samples []float32) []float32 {
	if r.srcRate == r.dstRate {
		return samples
	}

	srcLen := len(samples)
	if srcLen == 0 {
		return samples
	}

	ratio := float64(r.srcRate) / float64(r.dstRate)
	outLen := int(math.Ceil(float64(srcLen)/ratio - r.fracPos/ratio))
	if outLen <= 0 {
		return nil
	}

	if ratio > 1.0 {
		// Downsampling: apply simple low-pass averaging filter before decimation.
		return r.downsample(samples, ratio, outLen)
	}

	// Upsampling: linear interpolation.
	return r.upsample(samples, ratio, outLen)
}

func (r *ResamplerProcessor) upsample(samples []float32, ratio float64, outLen int) []float32 {
	out := make([]float32, 0, outLen)
	srcLen := len(samples)
	pos := r.fracPos

	for pos < float64(srcLen) {
		idx := int(pos)
		frac := float32(pos - float64(idx))

		if idx+1 < srcLen {
			out = append(out, samples[idx]*(1-frac)+samples[idx+1]*frac)
		} else {
			out = append(out, samples[idx])
		}
		pos += ratio
	}

	r.fracPos = pos - float64(srcLen)
	return out
}

func (r *ResamplerProcessor) downsample(samples []float32, ratio float64, outLen int) []float32 {
	out := make([]float32, 0, outLen)
	srcLen := len(samples)
	pos := r.fracPos
	filterRadius := int(math.Ceil(ratio / 2))

	for pos < float64(srcLen) {
		idx := int(pos)

		// Simple low-pass: average samples in the filter window.
		lo := idx - filterRadius
		if lo < 0 {
			lo = 0
		}
		hi := idx + filterRadius
		if hi >= srcLen {
			hi = srcLen - 1
		}

		var sum float32
		count := hi - lo + 1
		for i := lo; i <= hi; i++ {
			sum += samples[i]
		}
		out = append(out, sum/float32(count))
		pos += ratio
	}

	r.fracPos = pos - float64(srcLen)
	return out
}

// convertChannels handles mono↔stereo conversion.
func (r *ResamplerProcessor) convertChannels(samples []float32) []float32 {
	if r.srcChannels == r.dstChannels {
		return samples
	}

	if r.srcChannels == 1 && r.dstChannels == 2 {
		// Mono → Stereo: duplicate each sample.
		out := make([]float32, len(samples)*2)
		for i, s := range samples {
			out[i*2] = s
			out[i*2+1] = s
		}
		return out
	}

	if r.srcChannels == 2 && r.dstChannels == 1 {
		// Stereo → Mono: average pairs.
		n := len(samples) / 2
		out := make([]float32, n)
		for i := 0; i < n; i++ {
			out[i] = (samples[i*2] + samples[i*2+1]) / 2
		}
		return out
	}

	// Unsupported channel conversion — pass through.
	return samples
}
