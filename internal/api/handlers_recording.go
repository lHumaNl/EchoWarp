package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// recordingStartRequest is the body shape accepted by
// POST /api/v1/recording/start. Only the mode field is recognized;
// anything else is ignored so the struct can grow (e.g. output_dir)
// without breaking existing Decky plugin clients.
type recordingStartRequest struct {
	Mode string `json:"mode"`
}

// handleRecordingStart starts an audio recording session. The request
// body must be a JSON object with a `mode` field set to one of "mix",
// "tracks", or "both". Mode validation is performed both here (for a
// clean 400 without touching the runner) and in Node.StartRecording
// (defense in depth — unit tests exercise both paths).
//
// Response codes:
//   - 200 on success with {"status":"started","mode":"..."}.
//   - 400 on malformed JSON or an unknown/empty mode.
//   - 409 when the node is not running.
//   - 500 on any other runtime error, including "runner does not
//     implement RecordingController" (ErrInternalState) and
//     "recording already active" (propagated from the adapter).
func (s *APIServer) handleRecordingStart(w http.ResponseWriter, r *http.Request) {
	var req recordingStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	req.Mode = strings.TrimSpace(req.Mode)
	if req.Mode == "" {
		writeError(w, http.StatusBadRequest, "mode must be one of: mix, tracks, both")
		return
	}
	if !echowarp.IsValidRecordingMode(req.Mode) {
		writeError(w, http.StatusBadRequest, "invalid recording mode: "+req.Mode+" (expected mix|tracks|both)")
		return
	}

	if err := s.node.StartRecording(req.Mode); err != nil {
		writeError(w, recordingCommandStatus(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "started",
		"mode":   req.Mode,
	})
}

// handleRecordingStop stops the currently active recording session and
// returns its summary. When no recording was active but the node is
// otherwise healthy, the endpoint returns 200 with a zero-value result
// — matching the Node.StopRecording contract that idempotent "stop
// when nothing to stop" calls succeed.
//
// Response codes:
//   - 200 with {duration, duration_ms, size, files} on success.
//   - 409 when the node is not running.
//   - 500 on any other runtime error, including
//     "runner does not implement RecordingController".
func (s *APIServer) handleRecordingStop(w http.ResponseWriter, _ *http.Request) {
	result, err := s.node.StopRecording()
	if err != nil {
		writeError(w, recordingCommandStatus(err), err.Error())
		return
	}
	// Guarantee non-nil Files slice for JSON consumers — Node.Stop
	// helpers return an empty slice but a paranoid defense-in-depth
	// check here makes the contract explicit.
	if result.Files == nil {
		result.Files = []string{}
	}
	writeJSON(w, http.StatusOK, result)
}

// handleRecordingStatus returns a snapshot of the recorder state.
// Always responds 200 so clients can poll the endpoint safely
// regardless of node state — matching GET /api/v1/bans and
// /api/v1/conference/participants (phases 4b/5b).
func (s *APIServer) handleRecordingStatus(w http.ResponseWriter, _ *http.Request) {
	status := s.node.RecordingStatus()
	writeJSON(w, http.StatusOK, status)
}

// recordingCommandStatus maps a Node recording-command error into an
// HTTP status code. Mirrors banCommandStatus (phase 5b) and
// chatCommandStatus (phase 5a) so all command-style endpoints share
// the same error taxonomy.
//
// Mapping:
//   - ErrConfigValidation → 400 (invalid mode; already pre-checked in
//     the handler but kept here for defense in depth).
//   - ErrNotRunning       → 409 (daemon is idle/stopped).
//   - Anything else, including ErrInternalState ("runner does not
//     implement RecordingController" or "recording already active"),
//     maps to 500.
func recordingCommandStatus(err error) int {
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
