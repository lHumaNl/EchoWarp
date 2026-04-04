package app

import (
	"context"
	"log/slog"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// integrationLogger returns a logger for integration tests.
func integrationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// generateSineFrames creates PCM frames of a sine wave at the given frequency.
func generateSineFrames(sampleRate, channels, numFrames int) [][]float32 {
	frameSize := sampleRate / 50 * channels // 20ms frames
	frames := make([][]float32, numFrames)
	freq := 440.0
	phase := 0.0
	dt := 1.0 / float64(sampleRate)

	for f := 0; f < numFrames; f++ {
		frame := make([]float32, frameSize)
		for i := 0; i < frameSize; i += channels {
			sample := float32(0.5 * math.Sin(2*math.Pi*freq*phase))
			for ch := 0; ch < channels; ch++ {
				frame[i+ch] = sample
			}
			phase += dt
		}
		frames[f] = frame
	}
	return frames
}

// encodePCMFrames encodes PCM frames into Opus packets.
func encodePCMFrames(t *testing.T, sampleRate, channels int, pcmFrames [][]float32) [][]byte {
	t.Helper()
	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)
	require.NoError(t, enc.SetBitrate(64000))

	var encoded [][]byte
	for _, frame := range pcmFrames {
		data, err := enc.Encode(frame)
		require.NoError(t, err)
		cp := make([]byte, len(data))
		copy(cp, data)
		encoded = append(encoded, cp)
	}
	return encoded
}

// TestIntegration_EncodeDecode_WithSpectrumAndLevel tests a full encode→decode
// pipeline with concurrent spectrum and level meter feeding.
// This simulates the server-send → client-receive path.
func TestIntegration_EncodeDecode_WithSpectrumAndLevel(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const numFrames = 50 // 1 second of audio

	spectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	level := audio.NewLevelMeter(channels)

	// Generate and encode audio
	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	opusPackets := encodePCMFrames(t, sampleRate, channels, pcmFrames)

	// Simulate transport channel
	inCh := make(chan []byte, len(opusPackets))
	for _, pkt := range opusPackets {
		inCh <- pkt
	}
	close(inCh)

	// Decode with spectrum/level feeding (real concurrent code path from shared.go)
	dec, err := audio.NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	playbackCh := make(chan []float32, numFrames)
	decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, spectrum, level)

	// Verify decoded frames arrived
	count := 0
	for range playbackCh {
		count++
		if count >= numFrames {
			break
		}
	}
	assert.Equal(t, numFrames, count)

	// Spectrum should show activity (440Hz sine)
	bands := spectrum.Bands()
	assert.Greater(t, len(bands), 0)
	maxBand := 0.0
	for _, b := range bands {
		if b > maxBand {
			maxBand = b
		}
	}
	assert.Greater(t, maxBand, 0.1, "spectrum should detect 440Hz sine")

	// Level meter should show activity
	levels := level.Levels()
	assert.Greater(t, levels[0], 0.1, "level meter should detect signal")
}

// TestIntegration_EncodeDecode_Stereo tests stereo encode→decode with level meter.
func TestIntegration_EncodeDecode_Stereo(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 2
	const numFrames = 50

	level := audio.NewLevelMeter(channels)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	opusPackets := encodePCMFrames(t, sampleRate, channels, pcmFrames)

	inCh := make(chan []byte, len(opusPackets))
	for _, pkt := range opusPackets {
		inCh <- pkt
	}
	close(inCh)

	dec, err := audio.NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	playbackCh := make(chan []float32, numFrames)
	decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, nil, level)

	count := 0
	for range playbackCh {
		count++
		if count >= numFrames {
			break
		}
	}
	assert.Equal(t, numFrames, count)

	levels := level.Levels()
	assert.Len(t, levels, 2)
	assert.Greater(t, levels[0], 0.1, "left channel should have signal")
	assert.Greater(t, levels[1], 0.1, "right channel should have signal")
}

