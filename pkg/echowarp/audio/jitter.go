package audio

import (
	"sync"
)

// maxPooledFrameSize is the maximum frame size that will be pooled for reuse.
// 1920 samples = 40ms at 48kHz stereo, sufficient for all common frame sizes.
const maxPooledFrameSize = 1920

// framePool reuses frame buffers in the jitter buffer to reduce GC pressure.
var framePool = sync.Pool{
	New: func() interface{} {
		return make([]float32, maxPooledFrameSize)
	},
}

// JitterBuffer smooths out network packet timing variations by buffering frames
// before playback. Uses a ring buffer to avoid slice shifting and memory leaks.
//
// Thread-safe: concurrent Read/Write operations are protected by internal mutex.
type JitterBuffer struct {
	mu          sync.Mutex
	ring        [][]float32 // fixed-size ring buffer
	read        int         // read index
	write       int         // write index
	count       int         // current number of frames
	targetDepth int
	maxDepth    int
	minDepth    int
	initialized bool
}

// NewJitterBuffer creates a jitter buffer with the specified depth parameters.
// targetFrames: initial buffer depth (e.g., 3-5 frames for typical networks).
// maxFrames: maximum buffer depth to prevent excessive latency.
func NewJitterBuffer(targetFrames, maxFrames int) *JitterBuffer {
	minDepth := targetFrames * 2
	if minDepth > maxFrames {
		minDepth = maxFrames
	}
	return &JitterBuffer{
		ring:        make([][]float32, maxFrames),
		targetDepth: targetFrames,
		maxDepth:    maxFrames,
		minDepth:    minDepth,
	}
}

// Write adds a frame to the jitter buffer. If the buffer is full,
// the oldest frame is dropped to prevent unbounded growth.
// The frame is copied to an internally pooled buffer.
func (j *JitterBuffer) Write(frame []float32) {
	j.mu.Lock()
	defer j.mu.Unlock()

	var frameCopy []float32
	if len(frame) <= maxPooledFrameSize {
		pooledFrame := framePool.Get().([]float32) //nolint:errcheck
		frameCopy = pooledFrame[:len(frame)]
	} else {
		frameCopy = make([]float32, len(frame))
	}
	copy(frameCopy, frame)

	if j.count == j.maxDepth {
		// Buffer full — drop oldest frame
		dropped := j.ring[j.read]
		j.ring[j.read] = nil
		if dropped != nil && cap(dropped) <= maxPooledFrameSize {
			framePool.Put(dropped[:cap(dropped)]) //nolint:staticcheck // slices are pointer-like
		}
		j.read = (j.read + 1) % j.maxDepth
		j.count--
	}

	j.ring[j.write] = frameCopy
	j.write = (j.write + 1) % j.maxDepth
	j.count++
}

// Read retrieves the oldest frame from the buffer. Returns nil if the buffer
// is empty or hasn't reached minimum depth during initial buffering period.
// The caller owns the returned slice (pooled copy from Write).
func (j *JitterBuffer) Read() []float32 {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.count == 0 {
		return nil
	}

	if !j.initialized && j.count < j.minDepth {
		return nil
	}

	if !j.initialized {
		j.initialized = true
	}

	frame := j.ring[j.read]
	j.ring[j.read] = nil
	j.read = (j.read + 1) % j.maxDepth
	j.count--

	return frame
}

// Depth returns the current number of frames in the buffer.
func (j *JitterBuffer) Depth() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.count
}

// AdaptiveAdjust modifies the target buffer depth based on observed packet loss.
// When packet loss is detected, depth increases to improve resilience.
// When no loss is observed, depth decreases to reduce latency.
func (j *JitterBuffer) AdaptiveAdjust(packetLoss bool) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if packetLoss {
		if j.targetDepth < j.maxDepth-1 {
			j.targetDepth++
		}
	} else {
		if j.targetDepth > 2 {
			j.targetDepth--
		}
	}

	j.minDepth = j.targetDepth - 1
	if j.minDepth < 1 {
		j.minDepth = 1
	}
}
