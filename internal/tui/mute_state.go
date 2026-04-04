package tui

import "sync"

// MuteState tracks per-participant and global mute state for the conference TUI.
// All methods are safe for concurrent use.
type MuteState struct {
	mu              sync.RWMutex
	muted           map[string]bool // per-participant mute state
	muteAll         bool
	preMuteAllState map[string]bool // snapshot of muted map before mute-all was activated
}

// NewMuteState creates a new empty MuteState.
func NewMuteState() *MuteState {
	return &MuteState{
		muted: make(map[string]bool),
	}
}

// ToggleMute toggles the mute state for a single participant.
// Returns the new muted state (true = now muted).
// If mute-all is active and the participant is being unmuted, mute-all is cleared.
func (ms *MuteState) ToggleMute(participantID string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	wasMuted := ms.muted[participantID]
	newMuted := !wasMuted
	ms.muted[participantID] = newMuted

	// If mute-all is active and we just unmuted someone, clear the mute-all flag.
	if ms.muteAll && !newMuted {
		ms.muteAll = false
		ms.preMuteAllState = nil
	}

	// Clean up false entries.
	if !newMuted {
		delete(ms.muted, participantID)
	}

	return newMuted
}

// ToggleMuteAll toggles mute-all mode. When enabling, it saves the current per-participant
// mute state and mutes everyone. When disabling, it restores the saved state.
// participantIDs is the list of all current participant IDs (excluding self).
// Returns the new mute-all state.
func (ms *MuteState) ToggleMuteAll(participantIDs []string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.muteAll {
		// Enable mute-all: save current state, mute everyone.
		ms.preMuteAllState = make(map[string]bool, len(ms.muted))
		for k, v := range ms.muted {
			ms.preMuteAllState[k] = v
		}
		ms.muteAll = true
		for _, id := range participantIDs {
			ms.muted[id] = true
		}
	} else {
		// Disable mute-all: restore pre-mute-all state.
		ms.muteAll = false
		ms.muted = make(map[string]bool)
		if ms.preMuteAllState != nil {
			for k, v := range ms.preMuteAllState {
				ms.muted[k] = v
			}
		}
		ms.preMuteAllState = nil
	}

	return ms.muteAll
}

// IsMuted returns whether a participant is currently muted.
func (ms *MuteState) IsMuted(participantID string) bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.muted[participantID]
}

// IsMuteAll returns whether mute-all mode is active.
func (ms *MuteState) IsMuteAll() bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.muteAll
}

// MutedParticipants returns a list of all currently muted participant IDs.
func (ms *MuteState) MutedParticipants() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make([]string, 0, len(ms.muted))
	for id, muted := range ms.muted {
		if muted {
			result = append(result, id)
		}
	}
	return result
}

// MutedMap returns a copy of the per-participant mute map.
func (ms *MuteState) MutedMap() map[string]bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make(map[string]bool, len(ms.muted))
	for k, v := range ms.muted {
		result[k] = v
	}
	return result
}

// RemoveParticipant removes a participant from all mute tracking.
func (ms *MuteState) RemoveParticipant(participantID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.muted, participantID)
	delete(ms.preMuteAllState, participantID)
}
