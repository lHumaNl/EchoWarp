package tui

import (
	"time"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// tickMsg triggers a periodic UI refresh for the header timer.
type tickMsg time.Time

// spectrumTickMsg triggers a spectrum display refresh.
type spectrumTickMsg time.Time

// DeviceSelectedMsg is sent when a device is selected from the list.
type DeviceSelectedMsg struct {
	Device audio.AudioDevice
}

// ConnectedMsg is sent when a connection is established.
type ConnectedMsg struct {
	Stats transport.ConnectionStats
}

// StatsUpdateMsg is sent periodically with updated connection statistics.
type StatsUpdateMsg struct {
	Stats transport.ConnectionStats
}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct {
	Err error
}

// StreamingStartedMsg is sent when audio streaming begins.
type StreamingStartedMsg struct{}

// ErrorDismissMsg is sent after the error banner auto-dismiss timer fires.
type ErrorDismissMsg struct{}

// streamingStartedMsg carries channels returned by StartFunc.
type streamingStartedMsg struct {
	statsCh         <-chan transport.ConnectionStats
	errCh           <-chan error
	serverStoppedCh <-chan struct{}
}

// streamingEndedMsg is sent when the streaming session ends.
type streamingEndedMsg struct{}

// ServerStoppedMsg is sent when the server sends ActionStop (graceful shutdown).
type ServerStoppedMsg struct{}

// ReconnectTickMsg is the 1-second countdown tick for auto-reconnect after server stop.
type ReconnectTickMsg struct{}

// ReconnectProbeMsg carries the result of a probe attempt during reconnect.
type ReconnectProbeMsg struct {
	Result *views.ProbeServerResult // nil if probe failed
	Err    error                    // non-nil if probe failed
}

// PreStartProbeMsg carries the result of a pre-start probe (client mode).
type PreStartProbeMsg struct {
	Result *views.ProbeServerResult
	Err    error
}

// MultiStatsUpdateMsg carries multi-client stats from the server.
type MultiStatsUpdateMsg struct {
	Stats transport.MultiClientStats
}

// ParticipantsUpdateMsg carries a participant list update from the server (via chat DC).
type ParticipantsUpdateMsg struct {
	Participants []string
	MaxClients   int
}

// ConferenceStatsPayload is the data sent over the conference stats channel.
type ConferenceStatsPayload struct {
	States             []audio.ParticipantState
	Recording          bool
	PausedParticipants map[string]bool

	// RecordingStopped is set on the first stats tick after a recording stop.
	RecordingStopped    bool
	RecordingStopDur    time.Duration
	RecordingStopSize   uint64
	RecordingStopFiles  int
	RecordingStopDir    string
	RecordingStopReason string
}

// ConferenceStatsMsg carries conference participant state updates from the server.
type ConferenceStatsMsg = ConferenceStatsPayload

// ParticipantCommand mirrors app.ParticipantCommand for TUI layer.
type ParticipantCommand struct {
	Action        ParticipantAction
	ParticipantID string
}

// ParticipantAction mirrors app.ParticipantAction.
type ParticipantAction int

const (
	ParticipantMute          ParticipantAction = 0
	ParticipantUnmute        ParticipantAction = 1
	ParticipantVolumeUp      ParticipantAction = 2
	ParticipantVolumeDown    ParticipantAction = 3
	ParticipantMutePersist   ParticipantAction = 4
	ParticipantUnmutePersist ParticipantAction = 5
)

// RecordingCommand is sent from TUI to app layer to start/stop recording.
type RecordingCommand struct {
	Start          bool // true=start, false=stop
	Mode           views.RecordingMode
	LocalDeviceIDs []string // selected capture device IDs
	PlaybackIDs    []string // selected playback device IDs
	RemoteIDs      []string // selected remote source IDs
}

// RecordingStatusUpdate is sent from the app layer to TUI to update recording status.
type RecordingStatusUpdate struct {
	Dir      string // recording output directory
	FileName string // primary file name
	Size     uint64 // total size in bytes
}

// DeviceState holds the current display state of a capture/playback device.
type DeviceState struct {
	ID           uint32
	Name         string
	Role         string  // "capture" or "playback"
	Volume       float64 // 0.0–1.5
	Muted        bool
	AGC          bool
	Disconnected bool
}

// DeviceChangeMsg is sent when a device is added or removed (hot-plug).
type DeviceChangeMsg struct {
	Added   bool // true = added, false = removed
	Name    string
	IsInput bool
}

// ChatReceivedMsg is sent when a chat message arrives from the app layer.
type ChatReceivedMsg struct {
	Message app.ChatMessage
}

// chatNicknameMsg is sent when the server-assigned nickname arrives.
type chatNicknameMsg string

// ParticipantPauseMsg is sent when a remote participant changes pause state.
type ParticipantPauseMsg = app.ParticipantPauseMsg

// ConferenceParticipantsMsg carries a full participants list with pause states from the server.
type ConferenceParticipantsMsg = app.ConferenceParticipantsMsg

// ConferenceParticipantInfo holds participant data from a participants_update message.
type ConferenceParticipantInfo = app.ConferenceParticipantInfo

// PeerMuteMsg reports whether the remote side muted this client's outgoing audio.
type PeerMuteMsg bool

// KickedByServerMsg is sent when the server kicks this client.
type KickedByServerMsg struct {
	Reason string
}

// BannedByServerMsg is sent when the server bans this client.
type BannedByServerMsg struct {
	Reason   string
	Criteria []string
}

// recentServerSavedMsg is a no-op message sent after saving recent servers.
type recentServerSavedMsg struct{}
