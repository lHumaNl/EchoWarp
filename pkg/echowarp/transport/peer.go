package transport

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// WebRTCPeer implements the PeerManager interface using pion/webrtc.
// It manages a single WebRTC peer connection with audio tracks and data channels.
//
// Thread-safe: Internal state protected by sync.RWMutex.
type WebRTCPeer struct {
	pc                    *webrtc.PeerConnection
	direction             MediaDirection
	audioTrack            *webrtc.TrackLocalStaticSample
	audioTrackChan        chan []byte
	audioTrackDone        chan struct{}
	onAudioTrackHandler   func(inCh <-chan []byte)
	onRTPTrackHandler     func(RTPTrackInfo, <-chan *rtp.Packet)
	rtpTracks             map[string]*conferenceRTPTrack
	rtpTrackMu            sync.Mutex // Serializes track changes; Close never acquires it.
	controlOverflowOnce   sync.Once
	connStateHandler      func(state webrtc.PeerConnectionState)
	onICECandidateHandler func(candidate *webrtc.ICECandidate)
	dataChannelHandler    func(label string, msgCh <-chan []byte, sendFn func([]byte) error)
	dataChannels          map[chan []byte]struct{}
	bufferedCandidates    []webrtc.ICECandidateInit
	remoteDescSet         bool
	controlDC             *webrtc.DataChannel
	controlSendFn         func([]byte) error
	controlMsgCh          chan []byte
	chatDC                *webrtc.DataChannel
	chatSendFn            func([]byte) error
	chatMsgCh             chan []byte
	dcReadyCh             chan struct{}
	dcReadyOnce           sync.Once
	closed                bool
	mu                    sync.RWMutex
}

// NewWebRTCPeer creates a new WebRTCPeer with the specified media direction.
// DirectionSend: peer will send audio; DirectionReceive: peer will receive audio.
func NewWebRTCPeer(direction MediaDirection) *WebRTCPeer {
	return &WebRTCPeer{
		direction:    direction,
		dataChannels: make(map[chan []byte]struct{}),
		dcReadyCh:    make(chan struct{}),
		rtpTracks:    make(map[string]*conferenceRTPTrack),
	}
}

// CreatePeerConnection initializes the underlying WebRTC peer connection.
// Must be called before any other peer operations.
func (p *WebRTCPeer) CreatePeerConnection(iceConfig ICEConfig) error {
	config := iceConfig.ToWebRTCConfig()

	var pc *webrtc.PeerConnection
	var err error
	if iceConfig.UDPMux != nil {
		pc, err = iceConfig.UDPMux.WebRTCAPI().NewPeerConnection(config)
	} else {
		pc, err = webrtc.NewPeerConnection(config)
	}
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to create peer connection").
			WithSuggestion("Check ICE server configuration and network connectivity")
	}

	p.pc = pc
	p.setupTrackHandler(pc)
	p.setupDataChannelHandler(pc)
	p.setupConnectionStateHandler(pc)
	p.setupICECandidateHandler(pc)

	return nil
}

func (p *WebRTCPeer) setupDataChannelHandler(pc *webrtc.PeerConnection) {
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		p.mu.RLock()
		handler := p.dataChannelHandler
		p.mu.RUnlock()

		ch := p.dataChannelQueue(dc)
		dc.OnOpen(func() {
			p.handleDataChannelOpen(dc, ch, handler)
		})
	})
}

func (p *WebRTCPeer) dataChannelQueue(dc *webrtc.DataChannel) chan []byte {
	// Register before OnOpen so immediate remote messages (including dc_ready) survive.
	ch := make(chan []byte, dataChannelQueueCapacity)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		p.handleDataChannelMessage(dc.Label(), ch, msg)
	})
	return ch
}

func (p *WebRTCPeer) handleDataChannelOpen(dc *webrtc.DataChannel, ch chan []byte, handler func(string, <-chan []byte, func([]byte) error)) {
	p.mu.Lock()
	if p.closed {
		close(ch)
		p.mu.Unlock()
		return
	}
	p.dataChannels[ch] = struct{}{}

	if dc.Label() == LabelControl {
		p.controlDC = dc
		p.controlSendFn = func(data []byte) error {
			return dc.Send(data)
		}
		p.controlMsgCh = ch
	}
	if dc.Label() == LabelChat {
		p.chatDC = dc
		p.chatSendFn = func(data []byte) error {
			return dc.Send(data)
		}
		p.chatMsgCh = ch
	}
	p.mu.Unlock()

	// OnMessage is already registered before OnOpen (in setupDataChannelHandler)
	// to avoid losing messages sent by the remote side immediately after its OnOpen.

	sendFn := func(data []byte) error {
		return dc.Send(data)
	}
	if handler != nil {
		go handler(dc.Label(), ch, sendFn)
	}

	if dc.Label() == LabelControl {
		go p.sendDCReady()
	}
}

