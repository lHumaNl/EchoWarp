package transport

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const conferenceTestTimeout = 10 * time.Second

func newMediaPeer(t *testing.T) *WebRTCPeer {
	t.Helper()
	p := NewWebRTCPeer(DirectionSend)
	require.NoError(t, p.CreatePeerConnection(defaultICEConfig()))
	t.Cleanup(func() { require.NoError(t, p.Close()) })
	return p
}

func TestConferenceMediaTrackLifecycle(t *testing.T) {
	p := newMediaPeer(t)
	assert.Implements(t, (*ConferenceMedia)(nil), p)
	for range 10 {
		writer, err := p.AddRTPTrack("source")
		require.NoError(t, err)
		require.Equal(t, webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus,
			ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"}, writerCodec(t, p.rtpTracks["source"]))
		_, err = p.AddRTPTrack("source")
		require.Error(t, err)
		require.NoError(t, p.RemoveRTPTrack("source"))
		require.Error(t, writer.WriteRTP(mediaPacket(1, 960)))
	}
	require.NoError(t, p.RemoveRTPTrack("source"))
	require.Empty(t, p.pc.GetSenders())
}

func writerCodec(t *testing.T, track *conferenceRTPTrack) webrtc.RTPCodecCapability {
	t.Helper()
	local, ok := track.sender.Track().(*webrtc.TrackLocalStaticRTP)
	require.True(t, ok)
	return local.Codec()
}

func TestConferenceMediaValidation(t *testing.T) {
	p := NewWebRTCPeer(DirectionSend)
	_, err := p.AddRTPTrack("source")
	require.Error(t, err)
	require.NoError(t, p.CreatePeerConnection(defaultICEConfig()))
	t.Cleanup(func() { p.Close() })
	for _, source := range []string{"", " ", "bad\nsource", "bad source"} {
		_, err = p.AddRTPTrack(source)
		require.Error(t, err)
	}
	writer, err := p.AddRTPTrack("source")
	require.NoError(t, err)
	require.Error(t, writer.WriteRTP(nil))
	require.NoError(t, p.Close())
	require.Error(t, writer.WriteRTP(mediaPacket(1, 960)))
	_, err = p.AddRTPTrack("late")
	require.Error(t, err)
	require.NoError(t, p.RemoveRTPTrack("source"))
}

func mediaPacket(sequence uint16, timestamp uint32) *rtp.Packet {
	return &rtp.Packet{Header: rtp.Header{
		Version: 2, PayloadType: 127, SSRC: 1234,
		SequenceNumber: sequence, Timestamp: timestamp, Marker: true,
	}, Payload: []byte{0xf8, 0xff, 0xfe}}
}

func TestConferenceMediaCloseConcurrent(t *testing.T) {
	p := newMediaPeer(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			source := fmt.Sprintf("source-%d", i)
			writer, err := p.AddRTPTrack(source)
			if err == nil {
				_ = writer.WriteRTP(mediaPacket(1, 960))
			}
			p.OnRTPTrack(nil)
			_ = p.RemoveRTPTrack(source)
		})
	}
	require.NoError(t, p.Close())
	wg.Wait()
}

type receivedMediaTrack struct {
	info    RTPTrackInfo
	packets <-chan *rtp.Packet
}

func collectMediaTracks(peer *WebRTCPeer) <-chan receivedMediaTrack {
	tracks := make(chan receivedMediaTrack, 8)
	peer.OnRTPTrack(func(info RTPTrackInfo, packets <-chan *rtp.Packet) {
		tracks <- receivedMediaTrack{info: info, packets: packets}
	})
	return tracks
}

func awaitMediaTrack(t *testing.T, tracks <-chan receivedMediaTrack) receivedMediaTrack {
	t.Helper()
	select {
	case track := <-tracks:
		return track
	case <-time.After(conferenceTestTimeout):
		t.Fatal("timed out waiting for RTP track")
		return receivedMediaTrack{}
	}
}

func awaitMediaPacket(t *testing.T, packets <-chan *rtp.Packet) *rtp.Packet {
	t.Helper()
	select {
	case packet, ok := <-packets:
		require.True(t, ok, "RTP channel closed unexpectedly")
		return packet
	case <-time.After(conferenceTestTimeout):
		t.Fatal("timed out waiting for RTP packet")
		return nil
	}
}

