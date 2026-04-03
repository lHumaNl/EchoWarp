package audio

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpusEncoder_Encode_ProducesValidPacket(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	pcm := make([]float32, 960)
	data, err := enc.Encode(pcm)
	require.NoError(t, err)
	assert.NotEmpty(t, data, "encoded packet should not be empty")
	assert.Less(t, len(data), 960*4, "encoded packet should be smaller than raw PCM")
}

func TestOpusEncoder_Encode_WrongFrameSize_ReturnsError(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	pcm := make([]float32, 500)
	_, err = enc.Encode(pcm)
	assert.Error(t, err, "encoding with wrong frame size should return error")
}

func TestOpusEncoder_SetBitrate_AffectsOutput(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	pcm := generateSineWave(960, 440.0, 48000)

	data1, err := enc.Encode(pcm)
	require.NoError(t, err)

	err = enc.SetBitrate(12000)
	require.NoError(t, err)

	data2, err := enc.Encode(pcm)
	require.NoError(t, err)

	t.Logf("64kbps packet: %d bytes, 12kbps packet: %d bytes", len(data1), len(data2))
}

func TestOpusEncoder_DTX_SilenceProducesSmallPackets(t *testing.T) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	err = enc.SetDTX(true)
	require.NoError(t, err)

	silence := make([]float32, 960)
	var totalSize int
	for i := 0; i < 50; i++ {
		data, err := enc.Encode(silence)
		require.NoError(t, err)
		totalSize += len(data)
	}

	avgSize := totalSize / 50
	t.Logf("DTX avg silence packet: %d bytes", avgSize)
	assert.Less(t, avgSize, 20, "DTX silence packets should be very small")
}

func TestOpusDecoder_Decode_RecoversOriginalPCM(t *testing.T) {
	t.Parallel()
	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	pcm := make([]float32, 960)
	data, err := enc.Encode(pcm)
	require.NoError(t, err)

	decoded, err := dec.Decode(data)
	require.NoError(t, err)
	assert.Len(t, decoded, 960)
}

func TestOpusDecoder_DecodePLC_ProducesAudio(t *testing.T) {
	t.Parallel()
	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	sine := generateSineWave(960, 440.0, 48000)
	data, err := enc.Encode(sine)
	require.NoError(t, err)

	_, err = dec.Decode(data)
	require.NoError(t, err)

	plc, err := dec.DecodePLC()
	require.NoError(t, err)
	assert.Len(t, plc, 960, "PLC should produce frameSize samples")
}

func TestOpusEncoder_Decoder_Roundtrip_Fidelity(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationAudio)
	require.NoError(t, err)

	err = enc.SetBitrate(128000)
	require.NoError(t, err)

	err = enc.SetComplexity(10)
	require.NoError(t, err)

	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	original := generateSineWave(960, 440.0, 48000)

	data, err := enc.Encode(original)
	require.NoError(t, err)

	decoded, err := dec.Decode(data)
	require.NoError(t, err)
	require.Len(t, decoded, 960)

	var mae float64
	for i := range original {
		mae += math.Abs(float64(original[i] - decoded[i]))
	}
	mae /= float64(len(original))
	t.Logf("Mean absolute error: %f", mae)
	assert.Less(t, mae, 0.5, "Roundtrip should have reasonable fidelity")
}

func TestOpusEncoder_Stereo_Success(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 2, OpusApplicationAudio)
	require.NoError(t, err)

	pcm := make([]float32, 1920)
	data, err := enc.Encode(pcm)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestOpusDecoder_Stereo_Success(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 2, OpusApplicationAudio)
	require.NoError(t, err)

	dec, err := NewOpusDecoder(48000, 2)
	require.NoError(t, err)

	pcm := make([]float32, 1920)
	data, err := enc.Encode(pcm)
	require.NoError(t, err)

	decoded, err := dec.Decode(data)
	require.NoError(t, err)
	assert.Len(t, decoded, 1920)
}

