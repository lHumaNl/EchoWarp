package app

import (
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// participantCommandEnqueueTimeout bounds how long HandleParticipantCommand
// will block while trying to push a command onto the internal channel. Must
// be short enough that an HTTP API caller does not perceive a stall when the
// consumer goroutine is slow or not yet started.
const participantCommandEnqueueTimeout = 100 * time.Millisecond

// translateParticipantCommand converts a public echowarp.ParticipantCommand
// into the internal ParticipantCommand format used by conference.go.
//
// The internal ParticipantCommand was originally TUI-shaped (toggles like
// Mute/Unmute/VolumeUp/VolumeDown). For absolute-value commands originating
// from the API we introduce dedicated actions (ParticipantSetMute /
// ParticipantSetVolume / ParticipantKick — see commands.go) and forward the
// absolute Muted / Volume values on new fields.
//
// TODO(task-013): extend internal ConferenceHandler.handleCommand to honor
// the new ParticipantSet* / ParticipantKick actions. Until then, API calls
// reach the channel exposed via ParticipantCommandChannel() but the current
// consumer only handles TUI toggle actions. See
// .tasks/013-volume-mute-controls.md.
func translateParticipantCommand(cmd echowarp.ParticipantCommand) ParticipantCommand {
	internal := ParticipantCommand{
		ParticipantID: cmd.ID,
		Muted:         cmd.Muted,
		Volume:        cmd.Volume,
	}
	switch cmd.Action {
	case echowarp.ParticipantActionMute:
		internal.Action = ParticipantSetMute
	case echowarp.ParticipantActionKick:
		internal.Action = ParticipantKick
	case echowarp.ParticipantActionSetVolume:
		internal.Action = ParticipantSetVolume
	}
	return internal
}

// HandleParticipantCommand implements echowarp.ParticipantCommandReceiver. It
// enqueues the command onto the ServerApp's internal participant command
// channel with a short timeout to avoid blocking the caller (typically an
// HTTP handler).
//
// Returns an ErrNotRunning error if the channel is not initialized (zero-
// valued ServerApp constructed outside NewServerApp) and an ErrBufferOverflow
// error if the channel remains full longer than participantCommandEnqueueTimeout.
func (s *ServerApp) HandleParticipantCommand(cmd echowarp.ParticipantCommand) error {
	if s.participantCmdChAPI == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Server participant command channel not initialized").
			WithSuggestion("Construct ServerApp via NewServerApp")
	}
	internal := translateParticipantCommand(cmd)
	timer := time.NewTimer(participantCommandEnqueueTimeout)
	defer timer.Stop()
	select {
	case s.participantCmdChAPI <- internal:
		return nil
	case <-timer.C:
		return ewerrors.NewError(ewerrors.ErrBufferOverflow, "Participant command queue full").
			WithContext("participant_id", cmd.ID).
			WithSuggestion("Reduce command rate or check that the consumer goroutine is running")
	}
}

// Participants implements echowarp.ParticipantLister. It returns a snapshot
// of participants from the active conference handler, mapped to the public
// echowarp.ParticipantInfo type.
//
// Returns an empty (non-nil) slice if conference mode is not active or the
// conference handler has not yet been initialized.
func (s *ServerApp) Participants() []echowarp.ParticipantInfo {
	if s == nil {
		return []echowarp.ParticipantInfo{}
	}
	ch := s.conference
	if ch == nil {
		return []echowarp.ParticipantInfo{}
	}
	states := ch.GetParticipantStates()
	result := make([]echowarp.ParticipantInfo, 0, len(states))
	for _, st := range states {
		result = append(result, echowarp.ParticipantInfo{
			ID:     st.ID,
			Muted:  st.Muted,
			Volume: float64(st.Volume),
			// Nickname is not carried on audio.ParticipantState — the
			// mapping from participant id to nickname lives on the
			// multiClient record. TODO(task-013): plumb nickname into
			// the public ParticipantInfo by cross-referencing s.clients.
		})
	}
	return result
}

// HandleParticipantCommand implements echowarp.ParticipantCommandReceiver for
// ClientApp. In client mode participants aren't managed by the local node —
// conference participant control is a server-only operation — so this
// implementation returns ErrNotRunning to signal "not supported in this
// mode" without requiring the caller to know the mode.
//
// We still carry a no-op channel so ParticipantCommandChannel() returns a
// non-nil (read-only, always empty) channel, matching the phase 4a
// ClientApp.DeviceCommandChannel shape.
func (c *ClientApp) HandleParticipantCommand(cmd echowarp.ParticipantCommand) error {
	if c.participantCmdChAPI == nil {
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Client participant command channel not initialized").
			WithSuggestion("Construct ClientApp via NewClientApp")
	}
	internal := translateParticipantCommand(cmd)
	timer := time.NewTimer(participantCommandEnqueueTimeout)
	defer timer.Stop()
	select {
	case c.participantCmdChAPI <- internal:
		return nil
	case <-timer.C:
		return ewerrors.NewError(ewerrors.ErrBufferOverflow, "Participant command queue full").
			WithContext("participant_id", cmd.ID).
			WithSuggestion("Reduce command rate or check that the consumer goroutine is running")
	}
}

// Participants implements echowarp.ParticipantLister for ClientApp. In
// client mode the local node does not own the conference mixer, so this
// method always returns an empty slice. Conference participant lists are
// delivered to the client via the participants_update signaling message
// (see ConferenceParticipantsMsg) and consumed by the TUI — they are not
// cached in a form suitable for API exposure yet.
//
// TODO(task-013): cache the latest ConferenceParticipantsMsg snapshot on
// ClientApp so it can be exposed here.
func (c *ClientApp) Participants() []echowarp.ParticipantInfo {
	return []echowarp.ParticipantInfo{}
}
