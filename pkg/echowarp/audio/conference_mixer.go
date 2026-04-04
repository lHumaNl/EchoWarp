package audio

import (
	"context"
	"math"
	"sync"
)

// speakingRMSThreshold is the RMS level above which a participant is considered speaking.
const speakingRMSThreshold = 0.01

// ParticipantState holds per-participant state in a conference mix.
type ParticipantState struct {
	ID       string
	Volume   float32
	Muted    bool
	Speaking bool
	RMSLevel float32 // 0.0–1.0, current audio level for VU meter
	agc      *AGCProcessor
}

// ConferenceMixer implements the O(N) personal-mix algorithm:
// compute total_mix once, then subtract each participant's own contribution.
type ConferenceMixer struct {
	mu         sync.RWMutex
	frameSize  int
	sampleRate uint32

	participants map[string]*participantData

	bufPool sync.Pool
}

// participantData is internal per-participant storage.
type participantData struct {
	mu      sync.Mutex // guards samples, raw, and mutable fields of state
	state   ParticipantState
	samples []float32 // latest submitted frame (already volume-scaled)
	raw     []float32 // latest submitted frame (before volume, for subtraction)
}

// NewConferenceMixer creates a mixer for the given frame size and sample rate.
func NewConferenceMixer(frameSize int, sampleRate uint32) *ConferenceMixer {
	return &ConferenceMixer{
		frameSize:    frameSize,
		sampleRate:   sampleRate,
		participants: make(map[string]*participantData),
		bufPool: sync.Pool{
			New: func() any {
				buf := make([]float32, frameSize)
				return &buf
			},
		},
	}
}

func (cm *ConferenceMixer) getBuf() []float32 {
	bp := cm.bufPool.Get().(*[]float32) //nolint:errcheck
	buf := *bp
	if cap(buf) < cm.frameSize {
		buf = make([]float32, cm.frameSize)
	} else {
		buf = buf[:cm.frameSize]
	}
	for i := range buf {
		buf[i] = 0
	}
	return buf
}

func (cm *ConferenceMixer) putBuf(buf []float32) {
	cm.bufPool.Put(&buf)
}

// AddParticipant registers a participant with the given ID.
func (cm *ConferenceMixer) AddParticipant(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.participants[id]; exists {
		return
	}

	cm.participants[id] = &participantData{
		state: ParticipantState{
			ID:     id,
			Volume: 1.0,
			Muted:  false,
			agc:    NewAGCProcessor(DefaultAGCConfig()),
		},
		samples: make([]float32, cm.frameSize),
		raw:     make([]float32, cm.frameSize),
	}
}

// SetParticipantAGC enables or disables AGC for a participant.
func (cm *ConferenceMixer) SetParticipantAGC(id string, enabled bool) {
	cm.mu.RLock()
	pd, ok := cm.participants[id]
	cm.mu.RUnlock()
	if ok {
		pd.mu.Lock()
		if pd.state.agc != nil {
			pd.state.agc.SetEnabled(enabled)
		}
		pd.mu.Unlock()
	}
}

// RemoveParticipant removes a participant by ID.
func (cm *ConferenceMixer) RemoveParticipant(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.participants, id)
}

// SubmitAudio stores the latest audio frame for a participant.
// The samples slice is copied internally.

// HasParticipant returns whether a participant with the given ID exists.
func (cm *ConferenceMixer) HasParticipant(id string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.participants[id]
	return ok
}

func (cm *ConferenceMixer) SubmitAudio(participantID string, samples []float32) {
	// Look up participant under RLock — does not block other SubmitAudio calls.
	cm.mu.RLock()
	pd, ok := cm.participants[participantID]
	cm.mu.RUnlock()
	if !ok {
		return
	}

	// Write to the participant's own buffer under per-participant lock.
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Detect speaking and compute RMS level from raw input
	rms := computeRMS(samples)
	pd.state.Speaking = rms > speakingRMSThreshold
	pd.state.RMSLevel = float32(math.Min(rms*5, 1.0)) // scale to 0.0–1.0

	// Apply per-participant AGC before storing.
	if pd.state.agc != nil && pd.state.agc.IsEnabled() {
		processed, err := pd.state.agc.Process(context.Background(), samples)
		if err == nil {
			samples = processed
		}
	}

	// Store raw copy (volume=1, not muted) for subtraction later
	n := copy(pd.raw, samples)
	for i := n; i < cm.frameSize; i++ {
		pd.raw[i] = 0
	}

	// Store volume-scaled copy for the total mix
	copy(pd.samples, pd.raw)
	if pd.state.Muted {
		for i := range pd.samples {
			pd.samples[i] = 0
		}
	} else if pd.state.Volume != 1.0 {
		MixGain(pd.samples, pd.state.Volume)
	}
}