// TestIntegration_DuplexPipeline simulates duplex mode: two concurrent
// encode→decode streams running simultaneously with separate analyzers.
// This is the real concurrent pattern from client.go duplex mode.
func TestIntegration_DuplexPipeline(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const numFrames = 50

	// Separate analyzers for each direction (as fixed in SA-01)
	spectrumSend := audio.NewSpectrumAnalyzer(sampleRate, 0)
	levelSend := audio.NewLevelMeter(channels)
	spectrumRecv := audio.NewSpectrumAnalyzer(sampleRate, 0)
	levelRecv := audio.NewLevelMeter(channels)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	opusPackets := encodePCMFrames(t, sampleRate, channels, pcmFrames)

	var wg sync.WaitGroup

	// Direction 1: "send" path — encode, transport, decode with send analyzers
	wg.Add(1)
	go func() {
		defer wg.Done()
		inCh := make(chan []byte, len(opusPackets))
		for _, pkt := range opusPackets {
			cp := make([]byte, len(pkt))
			copy(cp, pkt)
			inCh <- cp
		}
		close(inCh)

		dec, err := audio.NewOpusDecoder(sampleRate, channels)
		if err != nil {
			t.Errorf("send decoder error: %v", err)
			return
		}
		playbackCh := make(chan []float32, numFrames)
		decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, spectrumSend, levelSend)
	}()

	// Direction 2: "receive" path — same but with receive analyzers
	wg.Add(1)
	go func() {
		defer wg.Done()
		inCh := make(chan []byte, len(opusPackets))
		for _, pkt := range opusPackets {
			cp := make([]byte, len(pkt))
			copy(cp, pkt)
			inCh <- cp
		}
		close(inCh)

		dec, err := audio.NewOpusDecoder(sampleRate, channels)
		if err != nil {
			t.Errorf("recv decoder error: %v", err)
			return
		}
		playbackCh := make(chan []float32, numFrames)
		decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, spectrumRecv, levelRecv)
	}()

	wg.Wait()

	// Both analyzers should have independent data
	maxBand := func(bands []float64) float64 {
		m := 0.0
		for _, b := range bands {
			if b > m {
				m = b
			}
		}
		return m
	}
	assert.Greater(t, maxBand(spectrumSend.Bands()), 0.0, "send spectrum should have data")
	assert.Greater(t, maxBand(spectrumRecv.Bands()), 0.0, "recv spectrum should have data")
	assert.Greater(t, levelSend.Levels()[0], 0.0, "send level should have data")
	assert.Greater(t, levelRecv.Levels()[0], 0.0, "recv level should have data")
}

// TestIntegration_ConcurrentSpectrumAccess simulates multiple goroutines
// feeding the same spectrum/level (like conference mode where multiple
// clients decode into a shared analyzer) while TUI reads concurrently.
func TestIntegration_ConcurrentSpectrumAccess(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const numFrames = 30
	const numProducers = 4

	spectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	level := audio.NewLevelMeter(channels)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	opusPackets := encodePCMFrames(t, sampleRate, channels, pcmFrames)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var producerWg sync.WaitGroup

	// Multiple producer goroutines decoding simultaneously
	for p := 0; p < numProducers; p++ {
		producerWg.Add(1)
		go func() {
			defer producerWg.Done()
			inCh := make(chan []byte, len(opusPackets))
			for _, pkt := range opusPackets {
				cp := make([]byte, len(pkt))
				copy(cp, pkt)
				inCh <- cp
			}
			close(inCh)

			dec, err := audio.NewOpusDecoder(sampleRate, channels)
			if err != nil {
				t.Errorf("decoder error: %v", err)
				return
			}
			playbackCh := make(chan []float32, numFrames)
			decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, spectrum, level)
		}()
	}

	// Consumer goroutine simulating TUI reads at 10 fps
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = spectrum.Bands()
				_ = level.Levels()
				_ = spectrum.NumBands()
				_ = level.Channels()
			}
		}
	}()

	// Wait for producers, then cancel to stop TUI reader
	producerWg.Wait()
	cancel()
}

