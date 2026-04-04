package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockECDHTransport struct {
	mu        sync.Mutex
	sendQueue []struct {
		msgType string
		payload []byte
	}
	recvQueue []struct {
		msgType string
		payload []byte
	}
	readIdx int
	cond    *sync.Cond
}

func newMockECDHTransport() *mockECDHTransport {
	m := &mockECDHTransport{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *mockECDHTransport) send(msgType string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m.sendQueue = append(m.sendQueue, struct {
		msgType string
		payload []byte
	}{msgType, data})
	m.cond.Broadcast()
	return nil
}

func (m *mockECDHTransport) recv() (string, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// First check recvQueue (for manually pushed messages)
	if m.readIdx < len(m.recvQueue) {
		msg := m.recvQueue[m.readIdx]
		m.readIdx++
		return msg.msgType, msg.payload, nil
	}

	// Then wait for messages in sendQueue
	for m.readIdx >= len(m.sendQueue) {
		m.cond.Wait()
	}

	msg := m.sendQueue[m.readIdx]
	m.readIdx++
	return msg.msgType, msg.payload, nil
}

func (m *mockECDHTransport) pushRecv(msgType string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m.recvQueue = append(m.recvQueue, struct {
		msgType string
		payload []byte
	}{msgType, data})
	return nil
}

func (m *mockECDHTransport) popSend() (string, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sendQueue) == 0 {
		return "", nil, fmt.Errorf("no messages to send")
	}
	msg := m.sendQueue[0]
	m.sendQueue = m.sendQueue[1:]
	return msg.msgType, msg.payload, nil
}

func (m *mockECDHTransport) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendQueue = nil
	m.recvQueue = nil
	m.readIdx = 0
}

func TestECDHAuth_KeyExchange_BothSidesDeriveSameKey(t *testing.T) {
	clientPrivate, clientPublic, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	serverPrivate, serverPublic, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	clientShared, err := ComputeSharedSecret(clientPrivate, serverPublic)
	require.NoError(t, err)

	serverShared, err := ComputeSharedSecret(serverPrivate, clientPublic)
	require.NoError(t, err)

	assert.Equal(t, clientShared, serverShared, "both sides should derive the same shared secret")

	clientKey, err := DeriveAESKey(clientShared, nil, "")
	require.NoError(t, err)

	serverKey, err := DeriveAESKey(serverShared, nil, "")
	require.NoError(t, err)

	assert.Equal(t, clientKey, serverKey, "both sides should derive the same AES key")
	assert.Len(t, clientKey, 32, "AES key should be 32 bytes")
}

func TestECDHAuth_CorrectPassword_Succeeds(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	var serverErr, clientErr error

	go func() {
		defer wg.Done()
		clientErr = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "secret123")
	}()

	go func() {
		defer wg.Done()
		serverErr = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "secret123")
	}()

	wg.Wait()

	require.NoError(t, clientErr, "client authentication should succeed")
	require.NoError(t, serverErr, "server authentication should succeed")

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey)
	require.NotNil(t, clientKey)
	assert.Equal(t, serverKey, clientKey, "both sides should have the same encryption key")
}

func TestECDHAuth_WrongPassword_Fails(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	var serverErr, clientErr error

	go func() {
		defer wg.Done()
		clientErr = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "wrong_password")
	}()

	go func() {
		defer wg.Done()
		serverErr = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "correct_password")
	}()

	wg.Wait()

	require.Error(t, serverErr, "server should fail with wrong password")
	assert.ErrorIs(t, serverErr, ErrAuthFailed)
	require.Error(t, clientErr, "client should receive auth failure")
	assert.ErrorIs(t, clientErr, ErrAuthFailed)
}

func TestECDHAuth_EmptyPassword_Succeeds(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	var serverErr, clientErr error

	go func() {
		defer wg.Done()
		clientErr = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		serverErr = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	require.NoError(t, clientErr, "client authentication should succeed with empty password")
	require.NoError(t, serverErr, "server authentication should succeed with empty password")

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey)
	require.NotNil(t, clientKey)
	assert.Equal(t, serverKey, clientKey, "both sides should have the same encryption key")
}

