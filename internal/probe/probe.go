// Package probe provides HTTP probing of EchoWarp servers via the session-info endpoint.
// Both the TUI and CLI packages use this to discover server parameters before connecting.
package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
)

// ProbeServerResult holds the response from GET /api/v1/session/info.
type ProbeServerResult struct {
	Mode             string `json:"mode"`
	TLSRequired      bool   `json:"tls_required"`
	TLSSelfSigned    bool   `json:"tls_self_signed"`
	CurrentClients   int    `json:"current_clients"`
	MaxClients       int    `json:"max_clients"`
	SampleRate       uint32 `json:"sample_rate"`
	Channels         uint32 `json:"channels"`
	OpusBitrate      int    `json:"opus_bitrate"`
	ServerVersion    string `json:"server_version"`
	ServerName       string `json:"server_name"`
	PasswordRequired bool   `json:"password_required"`
	HWIDRequired     bool   `json:"hwid_required"`
	ServerID         string `json:"server_id"` // Stable per-port UUID; empty for old servers.
}

// ProbeServer performs an HTTP request to the server's session-info endpoint.
// Tries the session info port (port+1, used in TUI mode) first, then the main port (daemon mode API).
// Returns the result or an error if the server is unavailable.
// Uses a default 3-second timeout.
func ProbeServer(addr string, port int) (*ProbeServerResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return ProbeServerWithContext(ctx, addr, port)
}

// ProbeServerWithContext performs an HTTP request to the server's session-info endpoint
// using the provided context for cancellation and timeout control.
// Tries the session info port (port+1, used in TUI mode) first, then the main port (daemon mode API).
func ProbeServerWithContext(ctx context.Context, addr string, port int) (*ProbeServerResult, error) {
	client := &http.Client{}

	// Session info port = signaling port + 1 (TUI mode), then main port (daemon mode).
	ports := []int{port + 1, port}

	for _, p := range ports {
		rawURL := fmt.Sprintf("http://%s:%d/api/v1/session/info", addr, p)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close() //nolint:errcheck
			continue
		}

		var result ProbeServerResult
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close() //nolint:errcheck
		if decodeErr != nil {
			return nil, decodeErr
		}
		return &result, nil
	}

	return nil, fmt.Errorf("server unavailable")
}

// ApplyProbeToConfig applies server probe result to client config.
// Returns error if password is required but not provided.
func ApplyProbeToConfig(cfg *config.Config, probe *ProbeServerResult) error {
	// Mode
	cfg.StreamMode = config.AudioMode(probe.Mode)
	if cfg.StreamMode == "" {
		cfg.StreamMode = config.AudioModeNormal
	}
	cfg.SyncFromStreamMode()

	// Audio params (guard zero-values)
	if probe.SampleRate > 0 {
		cfg.SampleRate = probe.SampleRate
	}
	if probe.Channels > 0 {
		cfg.Channels = probe.Channels
	}
	if probe.OpusBitrate > 0 {
		cfg.OpusBitrate = probe.OpusBitrate
	}
	if probe.MaxClients > 0 {
		cfg.MaxClients = probe.MaxClients
	}

	// TLS
	cfg.TLS = probe.TLSRequired
	if probe.TLSSelfSigned {
		cfg.TLSInsecure = true
		cfg.TLSSelfSigned = true
	}

	// Password check
	if probe.PasswordRequired && cfg.Password == "" {
		return fmt.Errorf("server requires password, use --password")
	}

	return nil
}
