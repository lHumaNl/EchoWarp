package audio

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceBuffer_WriteRead_FIFO(t *testing.T) {
	sb := NewSourceBuffer(3, 4)

	frame1 := []float32{1.0, 2.0, 3.0, 4.0}
	frame2 := []float32{5.0, 6.0, 7.0, 8.0}

	sb.Write(frame1)
	sb.Write(frame2)

	result1 := sb.Read()
	assert.Equal(t, frame1, result1)

	result2 := sb.Read()
	assert.Equal(t, frame2, result2)
}

func TestSourceBuffer_Overwrite_OldestOnFull(t *testing.T) {
	sb := NewSourceBuffer(2, 2)

	frame1 := []float32{1.0, 1.0}
	frame2 := []float32{2.0, 2.0}
	frame3 := []float32{3.0, 3.0}

	sb.Write(frame1)
	sb.Write(frame2)
	sb.Write(frame3)

	result1 := sb.Read()
	assert.Equal(t, frame2, result1)

	result2 := sb.Read()
	assert.Equal(t, frame3, result2)

	result3 := sb.Read()
	assert.Nil(t, result3)
}

func TestSourceBuffer_ReadEmpty_ReturnsNil(t *testing.T) {
	sb := NewSourceBuffer(3, 4)

	result := sb.Read()
	assert.Nil(t, result)
}

func TestMixer_TwoSources_OutputIsSummed(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)
	mixer.SetNormalize(true) // enable 1/sqrt(N) normalization

	ch1 := make(chan []float32, 1)
	ch2 := make(chan []float32, 1)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)

	ch1 <- []float32{0.1, 0.2, 0.3, 0.4}
	ch2 <- []float32{0.1, 0.2, 0.3, 0.4}

	waitForMixerSources(t, mixer, 2, 100*time.Millisecond)

	frame := mixer.MixFrame()

	expected0 := float32(math.Tanh(0.2 / math.Sqrt(2)))
	expected1 := float32(math.Tanh(0.4 / math.Sqrt(2)))
	assert.InDelta(t, expected0, frame[0], 0.01)
	assert.InDelta(t, expected1, frame[1], 0.01)
}

func TestMixer_ThreeSources_Normalized(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)
	mixer.SetNormalize(true)

	ch1 := make(chan []float32, 1)
	ch2 := make(chan []float32, 1)
	ch3 := make(chan []float32, 1)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)
	mixer.AddSource("client3", ch3)

	ch1 <- []float32{0.5, 0.5}
	ch2 <- []float32{0.5, 0.5}
	ch3 <- []float32{0.5, 0.5}

	waitForMixerSources(t, mixer, 3, 100*time.Millisecond)

	frame := mixer.MixFrame()

	sumRaw := 0.5 + 0.5 + 0.5
	gain := 1.0 / math.Sqrt(3)
	normalized := sumRaw * gain
	expected := math.Tanh(normalized)

	assert.InDelta(t, expected, float64(frame[0]), 0.02)
}

func TestMixer_SourceRemoved_MixContinues(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 1)
	ch2 := make(chan []float32, 1)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)

	assert.Equal(t, 2, mixer.SourceCount())

	mixer.RemoveSource("client2")

	assert.Equal(t, 1, mixer.SourceCount())

	ch1 <- []float32{0.3, 0.3}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	expected := float32(math.Tanh(0.3))
	assert.InDelta(t, expected, frame[0], 0.01)
}

func TestMixer_NoSources_OutputIsSilence(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	frame := mixer.MixFrame()

	assert.Len(t, frame, 4)
	for _, sample := range frame {
		assert.InDelta(t, float32(0.0), sample, 0.0001)
	}
}

func TestMixer_SourceDelayed_UseSilence(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 1)
	mixer.AddSource("client1", ch1)

	frame := mixer.MixFrame()

	assert.Len(t, frame, 2)
	for _, sample := range frame {
		assert.InDelta(t, float32(0.0), sample, 0.0001)
	}

	ch1 <- []float32{0.5, 0.5}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame = mixer.MixFrame()
	expected := float32(math.Tanh(0.5))
	assert.InDelta(t, expected, frame[0], 0.01)
}

func TestMixer_ConcurrentAddRemove_NoRace(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	var wg sync.WaitGroup
	numOps := 100

	for i := 0; i < numOps; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			clientID := string(rune('a' + id%26))
			ch := make(chan []float32, 1)
			mixer.AddSource(clientID, ch)
		}(i)

		go func(id int) {
			defer wg.Done()
			clientID := string(rune('a' + id%26))
			mixer.RemoveSource(clientID)
		}(i + 13)
	}

	wg.Wait()
}