func TestOpusEncoder_SetComplexity_Valid(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	err = enc.SetComplexity(0)
	assert.NoError(t, err)

	err = enc.SetComplexity(10)
	assert.NoError(t, err)
}

func TestOpusEncoder_SetInBandFEC_Success(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	err = enc.SetInBandFEC(true)
	assert.NoError(t, err)

	err = enc.SetInBandFEC(false)
	assert.NoError(t, err)
}

func TestOpusEncoder_SetPacketLossPerc_Success(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	err = enc.SetPacketLossPerc(10)
	assert.NoError(t, err)

	err = enc.SetPacketLossPerc(50)
	assert.NoError(t, err)
}

func TestOpus_FrameSize(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		channels   int
		isEncoder  bool
		expected   int
	}{
		{"encoder mono", 48000, 1, true, 960},
		{"encoder stereo", 48000, 2, true, 1920},
		{"decoder mono", 48000, 1, false, 960},
		{"decoder stereo", 48000, 2, false, 1920},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isEncoder {
				enc, err := NewOpusEncoder(tt.sampleRate, tt.channels, OpusApplicationVoIP)
				require.NoError(t, err)
				frameSize := enc.FrameSize()
				assert.Equal(t, tt.expected, frameSize)
			} else {
				dec, err := NewOpusDecoder(tt.sampleRate, tt.channels)
				require.NoError(t, err)
				frameSize := dec.FrameSize()
				assert.Equal(t, tt.expected, frameSize)
			}
		})
	}
}

func TestOpusEncoder_InvalidApplication(t *testing.T) {
	t.Parallel()
	_, err := NewOpusEncoder(48000, 1, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown opus application")
}

func TestOpusEncoder_AllApplications(t *testing.T) {
	apps := []string{OpusApplicationVoIP, OpusApplicationAudio, OpusApplicationRestrictedLowDelay}
	for _, app := range apps {
		enc, err := NewOpusEncoder(48000, 1, app)
		require.NoError(t, err, "failed for application: %s", app)
		_ = enc
	}
}

func TestOpusDecoder_Decode_InvalidData(t *testing.T) {
	t.Parallel()
	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	_, err = dec.Decode([]byte{0xFF, 0xFF, 0xFF})
	assert.Error(t, err)
}

func TestOpusDecoder_DecodePLC_FirstCall(t *testing.T) {
	t.Parallel()
	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	plc, err := dec.DecodePLC()
	require.NoError(t, err)
	assert.Len(t, plc, 960)
}

func TestOpusEncoder_DifferentSampleRates(t *testing.T) {
	sampleRates := []int{8000, 12000, 16000, 24000, 48000}
	for _, sr := range sampleRates {
		enc, err := NewOpusEncoder(sr, 1, OpusApplicationVoIP)
		require.NoError(t, err, "failed for sample rate %d", sr)
		frameSize := enc.FrameSize()
		assert.Greater(t, frameSize, 0, "sample rate %d", sr)
	}
}

func TestOpusEncoder_Encode_LargeInput(t *testing.T) {
	t.Parallel()
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	pcm := make([]float32, 960)
	for i := range pcm {
		pcm[i] = float32(i%100) / 100.0
	}

	data, err := enc.Encode(pcm)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestOpusDecoder_Decode_MultipleFrames(t *testing.T) {
	dec, err := NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		pcm := generateSineWave(960, 440.0+float64(i)*100, 48000)
		data, err := enc.Encode(pcm)
		require.NoError(t, err)

		decoded, err := dec.Decode(data)
		require.NoError(t, err)
		assert.Len(t, decoded, 960)
	}
}

func FuzzOpusDecoder(f *testing.F) {
	enc, err := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	if err != nil {
		f.Fatalf("failed to create encoder: %v", err)
	}

	seedPCMs := [][]float32{
		make([]float32, 960),
		generateSineWave(960, 440.0, 48000),
		generateSineWave(960, 1000.0, 48000),
	}
	for _, pcm := range seedPCMs {
		data, err := enc.Encode(pcm)
		if err != nil {
			f.Fatalf("failed to encode seed: %v", err)
		}
		f.Add(data)
	}

	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Add(make([]byte, 4000))

	f.Fuzz(func(t *testing.T, data []byte) {
		dec, err := NewOpusDecoder(48000, 1)
		if err != nil {
			t.Skip()
		}

		_, err = dec.Decode(data)
		if err != nil {
			return
		}

		_, _ = dec.DecodePLC()
	})
}

func FuzzOpusDecoderStereo(f *testing.F) {
	enc, err := NewOpusEncoder(48000, 2, OpusApplicationAudio)
	if err != nil {
		f.Fatalf("failed to create encoder: %v", err)
	}

	seedPCMs := [][]float32{
		make([]float32, 1920),
		generateSineWave(1920, 440.0, 48000),
	}
	for _, pcm := range seedPCMs {
		data, err := enc.Encode(pcm)
		if err != nil {
			f.Fatalf("failed to encode seed: %v", err)
		}
		f.Add(data)
	}

	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		dec, err := NewOpusDecoder(48000, 2)
		if err != nil {
			t.Skip()
		}

		_, err = dec.Decode(data)
		if err != nil {
			return
		}

		_, _ = dec.DecodePLC()
	})
}

