package app

import "sync/atomic"

func newMonitorSourceBuffer(capacity int, paused *atomic.Bool) *monitorSourceBuffer {
	return &monitorSourceBuffer{
		buf:    make([][]float32, capacity),
		gain:   NewDeviceGainControl(1.0),
		paused: paused,
	}
}

func firstPauseFlag(flags []*atomic.Bool) *atomic.Bool {
	if len(flags) == 0 {
		return nil
	}
	return flags[0]
}

func (b *monitorSourceBuffer) write(frame []float32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	copyFrame := append([]float32(nil), frame...)
	if b.count == len(b.buf) {
		b.read = (b.read + 1) % len(b.buf)
		b.count--
	}
	b.buf[b.writeIdx] = copyFrame
	b.writeIdx = (b.writeIdx + 1) % len(b.buf)
	b.count++
}

func (b *monitorSourceBuffer) readFrame() []float32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count == 0 {
		return nil
	}
	frame := b.buf[b.read]
	b.buf[b.read] = nil
	b.read = (b.read + 1) % len(b.buf)
	b.count--
	return frame
}

func (b *monitorSourceBuffer) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.buf {
		b.buf[i] = nil
	}
	b.read = 0
	b.writeIdx = 0
	b.count = 0
}

func (b *monitorSourceBuffer) isPaused() bool {
	return b.paused != nil && b.paused.Load()
}