func TestMixer_SoftClipping_Tanh(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("loud", ch)

	ch <- []float32{5.0, -5.0}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()

	assert.LessOrEqual(t, frame[0], float32(1.0))
	assert.GreaterOrEqual(t, frame[0], float32(-1.0))
	assert.LessOrEqual(t, frame[1], float32(1.0))
	assert.GreaterOrEqual(t, frame[1], float32(-1.0))

	assert.Greater(t, frame[0], float32(0.95))
}

func TestMixer_Output_ReturnsChannel(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	outCh := mixer.Output()
	assert.NotNil(t, outCh)
}

func TestMixer_Run_ContextCancellation(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    960,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- mixer.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(time.Second):
		t.Error("Run did not exit on context cancellation")
	}
}

func TestMixer_Run_ProducesOutput(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    960,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mixer.Run(ctx)

	select {
	case frame := <-mixer.Output():
		assert.Len(t, frame, 960)
	case <-time.After(100 * time.Millisecond):
		t.Error("Run did not produce output frame in time")
	}
}

func TestMixer_AddSource_DuplicateIgnored(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 1)
	ch2 := make(chan []float32, 1)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client1", ch2)

	assert.Equal(t, 1, mixer.SourceCount())
}

func TestMixer_RemoveSource_NonExistent_NoPanic(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	assert.NotPanics(t, func() {
		mixer.RemoveSource("nonexistent")
	})
}

func TestMixer_DefaultBufferFrames(t *testing.T) {
	cfg := MixerConfig{
		SampleRate: 48000,
		Channels:   1,
		FrameSize:  4,
	}
	mixer := NewAudioMixer(cfg)

	assert.NotNil(t, mixer)
}

func TestMixer_MixFrame_MultipleActiveSources(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 2)
	ch2 := make(chan []float32, 2)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)

	ch1 <- []float32{0.3, 0.3}
	ch1 <- []float32{0.4, 0.4}
	ch2 <- []float32{0.2, 0.2}
	ch2 <- []float32{0.1, 0.1}

	waitForMixerSources(t, mixer, 2, 100*time.Millisecond)

	frame1 := mixer.MixFrame()
	assert.NotNil(t, frame1)

	frame2 := mixer.MixFrame()
	assert.NotNil(t, frame2)
}

func TestMixer_Run_ZeroFrameDuration(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   0,
		Channels:     1,
		FrameSize:    0,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- mixer.Run(ctx)
	}()

	select {
	case <-time.After(50 * time.Millisecond):
		cancel()
	}

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Error("Run did not exit")
	}
}

func TestMixer_SourceBuffer_ConcurrentWriteRead(t *testing.T) {
	sb := NewSourceBuffer(100, 4)
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			sb.Write([]float32{float32(i), float32(i + 1), float32(i + 2), float32(i + 3)})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			sb.Read()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestMixer_MixFrame_ShorterFrame(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    4,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("client1", ch)

	ch <- []float32{0.5, 0.5}

	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	assert.Len(t, frame, 4)
}

func TestSourceBuffer_Write_OverwriteBehavior(t *testing.T) {
	sb := NewSourceBuffer(2, 2)

	sb.Write([]float32{1.0, 1.0})
	sb.Write([]float32{2.0, 2.0})
	sb.Write([]float32{3.0, 3.0})
	sb.Write([]float32{4.0, 4.0})

	r1 := sb.Read()
	assert.Equal(t, []float32{3.0, 3.0}, r1)

	r2 := sb.Read()
	assert.Equal(t, []float32{4.0, 4.0}, r2)

	r3 := sb.Read()
	assert.Nil(t, r3)
}

func TestMixer_Run_OutputChannelFull(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    960,
		BufferFrames: 1,
	}
	mixer := NewAudioMixer(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- mixer.Run(ctx)
	}()

	select {
	case <-mixer.Output():
	case <-time.After(100 * time.Millisecond):
		t.Error("expected output from mixer")
	}
}

func TestMixer_MixFrame_SingleSource(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("solo", ch)

	ch <- []float32{0.5, 0.5}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	expected := float32(math.Tanh(0.5))
	assert.InDelta(t, expected, frame[0], 0.01)
}

func waitForMixerSources(t *testing.T, mixer *AudioMixer, expectedCount int, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		mixer.mu.RLock()
		hasData := 0
		for _, src := range mixer.sources {
			src.buffer.mu.Lock()
			if src.buffer.count > 0 {
				hasData++
			}
			src.buffer.mu.Unlock()
		}
		mixer.mu.RUnlock()
		return hasData >= expectedCount
	}, timeout, 2*time.Millisecond, "waiting for %d mixer sources to have data", expectedCount)
}

