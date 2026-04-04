package auth

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSaltManager_Success(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)
	require.NotNil(t, sm)

	salt := sm.GetSalt()
	assert.Len(t, salt, 32)
	assert.NotEqual(t, make([]byte, 32), salt, "salt should not be all zeros")
}

func TestNewSaltManager_DefaultSize(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(0)
	require.NoError(t, err)
	assert.Equal(t, DefaultSaltSize, sm.GetSaltSize())

	sm2, err := NewSaltManager(-1)
	require.NoError(t, err)
	assert.Equal(t, DefaultSaltSize, sm2.GetSaltSize())
}

func TestNewSaltManagerFromBytes_Success(t *testing.T) {
	t.Parallel()

	originalSalt := []byte("test-salt-32-bytes-123456789012")
	sm, err := NewSaltManagerFromBytes(originalSalt)
	require.NoError(t, err)
	require.NotNil(t, sm)

	salt := sm.GetSalt()
	assert.Equal(t, originalSalt, salt)

	// Verify it's a copy, not the same reference
	originalSalt[0] = 'X'
	assert.NotEqual(t, originalSalt, salt)
}

func TestNewSaltManagerFromBytes_EmptyError(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManagerFromBytes([]byte{})
	require.Error(t, err)
	assert.Nil(t, sm)
	assert.Contains(t, err.Error(), "empty")
}

func TestSaltManager_GetSalt_ReturnsCopy(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	salt1 := sm.GetSalt()
	salt2 := sm.GetSalt()

	// Should be equal but different slices
	assert.Equal(t, salt1, salt2)
	assert.NotSame(t, &salt1[0], &salt2[0])
}

func TestSaltManager_RotateSalt(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	oldSalt := sm.GetSalt()

	err = sm.RotateSalt()
	require.NoError(t, err)

	newSalt := sm.GetSalt()

	// Salt should be different after rotation
	assert.NotEqual(t, oldSalt, newSalt)
	assert.Len(t, newSalt, 32)
}

func TestSaltManager_SetSalt(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	newSalt := []byte("new-salt-32-bytes-09876543210987")
	err = sm.SetSalt(newSalt)
	require.NoError(t, err)

	salt := sm.GetSalt()
	assert.Equal(t, newSalt, salt)

	// Verify it's a copy
	newSalt[0] = 'X'
	assert.NotEqual(t, newSalt, salt)
}

func TestSaltManager_SetSalt_EmptyError(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	err = sm.SetSalt([]byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestSaltManager_Clear(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	originalSalt := sm.GetSalt()
	assert.NotEqual(t, make([]byte, 32), originalSalt)

	sm.Clear()

	salt := sm.GetSalt()
	assert.Empty(t, salt)
}

func TestSaltManager_Concurrent(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(100)

	for i := 0; i < 25; i++ {
		go func() {
			defer wg.Done()
			_ = sm.GetSalt()
		}()
	}

	for i := 0; i < 25; i++ {
		go func() {
			defer wg.Done()
			_ = sm.RotateSalt()
		}()
	}

	for i := 0; i < 25; i++ {
		go func() {
			defer wg.Done()
			salt := []byte("concurrent-salt-test-32-bytes-test")
			_ = sm.SetSalt(salt)
		}()
	}

	for i := 0; i < 25; i++ {
		go func() {
			defer wg.Done()
			_ = sm.GetSaltSize()
		}()
	}

	wg.Wait()
}

func TestGetGlobalSaltManager(t *testing.T) {
	t.Parallel()

	sm1, err := GetGlobalSaltManager()
	require.NoError(t, err)
	require.NotNil(t, sm1)

	sm2, err := GetGlobalSaltManager()
	require.NoError(t, err)

	// Should return the same instance
	assert.Equal(t, sm1, sm2)
}

func TestGenerateRandomSalt(t *testing.T) {
	t.Parallel()

	salt1, err := GenerateRandomSalt(32)
	require.NoError(t, err)
	assert.Len(t, salt1, 32)

	salt2, err := GenerateRandomSalt(32)
	require.NoError(t, err)

	// Two generated salts should be different
	assert.NotEqual(t, salt1, salt2)
}

func TestGenerateRandomSalt_DefaultSize(t *testing.T) {
	t.Parallel()

	salt, err := GenerateRandomSalt(0)
	require.NoError(t, err)
	assert.Len(t, salt, DefaultSaltSize)

	salt2, err := GenerateRandomSalt(-1)
	require.NoError(t, err)
	assert.Len(t, salt2, DefaultSaltSize)
}

func TestSaltManager_UniqueSalts(t *testing.T) {
	t.Parallel()

	salts := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		sm, err := NewSaltManager(32)
		require.NoError(t, err)
		salts[i] = sm.GetSalt()
	}

	// All salts should be unique
	for i := 0; i < len(salts); i++ {
		for j := i + 1; j < len(salts); j++ {
			assert.False(t, bytes.Equal(salts[i], salts[j]),
				"salts %d and %d should be unique", i, j)
		}
	}
}

func TestSaltManager_IntegrationWithKeyDerivation(t *testing.T) {
	t.Parallel()

	sm, err := NewSaltManager(32)
	require.NoError(t, err)

	// Generate shared secret
	sharedSecret := make([]byte, x25519KeySize)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	// Derive key with dynamic salt
	key1, err := DeriveAESKey(sharedSecret, sm.GetSalt(), "test-context")
	require.NoError(t, err)
	assert.Len(t, key1, aesKeySize)

	// Rotate salt and derive again - should get different key
	err = sm.RotateSalt()
	require.NoError(t, err)

	key2, err := DeriveAESKey(sharedSecret, sm.GetSalt(), "test-context")
	require.NoError(t, err)

	// Keys should be different due to different salts
	assert.NotEqual(t, key1, key2)
}
