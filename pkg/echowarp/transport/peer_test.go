package transport

import (
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultICEConfig() ICEConfig {
	return ICEConfig{
		STUNServers: []string{},
	}
}

func TestWebRTCPeer_CreatePeerConnection_Success(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()
}

func TestWebRTCPeer_AddAudioTrack_Success(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	sendCh, err := peer.AddAudioTrack(48000, 1)
	require.NoError(t, err)
	require.NotNil(t, sendCh)
}

func TestWebRTCPeer_CreateOffer_ValidSDP(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	_, err = peer.AddAudioTrack(48000, 1)
	require.NoError(t, err)

	offer, err := peer.CreateOffer()
	require.NoError(t, err)
	assert.Equal(t, webrtc.SDPTypeOffer, offer.Type)
	assert.Contains(t, offer.SDP, "m=audio")
}

func TestWebRTCPeer_CreateAnswer_ValidSDP(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 1)
	require.NoError(t, err)

	offer, err := sender.CreateOffer()
	require.NoError(t, err)

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)
	assert.Equal(t, webrtc.SDPTypeAnswer, answer.Type)
	assert.Contains(t, answer.SDP, "m=audio")
}

func TestWebRTCPeer_FullHandshake_Loopback(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	sendCh, err := sender.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	senderConnected := make(chan struct{})
	receiverConnected := make(chan struct{})

	sender.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(senderConnected)
		}
	})
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(receiverConnected)
		}
	})

	sender.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = receiver.AddICECandidate(c.ToJSON())
		}
	})
	receiver.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = sender.AddICECandidate(c.ToJSON())
		}
	})

	trackReceived := make(chan struct{})
	receiver.OnAudioTrack(func(inCh <-chan []byte) {
		close(trackReceived)
	})

	offer, err := sender.CreateOffer()
	require.NoError(t, err)

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)

	err = sender.SetRemoteDescription(answer)
	require.NoError(t, err)

	select {
	case <-senderConnected:
	case <-time.After(10 * time.Second):
		t.Fatal("sender connection timeout")
	}
	select {
	case <-receiverConnected:
	case <-time.After(10 * time.Second):
		t.Fatal("receiver connection timeout")
	}

	// Send some audio data to trigger OnTrack callback
	dummyAudio := make([]byte, 100)
	select {
	case sendCh <- dummyAudio:
	case <-time.After(1 * time.Second):
		t.Fatal("failed to send audio data")
	}

	select {
	case <-trackReceived:
	case <-time.After(10 * time.Second):
		t.Fatal("track receive timeout")
	}
}

func TestWebRTCPeer_Close_ReleasesResources(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)

	err = peer.Close()
	assert.NoError(t, err)
}

func TestWebRTCPeer_GetStats_ReturnsStats(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	stats := peer.GetStats()
	assert.NotEmpty(t, stats.State)
}

func TestWebRTCPeer_GetStats_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	stats := peer.GetStats()
	assert.Equal(t, "new", stats.State)
}

func TestWebRTCPeer_CreateDataChannel_Success(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.CreateDataChannel("test-channel")
	assert.NoError(t, err)
}

func TestWebRTCPeer_CreateDataChannel_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreateDataChannel("test-channel")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_CreateControlDataChannel_Success(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.CreateControlDataChannel()
	assert.NoError(t, err)
}

func TestWebRTCPeer_CreateControlDataChannel_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreateControlDataChannel()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_OnDataChannel_SetsHandler(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	handlerCalled := false
	peer.OnDataChannel(func(label string, msgCh <-chan []byte, sendFn func([]byte) error) {
		handlerCalled = true
	})

	assert.False(t, handlerCalled)
}

func TestWebRTCPeer_DCReady_ReturnsChannel(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	readyCh := peer.DCReady()
	assert.NotNil(t, readyCh)
}