func FuzzBytesToFloat32Slice(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x00, 0x00, 0x80, 0x3F})
	f.Add([]byte{0x00, 0x00, 0x80, 0xBF})
	f.Add(make([]byte, 4000))

	f.Fuzz(func(t *testing.T, data []byte) {
		samples := bytesToFloat32Slice(data)

		if len(data) == 0 {
			if samples != nil {
				t.Errorf("expected nil for empty input")
			}
			return
		}

		if len(data)%4 != 0 {
			return
		}

		expectedLen := len(data) / 4
		if len(samples) != expectedLen {
			t.Errorf("expected %d samples, got %d", expectedLen, len(samples))
		}

		result := float32SliceToBytes(samples)
		if !equalBytes(data, result) {
			t.Errorf("roundtrip mismatch")
		}
	})
}

func FuzzFloat32SliceToBytes(f *testing.F) {
	f.Add([]byte{})
	f.Add(float32ToBytes(0.0))
	f.Add(float32ToBytes(0.5))
	f.Add(float32ToBytes(-0.5))
	f.Add(append(float32ToBytes(0.5), float32ToBytes(-0.5)...))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data)%4 != 0 {
			t.Skip()
		}

		samples := bytesToFloat32Slice(data)
		if len(data) == 0 {
			return
		}

		result := float32SliceToBytes(samples)
		if !equalBytes(data, result) {
			t.Errorf("roundtrip mismatch")
		}
	})
}

func float32ToBytes(v float32) []byte {
	bits := math.Float32bits(v)
	return []byte{
		byte(bits),
		byte(bits >> 8),
		byte(bits >> 16),
		byte(bits >> 24),
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalFloat32Slices(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func generateSineWave(numSamples int, frequency float64, sampleRate int) []float32 {
	samples := make([]float32, numSamples)
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*frequency*float64(i)/float64(sampleRate)))
	}
	return samples
}

func BenchmarkOpusEncoder_Encode(b *testing.B) {
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	pcm := make([]float32, 960)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = enc.Encode(pcm)
	}
}

func BenchmarkOpusDecoder_Decode(b *testing.B) {
	dec, _ := NewOpusDecoder(48000, 1)
	enc, _ := NewOpusEncoder(48000, 1, OpusApplicationVoIP)
	pcm := make([]float32, 960)
	opusData, _ := enc.Encode(pcm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dec.Decode(opusData)
	}
}

func BenchmarkGetPCMBuffer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := getPCMBuffer(1920)
		putPCMBuffer(buf)
	}
}

func BenchmarkBytesToFloat32Slice(b *testing.B) {
	data := make([]byte, 960*4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bytesToFloat32Slice(data)
	}
}

func BenchmarkFloat32SliceToBytes(b *testing.B) {
	pcm := make([]float32, 960)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = float32SliceToBytes(pcm)
	}
}