// ReturnBuffer returns a buffer obtained from GetPersonalMix back to the pool.
// Callers MUST call this after consuming the personal mix to avoid memory leaks.
func (cm *ConferenceMixer) ReturnBuffer(buf []float32) {
	if buf != nil {
		cm.putBuf(buf)
	}
}

// GetPersonalMix returns the mixed audio for participantID:
// total_mix minus their own contribution. Returns nil if participant unknown.
// The caller MUST call ReturnBuffer() when done with the returned slice.
func (cm *ConferenceMixer) GetPersonalMix(participantID string) []float32 {
	cm.mu.RLock()
	self, ok := cm.participants[participantID]
	if !ok {
		cm.mu.RUnlock()
		return nil
	}

	// Collect a stable snapshot of participant pointers under global RLock.
	pds := make([]*participantData, 0, len(cm.participants))
	for _, pd := range cm.participants {
		pds = append(pds, pd)
	}
	cm.mu.RUnlock()

	// Build total mix — lock each participant individually to read their samples.
	total := cm.getBuf()
	selfSamples := make([]float32, cm.frameSize)

	for _, pd := range pds {
		pd.mu.Lock()
		MixAccumulate(total, pd.samples)
		if pd == self {
			copy(selfSamples, pd.samples)
		}
		pd.mu.Unlock()
	}

	// Subtract self contribution: personal = total - self.samples
	for i := range total {
		total[i] -= selfSamples[i]
	}

	// Soft clipping to prevent digital distortion in Opus encoder
	MixTanh(total)

	return total
}

// GetTotalMix returns the mixed audio of all participants (no exclusion).
// The caller MUST call ReturnBuffer() when done with the returned slice.
// Returns nil if no participants have submitted audio.
func (cm *ConferenceMixer) GetTotalMix() []float32 {
	cm.mu.RLock()
	if len(cm.participants) == 0 {
		cm.mu.RUnlock()
		return nil
	}
	pds := make([]*participantData, 0, len(cm.participants))
	for _, pd := range cm.participants {
		pds = append(pds, pd)
	}
	cm.mu.RUnlock()

	total := cm.getBuf()
	hasAudio := false
	for _, pd := range pds {
		pd.mu.Lock()
		if pd.samples != nil {
			MixAccumulate(total, pd.samples)
			hasAudio = true
		}
		pd.mu.Unlock()
	}

	if !hasAudio {
		cm.putBuf(total)
		return nil
	}
	return total
}

// SetParticipantVolume sets the volume multiplier for a participant.
func (cm *ConferenceMixer) SetParticipantVolume(id string, vol float32) {
	cm.mu.RLock()
	pd, ok := cm.participants[id]
	cm.mu.RUnlock()
	if ok {
		pd.mu.Lock()
		pd.state.Volume = vol
		pd.mu.Unlock()
	}
}

// SetParticipantMuted sets the mute state for a participant.
func (cm *ConferenceMixer) SetParticipantMuted(id string, muted bool) {
	cm.mu.RLock()
	pd, ok := cm.participants[id]
	cm.mu.RUnlock()
	if ok {
		pd.mu.Lock()
		pd.state.Muted = muted
		pd.mu.Unlock()
	}
}

// GetParticipantStates returns a snapshot of all participant states.
func (cm *ConferenceMixer) GetParticipantStates() []ParticipantState {
	cm.mu.RLock()
	pds := make([]*participantData, 0, len(cm.participants))
	for _, pd := range cm.participants {
		pds = append(pds, pd)
	}
	cm.mu.RUnlock()

	states := make([]ParticipantState, 0, len(pds))
	for _, pd := range pds {
		pd.mu.Lock()
		states = append(states, pd.state)
		pd.mu.Unlock()
	}
	return states
}