func TestWebRTCPeer_SendControl_ChannelNotReady(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.SendControl("test_action", map[string]string{"key": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Control data channel not ready")
}

func TestWebRTCPeer_AddAudioTransceiver_Success(t *testing.T) {
	peer := NewWebRTCPeer(DirectionReceive)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.AddAudioTransceiver()
	assert.NoError(t, err)
}

func TestWebRTCPeer_AddAudioTransceiver_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionReceive)
	err := peer.AddAudioTransceiver()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_CreateOffer_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	_, err := peer.CreateOffer()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_CreateAnswer_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionReceive)
	_, err := peer.CreateAnswer(webrtc.SessionDescription{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_SetRemoteDescription_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.SetRemoteDescription(webrtc.SessionDescription{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_AddICECandidate_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.AddICECandidate(webrtc.ICECandidateInit{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_AddICECandidate_BuffersBeforeRemoteDesc(t *testing.T) {
	peer := NewWebRTCPeer(DirectionReceive)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	candidate := webrtc.ICECandidateInit{
		Candidate: "candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host",
	}

	err = peer.AddICECandidate(candidate)
	assert.NoError(t, err)
}

func TestWebRTCPeer_AddAudioTrack_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	_, err := peer.AddAudioTrack(48000, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_Close_MultipleCalls(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)

	err = peer.Close()
	assert.NoError(t, err)

	err = peer.Close()
	assert.NoError(t, err)
}

func TestWebRTCPeer_FullHandshake_WithICECandidates(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	err = receiver.AddAudioTransceiver()
	require.NoError(t, err)

	senderICECandidates := make(chan webrtc.ICECandidateInit, 10)
	receiverICECandidates := make(chan webrtc.ICECandidateInit, 10)

	sender.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			senderICECandidates <- c.ToJSON()
		}
	})
	receiver.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			receiverICECandidates <- c.ToJSON()
		}
	})

	offer, err := sender.CreateOffer()
	require.NoError(t, err)

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)

	err = sender.SetRemoteDescription(answer)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		select {
		case c := <-senderICECandidates:
			_ = receiver.AddICECandidate(c)
		case c := <-receiverICECandidates:
			_ = sender.AddICECandidate(c)
		case <-time.After(100 * time.Millisecond):
			break
		}
	}
}

func TestWebRTCPeer_FullHandshake_WithDataChannels(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	err = receiver.AddAudioTransceiver()
	require.NoError(t, err)

	senderConnected := make(chan struct{})
	receiverConnected := make(chan struct{})

	sender.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(senderConnected)
		}
	})
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(receiverConnected)
		}
	})

	sender.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = receiver.AddICECandidate(c.ToJSON())
		}
	})
	receiver.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = sender.AddICECandidate(c.ToJSON())
		}
	})

	dataChannelReceived := make(chan struct{})
	receiver.OnDataChannel(func(label string, msgCh <-chan []byte, sendFn func([]byte) error) {
		if label == "test-dc" {
			close(dataChannelReceived)
		}
	})

	err = sender.CreateDataChannel("test-dc")
	require.NoError(t, err)

	offer, err := sender.CreateOffer()
	require.NoError(t, err)

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)

	err = sender.SetRemoteDescription(answer)
	require.NoError(t, err)

	select {
	case <-senderConnected:
	case <-time.After(10 * time.Second):
		t.Fatal("sender connection timeout")
	}
	select {
	case <-receiverConnected:
	case <-time.After(10 * time.Second):
		t.Fatal("receiver connection timeout")
	}

	select {
	case <-dataChannelReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("data channel not received")
	}
}

func TestWebRTCPeer_ControlDataChannel_Bidirectional(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	err = sender.CreateControlDataChannel()
	require.NoError(t, err)

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	err = receiver.AddAudioTransceiver()
	require.NoError(t, err)

	senderConnected := make(chan struct{})
	receiverConnected := make(chan struct{})

	sender.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(senderConnected)
		}
	})
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			close(receiverConnected)
		}
	})

	sender.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = receiver.AddICECandidate(c.ToJSON())
		}
	})
	receiver.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = sender.AddICECandidate(c.ToJSON())
		}
	})

	offer, err := sender.CreateOffer()
	require.NoError(t, err)

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)

	err = sender.SetRemoteDescription(answer)
	require.NoError(t, err)

	// Wait for all four events with a single shared deadline.
	// Connection and DC ready happen concurrently; sequential selects with
	// independent timeouts can expire prematurely under CPU load.
	deadline := time.After(15 * time.Second)

	for _, ev := range []struct {
		ch   <-chan struct{}
		name string
	}{
		{senderConnected, "sender connection"},
		{receiverConnected, "receiver connection"},
		{sender.DCReady(), "sender DC ready"},
		{receiver.DCReady(), "receiver DC ready"},
	} {
		select {
		case <-ev.ch:
		case <-deadline:
			t.Fatalf("timeout waiting for %s", ev.name)
		}
	}

	err = sender.SendControl("test_action", map[string]string{"key": "value"})
	assert.NoError(t, err)
}
