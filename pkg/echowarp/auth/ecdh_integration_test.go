package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type capturingMockSignaler struct {
	sendCh           chan transport.SignalingMessage
	recvCh           chan transport.SignalingMessage
	mu               sync.Mutex
	capturedMessages []transport.SignalingMessage
	remote           string
}

func newCapturingMockSignaler() *capturingMockSignaler {
	return &capturingMockSignaler{
		sendCh:           make(chan transport.SignalingMessage, 20),
		recvCh:           make(chan transport.SignalingMessage, 20),
		capturedMessages: make([]transport.SignalingMessage, 0),
		remote:           "127.0.0.1:12345",
	}
}

func (m *capturingMockSignaler) Start(ctx context.Context) error {
	return nil
}

func (m *capturingMockSignaler) Send(msg transport.SignalingMessage) error {
	m.mu.Lock()
	m.capturedMessages = append(m.capturedMessages, msg)
	m.mu.Unlock()
	m.sendCh <- msg
	return nil
}

func (m *capturingMockSignaler) Receive() <-chan transport.SignalingMessage {
	return m.recvCh
}

func (m *capturingMockSignaler) RemoteAddr() string {
	return m.remote
}

func (m *capturingMockSignaler) Close() error {
	return nil
}

func (m *capturingMockSignaler) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *capturingMockSignaler) pushMessage(msg transport.SignalingMessage) {
	m.recvCh <- msg
}

func (m *capturingMockSignaler) getCapturedMessages() []transport.SignalingMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]transport.SignalingMessage, len(m.capturedMessages))
	copy(result, m.capturedMessages)
	return result
}

func connectSignalers(server, client *capturingMockSignaler) {
	go func() {
		for msg := range server.sendCh {
			client.pushMessage(msg)
		}
	}()
	go func() {
		for msg := range client.sendCh {
			server.pushMessage(msg)
		}
	}()
}

type mockPeerManager struct {
	offerSDP      string
	answerSDP     string
	answerErr     error
	remoteDescErr error
	iceCandidates []webrtc.ICECandidateInit
	iceHandler    func(candidate *webrtc.ICECandidate)
	connState     webrtc.PeerConnectionState
	connStateHdlr func(state webrtc.PeerConnectionState)
}

func (m *mockPeerManager) CreatePeerConnection(iceConfig transport.ICEConfig) error {
	return nil
}

func (m *mockPeerManager) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	ch := make(chan []byte, 64)
	return ch, nil
}

func (m *mockPeerManager) OnAudioTrack(handler func(inCh <-chan []byte)) {
}

func (m *mockPeerManager) CreateOffer() (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: m.offerSDP}, nil
}

func (m *mockPeerManager) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	if m.answerErr != nil {
		return webrtc.SessionDescription{}, m.answerErr
	}
	return webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: m.answerSDP}, nil
}

func (m *mockPeerManager) SetRemoteDescription(desc webrtc.SessionDescription) error {
	return m.remoteDescErr
}

func (m *mockPeerManager) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	m.iceCandidates = append(m.iceCandidates, candidate)
	return nil
}

func (m *mockPeerManager) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
	m.iceHandler = handler
}

func (m *mockPeerManager) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
	m.connStateHdlr = handler
}

func (m *mockPeerManager) GetStats() transport.ConnectionStats {
	return transport.ConnectionStats{State: m.connState.String()}
}

func (m *mockPeerManager) CreateDataChannel(label string) error {
	return nil
}

func (m *mockPeerManager) OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
}

func (m *mockPeerManager) CreateControlDataChannel() error {
	return nil
}

func (m *mockPeerManager) DCReady() <-chan struct{} {
	return make(chan struct{})
}

func (m *mockPeerManager) SendControl(action string, payload interface{}) error {
	return nil
}
func (m *mockPeerManager) ControlMessages() <-chan []byte { return nil }

func (m *mockPeerManager) CreateChatDataChannel() error { return nil }
func (m *mockPeerManager) SendChat([]byte) error        { return nil }
func (m *mockPeerManager) ChatMessages() <-chan []byte  { return nil }

func (m *mockPeerManager) Close() error {
	return nil
}

