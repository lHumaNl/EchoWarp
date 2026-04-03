// Package echowarp provides the public API for EchoWarp audio streaming.
// It uses a RunnerFactory pattern to create server/client runners, avoiding
// direct imports of internal packages. See runner.go for the Runner interface.
package echowarp

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type AudioDevice = audio.AudioDevice
type ConnectionStats = transport.ConnectionStats
type TURNServer = transport.TURNServer

// Mode represents the operational role of a Node (server or client).
type Mode string

const (
	// ModeServer configures the node to accept incoming connections and stream audio to clients.
	ModeServer Mode = "server"
	// ModeClient configures the node to connect to a server and receive audio.
	ModeClient Mode = "client"
)

// AudioMode represents the audio streaming mode.
type AudioMode string

const (
	// AudioModeNormal is the default mode: server captures, client plays back.
	AudioModeNormal AudioMode = "normal"
	// AudioModeReverse swaps audio direction: client captures, server plays back.
	AudioModeReverse AudioMode = "reverse"
	// AudioModeDuplex enables bidirectional audio.
	AudioModeDuplex AudioMode = "duplex"
	// AudioModeConference enables multi-participant audio mixing (SFU).
	AudioModeConference AudioMode = "conference"
)

// NodeStatus represents the current state of a Node in its lifecycle.
type NodeStatus string

// Node status constants define the possible states of a Node.
// State transitions follow this pattern:
//
//	Idle → Connecting → Streaming → Stopped
//	Streaming ↔ Paused (via Pause/Resume)
//	Any state → Stopped (via Stop or error)
const (
	// StatusIdle indicates the node is not running and ready to start.
	StatusIdle NodeStatus = "idle"
	// StatusConnecting indicates the node is establishing a connection.
	StatusConnecting NodeStatus = "connecting"
	// StatusStreaming indicates the node is actively streaming audio.
	StatusStreaming NodeStatus = "streaming"
	// StatusReconnecting indicates the node lost connection and is attempting to reconnect.
	StatusReconnecting NodeStatus = "reconnecting"
	// StatusStopped indicates the node has been stopped and can be restarted.
	StatusStopped NodeStatus = "stopped"
	// StatusPaused indicates streaming is temporarily suspended.
	StatusPaused NodeStatus = "paused"
)

// DeviceRole specifies whether a device is used for capture or playback.
type DeviceRole string

const (
	// RoleCapture means the device captures audio for sending.
	RoleCapture DeviceRole = "capture"
	// RolePlayback means the device plays back received audio.
	RolePlayback DeviceRole = "playback"
)

// DeviceType indicates whether the audio device is an input or output device.
type DeviceType string

const (
	// DeviceInput is a microphone / capture hardware device.
	DeviceInput DeviceType = "input"
	// DeviceOutput is a speaker / playback hardware device.
	DeviceOutput DeviceType = "output"
)

// DeviceEntry represents a single audio device in a multi-device configuration.
type DeviceEntry struct {
	ID     uint32     `yaml:"id"     json:"id"`               // Audio device ID.
	Name   string     `yaml:"name"   json:"name,omitempty"`   // Human-readable device name. Used as fallback when IDs change across reboots.
	Type   DeviceType `yaml:"type"   json:"type,omitempty"`   // Hardware type: input or output. Used to disambiguate overlapping IDs.
	Role   DeviceRole `yaml:"role"   json:"role,omitempty"`   // Device role: capture or playback. Empty = inferred from mode.
	Volume float64    `yaml:"volume" json:"volume,omitempty"` // Volume multiplier 0.0-2.0 (default 1.0).
	Muted  bool       `yaml:"muted"  json:"muted,omitempty"`  // If true, device is muted (capture paused / playback silent).

	// MixInputID, when set, specifies a local input device whose audio is mixed into
	// this playback device's output stream. Used to combine a local microphone with
	// a remote stream on virtual output devices (BlackHole, VB-Cable, etc.).
	MixInputID   *uint32 `yaml:"mix_input_id,omitempty"   json:"mix_input_id,omitempty"`
	MixInputName string  `yaml:"mix_input_name,omitempty" json:"mix_input_name,omitempty"`
}

