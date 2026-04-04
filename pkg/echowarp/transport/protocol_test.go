package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSignaler struct {
	sendCh    chan SignalingMessage
	recvCh    chan SignalingMessage
	sendError error
	remote    string
}

func newMockSignaler() *mockSignaler {
	return &mockSignaler{
		sendCh: make(chan SignalingMessage, 10),
		recvCh: make(chan SignalingMessage, 10),
		remote: "127.0.0.1:12345",
	}
}

func (m *mockSignaler) Start(ctx context.Context) error {
	return nil
}

func (m *mockSignaler) Send(msg SignalingMessage) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sendCh <- msg
	return nil
}

func (m *mockSignaler) Receive() <-chan SignalingMessage {
	return m.recvCh
}

func (m *mockSignaler) RemoteAddr() string {
	return m.remote
}

func (m *mockSignaler) Close() error {
	return nil
}

func (m *mockSignaler) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *mockSignaler) pushMessage(msg SignalingMessage) {
	m.recvCh <- msg
}

type mockAuthHandler struct {
	serverAuthErr error
	clientAuthErr error
}

func (m *mockAuthHandler) AuthenticateServer(send AuthSendFunc, recv AuthRecvFunc, password string) error {
	return m.serverAuthErr
}

func (m *mockAuthHandler) AuthenticateClient(send AuthSendFunc, recv AuthRecvFunc, password string) error {
	return m.clientAuthErr
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

func (m *mockPeerManager) CreatePeerConnection(iceConfig ICEConfig) error {
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

func (m *mockPeerManager) GetStats() ConnectionStats {
	return ConnectionStats{State: m.connState.String()}
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
func (m *mockPeerManager) CreateChatDataChannel() error   { return nil }
func (m *mockPeerManager) SendChat([]byte) error          { return nil }
func (m *mockPeerManager) ChatMessages() <-chan []byte    { return nil }

func (m *mockPeerManager) Close() error {
	return nil
}

func TestNewServerSignalingProtocol(t *testing.T) {
	auth := &mockAuthHandler{}
	protocol := NewServerSignalingProtocol(auth)
	assert.NotNil(t, protocol)
	assert.Equal(t, auth, protocol.auth)
}

func TestServerSignalingProtocol_HandleAuth_Success(t *testing.T) {
	auth := &mockAuthHandler{}
	protocol := NewServerSignalingProtocol(auth)
	signaler := newMockSignaler()

	ctx := context.Background()
	err := protocol.HandleAuth(ctx, signaler, "password")
	assert.NoError(t, err)
}

func TestServerSignalingProtocol_HandleAuth_Timeout(t *testing.T) {
	auth := &mockAuthHandler{serverAuthErr: context.DeadlineExceeded}
	protocol := NewServerSignalingProtocol(auth)
	signaler := newMockSignaler()
	signaler.sendError = context.DeadlineExceeded

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := protocol.HandleAuth(ctx, signaler, "password")
	assert.Error(t, err)
}

func TestServerSignalingProtocol_SendAuthResult_Success(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	result := AuthResultMessage{Success: true, Message: "OK"}
	err := protocol.SendAuthResult(signaler, result)
	require.NoError(t, err)

	select {
	case msg := <-signaler.sendCh:
		assert.Equal(t, "auth_result", msg.Type)
		var parsed AuthResultMessage
		require.NoError(t, json.Unmarshal(msg.Payload, &parsed))
		assert.True(t, parsed.Success)
		assert.Equal(t, "OK", parsed.Message)
	default:
		t.Fatal("expected message to be sent")
	}
}

func TestServerSignalingProtocol_HandleHandshake_Success(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{offerSDP: "test-offer-sdp"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	answerPayload, _ := json.Marshal(map[string]string{"sdp": "test-answer-sdp"})
	signaler.pushMessage(SignalingMessage{Type: "answer", Payload: answerPayload})

	err := protocol.HandleHandshake(ctx, signaler, peer)
	assert.NoError(t, err)
}

func TestServerSignalingProtocol_HandleHandshake_ContextCancelled(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{offerSDP: "test-offer-sdp"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := protocol.HandleHandshake(ctx, signaler, peer)
	assert.Error(t, err)
}

func TestServerSignalingProtocol_HandleHandshakeMessage_Answer(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	answerPayload, _ := json.Marshal(map[string]string{"sdp": "test-answer-sdp"})
	msg := SignalingMessage{Type: "answer", Payload: answerPayload}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.NoError(t, err)
}

func TestServerSignalingProtocol_HandleHandshakeMessage_Candidate(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	candidatePayload, _ := json.Marshal(webrtc.ICECandidateInit{Candidate: "test-candidate"})
	msg := SignalingMessage{Type: "candidate", Payload: candidatePayload}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.NoError(t, err)
	assert.Len(t, peer.iceCandidates, 1)
}

func TestNewClientSignalingProtocol(t *testing.T) {
	auth := &mockAuthHandler{}
	protocol := NewClientSignalingProtocol(auth)
	assert.NotNil(t, protocol)
	assert.Equal(t, auth, protocol.auth)
}

func TestClientSignalingProtocol_HandleAuth_Success(t *testing.T) {
	auth := &mockAuthHandler{}
	protocol := NewClientSignalingProtocol(auth)
	signaler := newMockSignaler()

	ctx := context.Background()
	err := protocol.HandleAuth(ctx, signaler, "password")
	assert.NoError(t, err)
}

func TestClientSignalingProtocol_WaitForAuthResult_Success(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	result := AuthResultMessage{Success: true, Message: "Authenticated", Code: 200}
	payload, _ := json.Marshal(result)
	signaler.pushMessage(SignalingMessage{Type: "auth_result", Payload: payload})

	got, err := protocol.WaitForAuthResult(signaler)
	require.NoError(t, err)
	assert.True(t, got.Success)
	assert.Equal(t, "Authenticated", got.Message)
}

func TestClientSignalingProtocol_WaitForAuthResult_WrongType(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	signaler.pushMessage(SignalingMessage{Type: "other", Payload: []byte("{}")})

	_, err := protocol.WaitForAuthResult(signaler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected auth_result")
}

func TestClientSignalingProtocol_WaitForAuthResult_NoMessage(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	// Close the receive channel so select returns zero-value immediately.
	close(signaler.recvCh)
	_, err := protocol.WaitForAuthResult(signaler)
	assert.Error(t, err)
}

func TestClientSignalingProtocol_HandleHandshake_Success(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{answerSDP: "test-answer-sdp"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	offerPayload, _ := json.Marshal(map[string]string{"sdp": "test-offer-sdp"})
	signaler.pushMessage(SignalingMessage{Type: "offer", Payload: offerPayload})

	err := protocol.HandleHandshake(ctx, signaler, peer)
	assert.NoError(t, err)
}

func TestClientSignalingProtocol_HandleHandshake_ContextCancelled(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := protocol.HandleHandshake(ctx, signaler, peer)
	assert.Error(t, err)
}

func TestClientSignalingProtocol_HandleOffer_Success(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{answerSDP: "test-answer-sdp"}

	offerPayload, _ := json.Marshal(map[string]string{"sdp": "test-offer-sdp"})
	msg := SignalingMessage{Type: "offer", Payload: offerPayload}

	err := protocol.handleOffer(msg, signaler, peer)
	assert.NoError(t, err)

	select {
	case sentMsg := <-signaler.sendCh:
		assert.Equal(t, "answer", sentMsg.Type)
	default:
		t.Fatal("expected answer to be sent")
	}
}

func TestClientSignalingProtocol_HandleOffer_InvalidJSON(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	msg := SignalingMessage{Type: "offer", Payload: []byte("invalid-json")}

	err := protocol.handleOffer(msg, signaler, peer)
	assert.Error(t, err)
}

func TestClientSignalingProtocol_HandleHandshakeMessage_Candidate(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	candidatePayload, _ := json.Marshal(webrtc.ICECandidateInit{Candidate: "test-candidate"})
	msg := SignalingMessage{Type: "candidate", Payload: candidatePayload}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.NoError(t, err)
	assert.Len(t, peer.iceCandidates, 1)
}

func TestRunServerHandshakeLoop_Candidate(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	candidatePayload, _ := json.Marshal(webrtc.ICECandidateInit{Candidate: "test-candidate"})
	signaler.pushMessage(SignalingMessage{Type: "candidate", Payload: candidatePayload})

	_ = RunServerHandshakeLoop(ctx, signaler, peer, nil)
	assert.Len(t, peer.iceCandidates, 1)
}

func TestRunServerHandshakeLoop_CandidateDone(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	var loggedMsg string
	logger := func(msg string, args ...interface{}) {
		loggedMsg = fmt.Sprintf(msg, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	signaler.pushMessage(SignalingMessage{Type: "candidate_done", Payload: []byte("{}")})

	_ = RunServerHandshakeLoop(ctx, signaler, peer, logger)
	assert.Equal(t, "Remote ICE gathering complete", loggedMsg)
}

func TestRunServerHandshakeLoop_ControlStop(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	var loggedMsg string
	logger := func(msg string, args ...interface{}) {
		loggedMsg = fmt.Sprintf(msg, args...)
	}

	ctx := context.Background()

	controlPayload, _ := json.Marshal(map[string]string{"action": "stop"})
	signaler.pushMessage(SignalingMessage{Type: "control", Payload: controlPayload})

	err := RunServerHandshakeLoop(ctx, signaler, peer, logger)
	assert.NoError(t, err)
	assert.Equal(t, "Client requested stop", loggedMsg)
}

func TestRunServerHandshakeLoop_ContextCancelled(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunServerHandshakeLoop(ctx, signaler, peer, nil)
	assert.Error(t, err)
}

func TestRunClientHandshakeLoop_Candidate(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	candidatePayload, _ := json.Marshal(webrtc.ICECandidateInit{Candidate: "test-candidate"})
	signaler.pushMessage(SignalingMessage{Type: "candidate", Payload: candidatePayload})

	_ = RunClientHandshakeLoop(ctx, signaler, peer, nil)
	assert.Len(t, peer.iceCandidates, 1)
}

func TestRunClientHandshakeLoop_CandidateDone(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	var loggedMsg string
	logger := func(msg string, args ...interface{}) {
		loggedMsg = fmt.Sprintf(msg, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	signaler.pushMessage(SignalingMessage{Type: "candidate_done", Payload: []byte("{}")})

	_ = RunClientHandshakeLoop(ctx, signaler, peer, logger)
	assert.Equal(t, "Remote ICE gathering complete", loggedMsg)
}

func TestRunClientHandshakeLoop_ControlStop(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	var loggedMsg string
	logger := func(msg string, args ...interface{}) {
		loggedMsg = fmt.Sprintf(msg, args...)
	}

	ctx := context.Background()

	controlPayload, _ := json.Marshal(map[string]string{"action": "stop"})
	signaler.pushMessage(SignalingMessage{Type: "control", Payload: controlPayload})

	err := RunClientHandshakeLoop(ctx, signaler, peer, logger)
	assert.NoError(t, err)
	assert.Equal(t, "Server requested stop", loggedMsg)
}

func TestRunClientHandshakeLoop_ContextCancelled(t *testing.T) {
	signaler := newMockSignaler()
	peer := &mockPeerManager{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunClientHandshakeLoop(ctx, signaler, peer, nil)
	assert.Error(t, err)
}

func TestEncryptAESGCM_Success(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := []byte("test message to encrypt")

	nonce, ciphertext, err := encryptAESGCM(key, plaintext)
	require.NoError(t, err)
	assert.Len(t, nonce, 12)
	assert.NotEmpty(t, ciphertext)
	assert.NotEqual(t, plaintext, ciphertext)
}

func TestDecryptAESGCM_Success(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := []byte("test message to encrypt")

	nonce, ciphertext, err := encryptAESGCM(key, plaintext)
	require.NoError(t, err)

	decrypted, err := decryptAESGCM(key, nonce, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptAESGCM_InvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	nonce := make([]byte, 12)
	ciphertext := []byte("invalid ciphertext")

	_, err := decryptAESGCM(key, nonce, ciphertext)
	assert.Error(t, err)
}

func TestDecryptAESGCM_InvalidKey(t *testing.T) {
	key := make([]byte, 16)
	nonce := make([]byte, 12)
	ciphertext := make([]byte, 32)

	_, err := decryptAESGCM(key, nonce, ciphertext)
	assert.Error(t, err)
}

func TestParseOfferSDP_Success(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"sdp": "v=0..."})

	sdp, err := parseOfferSDP(payload)
	require.NoError(t, err)
	assert.Equal(t, "v=0...", sdp.SDP)
}

func TestParseOfferSDP_InvalidJSON(t *testing.T) {
	payload := []byte("invalid json")

	_, err := parseOfferSDP(payload)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse offer SDP")
}

func TestServerSignalingProtocol_HandleHandshakeMessage_AnswerInvalidJSON(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	msg := SignalingMessage{Type: "answer", Payload: []byte("invalid json")}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse answer SDP")
}

func TestServerSignalingProtocol_HandleHandshakeMessage_CandidateInvalidJSON(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	msg := SignalingMessage{Type: "candidate", Payload: []byte("invalid json")}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.NoError(t, err)
}

func TestServerSignalingProtocol_HandleHandshakeMessage_SetRemoteDescError(t *testing.T) {
	protocol := NewServerSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{remoteDescErr: fmt.Errorf("remote desc error")}

	answerPayload, _ := json.Marshal(map[string]string{"sdp": "test-sdp"})
	msg := SignalingMessage{Type: "answer", Payload: answerPayload}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "set remote description")
}

func TestClientSignalingProtocol_HandleHandshakeMessage_CandidateInvalidJSON(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	peer := &mockPeerManager{}

	msg := SignalingMessage{Type: "candidate", Payload: []byte("invalid json")}

	err := protocol.handleHandshakeMessage(msg, peer)
	assert.NoError(t, err)
}

func TestClientSignalingProtocol_HandleOffer_CreateAnswerError(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()
	peer := &mockPeerManager{answerErr: fmt.Errorf("answer error")}

	offerPayload, _ := json.Marshal(map[string]string{"sdp": "test-offer-sdp"})
	msg := SignalingMessage{Type: "offer", Payload: offerPayload}

	err := protocol.handleOffer(msg, signaler, peer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create answer")
}

type mockEncryptedAuthHandler struct {
	mockAuthHandler
	key []byte
}

func (m *mockEncryptedAuthHandler) EncryptionKey() []byte {
	return m.key
}

func TestServerSignalingProtocol_HandleAuth_WithEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	auth := &mockEncryptedAuthHandler{key: key}
	protocol := NewServerSignalingProtocol(auth)
	signaler := newMockSignaler()

	ctx := context.Background()
	err := protocol.HandleAuth(ctx, signaler, "password")
	require.NoError(t, err)
	assert.NotNil(t, protocol.encryptor)
	assert.NotNil(t, protocol.decryptor)
}

func TestClientSignalingProtocol_HandleAuth_WithEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	auth := &mockEncryptedAuthHandler{key: key}
	protocol := NewClientSignalingProtocol(auth)
	signaler := newMockSignaler()

	ctx := context.Background()
	err := protocol.HandleAuth(ctx, signaler, "password")
	require.NoError(t, err)
	assert.NotNil(t, protocol.encryptor)
	assert.NotNil(t, protocol.decryptor)
}

func TestClientSignalingProtocol_DecryptPayload(t *testing.T) {
	protocol := NewClientSignalingProtocol(&mockAuthHandler{})

	plaintext := json.RawMessage(`{"sdp":"test"}`)
	result := protocol.decryptPayload(plaintext)
	assert.JSONEq(t, string(plaintext), string(result))
}

func TestClientSignalingProtocol_DecryptPayload_WithEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	protocol := NewClientSignalingProtocol(&mockEncryptedAuthHandler{key: key})
	signaler := newMockSignaler()

	ctx := context.Background()
	_ = protocol.HandleAuth(ctx, signaler, "password")

	plaintext := []byte(`{"sdp":"test"}`)
	nonce, ciphertext, err := encryptAESGCM(key, plaintext)
	require.NoError(t, err)

	encPayload := encryptedPayload{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	encData, _ := json.Marshal(encPayload)

	result := protocol.decryptPayload(encData)
	assert.JSONEq(t, string(plaintext), string(result))
}

// ── ClientAuthMeta Exchange Tests ───────────────────────────────────────────

// TestClientMetaExchange verifies that SendClientMeta / ReceiveClientMeta
// round-trip correctly through a mock signaler.
func TestClientMetaExchange(t *testing.T) {
	// client side — uses ClientSignalingProtocol.SendClientMeta
	clientProto := NewClientSignalingProtocol(&mockAuthHandler{})
	// server side — uses ServerSignalingProtocol.ReceiveClientMeta
	serverProto := NewServerSignalingProtocol(&mockAuthHandler{})

	// A pair of mock signalers wired together: client sends → server receives.
	clientSig := newMockSignaler()
	serverSig := newMockSignaler()

	meta := ClientAuthMeta{
		Nickname:  "Alice",
		HWID:      "abc",
		SessionID: "uuid-1",
	}

	// Client sends meta; message lands in clientSig.sendCh.
	require.NoError(t, clientProto.SendClientMeta(clientSig, meta))

	// Wire the sent message into the server's receive channel.
	select {
	case msg := <-clientSig.sendCh:
		serverSig.pushMessage(msg)
	default:
		t.Fatal("expected client to send a message")
	}

	// Server receives.
	ctx := context.Background()
	received, err := serverProto.ReceiveClientMeta(ctx, serverSig)
	require.NoError(t, err)
	assert.Equal(t, "Alice", received.Nickname)
	assert.Equal(t, "abc", received.HWID)
	assert.Equal(t, "uuid-1", received.SessionID)
}

// TestReceiveClientMeta_WrongType verifies that ReceiveClientMeta returns an
// error when the message type is not "client_meta".
func TestReceiveClientMeta_WrongType(t *testing.T) {
	serverProto := NewServerSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	// Push a message with the wrong type.
	wrongPayload, _ := json.Marshal(ClientAuthMeta{Nickname: "Hacker"})
	signaler.pushMessage(SignalingMessage{Type: "wrong", Payload: wrongPayload})

	ctx := context.Background()
	_, err := serverProto.ReceiveClientMeta(ctx, signaler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected client_meta")
}

// TestReceiveClientMeta_Timeout verifies that ReceiveClientMeta returns a
// context error when no message arrives before the deadline.
func TestReceiveClientMeta_Timeout(t *testing.T) {
	serverProto := NewServerSignalingProtocol(&mockAuthHandler{})
	signaler := newMockSignaler()

	// Use a context that is already canceled so the select hits Done immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := serverProto.ReceiveClientMeta(ctx, signaler)
	assert.Error(t, err)
}