func TestMixer_PerSourceVolume(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSourceWithVolume("client1", ch, 0.5)

	ch <- []float32{1.0, 1.0}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	// Volume 0.5 applied before tanh: tanh(1.0 * 0.5) = tanh(0.5)
	expected := float32(math.Tanh(0.5))
	assert.InDelta(t, expected, frame[0], 0.01)
}

func TestMixer_SourceMuted(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("client1", ch)
	mixer.SetSourceMuted("client1", true)

	ch <- []float32{1.0, 1.0}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	// Muted source → silence (tanh(0) = 0)
	assert.InDelta(t, float32(0.0), frame[0], 0.001)
}

func TestMixer_GlobalMute(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)
	mixer.SetGlobalMute(true)

	ch := make(chan []float32, 1)
	mixer.AddSource("client1", ch)

	ch <- []float32{1.0, 1.0}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	assert.InDelta(t, float32(0.0), frame[0], 0.001)
}

func TestMixer_SetSourceVolume(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 2)
	mixer.AddSource("client1", ch)
	mixer.SetSourceVolume("client1", 2.0) // max volume

	ch <- []float32{0.3, 0.3}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	frame := mixer.MixFrame()
	expected := float32(math.Tanh(0.6)) // 0.3 * 2.0
	assert.InDelta(t, expected, frame[0], 0.01)
}

func TestMixer_StalledSources_Empty(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("client1", ch)

	ch <- []float32{0.1, 0.1}
	waitForMixerSources(t, mixer, 1, 100*time.Millisecond)

	stalled := mixer.StalledSources()
	assert.Empty(t, stalled)
}

func TestMixer_StalledSources_Detected(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch := make(chan []float32, 1)
	mixer.AddSource("client1", ch)

	// Force lastWrite to be old
	mixer.sources["client1"].lastWriteNs.Store(time.Now().Add(-100 * time.Millisecond).UnixNano())

	stalled := mixer.StalledSources()
	require.Len(t, stalled, 1)
	assert.True(t, stalled[0].IsStalled)
	assert.False(t, stalled[0].IsWarning)
}

func TestMixer_TwoSources_NoNormalize(t *testing.T) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     1,
		FrameSize:    2,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)
	// normalize is off by default

	ch1 := make(chan []float32, 1)
	ch2 := make(chan []float32, 1)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)

	ch1 <- []float32{0.1, 0.2}
	ch2 <- []float32{0.1, 0.2}

	waitForMixerSources(t, mixer, 2, 100*time.Millisecond)

	frame := mixer.MixFrame()
	// Without normalization: tanh(0.2), tanh(0.4)
	expected0 := float32(math.Tanh(0.2))
	expected1 := float32(math.Tanh(0.4))
	assert.InDelta(t, expected0, frame[0], 0.01)
	assert.InDelta(t, expected1, frame[1], 0.01)
}

func BenchmarkAudioMixer_MixFrame(b *testing.B) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     2,
		FrameSize:    960,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 10)
	ch2 := make(chan []float32, 10)
	ch3 := make(chan []float32, 10)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)
	mixer.AddSource("client3", ch3)

	frameSize := cfg.FrameSize * cfg.Channels
	for i := 0; i < 10; i++ {
		ch1 <- make([]float32, frameSize)
		ch2 <- make([]float32, frameSize)
		ch3 <- make([]float32, frameSize)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := mixer.MixFrame()
		mixer.PutMixedFrame(frame)
	}
}

func BenchmarkAudioMixer_MixFrame_NoPool(b *testing.B) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     2,
		FrameSize:    960,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	ch1 := make(chan []float32, 10)
	ch2 := make(chan []float32, 10)
	ch3 := make(chan []float32, 10)

	mixer.AddSource("client1", ch1)
	mixer.AddSource("client2", ch2)
	mixer.AddSource("client3", ch3)

	frameSize := cfg.FrameSize * cfg.Channels
	for i := 0; i < 10; i++ {
		ch1 <- make([]float32, frameSize)
		ch2 <- make([]float32, frameSize)
		ch3 <- make([]float32, frameSize)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mixer.MixFrame()
	}
}

func BenchmarkAudioMixer_MixFrame_NoSources(b *testing.B) {
	cfg := MixerConfig{
		SampleRate:   48000,
		Channels:     2,
		FrameSize:    960,
		BufferFrames: 3,
	}
	mixer := NewAudioMixer(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := mixer.MixFrame()
		mixer.PutMixedFrame(frame)
	}
}
