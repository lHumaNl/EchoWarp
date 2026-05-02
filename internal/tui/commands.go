package tui

// ClientAction represents an action the TUI wants to perform on a client.
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

// DeviceCommand is sent from the TUI to control device volume/mute.
type DeviceCommand struct {
	Action   DeviceAction
	DeviceID uint32 // target device (ignored for GlobalMute)
}
