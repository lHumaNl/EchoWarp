package startup

import (
	"crypto/tls"
	"fmt"

	"github.com/lHumaNl/echowarp/internal/config"
)

// ValidateServerTLS refuses partial/unreadable TLS configuration. Callers must
// not reinterpret this failure as permission to open a plaintext listener.
func ValidateServerTLS(cfg config.Config) error {
	if cfg.Mode != config.ModeServer {
		return nil
	}
	if !cfg.TLS && cfg.TLSCert == "" && cfg.TLSKey == "" {
		return nil
	}
	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		return fmt.Errorf("server TLS requires both certificate and key")
	}
	if _, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey); err != nil {
		return fmt.Errorf("server TLS certificate/key could not be loaded; check paths and PEM contents")
	}
	return nil
}