func negotiateMedia(t *testing.T, sender, receiver *WebRTCPeer) {
	t.Helper()
	senderGathered := webrtc.GatheringCompletePromise(sender.pc)
	_, err := sender.CreateOffer()
	require.NoError(t, err)
	waitCh(t, senderGathered, "offer gathering")
	receiverGathered := webrtc.GatheringCompletePromise(receiver.pc)
	_, err = receiver.CreateAnswer(*sender.pc.LocalDescription())
	require.NoError(t, err)
	waitCh(t, receiverGathered, "answer gathering")
	require.NoError(t, sender.SetRemoteDescription(*receiver.pc.LocalDescription()))
	require.Eventually(t, func() bool {
		return sender.pc.ConnectionState() == webrtc.PeerConnectionStateConnected &&
			receiver.pc.ConnectionState() == webrtc.PeerConnectionStateConnected
	}, conferenceTestTimeout, time.Millisecond)
}

func TestConferenceMediaLoopbackMultipleTracks(t *testing.T) {
	sender, receiver := newMediaPeer(t), newMediaPeer(t)
	tracks := collectMediaTracks(receiver)
	legacy := make(chan struct{}, 1)
	receiver.OnAudioTrack(func(<-chan []byte) { legacy <- struct{}{} })
	first, err := sender.AddRTPTrack("first")
	require.NoError(t, err)
	second, err := sender.AddRTPTrack("second")
	require.NoError(t, err)
	negotiateMedia(t, sender, receiver)
	firstTrack := checkMediaDelivery(t, first, "first", tracks)
	secondTrack := checkMediaDelivery(t, second, "second", tracks)
	require.NotEqual(t, firstTrack.info.SSRC, secondTrack.info.SSRC)
	require.Empty(t, legacy, "RTP handler must supersede legacy handler")
	require.NoError(t, receiver.Close())
	assertMediaClosed(t, firstTrack.packets)
	assertMediaClosed(t, secondTrack.packets)
}

func checkMediaDelivery(t *testing.T, writer RTPWriter, source string, tracks <-chan receivedMediaTrack) receivedMediaTrack {
	t.Helper()
	packet := mediaPacket(65534, 0xfffff000)
	require.NoError(t, packet.SetExtension(1, []byte("foreign-mid")))
	original := packet.Clone()
	require.NoError(t, writer.WriteRTP(packet))
	require.Equal(t, original, packet, "writer mutated caller packet")
	track := awaitMediaTrack(t, tracks)
	require.Equal(t, source, track.info.SourceID)
	checkPacketIdentity(t, packet, awaitMediaPacket(t, track.packets), track.info.SSRC)
	packet = mediaPacket(2, 960) // Preserve wraparound and silence/loss gaps.
	require.NoError(t, writer.WriteRTP(packet))
	checkPacketIdentity(t, packet, awaitMediaPacket(t, track.packets), track.info.SSRC)
	return track
}

func checkPacketIdentity(t *testing.T, want, got *rtp.Packet, ssrc uint32) {
	t.Helper()
	require.Equal(t, want.Payload, got.Payload)
	require.Equal(t, want.SequenceNumber, got.SequenceNumber)
	require.Equal(t, want.Timestamp, got.Timestamp)
	require.Equal(t, want.Marker, got.Marker)
	require.Equal(t, ssrc, got.SSRC)
	require.NotEqual(t, want.PayloadType, got.PayloadType, "use negotiated payload type")
	require.Empty(t, got.Extensions)
	require.False(t, got.Extension)
}

func assertMediaClosed(t *testing.T, packets <-chan *rtp.Packet) {
	t.Helper()
	select {
	case _, open := <-packets:
		require.False(t, open)
	case <-time.After(conferenceTestTimeout):
		t.Fatal("RTP channel remained open after Close")
	}
}

