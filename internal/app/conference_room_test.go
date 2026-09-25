package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

const roomTestTimeout = 3 * time.Second

type roomTestControl struct {
	action string
	data   json.RawMessage
}
type roomTestPacket struct {
	source string
	packet *rtp.Packet
}

type roomTestPeer struct {
	transport.PeerManager
	mu                             sync.Mutex
	onTrack                        func(transport.RTPTrackInfo, <-chan *rtp.Packet)
	controls                       chan roomTestControl
	written                        chan roomTestPacket
	entered                        chan struct{}
	closed                         chan struct{}
	closeOnce                      sync.Once
	writeGate, sendGate            <-chan struct{}
	sendError                      bool
	writers                        map[string]*roomTestWriter
	adds, removes, offers, answers int
	pending                        bool
}

type roomTestWriter struct {
	peer    *roomTestPeer
	source  string
	removed atomic.Bool
}

type roomTestPeerSnapshot struct {
	adds, removes int
	writers       []string
}

func roomTestSnapshot(p *roomTestPeer) roomTestPeerSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := roomTestPeerSnapshot{adds: p.adds, removes: p.removes}
	for id := range p.writers {
		snapshot.writers = append(snapshot.writers, id)
	}
	return snapshot
}

func newRoomTestPeer() *roomTestPeer {
	return &roomTestPeer{controls: make(chan roomTestControl, 2048), written: make(chan roomTestPacket, 2048),
		entered: make(chan struct{}, 2048), closed: make(chan struct{}), writers: make(map[string]*roomTestWriter)}
}

func (p *roomTestPeer) OnRTPTrack(fn func(transport.RTPTrackInfo, <-chan *rtp.Packet)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTrack = fn
}

func (p *roomTestPeer) AddRTPTrack(id string) (transport.RTPWriter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending || p.writers[id] != nil {
		return nil, errors.New("track added during pending offer or duplicate track")
	}
	w := &roomTestWriter{peer: p, source: id}
	p.writers[id] = w
	p.adds++
	return w, nil
}

func (p *roomTestPeer) RemoveRTPTrack(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending {
		return errors.New("track removed during pending offer")
	}
	if writer := p.writers[id]; writer != nil {
		writer.removed.Store(true)
	}
	delete(p.writers, id)
	p.removes++
	return nil
}

func (p *roomTestPeer) CreateOffer() (webrtc.SessionDescription, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending {
		return webrtc.SessionDescription{}, errors.New("overlapping offer")
	}
	p.pending = true
	p.offers++
	return webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: "test-offer"}, nil
}

func (p *roomTestPeer) SetRemoteDescription(desc webrtc.SessionDescription) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.pending || desc.Type != webrtc.SDPTypeAnswer {
		return errors.New("unsolicited answer")
	}
	p.pending = false
	p.answers++
	return nil
}

func (p *roomTestPeer) SendControl(action string, payload any) error {
	p.mu.Lock()
	gate, fail := p.sendGate, p.sendError
	p.mu.Unlock()
	if fail {
		return errors.New("injected control failure")
	}
	if err := roomTestAwaitGate(gate, p.closed); err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	select {
	case p.controls <- roomTestControl{action, data}:
		return nil
	case <-p.closed:
		return errors.New("closed")
	}
}

func (p *roomTestPeer) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func (w *roomTestWriter) WriteRTP(packet *rtp.Packet) error {
	p := w.peer
	p.mu.Lock()
	gate := p.writeGate
	p.mu.Unlock()
	select {
	case p.entered <- struct{}{}:
	default:
	}
	if err := roomTestAwaitGate(gate, p.closed); err != nil {
		return err
	}
	if w.removed.Load() {
		return errors.New("removed writer")
	}
	select {
	case p.written <- roomTestPacket{w.source, packet.Clone()}:
		return nil
	case <-p.closed:
		return errors.New("closed")
	}
}

