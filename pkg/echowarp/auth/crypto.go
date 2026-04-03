package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	x25519KeySize   = 32
	aesKeySize      = 32
	aesGCMNonceSize = 12
	ecdhHKDFInfo    = "echowarp-ecdh-v1"
	ecdhHKDFSalt    = "echowarp-ecdh-salt-v1"
)

var (
	ErrInvalidKeySize   = fmt.Errorf("invalid key size: expected %d bytes", aesKeySize)
	ErrInvalidNonceSize = fmt.Errorf("invalid nonce size: expected %d bytes", aesGCMNonceSize)
	ErrInvalidPublicKey = fmt.Errorf("invalid public key")
)

// GenerateX25519KeyPair generates a new X25519 key pair.
// Returns (privateKey, publicKey, error)
func GenerateX25519KeyPair() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate X25519 key pair: %w", err)
	}
	publicKey := privateKey.PublicKey()
	return privateKey, publicKey, nil
}

// ComputeSharedSecret computes the shared secret using ECDH.
// Uses X25519: shared = X25519(privateKey, peerPublicKey)
func ComputeSharedSecret(privateKey *ecdh.PrivateKey, peerPublicKey *ecdh.PublicKey) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("compute shared secret: private key is nil")
	}
	if peerPublicKey == nil {
		return nil, fmt.Errorf("compute shared secret: peer public key is nil")
	}

	sharedSecret, err := privateKey.ECDH(peerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("compute shared secret: %w", err)
	}
	return sharedSecret, nil
}

// DeriveAESKey derives an AES-256-GCM key from the shared secret using HKDF.
// Parameters: sharedSecret (32 bytes from X25519), salt (optional context), info (context string)
// Returns: 32-byte AES key
func DeriveAESKey(sharedSecret []byte, salt []byte, info string) ([]byte, error) {
	if len(sharedSecret) != x25519KeySize {
		return nil, fmt.Errorf("derive AES key: %w: got %d bytes", ErrInvalidKeySize, len(sharedSecret))
	}

	if salt == nil {
		salt = []byte(ecdhHKDFSalt)
	}
	if info == "" {
		info = ecdhHKDFInfo
	}

	hkdfReader := hkdf.New(sha256.New, sharedSecret, salt, []byte(info))
	aesKey := make([]byte, aesKeySize)
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		return nil, fmt.Errorf("derive AES key: HKDF read: %w", err)
	}
	return aesKey, nil
}

// EncryptAESGCM encrypts plaintext using AES-256-GCM.
// Returns: (nonce, ciphertext, error)
// Nonce is 12 bytes (AES-GCM standard)
func EncryptAESGCM(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	if len(key) != aesKeySize {
		return nil, nil, fmt.Errorf("encrypt AES-GCM: %w: got %d bytes", ErrInvalidKeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt AES-GCM: create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt AES-GCM: create GCM: %w", err)
	}

	nonce = make([]byte, aesGCMNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("encrypt AES-GCM: generate nonce: %w", err)
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

// DecryptAESGCM decrypts ciphertext using AES-256-GCM.
// Parameters: key, nonce (12 bytes), ciphertext
func DecryptAESGCM(key, nonce, ciphertext []byte) ([]byte, error) {
	if len(key) != aesKeySize {
		return nil, fmt.Errorf("decrypt AES-GCM: %w: got %d bytes", ErrInvalidKeySize, len(key))
	}
	if len(nonce) != aesGCMNonceSize {
		return nil, fmt.Errorf("decrypt AES-GCM: %w: got %d bytes", ErrInvalidNonceSize, len(nonce))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decrypt AES-GCM: create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decrypt AES-GCM: create GCM: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt AES-GCM: %w", err)
	}
	return plaintext, nil
}

// EncodePublicKey encodes a public key to base64 for JSON transmission
func EncodePublicKey(pub *ecdh.PublicKey) string {
	if pub == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(pub.Bytes())
}

// DecodePublicKey decodes a base64 public key
func DecodePublicKey(s string) (*ecdh.PublicKey, error) {
	if s == "" {
		return nil, fmt.Errorf("decode public key: %w: empty string", ErrInvalidPublicKey)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode public key: base64 decode: %w: %w", err, ErrInvalidPublicKey)
	}

	if len(keyBytes) != x25519KeySize {
		return nil, fmt.Errorf("decode public key: %w: expected %d bytes, got %d", ErrInvalidPublicKey, x25519KeySize, len(keyBytes))
	}

	publicKey, err := ecdh.X25519().NewPublicKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w: %w", ErrInvalidPublicKey, err)
	}
	return publicKey, nil
}