func TestControlOverflowClosesPeer(t *testing.T) {
	p := newMediaPeer(t)
	pc := p.pc
	queue := make(chan []byte, 1)
	p.dataChannels[queue] = struct{}{}
	queue <- []byte("queued")
	p.handleDataChannelMessage(LabelControl, queue, webrtc.DataChannelMessage{Data: []byte("overflow")})
	require.Eventually(t, func() bool {
		return pc.ConnectionState() == webrtc.PeerConnectionStateClosed
	}, conferenceTestTimeout, time.Millisecond, "control overflow must close without deadlock")
	require.Equal(t, []byte("queued"), <-queue)
	_, open := <-queue
	require.False(t, open)
}

func TestChatOverflowDoesNotClosePeer(t *testing.T) {
	p := newMediaPeer(t)
	queue := make(chan []byte, 1)
	queue <- []byte("queued")
	p.handleDataChannelMessage(LabelChat, queue, webrtc.DataChannelMessage{Data: []byte("overflow")})
	require.Equal(t, webrtc.PeerConnectionStateNew, p.pc.ConnectionState())
	require.Equal(t, []byte("queued"), <-queue)
}

func TestConferenceMediaRenegotiatedRejoin(t *testing.T) {
	sender, receiver := newMediaPeer(t), newMediaPeer(t)
	tracks := collectMediaTracks(receiver)
	stable, err := sender.AddRTPTrack("stable")
	require.NoError(t, err)
	negotiateMedia(t, sender, receiver)
	stableTrack := checkMediaDelivery(t, stable, "stable", tracks)
	var previousSSRC uint32
	for generation := range 3 {
		previousSSRC = rejoinMediaSource(t, sender, receiver, tracks, previousSSRC)
		packet := mediaPacket(uint16(generation+3), uint32(generation+2)*960)
		require.NoError(t, stable.WriteRTP(packet))
		checkPacketIdentity(t, packet, awaitMediaPacket(t, stableTrack.packets), stableTrack.info.SSRC)
	}
}

func rejoinMediaSource(t *testing.T, sender, receiver *WebRTCPeer, tracks <-chan receivedMediaTrack, previousSSRC uint32) uint32 {
	t.Helper()
	writer, err := sender.AddRTPTrack("rejoining")
	require.NoError(t, err)
	negotiateMedia(t, sender, receiver)
	track := checkMediaDelivery(t, writer, "rejoining", tracks)
	require.NotEqual(t, previousSSRC, track.info.SSRC)
	rtcpDone := sender.rtpTracks["rejoining"].rtcpDone
	require.NoError(t, sender.RemoveRTPTrack("rejoining"))
	waitCh(t, rtcpDone, "removed sender RTCP reader")
	require.Error(t, writer.WriteRTP(mediaPacket(3, 1920)))
	negotiateMedia(t, sender, receiver)
	assertMediaClosed(t, track.packets)
	return track.info.SSRC
}

func TestConferenceMediaForwardingLoopback(t *testing.T) {
	source, ingress := newMediaPeer(t), newMediaPeer(t)
	egress, recipient := newMediaPeer(t), newMediaPeer(t)
	inTracks, outTracks := collectMediaTracks(ingress), collectMediaTracks(recipient)
	input, err := source.AddRTPTrack("untrusted-source")
	require.NoError(t, err)
	output, err := egress.AddRTPTrack("authenticated-source")
	require.NoError(t, err)
	negotiateMedia(t, source, ingress)
	negotiateMedia(t, egress, recipient)
	packet := mediaPacket(500, 48000)
	require.NoError(t, input.WriteRTP(packet))
	in := awaitMediaTrack(t, inTracks)
	relayed := awaitMediaPacket(t, in.packets)
	require.NoError(t, output.WriteRTP(relayed))
	out := awaitMediaTrack(t, outTracks)
	require.Equal(t, "authenticated-source", out.info.SourceID)
	checkPacketIdentity(t, packet, awaitMediaPacket(t, out.packets), out.info.SSRC)
	checkForwardedGap(t, input, output, in, out)
}

func checkForwardedGap(t *testing.T, input, output RTPWriter, in, out receivedMediaTrack) {
	t.Helper()
	packet := mediaPacket(550, 96000)
	require.NoError(t, input.WriteRTP(packet))
	relayed := awaitMediaPacket(t, in.packets)
	original := relayed.Clone()
	require.NoError(t, output.WriteRTP(relayed))
	require.Equal(t, original, relayed)
	checkPacketIdentity(t, packet, awaitMediaPacket(t, out.packets), out.info.SSRC)
}

