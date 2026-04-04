package transport

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pion/webrtc/v4"
)

// encryptedAuthenticator is an internal interface for authenticators that provide encryption.
type encryptedAuthenticator interface {
	EncryptionKey() []byte
}

const defaultAuthTimeout = 30 * time.Second

// SignalingProtocol defines the interface for authentication and WebRTC handshake.
type SignalingProtocol interface {
	HandleAuth(ctx context.Context, signaler Signaler, password string) error
	HandleHandshake(ctx context.Context, signaler Signaler, peer PeerManager) error
}

// AuthSendFunc is a function type for sending authentication messages.
type AuthSendFunc func(msgType string, payload interface{}) error

// AuthRecvFunc is a function type for receiving authentication messages.
type AuthRecvFunc func() (string, []byte, error)

// AuthHandler defines the interface for authentication handlers.
type AuthHandler interface {
	AuthenticateServer(send AuthSendFunc, recv AuthRecvFunc, password string) error
	AuthenticateClient(send AuthSendFunc, recv AuthRecvFunc, password string) error
}

// ServerSignalingProtocol implements the signaling protocol for the server side.
// It handles authentication and the SDP/ICE exchange, with optional AES-GCM encryption.
type ServerSignalingProtocol struct {
	auth      AuthHandler
	encryptor func([]byte) (nonce, ciphertext []byte, err error)
	decryptor func(nonce, ciphertext []byte) ([]byte, error)
}

// ClientAuthMeta carries client identity metadata sent during the auth handshake.
// These fields are optional and used for chat nicknames, session reconnect, and HWID-based bans.
type ClientAuthMeta struct {
	Nickname  string `json:"nickname,omitempty"`   // desired display name
	HWID      string `json:"hwid,omitempty"`       // hardware identifier (only if server requires it)
	SessionID string `json:"session_id,omitempty"` // UUID from previous session (for reconnect)
}

// AuthResultMessage represents the authentication result sent to the client.
type AuthResultMessage struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Code      int             `json:"code,omitempty"`
	SessionID string          `json:"session_id,omitempty"` // UUID assigned by server for this session
	Nickname  string          `json:"nickname,omitempty"`   // Server-assigned display name for chat
	Config    json.RawMessage `json:"config,omitempty"`
}

// NewServerSignalingProtocol creates a new server-side signaling protocol.
func NewServerSignalingProtocol(auth AuthHandler) *ServerSignalingProtocol {
	return &ServerSignalingProtocol{auth: auth}
}

// HandleAuth performs the server-side authentication handshake.
// If the authenticator provides encryption, it enables encrypted messaging for subsequent operations.
func (p *ServerSignalingProtocol) HandleAuth(ctx context.Context, signaler Signaler, password string) error {
	authCtx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	sendFn := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return signaler.Send(SignalingMessage{Type: msgType, Payload: data})
	}

	recvFn := func() (string, []byte, error) {
		select {
		case msg := <-signaler.Receive():
			return msg.Type, msg.Payload, nil
		case <-authCtx.Done():
			return "", nil, authCtx.Err()
		}
	}

	if err := p.auth.AuthenticateServer(sendFn, recvFn, password); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	if encAuth, ok := p.auth.(encryptedAuthenticator); ok {
		if key := encAuth.EncryptionKey(); key != nil {
			p.encryptor = func(plaintext []byte) (nonce, ciphertext []byte, err error) {
				return encryptAESGCM(key, plaintext)
			}
			p.decryptor = func(nonce, ciphertext []byte) ([]byte, error) {
				return decryptAESGCM(key, nonce, ciphertext)
			}
		}
	}

	return nil
}

// ReceiveClientMeta waits for a "client_meta" message from the client.
// Returns nil meta (not error) if the client is an older version that doesn't send client_meta;
// the caller should fall back to empty defaults.
func (p *ServerSignalingProtocol) ReceiveClientMeta(ctx context.Context, signaler Signaler) (*ClientAuthMeta, error) {
	metaCtx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	select {
	case msg := <-signaler.Receive():
		if msg.Type != "client_meta" {
			return nil, fmt.Errorf("expected client_meta, got %s", msg.Type)
		}
		var meta ClientAuthMeta
		if err := json.Unmarshal(msg.Payload, &meta); err != nil {
			return nil, fmt.Errorf("invalid client_meta: %w", err)
		}
		return &meta, nil
	case <-metaCtx.Done():
		return nil, metaCtx.Err()
	}
}

// SendAuthResult sends the authentication result to the client.
func (p *ServerSignalingProtocol) SendAuthResult(signaler Signaler, result AuthResultMessage) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return signaler.Send(SignalingMessage{Type: "auth_result", Payload: payload})
}

