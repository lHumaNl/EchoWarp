package transport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestControlMessages_NilBeforeDCReady verifies ControlMessages() returns nil
// before the DataChannel is established.
func TestControlMessages_NilBeforeDCReady(t *testing.T) {
	peer := NewWebRTCPeer(DirectionSend)
	err := peer.CreatePeerConnection(defaultICEConfig())
	require.NoError(t, err)
	defer peer.Close()

	assert.Nil(t, peer.ControlMessages())
}

// TestControlChannel uses a single peer pair for all subtests to avoid
// pion global state interference between tests.
func TestControlChannel(t *testing.T) {
	server, client := createConnectedPeers(t)
	t.Cleanup(func() {
		closePeerAndWait(server)
		closePeerAndWait(client)
	})

	waitDCReady(t, server, client)

	t.Run("DCReady_fires", func(t *testing.T) {
		// If we got here, DCReady already worked.
	})

	t.Run("SendControl_MessageFormat", func(t *testing.T) {
		err := server.SendControl(ActionStop, nil)
		require.NoError(t, err)

		ctrlCh := client.ControlMessages()
		require.NotNil(t, ctrlCh, "ControlMessages() should not be nil after DC is ready")

		select {
		case raw := <-ctrlCh:
			var msg struct {
				Type    string `json:"type"`
				Payload struct {
					Action string `json:"action"`
				} `json:"payload"`
			}
			err := json.Unmarshal(raw, &msg)
			require.NoError(t, err, "control message should be valid JSON: %s", string(raw))
			assert.Equal(t, TypeControl, msg.Type)
			assert.Equal(t, ActionStop, msg.Payload.Action)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for control message")
		}
	})

	t.Run("SendControl_WithPayloadData", func(t *testing.T) {
		err := server.SendControl(ActionStop, map[string]string{"reason": "user_quit"})
		require.NoError(t, err)

		ctrlCh := client.ControlMessages()
		require.NotNil(t, ctrlCh)

		select {
		case raw := <-ctrlCh:
			var msg struct {
				Type    string `json:"type"`
				Payload struct {
					Action string            `json:"action"`
					Data   map[string]string `json:"data"`
				} `json:"payload"`
			}
			err := json.Unmarshal(raw, &msg)
			require.NoError(t, err, "raw: %s", string(raw))
			assert.Equal(t, ActionStop, msg.Payload.Action)
			assert.Equal(t, "user_quit", msg.Payload.Data["reason"])
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for control message")
		}
	})

	t.Run("StopReceivedByPeer_ClientToServer", func(t *testing.T) {
		// Client sends stop to server (simulating TUI Ctrl+Q on client side).
		err := client.SendControl(ActionStop, nil)
		require.NoError(t, err)

		serverCtrlCh := server.ControlMessages()
		require.NotNil(t, serverCtrlCh)

		select {
		case raw := <-serverCtrlCh:
			var msg struct {
				Type    string `json:"type"`
				Payload struct {
					Action string `json:"action"`
				} `json:"payload"`
			}
			err := json.Unmarshal(raw, &msg)
			require.NoError(t, err, "raw: %s", string(raw))
			assert.Equal(t, TypeControl, msg.Type)
			assert.Equal(t, ActionStop, msg.Payload.Action)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for stop control on server")
		}
	})
}

// --- helpers ---

func createConnectedPeers(t *testing.T) (*WebRTCPeer, *WebRTCPeer) {
	t.Helper()

	server := NewWebRTCPeer(DirectionSend)
	client := NewWebRTCPeer(DirectionReceive)

	iceConfig := defaultICEConfig()

	require.NoError(t, server.CreatePeerConnection(iceConfig))
	require.NoError(t, client.CreatePeerConnection(iceConfig))

	// Register connection state handlers BEFORE signaling to avoid races.
	serverConnected := make(chan struct{}, 1)
	clientConnected := make(chan struct{}, 1)
	server.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case serverConnected <- struct{}{}:
			default:
			}
		}
	})
	client.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			select {
			case clientConnected <- struct{}{}:
			default:
			}
		}
	})

	// Client sets up handler for incoming data channels (answerer receives DC).
	client.setupDataChannelHandler(client.pc)

	// Server creates control data channel (offerer).
	require.NoError(t, server.CreateControlDataChannel())

	// Collect ICE candidates.
	serverCandidates := make(chan *webrtc.ICECandidate, 32)
	clientCandidates := make(chan *webrtc.ICECandidate, 32)

	server.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		serverCandidates <- candidate
	})
	client.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		clientCandidates <- candidate
	})

	// Server creates offer.
	offer, err := server.CreateOffer()
	require.NoError(t, err)

	// Client creates answer.
	answer, err := client.CreateAnswer(offer)
	require.NoError(t, err)

	// Server sets remote description.
	require.NoError(t, server.SetRemoteDescription(answer))

	// Exchange ICE candidates.
	exchangeICE(t, server, client, serverCandidates, clientCandidates)

	// Wait for both connections.
	waitCh(t, serverConnected, "server connection")
	waitCh(t, clientConnected, "client connection")

	return server, client
}

func exchangeICE(t *testing.T, server, client *WebRTCPeer, serverCh, clientCh <-chan *webrtc.ICECandidate) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	serverDone, clientDone := false, false

	for !serverDone || !clientDone {
		select {
		case c := <-serverCh:
			if c == nil {
				serverDone = true
				continue
			}
			_ = client.AddICECandidate(c.ToJSON())
		case c := <-clientCh:
			if c == nil {
				clientDone = true
				continue
			}
			_ = server.AddICECandidate(c.ToJSON())
		case <-timeout:
			return
		}
	}
}

func closePeerAndWait(p *WebRTCPeer) {
	closed := make(chan struct{})
	p.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateClosed {
			select {
			case <-closed:
			default:
				close(closed)
			}
		}
	})
	p.Close()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
	}
	// Extra settle time for pion's internal goroutines (ICE agent, DTLS).
	time.Sleep(100 * time.Millisecond)
}

func waitCh(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func waitDCReady(t *testing.T, peers ...*WebRTCPeer) {
	t.Helper()
	for _, p := range peers {
		select {
		case <-p.DCReady():
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for DCReady")
		}
	}
}
