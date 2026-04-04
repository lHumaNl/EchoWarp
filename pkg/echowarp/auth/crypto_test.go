package auth

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateX25519KeyPair_Success(t *testing.T) {
	t.Parallel()
	privateKey, publicKey, err := GenerateX25519KeyPair()

	assert.NoError(t, err)
	assert.NotNil(t, privateKey)
	assert.NotNil(t, publicKey)
	assert.Equal(t, privateKey.PublicKey(), publicKey)
}

func TestGenerateX25519KeyPair_DifferentEachTime(t *testing.T) {
	t.Parallel()
	priv1, pub1, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	priv2, pub2, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	assert.NotEqual(t, priv1.Bytes(), priv2.Bytes())
	assert.NotEqual(t, pub1.Bytes(), pub2.Bytes())
}

func TestGenerateX25519KeyPair_ValidKeySize(t *testing.T) {
	t.Parallel()
	privateKey, publicKey, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	assert.Equal(t, x25519KeySize, len(privateKey.Bytes()))
	assert.Equal(t, x25519KeySize, len(publicKey.Bytes()))
}

func TestComputeSharedSecret_BothSidesMatch(t *testing.T) {
	t.Parallel()
	alicePriv, alicePub, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	bobPriv, bobPub, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	aliceShared, err := ComputeSharedSecret(alicePriv, bobPub)
	require.NoError(t, err)

	bobShared, err := ComputeSharedSecret(bobPriv, alicePub)
	require.NoError(t, err)

	assert.Equal(t, aliceShared, bobShared)
	assert.Equal(t, x25519KeySize, len(aliceShared))
}

func TestComputeSharedSecret_Errors(t *testing.T) {
	t.Parallel()
	_, peerPub, err := GenerateX25519KeyPair()
	require.NoError(t, err)
	priv, _, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	tests := []struct {
		name        string
		privKey     interface{}
		pubKey      interface{}
		wantErr     bool
		errContains string
	}{
		{"nil private key", nil, peerPub, true, "private key is nil"},
		{"nil public key", priv, nil, true, "peer public key is nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var privKey *ecdh.PrivateKey
			var pubKey *ecdh.PublicKey
			if tt.privKey != nil {
				privKey = tt.privKey.(*ecdh.PrivateKey)
			}
			if tt.pubKey != nil {
				pubKey = tt.pubKey.(*ecdh.PublicKey)
			}
			shared, err := ComputeSharedSecret(privKey, pubKey)
			if tt.wantErr {
				assert.Nil(t, shared)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, shared)
			}
		})
	}
}

