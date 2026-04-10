package echowarp

import (
	"context"
	"crypto/tls"
	"log/slog"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// Runner defines the interface for the main execution loop of a Node.
// Implementations (ServerApp, ClientApp) handle the actual streaming workflow.
type Runner interface {
	// Run executes the streaming workflow. It blocks until the context is canceled
	// or a fatal error occurs. Returns nil on graceful shutdown.
	Run(ctx context.Context) error
}

// DeviceCommandReceiver is an optional interface that Runner implementations
// may satisfy to accept device-level control commands (mute / volume) issued
// via the public Node API (Node.SetDeviceMute, Node.SetDeviceVolume).
//
// Runners that do not implement this interface cause the corresponding Node
// methods to return an ErrNotRunning-class error. This keeps the core Runner
// contract minimal while still allowing typed device control without the
// caller knowing the concrete runner type.
type DeviceCommandReceiver interface {
	// HandleDeviceCommand enqueues a device command for processing by the
	// runner. Implementations should return quickly (non-blocking, or with
	// a short timeout) and must be safe to call from any goroutine while
	// the runner is executing Run(ctx).
	HandleDeviceCommand(cmd DeviceCommand) error
}

// ParticipantLister is an optional interface that Runner implementations may
// satisfy to expose a snapshot of the current conference participants via
// Node.Participants(). Runners that do not implement this interface cause
// Node.Participants() to return an empty slice (see echowarp.go).
type ParticipantLister interface {
	// Participants returns a snapshot of the current conference participants.
	// The returned slice is owned by the caller and must not be mutated by
	// the runner after being returned.
	Participants() []ParticipantInfo
}

// ChatSender is an optional interface that Runner implementations may satisfy
// to accept outgoing chat messages issued via the public Node API
// (Node.SendChat).
//
// Runners that do not implement this interface cause Node.SendChat to return
// an ErrInternalState-class error. This keeps the core Runner contract minimal
// while still allowing typed chat send without the caller knowing the
// concrete runner type.
type ChatSender interface {
	// SendChat enqueues a chat message for delivery. Empty to means
	// broadcast to all participants; non-empty to is a direct-message to
	// that participant (addressed by nickname or id, implementation-defined).
	// Implementations should return quickly (non-blocking, or with a short
	// timeout) and must be safe to call from any goroutine while the runner
	// is executing Run(ctx).
	SendChat(text, to string) error
}

// ParticipantCommandReceiver is an optional interface that Runner
// implementations may satisfy to accept conference-participant-level control
// commands (mute / kick / volume) issued via the public Node API
// (Node.MuteParticipant, Node.KickParticipant, Node.SetParticipantVolume).
//
// Runners that do not implement this interface cause the corresponding Node
// methods to return an ErrInternalState-class error. This keeps the core
// Runner contract minimal while still allowing typed participant control
// without the caller knowing the concrete runner type.
type ParticipantCommandReceiver interface {
	// HandleParticipantCommand enqueues a participant command for processing
	// by the runner. Implementations should return quickly (non-blocking, or
	// with a short timeout) and must be safe to call from any goroutine
	// while the runner is executing Run(ctx).
	HandleParticipantCommand(cmd ParticipantCommand) error
}

// BanManager is an optional interface that Runner implementations may
// satisfy to expose CRUD access to the underlying ban list via the public
// Node API (Node.BanList, Node.AddBan, Node.RemoveBan). Runners that do
// not implement this interface cause Node.BanList to return an empty
// slice and Node.AddBan/RemoveBan to return an ErrInternalState-class
// error. This mirrors the ParticipantLister / ChatSender pattern so the
// core Runner contract stays minimal — only runners that actually host a
// ban list (e.g. ServerApp) implement the interface.
//
// Implementations must be safe to call concurrently with Run(ctx). The
// adapter is expected to delegate to the same ban.BanManager instance
// that is consulted on incoming connections, so that bans added through
// the API take effect immediately without requiring a reload.
type BanManager interface {
	// BanList returns a snapshot of all currently active bans across all
	// kinds (IP, HWID, Nickname) flattened into a single slice. The
	// returned slice is owned by the caller; mutating it must not
	// affect the runner's internal state.
	BanList() []BanEntry

	// AddBan installs a new ban. Exactly one of entry.IP, entry.HWID,
	// or entry.Nickname must be non-empty — the runner is responsible
	// for validating this and returning an error otherwise (the Node
	// wrapper also pre-validates, so implementations can assume the
	// happy path in tests).
	AddBan(entry BanEntry) error

	// RemoveBan removes a previously added ban identified by its
	// stable ID ("<kind>:<subject>"). Unknown IDs are reported as an
	// error so the API layer can return a 404-equivalent status.
	RemoveBan(id string) error
}

// RecordingController is an optional interface that Runner implementations
// may satisfy to expose on-demand start/stop control of the audio recorder
// via the public Node API (Node.StartRecording, Node.StopRecording,
// Node.RecordingStatus). Runners that do not implement this interface
// cause the Start/Stop node methods to return an ErrInternalState-class
// error and Node.RecordingStatus to return a zero-value (Active=false)
// status so GET /api/v1/recording/status remains idempotent even on
// recording-incapable runners.
//
// Implementations must be safe to call concurrently with Run(ctx). The
// adapter is expected to delegate to the same ConferenceRecorder (or
// equivalent) instance that the CLI --record flag drives, so starting
// a recording via the API during an already-active CLI recording is
// reported as an error rather than silently spawning a parallel writer.
type RecordingController interface {
	// StartRecording begins a recording session in the given mode.
	// Must return a non-nil error if a recording is already active on
	// the underlying recorder so the caller learns about the conflict
	// instead of silently overwriting the existing session.
	StartRecording(mode RecordingMode) error

	// StopRecording stops the active recording and returns a summary
	// of the session. When no recording was active, implementations
	// should return a zero-value RecordingResult and nil error so the
	// API call is idempotent — the Node wrapper distinguishes "never
	// running" (ErrNotRunning) from "running but nothing to stop"
	// (nil error, empty result) itself.
	StopRecording() (RecordingResult, error)

	// RecordingStatus returns the current state of the recorder. Must
	// return a zero-value (Active=false) status when no recording is
	// in progress — see the Node wrapper comment for why this is
	// required to keep GET /api/v1/recording/status idempotent.
	RecordingStatus() RecordingStatus
}

// MuteController is an optional interface that Runner implementations may
// satisfy to expose a simple on/off toggle for the incoming audio stream
// via the public Node API (Node.SetMuted). The interface is intended for
// client-mode runners — "mute" in this context means "drop every decoded
// audio frame that would otherwise be handed to the playback device", not
// "tell the peer to stop sending" (that is a separate control message
// handled via the per-peer mute path). Server-mode runners typically do
// not implement this interface because a server has no single "incoming"
// stream to mute.
//
// Runners that do not implement this interface cause Node.SetMuted to
// return an ErrInternalState-class error. Implementations must be safe
// to call concurrently with Run(ctx) and must return quickly — the
// intended implementation is a single atomic store.
type MuteController interface {
	// SetMuted toggles local mute of the incoming audio stream. When
	// muted is true, decoded frames are dropped before reaching the
	// playback device; when false, audio resumes immediately. The
	// operation must be idempotent: SetMuted(true) twice is equivalent
	// to SetMuted(true) once.
	SetMuted(muted bool) error
}

// DiscoveryPublisher is an optional interface that Runner implementations
// may satisfy to expose a runtime on/off toggle for mDNS service
// publishing via the public Node API (Node.SetDiscoveryPublish). The
// interface is intended for server-mode runners — a client has nothing
// to advertise.
//
// Runners that do not implement this interface cause
// Node.SetDiscoveryPublish to return an ErrInternalState-class error.
// Implementations must be safe to call concurrently with Run(ctx); the
// canonical implementation spawns/tears down a background zeroconf
// publisher goroutine under its own mutex and never blocks the caller
// on network I/O.
type DiscoveryPublisher interface {
	// SetDiscoveryPublish starts or stops mDNS publishing. When enabled
	// is true and publishing is not currently active, the runner starts
	// a new publisher; when enabled is false and publishing is active,
	// the runner cancels the publisher's context and waits for it to
	// exit. Both directions are idempotent: calling SetDiscoveryPublish
	// with the current state is a no-op and returns nil.
	SetDiscoveryPublish(enabled bool) error
}

// PauseController is an optional interface that Runner implementations may
// satisfy to propagate Node-level Pause/Resume calls into the runner's audio
// send pipeline. "Pause" in this context means "stop advancing the outgoing
// audio frame counter" — the underlying pipeline stays wired so Resume is
// instantaneous and timing stays aligned. Runners that do not implement this
// interface cause Node.Pause/Node.Resume to fall back to state-only
// transitions (for backwards compatibility with runners predating phase 6);
// see echowarp.go Pause/Resume for the exact semantics.
//
// Implementations must:
//   - Be safe to call concurrently with Run(ctx). The canonical
//     implementation is a single atomic.Bool store plus an optional
//     best-effort notification to already-connected peers.
//   - Be idempotent in both directions: SetPaused(true) twice is
//     equivalent to SetPaused(true) once, and the same for false.
//   - Return quickly — no blocking I/O. The Node wrapper holds no locks
//     during the call, but the HTTP handler that triggered Pause/Resume
//     is waiting for the result synchronously.
type PauseController interface {
	// SetPaused toggles the runner-level pause state. When paused is
	// true the runner must stop advancing its outgoing audio frame
	// counter (drop or skip encode at the earliest sensible point in
	// the pipeline); when false it must resume immediately.
	SetPaused(paused bool) error
}

// RunnerFactory creates a Runner instance based on configuration.
// This function bridges the pkg/echowarp package with internal/app implementations,
// avoiding direct imports that would break the package boundary.
type RunnerFactory func(cfg NodeConfig, logger *slog.Logger, banMgr ban.BanManager, tlsConfig *tls.Config, rateLimiter *auth.IPRateLimiter) (Runner, error)
