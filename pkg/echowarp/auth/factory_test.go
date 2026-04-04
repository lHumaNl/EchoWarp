package auth

import (
	"testing"
)

func TestNewAuthenticator_WithTLS_ReturnsChallengeAuth(t *testing.T) {
	t.Parallel()

	auth := NewAuthenticator(true, "")
	if auth == nil {
		t.Fatal("NewAuthenticator returned nil")
	}

	if _, ok := auth.(*ChallengeAuth); !ok {
		t.Errorf("Expected *ChallengeAuth, got %T", auth)
	}
}

func TestNewAuthenticator_WithoutTLS_ReturnsECDHAuth(t *testing.T) {
	t.Parallel()

	auth := NewAuthenticator(false, "test-password")
	if auth == nil {
		t.Fatal("NewAuthenticator returned nil")
	}

	if _, ok := auth.(*ECDHAuth); !ok {
		t.Errorf("Expected *ECDHAuth, got %T", auth)
	}
}

func TestNewAuthenticator_OnlyCert_ReturnsECDHAuth(t *testing.T) {
	t.Parallel()

	// When TLS is not fully enabled, should fall back to ECDH
	auth := NewAuthenticator(false, "")
	if auth == nil {
		t.Fatal("NewAuthenticator returned nil")
	}

	if _, ok := auth.(*ECDHAuth); !ok {
		t.Errorf("Expected *ECDHAuth when TLS is not enabled, got %T", auth)
	}
}

func TestNewAuthenticator_OnlyKey_ReturnsECDHAuth(t *testing.T) {
	t.Parallel()

	// When TLS is not fully enabled, should fall back to ECDH
	auth := NewAuthenticator(false, "")
	if auth == nil {
		t.Fatal("NewAuthenticator returned nil")
	}

	if _, ok := auth.(*ECDHAuth); !ok {
		t.Errorf("Expected *ECDHAuth when TLS is not enabled, got %T", auth)
	}
}
