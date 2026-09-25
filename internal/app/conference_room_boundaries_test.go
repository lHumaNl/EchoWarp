package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func roomTestPeerRef(r *ConferenceRoom, id string) *conferencePeer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.peers[id]
}

func roomTestSource(r *ConferenceRoom, id string) conferenceSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return *r.sources[id]
}

func roomTestWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(roomTestTimeout):
		t.Fatal("operation did not complete")
	}
}

func TestConferenceRoomMediaQueueBoundsAndOwnership(t *testing.T) {
	r, p := roomTestServer(t)
	gate := roomTestBlockWrites(p)
	packet := roomTestRTP(1)
	r.WriteSource(ConferenceServerID, packet)
	roomTestWait(t, p.entered)
	packet.Payload[0] = 99
	packet.Timestamp = 99
	for range conferenceMediaQueue * 2 {
		r.WriteSource(ConferenceServerID, roomTestRTP(2))
	}
	require.Len(t, roomTestPeerRef(r, "alice").packets, conferenceMediaQueue)
	r.SetSourcePaused(ConferenceServerID, true)
	close(gate)
	got := roomTestReadPacket(t, p)
	require.Equal(t, roomTestRTP(1), got.packet)
	require.Eventually(t, func() bool { return len(roomTestPeerRef(r, "alice").packets) == 0 }, roomTestTimeout, time.Millisecond)
	require.Empty(t, p.written)
}

func TestConferenceRoomRejectsOversizedAndNilMedia(t *testing.T) {
	r, p := roomTestServer(t)
	r.WriteSource(ConferenceServerID, nil)
	r.WriteSource("unknown", roomTestRTP(1))
	r.WriteSource(ConferenceServerID, &rtp.Packet{Payload: make([]byte, conferenceMaxPacketBytes+1)})
	require.Empty(t, p.written)
	require.Empty(t, roomTestPeerRef(r, "alice").packets)
}

func TestConferenceRoomEndedSourceAndSameSSRCReplacement(t *testing.T) {
	r, a, b := roomTestPair(t)
	input := roomTestInput(a, "forged", 123)
	initial := roomTestSource(r, "alice")
	close(input)
	require.Eventually(t, func() bool { return roomTestSource(r, "alice").ended }, roomTestTimeout, time.Millisecond)
	require.False(t, roomTestAllowed(r, "alice", "bob"))
	replacement := roomTestInput(a, "forged", 123)
	require.Greater(t, roomTestSource(r, "alice").generation, initial.generation)
	replacement <- roomTestRTP(3)
	require.Equal(t, "alice", roomTestReadPacket(t, b).source)
}

func TestConferenceRoomSourceRestartInvalidatesQueuedMedia(t *testing.T) {
	r, a, b := roomTestPair(t)
	input := roomTestInput(a, "forged", 123)
	gate := roomTestBlockWrites(b)
	input <- roomTestRTP(1)
	roomTestWait(t, b.entered)
	r.WriteSource("alice", roomTestRTP(2))
	roomTestInput(a, "forged", 456)
	close(gate)
	require.Equal(t, uint16(1), roomTestReadPacket(t, b).packet.SequenceNumber)
	r.WriteSource("alice", roomTestRTP(3))
	require.Equal(t, uint16(3), roomTestReadPacket(t, b).packet.SequenceNumber)
	require.Empty(t, b.written)
}

func TestConferenceRoomServerGenerationAndGain(t *testing.T) {
	r, p := roomTestServer(t)
	r.WriteSource(ConferenceServerID, roomTestRTP(1))
	roomTestReadPacket(t, p)
	initial := roomTestSource(r, ConferenceServerID)
	packet := roomTestRTP(2)
	packet.SSRC++
	r.WriteSource(ConferenceServerID, packet)
	roomTestReadPacket(t, p)
	require.Greater(t, roomTestSource(r, ConferenceServerID).generation, initial.generation)
	roomTestGain(t, r)
}

func roomTestGain(t *testing.T, r *ConferenceRoom) {
	t.Helper()
	r.SetSourceGain(ConferenceServerID, 0.5)
	for _, gain := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		r.SetSourceGain(ConferenceServerID, gain)
	}
	state := r.sourceState(roomTestPeerRef(r, "alice"))
	require.Len(t, state.Sources, 2)
	require.False(t, state.Sources[0].Enabled)
	require.Equal(t, "alice", state.Sources[0].ID)
	require.Equal(t, float32(0.5), state.Sources[1].Gain)
	require.True(t, state.Sources[1].Enabled)
}