func (p *WebRTCPeer) handleDataChannelMessage(label string, ch chan<- []byte, msg webrtc.DataChannelMessage) {
	p.withOpenDataChannels(func() {
		if label == LabelControl && isDCReady(msg.Data) {
			p.markDCReady()
			return
		}
		select {
		case ch <- msg.Data:
		default:
			metrics.FramesDroppedTotal.WithLabelValues("datachannel").Inc()
			if label == LabelControl {
				p.controlOverflowOnce.Do(func() { go p.closeOnControlOverflow() })
			}
		}
	})
}

type peerControlMessage struct {
	Type    string `json:"type"`
	Payload struct {
		Action string      `json:"action"`
		Data   interface{} `json:"data,omitempty"`
	} `json:"payload"`
}

func isDCReady(data []byte) bool {
	var msg peerControlMessage
	return json.Unmarshal(data, &msg) == nil && msg.Type == TypeControl && msg.Payload.Action == ActionDCReady
}

func (p *WebRTCPeer) closeOnControlOverflow() {
	// Close needs the delivery guard exclusively and must not run in an SCTP
	// callback. Never acknowledge a session with lost reliable control state.
	slog.Warn("Closing peer after control channel queue overflow")
	if err := p.Close(); err != nil {
		slog.Warn("Failed to close peer after control overflow", "error", err)
	}
}

func (p *WebRTCPeer) withOpenDataChannels(deliver func()) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return
	}
	deliver()
}

func (p *WebRTCPeer) setupConnectionStateHandler(pc *webrtc.PeerConnection) {
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.mu.RLock()
		h := p.connStateHandler
		p.mu.RUnlock()
		if h != nil {
			h(state)
		}
	})
}

func (p *WebRTCPeer) setupICECandidateHandler(pc *webrtc.PeerConnection) {
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		p.mu.RLock()
		h := p.onICECandidateHandler
		p.mu.RUnlock()
		if h != nil {
			h(candidate)
		}
	})
}

// CreateOffer creates an SDP offer and sets it as the local description.
func (p *WebRTCPeer) CreateOffer() (webrtc.SessionDescription, error) {
	pc, err := p.connection()
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	err = pc.SetLocalDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return offer, nil
}

// CreateAnswer creates an SDP answer in response to an offer.
// Sets the remote description and local description automatically.
func (p *WebRTCPeer) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	pc, err := p.connection()
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	err = pc.SetRemoteDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "Failed to set remote description")
	}

	p.applyBufferedCandidates()

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

// SetRemoteDescription sets the remote peer's SDP.
func (p *WebRTCPeer) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	pc, err := p.connection()
	if err != nil {
		return err
	}

	err = pc.SetRemoteDescription(sdp)
	if err != nil {
		return err
	}

	p.applyBufferedCandidates()

	return nil
}

func (p *WebRTCPeer) applyBufferedCandidates() {
	p.mu.Lock()
	pc, candidates := p.pc, p.bufferedCandidates
	p.bufferedCandidates = nil
	p.remoteDescSet = true
	p.mu.Unlock()
	if pc == nil {
		return
	}
	for _, c := range candidates {
		if err := pc.AddICECandidate(c); err != nil {
			slog.Warn("Failed to apply buffered ICE candidate", "error", err)
		}
	}
}

// AddICECandidate adds a remote ICE candidate.
// Candidates received before remote description are buffered and applied later.
func (p *WebRTCPeer) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	p.mu.Lock()
	pc := p.pc
	if pc == nil {
		p.mu.Unlock()
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	if !p.remoteDescSet {
		p.bufferedCandidates = append(p.bufferedCandidates, candidate)
		p.mu.Unlock()
		return nil
	}

	p.mu.Unlock()
	return pc.AddICECandidate(candidate)
}

// OnICECandidate registers a handler for local ICE candidates.
// Candidates should be sent to the remote peer via the signaling channel.
func (p *WebRTCPeer) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
	p.mu.Lock()
	p.onICECandidateHandler = handler
	p.mu.Unlock()
}

// OnConnectionStateChange registers a handler for connection state changes.
func (p *WebRTCPeer) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
	p.mu.Lock()
	p.connStateHandler = handler
	p.mu.Unlock()
}

