package echowarp

// ParticipantInfo is a public, transport-agnostic snapshot of a single
// conference participant, returned by Node.Participants(). It intentionally
// exposes only the fields that an API / integration caller needs — internal
// fields like AGC processors or raw sample buffers are not surfaced.
//
// The struct is a value type (no pointers) so callers can freely copy / pass
// slices of ParticipantInfo without worrying about concurrent mutation of
// mixer state.
type ParticipantInfo struct {
	// ID is the unique participant identifier assigned by the server.
	ID string `json:"id"`
	// Nickname is the human-readable display name. May be empty for
	// server-assigned anonymous clients.
	Nickname string `json:"nickname,omitempty"`
	// Muted reflects the current mute state in the conference mix.
	Muted bool `json:"muted"`
	// Volume is the current volume multiplier. Range matches the mixer's
	// capability (typically 0.0–1.5 for API-set values, but the mixer may
	// carry any non-negative float for backwards compatibility with TUI).
	Volume float64 `json:"volume"`
}

// ParticipantAction enumerates the supported conference participant control
// actions that can be dispatched through Runner.HandleParticipantCommand.
//
// The values mirror internal/app.ParticipantAction conceptually but are kept
// in pkg/echowarp to keep the public API free of internal imports. Adapters
// in internal/app perform the translation — see internal/app/participant_handler.go.
type ParticipantAction int

const (
	// ParticipantActionMute sets the mute state of a participant in the
	// conference mix to an absolute value (not a toggle).
	ParticipantActionMute ParticipantAction = iota
	// ParticipantActionKick disconnects a participant from the conference.
	ParticipantActionKick
	// ParticipantActionSetVolume sets the volume multiplier of a
	// participant to an absolute value in the range 0.0–1.5.
	ParticipantActionSetVolume
)

// ParticipantCommand is a public, transport-agnostic command for controlling
// a single conference participant managed by a running Runner. It is the
// payload passed to ParticipantCommandReceiver.HandleParticipantCommand.
type ParticipantCommand struct {
	// Action specifies which operation to perform.
	Action ParticipantAction
	// ID is the target participant id (must be non-empty).
	ID string
	// Muted is consumed when Action == ParticipantActionMute.
	Muted bool
	// Volume is consumed when Action == ParticipantActionSetVolume. Valid
	// range 0.0–1.5 (matching Node.SetParticipantVolume).
	Volume float64
}