func TestDeriveAESKey(t *testing.T) {
	t.Parallel()
	sharedSecret := make([]byte, x25519KeySize)
	_, err := rand.Read(sharedSecret)
	require.NoError(t, err)
	shared1 := make([]byte, x25519KeySize)
	shared2 := make([]byte, x25519KeySize)
	_, err = rand.Read(shared1)
	require.NoError(t, err)
	_, err = rand.Read(shared2)
	require.NoError(t, err)

	tests := []struct {
		name       string
		secret     []byte
		salt       []byte
		info       string
		wantErr    bool
		errContain string
		checkFunc  func(t *testing.T, key []byte)
	}{
		{
			"correct size",
			sharedSecret,
			nil,
			"",
			false,
			"",
			func(t *testing.T, key []byte) {
				assert.Equal(t, aesKeySize, len(key))
			},
		},
		{
			"deterministic",
			sharedSecret,
			[]byte("salt"),
			"info",
			false,
			"",
			func(t *testing.T, key []byte) {
				key2, err := DeriveAESKey(sharedSecret, []byte("salt"), "info")
				require.NoError(t, err)
				assert.Equal(t, key, key2)
			},
		},
		{
			"different inputs different keys",
			shared1,
			nil,
			"",
			false,
			"",
			func(t *testing.T, key []byte) {
				key2, err := DeriveAESKey(shared2, nil, "")
				require.NoError(t, err)
				assert.NotEqual(t, key, key2)
			},
		},
		{
			"default salt info",
			sharedSecret,
			nil,
			"",
			false,
			"",
			func(t *testing.T, key []byte) {
				keyWithExplicit, err := DeriveAESKey(sharedSecret, []byte(ecdhHKDFSalt), ecdhHKDFInfo)
				require.NoError(t, err)
				assert.Equal(t, key, keyWithExplicit)
			},
		},
		{
			"wrong shared secret size",
			make([]byte, 16),
			nil,
			"",
			true,
			"invalid key size",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DeriveAESKey(tt.secret, tt.salt, tt.info)
			if tt.wantErr {
				assert.Nil(t, key)
				assert.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, key)
				if tt.checkFunc != nil {
					tt.checkFunc(t, key)
				}
			}
		})
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	t.Parallel()
	key := make([]byte, aesKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := []byte("Hello, World! This is a secret message.")

	nonce, ciphertext, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	decrypted, err := DecryptAESGCM(key, nonce, ciphertext)
	require.NoError(t, err)

	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentNonces(t *testing.T) {
	t.Parallel()
	key := make([]byte, aesKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := []byte("Same message")

	nonce1, ciphertext1, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	nonce2, ciphertext2, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	assert.NotEqual(t, nonce1, nonce2)
	assert.NotEqual(t, ciphertext1, ciphertext2)
	assert.Equal(t, aesGCMNonceSize, len(nonce1))
	assert.Equal(t, aesGCMNonceSize, len(nonce2))
}

func TestDecrypt_Errors(t *testing.T) {
	t.Parallel()
	key := make([]byte, aesKeySize)
	key2 := make([]byte, aesKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	_, err = rand.Read(key2)
	require.NoError(t, err)
	plaintext := []byte("Secret message")
	nonce, ciphertext, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)
	wrongNonce := make([]byte, aesGCMNonceSize)
	_, err = rand.Read(wrongNonce)
	require.NoError(t, err)

	tests := []struct {
		name       string
		key        []byte
		nonce      []byte
		ciphertext []byte
		wantErr    bool
		errContain string
	}{
		{"wrong key", key2, nonce, ciphertext, true, "decrypt AES-GCM"},
		{"wrong nonce", key, wrongNonce, ciphertext, true, ""},
		{"tampered ciphertext", key, nonce, append([]byte{0xFF}, ciphertext[1:]...), true, ""},
		{"wrong key size", make([]byte, 16), make([]byte, aesGCMNonceSize), []byte("dummy"), true, "invalid key size"},
		{"wrong nonce size", key, make([]byte, 16), []byte("dummy"), true, "invalid nonce size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decrypted, err := DecryptAESGCM(tt.key, tt.nonce, tt.ciphertext)
			assert.Nil(t, decrypted)
			assert.Error(t, err)
			if tt.errContain != "" {
				assert.Contains(t, err.Error(), tt.errContain)
			}
		})
	}
}

func TestEncodeDecodePublicKey_Roundtrip(t *testing.T) {
	t.Parallel()
	_, pub, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	encoded := EncodePublicKey(pub)
	assert.NotEmpty(t, encoded)

	decoded, err := DecodePublicKey(encoded)
	require.NoError(t, err)

	assert.Equal(t, pub.Bytes(), decoded.Bytes())
}

func TestDecodePublicKey_Errors(t *testing.T) {
	t.Parallel()
	wrongSizeKey := make([]byte, 16)
	encoded := base64.StdEncoding.EncodeToString(wrongSizeKey)

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		errContain string
	}{
		{"empty string", "", true, "empty string"},
		{"invalid base64", "not-valid-base64!!!", true, "base64 decode"},
		{"wrong size", encoded, true, "expected 32 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DecodePublicKey(tt.input)
			assert.Nil(t, key)
			assert.Error(t, err)
			if tt.errContain != "" {
				assert.Contains(t, err.Error(), tt.errContain)
			}
		})
	}
}

func TestEncodePublicKey_Nil_EmptyString(t *testing.T) {
	t.Parallel()
	encoded := EncodePublicKey(nil)

	assert.Empty(t, encoded)
}

func TestE2E_FullFlow(t *testing.T) {
	t.Parallel()
	alicePriv, alicePub, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	bobPriv, bobPub, err := GenerateX25519KeyPair()
	require.NoError(t, err)

	aliceShared, err := ComputeSharedSecret(alicePriv, bobPub)
	require.NoError(t, err)

	bobShared, err := ComputeSharedSecret(bobPriv, alicePub)
	require.NoError(t, err)

	assert.Equal(t, aliceShared, bobShared)

	aliceKey, err := DeriveAESKey(aliceShared, nil, "")
	require.NoError(t, err)

	bobKey, err := DeriveAESKey(bobShared, nil, "")
	require.NoError(t, err)

	assert.Equal(t, aliceKey, bobKey)

	message := []byte("Hello from Alice to Bob!")

	nonce, ciphertext, err := EncryptAESGCM(aliceKey, message)
	require.NoError(t, err)

	decrypted, err := DecryptAESGCM(bobKey, nonce, ciphertext)
	require.NoError(t, err)

	assert.Equal(t, message, decrypted)

	alicePubEncoded := EncodePublicKey(alicePub)
	alicePubDecoded, err := DecodePublicKey(alicePubEncoded)
	require.NoError(t, err)
	assert.Equal(t, alicePub.Bytes(), alicePubDecoded.Bytes())
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	t.Parallel()
	key := make([]byte, aesKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := []byte{}

	nonce, ciphertext, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	decrypted, err := DecryptAESGCM(key, nonce, ciphertext)
	require.NoError(t, err)

	assert.True(t, bytes.Equal(plaintext, decrypted))
}

func TestEncryptDecrypt_LargePlaintext(t *testing.T) {
	t.Parallel()
	key := make([]byte, aesKeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	plaintext := make([]byte, 1024*1024)
	_, err = rand.Read(plaintext)
	require.NoError(t, err)

	nonce, ciphertext, err := EncryptAESGCM(key, plaintext)
	require.NoError(t, err)

	decrypted, err := DecryptAESGCM(key, nonce, ciphertext)
	require.NoError(t, err)

	assert.True(t, bytes.Equal(plaintext, decrypted))
}
