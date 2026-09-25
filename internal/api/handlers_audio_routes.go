package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// A pointer distinguishes explicit unmute from an omitted or null boolean.
type audioRouteRequest struct {
	Scope     string `json:"scope"`
	Source    string `json:"source"`
	Recipient string `json:"recipient"`
	Muted     *bool  `json:"muted"`
}

func (s *APIServer) handleAudioRoutes(w http.ResponseWriter, r *http.Request) {
	state, err := s.node.AudioRoutes(r.Context())
	if err != nil {
		writeError(w, audioRouteCommandStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *APIServer) handleSetAudioRoute(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	rule, err := decodeAudioRoute(r.Body)
	if err != nil {
		writeError(w, audioRouteBodyStatus(err), err.Error())
		return
	}
	if err := s.node.SetAudioRoute(r.Context(), rule); err != nil {
		writeError(w, audioRouteCommandStatus(err), err.Error())
		return
	}
	// Echo the acknowledged command, avoiding a second read that could fail or
	// race with another mutation after this command has already been applied.
	writeJSON(w, http.StatusOK, rule)
}

func decodeAudioRoute(body io.Reader) (echowarp.AudioRouteRule, error) {
	var req audioRouteRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return echowarp.AudioRouteRule{}, err
	}
	if err := requireAudioRouteEOF(decoder); err != nil {
		return echowarp.AudioRouteRule{}, err
	}
	if req.Muted == nil {
		return echowarp.AudioRouteRule{}, errors.New("missing required boolean field: muted")
	}
	return echowarp.AudioRouteRule{
		Scope: req.Scope, Source: req.Source, Recipient: req.Recipient, Muted: *req.Muted,
	}, nil
}

func requireAudioRouteEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("request body must contain exactly one JSON object")
}

func audioRouteBodyStatus(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func audioRouteCommandStatus(err error) int {
	if errors.Is(err, context.Canceled) {
		return http.StatusRequestTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return audioRouteErrorCodeStatus(ewerrors.GetCode(err))
}

func audioRouteErrorCodeStatus(code string) int {
	switch code {
	case ewerrors.ErrConfigValidation:
		return http.StatusBadRequest
	case ewerrors.ErrAuthFailed:
		return http.StatusForbidden
	case ewerrors.ErrNotRunning:
		return http.StatusConflict
	case ewerrors.ErrConnectionTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
