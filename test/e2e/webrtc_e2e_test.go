//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

func TestE2E_WebRTC_FullHandshake_Loopback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow WebRTC test in short mode")
	}
	t.Parallel()

	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	server := transport.NewTCPSignaler(transport.RoleServer, addr)
	client := transport.NewTCPSignaler(transport.RoleClient, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server signaler error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.ListenAddr() != ""
	}, 2*time.Second, 50*time.Millisecond, "server listen addr")

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("client signaler error: %v", err)
		}
	}()

	require.Eventually(t, func() bool {
		return server.RemoteAddr() != ""
	}, 5*time.Second, 50*time.Millisecond, "server remote addr")

	senderPeer := transport.NewWebRTCPeer(transport.DirectionSend)
	receiverPeer := transport.NewWebRTCPeer(transport.DirectionReceive)

	iceConfig := transport.ICEConfig{
		STUNServers: []string{},
	}

	if err := senderPeer.CreatePeerConnection(iceConfig); err != nil {
		t.Fatalf("failed to create sender peer connection: %v", err)
	}
	defer senderPeer.Close()

	if err := receiverPeer.CreatePeerConnection(iceConfig); err != nil {
		t.Fatalf("failed to create receiver peer connection: %v", err)
	}
	defer receiverPeer.Close()

	audioReceived := make(chan struct{})
	receiverPeer.OnAudioTrack(func(inCh <-chan []byte) {
		close(audioReceived)
	})

	_, err := senderPeer.AddAudioTrack(48000, 1)
	if err != nil {
		t.Fatalf("failed to add audio track: %v", err)
	}

	senderConnected := make(chan webrtc.PeerConnectionState, 10)
	receiverConnected := make(chan webrtc.PeerConnectionState, 10)

	senderPeer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		select {
		case senderConnected <- state:
		default:
		}
	})

	receiverPeer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		select {
		case receiverConnected <- state:
		default:
		}
	})

	var senderCandidatesMux sync.Mutex
	senderCandidates := make([]webrtc.ICECandidateInit, 0)
	senderCandidateDone := make(chan struct{})
	receiverCandidateDone := make(chan struct{})

	senderPeer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			close(senderCandidateDone)
			return
		}
		senderCandidatesMux.Lock()
		senderCandidates = append(senderCandidates, candidate.ToJSON())
		senderCandidatesMux.Unlock()
	})

	receiverPeer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			close(receiverCandidateDone)
			return
		}
		candidateJSON := candidate.ToJSON()
		payload, _ := json.Marshal(candidateJSON)
		_ = client.Send(transport.SignalingMessage{Type: "candidate", Payload: payload})
	})

	offer, err := senderPeer.CreateOffer()
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}

	offerPayload, _ := json.Marshal(map[string]string{"sdp": offer.SDP})
	if err := server.Send(transport.SignalingMessage{Type: "offer", Payload: offerPayload}); err != nil {
		t.Fatalf("failed to send offer: %v", err)
	}

	var answer webrtc.SessionDescription
	select {
	case msg := <-client.Receive():
		if msg.Type != "offer" {
			t.Fatalf("expected offer, got %s", msg.Type)
		}
		var sdpMsg struct{ SDP string }
		if err := json.Unmarshal(msg.Payload, &sdpMsg); err != nil {
			t.Fatalf("failed to unmarshal offer: %v", err)
		}
		offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdpMsg.SDP}

		answer, err = receiverPeer.CreateAnswer(offer)
		if err != nil {
			t.Fatalf("failed to create answer: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for offer")
	}

	answerPayload, _ := json.Marshal(map[string]string{"sdp": answer.SDP})
	if err := client.Send(transport.SignalingMessage{Type: "answer", Payload: answerPayload}); err != nil {
		t.Fatalf("failed to send answer: %v", err)
	}

	select {
	case msg := <-server.Receive():
		if msg.Type != "answer" {
			t.Fatalf("expected answer, got %s", msg.Type)
		}
		var sdpMsg struct{ SDP string }
		if err := json.Unmarshal(msg.Payload, &sdpMsg); err != nil {
			t.Fatalf("failed to unmarshal answer: %v", err)
		}
		answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdpMsg.SDP}
		if err := senderPeer.SetRemoteDescription(answer); err != nil {
			t.Fatalf("failed to set remote description: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for answer")
	}

	go func() {
		for {
			select {
			case msg := <-client.Receive():
				if msg.Type == "candidate" {
					var candidate webrtc.ICECandidateInit
					if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
						continue
					}
					_ = receiverPeer.AddICECandidate(candidate)
				}
			case msg := <-server.Receive():
				if msg.Type == "candidate" {
					var candidate webrtc.ICECandidateInit
					if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
						continue
					}
					_ = senderPeer.AddICECandidate(candidate)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	senderCandidatesMux.Lock()
	for _, c := range senderCandidates {
		payload, _ := json.Marshal(c)
		_ = server.Send(transport.SignalingMessage{Type: "candidate", Payload: payload})
	}
	senderCandidatesMux.Unlock()

	timeout := time.After(15 * time.Second)
	for {
		select {
		case state := <-senderConnected:
			if state == webrtc.PeerConnectionStateConnected {
				t.Log("sender peer connected")
				goto checkReceiver
			}
		case <-timeout:
			t.Fatal("timeout waiting for sender peer to connect")
		}
	}

checkReceiver:
	timeout = time.After(15 * time.Second)
	for {
		select {
		case state := <-receiverConnected:
			if state == webrtc.PeerConnectionStateConnected {
				t.Log("receiver peer connected")
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for receiver peer to connect")
		}
	}

done:
	_ = server.Close()
	_ = client.Close()
}

func TestE2E_WebRTC_DataChannel_CreatedAfterConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow WebRTC test in short mode")
	}
	t.Parallel()

	senderPeer := transport.NewWebRTCPeer(transport.DirectionSend)
	receiverPeer := transport.NewWebRTCPeer(transport.DirectionReceive)

	iceConfig := transport.ICEConfig{STUNServers: []string{}}

	require.NoError(t, senderPeer.CreatePeerConnection(iceConfig))
	defer senderPeer.Close()

	require.NoError(t, receiverPeer.CreatePeerConnection(iceConfig))
	defer receiverPeer.Close()

	// Set up direct ICE candidate exchange between peers (no signaler needed for loopback)
	senderPeer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			_ = receiverPeer.AddICECandidate(candidate.ToJSON())
		}
	})
	receiverPeer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			_ = senderPeer.AddICECandidate(candidate.ToJSON())
		}
	})

	dataChannelReceived := make(chan string, 1)
	receiverPeer.OnDataChannel(func(label string, msgCh <-chan []byte, sendFn func([]byte) error) {
		dataChannelReceived <- label
	})

	require.NoError(t, senderPeer.CreateDataChannel("control"))

	offer, err := senderPeer.CreateOffer()
	require.NoError(t, err)

	answer, err := receiverPeer.CreateAnswer(offer)
	require.NoError(t, err)

	require.NoError(t, senderPeer.SetRemoteDescription(answer))

	select {
	case label := <-dataChannelReceived:
		require.Equal(t, "control", label)
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for data channel")
	}
}

func TestE2E_WebRTC_ICECandidate_Trickle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow WebRTC test in short mode")
	}
	t.Parallel()

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	iceConfig := transport.ICEConfig{STUNServers: []string{}}

	if err := peer.CreatePeerConnection(iceConfig); err != nil {
		t.Fatalf("failed to create peer connection: %v", err)
	}
	defer peer.Close()

	candidates := make(chan *webrtc.ICECandidate, 100)
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		candidates <- candidate
	})

	offer, err := peer.CreateOffer()
	if err != nil {
		t.Fatalf("failed to create offer: %v", err)
	}

	if offer.SDP == "" {
		t.Error("offer SDP should not be empty")
	}

	timeout := time.After(5 * time.Second)
	candidateCount := 0
	for {
		select {
		case c := <-candidates:
			if c == nil {
				t.Logf("ICE gathering complete, collected %d candidates", candidateCount)
				return
			}
			candidateCount++
		case <-timeout:
			if candidateCount > 0 {
				t.Logf("Collected %d candidates before timeout", candidateCount)
				return
			}
			t.Fatal("timeout waiting for ICE candidates")
		}
	}
}
