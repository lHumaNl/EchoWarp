package auth

import "github.com/lHumaNl/echowarp/pkg/echowarp/transport"

// Authenticator defines the interface for authentication handlers.
// Implementations handle the challenge-response authentication flow.
type Authenticator interface {
	AuthenticateServer(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error
	AuthenticateClient(send transport.AuthSendFunc, recv transport.AuthRecvFunc, password string) error
}

// EncryptedAuthenticator extends Authenticator with post-handshake encryption support.
// Implementations return an encryption key that can be used to encrypt subsequent messages.
type EncryptedAuthenticator interface {
	Authenticator
	EncryptionKey() []byte
}

// EncryptionKey returns nil for ChallengeAuth since it relies on TLS for encryption.
func (a *ChallengeAuth) EncryptionKey() []byte {
	return nil
}

var (
	_ Authenticator          = (*ChallengeAuth)(nil)
	_ EncryptedAuthenticator = (*ChallengeAuth)(nil)
)
