package tui

import "sync"

// PauseState tracks per-device pause state for capture devices.
// Thread-safe: all methods are safe for concurrent use.
type PauseState struct {
	mu      sync.RWMutex
	paused  map[string]bool // deviceID → paused
	devices []string        // ordered device IDs
}

// NewPauseState creates a new PauseState for the given capture device IDs.
func NewPauseState(deviceIDs []string) *PauseState {
	paused := make(map[string]bool, len(deviceIDs))
	for _, id := range deviceIDs {
		paused[id] = false
	}
	return &PauseState{
		paused:  paused,
		devices: append([]string(nil), deviceIDs...),
	}
}

// Toggle toggles the pause state for a specific device. Returns new state.
func (p *PauseState) Toggle(deviceID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused[deviceID] = !p.paused[deviceID]
	return p.paused[deviceID]
}

// ToggleAll toggles all devices. If any are unpaused, pauses all; otherwise resumes all.
// Returns true if all are now paused.
func (p *PauseState) ToggleAll() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	allPaused := p.isAllPausedLocked()
	for id := range p.paused {
		p.paused[id] = !allPaused
	}
	return !allPaused
}

// IsPaused returns whether a specific device is paused.
func (p *PauseState) IsPaused(deviceID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.paused[deviceID]
}

// IsAllPaused returns true if all devices are paused.
func (p *PauseState) IsAllPaused() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isAllPausedLocked()
}

func (p *PauseState) isAllPausedLocked() bool {
	if len(p.paused) == 0 {
		return false
	}
	for _, v := range p.paused {
		if !v {
			return false
		}
	}
	return true
}

// PausedDevices returns the IDs of all currently paused devices.
func (p *PauseState) PausedDevices() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []string
	for _, id := range p.devices {
		if p.paused[id] {
			result = append(result, id)
		}
	}
	return result
}

// Devices returns all tracked device IDs in order.
func (p *PauseState) Devices() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]string(nil), p.devices...)
}

// Count returns the total number of tracked devices.
func (p *PauseState) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.devices)
}

// SetPaused sets the pause state for a specific device.
func (p *PauseState) SetPaused(deviceID string, paused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused[deviceID] = paused
}