// HandleHandshake performs the SDP/ICE exchange with the client.
// Creates an offer and handles the answer and ICE candidates.
func (p *ServerSignalingProtocol) HandleHandshake(ctx context.Context, signaler Signaler, peer PeerManager) error {
	p.setupICECandidateHandler(signaler, peer)

	offer, err := peer.CreateOffer()
	if err != nil {
		return fmt.Errorf("create offer: %w", err)
	}

	offerPayload, _ := json.Marshal(map[string]string{"sdp": offer.SDP}) //nolint:errcheck
	if err := p.sendOffer(signaler, offerPayload); err != nil {
		return err
	}

	return p.handleHandshakeLoop(ctx, signaler, peer)
}

func (p *ServerSignalingProtocol) setupICECandidateHandler(signaler Signaler, peer PeerManager) {
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			payload, _ := json.Marshal(struct{}{})                                        //nolint:errcheck
			_ = signaler.Send(SignalingMessage{Type: "candidate_done", Payload: payload}) //nolint:errcheck
			return
		}
		payload, _ := json.Marshal(candidate.ToJSON())                           //nolint:errcheck
		_ = signaler.Send(SignalingMessage{Type: "candidate", Payload: payload}) //nolint:errcheck
	})
}

func (p *ServerSignalingProtocol) sendOffer(signaler Signaler, offerPayload json.RawMessage) error {
	if p.encryptor != nil {
		nonce, ciphertext, err := p.encryptor(offerPayload)
		if err != nil {
			return fmt.Errorf("encrypt offer: %w", err)
		}
		ep := encryptedPayload{
			Nonce:      base64.StdEncoding.EncodeToString(nonce),
			Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		}
		encryptedData, _ := json.Marshal(ep) //nolint:errcheck
		if err := signaler.Send(SignalingMessage{Type: "offer", Payload: encryptedData}); err != nil {
			return fmt.Errorf("send offer: %w", err)
		}
	} else {
		if err := signaler.Send(SignalingMessage{Type: "offer", Payload: offerPayload}); err != nil {
			return fmt.Errorf("send offer: %w", err)
		}
	}
	return nil
}

func (p *ServerSignalingProtocol) handleHandshakeLoop(ctx context.Context, signaler Signaler, peer PeerManager) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-signaler.Receive():
			if err := p.handleHandshakeMessage(msg, peer); err != nil {
				return err
			}
			if msg.Type == "answer" {
				return nil
			}
		}
	}
}

func (p *ServerSignalingProtocol) handleHandshakeMessage(msg SignalingMessage, peer PeerManager) error {
	payload := msg.Payload

	if p.decryptor != nil && msg.Type != "candidate_done" {
		var ep encryptedPayload
		if err := json.Unmarshal(payload, &ep); err == nil && ep.Nonce != "" && ep.Ciphertext != "" {
			nonce, err1 := base64.StdEncoding.DecodeString(ep.Nonce)
			ciphertext, err2 := base64.StdEncoding.DecodeString(ep.Ciphertext)
			if err1 != nil || err2 != nil {
				return fmt.Errorf("decrypt message: base64 decode failed")
			}
			decrypted, err := p.decryptor(nonce, ciphertext)
			if err != nil {
				return fmt.Errorf("decrypt message: %w", err)
			}
			payload = decrypted
		}
	}

	switch msg.Type {
	case "answer":
		var sdpMsg struct {
			SDP string `json:"sdp"`
		}
		if err := json.Unmarshal(payload, &sdpMsg); err != nil {
			return fmt.Errorf("parse answer SDP: %w", err)
		}
		answer := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  sdpMsg.SDP,
		}
		if err := peer.SetRemoteDescription(answer); err != nil {
			return fmt.Errorf("set remote description: %w", err)
		}

	case "candidate":
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal(payload, &candidate); err != nil {
			return nil
		}
		_ = peer.AddICECandidate(candidate) //nolint:errcheck
	}

	return nil
}

// DecryptPayload decrypts an AES-GCM encrypted payload if a decryptor is configured.
// Returns the original payload unchanged if no encryption is active or the payload is not encrypted.
// Used by the server's post-handshake signaling loop to decrypt incoming messages.
func (p *ServerSignalingProtocol) DecryptPayload(payload json.RawMessage) json.RawMessage {
	if p.decryptor == nil {
		return payload
	}
	var ep encryptedPayload
	if err := json.Unmarshal(payload, &ep); err != nil || ep.Nonce == "" || ep.Ciphertext == "" {
		return payload
	}
	nonce, err1 := base64.StdEncoding.DecodeString(ep.Nonce)
	ciphertext, err2 := base64.StdEncoding.DecodeString(ep.Ciphertext)
	if err1 != nil || err2 != nil {
		return payload
	}
	decrypted, err := p.decryptor(nonce, ciphertext)
	if err != nil {
		return payload
	}
	return decrypted
}

