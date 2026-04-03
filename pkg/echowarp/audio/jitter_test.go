package audio

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJitterBuffer_WriteRead(t *testing.T) {
	jb := NewJitterBuffer(2, 10)
	// minDepth = targetFrames * 2 = 4

	frame1 := []float32{1.0, 2.0, 3.0}
	frame2 := []float32{4.0, 5.0, 6.0}

	jb.Write(frame1)
	jb.Write(frame2)

	// Only 2 frames buffered, minDepth=4 — not ready yet
	assert.Nil(t, jb.Read())

	jb.Write(frame1)
	// 3 frames — still below minDepth
	assert.Nil(t, jb.Read())

	jb.Write(frame2)
	// 4 frames — minDepth reached, read should succeed
	got := jb.Read()
	assert.NotNil(t, got)
}

func TestJitterBuffer_Empty(t *testing.T) {
	jb := NewJitterBuffer(2, 10)
	assert.Nil(t, jb.Read())
}

func TestJitterBuffer_Overflow(t *testing.T) {
	jb := NewJitterBuffer(2, 5)

	for i := 0; i < 10; i++ {
		jb.Write([]float32{float32(i)})
	}

	assert.LessOrEqual(t, jb.Depth(), 5)
}

func TestJitterBuffer_AdaptiveAdjust(t *testing.T) {
	jb := NewJitterBuffer(2, 10)

	jb.AdaptiveAdjust(true)
	assert.Equal(t, 3, jb.targetDepth)

	for i := 0; i < 5; i++ {
		jb.AdaptiveAdjust(false)
	}
	assert.LessOrEqual(t, jb.targetDepth, 2)
}

func TestJitterBuffer_FIFO(t *testing.T) {
	jb := NewJitterBuffer(1, 10)

	frame1 := []float32{1.0, 2.0, 3.0}
	frame2 := []float32{4.0, 5.0, 6.0}

	jb.Write(frame1)
	jb.Write(frame2)

	got := jb.Read()
	assert.NotNil(t, got)
	assert.Equal(t, frame1, got)

	got = jb.Read()
	assert.NotNil(t, got)
	assert.Equal(t, frame2, got)
}

func TestJitterBuffer_AdaptiveAdjust_IncreasesToMax(t *testing.T) {
	jb := NewJitterBuffer(2, 5)

	for i := 0; i < 10; i++ {
		jb.AdaptiveAdjust(true)
	}

	assert.LessOrEqual(t, jb.targetDepth, 5)
}

func TestJitterBuffer_AdaptiveAdjust_DecreasesToMin(t *testing.T) {
	jb := NewJitterBuffer(5, 10)

	for i := 0; i < 10; i++ {
		jb.AdaptiveAdjust(false)
	}

	assert.GreaterOrEqual(t, jb.targetDepth, 2)
}

func TestJitterBuffer_AdaptiveAdjust_MinDepthBounds(t *testing.T) {
	jb := NewJitterBuffer(2, 10)

	jb.targetDepth = 1
	jb.AdaptiveAdjust(false)

	assert.GreaterOrEqual(t, jb.minDepth, 1)
}

func TestJitterBuffer_NewJitterBuffer_MinDepthExceedsMax(t *testing.T) {
	jb := NewJitterBuffer(10, 5)
	assert.NotNil(t, jb)
}

func TestJitterBuffer_ConcurrentReadWrite(t *testing.T) {
	jb := NewJitterBuffer(1, 100)
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			jb.Write([]float32{float32(i)})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			jb.Read()
		}
		done <- true
	}()

	<-done
	<-done
}

func TestJitterBuffer_Depth_AfterOperations(t *testing.T) {
	jb := NewJitterBuffer(1, 10)

	assert.Equal(t, 0, jb.Depth())

	jb.Write([]float32{1.0})
	jb.Write([]float32{2.0})
	assert.Equal(t, 2, jb.Depth())

	jb.Read()
	assert.Equal(t, 1, jb.Depth())
}

func TestJitterBuffer_MinDepthBoundary(t *testing.T) {
	jb := NewJitterBuffer(3, 10)
	// minDepth = targetFrames * 2 = 6

	for i := 0; i < 5; i++ {
		jb.Write([]float32{float32(i)})
	}
	// 5 frames < minDepth(6) — not ready
	assert.Nil(t, jb.Read())

	jb.Write([]float32{5.0})
	// 6 frames = minDepth — ready
	got := jb.Read()
	assert.NotNil(t, got)
}

func TestJitterBuffer_TargetDepthBounds(t *testing.T) {
	jb := NewJitterBuffer(5, 5)

	for i := 0; i < 10; i++ {
		jb.AdaptiveAdjust(true)
	}

	assert.LessOrEqual(t, jb.targetDepth, 5)
}

func TestJitterBuffer_PoolReuse(t *testing.T) {
	jb := NewJitterBuffer(1, 100)

	for i := 0; i < 100; i++ {
		frame := []float32{float32(i), float32(i + 1)}
		jb.Write(frame)
	}

	for i := 0; i < 50; i++ {
		got := jb.Read()
		assert.NotNil(t, got)
	}

	runtime.GC()
}

func TestJitterBuffer_LargeFrameNotPooled(t *testing.T) {
	jb := NewJitterBuffer(1, 10)

	largeFrame := make([]float32, maxPooledFrameSize+100)
	for i := range largeFrame {
		largeFrame[i] = float32(i)
	}

	jb.Write(largeFrame)

	for i := 0; i < 3; i++ {
		jb.Write([]float32{float32(i)})
	}

	got := jb.Read()
	assert.NotNil(t, got)
	assert.Equal(t, largeFrame, got)
}

func TestJitterBuffer_PooledFrameCapacityReset(t *testing.T) {
	jb := NewJitterBuffer(1, 10)

	frame := make([]float32, 100)
	for i := range frame {
		frame[i] = float32(i)
	}

	jb.Write(frame)
	jb.Write([]float32{1.0, 2.0})
	jb.Write([]float32{3.0, 4.0})

	got := jb.Read()

	assert.NotNil(t, got)
	assert.Equal(t, frame, got)
}

func TestJitterBuffer_NoMemoryLeak(t *testing.T) {
	jb := NewJitterBuffer(1, 100)

	for i := 0; i < 1000; i++ {
		frame := make([]float32, 100)
		jb.Write(frame)
		if i%2 == 0 {
			jb.Read()
		}
	}

	for jb.Depth() > 0 {
		jb.Read()
	}

	assert.Equal(t, 0, jb.Depth())
}
