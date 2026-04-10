package app

import "github.com/lHumaNl/echowarp/pkg/echowarp/audio"

// ClientAction represents an action to perform on a client.
type ClientAction int

const (
	// ActionKick disconnects a client (they can reconnect).
	ActionKick ClientAction = iota
	// ActionBan disconnects and bans a client by IP.
	ActionBan
	// ActionUnban removes an IP from the ban list.
	ActionUnban
	// ActionMuteOutgoing toggles server-initiated outgoing mute for a client.
	ActionMuteOutgoing
	// ActionMuteIncoming toggles server-initiated incoming mute for a client.
	ActionMuteIncoming
	// ActionVolumeUp increases per-client volume by 10%.
	ActionVolumeUp
	// ActionVolumeDown decreases per-client volume by 10%.
	ActionVolumeDown
)

// ClientCommand is a command sent from the TUI to the server backend.
type ClientCommand struct {
	Action      ClientAction
	ClientID    string   // for kick/ban
	IP          string   // for unban
	Reason      string   // optional kick/ban reason
	BanCriteria []string // ban criteria: "ip", "nickname", "hwid"
}

// DeviceAction represents an action on a capture/playback device.
type DeviceAction int

const (
	// DeviceToggleMute toggles mute for a single device.
	DeviceToggleMute DeviceAction = iota
	// DeviceGlobalMute toggles global mute (all devices).
	DeviceGlobalMute
	// DeviceVolumeUp increases device volume by 10%.
	DeviceVolumeUp
	// DeviceVolumeDown decreases device volume by 10%.
	DeviceVolumeDown
	// DeviceToggleAGC toggles automatic gain control for a device.
	DeviceToggleAGC
)

// DeviceCommand is sent from the TUI to control device volume/mute on the mixer.
type DeviceCommand struct {
	Action   DeviceAction
	DeviceID uint32 // target device (ignored for GlobalMute)
}

// ParticipantAction represents an action on a conference participant.
type ParticipantAction int

const (
	// ParticipantMute mutes a participant in the conference mix.
	ParticipantMute ParticipantAction = iota
	// ParticipantUnmute unmutes a participant.
	ParticipantUnmute
	// ParticipantVolumeUp increases participant volume by 10%.
	ParticipantVolumeUp
	// ParticipantVolumeDown decreases participant volume by 10%.
	ParticipantVolumeDown
	// ParticipantMutePersist mutes and remembers — auto-applied on reconnect.
	ParticipantMutePersist
	// ParticipantUnmutePersist unmutes and removes from persistent mute list.
	ParticipantUnmutePersist
	// ParticipantKick disconnects a participant from the conference.
	// Consumed by the API-level HandleParticipantCommand path; TUI currently
	// does not emit this action.
	ParticipantKick
	// ParticipantSetVolume sets an absolute volume multiplier (carried on
	// the Volume field of ParticipantCommand). Used by the API path — TUI
	// still uses the VolumeUp/VolumeDown toggle actions.
	ParticipantSetVolume
	// ParticipantSetMute sets an absolute mute state (carried on the Muted
	// field of ParticipantCommand). Used by the API path — TUI uses the
	// Mute/Unmute toggle actions.
	ParticipantSetMute
)

// ParticipantCommand is sent from TUI or the daemon HTTP API to control
// conference participants.
//
// The Muted / Volume fields are only consumed for the API-originating
// actions (ParticipantSetMute / ParticipantSetVolume). TUI-originating
// actions (Mute / Unmute / VolumeUp / VolumeDown) leave them at their
// zero values.
type ParticipantCommand struct {
	Action        ParticipantAction
	ParticipantID string
	Muted         bool    // absolute value for ParticipantSetMute (ignored otherwise)
	Volume        float64 // absolute value for ParticipantSetVolume, range 0.0–1.5
}

// ConferenceStatsPayload is sent from server to TUI with participant states and recording status.
type ConferenceStatsPayload struct {
	States             []audio.ParticipantState
	Recording          bool
	PausedParticipants map[string]bool // per-participant pause state
}

// ParticipantPauseMsg notifies the TUI that a remote participant changed pause state.
type ParticipantPauseMsg struct {
	ParticipantID string
	Paused        bool
}

// ConferenceParticipantsMsg carries a full participant list with pause states from the server.
type ConferenceParticipantsMsg struct {
	Participants []ConferenceParticipantInfo
}

// ConferenceParticipantInfo holds participant data from a participants_update message.
type ConferenceParticipantInfo struct {
	ID       string
	Nickname string
	Paused   bool
}

// RecordingCommand is sent from TUI to start/stop conference recording.
type RecordingCommand struct {
	Start          bool // true=start, false=stop
	Mode           audio.RecordingMode
	LocalDeviceIDs []string
	RemoteIDs      []string
}