// NodeConfig holds all configuration parameters for a Node.
// Use NodeConfigBuilder for fluent construction with sensible defaults.
type NodeConfig struct {
	// Mode specifies whether the node operates as server or client.
	Mode Mode
	// Reverse swaps audio direction: server receives, client sends.
	Reverse bool
	// Duplex enables bidirectional audio: both sides send and receive simultaneously.
	Duplex bool
	// Conference enables conference mode (SFU: multi-participant audio forwarding).
	Conference bool

	// Port is the TCP port for signaling (server: listen, client: connect).
	Port int
	// Address is the server address to connect to (client mode only).
	Address string

	// DeviceID specifies the audio device to use. Nil triggers interactive selection.
	DeviceID *uint32
	// InputDeviceID specifies the input (microphone) device for duplex mode.
	InputDeviceID *uint32
	// OutputDeviceID specifies the output (speaker) device for duplex mode.
	OutputDeviceID *uint32
	// Devices is the multi-device configuration. Takes precedence over single device fields.
	Devices []DeviceEntry
	// SampleRate is the audio sample rate in Hz (8000, 12000, 16000, 24000, or 48000).
	SampleRate uint32
	// Channels is the number of audio channels (1 for mono, 2 for stereo).
	Channels uint32
	// VirtualMic enables creation of a virtual microphone device.
	VirtualMic bool

	// Opus configuration parameters.
	OpusBitrate     int    // Target bitrate in bits per second.
	OpusComplexity  int    // Encoder complexity (0-10).
	OpusApplication string // Application type: "voip", "audio", or "lowdelay".
	OpusDTX         bool   // Enable discontinuous transmission.
	OpusFEC         bool   // Enable in-band forward error correction.

	// Password for HMAC-SHA256 challenge-response authentication.
	Password string

	// TLS configuration for encrypted signaling.
	TLSCert       string // Path to TLS certificate file.
	TLSKey        string // Path to TLS private key file.
	TLS           bool   // Enable TLS for signaling.
	TLSInsecure   bool   // Skip certificate verification (insecure, testing only).
	TLSSelfSigned bool   // Runtime-only: true if TLS certificate is self-signed.

	// ICE server configuration for NAT traversal.
	STUNServers []string     // List of STUN server URLs.
	TURNServers []TURNServer // List of TURN relay servers with credentials.

	// Connection management parameters.
	MaxClients           int // Maximum concurrent clients (server mode). 1 = single-client mode.
	MaxReconnectAttempts int // Maximum reconnection attempts (0 = infinite).
	ReconnectIntervalSec int // Base interval between reconnection attempts in seconds.
	MaxFailedAttempts    int // Failed auth attempts before IP ban (0 = disabled).

	// BanFilePath is the path to the JSON file storing banned IPs.
	BanFilePath string

	// Nickname is the chat display name (client only). Empty = server assigns "Client-N".
	Nickname string
	// HWIDRequired when true, server requires clients to send a hardware identifier (for bans).
	HWIDRequired bool
}

// NodeConfigBuilder provides a fluent API for constructing NodeConfig with defaults.
// Example:
//
//	cfg := NewNodeConfigBuilder().
//	    Mode(ModeServer).
//	    Port(8080).
//	    Password("secret").
//	    Build()
type NodeConfigBuilder struct {
	cfg NodeConfig
}

// NewNodeConfigBuilder creates a builder with sensible defaults:
// 48kHz mono, 64kbps Opus, DTX and FEC enabled, 5 reconnect attempts.
func NewNodeConfigBuilder() *NodeConfigBuilder {
	return &NodeConfigBuilder{
		cfg: NodeConfig{
			SampleRate:           48000,
			Channels:             1,
			OpusBitrate:          64000,
			OpusComplexity:       5,
			OpusApplication:      "voip",
			OpusDTX:              true,
			OpusFEC:              true,
			MaxReconnectAttempts: 5,
			ReconnectIntervalSec: 5,
			MaxFailedAttempts:    5,
		},
	}
}

func (b *NodeConfigBuilder) Mode(mode Mode) *NodeConfigBuilder {
	b.cfg.Mode = mode
	return b
}

func (b *NodeConfigBuilder) Reverse(reverse bool) *NodeConfigBuilder {
	b.cfg.Reverse = reverse
	return b
}

func (b *NodeConfigBuilder) Duplex(duplex bool) *NodeConfigBuilder {
	b.cfg.Duplex = duplex
	return b
}

func (b *NodeConfigBuilder) Port(port int) *NodeConfigBuilder {
	b.cfg.Port = port
	return b
}

