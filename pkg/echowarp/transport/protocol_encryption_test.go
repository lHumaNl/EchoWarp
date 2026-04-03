package transport

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestEncryptDecryptAESGCM(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("test message for encryption")

	nonce, ciphertext, err := encryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("encryptAESGCM failed: %v", err)
	}

	if len(nonce) != 12 {
		t.Errorf("expected nonce length 12, got %d", len(nonce))
	}

	if len(ciphertext) == 0 {
		t.Error("ciphertext should not be empty")
	}

	decrypted, err := decryptAESGCM(key, nonce, ciphertext)
	if err != nil {
		t.Fatalf("decryptAESGCM failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted text doesn't match: got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptedPayloadJSON(t *testing.T) {
	nonce := []byte("123456789012")
	ciphertext := []byte("encrypted data here")

	payload := encryptedPayload{
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal encryptedPayload: %v", err)
	}

	var unmarshaled encryptedPayload
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal encryptedPayload: %v", err)
	}

	decodedNonce, _ := base64.StdEncoding.DecodeString(unmarshaled.Nonce)
	decodedCiphertext, _ := base64.StdEncoding.DecodeString(unmarshaled.Ciphertext)

	if !bytes.Equal(decodedNonce, nonce) {
		t.Errorf("nonce mismatch: got %s, want %s", decodedNonce, nonce)
	}

	if !bytes.Equal(decodedCiphertext, ciphertext) {
		t.Errorf("ciphertext mismatch: got %s, want %s", decodedCiphertext, ciphertext)
	}
}

func TestDecryptWithInvalidKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("test message")

	nonce, ciphertext, err := encryptAESGCM(key, plaintext)
	if err != nil {
		t.Fatalf("encryptAESGCM failed: %v", err)
	}

	wrongKey := make([]byte, 32)
	for i := range wrongKey {
		wrongKey[i] = byte(i + 1)
	}

	_, err = decryptAESGCM(wrongKey, nonce, ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}