func TestECDHAuth_EncryptionKey_AfterAuth(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey)
	require.NotNil(t, clientKey)
	assert.Len(t, serverKey, 32)
	assert.Len(t, clientKey, 32)
	assert.Equal(t, serverKey, clientKey)
}

func TestECDHAuth_EncryptionKey_BeforeAuth_Nil(t *testing.T) {
	auth := NewECDHAuth("")

	key := auth.EncryptionKey()
	assert.Nil(t, key, "encryption key should be nil before authentication")
}

func TestECDHAuth_EncryptDecrypt_Roundtrip(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	plaintext := []byte("Hello, ECDH encrypted world!")

	nonce, ciphertext, err := serverAuth.EncryptMessage(plaintext)
	require.NoError(t, err)
	require.NotNil(t, nonce)
	require.NotNil(t, ciphertext)
	assert.Len(t, nonce, 12, "nonce should be 12 bytes")

	decrypted, err := clientAuth.DecryptMessage(nonce, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	nonce2, ciphertext2, err := clientAuth.EncryptMessage(plaintext)
	require.NoError(t, err)

	decrypted2, err := serverAuth.DecryptMessage(nonce2, ciphertext2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)
}

func TestECDHAuth_EncryptMessage_BeforeAuth_Error(t *testing.T) {
	auth := NewECDHAuth("")

	_, _, err := auth.EncryptMessage([]byte("test"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication not completed")
}

func TestECDHAuth_DecryptMessage_BeforeAuth_Error(t *testing.T) {
	auth := NewECDHAuth("")

	_, err := auth.DecryptMessage([]byte("nonce12345678"), []byte("ciphertext"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication not completed")
}

func TestECDHAuth_Server_InvalidKeyExchange_Error(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	err := transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: "invalid_base64!"})
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode client public key")
}

func TestECDHAuth_Client_InvalidKeyExchange_Error(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	recvInvalidKeyExchange := func() (string, []byte, error) {
		return "key_exchange", []byte(`{"public_key": "not_valid_base64!!!"}`), nil
	}

	err := auth.AuthenticateClient(transport.send, recvInvalidKeyExchange, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode server public key")
}

func TestECDHAuth_Server_WrongMessageType_Error(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	err := transport.pushRecv("wrong_message_type", KeyExchangePayload{PublicKey: "test"})
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key_exchange")
	assert.Contains(t, err.Error(), "wrong_message_type")
}

func TestECDHAuth_PasswordTooLong_Error(t *testing.T) {
	auth := NewECDHAuth("")
	longPassword := string(make([]byte, 257))

	err := auth.AuthenticateServer(nil, nil, longPassword)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")

	err = auth.AuthenticateClient(nil, nil, longPassword)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestECDHAuth_FullProtocolFlow_Success(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	var serverErr, clientErr error
	var serverDone, clientDone bool

	go func() {
		defer wg.Done()
		serverErr = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "test_password")
		serverDone = true
	}()

	go func() {
		defer wg.Done()
		clientErr = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "test_password")
		clientDone = true
	}()

	wg.Wait()

	assert.True(t, serverDone, "server should complete")
	assert.True(t, clientDone, "client should complete")
	require.NoError(t, serverErr, "server authentication should succeed")
	require.NoError(t, clientErr, "client authentication should succeed")

	serverKey := serverAuth.EncryptionKey()
	clientKey := clientAuth.EncryptionKey()

	require.NotNil(t, serverKey)
	require.NotNil(t, clientKey)
	assert.Equal(t, serverKey, clientKey)

	testMessage := []byte("Test message after full protocol flow")
	nonce, ciphertext, err := serverAuth.EncryptMessage(testMessage)
	require.NoError(t, err)

	decrypted, err := clientAuth.DecryptMessage(nonce, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, testMessage, decrypted)
}

func TestECDHAuth_EncryptDecrypt_MultipleMessages(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	messages := [][]byte{
		[]byte("First message"),
		[]byte("Second message with more content"),
		[]byte("Third"),
		[]byte(""),
		[]byte("Last message with special chars: !@#$%^&*()"),
	}

	for _, msg := range messages {
		nonce, ciphertext, err := serverAuth.EncryptMessage(msg)
		require.NoError(t, err)

		decrypted, err := clientAuth.DecryptMessage(nonce, ciphertext)
		require.NoError(t, err)
		assert.True(t, bytes.Equal(msg, decrypted), "message should match after decryption")
	}
}

func TestECDHAuth_EncryptDecrypt_LargeMessage(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	largeMessage := make([]byte, 64*1024)
	for i := range largeMessage {
		largeMessage[i] = byte(i % 256)
	}

	nonce, ciphertext, err := serverAuth.EncryptMessage(largeMessage)
	require.NoError(t, err)

	decrypted, err := clientAuth.DecryptMessage(nonce, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, largeMessage, decrypted)
}

func TestECDHAuth_Server_RecvError(t *testing.T) {
	auth := NewECDHAuth("")

	recvErr := func() (string, []byte, error) {
		return "", nil, fmt.Errorf("network error")
	}

	err := auth.AuthenticateServer(nil, recvErr, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv key_exchange")
	assert.Contains(t, err.Error(), "network error")
}

func TestECDHAuth_Client_RecvError(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	recvErr := func() (string, []byte, error) {
		return "", nil, fmt.Errorf("connection timeout")
	}

	err := auth.AuthenticateClient(transport.send, recvErr, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv key_exchange")
	assert.Contains(t, err.Error(), "connection timeout")
}

func TestECDHAuth_Client_SendError(t *testing.T) {
	auth := NewECDHAuth("")

	sendErr := func(msgType string, payload interface{}) error {
		return fmt.Errorf("send failed")
	}

	err := auth.AuthenticateClient(sendErr, nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send key_exchange")
	assert.Contains(t, err.Error(), "send failed")
}

func TestECDHAuth_Server_SendError(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	privateKey, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	pubKeyBase64 := EncodePublicKey(publicKey)

	err = transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: pubKeyBase64})
	require.NoError(t, err)

	sendErr := func(msgType string, payload interface{}) error {
		return fmt.Errorf("send failed")
	}

	err = auth.AuthenticateServer(sendErr, transport.recv, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send key_exchange")

	_ = privateKey
}

func TestECDHAuth_Client_RecvWrongMessageTypeAfterKeyExchange(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	callCount := 0
	recvWrongType := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			_, publicKey, err := GenerateX25519KeyPair()
			if err != nil {
				return "", nil, err
			}
			pubKeyBase64 := EncodePublicKey(publicKey)
			return "key_exchange", []byte(fmt.Sprintf(`{"public_key":"%s"}`, pubKeyBase64)), nil
		}
		return "wrong_type", []byte("{}"), nil
	}

	err := auth.AuthenticateClient(transport.send, recvWrongType, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected encrypted message")
	assert.Contains(t, err.Error(), "wrong_type")
}

func TestECDHAuth_EncryptDecrypt_WrongKey(t *testing.T) {
	auth1 := NewECDHAuth("")
	auth2 := NewECDHAuth("")

	serverToClient1 := newMockECDHTransport()
	clientToServer1 := newMockECDHTransport()

	serverToClient2 := newMockECDHTransport()
	clientToServer2 := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		_ = auth1.AuthenticateServer(serverToClient1.send, clientToServer1.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = auth1.AuthenticateClient(clientToServer1.send, serverToClient1.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = auth2.AuthenticateServer(serverToClient2.send, clientToServer2.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = auth2.AuthenticateClient(clientToServer2.send, serverToClient2.recv, "")
	}()

	wg.Wait()

	plaintext := []byte("Secret message")
	nonce, ciphertext, err := auth1.EncryptMessage(plaintext)
	require.NoError(t, err)

	_, err = auth2.DecryptMessage(nonce, ciphertext)
	require.Error(t, err, "decryption with wrong key should fail")
}

func TestECDHAuth_DecryptMessage_TamperedNonce(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	plaintext := []byte("Secret message")
	nonce, ciphertext, err := serverAuth.EncryptMessage(plaintext)
	require.NoError(t, err)

	tamperedNonce := make([]byte, len(nonce))
	copy(tamperedNonce, nonce)
	tamperedNonce[0] ^= 0xFF

	_, err = clientAuth.DecryptMessage(tamperedNonce, ciphertext)
	require.Error(t, err, "decryption with tampered nonce should fail")
}

func TestECDHAuth_DecryptMessage_TamperedCiphertext(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	plaintext := []byte("Secret message")
	nonce, ciphertext, err := serverAuth.EncryptMessage(plaintext)
	require.NoError(t, err)

	if len(ciphertext) > 0 {
		tamperedCiphertext := make([]byte, len(ciphertext))
		copy(tamperedCiphertext, ciphertext)
		tamperedCiphertext[0] ^= 0xFF

		_, err = clientAuth.DecryptMessage(nonce, tamperedCiphertext)
		require.Error(t, err, "decryption with tampered ciphertext should fail")
	}
}

func TestECDHAuth_EncryptedMessage_Wrapper(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "password123")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "password123")
	}()

	wg.Wait()

	require.NotNil(t, serverAuth.EncryptionKey(), "server should have encryption key")
	require.NotNil(t, clientAuth.EncryptionKey(), "client should have encryption key")
}

func TestECDHAuth_MultipleAuthentications(t *testing.T) {
	for i := 0; i < 5; i++ {
		serverAuth := NewECDHAuth("")
		clientAuth := NewECDHAuth("")

		serverToClient := newMockECDHTransport()
		clientToServer := newMockECDHTransport()

		var wg sync.WaitGroup
		wg.Add(2)

		var serverErr, clientErr error

		go func() {
			defer wg.Done()
			clientErr = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
		}()

		go func() {
			defer wg.Done()
			serverErr = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
		}()

		wg.Wait()

		require.NoError(t, serverErr, "server auth %d should succeed", i)
		require.NoError(t, clientErr, "client auth %d should succeed", i)

		serverKey := serverAuth.EncryptionKey()
		clientKey := clientAuth.EncryptionKey()

		require.NotNil(t, serverKey)
		require.NotNil(t, clientKey)
		assert.Equal(t, serverKey, clientKey, "keys should match for iteration %d", i)
	}
}

func TestECDHAuth_KeyExchangePayload_InvalidJSON(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	err := transport.pushRecv("key_exchange", "not a valid json object")
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestECDHAuth_Client_RecvEncryptedError(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	callCount := 0
	recvErr := func() (string, []byte, error) {
		callCount++
		if callCount == 1 {
			_, publicKey, err := GenerateX25519KeyPair()
			if err != nil {
				return "", nil, err
			}
			pubKeyBase64 := EncodePublicKey(publicKey)
			return "key_exchange", []byte(fmt.Sprintf(`{"public_key":"%s"}`, pubKeyBase64)), nil
		}
		return "", nil, fmt.Errorf("connection lost")
	}

	err := auth.AuthenticateClient(transport.send, recvErr, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recv encrypted message")
}

func TestECDHAuth_RecvAndDecrypt_WrongMessageType(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	_, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	pubKeyBase64 := EncodePublicKey(publicKey)

	err = transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: pubKeyBase64})
	require.NoError(t, err)

	err = transport.pushRecv("wrong_type", map[string]interface{}{"data": "test"})
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected encrypted message")
}

func TestECDHAuth_RecvAndDecrypt_InvalidWrapperJSON(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	_, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	pubKeyBase64 := EncodePublicKey(publicKey)

	err = transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: pubKeyBase64})
	require.NoError(t, err)

	err = transport.pushRecv("encrypted", "not valid json")
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal wrapper")
}

func TestECDHAuth_RecvAndDecrypt_InvalidNonce(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	_, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	pubKeyBase64 := EncodePublicKey(publicKey)

	err = transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: pubKeyBase64})
	require.NoError(t, err)

	invalidNoncePayload := map[string]interface{}{
		"type": "auth_challenge",
		"payload": map[string]interface{}{
			"nonce":      "not_valid_base64!!!",
			"ciphertext": base64.StdEncoding.EncodeToString([]byte("test")),
		},
	}
	err = transport.pushRecv("encrypted", invalidNoncePayload)
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode nonce")
}

