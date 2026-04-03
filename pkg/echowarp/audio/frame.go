package audio

// FrameAccumulator buffers PCM samples and outputs exactly frameSize samples per frame.
// This is necessary because malgo callbacks do not guarantee exactly 960 samples (20ms at 48kHz).
type FrameAccumulator struct {
	buf       []float32
	frameSize int
	frameBuf  [4][]float32 // pre-allocated for up to 4 frames without allocation
}

// NewFrameAccumulator creates a new FrameAccumulator.
// frameSize: number of samples per frame (e.g., 960 for 48kHz/20ms mono, 1920 for stereo).
func NewFrameAccumulator(frameSize int) *FrameAccumulator {
	// Pre-allocate 2x frameSize to reduce reallocations during normal operation.
	return &FrameAccumulator{
		buf:       make([]float32, 0, frameSize*2),
		frameSize: frameSize,
	}
}

// Write adds PCM samples to the buffer.
// Returns complete frames (each of frameSize samples) if enough data has accumulated.
func (a *FrameAccumulator) Write(samples []float32) [][]float32 {
	if len(samples) == 0 {
		return nil
	}

	a.buf = append(a.buf, samples...)

	numFrames := len(a.buf) / a.frameSize
	if numFrames == 0 {
		return nil
	}

	var frames [][]float32
	if numFrames <= len(a.frameBuf) {
		frames = a.frameBuf[:numFrames]
	} else {
		frames = make([][]float32, numFrames)
	}
	for i := 0; i < numFrames; i++ {
		frame := getPCMBuffer(a.frameSize)
		frame = append(frame, a.buf[i*a.frameSize:(i+1)*a.frameSize]...)
		frames[i] = frame
	}

	remaining := len(a.buf) - numFrames*a.frameSize
	if remaining > 0 {
		copy(a.buf[:remaining], a.buf[numFrames*a.frameSize:])
		a.buf = a.buf[:remaining]
	} else {
		a.buf = a.buf[:0]
	}

	return frames
}

// Reset clears the internal buffer, discarding any accumulated samples.
func (a *FrameAccumulator) Reset() {
	a.buf = a.buf[:0]
}

// Buffered returns the number of samples currently in the buffer.
func (a *FrameAccumulator) Buffered() int {
	return len(a.buf)
}
