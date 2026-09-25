package app

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// ConferenceHandler manages the conference mix engine and participant audio routing.
// It sits between the server's WebRTC peers and the ConferenceMixer.
type ConferenceHandler struct {
	mixer       *audio.ConferenceMixer
	prioritizer *audio.StreamPrioritizer
	logger      *slog.Logger

	mu          sync.RWMutex
	serverMuted bool
	frameCount  int // counts frames for periodic priority updates

	// persistentMutes tracks client IDs that should be auto-muted on (re)connect.
	persistentMutes map[string]bool

	// pausedParticipants tracks which participants have paused their capture.
	pausedParticipants map[string]bool

	// participantCmdCh receives mute/volume commands from TUI.
	participantCmdCh <-chan ParticipantCommand

	// AttachRoom is initialization-only; room and channels then remain immutable.
	room             *ConferenceRoom
	channels         uint32
	recordingMu      sync.Mutex
	recorder         *audio.ConferenceRecorder
	recording        atomic.Pointer[conferenceRecording]
	recordingRunning atomic.Bool
	sourceRoster     atomic.Pointer[conferenceRecordingSources]
}

// NewConferenceHandler creates a handler with the given frame size and sample rate.
func NewConferenceHandler(frameSize int, sampleRate uint32, serverMuted bool, logger *slog.Logger) *ConferenceHandler {
	return &ConferenceHandler{
		mixer:              audio.NewConferenceMixer(frameSize, sampleRate),
		prioritizer:        audio.NewStreamPrioritizer(),
		logger:             logger,
		serverMuted:        serverMuted,
		persistentMutes:    make(map[string]bool),
		pausedParticipants: make(map[string]bool),
	}
}

// WithParticipantCommands sets the channel for receiving participant control commands.
func (ch *ConferenceHandler) WithParticipantCommands(cmdCh <-chan ParticipantCommand) *ConferenceHandler {
	ch.participantCmdCh = cmdCh
	return ch
}

// AddParticipant registers a participant (client or server) in the mix.
// If the participant was previously muted persistently, the mute is re-applied.
func (ch *ConferenceHandler) AddParticipant(id string) {
	ch.mixer.AddParticipant(id)
	ch.mu.RLock()
	muted := ch.persistentMutes[id]
	ch.mu.RUnlock()
	if muted {
		ch.SetParticipantMuted(id, true)
		ch.logger.Info("Conference participant added (auto-muted)", "id", id)
	} else {
		ch.logger.Info("Conference participant added", "id", id)
	}
}

// RemoveParticipant removes a participant from the mix.
func (ch *ConferenceHandler) RemoveParticipant(id string) {
	ch.mixer.RemoveParticipant(id)
	ch.prioritizer.RemoveParticipant(id)
	ch.mu.Lock()
	delete(ch.pausedParticipants, id)
	ch.mu.Unlock()
	ch.logger.Info("Conference participant removed", "id", id)
}

// SetParticipantPaused sets the paused state of a participant.
func (ch *ConferenceHandler) SetParticipantPaused(id string, paused bool) {
	ch.mu.Lock()
	if paused {
		ch.pausedParticipants[id] = true
	} else {
		delete(ch.pausedParticipants, id)
	}
	ch.mu.Unlock()
	if ch.room != nil {
		ch.room.SetSourcePaused(id, paused)
	}
}

// GetPausedParticipants returns a snapshot of the paused participants map.
func (ch *ConferenceHandler) GetPausedParticipants() map[string]bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	result := make(map[string]bool, len(ch.pausedParticipants))
	for k, v := range ch.pausedParticipants {
		result[k] = v
	}
	return result
}

// ParticipantIDs returns a list of all current participant IDs.
func (ch *ConferenceHandler) ParticipantIDs() []string {
	states := ch.mixer.GetParticipantStates()
	ids := make([]string, len(states))
	for i, s := range states {
		ids[i] = s.ID
	}
	return ids
}

// SubmitAudio submits a decoded audio frame from a participant.
func (ch *ConferenceHandler) SubmitAudio(participantID string, samples []float32) {
	// An attached handler is metadata-only; recording decodes the opaque RTP tap.
	if ch.room != nil {
		return
	}
	ch.mixer.SubmitAudio(participantID, samples)

	// Write to recording if active and participant is still known (avoid orphaned tracks).
	if ch.mixer.HasParticipant(participantID) {
		ch.recordingMu.Lock()
		rec := ch.recorder
		if rec != nil && rec.IsActive() {
			_ = rec.WriteTrack(participantID, samples) //nolint:errcheck
		}
		ch.recordingMu.Unlock()
	}

	// Update stream priorities every 5 frames (~100ms at 20ms/frame).
	ch.mu.Lock()
	ch.frameCount++
	shouldUpdate := ch.frameCount%5 == 0
	ch.mu.Unlock()
	if shouldUpdate {
		ch.prioritizer.UpdateRMS(participantID, samples)
	}
}

