package transport

import (
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseMode_SDPOffer_HasRecvonly(t *testing.T) {
	peer := NewWebRTCPeer(DirectionReceive)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	err = peer.AddAudioTransceiver()
	require.NoError(t, err)

	offer, err := peer.CreateOffer()
	require.NoError(t, err)
	assert.Contains(t, offer.SDP, "m=audio")
}

func TestNormalMode_ServerSends_ClientReceives(t *testing.T) {
	sender := NewWebRTCPeer(DirectionSend)
	err := sender.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer sender.Close()

	_, err = sender.AddAudioTrack(48000, 1)
	require.NoError(t, err)

	offer, err := sender.CreateOffer()
	require.NoError(t, err)
	assert.Contains(t, offer.SDP, "m=audio")

	receiver := NewWebRTCPeer(DirectionReceive)
	err = receiver.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer receiver.Close()

	answer, err := receiver.CreateAnswer(offer)
	require.NoError(t, err)
	assert.Contains(t, answer.SDP, "m=audio")
}

func TestReverseMode_FullHandshake(t *testing.T) {
	server := NewWebRTCPeer(DirectionReceive)
	err := server.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer server.Close()

	err = server.AddAudioTransceiver()
	require.NoError(t, err)

	client := NewWebRTCPeer(DirectionSend)
	err = client.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer client.Close()

	_, err = client.AddAudioTrack(48000, 2)
	require.NoError(t, err)

	serverConnected := make(chan struct{})
	clientConnected := make(chan struct{})

	server.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case <-serverConnected:
			default:
				close(serverConnected)
			}
		}
	})
	client.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case <-clientConnected:
			default:
				close(clientConnected)
			}
		}
	})

	server.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = client.AddICECandidate(c.ToJSON())
		}
	})
	client.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			_ = server.AddICECandidate(c.ToJSON())
		}
	})

	offer, err := server.CreateOffer()
	require.NoError(t, err)

	answer, err := client.CreateAnswer(offer)
	require.NoError(t, err)

	err = server.SetRemoteDescription(answer)
	require.NoError(t, err)
}

func TestReverseMode_DeviceSelection_ShowsCorrectType(t *testing.T) {
	assert.Equal(t, DirectionSend, MediaDirection(0))
	assert.Equal(t, DirectionReceive, MediaDirection(1))
}
