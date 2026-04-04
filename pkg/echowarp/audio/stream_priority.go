package audio

import (
	"math"
	"sync"
)

// StreamPriority represents the priority level for a participant's stream.
type StreamPriority int

const (
	// PriorityPrimary is the active speaker — full bitrate, no degradation.
	PriorityPrimary StreamPriority = 1
	// PrioritySecondary is a non-primary speaker — can degrade to 50% bitrate.
	PrioritySecondary StreamPriority = 2
	// PrioritySilent is a silent participant — use Opus DTX (near-zero traffic).
	PrioritySilent StreamPriority = 3
)

// silenceThresholdDB is the RMS threshold below which a participant is silent (-40 dB).
const silenceThresholdDB = -40.0

// StreamPrioritizer computes per-participant stream priorities based on RMS levels.
// Updated every ~100ms (every 5th audio frame at 20ms/frame).
type StreamPrioritizer struct {
	mu         sync.RWMutex
	priorities map[string]StreamPriority
	rmsLevels  map[string]float64
	primaryID  string
}

// NewStreamPrioritizer creates a new prioritizer.
func NewStreamPrioritizer() *StreamPrioritizer {
	return &StreamPrioritizer{
		priorities: make(map[string]StreamPriority),
		rmsLevels:  make(map[string]float64),
	}
}

// UpdateRMS updates the RMS level for a participant and recalculates priorities.
func (sp *StreamPrioritizer) UpdateRMS(participantID string, samples []float32) {
	rms := computeRMS(samples)

	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.rmsLevels[participantID] = rms
	sp.recalculate()
}

// RemoveParticipant removes a participant from tracking.
func (sp *StreamPrioritizer) RemoveParticipant(participantID string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	delete(sp.rmsLevels, participantID)
	delete(sp.priorities, participantID)
	if sp.primaryID == participantID {
		sp.primaryID = ""
	}
	sp.recalculate()
}

// GetPriority returns the current priority for a participant.
func (sp *StreamPrioritizer) GetPriority(participantID string) StreamPriority {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	if p, ok := sp.priorities[participantID]; ok {
		return p
	}
	return PrioritySilent
}

// GetPrimaryID returns the current primary speaker ID (empty if none).
func (sp *StreamPrioritizer) GetPrimaryID() string {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return sp.primaryID
}

// GetAllPriorities returns a snapshot of all priorities.
func (sp *StreamPrioritizer) GetAllPriorities() map[string]StreamPriority {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	result := make(map[string]StreamPriority, len(sp.priorities))
	for k, v := range sp.priorities {
		result[k] = v
	}
	return result
}

// recalculate determines priorities. Must be called with mu held.
func (sp *StreamPrioritizer) recalculate() {
	silenceThreshold := math.Pow(10, silenceThresholdDB/20.0) // ~0.01

	// Find the participant with the highest RMS
	maxRMS := 0.0
	maxID := ""
	for id, rms := range sp.rmsLevels {
		if rms > maxRMS {
			maxRMS = rms
			maxID = id
		}
	}

	for id, rms := range sp.rmsLevels {
		if id == maxID && rms > silenceThreshold {
			sp.priorities[id] = PriorityPrimary
		} else if rms > silenceThreshold {
			sp.priorities[id] = PrioritySecondary
		} else {
			sp.priorities[id] = PrioritySilent
		}
	}

	if maxRMS > silenceThreshold {
		sp.primaryID = maxID
	} else {
		sp.primaryID = ""
	}
}

func computeRMS(samples []float32) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

// BitrateForPriority returns the recommended Opus bitrate for the given priority.
func BitrateForPriority(baseBitrate int, priority StreamPriority) int {
	switch priority {
	case PriorityPrimary:
		return baseBitrate
	case PrioritySecondary:
		return baseBitrate / 2
	case PrioritySilent:
		return 6000 // Opus minimum, DTX will handle
	default:
		return baseBitrate
	}
}
