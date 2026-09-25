package app

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type conferenceTestEndpoint struct {
	app     *ClientApp
	session *conferenceClientSession
	writer  transport.RTPWriter
	cancel  context.CancelFunc
}

func connectConferenceEndpoint(t *testing.T, server *ServerApp, id string, channels uint32) *conferenceTestEndpoint {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	sp := transport.NewWebRTCPeer(transport.DirectionDuplex)
	cp := transport.NewWebRTCPeer(transport.DirectionDuplex)
	require.NoError(t, sp.CreatePeerConnection(transport.ICEConfig{}))
	require.NoError(t, cp.CreatePeerConnection(transport.ICEConfig{}))
	t.Cleanup(func() { cancel(); _ = cp.Close(); _ = sp.Close() })
	require.NoError(t, sp.AddAudioTransceiver())
	writer, err := cp.AddRTPTrack("untrusted-capture-id")
	require.NoError(t, err)
	cfg := clientTestConfig()
	cfg.Conference, cfg.Channels = true, channels
	client := NewClientApp(cfg, testClientLogger(), nil)
	session, err := newConferenceClientSession(ctx, cp, cfg, testClientLogger())
	require.NoError(t, err)
	client.conferenceSession = session
	t.Cleanup(session.Close)
	_, err = server.setupConferenceAudioPipeline(ctx, sp, id, nil)
	require.NoError(t, err)
	server.mu.Lock()
	server.clients[id] = &multiClient{id: id, nickname: id, peer: sp}
	server.mu.Unlock()
	server.conference.AddParticipant(id)
	sp.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			_ = cp.AddICECandidate(candidate.ToJSON())
		}
	})
	cp.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			_ = sp.AddICECandidate(candidate.ToJSON())
		}
	})
	require.NoError(t, sp.CreateControlDataChannel())
	offer, err := sp.CreateOffer()
	require.NoError(t, err)
	answer, err := cp.CreateAnswer(offer)
	require.NoError(t, err)
	require.NoError(t, sp.SetRemoteDescription(answer))
	for _, ready := range []<-chan struct{}{sp.DCReady(), cp.DCReady()} {
		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			t.Fatal("data channel handshake timed out")
		}
	}
	var readers sync.WaitGroup
	pump := func(peer transport.PeerManager, handle func([]byte)) {
		defer readers.Done()
		ch := peer.ControlMessages()
		for {
			select {
			case <-ctx.Done():
				return
			case raw, ok := <-ch:
				if !ok {
					return
				}
				handle(raw)
			}
		}
	}
	readers.Add(2)
	go pump(sp, func(raw []byte) { server.handleDCControlMulti(raw, time.Now(), id) })
	go pump(cp, func(raw []byte) { client.handleDCControl(raw) })
	t.Cleanup(func() { cancel(); readers.Wait() })
	session.Ready()
	server.conferenceRoom.Ready(id)
	require.Eventually(t, func() bool {
		server.conferenceRoom.mu.RLock()
		defer server.conferenceRoom.mu.RUnlock()
		return server.conferenceRoom.peers[id].compatible
	}, 10*time.Second, time.Millisecond)
	return &conferenceTestEndpoint{app: client, session: session, writer: writer, cancel: cancel}
}

func waitConferenceTracks(t *testing.T, server *ServerApp, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		server.conferenceRoom.mu.RLock()
		defer server.conferenceRoom.mu.RUnlock()
		if len(server.conferenceRoom.peers) != count {
			return false
		}
		for id, peer := range server.conferenceRoom.peers {
			for source := range server.conferenceRoom.sources {
				if source != id && !peer.writers[source].negotiated {
					return false
				}
			}
		}
		return true
	}, 10*time.Second, 5*time.Millisecond, "conference tracks were not negotiated")
}

