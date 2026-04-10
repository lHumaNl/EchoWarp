package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// handleBanList returns a JSON array of every currently active ban
// across all kinds (IP, HWID, Nickname). The handler is deliberately
// lenient: it always responds 200 with a (possibly empty) array. This
// mirrors GET /api/v1/conference/participants and keeps the endpoint
// safe to poll by a Decky plugin regardless of daemon state — the
// client inspects GET /api/v1/status separately if it needs to
// distinguish "idle" from "no bans".
func (s *APIServer) handleBanList(w http.ResponseWriter, _ *http.Request) {
	list := s.node.BanList()
	if list == nil {
		list = []echowarp.BanEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleBanAdd installs a new ban. The request body accepts exactly one
// of {ip, hwid, nickname}; the Node wrapper re-validates this and the
// handler performs an additional pre-check so the caller gets a clean
// 400 without touching the runner when the body is obviously malformed.
//
// Response codes:
//   - 201 Created on success with the created entry (including its
//     server-assigned ID) in the body so the client can use it in a
//     subsequent DELETE without a round-trip through GET.
//   - 400 on malformed JSON or missing subject.
//   - 409 when the node is not running.
//   - 500 on any other runtime error propagated from the runner.
func (s *APIServer) handleBanAdd(w http.ResponseWriter, r *http.Request) {
	var req echowarp.BanEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Trim once — a body of {"ip":"   "} is a malformed subject, not a
	// valid address, so normalise before counting populated fields.
	req.IP = strings.TrimSpace(req.IP)
	req.HWID = strings.TrimSpace(req.HWID)
	req.Nickname = strings.TrimSpace(req.Nickname)

	if req.IP == "" && req.HWID == "" && req.Nickname == "" {
		writeError(w, http.StatusBadRequest, "at least one of ip, hwid, or nickname must be set")
		return
	}

	entry := echowarp.BanEntry{
		IP:       req.IP,
		HWID:     req.HWID,
		Nickname: req.Nickname,
		Reason:   req.Reason,
	}

	if err := s.node.AddBan(entry); err != nil {
		writeError(w, banCommandStatus(err), err.Error())
		return
	}

	// Attach the server-assigned ID so the client can round-trip to
	// DELETE without calling GET first. The scheme matches
	// internal/app/ban_adapter.go: "<kind>:<subject>".
	entry.ID = banEntryID(entry)
	writeJSON(w, http.StatusCreated, entry)
}

// handleBanRemove deletes a previously installed ban by id. The id is
// extracted from the URL path and must match the "<kind>:<subject>"
// form returned by GET /api/v1/bans. Unknown or malformed ids return
// 400 so callers can distinguish "bad request" from "runtime error".
//
// Response codes:
//   - 200 with {"status":"ok"} on success. (We return 200 rather than
//     204 for parity with POST /api/v1/disconnect and friends, which
//     also wrap their success in a tiny JSON body — keeping all
//     state-changing endpoints structurally uniform simplifies the
//     Decky plugin's response handling.)
//   - 400 on missing/malformed id.
//   - 409 when the node is not running.
//   - 500 on any other runtime error propagated from the runner.
func (s *APIServer) handleBanRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Manual extraction for older patterns — mirrors
		// parseParticipantID in handlers.go.
		const prefix = "/api/v1/bans/"
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		id = parts[0]
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing ban id")
		return
	}

	if err := s.node.RemoveBan(id); err != nil {
		writeError(w, banCommandStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"id":     id,
	})
}

// banEntryID derives the public id for a newly created ban entry. Kept
// in lockstep with internal/app/ban_adapter.go — if you change the
// prefix scheme there, change it here too. A central const package
// would be cleaner but would pull pkg/echowarp into internal/app
// import-cycle territory, so we accept the duplication.
func banEntryID(entry echowarp.BanEntry) string {
	switch {
	case entry.IP != "":
		return "ip:" + entry.IP
	case entry.HWID != "":
		return "hwid:" + entry.HWID
	case entry.Nickname != "":
		return "nick:" + entry.Nickname
	}
	return ""
}

// banCommandStatus maps a Node ban-command error into an HTTP status
// code. Matches the chatCommandStatus layout (phase 5a) for a uniform
// error taxonomy across all command-style endpoints.
//
// Mapping:
//   - ErrConfigValidation → 400 (body was structurally valid JSON but
//     the subject set was inconsistent, or an unknown id kind).
//   - ErrNotRunning       → 409 (daemon is idle/stopped).
//   - Anything else, including ErrInternalState from a runner that
//     does not implement BanManager, maps to 500.
func banCommandStatus(err error) int {
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
