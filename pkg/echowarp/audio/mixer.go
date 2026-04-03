package audio

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxMixedFrameSize = 4096
)

var mixedFramePool = sync.Pool{
	New: func() interface{} {
		return make([]float32, 0, maxMixedFrameSize)
	},
}

func getMixedFrame(size int) []float32 {
	buf := mixedFramePool.Get().([]float32) //nolint:errcheck
	if cap(buf) < size {
		buf = make([]float32, size)
	} else {
		buf = buf[:size]
	}
	for i := range buf {
		buf[i] = 0
	}
	return buf
}

func putMixedFrame(buf []float32) {
	if cap(buf) > maxMixedFrameSize {
		return
	}
	mixedFramePool.Put(buf[:0]) //nolint:staticcheck // slices are pointer-like
}

// MixerConfig holds configuration parameters for the audio mixer.
type MixerConfig struct {
	// SampleRate is the audio sample rate in Hz (e.g., 48000).
	SampleRate uint32
	// Channels is the number of audio channels (1 for mono, 2 for stereo).
	Channels int
	// FrameSize is the number of samples per frame.
	FrameSize int
	// BufferFrames is the number of frames to buffer per source.
	BufferFrames int
}

// SourceBuffer is a thread-safe circular buffer for storing audio frames
// from a single source. It handles overflow by overwriting old frames.
type SourceBuffer struct {
	mu        sync.Mutex
	buf       [][]float32
	read      int
	write     int
	count     int
	capacity  int
	frameSize int
}

// NewSourceBuffer creates a circular buffer with the specified capacity and frame size.
func NewSourceBuffer(capacity, frameSize int) *SourceBuffer {
	return &SourceBuffer{
		buf:       make([][]float32, capacity),
		capacity:  capacity,
		frameSize: frameSize,
	}
}

// Write adds a frame to the buffer. If the buffer is full, the oldest frame
// is overwritten. The frame is copied internally.
func (sb *SourceBuffer) Write(frame []float32) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	frameCopy := getPCMBuffer(len(frame))
	frameCopy = frameCopy[:len(frame)]
	copy(frameCopy, frame)

	if sb.count < sb.capacity {
		sb.buf[sb.write] = frameCopy
		sb.write = (sb.write + 1) % sb.capacity
		sb.count++
	} else {
		// Return the overwritten buffer to the pool before replacing.
		if old := sb.buf[sb.write]; old != nil {
			putPCMBuffer(old)
		}
		sb.buf[sb.write] = frameCopy
		sb.write = (sb.write + 1) % sb.capacity
		sb.read = (sb.read + 1) % sb.capacity
	}
}

// Read retrieves and removes the oldest frame from the buffer.
// Returns nil if the buffer is empty.
func (sb *SourceBuffer) Read() []float32 {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.count == 0 {
		return nil
	}

	frame := sb.buf[sb.read]
	sb.buf[sb.read] = nil
	sb.read = (sb.read + 1) % sb.capacity
	sb.count--

	return frame
}

type mixerSource struct {
	buffer      *SourceBuffer
	cancel      context.CancelFunc
	volume      float32 // 0.0–2.0, default 1.0
	muted       bool
	lastWriteNs atomic.Int64 // UnixNano timestamp for stall detection (lock-free)
}

// AudioMixer combines audio from multiple sources into a single output stream.
// It handles automatic gain control and soft clipping to prevent distortion
// when mixing multiple sources.
//
// Thread-safe: sources can be added/removed concurrently with mixing.
type AudioMixer struct {
	mu         sync.RWMutex
	sources    map[string]*mixerSource
	outputCh   chan []float32
	config     MixerConfig
	globalMute bool
	normalize  bool // if true, apply 1/sqrt(N) normalization
}

// NewAudioMixer creates a mixer with the specified configuration.
// If BufferFrames is not set, a default of 3 is used.
func NewAudioMixer(cfg MixerConfig) *AudioMixer {
	if cfg.BufferFrames <= 0 {
		cfg.BufferFrames = 3
	}
	return &AudioMixer{
		sources:  make(map[string]*mixerSource),
		outputCh: make(chan []float32, cfg.BufferFrames),
		config:   cfg,
	}
}

// AddSource registers a new audio source identified by clientID.
// Audio from the provided channel is buffered and included in the mix.
// If clientID already exists, this is a no-op.
func (m *AudioMixer) AddSource(clientID string, ch <-chan []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sources[clientID]; exists {
		return
	}

	buffer := NewSourceBuffer(m.config.BufferFrames, m.config.FrameSize*m.config.Channels)
	ctx, cancel := context.WithCancel(context.Background())

	src := &mixerSource{
		buffer: buffer,
		cancel: cancel,
		volume: 1.0,
	}
	src.lastWriteNs.Store(time.Now().UnixNano())
	m.sources[clientID] = src

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-ch:
				if !ok {
					return
				}
				buffer.Write(frame)
				src.lastWriteNs.Store(time.Now().UnixNano())
			}
		}
	}()
}