func newRoomTest(t *testing.T) *ConferenceRoom {
	t.Helper()
	r := NewConferenceRoom(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(r.Close)
	return r
}

func addRoomTestPeer(t *testing.T, r *ConferenceRoom, id string) *roomTestPeer {
	t.Helper()
	p := newRoomTestPeer()
	require.NoError(t, r.AddPeer(context.Background(), id, p))
	return p
}

func roomTestReceive(t *testing.T, p *roomTestPeer, action string) json.RawMessage {
	t.Helper()
	timer := time.NewTimer(roomTestTimeout)
	defer timer.Stop()
	for {
		select {
		case message := <-p.controls:
			if message.action == action {
				return message.data
			}
		case <-timer.C:
			t.Fatalf("missing control %s", action)
			return nil
		}
	}
}

func roomTestHandle(t *testing.T, r *ConferenceRoom, id, action string, payload any) {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	handled, err := r.HandleControl(id, action, data)
	require.True(t, handled)
	require.NoError(t, err)
}

func roomTestHello(t *testing.T, r *ConferenceRoom, id string, p *roomTestPeer) {
	t.Helper()
	r.Ready(id)
	var hello ConferenceHello
	require.NoError(t, json.Unmarshal(roomTestReceive(t, p, ActionConferenceHello), &hello))
	require.Equal(t, ConferenceHello{Version: ConferenceVersion, SelfID: id}, hello)
	roomTestHandle(t, r, id, ActionConferenceHelloAck, map[string]int{"version": ConferenceVersion})
	require.Eventually(t, func() bool { r.mu.RLock(); defer r.mu.RUnlock(); return r.peers[id] != nil && r.peers[id].compatible }, roomTestTimeout, time.Millisecond)
}

func roomTestAnswer(t *testing.T, r *ConferenceRoom, id string, p *roomTestPeer) ConferenceDescription {
	t.Helper()
	var offer ConferenceDescription
	require.NoError(t, json.Unmarshal(roomTestReceive(t, p, ActionConferenceOffer), &offer))
	roomTestHandle(t, r, id, ActionConferenceAnswer, ConferenceDescription{ID: offer.ID, SDP: "test-answer"})
	require.Eventually(t, func() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.answers >= int(offer.ID) }, roomTestTimeout, time.Millisecond)
	return offer
}

func roomTestStart(t *testing.T, r *ConferenceRoom, id string, p *roomTestPeer) {
	t.Helper()
	roomTestHello(t, r, id, p)
	roomTestAnswer(t, r, id, p)
	require.Eventually(t, func() bool {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, writer := range r.peers[id].writers {
			if !writer.negotiated {
				return false
			}
		}
		return true
	}, roomTestTimeout, time.Millisecond)
}

func roomTestReadPacket(t *testing.T, p *roomTestPeer) roomTestPacket {
	t.Helper()
	select {
	case packet := <-p.written:
		return packet
	case <-time.After(roomTestTimeout):
		t.Fatal("missing RTP packet")
		return roomTestPacket{}
	}
}

func roomTestClosed(t *testing.T, p *roomTestPeer) {
	t.Helper()
	select {
	case <-p.closed:
	case <-time.After(roomTestTimeout):
		t.Fatal("peer not closed")
	}
}

func roomTestInput(p *roomTestPeer, source string, ssrc uint32) chan *rtp.Packet {
	input := make(chan *rtp.Packet, conferenceMediaQueue)
	p.mu.Lock()
	handler := p.onTrack
	p.mu.Unlock()
	handler(transport.RTPTrackInfo{SourceID: source, SSRC: ssrc}, input)
	return input
}

func roomTestRTP(sequence uint16) *rtp.Packet {
	return &rtp.Packet{Header: rtp.Header{Version: 2, SSRC: 123, SequenceNumber: sequence, Timestamp: uint32(sequence) * 960}, Payload: []byte{1, 2, 3}}
}

func TestConferenceRoomReadyAndCompatibility(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	p := addRoomTestPeer(t, r, "alice")
	r.SetSourcePaused(ConferenceServerID, true)
	require.Never(t, func() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.adds != 0 }, 30*time.Millisecond, time.Millisecond)
	r.Ready("alice")
	roomTestReceive(t, p, ActionConferenceHello)
	require.Zero(t, roomTestSnapshot(p).adds)
	_, err := r.HandleControl("alice", ActionConferenceHelloAck, json.RawMessage(`{"version":2}`))
	require.Error(t, err)
	roomTestClosed(t, p)
}

