package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudioPipeline_CaptureToPlayback(t *testing.T) {
	sampleRate := 48000
	channels := 1
	frameSize := sampleRate / 50 * channels

	acc := NewFrameAccumulator(frameSize)
	enc, err := NewOpusEncoder(sampleRate, channels, OpusApplicationVoIP)
	require.NoError(t, err)
	dec, err := NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	totalSamples := sampleRate / 10
	original := make([]float32, totalSamples)
	for i := range original {
		original[i] = float32(0.3 * math.Sin(2*math.Pi*440.0*float64(i)/float64(sampleRate)))
	}

	chunkSizes := []int{480, 512, 256, 1024, 480, 512, 256, 1024, 480}
	offset := 0
	var decodedAll []float32

	for _, chunkSize := range chunkSizes {
		if offset >= len(original) {
			break
		}
		end := offset + chunkSize
		if end > len(original) {
			end = len(original)
		}
		chunk := original[offset:end]
		offset = end

		frames := acc.Write(chunk)
		for _, frame := range frames {
			encoded, err := enc.Encode(frame)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			decoded, err := dec.Decode(encoded)
			require.NoError(t, err)
			require.Len(t, decoded, frameSize)

			decodedAll = append(decodedAll, decoded...)
		}
	}

	require.NotEmpty(t, decodedAll)
	t.Logf("Original samples: %d, Decoded samples: %d", totalSamples, len(decodedAll))

	minLen := len(decodedAll)
	if minLen > len(original) {
		minLen = len(original)
	}

	var sumOrigSq, sumDecSq, sumOrigDec float64
	for i := 0; i < minLen; i++ {
		o := float64(original[i])
		d := float64(decodedAll[i])
		sumOrigSq += o * o
		sumDecSq += d * d
		sumOrigDec += o * d
	}

	if sumOrigSq > 0 && sumDecSq > 0 {
		correlation := sumOrigDec / (math.Sqrt(sumOrigSq) * math.Sqrt(sumDecSq))
		t.Logf("Signal correlation: %.4f", correlation)
		assert.Greater(t, correlation, 0.4, "Decoded signal should correlate with original")
	}
}

func TestAudioPipeline_Stereo(t *testing.T) {
	sampleRate := 48000
	channels := 2
	frameSize := sampleRate / 50 * channels

	acc := NewFrameAccumulator(frameSize)
	enc, err := NewOpusEncoder(sampleRate, channels, OpusApplicationAudio)
	require.NoError(t, err)
	dec, err := NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	stereoSamples := make([]float32, frameSize*2)
	for i := 0; i < len(stereoSamples); i += 2 {
		phase := float64(i/2) / float64(sampleRate)
		stereoSamples[i] = float32(0.3 * math.Sin(2*math.Pi*440.0*phase))
		stereoSamples[i+1] = float32(0.3 * math.Sin(2*math.Pi*880.0*phase))
	}

	frames := acc.Write(stereoSamples)
	require.Len(t, frames, 2)

	for _, frame := range frames {
		encoded, err := enc.Encode(frame)
		require.NoError(t, err)

		decoded, err := dec.Decode(encoded)
		require.NoError(t, err)
		assert.Len(t, decoded, frameSize)
	}
}

func TestAudioPipeline_SilenceThenSignal(t *testing.T) {
	sampleRate := 48000
	channels := 1
	frameSize := sampleRate / 50

	acc := NewFrameAccumulator(frameSize)
	enc, err := NewOpusEncoder(sampleRate, channels, OpusApplicationVoIP)
	require.NoError(t, err)
	err = enc.SetDTX(true)
	require.NoError(t, err)

	dec, err := NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	silence := make([]float32, frameSize*3)
	frames := acc.Write(silence)
	require.Len(t, frames, 3)

	for _, frame := range frames {
		encoded, err := enc.Encode(frame)
		require.NoError(t, err)
		t.Logf("Silence packet size: %d bytes", len(encoded))

		decoded, err := dec.Decode(encoded)
		require.NoError(t, err)
		assert.Len(t, decoded, frameSize)
	}

	signal := make([]float32, frameSize*2)
	for i := range signal {
		signal[i] = float32(0.5 * math.Sin(2*math.Pi*440.0*float64(i)/float64(sampleRate)))
	}
	frames = acc.Write(signal)
	require.Len(t, frames, 2)

	for _, frame := range frames {
		encoded, err := enc.Encode(frame)
		require.NoError(t, err)
		t.Logf("Signal packet size: %d bytes", len(encoded))

		decoded, err := dec.Decode(encoded)
		require.NoError(t, err)
		assert.Len(t, decoded, frameSize)
	}
}
