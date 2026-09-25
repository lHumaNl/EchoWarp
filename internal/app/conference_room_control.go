package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

const (
	ConferenceVersion           = 1
	ActionConferenceHello       = "conference_hello"
	ActionConferenceHelloAck    = "conference_hello_ack"
	ActionConferenceState       = "conference_state"
	ActionConferenceOffer       = "conference_offer"
	ActionConferenceAnswer      = "conference_answer"
	ActionConferenceRoute       = "conference_route"
	ActionConferenceRouteResult = "conference_route_result"
	ActionConferenceRoutes      = "conference_routes"
	conferenceMaxControlBytes   = 256 * 1024
)

type ConferenceHello struct {
	Version int    `json:"version"`
	SelfID  string `json:"self_id"`
}

type ConferenceSourceState struct {
	ID         string  `json:"id"`
	Enabled    bool    `json:"enabled"`
	Gain       float32 `json:"gain"`
	Generation uint64  `json:"generation"`
}

type ConferenceStateMessage struct {
	Sources []ConferenceSourceState  `json:"sources"`
	Routes  echowarp.AudioRouteState `json:"routes"`
}

type ConferenceDescription struct {
	ID  uint64 `json:"id"`
	SDP string `json:"sdp"`
}

type ConferenceRouteResult struct {
	ID    uint64                   `json:"id"`
	Error string                   `json:"error,omitempty"`
	State echowarp.AudioRouteState `json:"state"`
}

type conferenceCommand struct {
	action  string
	payload any
	answer  ConferenceDescription
}

func (r *ConferenceRoom) HandleControl(id, action string, data json.RawMessage) (bool, error) {
	if !conferenceClientAction(action) {
		return false, nil
	}
	r.mu.RLock()
	p := r.peers[id]
	r.mu.RUnlock()
	if p == nil {
		return true, errors.New("unknown conference peer")
	}
	switch action {
	case ActionConferenceHelloAck:
		return true, r.handleHelloAck(p, data)
	case ActionConferenceAnswer:
		return true, r.handleAnswer(p, data)
	default:
		return true, r.handleRoute(p, action, data)
	}
}

func conferenceClientAction(action string) bool {
	switch action {
	case ActionConferenceHelloAck, ActionConferenceAnswer, ActionConferenceRoute, ActionConferenceRoutes:
		return true
	default:
		return false
	}
}

func conferenceDecode(data json.RawMessage, out any) error {
	if len(data) == 0 || len(data) > conferenceMaxControlBytes || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("invalid conference payload")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("trailing conference payload")
	}
	return nil
}

func (r *ConferenceRoom) handleHelloAck(p *conferencePeer, data json.RawMessage) error {
	var ack struct {
		Version int `json:"version"`
	}
	if err := conferenceDecode(data, &ack); err != nil {
		return err
	}
	if ack.Version != ConferenceVersion {
		err := errors.New("incompatible conference protocol")
		r.fail(p, err)
		return err
	}
	return r.enqueueControl(p, conferenceCommand{action: ActionConferenceHelloAck})
}

func (r *ConferenceRoom) handleAnswer(p *conferencePeer, data json.RawMessage) error {
	var answer ConferenceDescription
	if err := conferenceDecode(data, &answer); err != nil {
		return err
	}
	if answer.ID == 0 || strings.TrimSpace(answer.SDP) == "" {
		return errors.New("answer requires id and SDP")
	}
	return r.enqueueControl(p, conferenceCommand{action: ActionConferenceAnswer, answer: answer})
}

type conferenceRouteRequest struct {
	ID   uint64 `json:"id"`
	Rule *struct {
		Scope     string `json:"scope"`
		Source    string `json:"source"`
		Recipient string `json:"recipient"`
		Muted     *bool  `json:"muted"`
	} `json:"rule,omitempty"`
}

func (r *ConferenceRoom) handleRoute(p *conferencePeer, action string, data json.RawMessage) error {
	var request conferenceRouteRequest
	err := conferenceDecode(data, &request)
	if err == nil {
		err = r.applyRouteRequest(p, action, request)
	}
	result := ConferenceRouteResult{ID: request.ID, State: r.State(p.id, false)}
	if err != nil {
		result.Error = err.Error()
	}
	if sendErr := r.enqueueControl(p, conferenceCommand{action: ActionConferenceRouteResult, payload: result}); sendErr != nil {
		return sendErr
	}
	return err
}

func (r *ConferenceRoom) applyRouteRequest(p *conferencePeer, action string, request conferenceRouteRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !p.compatible || r.peers[p.id] != p || p.ctx.Err() != nil || request.ID == 0 {
		return errors.New("route requires compatible session and nonzero id")
	}
	if action == ActionConferenceRoutes {
		if request.Rule != nil {
			return errors.New("readback cannot contain a rule")
		}
		return nil
	}
	if request.Rule == nil || request.Rule.Muted == nil {
		return errors.New("route requires a complete rule including muted")
	}
	rule := request.Rule
	return r.setRuleLocked(p.id, false, echowarp.AudioRouteRule{Scope: rule.Scope, Source: rule.Source, Recipient: rule.Recipient, Muted: *rule.Muted})
}

func (r *ConferenceRoom) enqueueControl(p *conferencePeer, command conferenceCommand) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	select {
	case p.commands <- command:
		return nil
	default:
		err := errors.New("conference control queue full")
		r.fail(p, err)
		return err
	}
}