// ClientSignalingProtocol implements the signaling protocol for the client side.
type ClientSignalingProtocol struct {
	auth      AuthHandler
	encryptor func([]byte) (nonce, ciphertext []byte, err error)
	decryptor func(nonce, ciphertext []byte) ([]byte, error)
}

// NewClientSignalingProtocol creates a new client-side signaling protocol.
func NewClientSignalingProtocol(auth AuthHandler) *ClientSignalingProtocol {
	return &ClientSignalingProtocol{auth: auth}
}

// HandleAuth performs the client-side authentication handshake.
func (p *ClientSignalingProtocol) HandleAuth(ctx context.Context, signaler Signaler, password string) error {
	authCtx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	sendFn := func(msgType string, payload interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return signaler.Send(SignalingMessage{Type: msgType, Payload: data})
	}

	recvFn := func() (string, []byte, error) {
		select {
		case msg := <-signaler.Receive():
			return msg.Type, msg.Payload, nil
		case <-authCtx.Done():
			return "", nil, authCtx.Err()
		}
	}

	if err := p.auth.AuthenticateClient(sendFn, recvFn, password); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	if encAuth, ok := p.auth.(encryptedAuthenticator); ok {
		if key := encAuth.EncryptionKey(); key != nil {
			p.encryptor = func(plaintext []byte) (nonce, ciphertext []byte, err error) {
				return encryptAESGCM(key, plaintext)
			}
			p.decryptor = func(nonce, ciphertext []byte) ([]byte, error) {
				return decryptAESGCM(key, nonce, ciphertext)
			}
		}
	}

	return nil
}

// SendClientMeta sends the client's identity metadata to the server.
func (p *ClientSignalingProtocol) SendClientMeta(signaler Signaler, meta ClientAuthMeta) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return signaler.Send(SignalingMessage{Type: "client_meta", Payload: payload})
}

// WaitForAuthResult waits for and returns the authentication result from the server.
func (p *ClientSignalingProtocol) WaitForAuthResult(signaler Signaler) (*AuthResultMessage, error) {
	timer := time.NewTimer(defaultAuthTimeout)
	defer timer.Stop()

	select {
	case msg := <-signaler.Receive():
		if msg.Type != "auth_result" {
			return nil, fmt.Errorf("expected auth_result, got %s", msg.Type)
		}
		var result AuthResultMessage
		if err := json.Unmarshal(msg.Payload, &result); err != nil {
			return nil, fmt.Errorf("parse auth result: %w", err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("timeout waiting for auth result")
	}
}

// HandleHandshake performs the SDP/ICE exchange with the server.
// Waits for the offer, creates an answer, and handles ICE candidates.
func (p *ClientSignalingProtocol) HandleHandshake(ctx context.Context, signaler Signaler, peer PeerManager) error {
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			payload, _ := json.Marshal(struct{}{})                                        //nolint:errcheck
			_ = signaler.Send(SignalingMessage{Type: "candidate_done", Payload: payload}) //nolint:errcheck
			return
		}
		payload, _ := json.Marshal(candidate.ToJSON())                           //nolint:errcheck
		_ = signaler.Send(SignalingMessage{Type: "candidate", Payload: payload}) //nolint:errcheck
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-signaler.Receive():
			if msg.Type == "offer" {
				return p.handleOffer(msg, signaler, peer)
			}
			if msg.Type == "auth_result" {
				var result AuthResultMessage
				if err := json.Unmarshal(msg.Payload, &result); err == nil && !result.Success {
					return fmt.Errorf("server rejected connection: %s", result.Message)
				}
				continue
			}
			if err := p.handleHandshakeMessage(msg, peer); err != nil {
				return err
			}
		}
	}
}

func (p *ClientSignalingProtocol) handleOffer(msg SignalingMessage, signaler Signaler, peer PeerManager) error {
	payload := p.decryptPayload(msg.Payload)

	sdpMsg, err := parseOfferSDP(payload)
	if err != nil {
		return err
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpMsg.SDP,
	}

	answer, err := peer.CreateAnswer(offer)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}

	answerPayload, _ := json.Marshal(map[string]string{"sdp": answer.SDP}) //nolint:errcheck
	return p.sendAnswer(signaler, answerPayload)
}