func (b *NodeConfigBuilder) Address(addr string) *NodeConfigBuilder {
	b.cfg.Address = addr
	return b
}

func (b *NodeConfigBuilder) Device(deviceID uint32) *NodeConfigBuilder {
	b.cfg.DeviceID = &deviceID
	return b
}

func (b *NodeConfigBuilder) InputDevice(deviceID uint32) *NodeConfigBuilder {
	b.cfg.InputDeviceID = &deviceID
	return b
}

func (b *NodeConfigBuilder) OutputDevice(deviceID uint32) *NodeConfigBuilder {
	b.cfg.OutputDeviceID = &deviceID
	return b
}

// AddDevice adds a device entry with the given role and default volume.
func (b *NodeConfigBuilder) AddDevice(deviceID uint32, role DeviceRole) *NodeConfigBuilder {
	b.cfg.Devices = append(b.cfg.Devices, DeviceEntry{
		ID:     deviceID,
		Role:   role,
		Volume: 1.0,
	})
	return b
}

// AddDeviceWithVolume adds a device entry with the given role and volume.
func (b *NodeConfigBuilder) AddDeviceWithVolume(deviceID uint32, role DeviceRole, volume float64) *NodeConfigBuilder {
	b.cfg.Devices = append(b.cfg.Devices, DeviceEntry{
		ID:     deviceID,
		Role:   role,
		Volume: volume,
	})
	return b
}

func (b *NodeConfigBuilder) Audio(sampleRate, channels uint32) *NodeConfigBuilder {
	b.cfg.SampleRate = sampleRate
	b.cfg.Channels = channels
	return b
}

func (b *NodeConfigBuilder) VirtualMic(enable bool) *NodeConfigBuilder {
	b.cfg.VirtualMic = enable
	return b
}

func (b *NodeConfigBuilder) Opus(bitrate, complexity int, app string) *NodeConfigBuilder {
	b.cfg.OpusBitrate = bitrate
	b.cfg.OpusComplexity = complexity
	b.cfg.OpusApplication = app
	return b
}

func (b *NodeConfigBuilder) OpusFeatures(dtx, fec bool) *NodeConfigBuilder {
	b.cfg.OpusDTX = dtx
	b.cfg.OpusFEC = fec
	return b
}

func (b *NodeConfigBuilder) Password(pwd string) *NodeConfigBuilder {
	b.cfg.Password = pwd
	return b
}

func (b *NodeConfigBuilder) TLS(cert, key string, enable bool) *NodeConfigBuilder {
	b.cfg.TLSCert = cert
	b.cfg.TLSKey = key
	b.cfg.TLS = enable
	return b
}

func (b *NodeConfigBuilder) TLSInsecure(insecure bool) *NodeConfigBuilder {
	b.cfg.TLSInsecure = insecure
	return b
}

func (b *NodeConfigBuilder) STUN(servers []string) *NodeConfigBuilder {
	b.cfg.STUNServers = servers
	return b
}

func (b *NodeConfigBuilder) TURN(servers []TURNServer) *NodeConfigBuilder {
	b.cfg.TURNServers = servers
	return b
}

func (b *NodeConfigBuilder) MaxClients(maxVal int) *NodeConfigBuilder {
	b.cfg.MaxClients = maxVal
	return b
}

func (b *NodeConfigBuilder) Reconnect(maxAttempts, intervalSec int) *NodeConfigBuilder {
	b.cfg.MaxReconnectAttempts = maxAttempts
	b.cfg.ReconnectIntervalSec = intervalSec
	return b
}

func (b *NodeConfigBuilder) BanFile(path string) *NodeConfigBuilder {
	b.cfg.BanFilePath = path
	return b
}

func (b *NodeConfigBuilder) Build() NodeConfig {
	return b.cfg
}

// ClientInfo represents a connected client in multi-client server mode.
type ClientInfo struct {
	ID      string // Unique client identifier.
	Address string // Client's remote address.
}

// Node is the main facade for EchoWarp audio streaming operations. It provides a
// high-level API for managing audio streaming sessions, hiding the complexity of
// transport, authentication, and audio processing subsystems.
//
// Node implements a state machine with the following transitions:
//
//	Idle → Connecting → Streaming → Stopped
//	Streaming ↔ Paused
//	Any state → Stopped (on fatal error or Stop call)
//
// Thread-safe: All public methods are safe for concurrent use.
type Node struct {
	mu            sync.RWMutex
	config        NodeConfig
	status        NodeStatus
	state         NodeState
	stats         ConnectionStats
	logger        *slog.Logger
	events        *EventHandler
	banMgr        ban.BanManager
	cancel        context.CancelFunc
	startTime     time.Time
	clients       []ClientInfo
	maxClients    int
	paused        atomic.Bool
	runnerFactory RunnerFactory

	runner     Runner
	runnerDone chan struct{} // closed when runRunner goroutine exits
}

