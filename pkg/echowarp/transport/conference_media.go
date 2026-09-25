package transport

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

const (
	dataChannelQueueCapacity = 64
	mediaQueueCapacity       = 64
	opusRTPClockRate         = 48000
	opusRTPChannels          = 2
	opusFrameDuration        = 20 * time.Millisecond
	opusFormatParameters     = "minptime=10;useinbandfec=1"
)

// RTPTrackInfo identifies a remote source. SourceID is untrusted track metadata;
// the server must associate it with an authenticated connection separately.
type RTPTrackInfo struct {
	SourceID string
	SSRC     uint32
}

// RTPWriter forwards RTP without changing the caller's packet or Opus payload.
type RTPWriter interface {
	WriteRTP(*rtp.Packet) error
}

// ConferenceMedia is an optional capability; PeerManager remains unchanged.
// Callers serialize renegotiation and own per-recipient delivery queues.
type ConferenceMedia interface {
	OnRTPTrack(func(RTPTrackInfo, <-chan *rtp.Packet))
	AddRTPTrack(sourceID string) (RTPWriter, error)
	RemoveRTPTrack(sourceID string) error
}

var _ ConferenceMedia = (*WebRTCPeer)(nil)

type conferenceRTPTrack struct {
	track    RTPWriter
	sender   *webrtc.RTPSender
	closed   atomic.Bool
	rtcpDone chan struct{}
}

func (t *conferenceRTPTrack) WriteRTP(packet *rtp.Packet) error {
	if t.closed.Load() {
		return errors.New("RTP track closed")
	}
	if packet == nil {
		return errors.New("RTP packet is nil")
	}
	owned := packet.Clone()
	// Extension IDs are negotiated per connection, not per source. Pion supplies
	// the recipient's SSRC and payload type; sequence/timestamp stay unchanged.
	owned.Extension, owned.ExtensionProfile, owned.Extensions = false, 0, nil
	return t.track.WriteRTP(owned)
}

func (t *conferenceRTPTrack) drainRTCP() {
	defer close(t.rtcpDone)
	for {
		if _, _, err := t.sender.ReadRTCP(); err != nil {
			return
		}
	}
}

func (t *conferenceRTPTrack) disable() {
	// Shutdown must reach Pion even while a packet write is blocked in I/O.
	t.closed.Store(true)
}

// OnRTPTrack supersedes OnAudioTrack for subsequently discovered tracks when set.
// Register before negotiation. Packets are owned by the receiver, never pooled;
// a full bounded queue drops incoming media rather than blocking other sources.
func (p *WebRTCPeer) OnRTPTrack(handler func(RTPTrackInfo, <-chan *rtp.Packet)) {
	p.mu.Lock()
	p.onRTPTrackHandler = handler
	p.mu.Unlock()
}

// AddRTPTrack adds an independent Opus sender with sourceID as its track ID.
// Duplicate IDs fail; removal permits re-adding the same ID with a new writer.
func (p *WebRTCPeer) AddRTPTrack(sourceID string) (RTPWriter, error) {
	p.rtpTrackMu.Lock()
	defer p.rtpTrackMu.Unlock()
	pc, err := p.rtpTrackConnection(sourceID)
	if err != nil {
		return nil, err
	}
	track, err := newConferenceRTPTrack(pc, sourceID)
	if err != nil {
		return nil, err
	}
	return p.registerRTPTrack(sourceID, track)
}

func (p *WebRTCPeer) rtpTrackConnection(sourceID string) (*webrtc.PeerConnection, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed || p.pc == nil {
		return nil, errors.New("peer connection not available")
	}
	if sourceID == "" || strings.ContainsFunc(sourceID, invalidSourceRune) {
		return nil, errors.New("source ID must be a nonempty SDP token")
	}
	if _, exists := p.rtpTracks[sourceID]; exists {
		return nil, errors.New("RTP source already exists")
	}
	return p.pc, nil
}

func invalidSourceRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r)
}

func newConferenceRTPTrack(pc *webrtc.PeerConnection, sourceID string) (*conferenceRTPTrack, error) {
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus,
		ClockRate: opusRTPClockRate, Channels: opusRTPChannels, SDPFmtpLine: opusFormatParameters}
	track, err := webrtc.NewTrackLocalStaticRTP(codec, sourceID, sourceID)
	if err != nil {
		return nil, err
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		return nil, err
	}
	out := &conferenceRTPTrack{track: track, sender: sender, rtcpDone: make(chan struct{})}
	go out.drainRTCP()
	return out, nil
}

func (p *WebRTCPeer) registerRTPTrack(sourceID string, track *conferenceRTPTrack) (RTPWriter, error) {
	p.mu.Lock()
	closed := p.closed
	if !closed {
		p.rtpTracks[sourceID] = track
	}
	p.mu.Unlock()
	if !closed {
		return track, nil
	}
	track.disable()
	err := track.sender.Stop()
	<-track.rtcpDone
	return nil, errors.Join(errors.New("peer connection closed while adding RTP track"), err)
}

// RemoveRTPTrack stops a sender and its RTCP reader. Missing IDs are a no-op.
// Previously returned writers reject writes after removal, including after re-add.
func (p *WebRTCPeer) RemoveRTPTrack(sourceID string) error {
	// Serialize source changes, but never make Close wait for this lock or I/O.
	p.rtpTrackMu.Lock()
	defer p.rtpTrackMu.Unlock()
	pc, track := p.detachRTPTrack(sourceID)
	if track == nil {
		return nil
	}
	err := pc.RemoveTrack(track.sender)
	if errors.Is(err, webrtc.ErrConnectionClosed) {
		err = nil
	}
	stopErr := track.sender.Stop() // Also stop if RemoveTrack lost the race with Close.
	<-track.rtcpDone
	return errors.Join(err, stopErr)
}

