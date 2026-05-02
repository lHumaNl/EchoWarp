package api

import (
	"encoding/json"
	"errors"
	"net/http"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// muteToggleRequest is the body shape accepted by POST /api/v1/mute.
// Only the muted field is recognized; anything else is ignored so the
// struct can grow without breaking existing Decky plugin clients. The
// field is a pointer so the handler can distinguish between an absent
// field (400) and an explicit false (200 — unmute).
type muteToggleRequest struct {
	Muted *bool `json:"muted"`
}

// discoveryPublishRequest is the body shape accepted by
// POST /api/v1/discovery/publish. Enabled is a pointer for the same
// reason muteToggleRequest.Muted is.
type discoveryPublishRequest struct {
	Enabled *bool `json:"enabled"`
}

// handleMuteToggle toggles local mute of the incoming audio stream for
// client-mode runners. The request body must be a JSON object with a
// boolean "muted" field. The handler is deliberately thin: it decodes,
// delegates to Node.SetMuted, and maps the resulting error into an
// HTTP status. Server-mode runners lack MuteController and therefore
// produce ErrInternalState → 500 — the endpoint is wired uniformly
// across runner types but only takes effect on clients.
//
// Response codes:
//   - 200 with {"status":"ok","muted":bool} on success.
//   - 400 on malformed JSON or missing "muted" field.
//   - 409 when the node is not running.
//   - 500 on any other runtime error, including "runner does not
//     implement MuteController" (ErrInternalState) — this is what a
//     server-mode daemon returns.
func (s *APIServer) handleMuteToggle(w http.ResponseWriter, r *http.Request) {
	var req muteToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Muted == nil {
		writeError(w, http.StatusBadRequest, "missing required field: muted")
		return
	}

	if err := s.node.SetMuted(*req.Muted); err != nil {
		writeError(w, toggleCommandStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"muted":  *req.Muted,
	})
}

// handleDiscoveryPublish toggles mDNS service publishing for server-mode
// runners. Client-mode runners lack DiscoveryPublisher and therefore
// produce ErrInternalState → 500.
//
// Response codes:
//   - 200 with {"status":"ok","enabled":bool} on success.
//   - 400 on malformed JSON or missing "enabled" field.
//   - 409 when the node is not running.
//   - 500 on any other runtime error, including "runner does not
//     implement DiscoveryPublisher" (ErrInternalState) and
//     "failed to create mDNS publisher" (zeroconf registration error).
func (s *APIServer) handleDiscoveryPublish(w http.ResponseWriter, r *http.Request) {
	var req discoveryPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "missing required field: enabled")
		return
	}

	if err := s.node.SetDiscoveryPublish(*req.Enabled); err != nil {
		writeError(w, toggleCommandStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"enabled": *req.Enabled,
	})
}

// toggleCommandStatus maps a Node toggle-command error into an HTTP
// status code. Shared between handleMuteToggle and handleDiscoveryPublish
// because both endpoints have identical error taxonomy. Mirrors
// recordingCommandStatus / banCommandStatus / chatCommandStatus so
// every command-style endpoint shares the same mapping discipline.
//
// Mapping:
//   - ErrConfigValidation → 400 (reserved — neither toggle produces
//     this today, but the mapping is kept so a future validation rule
//     slots in without rework).
//   - ErrNotRunning       → 409 (daemon is idle/stopped).
//   - Anything else, including ErrInternalState ("runner does not
//     implement MuteController/DiscoveryPublisher" or a zeroconf
//     registration error), maps to 500.
func toggleCommandStatus(err error) int {
	var ewErr *ewerrors.EchoWarpError
	if errors.As(err, &ewErr) {
		switch ewErr.Code {
		case ewerrors.ErrConfigValidation:
			return http.StatusBadRequest
		case ewerrors.ErrNotRunning:
			return http.StatusConflict
		}
	}
	return http.StatusInternalServerError
}