// TestIntegration_EncodeAccumulate_WithSpectrumFeed tests the capture-side
// pipeline: PCM → FrameAccumulator → Opus encode, with spectrum/level feeding.
// This simulates the CapturePipeline.Run loop without real audio hardware.
func TestIntegration_EncodeAccumulate_WithSpectrumFeed(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const numFrames = 50

	spectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	level := audio.NewLevelMeter(channels)

	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)
	require.NoError(t, enc.SetBitrate(64000))

	frameSize := sampleRate / 50 * channels
	acc := audio.NewFrameAccumulator(frameSize)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	sendCh := make(chan []byte, numFrames*2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate capture pipeline loop
	for _, samples := range pcmFrames {
		spectrum.Feed(samples)
		level.Feed(samples)

		frames := acc.Write(samples)
		for _, frame := range frames {
			encoded, err := enc.Encode(frame)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}
			select {
			case sendCh <- encoded:
			case <-ctx.Done():
				t.Fatal("timeout sending")
			}
		}
	}
	close(sendCh)

	// Count encoded packets
	count := 0
	for range sendCh {
		count++
	}
	assert.Greater(t, count, 0, "should produce encoded packets")

	// Spectrum should show 440Hz
	bands := spectrum.Bands()
	maxBand := 0.0
	for _, b := range bands {
		if b > maxBand {
			maxBand = b
		}
	}
	assert.Greater(t, maxBand, 0.1)
}

// TestIntegration_FullRoundTrip_EncodeDecodeWithAnalyzers tests the complete
// audio round trip: PCM → encode → "transport" → decode → spectrum/level.
// Two goroutines: encoder (producer) and decoder (consumer) connected by a channel.
func TestIntegration_FullRoundTrip_EncodeDecodeWithAnalyzers(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 2
	const numFrames = 100

	captureSpectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	captureLevel := audio.NewLevelMeter(channels)
	decodeSpectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	decodeLevel := audio.NewLevelMeter(channels)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)

	// Simulated transport channel (like WebRTC audio track)
	transportCh := make(chan []byte, numFrames)

	var wg sync.WaitGroup

	// Encoder goroutine (server capture side)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(transportCh)

		enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
		if err != nil {
			t.Errorf("encoder error: %v", err)
			return
		}
		if err := enc.SetBitrate(64000); err != nil {
			t.Errorf("set bitrate error: %v", err)
			return
		}

		frameSize := sampleRate / 50 * channels
		acc := audio.NewFrameAccumulator(frameSize)

		for _, samples := range pcmFrames {
			captureSpectrum.Feed(samples)
			captureLevel.Feed(samples)

			frames := acc.Write(samples)
			for _, frame := range frames {
				encoded, err := enc.Encode(frame)
				if err != nil {
					t.Errorf("encode error: %v", err)
					return
				}
				cp := make([]byte, len(encoded))
				copy(cp, encoded)
				transportCh <- cp
			}
		}
	}()

	// Decoder goroutine (client receive side)
	wg.Add(1)
	go func() {
		defer wg.Done()
		playbackCh := make(chan []float32, numFrames)
		dec, err := audio.NewOpusDecoder(sampleRate, channels)
		if err != nil {
			t.Errorf("decoder error: %v", err)
			return
		}
		decodeAudioStream(integrationLogger(), transportCh, dec, playbackCh, decodeSpectrum, decodeLevel)
	}()

	// TUI simulator — read analyzers concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = captureSpectrum.Bands()
			_ = captureLevel.Levels()
			_ = decodeSpectrum.Bands()
			_ = decodeLevel.Levels()
			time.Sleep(50 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Both sides should detect audio
	capBands := captureSpectrum.Bands()
	decBands := decodeSpectrum.Bands()
	capMax, decMax := 0.0, 0.0
	for _, b := range capBands {
		if b > capMax {
			capMax = b
		}
	}
	for _, b := range decBands {
		if b > decMax {
			decMax = b
		}
	}
	assert.Greater(t, capMax, 0.1, "capture spectrum should detect signal")
	assert.Greater(t, decMax, 0.1, "decode spectrum should detect signal")

	capLevels := captureLevel.Levels()
	decLevels := decodeLevel.Levels()
	assert.Greater(t, capLevels[0], 0.1, "capture level L should detect signal")
	assert.Greater(t, capLevels[1], 0.1, "capture level R should detect signal")
	assert.Greater(t, decLevels[0], 0.1, "decode level L should detect signal")
	assert.Greater(t, decLevels[1], 0.1, "decode level R should detect signal")
}

