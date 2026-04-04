package transport

import (
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebRTCPeer_CreateChatDataChannel(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.CreateChatDataChannel()
	assert.NoError(t, err)
}

func TestWebRTCPeer_CreateChatDataChannel_NoConnection(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreateChatDataChannel()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Peer connection not created")
}

func TestWebRTCPeer_ChatMessages_NilBeforeCreate(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	ch := peer.ChatMessages()
	assert.Nil(t, ch)
}

func TestWebRTCPeer_SendChat_ErrorBeforeCreate(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.SendChat([]byte("hello"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Chat data channel not ready")
}

func TestWebRTCPeer_ChatDataChannel_MessageExchange(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	err = sender.CreateChatDataChannel()
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

	// Wait for the chat DC to be set up on both sides
	require.Eventually(t, func() bool {
		return sender.ChatMessages() != nil
	}, 5*time.Second, 50*time.Millisecond, "sender chat channel not ready")

	require.Eventually(t, func() bool {
		return receiver.ChatMessages() != nil
	}, 5*time.Second, 50*time.Millisecond, "receiver chat channel not ready")

	// Sender -> Receiver
	err = sender.SendChat([]byte("hello from sender"))
	require.NoError(t, err)

	select {
	case msg := <-receiver.ChatMessages():
		assert.Equal(t, "hello from sender", string(msg))
	case <-time.After(5 * time.Second):
		t.Fatal("receiver did not get chat message from sender")
	}

	// Receiver -> Sender
	err = receiver.SendChat([]byte("hello from receiver"))
	require.NoError(t, err)

	select {
	case msg := <-sender.ChatMessages():
		assert.Equal(t, "hello from receiver", string(msg))
	case <-time.After(5 * time.Second):
		t.Fatal("sender did not get chat message from receiver")
	}
}