func TestECDHAuth_RecvAndDecrypt_InvalidCiphertext(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	_, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	pubKeyBase64 := EncodePublicKey(publicKey)

	err = transport.pushRecv("key_exchange", KeyExchangePayload{PublicKey: pubKeyBase64})
	require.NoError(t, err)

	invalidCiphertextPayload := map[string]interface{}{
		"type": "auth_challenge",
		"payload": map[string]interface{}{
			"nonce":      base64.StdEncoding.EncodeToString([]byte("123456789012")),
			"ciphertext": "not_valid_base64!!!",
		},
	}
	err = transport.pushRecv("encrypted", invalidCiphertextPayload)
	require.NoError(t, err)

	err = auth.AuthenticateServer(transport.send, transport.recv, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode ciphertext")
}

func TestECDHAuth_EncryptAndSend_MarshalError(t *testing.T) {
	auth := NewECDHAuth("")

	err := auth.encryptAndSend(nil, []byte("32-byte-key-12345678901234567890"), "test", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
}

func TestECDHAuth_EncryptAndSend_EncryptError(t *testing.T) {
	auth := NewECDHAuth("")
	transport := newMockECDHTransport()

	err := auth.encryptAndSend(transport.send, []byte("short"), "test", map[string]string{"key": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encrypt")
}

func TestECDHAuth_EncryptAndSend_SendError(t *testing.T) {
	auth := NewECDHAuth("")

	sendErr := func(msgType string, payload interface{}) error {
		return fmt.Errorf("network failure")
	}

	err := auth.encryptAndSend(sendErr, []byte("32-byte-key-12345678901234567890"), "test", map[string]string{"key": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network failure")
}

func TestECDHAuth_DecryptMessage_InvalidNonceSize(t *testing.T) {
	serverAuth := NewECDHAuth("")
	clientAuth := NewECDHAuth("")

	serverToClient := newMockECDHTransport()
	clientToServer := newMockECDHTransport()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = clientAuth.AuthenticateClient(clientToServer.send, serverToClient.recv, "")
	}()

	go func() {
		defer wg.Done()
		_ = serverAuth.AuthenticateServer(serverToClient.send, clientToServer.recv, "")
	}()

	wg.Wait()

	_, err := clientAuth.DecryptMessage([]byte("short"), []byte("ciphertext"))
	require.Error(t, err)
}

func TestECDHAuth_EncryptMessage_InvalidKeySize(t *testing.T) {
	auth := NewECDHAuth("")
	auth.mu.Lock()
	auth.encryptionKey = []byte("short")
	auth.mu.Unlock()

	_, _, err := auth.EncryptMessage([]byte("test"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")
}

func TestECDHAuth_NewECDHAuth(t *testing.T) {
	auth := NewECDHAuth("test_password")
	require.NotNil(t, auth)
	assert.Equal(t, "test_password", auth.password)
}

func TestECDHAuth_MustMarshal(t *testing.T) {
	result := mustMarshal(map[string]string{"key": "value"})
	require.NotNil(t, result)
	assert.Contains(t, string(result), "key")
	assert.Contains(t, string(result), "value")
}
