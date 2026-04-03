package auth

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSignaling struct {
	mu       sync.Mutex
	messages []struct {
		msgType string
		payload []byte
	}
	readIdx int
}

func newMockSignaling() *mockSignaling {
	return &mockSignaling{}
}

func (m *mockSignaling) send(msgType string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m.messages = append(m.messages, struct {
		msgType string
		payload []byte
	}{msgType, data})
	return nil
}

func (m *mockSignaling) recv() (string, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readIdx >= len(m.messages) {
		return "", nil, fmt.Errorf("no more messages")
	}
	msg := m.messages[m.readIdx]
	m.readIdx++
	return msg.msgType, msg.payload, nil
}

func TestChallengeAuth_CorrectPassword_Succeeds(t *testing.T) {
	t.Parallel()
	auth := NewChallengeAuth()

	serverSent := newMockSignaling()
	clientSent := newMockSignaling()

	_ = auth.AuthenticateServer(serverSent.send, clientSent.recv, "secret123")

	nonce, err := generateNonce()
	require.NoError(t, err)
	assert.Len(t, nonce, 64)

	key, err := deriveKey("secret123")
	require.NoError(t, err)
	assert.Len(t, key, 32)

	hmac1 := computeHMAC(key, nonce)
	assert.NotEmpty(t, hmac1)

	assert.True(t, verifyHMAC(key, nonce, hmac1))
}

func TestChallengeAuth_WrongPassword_Fails(t *testing.T) {
	t.Parallel()
	nonce, err := generateNonce()
	require.NoError(t, err)

	serverKey, err := deriveKey("correct_password")
	require.NoError(t, err)
	clientKey, err := deriveKey("wrong_password")
	require.NoError(t, err)

	clientHMAC := computeHMAC(clientKey, nonce)
	assert.False(t, verifyHMAC(serverKey, nonce, clientHMAC))
}

func TestChallengeAuth_EmptyPassword_SkipsAuth(t *testing.T) {
	t.Parallel()
	key, err := deriveKey("")
	require.NoError(t, err)
	assert.Len(t, key, 32)
}

func TestChallengeAuth_NonceIsRandom(t *testing.T) {
	t.Parallel()
	nonce1, err := generateNonce()
	require.NoError(t, err)
	nonce2, err := generateNonce()
	require.NoError(t, err)

	assert.NotEqual(t, nonce1, nonce2, "two nonces should be different")
}

func TestChallengeAuth_ConstantTimeComparison(t *testing.T) {
	t.Parallel()
	nonce, _ := generateNonce()
	key, err := deriveKey("password")
	require.NoError(t, err)
	hmacVal := computeHMAC(key, nonce)

	assert.True(t, verifyHMAC(key, nonce, hmacVal))

	tampered := hmacVal[:len(hmacVal)-1] + "0"
	if tampered == hmacVal {
		tampered = hmacVal[:len(hmacVal)-1] + "1"
	}
	assert.False(t, verifyHMAC(key, nonce, tampered))
}

func TestChallengeAuth_DeriveKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		password   string
		wantErr    bool
		errContain string
		checkFunc  func(t *testing.T, key []byte)
	}{
		{
			"deterministic",
			"same_password",
			false,
			"",
			func(t *testing.T, key []byte) {
				key2, err := deriveKey("same_password")
				require.NoError(t, err)
				assert.Equal(t, key, key2, "same password should derive same key")
			},
		},
		{
			"different passwords",
			"password1",
			false,
			"",
			func(t *testing.T, key []byte) {
				key2, err := deriveKey("password2")
				require.NoError(t, err)
				assert.NotEqual(t, key, key2, "different passwords should derive different keys")
			},
		},
		{
			"empty password",
			"",
			false,
			"",
			func(t *testing.T, key []byte) {
				assert.Len(t, key, 32)
			},
		},
		{
			"password too long",
			string(make([]byte, 257)),
			true,
			"password too long",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := deriveKey(tt.password)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, key, 32)
				if tt.checkFunc != nil {
					tt.checkFunc(t, key)
				}
			}
		})
	}
}