// NewNode creates a new Node with the given configuration and optional functional options.
// The node starts in Idle status and requires Start() to begin streaming.
func NewNode(cfg NodeConfig, opts ...Option) (*Node, error) {
	n := &Node{
		config:     cfg,
		status:     StatusIdle,
		state:      stateIdle,
		logger:     slog.Default(),
		maxClients: cfg.MaxClients,
	}
	if n.maxClients == 0 {
		n.maxClients = 1
	}
	for _, opt := range opts {
		opt(n)
	}
	return n, nil
}

// Start begins audio streaming in the configured direction. It transitions the node
// from Idle to Connecting, performs authentication, establishes WebRTC connection,
// and starts audio capture/playback.
//
// Returns an error if the node is not in Idle or Stopped state, or if the runner
// factory is not configured. The method is non-blocking; errors during streaming
// are reported via the EventHandler.
func (n *Node) Start(ctx context.Context) error {
	if err := n.prepareToStart(); err != nil {
		return err
	}

	if n.runnerFactory == nil {
		n.mu.Lock()
		n.setState(StatusStopped)
		n.mu.Unlock()
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "No runner factory configured").
			WithSuggestion("Configure a runner factory using WithRunnerFactory option")
	}

	runCtx, cancel := context.WithCancel(ctx)
	n.cancel = cancel

	tlsConfig := n.createTLSConfig()
	rateLimiter := n.createRateLimiter()

	runner, err := n.runnerFactory(n.config, n.logger, n.banMgr, tlsConfig, rateLimiter)
	if err != nil {
		cancel()
		n.mu.Lock()
		n.setState(StatusStopped)
		n.mu.Unlock()
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to create runner")
	}
	n.runner = runner
	n.runnerDone = make(chan struct{})

	go n.runRunner(runCtx)
	return nil
}

func (n *Node) prepareToStart() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.state.CanStart() {
		return ewerrors.NewError(ewerrors.ErrAlreadyRunning, "Node already running").
			WithContext("status", string(n.status)).
			WithSuggestion("Stop the node before starting again")
	}
	n.setState(StatusConnecting)
	n.startTime = time.Now()
	return nil
}

func (n *Node) createTLSConfig() *tls.Config {
	if n.config.TLS {
		return &tls.Config{
			InsecureSkipVerify: n.config.TLSInsecure,
			MinVersion:         tls.VersionTLS12,
		}
	}
	return nil
}

func (n *Node) createRateLimiter() *auth.IPRateLimiter {
	if n.config.MaxFailedAttempts > 0 {
		return auth.NewIPRateLimiter(n.config.MaxFailedAttempts)
	}
	return nil
}

func (n *Node) runRunner(runCtx context.Context) {
	defer close(n.runnerDone)
	defer func() {
		n.mu.Lock()
		n.setState(StatusStopped)
		n.paused.Store(false)
		n.mu.Unlock()
		if n.events != nil {
			n.events.Bus().EmitDisconnected("stopped")
		}
	}()

	if n.events != nil {
		n.events.Bus().EmitConnected("")
	}

	n.mu.Lock()
	n.setState(StatusStreaming)
	n.mu.Unlock()

	if err := n.runner.Run(runCtx); err != nil && runCtx.Err() == nil {
		if n.events != nil {
			n.events.Bus().EmitError(err)
		}
	}
}

// Stop gracefully stops the streaming session. It cancels the internal context,
// transitions to Stopped status, and clears client state.
// Returns an error if the node is not currently running.
func (n *Node) Stop() error {
	n.mu.Lock()

	if !n.state.CanStop() {
		n.mu.Unlock()
		return ewerrors.NewError(ewerrors.ErrNotRunning, "Cannot stop: not running").
			WithContext("status", string(n.status)).
			WithSuggestion("Start the node before stopping")
	}

	done := n.runnerDone
	if n.cancel != nil {
		n.cancel()
		n.cancel = nil
	}
	n.mu.Unlock()

	// Wait for the runner goroutine to finish before updating state,
	// so a subsequent Start() won't race with the old goroutine's defer.
	if done != nil {
		<-done
	}

	n.mu.Lock()
	n.startTime = time.Time{}
	n.clients = nil
	n.paused.Store(false)
	n.mu.Unlock()
	return nil
}

