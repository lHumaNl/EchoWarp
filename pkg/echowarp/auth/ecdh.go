package auth

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// KeyExchangePayload represents an X25519 public key exchange message.
type KeyExchangePayload struct {
	PublicKey string `json:"public_key"`
}

// EncryptedPayload represents an AES-GCM encrypted message.
type EncryptedPayload struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// ECDHAuth implements password-authenticated key exchange using X25519 and AES-256-GCM.
// It provides both authentication and encryption for signaling messages.
//
// Security properties:
//   - Forward secrecy: New X25519 keypair per session
//   - No password transmission: Only HMAC of challenge is sent (after encryption)
//   - Authenticated encryption: AES-256-GCM with random nonces
//   - Key derivation: HKDF-SHA256 from X25519 shared secret
//
// Protocol flow:
//  1. Client sends X25519 public key
//  2. Server responds with its X25519 public key
//  3. Both compute shared secret and derive AES-256 key
//  4. Server sends encrypted auth_challenge (if password required)
//  5. Client responds with encrypted auth_response
//  6. Server sends encrypted auth_result
//  7. All subsequent messages are encrypted
//
// Thread-safe: Internal state protected by sync.RWMutex.
type ECDHAuth struct {
	password      string
	encryptionKey []byte
	mu            sync.RWMutex
}

// NewECDHAuth creates a new ECDH-based authenticator with optional password.
// If password is empty, only key exchange is performed (no HMAC verification).
func NewECDHAuth(password string) *ECDHAuth {
	return &ECDHAuth{password: password}
}

func (a *ECDHAuth) receiveKeyExchange(recv transport.AuthRecvFunc, side string) (*ecdh.PublicKey, error) {
	msgType, payload, err := recv()
	if err != nil {
		return nil, fmt.Errorf("recv key_exchange: %w", err)
	}
	if msgType != "key_exchange" {
		return nil, fmt.Errorf("expected key_exchange, got %s: %w", msgType, ErrKeyExchangeFailed)
	}
	var kx KeyExchangePayload
	err = json.Unmarshal(payload, &kx)
	if err != nil {
		return nil, fmt.Errorf("unmarshal key_exchange: %w", err)
	}
	pub, err := DecodePublicKey(kx.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode %s public key: %w", side, err)
	}
	return pub, nil
}

func (a *ECDHAuth) performServerKeyExchange(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
) (*ecdh.PrivateKey, []byte, []byte, error) {
	clientPub, err := a.receiveKeyExchange(recv, "client")
	if err != nil {
		return nil, nil, nil, err
	}

	privateKey, publicKey, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	publicKeyBase64 := EncodePublicKey(publicKey)
	err = send("key_exchange", KeyExchangePayload{PublicKey: publicKeyBase64})
	if err != nil {
		zeroBytes(privateKey.Bytes())
		return nil, nil, nil, fmt.Errorf("send key_exchange: %w", err)
	}

	sharedSecret, err := ComputeSharedSecret(privateKey, clientPub)
	if err != nil {
		zeroBytes(privateKey.Bytes())
		return nil, nil, nil, fmt.Errorf("compute shared secret: %w", err)
	}

	aesKey, err := DeriveAESKey(sharedSecret, nil, "")
	if err != nil {
		zeroBytes(privateKey.Bytes())
		zeroBytes(sharedSecret)
		return nil, nil, nil, fmt.Errorf("derive AES key: %w", err)
	}

	a.mu.Lock()
	a.encryptionKey = aesKey
	a.mu.Unlock()

	return privateKey, sharedSecret, aesKey, nil
}

func (a *ECDHAuth) verifyChallengeHMAC(password, nonce, responseHMAC string) error {
	key, err := deriveKey(password)
	if err != nil {
		return fmt.Errorf("derive HMAC key: %w", err)
	}
	defer zeroBytes(key)

	if !verifyHMAC(key, nonce, responseHMAC) {
		return ErrAuthFailed
	}
	return nil
}

