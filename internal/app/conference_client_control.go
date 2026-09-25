package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/pion/webrtc/v4"
)

func (s *conferenceClientSession) HandleControl(action string, data json.RawMessage) (bool, error) {
	if !strings.HasPrefix(action, "conference_") {
		return false, nil
	}
	if err := s.ctx.Err(); err != nil {
		return true, context.Cause(s.ctx)
	}
	err := s.dispatchControl(action, data)
	if err != nil {
		s.fail(err)
	}
	return true, err
}

func (s *conferenceClientSession) dispatchControl(action string, data json.RawMessage) error {
	switch action {
	case ActionConferenceHello:
		return s.handleHello(data)
	case ActionConferenceState:
		return s.handleState(data)
	case ActionConferenceOffer:
		return s.handleOffer(data)
	case ActionConferenceRouteResult:
		return s.handleRouteResult(data)
	default:
		return errors.New("unsupported conference control")
	}
}

func (s *conferenceClientSession) handleHello(data json.RawMessage) error {
	var hello ConferenceHello
	if err := conferenceDecode(data, &hello); err != nil {
		return err
	}
	if hello.Version != ConferenceVersion || !conferenceValidID(hello.SelfID) ||
		hello.SelfID == conferenceSelf || hello.SelfID == ConferenceServerID {
		return errors.New("incompatible conference version or invalid self identity")
	}
	if err := s.acceptIdentity(hello.SelfID); err != nil {
		return err
	}
	return s.peer.SendControl(ActionConferenceHelloAck, struct {
		Version int `json:"version"`
	}{ConferenceVersion})
}

func (s *conferenceClientSession) acceptIdentity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selfID != "" {
		return errors.New("duplicate conference hello")
	}
	s.selfID = id
	close(s.helloDone)
	return nil
}

func (s *conferenceClientSession) handleOffer(data json.RawMessage) error {
	var offer ConferenceDescription
	if err := conferenceDecode(data, &offer); err != nil {
		return err
	}
	if err := s.acceptOffer(offer); err != nil {
		return err
	}
	answer, err := s.peer.CreateAnswer(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer.SDP})
	if err != nil {
		return err
	}
	if answer.Type != webrtc.SDPTypeAnswer || strings.TrimSpace(answer.SDP) == "" {
		return errors.New("invalid conference answer")
	}
	return s.peer.SendControl(ActionConferenceAnswer, ConferenceDescription{ID: offer.ID, SDP: answer.SDP})
}

func (s *conferenceClientSession) acceptOffer(offer ConferenceDescription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selfID == "" || offer.ID == 0 || offer.ID <= s.lastOffer || strings.TrimSpace(offer.SDP) == "" {
		return errors.New("unsolicited, stale, or invalid conference offer")
	}
	s.lastOffer = offer.ID
	return nil
}

func (s *conferenceClientSession) handleState(data json.RawMessage) error {
	var state ConferenceStateMessage
	if err := conferenceDecode(data, &state); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateRouteState(state.Routes); err != nil {
		return err
	}
	if err := validateConferenceClientSources(state.Sources); err != nil {
		return err
	}
	return s.reconcileSources(state.Sources)
}

func (s *conferenceClientSession) handleRouteResult(data json.RawMessage) error {
	var result ConferenceRouteResult
	if err := conferenceDecode(data, &result); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if result.ID == 0 || result.ID > s.requestID {
		return errors.New("unsolicited conference route result")
	}
	if err := s.validateRouteState(result.State); err != nil {
		return err
	}
	if reply := s.pending[result.ID]; reply != nil {
		delete(s.pending, result.ID)
		reply <- result
	}
	return nil // A canceled caller can leave a valid, late acknowledgment.
}
