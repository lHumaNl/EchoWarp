package transport

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
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
	connStateHandler      func(state webrtc.PeerConnectionState)
	onICECandidateHandler func(candidate *webrtc.ICECandidate)
	dataChannelHandler    func(label string, msgCh <-chan []byte, sendFn func([]byte) error)
	dataChannels          map[string]chan []byte
	bufferedCandidates    []webrtc.ICECandidateInit
	remoteDescSet         bool
	controlDC             *webrtc.DataChannel
	controlSendFn         func([]byte) error
	chatDC                *webrtc.DataChannel
	chatSendFn            func([]byte) error
	chatMsgCh             chan []byte
	dcReadyCh             chan struct{}
	dcReadyOnce           sync.Once
	mu                    sync.RWMutex
}

// NewWebRTCPeer creates a new WebRTCPeer with the specified media direction.
// DirectionSend: peer will send audio; DirectionReceive: peer will receive audio.
func NewWebRTCPeer(direction MediaDirection) *WebRTCPeer {
	return &WebRTCPeer{
		direction:    direction,
		dataChannels: make(map[string]chan []byte),
		dcReadyCh:    make(chan struct{}),
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

func (p *WebRTCPeer) setupTrackHandler(pc *webrtc.PeerConnection) {
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		p.mu.RLock()
		handler := p.onAudioTrackHandler
		p.mu.RUnlock()

		if handler == nil {
			return
		}

		ch := make(chan []byte, 64)
		go func() {
			defer close(ch)
			for {
				pkt, _, err := track.ReadRTP()
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					return
				}
				select {
				case ch <- pkt.Payload:
				default:
					metrics.FramesDroppedTotal.WithLabelValues("opus").Inc()
				}
			}
		}()
		handler(ch)
	})
}

func (p *WebRTCPeer) setupDataChannelHandler(pc *webrtc.PeerConnection) {
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		p.mu.RLock()
		handler := p.dataChannelHandler
		p.mu.RUnlock()

		// Register OnMessage immediately — before OnOpen fires — so that
		// messages sent by the remote side right after its OnOpen are not lost.
		// This fixes a race where the creator sends dc_ready before the
		// receiver's OnOpen callback has had a chance to register OnMessage.
		ch := make(chan []byte, 64)
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			p.handleDataChannelMessage(dc.Label(), ch, msg)
		})

		dc.OnOpen(func() {
			p.handleDataChannelOpen(dc, ch, handler)
		})
	})
}