func (a *ECDHAuth) recvAuthResponse(recv transport.AuthRecvFunc, aesKey []byte) (AuthResponsePayload, error) {
	msgType, payload, err := a.recvAndDecrypt(recv, aesKey)
	if err != nil {
		return AuthResponsePayload{}, fmt.Errorf("recv encrypted response: %w", err)
	}
	if msgType != "auth_response" {
		return AuthResponsePayload{}, fmt.Errorf("expected auth_response, got %s", msgType)
	}

	var response AuthResponsePayload
	err = json.Unmarshal(payload, &response)
	if err != nil {
		return AuthResponsePayload{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return response, nil
}

func (a *ECDHAuth) runEncryptedChallenge(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	aesKey []byte,
	password string,
) error {
	nonce, err := generateNonce()
	if err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	err = a.encryptAndSend(send, aesKey, "auth_challenge", AuthChallengePayload{
		Nonce:   nonce,
		Version: Version,
	})
	if err != nil {
		return fmt.Errorf("send encrypted challenge: %w", err)
	}

	response, err := a.recvAuthResponse(recv, aesKey)
	if err != nil {
		return err
	}

	if err := checkVersionCompat(response.Version); err != nil {
		_ = a.encryptAndSend(send, aesKey, "auth_result", AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: fmt.Sprintf("Version mismatch: %v", err),
			Code:    426,
		})
		return err
	}

	if err := a.verifyChallengeHMAC(password, nonce, response.HMAC); err != nil {
		_ = a.encryptAndSend(send, aesKey, "auth_result", AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: "Invalid password",
			Code:    401,
		})
		return err
	}

	return a.encryptAndSend(send, aesKey, "auth_result", AuthResultPayload{
		Success: true,
		Message: "Authenticated",
	})
}

func (a *ECDHAuth) AuthenticateServer(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	password string,
) error {
	if len(password) > maxPasswordLength {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxPasswordLength)
	}

	privateKey, sharedSecret, aesKey, err := a.performServerKeyExchange(send, recv)
	if err != nil {
		return fmt.Errorf("ecdh server: %w", err)
	}
	defer zeroBytes(privateKey.Bytes())
	defer zeroBytes(sharedSecret)

	if password == "" {
		return a.encryptAndSend(send, aesKey, "auth_result", AuthResultPayload{
			Success: true,
			Message: "ECDH authentication successful",
		})
	}

	if err := a.runEncryptedChallenge(send, recv, aesKey, password); err != nil {
		return fmt.Errorf("ecdh server: %w", err)
	}

	return nil
}

func (a *ECDHAuth) performClientKeyExchange(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
) (*ecdh.PrivateKey, []byte, []byte, error) {
	privateKey, publicKey, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate keypair: %w", err)
	}

	publicKeyBase64 := EncodePublicKey(publicKey)
	err = send("key_exchange", KeyExchangePayload{PublicKey: publicKeyBase64})
	if err != nil {
		zeroBytes(privateKey.Bytes())
		return nil, nil, nil, fmt.Errorf("send key_exchange: %w", err)
	}

	serverPub, err := a.receiveKeyExchange(recv, "server")
	if err != nil {
		zeroBytes(privateKey.Bytes())
		return nil, nil, nil, err
	}

	sharedSecret, err := ComputeSharedSecret(privateKey, serverPub)
	if err != nil {
		zeroBytes(privateKey.Bytes())
		return nil, nil, nil, fmt.Errorf("compute shared secret: %w", err)
	}

	aesKey, err := DeriveAESKey(sharedSecret, nil, "")
	if err != nil {
		zeroBytes(privateKey.Bytes())
		zeroBytes(sharedSecret)
		return nil, nil, nil, fmt.Errorf("derive AES key: %w", err)
	}

	a.mu.Lock()
	a.encryptionKey = aesKey
	a.mu.Unlock()

	return privateKey, sharedSecret, aesKey, nil
}

func (a *ECDHAuth) handleEncryptedChallenge(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	aesKey []byte,
	password string,
) error {
	msgType, payload, err := a.recvAndDecrypt(recv, aesKey)
	if err != nil {
		return fmt.Errorf("recv encrypted message: %w", err)
	}

	if msgType == "auth_result" {
		return a.handleAuthResultNoChallenge(payload)
	}

	if msgType != "auth_challenge" {
		return fmt.Errorf("expected auth_challenge, got %s", msgType)
	}

	var challenge AuthChallengePayload
	err = json.Unmarshal(payload, &challenge)
	if err != nil {
		return fmt.Errorf("unmarshal challenge: %w", err)
	}

	err = checkVersionCompat(challenge.Version)
	if err != nil {
		return fmt.Errorf("ecdh client: %w", err)
	}

	key, err := deriveKey(password)
	if err != nil {
		return fmt.Errorf("derive HMAC key: %w", err)
	}
	defer zeroBytes(key)
	hmacHex := computeHMAC(key, challenge.Nonce)

	err = a.encryptAndSend(send, aesKey, "auth_response", AuthResponsePayload{
		HMAC:    hmacHex,
		Version: Version,
	})
	if err != nil {
		return fmt.Errorf("send encrypted response: %w", err)
	}

	return a.recvAndVerifyResult(recv, aesKey)
}