// TestIntegration_MixerToEncoder simulates multi-device capture:
// multiple PCM sources → AudioMixer → encode → decode, with spectrum feeding.
func TestIntegration_MixerToEncoder(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const numFrames = 50
	const numSources = 3

	frameSize := sampleRate / 50
	mixer := audio.NewAudioMixer(audio.MixerConfig{
		SampleRate:   sampleRate,
		Channels:     channels,
		FrameSize:    frameSize,
		BufferFrames: 3,
	})

	spectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	level := audio.NewLevelMeter(channels)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create source channels and feed them
	var wg sync.WaitGroup
	for s := 0; s < numSources; s++ {
		ch := make(chan []float32, numFrames)
		sourceID := string(rune('A' + s))
		mixer.AddSource(sourceID, ch)

		wg.Add(1)
		go func(ch chan []float32, freq float64) {
			defer wg.Done()
			defer close(ch)
			phase := 0.0
			dt := 1.0 / float64(sampleRate)
			for f := 0; f < numFrames; f++ {
				frame := make([]float32, frameSize)
				for i := range frame {
					frame[i] = float32(0.3 * math.Sin(2*math.Pi*freq*phase))
					phase += dt
				}
				select {
				case ch <- frame:
				case <-ctx.Done():
					return
				}
			}
		}(ch, 440.0+float64(s)*200) // different freq per source
	}

	// Start mixer
	go func() {
		if err := mixer.Run(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("mixer error: %v", err)
		}
	}()

	// Read mixed output, feed spectrum/level, encode
	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)
	require.NoError(t, enc.SetBitrate(64000))

	acc := audio.NewFrameAccumulator(frameSize * channels)
	transportCh := make(chan []byte, numFrames*2)

	mixedCount := 0
	timeout := time.After(8 * time.Second)

loop:
	for {
		select {
		case mixed, ok := <-mixer.Output():
			if !ok {
				break loop
			}
			spectrum.Feed(mixed)
			level.Feed(mixed)

			frames := acc.Write(mixed)
			mixer.PutMixedFrame(mixed)
			for _, frame := range frames {
				encoded, encErr := enc.Encode(frame)
				if encErr != nil {
					t.Logf("encode error: %v", encErr)
					continue
				}
				transportCh <- encoded
			}
			mixedCount++
			if mixedCount >= numFrames {
				break loop
			}
		case <-timeout:
			t.Logf("got %d mixed frames before timeout", mixedCount)
			break loop
		}
	}
	cancel()
	wg.Wait()
	close(transportCh)

	assert.Greater(t, mixedCount, 0, "should have mixed frames")

	// Decode the transported audio
	decSpectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	dec, err := audio.NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	playbackCh := make(chan []float32, numFrames)
	decodeAudioStream(integrationLogger(), transportCh, dec, playbackCh, decSpectrum, nil)

	assert.Greater(t, len(playbackCh), 0, "should have decoded frames")

	bands := spectrum.Bands()
	maxBand := 0.0
	for _, b := range bands {
		if b > maxBand {
			maxBand = b
		}
	}
	assert.Greater(t, maxBand, 0.0, "mixer output should have spectral content")
}

// TestIntegration_HighThroughput_NoRace runs many frames through the full
// pipeline at high speed to stress-test concurrent access patterns.
func TestIntegration_HighThroughput_NoRace(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 2
	const numFrames = 500 // 10 seconds

	spectrum := audio.NewSpectrumAnalyzer(sampleRate, 0)
	level := audio.NewLevelMeter(channels)

	pcmFrames := generateSineFrames(sampleRate, channels, numFrames)
	opusPackets := encodePCMFrames(t, sampleRate, channels, pcmFrames)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var producerWg sync.WaitGroup

	// Multiple concurrent decode streams into same analyzers
	for i := 0; i < 3; i++ {
		producerWg.Add(1)
		go func() {
			defer producerWg.Done()
			inCh := make(chan []byte, len(opusPackets))
			for _, pkt := range opusPackets {
				cp := make([]byte, len(pkt))
				copy(cp, pkt)
				inCh <- cp
			}
			close(inCh)

			dec, err := audio.NewOpusDecoder(sampleRate, channels)
			if err != nil {
				t.Errorf("decoder error: %v", err)
				return
			}
			playbackCh := make(chan []float32, numFrames)
			decodeAudioStream(integrationLogger(), inCh, dec, playbackCh, spectrum, level)
		}()
	}

	// Concurrent readers run while producers are working
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = spectrum.Bands()
				_ = level.Levels()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	producerWg.Wait()
	cancel()
}