func TestConferenceMediaBoundedQueueOwnership(t *testing.T) {
	sender, receiver := newMediaPeer(t), newMediaPeer(t)
	tracks := collectMediaTracks(receiver)
	writer, err := sender.AddRTPTrack("source")
	require.NoError(t, err)
	negotiateMedia(t, sender, receiver)
	packet := mediaPacket(1, 960)
	require.NoError(t, writer.WriteRTP(packet))
	track := awaitMediaTrack(t, tracks)
	owned := awaitMediaPacket(t, track.packets)
	require.Equal(t, mediaQueueCapacity, cap(track.packets))
	floodMediaQueue(t, writer, track.packets)
	checkPacketIdentity(t, packet, owned, track.info.SSRC)
	owned.Payload[0] ^= 0xff
	require.Equal(t, packet.Payload, awaitMediaPacket(t, track.packets).Payload)
	require.NoError(t, receiver.Close())
}

func floodMediaQueue(t *testing.T, writer RTPWriter, packets <-chan *rtp.Packet) {
	t.Helper()
	sequence := uint16(2)
	require.Eventually(t, func() bool {
		for range mediaQueueCapacity {
			require.NoError(t, writer.WriteRTP(mediaPacket(sequence, uint32(sequence)*960)))
			sequence++
		}
		return len(packets) == cap(packets)
	}, conferenceTestTimeout, time.Millisecond)
}

func TestConferenceMediaConnectedCloseConcurrent(t *testing.T) {
	sender, receiver := newMediaPeer(t), newMediaPeer(t)
	tracks := collectMediaTracks(receiver)
	writer, err := sender.AddRTPTrack("source")
	require.NoError(t, err)
	negotiateMedia(t, sender, receiver)
	track := checkMediaDelivery(t, writer, "source", tracks)
	rtcpDone := sender.rtpTracks["source"].rtcpDone
	var wg sync.WaitGroup
	wg.Go(func() { _ = sender.RemoveRTPTrack("source") })
	wg.Go(func() { _ = writer.WriteRTP(mediaPacket(3, 1920)) })
	require.NoError(t, sender.Close())
	wg.Wait()
	waitCh(t, rtcpDone, "closed sender RTCP reader")
	require.Error(t, writer.WriteRTP(mediaPacket(4, 2880)))
	require.NoError(t, receiver.Close())
	for range track.packets {
	}
}

func TestControlOverflowLoopback(t *testing.T) {
	sender, receiver := newMediaPeer(t), newMediaPeer(t)
	pc := receiver.pc
	require.NoError(t, sender.CreateControlDataChannel())
	negotiateMedia(t, sender, receiver)
	waitDCReady(t, sender, receiver)
	require.Eventually(t, func() bool { return receiver.ControlMessages() != nil },
		conferenceTestTimeout, time.Millisecond)
	queue := receiver.ControlMessages()
	require.NotNil(t, queue)
	for range cap(queue) + 1 {
		require.NoError(t, sender.SendControl("queued-control", nil))
	}
	require.Eventually(t, func() bool {
		return pc.ConnectionState() == webrtc.PeerConnectionStateClosed
	}, conferenceTestTimeout, time.Millisecond)
}

func TestDCReadyBypassesFullControlQueue(t *testing.T) {
	p := newMediaPeer(t)
	queue := make(chan []byte, 1)
	queue <- []byte("queued")
	p.handleDataChannelMessage(LabelControl, queue, webrtc.DataChannelMessage{
		Data: []byte(`{"type":"control","payload":{"action":"dc_ready"}}`),
	})
	waitCh(t, p.DCReady(), "dc_ready with full queue")
	require.Equal(t, webrtc.PeerConnectionStateNew, p.pc.ConnectionState())
}