// RemoveSource unregisters and stops buffering audio for the specified clientID.
func (m *AudioMixer) RemoveSource(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if src, exists := m.sources[clientID]; exists {
		src.cancel()
		delete(m.sources, clientID)
	}
}

// AddSourceWithVolume registers a new audio source with a specific volume multiplier.
func (m *AudioMixer) AddSourceWithVolume(clientID string, ch <-chan []float32, volume float32) {
	m.AddSource(clientID, ch)
	m.mu.Lock()
	if src, ok := m.sources[clientID]; ok {
		src.volume = volume
	}
	m.mu.Unlock()
}

// SetSourceVolume sets the volume multiplier (0.0-2.0) for a specific source.
func (m *AudioMixer) SetSourceVolume(clientID string, volume float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if src, ok := m.sources[clientID]; ok {
		src.volume = volume
	}
}

// SetSourceMuted sets the mute state for a specific source.
func (m *AudioMixer) SetSourceMuted(clientID string, muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if src, ok := m.sources[clientID]; ok {
		src.muted = muted
	}
}

// SetGlobalMute sets the global mute state. When globally muted, output is silence.
func (m *AudioMixer) SetGlobalMute(muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalMute = muted
}

// SetNormalize enables/disables 1/sqrt(N) normalization when mixing N sources.
func (m *AudioMixer) SetNormalize(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.normalize = enabled
}

// IsSourceMuted returns whether a specific source is muted.
func (m *AudioMixer) IsSourceMuted(clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if src, ok := m.sources[clientID]; ok {
		return src.muted
	}
	return false
}

// IsGlobalMuted returns whether global mute is active.
func (m *AudioMixer) IsGlobalMuted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalMute
}

// GetSourceVolume returns the volume of a specific source (0.0–2.0).
func (m *AudioMixer) GetSourceVolume(clientID string) float32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if src, ok := m.sources[clientID]; ok {
		return src.volume
	}
	return 1.0
}

// SourceStallInfo reports the stall state of a source.
type SourceStallInfo struct {
	ID         string
	StalledFor time.Duration
	IsStalled  bool // >40ms without data
	IsWarning  bool // >3s without data
}

// StalledSources returns info about sources that haven't provided data recently.
func (m *AudioMixer) StalledSources() []SourceStallInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	var stalled []SourceStallInfo
	for id, src := range m.sources {
		since := now.Sub(time.Unix(0, src.lastWriteNs.Load()))
		if since > 40*time.Millisecond {
			stalled = append(stalled, SourceStallInfo{
				ID:         id,
				StalledFor: since,
				IsStalled:  true,
				IsWarning:  since > 3*time.Second,
			})
		}
	}
	return stalled
}

// SourceCount returns the number of registered audio sources.
func (m *AudioMixer) SourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sources)
}

// Output returns the channel for receiving mixed audio frames.
func (m *AudioMixer) Output() <-chan []float32 {
	return m.outputCh
}

// MixFrame combines all available source frames into a single mixed frame.
// Applies per-source volume and mute, optional normalization (1/sqrt(N)),
// and tanh soft clipping to prevent distortion.
func (m *AudioMixer) MixFrame() []float32 {
	totalSamples := m.config.FrameSize * m.config.Channels
	mixed := getMixedFrame(totalSamples)

	m.mu.RLock()
	globalMute := m.globalMute
	normalize := m.normalize

	if globalMute {
		m.mu.RUnlock()
		return mixed
	}

	activeCount := 0
	for _, src := range m.sources {
		if src.muted {
			// Drain buffer to avoid stale frames on unmute
			if drained := src.buffer.Read(); drained != nil {
				putPCMBuffer(drained)
			}
			continue
		}
		frame := src.buffer.Read()
		if frame != nil {
			activeCount++
			// Apply per-source volume before accumulation
			if src.volume != 1.0 {
				MixGain(frame, src.volume)
			}
			MixAccumulate(mixed, frame)
			putPCMBuffer(frame)
		}
	}
	m.mu.RUnlock()

	if activeCount > 1 && normalize {
		gain := float32(1.0 / math.Sqrt(float64(activeCount)))
		MixGain(mixed, gain)
	}

	MixTanh(mixed)

	return mixed
}

func (m *AudioMixer) PutMixedFrame(buf []float32) {
	putMixedFrame(buf)
}

// Run starts the mixing loop, producing mixed frames at the configured frame rate.
// Mixed frames are sent to the output channel. Blocks until ctx is canceled.
func (m *AudioMixer) Run(ctx context.Context) error {
	frameDuration := time.Duration(float64(m.config.FrameSize) / float64(m.config.SampleRate) * float64(time.Second))
	if frameDuration <= 0 {
		frameDuration = 20 * time.Millisecond
	}
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frame := m.MixFrame()
			select {
			case m.outputCh <- frame:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
