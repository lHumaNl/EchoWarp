package transport

import (
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUDPMuxManager(t *testing.T) {
	mux, err := NewUDPMuxManager(0) // port 0 = OS picks free port
	require.NoError(t, err)
	require.NotNil(t, mux)
	defer func() { _ = mux.Close() }()

	assert.NotNil(t, mux.WebRTCAPI())
	assert.NotZero(t, mux.Port())
}

func TestUDPMuxManager_CloseIdempotent(t *testing.T) {
	mux, err := NewUDPMuxManager(0)
	require.NoError(t, err)

	assert.NoError(t, mux.Close())
	assert.NoError(t, mux.Close()) // second close should not error
}

func TestUDPMuxManager_CreatePeerConnection(t *testing.T) {
	mux, err := NewUDPMuxManager(0)
	require.NoError(t, err)
	defer func() { _ = mux.Close() }()

	peer := NewWebRTCPeer(DirectionSend)
	iceConfig := ICEConfig{UDPMux: mux}
	err = peer.CreatePeerConnection(iceConfig)
	require.NoError(t, err)
	defer func() { _ = peer.Close() }()

	stats := peer.GetStats()
	assert.NotEmpty(t, stats.State)
}

func TestUDPMuxManager_MultiplePeersConnected(t *testing.T) {
	// Server-side mux: all server peers share one UDP port.
	serverMux, err := NewUDPMuxManager(0)
	require.NoError(t, err)
	defer func() { _ = serverMux.Close() }()

	serverICE := ICEConfig{UDPMux: serverMux}
	clientICE := ICEConfig{} // clients use ephemeral ports (normal behavior)

	const numClients = 3

	// connectPair establishes a full WebRTC connection between a server peer and a client peer.
	// Server peer sends audio, client peer receives it, verifying data flows through the mux.
	connectPair := func(t *testing.T, id int) {
		t.Helper()

		server := NewWebRTCPeer(DirectionSend)
		require.NoError(t, server.CreatePeerConnection(serverICE))
		defer func() { _ = server.Close() }()

		sendCh, err := server.AddAudioTrack(48000, 1)
		require.NoError(t, err)

		client := NewWebRTCPeer(DirectionReceive)
		require.NoError(t, client.CreatePeerConnection(clientICE))
		defer func() { _ = client.Close() }()

		require.NoError(t, client.AddAudioTransceiver())

		serverConnected := make(chan struct{})
		clientConnected := make(chan struct{})

		server.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			if state == webrtc.PeerConnectionStateConnected {
				close(serverConnected)
			}
		})
		client.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			if state == webrtc.PeerConnectionStateConnected {
				close(clientConnected)
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

		trackReceived := make(chan struct{})
		client.OnAudioTrack(func(inCh <-chan []byte) {
			close(trackReceived)
		})

		offer, err := server.CreateOffer()
		require.NoError(t, err)

		answer, err := client.CreateAnswer(offer)
		require.NoError(t, err)

		require.NoError(t, server.SetRemoteDescription(answer))

		select {
		case <-serverConnected:
		case <-time.After(10 * time.Second):
			t.Fatalf("client %d: server connection timeout", id)
		}
		select {
		case <-clientConnected:
		case <-time.After(10 * time.Second):
			t.Fatalf("client %d: client connection timeout", id)
		}

		// Send audio data to trigger track callback.
		dummyAudio := make([]byte, 160)
		select {
		case sendCh <- dummyAudio:
		case <-time.After(time.Second):
			t.Fatalf("client %d: send audio timeout", id)
		}

		select {
		case <-trackReceived:
		case <-time.After(10 * time.Second):
			t.Fatalf("client %d: track receive timeout", id)
		}
	}

	// Connect all clients in parallel — all server peers share the same UDP port.
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			connectPair(t, id)
		}(i)
	}
	wg.Wait()
}

func TestICEConfig_WithoutUDPMux(t *testing.T) {
	// Without UDPMux, should use default webrtc.NewPeerConnection
	peer := NewWebRTCPeer(DirectionSend)
	iceConfig := ICEConfig{}
	err := peer.CreatePeerConnection(iceConfig)
	require.NoError(t, err)
	defer func() { _ = peer.Close() }()
}