func TestECDHIntegration_ServerClient_Success(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "test_password_123"
	serverAuth := NewECDHAuth(password)
	clientAuth := NewECDHAuth(password)

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() {
		err := serverProto.HandleAuth(ctx, serverSignaler, password)
		serverErrCh <- err
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		err := clientProto.HandleAuth(ctx, clientSignaler, password)
		clientErrCh <- err
	}()

	select {
	case err := <-serverErrCh:
		assert.NoError(t, err, "server authentication should succeed")
	case <-ctx.Done():
		t.Fatal("server authentication timed out")
	}

	select {
	case err := <-clientErrCh:
		assert.NoError(t, err, "client authentication should succeed")
	case <-ctx.Done():
		t.Fatal("client authentication timed out")
	}

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey, "server should have encryption key after auth")
	require.NotNil(t, clientKey, "client should have encryption key after auth")
	assert.Equal(t, serverKey, clientKey, "server and client should have same encryption key")
	assert.Len(t, serverKey, 32, "encryption key should be 32 bytes")
}

func TestECDHIntegration_HandshakeMessages_Encrypted(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "test_password_456"
	serverAuth := NewECDHAuth(password)
	clientAuth := NewECDHAuth(password)

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	serverPeer := &mockPeerManager{offerSDP: "v=0\r\no=- 123 123 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverAuthErrCh := make(chan error, 1)
	go func() {
		serverAuthErrCh <- serverProto.HandleAuth(ctx, serverSignaler, password)
	}()

	clientAuthErrCh := make(chan error, 1)
	go func() {
		clientAuthErrCh <- clientProto.HandleAuth(ctx, clientSignaler, password)
	}()

	require.NoError(t, <-serverAuthErrCh, "server auth should succeed")
	require.NoError(t, <-clientAuthErrCh, "client auth should succeed")

	serverSignaler.capturedMessages = nil

	handshakeCtx, handshakeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer handshakeCancel()

	handshakeErrCh := make(chan error, 1)
	go func() {
		handshakeErrCh <- serverProto.HandleHandshake(handshakeCtx, serverSignaler, serverPeer)
	}()

	// Wait for handshake to produce messages deterministically
	require.Eventually(t, func() bool {
		return len(serverSignaler.getCapturedMessages()) > 0
	}, time.Second, 10*time.Millisecond, "should have captured handshake messages")

	// Check for any handshake errors (non-blocking)
	select {
	case err := <-handshakeErrCh:
		if err != nil {
			t.Logf("Handshake error: %v", err)
		}
	default:
	}

	messages := serverSignaler.getCapturedMessages()

	offerFound := false
	for _, msg := range messages {
		if msg.Type == "offer" {
			offerFound = true

			var encryptedPayload struct {
				Nonce      string `json:"nonce"`
				Ciphertext string `json:"ciphertext"`
			}

			err := json.Unmarshal(msg.Payload, &encryptedPayload)
			require.NoError(t, err, "offer payload should be valid JSON")

			assert.NotEmpty(t, encryptedPayload.Nonce, "nonce should be present")
			assert.NotEmpty(t, encryptedPayload.Ciphertext, "ciphertext should be present")

			nonce, err := base64.StdEncoding.DecodeString(encryptedPayload.Nonce)
			require.NoError(t, err, "nonce should be valid base64")
			assert.Len(t, nonce, 12, "nonce should be 12 bytes")

			ciphertext, err := base64.StdEncoding.DecodeString(encryptedPayload.Ciphertext)
			require.NoError(t, err, "ciphertext should be valid base64")
			assert.NotEmpty(t, ciphertext, "ciphertext should not be empty")

			encryptionKey := serverAuth.EncryptionKey()
			require.NotNil(t, encryptionKey, "encryption key should be available")

			plaintext, err := DecryptAESGCM(encryptionKey, nonce, ciphertext)
			require.NoError(t, err, "decryption should succeed")

			var sdpMsg struct {
				SDP string `json:"sdp"`
			}
			err = json.Unmarshal(plaintext, &sdpMsg)
			require.NoError(t, err, "decrypted payload should be valid JSON")
			assert.Contains(t, sdpMsg.SDP, "v=0", "decrypted SDP should contain SDP header")
		}
	}

	assert.True(t, offerFound, "should have found encrypted offer message")
}

func TestChallengeAuthIntegration_HandshakeMessages_Plaintext(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "test_password_789"
	serverAuth := NewChallengeAuth()
	clientAuth := NewChallengeAuth()

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	serverPeer := &mockPeerManager{offerSDP: "v=0\r\no=- 789 789 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverAuthErrCh := make(chan error, 1)
	go func() {
		serverAuthErrCh <- serverProto.HandleAuth(ctx, serverSignaler, password)
	}()

	clientAuthErrCh := make(chan error, 1)
	go func() {
		clientAuthErrCh <- clientProto.HandleAuth(ctx, clientSignaler, password)
	}()

	require.NoError(t, <-serverAuthErrCh, "server auth should succeed")
	require.NoError(t, <-clientAuthErrCh, "client auth should succeed")

	serverSignaler.capturedMessages = nil

	handshakeCtx, handshakeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer handshakeCancel()

	handshakeErrCh := make(chan error, 1)
	go func() {
		handshakeErrCh <- serverProto.HandleHandshake(handshakeCtx, serverSignaler, serverPeer)
	}()

	// Wait for handshake to produce messages deterministically
	require.Eventually(t, func() bool {
		return len(serverSignaler.getCapturedMessages()) > 0
	}, time.Second, 10*time.Millisecond, "should have captured handshake messages")

	// Check for any handshake errors (non-blocking)
	select {
	case err := <-handshakeErrCh:
		if err != nil {
			t.Logf("Handshake error: %v", err)
		}
	default:
	}

	messages := serverSignaler.getCapturedMessages()

	offerFound := false
	for _, msg := range messages {
		if msg.Type == "offer" {
			offerFound = true

			var encryptedPayload struct {
				Nonce      string `json:"nonce"`
				Ciphertext string `json:"ciphertext"`
			}

			err := json.Unmarshal(msg.Payload, &encryptedPayload)
			if err == nil && encryptedPayload.Nonce != "" && encryptedPayload.Ciphertext != "" {
				t.Error("ChallengeAuth should send plaintext messages, not encrypted")
			}

			var sdpMsg struct {
				SDP string `json:"sdp"`
			}
			err = json.Unmarshal(msg.Payload, &sdpMsg)
			require.NoError(t, err, "ChallengeAuth should send plaintext JSON")
			assert.Contains(t, sdpMsg.SDP, "v=0", "plaintext SDP should be readable")
		}
	}

	assert.True(t, offerFound, "should have found plaintext offer message")

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()
	assert.Nil(t, serverKey, "ChallengeAuth should not have encryption key")
	assert.Nil(t, clientKey, "ChallengeAuth should not have encryption key")
}

func TestECDHIntegration_ServerECDH_ClientChallenge_Fails(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "test_password_mixed"
	serverAuth := NewECDHAuth(password)
	clientAuth := NewChallengeAuth()

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() {
		err := serverProto.HandleAuth(ctx, serverSignaler, password)
		serverErrCh <- err
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		err := clientProto.HandleAuth(ctx, clientSignaler, password)
		clientErrCh <- err
	}()

	var serverErr, clientErr error
	select {
	case err := <-serverErrCh:
		serverErr = err
	case <-time.After(2 * time.Second):
		t.Log("Server auth timed out (expected for mismatched auth)")
	}

	select {
	case err := <-clientErrCh:
		clientErr = err
	case <-time.After(2 * time.Second):
		t.Log("Client auth timed out (expected for mismatched auth)")
	}

	hasError := serverErr != nil || clientErr != nil
	assert.True(t, hasError, "Mixed ECDH/ChallengeAuth should fail - at least one side should error")

	if serverErr != nil {
		t.Logf("Server error (expected): %v", serverErr)
	}
	if clientErr != nil {
		t.Logf("Client error (expected): %v", clientErr)
	}
}

func TestECDHIntegration_NoPassword_Success(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() {
		err := serverProto.HandleAuth(ctx, serverSignaler, "")
		serverErrCh <- err
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		err := clientProto.HandleAuth(ctx, clientSignaler, "")
		clientErrCh <- err
	}()

	select {
	case err := <-serverErrCh:
		assert.NoError(t, err, "server authentication should succeed without password")
	case <-ctx.Done():
		t.Fatal("server authentication timed out")
	}

	select {
	case err := <-clientErrCh:
		assert.NoError(t, err, "client authentication should succeed without password")
	case <-ctx.Done():
		t.Fatal("client authentication timed out")
	}

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey, "server should have encryption key even without password")
	require.NotNil(t, clientKey, "client should have encryption key even without password")
	assert.Equal(t, serverKey, clientKey, "server and client should have same encryption key")
}

func TestECDHIntegration_WrongPassword_Fails(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	serverAuth := NewECDHAuth("server_password")
	clientAuth := NewECDHAuth("client_password")

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() {
		err := serverProto.HandleAuth(ctx, serverSignaler, "server_password")
		serverErrCh <- err
	}()

	clientErrCh := make(chan error, 1)
	go func() {
		err := clientProto.HandleAuth(ctx, clientSignaler, "client_password")
		clientErrCh <- err
	}()

	var serverErr, clientErr error
	select {
	case err := <-serverErrCh:
		serverErr = err
	case <-ctx.Done():
		t.Fatal("server authentication timed out")
	}

	select {
	case err := <-clientErrCh:
		clientErr = err
	case <-ctx.Done():
		t.Fatal("client authentication timed out")
	}

	hasAuthError := serverErr != nil || clientErr != nil
	assert.True(t, hasAuthError, "Authentication should fail with different passwords")
}

func TestECDHIntegration_EncryptedHandshake_RoundTrip(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "roundtrip_test_password"
	serverAuth := NewECDHAuth(password)
	clientAuth := NewECDHAuth(password)

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	serverPeer := &mockPeerManager{
		offerSDP:  "v=0\r\no=- 111 111 IN IP4 127.0.0.1\r\ns=ServerOffer\r\nt=0 0\r\n",
		answerSDP: "v=0\r\no=- 222 222 IN IP4 127.0.0.1\r\ns=ServerAnswer\r\nt=0 0\r\n",
	}
	clientPeer := &mockPeerManager{
		offerSDP:  "v=0\r\no=- 333 333 IN IP4 127.0.0.1\r\ns=ClientOffer\r\nt=0 0\r\n",
		answerSDP: "v=0\r\no=- 444 444 IN IP4 127.0.0.1\r\ns=ClientAnswer\r\nt=0 0\r\n",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverAuthErrCh := make(chan error, 1)
	go func() {
		serverAuthErrCh <- serverProto.HandleAuth(ctx, serverSignaler, password)
	}()

	clientAuthErrCh := make(chan error, 1)
	go func() {
		clientAuthErrCh <- clientProto.HandleAuth(ctx, clientSignaler, password)
	}()

	require.NoError(t, <-serverAuthErrCh, "server auth should succeed")
	require.NoError(t, <-clientAuthErrCh, "client auth should succeed")

	handshakeErrCh := make(chan error, 2)

	go func() {
		handshakeErrCh <- serverProto.HandleHandshake(ctx, serverSignaler, serverPeer)
	}()

	go func() {
		handshakeErrCh <- clientProto.HandleHandshake(ctx, clientSignaler, clientPeer)
	}()

	// Wait for handshake offer and answer to be exchanged
	require.Eventually(t, func() bool {
		serverMsgs := serverSignaler.getCapturedMessages()
		clientMsgs := clientSignaler.getCapturedMessages()
		hasOffer := false
		hasAnswer := false
		for _, msg := range serverMsgs {
			if msg.Type == "offer" {
				hasOffer = true
			}
		}
		for _, msg := range clientMsgs {
			if msg.Type == "answer" {
				hasAnswer = true
			}
		}
		return hasOffer && hasAnswer
	}, 2*time.Second, 10*time.Millisecond, "should have exchanged offer and answer")

	// Collect any errors (non-blocking since handshake might still be ongoing)
	var errors []error
	for i := 0; i < 2; i++ {
		select {
		case err := <-handshakeErrCh:
			if err != nil && err != context.DeadlineExceeded {
				errors = append(errors, err)
			}
		default:
		}
	}

	serverMessages := serverSignaler.getCapturedMessages()
	clientMessages := clientSignaler.getCapturedMessages()

	hasEncryptedOffer := false
	hasEncryptedAnswer := false

	for _, msg := range serverMessages {
		if msg.Type == "offer" {
			var encPayload struct {
				Nonce      string `json:"nonce"`
				Ciphertext string `json:"ciphertext"`
			}
			if err := json.Unmarshal(msg.Payload, &encPayload); err == nil {
				if encPayload.Nonce != "" && encPayload.Ciphertext != "" {
					hasEncryptedOffer = true
				}
			}
		}
	}

	for _, msg := range clientMessages {
		if msg.Type == "answer" {
			var encPayload struct {
				Nonce      string `json:"nonce"`
				Ciphertext string `json:"ciphertext"`
			}
			if err := json.Unmarshal(msg.Payload, &encPayload); err == nil {
				if encPayload.Nonce != "" && encPayload.Ciphertext != "" {
					hasEncryptedAnswer = true
				}
			}
		}
	}

	assert.True(t, hasEncryptedOffer, "should have encrypted offer message")
	assert.True(t, hasEncryptedAnswer, "should have encrypted answer message")
	assert.LessOrEqual(t, len(errors), 1, "handshake should mostly succeed")
}

func TestECDHIntegration_MultipleHandshakes_Independent(t *testing.T) {
	password := "multi_handshake_password"

	for i := 0; i < 3; i++ {
		t.Run("handshake_"+string(rune('A'+i)), func(t *testing.T) {
			serverSignaler := newCapturingMockSignaler()
			clientSignaler := newCapturingMockSignaler()

			connectSignalers(serverSignaler, clientSignaler)

			serverAuth := NewECDHAuth(password)
			clientAuth := NewECDHAuth(password)

			serverProto := transport.NewServerSignalingProtocol(serverAuth)
			clientProto := transport.NewClientSignalingProtocol(clientAuth)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			serverErrCh := make(chan error, 1)
			go func() {
				serverErrCh <- serverProto.HandleAuth(ctx, serverSignaler, password)
			}()

			clientErrCh := make(chan error, 1)
			go func() {
				clientErrCh <- clientProto.HandleAuth(ctx, clientSignaler, password)
			}()

			require.NoError(t, <-serverErrCh, "server auth should succeed")
			require.NoError(t, <-clientErrCh, "client auth should succeed")

			serverKey := serverAuth.EncryptionKey()
			clientKey := clientAuth.EncryptionKey()

			require.NotNil(t, serverKey)
			require.NotNil(t, clientKey)
			assert.Equal(t, serverKey, clientKey)
			assert.Len(t, serverKey, 32)
		})
	}
}

func TestECDHIntegration_LargePayload_Encryption(t *testing.T) {
	serverSignaler := newCapturingMockSignaler()
	clientSignaler := newCapturingMockSignaler()

	connectSignalers(serverSignaler, clientSignaler)

	password := "large_payload_test"
	serverAuth := NewECDHAuth(password)
	clientAuth := NewECDHAuth(password)

	serverProto := transport.NewServerSignalingProtocol(serverAuth)
	clientProto := transport.NewClientSignalingProtocol(clientAuth)

	largeSDP := "v=0\r\no=- 999 999 IN IP4 127.0.0.1\r\ns=LargeSDPTest\r\nt=0 0\r\n"
	for i := 0; i < 100; i++ {
		largeSDP += "a=rtpmap:96 opus/48000/2\r\n"
	}

	serverPeer := &mockPeerManager{offerSDP: largeSDP}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverAuthErrCh := make(chan error, 1)
	go func() {
		serverAuthErrCh <- serverProto.HandleAuth(ctx, serverSignaler, password)
	}()

	clientAuthErrCh := make(chan error, 1)
	go func() {
		clientAuthErrCh <- clientProto.HandleAuth(ctx, clientSignaler, password)
	}()

	require.NoError(t, <-serverAuthErrCh)
	require.NoError(t, <-clientAuthErrCh)

	serverSignaler.capturedMessages = nil

	handshakeCtx, handshakeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer handshakeCancel()

	go func() {
		_ = serverProto.HandleHandshake(handshakeCtx, serverSignaler, serverPeer)
	}()

	// Wait for handshake to produce messages deterministically
	require.Eventually(t, func() bool {
		return len(serverSignaler.getCapturedMessages()) > 0
	}, time.Second, 10*time.Millisecond, "should have captured handshake messages")

	messages := serverSignaler.getCapturedMessages()

	for _, msg := range messages {
		if msg.Type == "offer" {
			var encPayload struct {
				Nonce      string `json:"nonce"`
				Ciphertext string `json:"ciphertext"`
			}
			err := json.Unmarshal(msg.Payload, &encPayload)
			require.NoError(t, err)

			nonce, _ := base64.StdEncoding.DecodeString(encPayload.Nonce)
			ciphertext, _ := base64.StdEncoding.DecodeString(encPayload.Ciphertext)

			encryptionKey := serverAuth.EncryptionKey()
			plaintext, err := DecryptAESGCM(encryptionKey, nonce, ciphertext)
			require.NoError(t, err)

			var sdpMsg struct {
				SDP string `json:"sdp"`
			}
			err = json.Unmarshal(plaintext, &sdpMsg)
			require.NoError(t, err)
			assert.Contains(t, sdpMsg.SDP, "LargeSDPTest")
			assert.Contains(t, sdpMsg.SDP, "rtpmap")
			break
		}
	}
}
