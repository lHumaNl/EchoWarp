package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPCMBuffer_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	buf := getPCMBuffer(100)
	assert.NotNil(t, buf)
	assert.Len(t, buf, 0)
}

func TestGetPCMBuffer_SmallSize(t *testing.T) {
	t.Parallel()
	buf := getPCMBuffer(100)
	assert.GreaterOrEqual(t, cap(buf), 100)
}

func TestGetPCMBuffer_LargerThanDefault(t *testing.T) {
	t.Parallel()
	buf := getPCMBuffer(5000)
	assert.GreaterOrEqual(t, cap(buf), 5000)
}

func TestPutPCMBuffer_SmallBuffer(t *testing.T) {
	t.Parallel()
	buf := make([]float32, 0, 1000)
	assert.NotPanics(t, func() {
		putPCMBuffer(buf)
	})
}

func TestPutPCMBuffer_MaxSizeBuffer(t *testing.T) {
	t.Parallel()
	buf := make([]float32, 0, maxPCMBufferSize)
	assert.NotPanics(t, func() {
		putPCMBuffer(buf)
	})
}

func TestPutPCMBuffer_TooLargeBuffer_Dropped(t *testing.T) {
	t.Parallel()
	buf := make([]float32, 0, maxPCMBufferSize+1000)
	assert.NotPanics(t, func() {
		putPCMBuffer(buf)
	})
}

func TestGetPutPCMBuffer_Roundtrip(t *testing.T) {
	t.Parallel()
	buf := getPCMBuffer(500)
	buf = append(buf, 0.5, 0.6, 0.7)

	putPCMBuffer(buf)

	buf2 := getPCMBuffer(500)
	assert.NotNil(t, buf2)
	assert.Len(t, buf2, 0)
}

func TestGetPCMBuffer_ZeroSize(t *testing.T) {
	t.Parallel()
	buf := getPCMBuffer(0)
	assert.NotNil(t, buf)
}

func TestGetPCMBuffer_Concurrent(t *testing.T) {
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			buf := getPCMBuffer(100)
			putPCMBuffer(buf)
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestWarmupPools_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		WarmupPools()
	})
}

func TestWarmupPools_Idempotent(t *testing.T) {
	// WarmupPools can be called multiple times without issues
	WarmupPools()
	WarmupPools()
	WarmupPools()

	// Verify pools still work after warmup
	buf := getPCMBuffer(100)
	assert.NotNil(t, buf)
	putPCMBuffer(buf)
}

func TestWarmupPools_AllPoolsFunctional(t *testing.T) {
	WarmupPools()

	// Test PCM buffer pool
	pcmBuf := getPCMBuffer(defaultPCMBufferSize)
	assert.NotNil(t, pcmBuf)
	putPCMBuffer(pcmBuf)

	// Test mixed frame pool
	mixedBuf := getMixedFrame(maxMixedFrameSize)
	assert.NotNil(t, mixedBuf)
	putMixedFrame(mixedBuf)

	// Test opus encode pool
	opusBuf := opusEncodePool.Get().([]byte)
	assert.NotNil(t, opusBuf)
	opusEncodePool.Put(opusBuf)

	// Test jitter frame pool
	frameBuf := framePool.Get().([]float32)
	assert.NotNil(t, frameBuf)
	framePool.Put(frameBuf)
}

// TestPCMBufferPool_ReusesBuffers verifies that sync.Pool actually reuses buffers.
// TestPCMBufferPool_ReusesBuffers verifies that pcmBufferPool returns buffers with correct capacity
// and that put/get cycle works without errors. sync.Pool reuse is best-effort, so we don't assert pointer equality.
func TestPCMBufferPool_ReusesBuffers(t *testing.T) {
	const frameSize = 100

	buf1 := getPCMBuffer(frameSize)
	assert.GreaterOrEqual(t, cap(buf1), frameSize, "buffer should have sufficient capacity")

	putPCMBuffer(buf1)

	buf2 := getPCMBuffer(frameSize)
	assert.GreaterOrEqual(t, cap(buf2), frameSize, "buffer should have sufficient capacity")

	putPCMBuffer(buf2)
}

// TestMixedFramePool_ReusesBuffers verifies that mixedFramePool returns buffers with correct size.
func TestMixedFramePool_ReusesBuffers(t *testing.T) {
	const frameSize = 100

	buf1 := getMixedFrame(frameSize)
	assert.Len(t, buf1, frameSize, "buffer should have requested length")

	putMixedFrame(buf1)

	buf2 := getMixedFrame(frameSize)
	assert.Len(t, buf2, frameSize, "buffer should have requested length")

	// Cleanup
	putMixedFrame(buf2)
}

func BenchmarkWarmupPools(b *testing.B) {
	for i := 0; i < b.N; i++ {
		WarmupPools()
	}
}