func TestConferenceEndToEndRoutingAndMixing(t *testing.T) {
	for _, channels := range []uint32{1, 2} {
		t.Run(fmt.Sprintf("channels%d", channels), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Conference, cfg.ServerMuted, cfg.Channels = true, true, channels
			server := NewServerApp(cfg, testClientLogger(), nil, nil, nil)
			t.Cleanup(server.conferenceRoom.Close)
			alice := connectConferenceEndpoint(t, server, "alice", channels)
			bob := connectConferenceEndpoint(t, server, "bob", channels)
			carol := connectConferenceEndpoint(t, server, "carol", channels)
			server.conferenceRoom.AddServerSource()
			waitConferenceTracks(t, server, 3)
			endpoints := []*conferenceTestEndpoint{alice, bob, carol}
			frequencies := []float64{300, 600, 900, 1200}
			encoders := make([]*audio.OpusEncoder, 4)
			for i := range encoders {
				var err error
				encoders[i], err = audio.NewOpusEncoder(48000, int(channels), "audio")
				require.NoError(t, err)
			}
			var frameNumber uint32
			sendFrames := func(n int) {
				for range n {
					for source, frequency := range frequencies {
						pcm := make([]float32, 960*int(channels))
						for i := range 960 {
							for ch := range int(channels) {
								pcm[i*int(channels)+ch] = float32(0.1 * math.Sin(2*math.Pi*frequency*float64(frameNumber*960+uint32(i))/48000))
							}
						}
						payload, err := encoders[source].Encode(pcm)
						require.NoError(t, err)
						packet := &rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: uint16(frameNumber), Timestamp: frameNumber * 960, SSRC: uint32(source + 1)}, Payload: payload}
						if source == 3 {
							server.conferenceRoom.WriteSource("server", packet)
						} else {
							require.NoError(t, endpoints[source].writer.WriteRTP(packet))
						}
						audio.PutOpusOutput(payload)
					}
					frameNumber++
					time.Sleep(20 * time.Millisecond)
				}
			}
			sendFrames(8)
			for recipient, endpoint := range endpoints {
				mixed := endpoint.session.mixer.Render(nil)
				require.Len(t, mixed, 960*int(channels))
				for source, frequency := range frequencies {
					amplitude := conferenceToneAmplitude(mixed, int(channels), frequency)
					if source == recipient {
						require.Less(t, amplitude, 0.02, "self audio leaked")
					} else {
						require.Greater(t, amplitude, 0.025, "missing source %d for recipient %d", source, recipient)
					}
				}
			}
			// The two legacy admin controls are independent enforcement reasons.
			server.conference.SetParticipantMuted("bob", true)
			server.toggleMuteIncoming("bob")
			server.toggleMuteIncoming("bob")
			sendFrames(4)
			require.Less(t, conferenceToneAmplitude(alice.session.mixer.Render(nil), int(channels), 600), 0.01)
			server.toggleMuteIncoming("bob")
			server.conference.SetParticipantMuted("bob", false)
			sendFrames(4)
			require.Less(t, conferenceToneAmplitude(alice.session.mixer.Render(nil), int(channels), 600), 0.01)
			server.toggleMuteIncoming("bob")
			server.adjustClientVolume("bob", -0.5)
			sendFrames(8)
			// Opus decoder startup after unmute has codec lookahead; inspect a
			// settled frame rather than treating the first partial tone as unity.
			for range 3 {
				alice.session.mixer.Render(nil)
			}
			require.InDelta(t, 0.05, conferenceToneAmplitude(alice.session.mixer.Render(nil), int(channels), 600), 0.02)
			server.adjustClientVolume("bob", 0.5)
			// Receiver's and sender's rules remain independent, and no renegotiation
			// is required for any of these delivery changes.
			receive := echowarp.AudioRouteRule{Scope: "receive", Source: "bob", Recipient: "self", Muted: true}
			require.NoError(t, alice.app.SetAudioRoute(t.Context(), receive))
			send := echowarp.AudioRouteRule{Scope: "send", Source: "self", Recipient: "alice", Muted: true}
			require.NoError(t, bob.app.SetAudioRoute(t.Context(), send))
			receive.Muted = false
			require.NoError(t, alice.app.SetAudioRoute(t.Context(), receive))
			sendFrames(4)
			require.Eventually(t, func() bool {
				alice.session.mu.Lock()
				defer alice.session.mu.Unlock()
				return !alice.session.sources["bob"].state.Enabled
			}, time.Second, time.Millisecond)
			require.Less(t, conferenceToneAmplitude(alice.session.mixer.Render(nil), int(channels), 600), 0.01)
			send.Muted = false
			require.NoError(t, bob.app.SetAudioRoute(t.Context(), send))
			require.NoError(t, server.SetAudioRoute(t.Context(), echowarp.AudioRouteRule{Scope: "admin", Source: "server", Recipient: "alice", Muted: true}))
			sendFrames(8)
			mixed := alice.session.mixer.Render(nil)
			require.Greater(t, conferenceToneAmplitude(mixed, int(channels), 600), 0.025)
			require.Less(t, conferenceToneAmplitude(mixed, int(channels), 1200), 0.01)
			carol.cancel()
			waitConferenceTracks(t, server, 2)
			carol = connectConferenceEndpoint(t, server, "carol-new", channels)
			endpoints[2] = carol
			waitConferenceTracks(t, server, 3)
			sendFrames(8)
			require.Greater(t, conferenceToneAmplitude(alice.session.mixer.Render(nil), int(channels), 900), 0.025)
		})
	}
}

func conferenceToneAmplitude(pcm []float32, channels int, frequency float64) float64 {
	var real, imaginary float64
	for i := range len(pcm) / channels {
		phase := 2 * math.Pi * frequency * float64(i) / 48000
		real += float64(pcm[i*channels]) * math.Cos(phase)
		imaginary += float64(pcm[i*channels]) * math.Sin(phase)
	}
	return 2 * math.Hypot(real, imaginary) / float64(len(pcm)/channels)
}
