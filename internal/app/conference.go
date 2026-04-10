package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	// Recording support.
	recorder *audio.ConferenceRecorder
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
		ch.mixer.SetParticipantMuted(id, true)
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
	defer ch.mu.Unlock()
	if paused {
		ch.pausedParticipants[id] = true
	} else {
		delete(ch.pausedParticipants, id)
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

// StartRecording begins conference recording in the given mode.
func (ch *ConferenceHandler) StartRecording(mode audio.RecordingMode, sampleRate uint32) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.recorder = audio.NewConferenceRecorder(mode, sampleRate, 1) // mono
	homeDir, _ := os.UserHomeDir()                                 //nolint:errcheck
	baseDir := filepath.Join(homeDir, "Documents", "EchoWarp_records")
	return ch.recorder.Start(baseDir)
}

// StopRecording stops recording and returns summary info.
func (ch *ConferenceHandler) StopRecording() (duration time.Duration, totalSize uint64, fileCount int, err error) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.recorder == nil {
		return 0, 0, 0, nil
	}
	dur, size, files, err := ch.recorder.Stop()
	ch.recorder = nil
	return dur, size, files, err
}

// RecordingDir returns the output directory of the active recording,
// or empty string when nothing is being recorded. Used by the daemon
// API adapter to enumerate the produced files after Stop.
func (ch *ConferenceHandler) RecordingDir() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	if ch.recorder == nil {
		return ""
	}
	return ch.recorder.Dir()
}

// IsRecording returns whether recording is active.
func (ch *ConferenceHandler) IsRecording() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.recorder != nil && ch.recorder.IsActive()
}

// FlushHeaders flushes WAV headers on all active recording writers for crash safety.
func (ch *ConferenceHandler) FlushHeaders() {
	ch.mu.RLock()
	rec := ch.recorder
	ch.mu.RUnlock()
	if rec != nil && rec.IsActive() {
		rec.FlushHeaders()
	}
}

// SubmitAudio submits a decoded audio frame from a participant.
func (ch *ConferenceHandler) SubmitAudio(participantID string, samples []float32) {
	ch.mixer.SubmitAudio(participantID, samples)

	// Write to recording if active and participant is still known (avoid orphaned tracks).
	if ch.mixer.HasParticipant(participantID) {
		ch.mu.RLock()
		rec := ch.recorder
		ch.mu.RUnlock()
		if rec != nil && rec.IsActive() {
			_ = rec.WriteTrack(participantID, samples) //nolint:errcheck
		}
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
	ch.mu.RLock()
	rec := ch.recorder
	ch.mu.RUnlock()
	if rec != nil && rec.IsActive() {
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
		ch.mixer.SetParticipantMuted(cmd.ParticipantID, true)
		ch.logger.Info("Participant muted", "id", cmd.ParticipantID)
	case ParticipantUnmute:
		ch.mixer.SetParticipantMuted(cmd.ParticipantID, false)
		ch.logger.Info("Participant unmuted", "id", cmd.ParticipantID)
	case ParticipantMutePersist:
		ch.mixer.SetParticipantMuted(cmd.ParticipantID, true)
		ch.mu.Lock()
		ch.persistentMutes[cmd.ParticipantID] = true
		ch.mu.Unlock()
		ch.logger.Info("Participant muted (persistent)", "id", cmd.ParticipantID)
	case ParticipantUnmutePersist:
		ch.mixer.SetParticipantMuted(cmd.ParticipantID, false)
		ch.mu.Lock()
		delete(ch.persistentMutes, cmd.ParticipantID)
		ch.mu.Unlock()
		ch.logger.Info("Participant unmuted (persistent removed)", "id", cmd.ParticipantID)
	case ParticipantVolumeUp:
		states := ch.mixer.GetParticipantStates()
		for _, s := range states {
			if s.ID == cmd.ParticipantID {
				newVol := s.Volume + 0.1
				if newVol > 2.0 {
					newVol = 2.0
				}
				ch.mixer.SetParticipantVolume(cmd.ParticipantID, newVol)
				break
			}
		}
	case ParticipantVolumeDown:
		states := ch.mixer.GetParticipantStates()
		for _, s := range states {
			if s.ID == cmd.ParticipantID {
				newVol := s.Volume - 0.1
				if newVol < 0 {
					newVol = 0
				}
				ch.mixer.SetParticipantVolume(cmd.ParticipantID, newVol)
				break
			}
		}
	}
}