func TestConferenceRoomRejoinReplacesOnlyOwnTrack(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	a := addRoomTestPeer(t, r, "alice")
	old := addRoomTestPeer(t, r, "bob")
	roomTestHello(t, r, "alice", a)
	var first ConferenceDescription
	require.NoError(t, json.Unmarshal(roomTestReceive(t, a, ActionConferenceOffer), &first))
	initial := roomTestSource(r, "bob")
	previous := roomTestPeerRef(r, "bob")
	r.RemovePeer("bob")
	addRoomTestPeer(t, r, "bob")
	roomTestHandle(t, r, "alice", ActionConferenceAnswer, ConferenceDescription{ID: first.ID, SDP: "valid"})
	roomTestAnswer(t, r, "alice", a)
	require.Greater(t, roomTestSource(r, "bob").generation, initial.generation)
	roomTestInput(old, "forged", 789)
	require.False(t, roomTestSource(r, "bob").seen)
	request := conferenceRouteRequest{ID: 1}
	require.Error(t, r.applyRouteRequest(previous, ActionConferenceRoutes, request))
	snapshot := roomTestSnapshot(a)
	require.Equal(t, 1, snapshot.removes)
	require.Equal(t, 3, snapshot.adds)
}

func TestConferenceRoomNoUpstreamBeforeCompatibleAck(t *testing.T) {
	r := newRoomTest(t)
	a := addRoomTestPeer(t, r, "alice")
	b := addRoomTestPeer(t, r, "bob")
	roomTestStart(t, r, "bob", b)
	input := roomTestInput(a, "bob", 123)
	input <- roomTestRTP(1)
	require.Eventually(t, func() bool { return len(input) == 0 }, roomTestTimeout, time.Millisecond)
	require.Empty(t, b.written)
	state := r.sourceState(roomTestPeerRef(r, "bob"))
	require.False(t, state.Sources[0].Enabled)
	roomTestStart(t, r, "alice", a)
	input <- roomTestRTP(2)
	require.Equal(t, uint16(2), roomTestReadPacket(t, b).packet.SequenceNumber)
}

func TestConferenceRoomBlockedSignalDeadline(t *testing.T) {
	r := newRoomTest(t)
	p := addRoomTestPeer(t, r, "alice")
	p.mu.Lock()
	p.sendGate = make(chan struct{})
	p.mu.Unlock()
	r.Ready("alice")
	require.Eventually(t, func() bool { r.mu.RLock(); defer r.mu.RUnlock(); return !r.peers["alice"].ioDeadline.IsZero() }, roomTestTimeout, time.Millisecond)
	r.mu.Lock()
	r.peers["alice"].ioDeadline = time.Now().Add(-time.Second)
	r.mu.Unlock()
	roomTestClosed(t, p)
}

func TestConferenceRoomContextCancellation(t *testing.T) {
	r := newRoomTest(t)
	p := newRoomTestPeer()
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, r.AddPeer(ctx, "alice", p))
	cancel()
	roomTestClosed(t, p)
	require.Empty(t, r.State(ConferenceServerID, true).Rules)
	require.Nil(t, roomTestPeerRef(r, "alice"))
}

func TestConferenceRoomUnknownAndMalformedControls(t *testing.T) {
	r := newRoomTest(t)
	addRoomTestPeer(t, r, "alice")
	handled, err := r.HandleControl("alice", "legacy", nil)
	require.False(t, handled)
	require.NoError(t, err)
	handled, err = r.HandleControl("absent", ActionConferenceAnswer, json.RawMessage(`{"id":1,"sdp":"x"}`))
	require.True(t, handled)
	require.Error(t, err)
	for _, data := range []string{`{}`, `{"id":1}`, `{"id":0,"sdp":"x"}`, `{"id":1,"sdp":" "}`, `null`} {
		_, err = r.HandleControl("alice", ActionConferenceAnswer, json.RawMessage(data))
		require.Error(t, err)
	}
}

func TestConferenceRoomCapacity(t *testing.T) {
	r := newRoomTest(t)
	for i := range conferenceMaxSources - 1 {
		addRoomTestPeer(t, r, fmt.Sprintf("peer-%d", i))
	}
	require.Error(t, r.AddPeer(context.Background(), "overflow", newRoomTestPeer()))
	r.AddServerSource()
	r.mu.RLock()
	count := len(r.sources)
	r.mu.RUnlock()
	require.Equal(t, conferenceMaxSources, count)
}

func TestConferenceRoomLeaveDuringWriteKeepsRecipient(t *testing.T) {
	r := newRoomTest(t)
	r.AddServerSource()
	a := addRoomTestPeer(t, r, "alice")
	b := addRoomTestPeer(t, r, "bob")
	roomTestStart(t, r, "alice", a)
	roomTestStart(t, r, "bob", b)
	gate := roomTestBlockWrites(a)
	r.WriteSource("bob", roomTestRTP(1))
	roomTestWait(t, a.entered)
	r.RemovePeer("bob")
	roomTestAnswer(t, r, "alice", a)
	close(gate)
	r.WriteSource(ConferenceServerID, roomTestRTP(2))
	require.Equal(t, ConferenceServerID, roomTestReadPacket(t, a).source)
	require.Empty(t, a.closed)
}
