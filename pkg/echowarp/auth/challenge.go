package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

const (
	nonceSize         = 32
	keySize           = 32
	hkdfSalt          = "echowarp-auth-v2"
	hkdfInfo          = "hmac-key"
	maxPasswordLength = 256
)

// ChallengeAuth implements HMAC-SHA256 challenge-response authentication.
// It provides secure password verification without transmitting the password.
//
// Security properties:
//   - Password never transmitted: Only HMAC of challenge is sent
//   - Key derivation: HKDF-SHA256 derives keys from passwords
//   - Constant-time comparison: Prevents timing attacks
//   - Forward secrecy: Random nonce per session
//
// Note: This authenticator does not encrypt messages. Use with TLS or ECDHAuth.
type ChallengeAuth struct{}

// NewChallengeAuth creates a new HMAC challenge-response authenticator.
func NewChallengeAuth() *ChallengeAuth {
	return &ChallengeAuth{}
}

// AuthChallengePayload represents the server's authentication challenge.
type AuthChallengePayload struct {
	Nonce   string `json:"nonce"`
	Version string `json:"version"`
}

// AuthResponsePayload represents the client's authentication response.
type AuthResponsePayload struct {
	HMAC    string `json:"hmac"`
	Version string `json:"version"`
	Reverse bool   `json:"reverse"`
}

// AuthResultPayload represents the authentication result.
type AuthResultPayload struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Code    int             `json:"code,omitempty"`
	Config  json.RawMessage `json:"config,omitempty"`
}

func verifyChallengeResponse(key []byte, nonce, responseHMAC string) error {
	if !verifyHMAC(key, nonce, responseHMAC) {
		return ErrAuthFailed
	}
	return nil
}

func recvAuthResponse(recv transport.AuthRecvFunc) (AuthResponsePayload, error) {
	msgType, payload, err := recv()
	if err != nil {
		return AuthResponsePayload{}, fmt.Errorf("recv response: %w", err)
	}
	if msgType != "auth_response" {
		return AuthResponsePayload{}, fmt.Errorf("expected auth_response, got %s", msgType)
	}

	var response AuthResponsePayload
	if err := json.Unmarshal(payload, &response); err != nil {
		return AuthResponsePayload{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return response, nil
}

func sendChallengeAndVerify(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	password string,
) error {
	nonce, err := generateNonce()
	if err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	err = send("auth_challenge", AuthChallengePayload{
		Nonce:   nonce,
		Version: Version,
	})
	if err != nil {
		return fmt.Errorf("send challenge: %w", err)
	}

	response, err := recvAuthResponse(recv)
	if err != nil {
		return err
	}

	err = checkVersionCompat(response.Version)
	if err != nil {
		_ = send("auth_result", AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: fmt.Sprintf("Version mismatch: %v", err),
			Code:    426,
		})
		return err
	}

	key, err := deriveKey(password)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}
	defer zeroBytes(key)

	err = verifyChallengeResponse(key, nonce, response.HMAC)
	if err != nil {
		_ = send("auth_result", AuthResultPayload{ //nolint:errcheck
			Success: false,
			Message: "Invalid password",
			Code:    401,
		})
		return err
	}

	err = send("auth_result", AuthResultPayload{
		Success: true,
		Message: "Authenticated",
	})
	if err != nil {
		return fmt.Errorf("send result: %w", err)
	}

	return nil
}

func (a *ChallengeAuth) AuthenticateServer(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	password string,
) error {
	if len(password) > maxPasswordLength {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxPasswordLength)
	}

	if password == "" {
		return send("auth_result", AuthResultPayload{
			Success: true,
			Message: "No authentication required",
		})
	}

	if err := sendChallengeAndVerify(send, recv, password); err != nil {
		return fmt.Errorf("auth server: %w", err)
	}

	return nil
}

func handleNoAuthResult(payload []byte) error {
	var result AuthResultPayload
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if result.Success {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrAuthFailed, result.Message)
}

func computeAndSendResponse(send transport.AuthSendFunc, key []byte, nonce string) error {
	hmacHex := computeHMAC(key, nonce)
	err := send("auth_response", AuthResponsePayload{
		HMAC:    hmacHex,
		Version: Version,
	})
	if err != nil {
		return fmt.Errorf("send response: %w", err)
	}
	return nil
}

func recvAndVerifyResult(recv transport.AuthRecvFunc) error {
	msgType, payload, err := recv()
	if err != nil {
		return fmt.Errorf("recv result: %w", err)
	}
	if msgType != "auth_result" {
		return fmt.Errorf("expected auth_result, got %s", msgType)
	}

	var result AuthResultPayload
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("unmarshal result: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("%w: %s", ErrAuthFailed, result.Message)
	}

	return nil
}

func (a *ChallengeAuth) AuthenticateClient(
	send transport.AuthSendFunc,
	recv transport.AuthRecvFunc,
	password string,
) error {
	if len(password) > maxPasswordLength {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxPasswordLength)
	}

	msgType, payload, err := recv()
	if err != nil {
		return fmt.Errorf("recv: %w", err)
	}

	if msgType == "auth_result" {
		err = handleNoAuthResult(payload)
		if err != nil {
			return fmt.Errorf("auth client: %w", err)
		}
		return nil
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
		return fmt.Errorf("auth client: %w", err)
	}

	key, err := deriveKey(password)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}
	defer zeroBytes(key)

	if err := computeAndSendResponse(send, key, challenge.Nonce); err != nil {
		return fmt.Errorf("auth client: %w", err)
	}

	if err := recvAndVerifyResult(recv); err != nil {
		return fmt.Errorf("auth client: %w", err)
	}

	return nil
}

func generateNonce() (string, error) {
	b := make([]byte, nonceSize)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func deriveKey(password string) ([]byte, error) {
	if len(password) > maxPasswordLength {
		return nil, fmt.Errorf("password too long (max %d bytes)", maxPasswordLength)
	}

	hkdfReader := hkdf.New(sha256.New, []byte(password), []byte(hkdfSalt), []byte(hkdfInfo))
	key := make([]byte, keySize)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("HKDF key derivation failed: %w", err)
	}
	return key, nil
}

func computeHMAC(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyHMAC(key []byte, message, expectedHex string) bool {
	expected, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hmac.Equal(mac.Sum(nil), expected)
}