// GetPersonalMix returns the mixed audio for a participant (everyone except themselves).
// The caller MUST call ReturnMixBuffer() when done with the returned slice.
func (ch *ConferenceHandler) GetPersonalMix(participantID string) []float32 {
	return ch.mixer.GetPersonalMix(participantID)
}

// GetTotalMix returns the mixed audio of all participants.
// The caller MUST call ReturnMixBuffer() when done.
func (ch *ConferenceHandler) GetTotalMix() []float32 {
	return ch.mixer.GetTotalMix()
}

// WriteMix writes the total mix frame to the recorder (if active and mode includes mix).
func (ch *ConferenceHandler) WriteMix(samples []float32) error {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	rec := ch.recorder
	if ch.room == nil && rec != nil && rec.IsActive() {
		return rec.WriteMix(samples)
	}
	return nil
}

// ReturnMixBuffer returns a buffer from GetPersonalMix/GetTotalMix back to the pool.
func (ch *ConferenceHandler) ReturnMixBuffer(buf []float32) {
	ch.mixer.ReturnBuffer(buf)
}

// IsServerMuted returns whether the server participant is muted.
func (ch *ConferenceHandler) IsServerMuted() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.serverMuted
}

// GetParticipantStates returns a snapshot of all participant states.
func (ch *ConferenceHandler) GetParticipantStates() []audio.ParticipantState {
	return ch.mixer.GetParticipantStates()
}

// GetStreamPriority returns the stream priority for a participant.
func (ch *ConferenceHandler) GetStreamPriority(participantID string) audio.StreamPriority {
	return ch.prioritizer.GetPriority(participantID)
}

// GetRecommendedBitrate returns the recommended Opus bitrate for a participant
// based on their current stream priority.
func (ch *ConferenceHandler) GetRecommendedBitrate(participantID string, baseBitrate int) int {
	priority := ch.prioritizer.GetPriority(participantID)
	return audio.BitrateForPriority(baseBitrate, priority)
}

// ProcessCommands handles participant control commands in a loop.
func (ch *ConferenceHandler) ProcessCommands(ctx context.Context) {
	if ch.participantCmdCh == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-ch.participantCmdCh:
			if !ok {
				return
			}
			ch.handleCommand(cmd)
		}
	}
}

func (ch *ConferenceHandler) handleCommand(cmd ParticipantCommand) {
	switch cmd.Action {
	case ParticipantMute:
		ch.SetParticipantMuted(cmd.ParticipantID, true)
	case ParticipantUnmute:
		ch.SetParticipantMuted(cmd.ParticipantID, false)
	case ParticipantSetMute:
		ch.SetParticipantMuted(cmd.ParticipantID, cmd.Muted)
	case ParticipantMutePersist, ParticipantUnmutePersist:
		ch.setPersistentMute(cmd.ParticipantID, cmd.Action == ParticipantMutePersist)
	case ParticipantSetVolume:
		ch.SetParticipantGain(cmd.ParticipantID, float32(cmd.Volume))
	case ParticipantVolumeUp, ParticipantVolumeDown:
		ch.adjustParticipantGain(cmd.ParticipantID, cmd.Action == ParticipantVolumeUp)
	}
}

const conferenceVolumeStep, conferenceMaxVolume float32 = 0.1, 2

// SetParticipantMuted changes the source gate without modifying route rules.
func (ch *ConferenceHandler) SetParticipantMuted(id string, muted bool) {
	ch.mixer.SetParticipantMuted(id, muted)
	if ch.room != nil {
		ch.room.SetSourceBlocked(id, muted)
	}
}

// SetParticipantGain updates both the legacy snapshot and forwarded source gain.
func (ch *ConferenceHandler) SetParticipantGain(id string, gain float32) {
	if gain < 0 || math.IsNaN(float64(gain)) || math.IsInf(float64(gain), 0) {
		return
	}
	gain = min(gain, conferenceMaxVolume)
	ch.mixer.SetParticipantVolume(id, gain)
	if ch.room != nil {
		ch.room.SetSourceGain(id, gain)
	}
}

func (ch *ConferenceHandler) setPersistentMute(id string, muted bool) {
	ch.mu.Lock()
	if muted {
		ch.persistentMutes[id] = true
	} else {
		delete(ch.persistentMutes, id)
	}
	ch.mu.Unlock()
	ch.SetParticipantMuted(id, muted)
}

func (ch *ConferenceHandler) adjustParticipantGain(id string, increase bool) {
	delta := -conferenceVolumeStep
	if increase {
		delta = conferenceVolumeStep
	}
	for _, state := range ch.mixer.GetParticipantStates() {
		if state.ID == id {
			ch.SetParticipantGain(id, max(0, state.Volume+delta))
			return
		}
	}
}
