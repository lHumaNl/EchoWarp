package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameAccumulator_Write(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		frameSize     int
		inputs        [][]float32
		expectedCount int
		expected      [][]float32
	}{
		{
			"exact frame returns one",
			4,
			[][]float32{{1.0, 2.0, 3.0, 4.0}},
			1,
			[][]float32{{1.0, 2.0, 3.0, 4.0}},
		},
		{
			"half frame returns none",
			4,
			[][]float32{{1.0, 2.0}},
			0,
			nil,
		},
		{
			"empty write returns none",
			4,
			[][]float32{{}},
			0,
			nil,
		},
		{
			"nil write returns none",
			4,
			[][]float32{nil},
			0,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := NewFrameAccumulator(tt.frameSize)
			var frames [][]float32
			for _, input := range tt.inputs {
				frames = acc.Write(input)
			}
			require.Len(t, frames, tt.expectedCount)
			if tt.expected != nil {
				for i, expected := range tt.expected {
					assert.Equal(t, expected, frames[i])
				}
			}
		})
	}
}

func TestFrameAccumulator_TwoAndHalf_ReturnsTwoFrames(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	frames := acc.Write([]float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0})
	require.Len(t, frames, 2)
	assert.Equal(t, []float32{1.0, 2.0, 3.0, 4.0}, frames[0])
	assert.Equal(t, []float32{5.0, 6.0, 7.0, 8.0}, frames[1])
}

func TestFrameAccumulator_SmallCallbacks_AccumulatesCorrectly(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)

	// First write: 1 sample, not enough
	frames := acc.Write([]float32{1.0})
	assert.Empty(t, frames)

	// Second write: 2 more samples, still not enough (total 3)
	frames = acc.Write([]float32{2.0, 3.0})
	assert.Empty(t, frames)

	// Third write: 1 more sample, now we have 4 = 1 frame
	frames = acc.Write([]float32{4.0})
	require.Len(t, frames, 1)
	assert.Equal(t, []float32{1.0, 2.0, 3.0, 4.0}, frames[0])
}

func TestFrameAccumulator_MultipleSmallWrites_ProducesFrames(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)

	// Write 3 samples
	frames := acc.Write([]float32{1.0, 2.0, 3.0})
	assert.Empty(t, frames)

	// Write 5 more (total 8 = 2 frames)
	frames = acc.Write([]float32{4.0, 5.0, 6.0, 7.0, 8.0})
	require.Len(t, frames, 2)
	assert.Equal(t, []float32{1.0, 2.0, 3.0, 4.0}, frames[0])
	assert.Equal(t, []float32{5.0, 6.0, 7.0, 8.0}, frames[1])
}

func TestFrameAccumulator_LargeWrite_ReturnsManyFrames(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(2)
	samples := make([]float32, 7)
	for i := range samples {
		samples[i] = float32(i + 1)
	}
	frames := acc.Write(samples) // 7 samples, frameSize=2 → 3 frames, 1 leftover
	require.Len(t, frames, 3)
	assert.Equal(t, []float32{1.0, 2.0}, frames[0])
	assert.Equal(t, []float32{3.0, 4.0}, frames[1])
	assert.Equal(t, []float32{5.0, 6.0}, frames[2])

	// The remaining 1 sample (7.0) should be buffered
	frames = acc.Write([]float32{8.0})
	require.Len(t, frames, 1)
	assert.Equal(t, []float32{7.0, 8.0}, frames[0])
}

func TestFrameAccumulator_RealisticFrameSize(t *testing.T) {
	t.Parallel()
	// 960 samples = 20ms at 48kHz mono (realistic Opus frame)
	acc := NewFrameAccumulator(960)

	// Simulate malgo callback with 480 samples (10ms)
	samples480 := make([]float32, 480)
	for i := range samples480 {
		samples480[i] = float32(i) / 480.0
	}

	frames := acc.Write(samples480)
	assert.Empty(t, frames, "480 samples should not produce a 960-sample frame")

	frames = acc.Write(samples480)
	require.Len(t, frames, 1, "960 total samples should produce exactly 1 frame")
	assert.Len(t, frames[0], 960)
}

func TestFrameAccumulator_Reset(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	acc.Write([]float32{1.0, 2.0}) // Buffer has 2 samples
	acc.Reset()
	frames := acc.Write([]float32{3.0, 4.0, 5.0, 6.0})
	require.Len(t, frames, 1)
	assert.Equal(t, []float32{3.0, 4.0, 5.0, 6.0}, frames[0]) // Old samples discarded
}

func TestFrameAccumulator_Buffered(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	assert.Equal(t, 0, acc.Buffered())
	acc.Write([]float32{1.0, 2.0})
	assert.Equal(t, 2, acc.Buffered())
	acc.Write([]float32{3.0, 4.0})
	assert.Equal(t, 0, acc.Buffered()) // Frame was emitted, buffer empty
}

func BenchmarkFrameAccumulator_Write(b *testing.B) {
	acc := NewFrameAccumulator(960)
	samples := make([]float32, 480)
	for i := range samples {
		samples[i] = float32(i) / 480.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frames := acc.Write(samples)
		for _, f := range frames {
			putPCMBuffer(f)
		}
	}
}

var globalBuf []float32

func BenchmarkPCMBufferPool(b *testing.B) {
	data := make([]float32, 960)

	b.Run("with_pool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := getPCMBuffer(960)
			buf = append(buf, data...)
			globalBuf = buf
			putPCMBuffer(buf)
		}
	})

	b.Run("without_pool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := make([]float32, 960)
			copy(buf, data)
			globalBuf = buf
		}
	})
}

func TestFrameAccumulator_WriteNil(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	frames := acc.Write(nil)
	assert.Empty(t, frames)
}

func TestFrameAccumulator_ExactMultiple(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(3)

	frames := acc.Write([]float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0})
	require.Len(t, frames, 2)
	assert.Equal(t, []float32{1.0, 2.0, 3.0}, frames[0])
	assert.Equal(t, []float32{4.0, 5.0, 6.0}, frames[1])

	assert.Equal(t, 0, acc.Buffered())
}

func TestFrameAccumulator_BufferedAfterPartial(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	acc.Write([]float32{1.0, 2.0, 3.0, 4.0, 5.0})

	assert.Equal(t, 1, acc.Buffered())
}

func TestFrameAccumulator_ResetClearsBuffer(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(4)
	acc.Write([]float32{1.0, 2.0, 3.0})
	assert.Equal(t, 3, acc.Buffered())

	acc.Reset()
	assert.Equal(t, 0, acc.Buffered())
}

func TestFrameAccumulator_LargeFrameSize(t *testing.T) {
	t.Parallel()
	acc := NewFrameAccumulator(1920)

	frames := acc.Write(make([]float32, 960))
	assert.Empty(t, frames)
	assert.Equal(t, 960, acc.Buffered())

	frames = acc.Write(make([]float32, 960))
	require.Len(t, frames, 1)
	assert.Len(t, frames[0], 1920)
	assert.Equal(t, 0, acc.Buffered())
}