func TestChallengeAuth_Authenticate_PasswordTooLong(t *testing.T) {
	t.Parallel()
	auth := NewChallengeAuth()
	longPassword := string(make([]byte, 257))

	tests := []struct {
		name     string
		auth     *ChallengeAuth
		isServer bool
	}{
		{"authenticate server", auth, true},
		{"authenticate client", auth, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.isServer {
				err = tt.auth.AuthenticateServer(nil, nil, longPassword)
			} else {
				err = tt.auth.AuthenticateClient(nil, nil, longPassword)
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "exceeds maximum length")
		})
	}
}

func TestChallengeAuth_AuthenticateServer_EmptyPassword_SendsResult(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	err := auth.AuthenticateServer(signaling.send, nil, "")
	require.NoError(t, err)

	signaling.mu.Lock()
	require.Len(t, signaling.messages, 1)
	assert.Equal(t, "auth_result", signaling.messages[0].msgType)
	var result AuthResultPayload
	require.NoError(t, json.Unmarshal(signaling.messages[0].payload, &result))
	assert.True(t, result.Success)
	assert.Equal(t, "No authentication required", result.Message)
	signaling.mu.Unlock()
}

func TestChallengeAuth_AuthenticateServer_SendChallengeError(t *testing.T) {
	auth := NewChallengeAuth()

	sendErr := func(msgType string, payload interface{}) error {
		return fmt.Errorf("network error")
	}

	err := auth.AuthenticateServer(sendErr, nil, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send challenge")
	assert.Contains(t, err.Error(), "network error")
}

func TestChallengeAuth_AuthenticateServer_RecvResponseError(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	recvErr := func() (string, []byte, error) {
		return "", nil, fmt.Errorf("connection lost")
	}

	err := auth.AuthenticateServer(signaling.send, recvErr, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv response")
	assert.Contains(t, err.Error(), "connection lost")
}

func TestChallengeAuth_AuthenticateServer_WrongMessageType(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	recvWrongType := func() (string, []byte, error) {
		return "wrong_type", []byte("{}"), nil
	}

	err := auth.AuthenticateServer(signaling.send, recvWrongType, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected auth_response")
	assert.Contains(t, err.Error(), "wrong_type")
}

func TestChallengeAuth_AuthenticateServer_UnmarshalResponseError(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	recvBadJSON := func() (string, []byte, error) {
		return "auth_response", []byte("invalid json"), nil
	}

	err := auth.AuthenticateServer(signaling.send, recvBadJSON, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestChallengeAuth_AuthenticateServer_InvalidHMAC(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	recvBadHMAC := func() (string, []byte, error) {
		return "auth_response", []byte(`{"hmac": "invalid_hex!", "version": "1.0.0"}`), nil
	}

	err := auth.AuthenticateServer(signaling.send, recvBadHMAC, "password")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthFailed)
}

func TestChallengeAuth_VerifyHMAC_InvalidHex(t *testing.T) {
	t.Parallel()
	key := []byte("test_key_32_bytes_long_enough!")
	result := verifyHMAC(key, "message", "not_valid_hex!")
	assert.False(t, result)
}

func TestChallengeAuth_VerifyHMAC_EmptyHex(t *testing.T) {
	t.Parallel()
	key := []byte("test_key_32_bytes_long_enough!")
	result := verifyHMAC(key, "message", "")
	assert.False(t, result)
}

func TestChallengeAuth_ComputeHMAC_Consistent(t *testing.T) {
	t.Parallel()
	key := []byte("test_key_32_bytes_long_enough!")
	message := "test_message"

	hmac1 := computeHMAC(key, message)
	hmac2 := computeHMAC(key, message)

	assert.Equal(t, hmac1, hmac2)
	assert.Len(t, hmac1, 64)
}

func TestChallengeAuth_Errors(t *testing.T) {
	t.Parallel()
	assert.Error(t, ErrAuthFailed)
	assert.Error(t, ErrAuthTimeout)
	assert.Error(t, ErrVersionMismatch)
}

func TestChallengeAuth_AuthenticateClient_RecvError(t *testing.T) {
	auth := NewChallengeAuth()

	recvErr := func() (string, []byte, error) {
		return "", nil, fmt.Errorf("network timeout")
	}

	err := auth.AuthenticateClient(nil, recvErr, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv")
	assert.Contains(t, err.Error(), "network timeout")
}

func TestChallengeAuth_AuthenticateClient_AuthResultNoAuthRequired(t *testing.T) {
	auth := NewChallengeAuth()

	recvSuccessResult := func() (string, []byte, error) {
		return "auth_result", []byte(`{"success": true, "message": "No auth required"}`), nil
	}

	err := auth.AuthenticateClient(nil, recvSuccessResult, "password")
	require.NoError(t, err)
}

func TestChallengeAuth_AuthenticateClient_AuthResultFailed(t *testing.T) {
	auth := NewChallengeAuth()

	recvFailedResult := func() (string, []byte, error) {
		return "auth_result", []byte(`{"success": false, "message": "Access denied"}`), nil
	}

	err := auth.AuthenticateClient(nil, recvFailedResult, "password")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthFailed)
	assert.Contains(t, err.Error(), "Access denied")
}

func TestChallengeAuth_AuthenticateClient_AuthResultUnmarshalError(t *testing.T) {
	auth := NewChallengeAuth()

	recvBadResult := func() (string, []byte, error) {
		return "auth_result", []byte("not json"), nil
	}

	err := auth.AuthenticateClient(nil, recvBadResult, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal result")
}

func TestChallengeAuth_AuthenticateClient_WrongMessageType(t *testing.T) {
	auth := NewChallengeAuth()

	recvWrongType := func() (string, []byte, error) {
		return "unknown_type", []byte("{}"), nil
	}

	err := auth.AuthenticateClient(nil, recvWrongType, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected auth_challenge")
	assert.Contains(t, err.Error(), "unknown_type")
}

func TestChallengeAuth_AuthenticateClient_UnmarshalChallengeError(t *testing.T) {
	auth := NewChallengeAuth()

	recvBadChallenge := func() (string, []byte, error) {
		return "auth_challenge", []byte("bad json"), nil
	}

	err := auth.AuthenticateClient(nil, recvBadChallenge, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal challenge")
}

func TestChallengeAuth_AuthenticateClient_SendResponseError(t *testing.T) {
	auth := NewChallengeAuth()

	sendErr := func(msgType string, payload interface{}) error {
		return fmt.Errorf("send failed")
	}

	recvChallenge := func() (string, []byte, error) {
		return "auth_challenge", []byte(`{"nonce": "abc123", "version": "1.0.0"}`), nil
	}

	err := auth.AuthenticateClient(sendErr, recvChallenge, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send response")
}

func TestChallengeAuth_AuthenticateClient_RecvResultError(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	callCount := 0
	recvThenErr := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			return "auth_challenge", []byte(`{"nonce": "abc123", "version": "1.0.0"}`), nil
		}
		return "", nil, fmt.Errorf("connection lost")
	}

	err := auth.AuthenticateClient(signaling.send, recvThenErr, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv result")
}

func TestChallengeAuth_AuthenticateClient_ResultWrongMessageType(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	callCount := 0
	recvWrongResultType := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			return "auth_challenge", []byte(`{"nonce": "abc123", "version": "1.0.0"}`), nil
		}
		return "wrong_type", []byte("{}"), nil
	}

	err := auth.AuthenticateClient(signaling.send, recvWrongResultType, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected auth_result")
	assert.Contains(t, err.Error(), "wrong_type")
}

func TestChallengeAuth_AuthenticateClient_ResultUnmarshalError(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	callCount := 0
	recvBadResult := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			return "auth_challenge", []byte(`{"nonce": "abc123", "version": "1.0.0"}`), nil
		}
		return "auth_result", []byte("bad json"), nil
	}

	err := auth.AuthenticateClient(signaling.send, recvBadResult, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal result")
}

func TestChallengeAuth_AuthenticateClient_ResultFailure(t *testing.T) {
	auth := NewChallengeAuth()
	signaling := newMockSignaling()

	callCount := 0
	recvFailedResult := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			return "auth_challenge", []byte(`{"nonce": "abc123", "version": "1.0.0"}`), nil
		}
		return "auth_result", []byte(`{"success": false, "message": "Invalid password"}`), nil
	}

	err := auth.AuthenticateClient(signaling.send, recvFailedResult, "password")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthFailed)
	assert.Contains(t, err.Error(), "Invalid password")
}