func (p *WebRTCPeer) detachRTPTrack(sourceID string) (*webrtc.PeerConnection, *conferenceRTPTrack) {
	p.mu.Lock()
	defer p.mu.Unlock()
	track := p.rtpTracks[sourceID]
	if track != nil {
		track.disable()
		delete(p.rtpTracks, sourceID)
	}
	return p.pc, track
}

func (p *WebRTCPeer) setupTrackHandler(pc *webrtc.PeerConnection) {
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		p.mu.RLock()
		rtpHandler, audioHandler, closed := p.onRTPTrackHandler, p.onAudioTrackHandler, p.closed
		p.mu.RUnlock()
		if closed {
			return
		}
		if rtpHandler != nil {
			deliverRTPTrack(track, rtpHandler)
		} else if audioHandler != nil {
			deliverAudioTrack(track, audioHandler)
		}
	})
}

func deliverRTPTrack(track *webrtc.TrackRemote, handler func(RTPTrackInfo, <-chan *rtp.Packet)) {
	ch := make(chan *rtp.Packet, mediaQueueCapacity)
	go func() {
		defer close(ch)
		readMediaTrack(track, func(packet *rtp.Packet) {
			select {
			case ch <- packet.Clone():
			default:
				metrics.FramesDroppedTotal.WithLabelValues("opus").Inc()
			}
		})
	}()
	handler(RTPTrackInfo{SourceID: track.ID(), SSRC: uint32(track.SSRC())}, ch)
}

func deliverAudioTrack(track *webrtc.TrackRemote, handler func(<-chan []byte)) {
	ch := make(chan []byte, mediaQueueCapacity)
	go func() {
		defer close(ch)
		readMediaTrack(track, func(packet *rtp.Packet) {
			select {
			case ch <- packet.Payload:
			default:
				metrics.FramesDroppedTotal.WithLabelValues("opus").Inc()
			}
		})
	}()
	handler(ch)
}

func readMediaTrack(track *webrtc.TrackRemote, deliver func(*rtp.Packet)) {
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		deliver(packet)
	}
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
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeOpus, ClockRate: opusRTPClockRate, SDPFmtpLine: opusFormatParameters,
	}, "audio", "echo")
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to create audio track").
			WithContext("sample_rate", sampleRate).WithContext("channels", channels)
	}
	return p.addAudioSampleTrack(track)
}

func (p *WebRTCPeer) addAudioSampleTrack(track *webrtc.TrackLocalStaticSample) (chan<- []byte, error) {
	if _, err := p.pc.AddTrack(track); err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrConnectionFailed, "Failed to add track to connection")
	}
	p.audioTrack = track
	p.audioTrackChan = make(chan []byte, mediaQueueCapacity)
	p.audioTrackDone = make(chan struct{})
	go writeAudioSamples(track, p.audioTrackChan, p.audioTrackDone)
	return p.audioTrackChan, nil
}

func writeAudioSamples(track *webrtc.TrackLocalStaticSample, frames <-chan []byte, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case data, ok := <-frames:
			if !ok {
				return
			}
			err := track.WriteSample(media.Sample{Data: data, Duration: opusFrameDuration})
			audio.PutOpusOutput(data)
			if err != nil {
				return
			}
		}
	}
}

// OnAudioTrack registers a handler for incoming audio tracks.
// The handler receives a channel of Opus-encoded audio frames.
func (p *WebRTCPeer) OnAudioTrack(handler func(inCh <-chan []byte)) {
	p.mu.Lock()
	p.onAudioTrackHandler = handler
	p.mu.Unlock()
}

type peerResources struct {
	pc     *webrtc.PeerConnection
	chat   *webrtc.DataChannel
	frames chan []byte
	done   chan struct{}
	tracks map[string]*conferenceRTPTrack
}

// Close terminates the peer connection and releases all resources.
func (p *WebRTCPeer) Close() error {
	resources := p.detachResources()
	resources.closeLegacyAudio()
	if resources.chat != nil {
		_ = resources.chat.Close()
	}
	if resources.pc == nil {
		return nil
	}
	err := resources.pc.Close()
	for _, track := range resources.tracks {
		<-track.rtcpDone
	}
	return err
}

func (p *WebRTCPeer) detachResources() peerResources {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return peerResources{}
	}
	p.closed = true
	resources := peerResources{p.pc, p.chatDC, p.audioTrackChan, p.audioTrackDone, p.rtpTracks}
	for _, track := range p.rtpTracks {
		track.disable()
	}
	for ch := range p.dataChannels {
		close(ch)
	}
	p.clearResources()
	return resources
}

// The caller holds p.mu; actual Pion shutdown happens outside the delivery guard.
func (p *WebRTCPeer) clearResources() {
	p.audioTrackDone, p.audioTrackChan = nil, nil
	p.controlDC, p.controlSendFn, p.controlMsgCh = nil, nil, nil
	p.chatDC, p.chatSendFn, p.chatMsgCh = nil, nil, nil
	p.pc, p.rtpTracks = nil, nil
	p.dataChannels = make(map[chan []byte]struct{})
}

func (r peerResources) closeLegacyAudio() {
	if r.done != nil {
		close(r.done)
	}
	if r.frames != nil {
		close(r.frames)
	}
}