func (a *ECDHAuth) handleAuthResultNoChallenge(payload []byte) error {
	var result AuthResultPayload
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if result.Success {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrAuthFailed, result.Message)
}

func (a *ECDHAuth) recvAndVerifyResult(recv transport.AuthRecvFunc, aesKey []byte) error {
	resultMsgType, resultPayload, err := a.recvAndDecrypt(recv, aesKey)
	if err != nil {
		return fmt.Errorf("recv encrypted result: %w", err)
	}
	if resultMsgType != "auth_result" {
		return fmt.Errorf("expected auth_result, got %s", resultMsgType)
	}

	var result AuthResultPayload
	if err := json.Unmarshal(resultPayload, &result); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("%w: %s", ErrAuthFailed, result.Message)
	}

	return nil
}

func (a *ECDHAuth) AuthenticateClient(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	password string,
) error {
	if len(password) > maxPasswordLength {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxPasswordLength)
	}

	privateKey, sharedSecret, aesKey, err := a.performClientKeyExchange(send, recv)
	if err != nil {
		return fmt.Errorf("ecdh client: %w", err)
	}
	defer zeroBytes(privateKey.Bytes())
	defer zeroBytes(sharedSecret)

	if err := a.handleEncryptedChallenge(send, recv, aesKey, password); err != nil {
		return fmt.Errorf("ecdh client: %w", err)
	}

	return nil
}

func (a *ECDHAuth) encryptAndSend(
	send transport.AuthSendFunc,
	aesKey []byte,
	msgType string,
	payload interface{},
) error {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	nonce, ciphertext, err := EncryptAESGCM(aesKey, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	encryptedPayload := EncryptedPayload{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}

	wrapper := struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}{
		Type:    msgType,
		Payload: mustMarshal(encryptedPayload),
	}

	return send("encrypted", wrapper)
}

func (a *ECDHAuth) recvAndDecrypt(
	recv transport.AuthRecvFunc,
	aesKey []byte,
) (string, []byte, error) {
	msgType, payload, err := recv()
	if err != nil {
		return "", nil, fmt.Errorf("recv: %w", err)
	}

	if msgType != "encrypted" {
		return "", nil, fmt.Errorf("expected encrypted message, got %s: %w", msgType, ErrDecryptionFailed)
	}

	var wrapper struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	err = json.Unmarshal(payload, &wrapper)
	if err != nil {
		return "", nil, fmt.Errorf("unmarshal wrapper: %w", err)
	}

	var encryptedPayload EncryptedPayload
	err = json.Unmarshal(wrapper.Payload, &encryptedPayload)
	if err != nil {
		return "", nil, fmt.Errorf("unmarshal encrypted payload: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(encryptedPayload.Nonce)
	if err != nil {
		return "", nil, fmt.Errorf("decode nonce: %w: %w", err, ErrDecryptionFailed)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPayload.Ciphertext)
	if err != nil {
		return "", nil, fmt.Errorf("decode ciphertext: %w: %w", err, ErrDecryptionFailed)
	}

	plaintext, err := DecryptAESGCM(aesKey, nonce, ciphertext)
	if err != nil {
		return "", nil, fmt.Errorf("decrypt: %w: %w", err, ErrDecryptionFailed)
	}

	return wrapper.Type, plaintext, nil
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v) //nolint:errcheck
	return data
}

func (a *ECDHAuth) EncryptionKey() []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.encryptionKey
}

// EncryptMessage encrypts a message using the session's AES-256-GCM key.
// Returns error if authentication has not completed.
func (a *ECDHAuth) EncryptMessage(plaintext []byte) (nonce, ciphertext []byte, err error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.encryptionKey == nil {
		return nil, nil, fmt.Errorf("encrypt message: authentication not completed")
	}

	return EncryptAESGCM(a.encryptionKey, plaintext)
}

// DecryptMessage decrypts a message using the session's AES-256-GCM key.
// Returns error if authentication has not completed.
func (a *ECDHAuth) DecryptMessage(nonce, ciphertext []byte) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.encryptionKey == nil {
		return nil, fmt.Errorf("decrypt message: authentication not completed")
	}

	return DecryptAESGCM(a.encryptionKey, nonce, ciphertext)
}

var (
	_ Authenticator          = (*ECDHAuth)(nil)
	_ EncryptedAuthenticator = (*ECDHAuth)(nil)
)