func TestConferenceNegotiationConcurrentClose(t *testing.T) {
	sender := newMediaPeer(t)
	_, err := sender.AddRTPTrack("source")
	require.NoError(t, err)
	offer, err := sender.CreateOffer()
	require.NoError(t, err)
	for _, operation := range []func(*WebRTCPeer){
		func(p *WebRTCPeer) { _, _ = p.CreateOffer() },
		func(p *WebRTCPeer) { _, _ = p.CreateAnswer(offer) },
		func(p *WebRTCPeer) { _ = p.SetRemoteDescription(offer) },
		func(p *WebRTCPeer) { _ = p.CreateControlDataChannel() },
		func(p *WebRTCPeer) { _ = p.AddICECandidate(webrtc.ICECandidateInit{}) },
		func(p *WebRTCPeer) { p.applyBufferedCandidates(); _ = p.AddICECandidate(webrtc.ICECandidateInit{}) },
	} {
		peer := newMediaPeer(t)
		require.NoError(t, peer.AddICECandidate(webrtc.ICECandidateInit{}))
		raceSignalingShutdown(t, peer, operation)
	}
}

func raceSignalingShutdown(t *testing.T, peer *WebRTCPeer, operation func(*WebRTCPeer)) {
	t.Helper()
	started, finished, callback := make(chan struct{}), make(chan struct{}), make(chan struct{})
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		peer.OnRTPTrack(nil) // Re-enter the peer lock from a Pion callback.
		if state == webrtc.PeerConnectionStateClosed {
			close(callback)
		}
	})
	go func() { defer close(finished); close(started); operation(peer) }()
	waitCh(t, started, "signaling entry")
	require.NoError(t, peer.Close())
	waitCh(t, finished, "signaling during shutdown")
	waitCh(t, callback, "reentrant shutdown callback")
}

type blockedRTPWriter struct {
	started  chan struct{}
	stopped  <-chan struct{}
	fallback <-chan struct{}
}

func (w blockedRTPWriter) WriteRTP(*rtp.Packet) error {
	close(w.started)
	select {
	case <-w.stopped:
	case <-w.fallback:
	}
	return io.ErrClosedPipe
}

func TestConferenceBlockedWriterShutdown(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprintf("remove=%t", remove), func(t *testing.T) {
			peer := newMediaPeer(t)
			_, err := peer.AddRTPTrack("blocked")
			require.NoError(t, err)
			track := peer.rtpTracks["blocked"]
			startBlockedWriter(t, track)
			stopWithBlockedWriter(t, peer, remove)
			waitCh(t, track.rtcpDone, "RTCP reader shutdown")
			require.Error(t, track.WriteRTP(mediaPacket(2, 1920)))
		})
	}
}

func startBlockedWriter(t *testing.T, track *conferenceRTPTrack) {
	t.Helper()
	started, fallback, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	track.track = blockedRTPWriter{started: started, stopped: track.rtcpDone, fallback: fallback}
	t.Cleanup(func() { close(fallback); waitCh(t, finished, "blocked writer cleanup") })
	go func() {
		defer close(finished)
		assert.ErrorIs(t, track.WriteRTP(mediaPacket(1, 960)), io.ErrClosedPipe)
	}()
	waitCh(t, started, "blocked writer entry")
}

func stopWithBlockedWriter(t *testing.T, peer *WebRTCPeer, remove bool) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if remove {
			assert.NoError(t, peer.RemoveRTPTrack("blocked"))
		} else {
			assert.NoError(t, peer.Close())
		}
	}()
	waitCh(t, done, "shutdown while WriteRTP is blocked")
}

func TestConferenceTrackRegistrationAfterClose(t *testing.T) {
	peer := newMediaPeer(t)
	track, err := newConferenceRTPTrack(peer.pc, "late")
	require.NoError(t, err)
	require.NoError(t, peer.Close())
	writer, err := peer.registerRTPTrack("late", track)
	require.Error(t, err)
	require.Nil(t, writer)
	waitCh(t, track.rtcpDone, "unpublished sender cleanup")
	require.Error(t, track.WriteRTP(mediaPacket(1, 960)))
}

func TestConferenceCloseDoesNotWaitForTrackMutation(t *testing.T) {
	peer := newMediaPeer(t)
	peer.rtpTrackMu.Lock()
	defer peer.rtpTrackMu.Unlock()
	done := make(chan struct{})
	go func() { defer close(done); assert.NoError(t, peer.Close()) }()
	waitCh(t, done, "shutdown during track mutation")
}