// GetStats returns current connection statistics.
func (p *WebRTCPeer) GetStats() ConnectionStats {
	p.mu.Lock()
	pc := p.pc
	p.mu.Unlock()

	if pc == nil {
		return ConnectionStats{State: "new"}
	}

	state := pc.ConnectionState()
	stats := ConnectionStats{
		State: state.String(),
	}

	for _, stat := range pc.GetStats() {
		switch s := stat.(type) {
		case webrtc.ICECandidatePairStats:
			if s.Nominated && s.State == webrtc.StatsICECandidatePairStateSucceeded {
				if s.CurrentRoundTripTime > 0 {
					stats.RoundTrip = s.CurrentRoundTripTime * 1000 // seconds → ms
				}
				stats.BytesRecv = s.BytesReceived
				stats.BytesSent = s.BytesSent
			}
		case webrtc.InboundRTPStreamStats:
			if s.Kind == "audio" {
				stats.Jitter = s.Jitter * 1000 // seconds → ms
				if s.PacketsLost > 0 {
					stats.PacketsLost = uint32(s.PacketsLost)
				}
			}
		}
	}

	return stats
}

// CreateDataChannel creates a new data channel with the specified label.
func (p *WebRTCPeer) CreateDataChannel(label string) error {
	pc, err := p.connection()
	if err != nil {
		return err
	}

	_, err = pc.CreateDataChannel(label, nil)
	return err
}

// OnDataChannel registers a handler for incoming data channels.
func (p *WebRTCPeer) OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
	p.mu.Lock()
	p.dataChannelHandler = handler
	p.mu.Unlock()
}

// CreateControlDataChannel creates the control data channel used for post-handshake messaging.
// Automatically sends dc_ready when the channel opens.
func (p *WebRTCPeer) CreateControlDataChannel() error {
	return p.createMessagingDataChannel(LabelControl)
}

func (p *WebRTCPeer) createMessagingDataChannel(label string) error {
	pc, err := p.connection()
	if err != nil {
		return err
	}

	dc, err := pc.CreateDataChannel(label, nil)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDataChannelFailed, "Failed to create "+label+" data channel")
	}

	ch := p.dataChannelQueue(dc)
	dc.OnOpen(func() {
		p.handleDataChannelOpen(dc, ch, nil)
	})
	return nil
}

func (p *WebRTCPeer) markDCReady() {
	p.dcReadyOnce.Do(func() {
		close(p.dcReadyCh)
	})
}

func (p *WebRTCPeer) sendDCReady() {
	_ = p.SendControl(ActionDCReady, nil)
}

// DCReady returns a channel that closes when both peers have exchanged dc_ready.
func (p *WebRTCPeer) DCReady() <-chan struct{} {
	return p.dcReadyCh
}

// SendControl sends a control message through the control data channel.
func (p *WebRTCPeer) SendControl(action string, payload interface{}) error {
	p.mu.RLock()
	sendFn := p.controlSendFn
	p.mu.RUnlock()

	if sendFn == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Control data channel not ready").
			WithSuggestion("Wait for the data channel to open before sending")
	}

	msg := peerControlMessage{Type: TypeControl}
	msg.Payload.Action, msg.Payload.Data = action, payload

	data, err := json.Marshal(msg)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to marshal control message")
	}

	return sendFn(data)
}

// ControlMessages returns the channel that receives raw control messages from the peer
// via the WebRTC data channel. Returns nil if the control channel hasn't been set up yet.
func (p *WebRTCPeer) ControlMessages() <-chan []byte {
	p.mu.RLock()
	ch := p.controlMsgCh
	p.mu.RUnlock()
	return ch
}

// CreateChatDataChannel creates the chat data channel used for text messaging.
func (p *WebRTCPeer) CreateChatDataChannel() error {
	return p.createMessagingDataChannel(LabelChat)
}

// SendChat sends raw bytes through the chat data channel.
func (p *WebRTCPeer) SendChat(data []byte) error {
	p.mu.RLock()
	sendFn := p.chatSendFn
	p.mu.RUnlock()

	if sendFn == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Chat data channel not ready").
			WithSuggestion("Wait for the chat data channel to open before sending")
	}

	return sendFn(data)
}

// ChatMessages returns the channel that receives raw chat messages from the peer
// via the WebRTC data channel. Returns nil if the chat channel hasn't been set up yet.
func (p *WebRTCPeer) ChatMessages() <-chan []byte {
	p.mu.RLock()
	ch := p.chatMsgCh
	p.mu.RUnlock()
	return ch
}

func (p *WebRTCPeer) connection() (*webrtc.PeerConnection, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.pc == nil {
		return nil, ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}
	return p.pc, nil
}