// Status returns the current operational status of the node.
func (n *Node) Status() NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.status
}

// Stats returns the current connection statistics including bytes sent/received and latency.
func (n *Node) Stats() ConnectionStats {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.stats
}

// Config returns a copy of the node's current configuration.
func (n *Node) Config() NodeConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config
}

// SafeConfig returns a copy of the configuration with sensitive fields masked.
// Password and TURN credentials are replaced with "********" for safe logging.
func (n *Node) SafeConfig() NodeConfig {
	cfg := n.Config()
	if cfg.Password != "" {
		cfg.Password = "********"
	}
	for i := range cfg.TURNServers {
		cfg.TURNServers[i].Credential = "********"
	}
	return cfg
}

// Reconfigure updates the node's configuration. Only allowed when the node is
// Idle or Stopped. Returns an error if called while streaming.
func (n *Node) Reconfigure(cfg NodeConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.state.CanReconfigure() {
		return ewerrors.NewError(ewerrors.ErrAlreadyRunning, "Cannot reconfigure while running").
			WithContext("status", string(n.status)).
			WithSuggestion("Stop the node before reconfiguring")
	}
	n.config = cfg
	return nil
}

// UpdateStats replaces the current connection statistics. Used by runners to report metrics.
func (n *Node) UpdateStats(stats ConnectionStats) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stats = stats
}

// Uptime returns the duration since streaming started. Returns 0 if not currently streaming.
func (n *Node) Uptime() time.Duration {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.startTime.IsZero() || !n.state.CanStop() {
		return 0
	}
	return time.Since(n.startTime)
}

// AddClient registers a new connected client. Used in server mode for multi-client tracking.
func (n *Node) AddClient(info ClientInfo) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clients = append(n.clients, info)
}

// RemoveClient removes a client from the active client list by ID. No-op if ID not found.
func (n *Node) RemoveClient(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for i, c := range n.clients {
		if c.ID == id {
			n.clients = append(n.clients[:i], n.clients[i+1:]...)
			return
		}
	}
}

// Clients returns a copy of the active client list and the maximum client capacity.
func (n *Node) Clients() ([]ClientInfo, int) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	result := make([]ClientInfo, len(n.clients))
	copy(result, n.clients)
	return result, n.maxClients
}

func (n *Node) setState(status NodeStatus) {
	n.status = status
	n.state = stateFromStatus(status)
}

// Pause temporarily suspends audio streaming. Only valid when status is Streaming.
// Use Resume to continue streaming.
func (n *Node) Pause() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.state.CanPause() {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Cannot pause: not streaming").
			WithContext("status", string(n.status)).
			WithSuggestion("Start streaming before pausing")
	}
	n.paused.Store(true)
	n.setState(StatusPaused)
	return nil
}

// Resume continues audio streaming after being paused.
// Only valid when status is Paused.
func (n *Node) Resume() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.state.CanResume() {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Cannot resume: not paused").
			WithContext("status", string(n.status)).
			WithSuggestion("Pause the node before resuming")
	}
	n.paused.Store(false)
	n.setState(StatusStreaming)
	return nil
}

// IsPaused returns true if streaming is currently paused.
func (n *Node) IsPaused() bool {
	return n.paused.Load()
}

func (n *Node) Devices() ([]AudioDevice, error) {
	return listDevices()
}

// Devices returns a list of all available audio input and output devices on the system.
// This is a package-level convenience function equivalent to Node.Devices().
func Devices() ([]AudioDevice, error) {
	return listDevices()
}

func listDevices() ([]AudioDevice, error) {
	dm, err := audio.NewDeviceManager()
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "Failed to create device manager").
			WithSuggestion("Check audio system permissions and device availability")
	}
	defer func() { _ = dm.Close() }() //nolint:errcheck

	inputs, err := dm.ListInputDevices()
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "Failed to list input devices").
			WithSuggestion("Check audio system permissions")
	}
	outputs, err := dm.ListOutputDevices()
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "Failed to list output devices").
			WithSuggestion("Check audio system permissions")
	}

	return append(inputs, outputs...), nil
}