func TestConferenceRoomSourceIdentityAndRTP(t *testing.T) {
	r, a, b := roomTestPair(t)
	input := roomTestInput(a, "bob", 123)
	for _, seq := range []uint16{1, 7, 8} {
		packet := roomTestRTP(seq)
		input <- packet
		got := roomTestReadPacket(t, b)
		require.Equal(t, "alice", got.source)
		require.Equal(t, packet, got.packet)
	}
	require.Empty(t, a.written)
	roomTestRestart(t, r, a, b)
}

func roomTestRestart(t *testing.T, r *ConferenceRoom, a, b *roomTestPeer) {
	t.Helper()
	before := roomTestSource(r, "alice").generation
	other := roomTestSource(r, "bob").generation
	input := roomTestInput(a, ConferenceServerID, 456)
	input <- roomTestRTP(20)
	require.Equal(t, "alice", roomTestReadPacket(t, b).source)
	require.Greater(t, roomTestSource(r, "alice").generation, before)
	require.Equal(t, other, roomTestSource(r, "bob").generation)
	snapshot := roomTestSnapshot(b)
	require.Equal(t, 1, snapshot.adds)
	require.Zero(t, snapshot.removes)
}

func TestConferenceRoomSerializedNegotiation(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	p := addRoomTestPeer(t, r, "alice")
	roomTestHello(t, r, "alice", p)
	var first ConferenceDescription
	require.NoError(t, json.Unmarshal(roomTestReceive(t, p, ActionConferenceOffer), &first))
	addRoomTestPeer(t, r, "bob")
	addRoomTestPeer(t, r, "carol")
	r.RemovePeer("bob")
	roomTestHandle(t, r, "alice", ActionConferenceAnswer, ConferenceDescription{ID: first.ID + 1, SDP: "unsolicited"})
	r.SetSourcePaused(ConferenceServerID, true)
	require.Never(t, func() bool { p.mu.Lock(); defer p.mu.Unlock(); return p.offers != 1 || p.adds != 1 || p.answers != 0 }, 30*time.Millisecond, time.Millisecond)
	roomTestHandle(t, r, "alice", ActionConferenceAnswer, ConferenceDescription{ID: first.ID, SDP: "valid"})
	second := roomTestAnswer(t, r, "alice", p)
	require.Equal(t, first.ID+1, second.ID)
	snapshot := roomTestSnapshot(p)
	require.Contains(t, snapshot.writers, "carol")
	require.NotContains(t, snapshot.writers, "bob")
}

func TestConferenceRoomSignalingTimeouts(t *testing.T) {
	for _, negotiate := range []bool{false, true} {
		t.Run(map[bool]string{false: "hello", true: "offer"}[negotiate], func(t *testing.T) {
			r := newRoomTest(t)
			r.AddServerSource()
			p := addRoomTestPeer(t, r, "alice")
			if negotiate {
				roomTestHello(t, r, "alice", p)
				roomTestReceive(t, p, ActionConferenceOffer)
			} else {
				r.Ready("alice")
				roomTestReceive(t, p, ActionConferenceHello)
			}
			r.mu.Lock()
			r.peers["alice"].deadline = time.Now().Add(-time.Second)
			r.mu.Unlock()
			roomTestClosed(t, p)
		})
	}
}

func TestConferenceRoomSendFailure(t *testing.T) {
	r := newRoomTest(t)
	p := addRoomTestPeer(t, r, "alice")
	p.mu.Lock()
	p.sendError = true
	p.mu.Unlock()
	r.Ready("alice")
	roomTestClosed(t, p)
}

func TestConferenceRoomControlQueueBounded(t *testing.T) {
	r := newRoomTest(t)
	p := addRoomTestPeer(t, r, "alice")
	p.mu.Lock()
	p.sendGate = make(chan struct{})
	p.mu.Unlock()
	r.Ready("alice")
	var err error
	for range conferenceControlQueue + 2 {
		_, err = r.HandleControl("alice", ActionConferenceRoutes, json.RawMessage(`{"id":1}`))
	}
	require.Error(t, err)
	roomTestClosed(t, p)
}

func TestConferenceRoomCancellationUnblocksIO(t *testing.T) {
	r, p := roomTestServer(t)
	roomTestBlockWrites(p)
	r.WriteSource(ConferenceServerID, roomTestRTP(1))
	roomTestWait(t, p.entered)
	done := make(chan struct{})
	go func() { r.Close(); close(done) }()
	roomTestWait(t, done)
}
