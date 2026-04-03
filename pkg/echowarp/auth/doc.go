// Package auth provides authentication mechanisms for EchoWarp.
//
// Two authentication modes are supported:
//
// # ChallengeAuth (HMAC-SHA256)
//
// For TLS-enabled connections, ChallengeAuth provides password verification
// using HMAC-SHA256 challenge-response. The protocol ensures passwords are
// never transmitted over the network.
//
// Protocol flow:
//   - Server sends random nonce
//   - Client responds with HMAC-SHA256(key, nonce)
//   - Server verifies using constant-time comparison
//
// # ECDHAuth (X25519 + AES-256-GCM)
//
// For non-TLS connections, ECDHAuth provides both authentication and encryption
// using X25519 key exchange and AES-256-GCM.
//
// Security properties:
//   - Forward secrecy: Ephemeral X25519 keypair per session
//   - No password transmission: Only HMAC of challenge is sent
//   - Authenticated encryption: AES-256-GCM with random nonces
//   - Key derivation: HKDF-SHA256 from shared secret
//
// Protocol flow:
//   - Client sends X25519 public key
//   - Server responds with its X25519 public key
//   - Both compute shared secret via X25519
//   - Derive AES-256-GCM key via HKDF
//   - Server sends encrypted challenge (if password required)
//   - Client sends encrypted HMAC response
//   - All subsequent messages encrypted with AES-256-GCM
package auth
