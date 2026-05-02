package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// chatSendRequest is the request body for POST /api/v1/chat/send.
//
// Semantics:
//   - Text must be non-empty (whitespace-only is also rejected).
//   - An empty To means "broadcast to all participants".
//   - A non-empty To is delivered as a direct message addressed to that
//     participant (matched by nickname on the server side).
//
// Length limit: the server-side ChatHub truncates messages longer than the
// internal MaxChatMessageLen (500 runes). This handler does not pre-reject
// long messages — it just forwards them to the runner, which truncates
// transparently. See internal/app/chat.go:HandleIncoming for the rationale.
type chatSendRequest struct {
	Text string `json:"text"`
	To   string `json:"to,omitempty"`
}

// handleChatSend sends a chat message via the running runner. This is the
// API counterpart of the TUI chat input: server operators use it to post
// broadcast announcements or DMs to specific participants; client-mode
// daemons use it to post messages to the server from external integrations
// (Decky plugin, automation, etc.).
//
// Response codes:
//   - 200 on success.
//   - 400 on empty text, malformed JSON, or a validation error from Node.
//   - 409 when the node is not running (no runner attached).
//   - 500 on any other error propagated from the runner (e.g. send failure
//     on the underlying data channel, or a runner that does not implement
//     the ChatSender optional interface).
//
// The request body is capped by the global body-limit middleware, so we do
// not need a per-handler size guard beyond the non-empty-text check.
func (s *APIServer) handleChatSend(w http.ResponseWriter, r *http.Request) {
	var req chatSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Pre-validate text at the handler so callers get a deterministic 400
	// even before reaching the Node. Trim trailing whitespace so a body of
	// "   " is rejected the same as "".
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text must not be empty")
		return
	}

	if err := s.node.SendChat(req.Text, req.To); err != nil {
		writeError(w, chatCommandStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"to":     req.To,
	})
}

// chatCommandStatus maps a Node.SendChat error into an HTTP status code.
//
// Mapping:
//   - ErrConfigValidation → 400 (shouldn't happen after the handler's own
//     pre-validation, but guarded for defense-in-depth in case Node-side
//     checks tighten).
//   - ErrNotRunning       → 409
//   - Anything else       → 500
//
// We match on the structured EchoWarpError code so the mapping is stable
// regardless of the human-readable message.
func chatCommandStatus(err error) int {
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