func (p *WebRTCPeer) handleDataChannelOpen(dc *webrtc.DataChannel, ch chan []byte, handler func(string, <-chan []byte, func([]byte) error)) {
	p.mu.Lock()
	p.dataChannels[dc.Label()] = ch

	if dc.Label() == LabelControl {
		p.controlDC = dc
		p.controlSendFn = func(data []byte) error {
			return dc.Send(data)
		}
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
	if label == LabelControl {
		var ctrlMsg struct {
			Type    string `json:"type"`
			Payload struct {
				Action string `json:"action"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(msg.Data, &ctrlMsg); err == nil {
			if ctrlMsg.Type == TypeControl && ctrlMsg.Payload.Action == ActionDCReady {
				p.markDCReady()
				return
			}
		}
	}
	select {
	case ch <- msg.Data:
	default:
		metrics.FramesDroppedTotal.WithLabelValues("datachannel").Inc()
	}
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

// AddAudioTransceiver adds an audio transceiver for receiving audio.
// Required for peers that will receive audio tracks.
func (p *WebRTCPeer) AddAudioTransceiver() error {
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	_, err := p.pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to add audio transceiver")
	}

	return nil
}

// AddAudioTrack creates an audio track for sending Opus-encoded audio.
// Returns a channel for writing audio frames. Each frame should be ~20ms of audio.
func (p *WebRTCPeer) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	if p.pc == nil {
		return nil, ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	codec := webrtc.RTPCodecCapability{
		MimeType:    webrtc.MimeTypeOpus,
		ClockRate:   48000,
		Channels:    0,
		SDPFmtpLine: "minptime=10;useinbandfec=1",
	}

	track, err := webrtc.NewTrackLocalStaticSample(codec, "audio", "echo")
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to create audio track").
			WithContext("sample_rate", sampleRate).
			WithContext("channels", channels)
	}

	_, err = p.pc.AddTrack(track)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to add track to connection")
	}

	p.audioTrack = track
	done := make(chan struct{})
	dataCh := make(chan []byte, 64)
	p.audioTrackChan = dataCh
	p.audioTrackDone = done

	go func() {
		frameDuration := 20 * time.Millisecond
		for {
			select {
			case <-done:
				return
			case data, ok := <-dataCh:
				if !ok {
					return
				}
				sample := media.Sample{Data: data, Duration: frameDuration}
				err := track.WriteSample(sample)
				audio.PutOpusOutput(data)
				if err != nil {
					return
				}
			}
		}
	}()

	return p.audioTrackChan, nil
}

// OnAudioTrack registers a handler for incoming audio tracks.
// The handler receives a channel of Opus-encoded audio frames.
func (p *WebRTCPeer) OnAudioTrack(handler func(inCh <-chan []byte)) {
	p.mu.Lock()
	p.onAudioTrackHandler = handler
	p.mu.Unlock()
}

// CreateOffer creates an SDP offer and sets it as the local description.
func (p *WebRTCPeer) CreateOffer() (webrtc.SessionDescription, error) {
	if p.pc == nil {
		return webrtc.SessionDescription{}, ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	err = p.pc.SetLocalDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return offer, nil
}

// CreateAnswer creates an SDP answer in response to an offer.
// Sets the remote description and local description automatically.
func (p *WebRTCPeer) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if p.pc == nil {
		return webrtc.SessionDescription{}, ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	err := p.pc.SetRemoteDescription(offer)
	if err != nil {
		return webrtc.SessionDescription{}, ewerrors.Wrap(err, ewerrors.ErrSignalingFailed, "Failed to set remote description")
	}

	p.applyBufferedCandidates()

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	err = p.pc.SetLocalDescription(answer)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	return answer, nil
}

// SetRemoteDescription sets the remote peer's SDP.
func (p *WebRTCPeer) SetRemoteDescription(sdp webrtc.SessionDescription) error {
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	err := p.pc.SetRemoteDescription(sdp)
	if err != nil {
		return err
	}

	p.applyBufferedCandidates()

	return nil
}

func (p *WebRTCPeer) applyBufferedCandidates() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.remoteDescSet = true
	for _, c := range p.bufferedCandidates {
		if err := p.pc.AddICECandidate(c); err != nil {
			slog.Warn("Failed to apply buffered ICE candidate", "error", err)
		}
	}
	p.bufferedCandidates = nil
}

// AddICECandidate adds a remote ICE candidate.
// Candidates received before remote description are buffered and applied later.
func (p *WebRTCPeer) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.remoteDescSet {
		p.bufferedCandidates = append(p.bufferedCandidates, candidate)
		return nil
	}

	return p.pc.AddICECandidate(candidate)
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
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	_, err := p.pc.CreateDataChannel(label, nil)
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
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	dc, err := p.pc.CreateDataChannel(LabelControl, nil)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDataChannelFailed, "Failed to create control data channel")
	}

	// Register OnMessage before OnOpen to avoid losing dc_ready messages
	// sent by the remote side immediately after its own OnOpen fires.
	ch := make(chan []byte, 64)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		var ctrlMsg struct {
			Type    string `json:"type"`
			Payload struct {
				Action string `json:"action"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(msg.Data, &ctrlMsg); err == nil {
			if ctrlMsg.Type == TypeControl && ctrlMsg.Payload.Action == ActionDCReady {
				p.markDCReady()
				return
			}
		}
		select {
		case ch <- msg.Data:
		default:
		}
	})

	dc.OnOpen(func() {
		p.mu.Lock()
		p.dataChannels[LabelControl] = ch
		p.controlDC = dc
		p.controlSendFn = func(data []byte) error {
			return dc.Send(data)
		}
		p.mu.Unlock()

		p.sendDCReady()
	})

	return nil
}

func (p *WebRTCPeer) markDCReady() {
	p.dcReadyOnce.Do(func() {
		close(p.dcReadyCh)
	})
}

func (p *WebRTCPeer) sendDCReady() {
	p.mu.RLock()
	sendFn := p.controlSendFn
	p.mu.RUnlock()

	if sendFn == nil {
		return
	}

	msg := struct {
		Type    string `json:"type"`
		Payload struct {
			Action string `json:"action"`
		} `json:"payload"`
	}{
		Type: TypeControl,
		Payload: struct {
			Action string `json:"action"`
		}{
			Action: ActionDCReady,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = sendFn(data) //nolint:errcheck
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

	msg := struct {
		Type    string `json:"type"`
		Payload struct {
			Action string      `json:"action"`
			Data   interface{} `json:"data,omitempty"`
		} `json:"payload"`
	}{
		Type: TypeControl,
		Payload: struct {
			Action string      `json:"action"`
			Data   interface{} `json:"data,omitempty"`
		}{
			Action: action,
			Data:   payload,
		},
	}

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
	ch := p.dataChannels[LabelControl]
	p.mu.RUnlock()
	return ch
}

// CreateChatDataChannel creates the chat data channel used for text messaging.
func (p *WebRTCPeer) CreateChatDataChannel() error {
	if p.pc == nil {
		return ewerrors.NewError(ewerrors.ErrNotInitialized, "Peer connection not created").
			WithSuggestion("Call CreatePeerConnection first")
	}

	dc, err := p.pc.CreateDataChannel(LabelChat, nil)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDataChannelFailed, "Failed to create chat data channel")
	}

	// Register OnMessage before OnOpen to avoid losing messages
	// sent by the remote side immediately after its own OnOpen fires.
	ch := make(chan []byte, 64)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		select {
		case ch <- msg.Data:
		default:
			metrics.FramesDroppedTotal.WithLabelValues("datachannel").Inc()
		}
	})

	dc.OnOpen(func() {
		p.mu.Lock()
		p.dataChannels[LabelChat] = ch
		p.chatDC = dc
		p.chatSendFn = func(data []byte) error {
			return dc.Send(data)
		}
		p.chatMsgCh = ch
		p.mu.Unlock()
	})

	return nil
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

// Close terminates the peer connection and releases all resources.
func (p *WebRTCPeer) Close() error {
	p.mu.Lock()

	done := p.audioTrackDone
	dataCh := p.audioTrackChan
	chatDC := p.chatDC
	pc := p.pc

	p.audioTrackDone = nil
	p.audioTrackChan = nil
	p.chatDC = nil
	p.chatSendFn = nil
	p.chatMsgCh = nil
	p.pc = nil

	for k, ch := range p.dataChannels {
		close(ch)
		delete(p.dataChannels, k)
	}
	p.dataChannels = make(map[string]chan []byte)

	p.mu.Unlock()

	if done != nil {
		close(done)
	}
	if dataCh != nil {
		close(dataCh)
	}
	if chatDC != nil {
		_ = chatDC.Close()
	}

	if pc != nil {
		return pc.Close()
	}
	return nil
}
