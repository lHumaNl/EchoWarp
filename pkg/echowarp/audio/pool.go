package audio

import "sync"

const (
	// defaultPCMBufferSize is the default capacity for pooled PCM audio buffers.
	//
	// Magic number explanation:
	//   1920 samples = 40ms at 48kHz stereo (2 channels × 960 samples per channel)
	//   - 48kHz sample rate: 48000 samples/sec per channel
	//   - Stereo: 2 channels
	//   - 20ms frame: 48000 × 0.020 = 960 samples per channel
	//   - 2 frames: 960 × 2 = 1920 total samples
	//
	// This accommodates two Opus frames of 20ms each, which is a common encoding unit
	// for real-time audio streaming. WebRTC/Opus typically uses 20ms frames.
	defaultPCMBufferSize = 1920

	// maxPCMBufferSize is the maximum buffer capacity that will be retained in the pool.
	//
	// Magic number explanation:
	//   4096 samples ≈ 85ms at 48kHz stereo
	//   - ~4.27× the default 1920 samples
	//   - Prevents memory bloat from unusually large allocations
	//   - Allows for larger frames (e.g., 4× 20ms = 80ms) while keeping memory bounded
	//
	// Buffers larger than this are discarded rather than pooled to prevent unbounded
	// memory growth. This trade-off prioritizes memory efficiency over allocation reuse
	// for exceptionally large buffers.
	maxPCMBufferSize = 4096

	// warmupItemsPerPool is the number of items to pre-allocate per pool during warmup.
	// This value balances startup latency reduction against memory overhead.
	warmupItemsPerPool = 3
)

var pcmBufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]float32, 0, defaultPCMBufferSize)
		return buf
	},
}

// getPCMBuffer retrieves a PCM buffer from the pool with at least the requested capacity.
// The returned slice has length 0 but capacity >= size. After use, return the buffer
// to the pool with putPCMBuffer.
func getPCMBuffer(size int) []float32 {
	buf := pcmBufferPool.Get().([]float32) //nolint:errcheck
	if cap(buf) < size {
		buf = make([]float32, 0, size)
	}
	return buf[:0]
}

// putPCMBuffer returns a PCM buffer to the pool for reuse.
// Buffers larger than maxPCMBufferSize are discarded to prevent memory bloat.
// The buffer is reset to length 0 before pooling.
func putPCMBuffer(buf []float32) {
	if cap(buf) > maxPCMBufferSize {
		return
	}
	pcmBufferPool.Put(buf[:0]) //nolint:staticcheck // slices are pointer-like
}

// WarmupPools pre-allocates items in all audio sync.Pools to avoid latency spikes
// during first allocations. This should be called during application startup.
//
// The function warms up the following pools:
//   - pcmBufferPool: PCM audio buffers
//   - mixedFramePool: Mixed audio frames
//   - opusEncodePool: Opus encoding buffers
//   - framePool: Jitter buffer frames
//
// Each pool gets warmupItemsPerPool items pre-allocated, which is typically enough
// to cover the initial burst of allocations without significant memory overhead.
func WarmupPools() {
	// Warm up PCM buffer pool
	for i := 0; i < warmupItemsPerPool; i++ {
		putPCMBuffer(getPCMBuffer(defaultPCMBufferSize))
	}

	// Warm up mixed frame pool
	for i := 0; i < warmupItemsPerPool; i++ {
		putMixedFrame(getMixedFrame(maxMixedFrameSize))
	}

	// Warm up Opus encode pool
	for i := 0; i < warmupItemsPerPool; i++ {
		buf := opusEncodePool.Get().([]byte) //nolint:errcheck
		opusEncodePool.Put(buf)              //nolint:staticcheck // slices are pointer-like
	}

	// Warm up Opus output pool
	for i := 0; i < warmupItemsPerPool; i++ {
		PutOpusOutput(getOpusOutput(defaultOpusOutputSize))
	}

	// Warm up jitter frame pool
	for i := 0; i < warmupItemsPerPool; i++ {
		buf := framePool.Get().([]float32) //nolint:errcheck
		framePool.Put(buf)                 //nolint:staticcheck // slices are pointer-like
	}
}
