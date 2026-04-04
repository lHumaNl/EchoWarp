// Package auth provides authentication mechanisms for EchoWarp.
// This file implements dynamic HKDF salt generation for enhanced security.
package auth

import (
	"crypto/rand"
	"fmt"
	"sync"
)

const (
	// DefaultSaltSize is the default size for HKDF salts in bytes.
	DefaultSaltSize = 32
)

// SaltManager manages dynamic HKDF salts for key derivation.
// Using dynamic salts enhances security by preventing precomputation attacks
// and ensuring that each session uses a unique salt.
//
// Thread-safe: Internal state protected by sync.RWMutex.
type SaltManager struct {
	salt     []byte
	saltSize int
	mu       sync.RWMutex
}

// NewSaltManager creates a new SaltManager with a randomly generated salt.
func NewSaltManager(saltSize int) (*SaltManager, error) {
	if saltSize <= 0 {
		saltSize = DefaultSaltSize
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	return &SaltManager{
		salt:     salt,
		saltSize: saltSize,
	}, nil
}

// NewSaltManagerFromBytes creates a SaltManager from an existing salt.
// Useful for restoring a salt from a serialized state or protocol message.
func NewSaltManagerFromBytes(salt []byte) (*SaltManager, error) {
	if len(salt) == 0 {
		return nil, fmt.Errorf("salt cannot be empty")
	}

	saltCopy := make([]byte, len(salt))
	copy(saltCopy, salt)

	return &SaltManager{
		salt:     saltCopy,
		saltSize: len(saltCopy),
	}, nil
}

// GetSalt returns the current salt.
// The returned slice is a copy to prevent modification.
func (sm *SaltManager) GetSalt() []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	saltCopy := make([]byte, len(sm.salt))
	copy(saltCopy, sm.salt)
	return saltCopy
}

// GetSaltSize returns the salt size in bytes.
func (sm *SaltManager) GetSaltSize() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.saltSize
}

// RotateSalt generates a new random salt.
// This should be called periodically or per-session for enhanced security.
func (sm *SaltManager) RotateSalt() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate new salt
	newSalt := make([]byte, sm.saltSize)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("rotate salt: %w", err)
	}

	// Zero the old salt before replacing
	for i := range sm.salt {
		sm.salt[i] = 0
	}

	sm.salt = newSalt
	return nil
}

// SetSalt sets the salt from an external source.
// Used when receiving a salt from a peer during key exchange.
func (sm *SaltManager) SetSalt(salt []byte) error {
	if len(salt) == 0 {
		return fmt.Errorf("salt cannot be empty")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Zero the old salt
	for i := range sm.salt {
		sm.salt[i] = 0
	}

	// Copy new salt
	sm.salt = make([]byte, len(salt))
	copy(sm.salt, salt)
	sm.saltSize = len(salt)

	return nil
}

// Clear securely zeroes the salt in memory.
func (sm *SaltManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i := range sm.salt {
		sm.salt[i] = 0
	}
	sm.salt = nil
}

// GlobalSaltManager is a package-level default salt manager.
// Initialized on first use. For production use, prefer creating
// a dedicated SaltManager instance per session.
var (
	globalSaltManager     *SaltManager
	globalSaltManagerOnce sync.Once
	globalSaltManagerErr  error
)

// GetGlobalSaltManager returns the global salt manager, initializing it if necessary.
// For enhanced security, prefer creating per-session SaltManager instances.
func GetGlobalSaltManager() (*SaltManager, error) {
	globalSaltManagerOnce.Do(func() {
		globalSaltManager, globalSaltManagerErr = NewSaltManager(DefaultSaltSize)
	})
	return globalSaltManager, globalSaltManagerErr
}

// GenerateRandomSalt generates a random salt of the specified size.
// This is a convenience function for one-time salt generation.
func GenerateRandomSalt(size int) ([]byte, error) {
	if size <= 0 {
		size = DefaultSaltSize
	}

	salt := make([]byte, size)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate random salt: %w", err)
	}

	return salt, nil
}