func (p *ClientSignalingProtocol) decryptPayload(payload json.RawMessage) json.RawMessage {
	if p.decryptor != nil {
		var ep encryptedPayload
		if err := json.Unmarshal(payload, &ep); err == nil && ep.Nonce != "" && ep.Ciphertext != "" {
			nonce, err1 := base64.StdEncoding.DecodeString(ep.Nonce)
			ciphertext, err2 := base64.StdEncoding.DecodeString(ep.Ciphertext)
			if err1 != nil || err2 != nil {
				return payload
			}
			decrypted, err := p.decryptor(nonce, ciphertext)
			if err == nil {
				return decrypted
			}
		}
	}
	return payload
}

func parseOfferSDP(payload json.RawMessage) (*struct {
	SDP string `json:"sdp"`
}, error) {
	var sdpMsg struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(payload, &sdpMsg); err != nil {
		return nil, fmt.Errorf("parse offer SDP: %w", err)
	}
	return &sdpMsg, nil
}

func (p *ClientSignalingProtocol) sendAnswer(signaler Signaler, answerPayload json.RawMessage) error {
	if p.encryptor != nil {
		nonce, ciphertext, err := p.encryptor(answerPayload)
		if err != nil {
			return fmt.Errorf("encrypt answer: %w", err)
		}
		ep := encryptedPayload{
			Nonce:      base64.StdEncoding.EncodeToString(nonce),
			Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		}
		encryptedData, _ := json.Marshal(ep) //nolint:errcheck
		if err := signaler.Send(SignalingMessage{Type: "answer", Payload: encryptedData}); err != nil {
			return fmt.Errorf("send answer: %w", err)
		}
	} else {
		if err := signaler.Send(SignalingMessage{Type: "answer", Payload: answerPayload}); err != nil {
			return fmt.Errorf("send answer: %w", err)
		}
	}
	return nil
}

func (p *ClientSignalingProtocol) handleHandshakeMessage(msg SignalingMessage, peer PeerManager) error {
	payload := msg.Payload

	if p.decryptor != nil && msg.Type != "candidate_done" {
		var ep encryptedPayload
		if err := json.Unmarshal(payload, &ep); err == nil && ep.Nonce != "" && ep.Ciphertext != "" {
			nonce, err1 := base64.StdEncoding.DecodeString(ep.Nonce)
			ciphertext, err2 := base64.StdEncoding.DecodeString(ep.Ciphertext)
			if err1 != nil || err2 != nil {
				return fmt.Errorf("decrypt message: base64 decode failed")
			}
			decrypted, err := p.decryptor(nonce, ciphertext)
			if err != nil {
				return fmt.Errorf("decrypt message: %w", err)
			}
			payload = decrypted
		}
	}

	if msg.Type == "candidate" {
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal(payload, &candidate); err != nil {
			return nil
		}
		_ = peer.AddICECandidate(candidate) //nolint:errcheck
	}

	return nil
}

// HandshakeLoopHandler is a function type for handling post-handshake signaling messages.
type HandshakeLoopHandler func(ctx context.Context, signaler Signaler, onMessage func(msg SignalingMessage) bool) error

// RunServerHandshakeLoop runs the post-handshake message loop for the server.
// Handles incoming ICE candidates and control messages.
func RunServerHandshakeLoop(ctx context.Context, signaler Signaler, peer PeerManager, logger func(msg string, args ...interface{})) error {
	return runHandshakeLoop(ctx, signaler, peer, logger, "Client")
}

// RunClientHandshakeLoop runs the post-handshake message loop for the client.
// Handles incoming ICE candidates and control messages.
func RunClientHandshakeLoop(ctx context.Context, signaler Signaler, peer PeerManager, logger func(msg string, args ...interface{})) error {
	return runHandshakeLoop(ctx, signaler, peer, logger, "Server")
}

// runHandshakeLoop is the shared implementation for post-handshake message loops.
// role parameter specifies the remote peer role for logging ("Client" or "Server").
func runHandshakeLoop(ctx context.Context, signaler Signaler, peer PeerManager, logger func(msg string, args ...interface{}), role string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-signaler.Receive():
			switch msg.Type {
			case "candidate":
				var candidate webrtc.ICECandidateInit
				if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
					continue
				}
				_ = peer.AddICECandidate(candidate) //nolint:errcheck

			case "candidate_done":
				if logger != nil {
					logger("Remote ICE gathering complete")
				}

			case "control":
				var ctrl struct {
					Action string `json:"action"`
				}
				if err := json.Unmarshal(msg.Payload, &ctrl); err != nil {
					continue
				}
				if ctrl.Action == "stop" {
					if logger != nil {
						logger("%s requested stop", role)
					}
					return nil
				}
			}
		}
	}
}

// encryptedPayload represents an AES-GCM encrypted message with nonce.
type encryptedPayload struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func encryptAESGCM(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce = make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func decryptAESGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}
