package app

import (
	"context"
	"errors"
	"strings"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

type conferenceClientRequest struct {
	ID   uint64                   `json:"id"`
	Rule *echowarp.AudioRouteRule `json:"rule,omitempty"`
}

func (s *conferenceClientSession) SetAudioRoute(ctx context.Context, rule echowarp.AudioRouteRule) error {
	s.mu.Lock()
	err := s.validateRule(rule)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	_, err = s.requestRoutes(ctx, &rule)
	return err
}

func (s *conferenceClientSession) AudioRoutes(ctx context.Context) (echowarp.AudioRouteState, error) {
	return s.requestRoutes(ctx, nil)
}

func (s *conferenceClientSession) validateRule(rule echowarp.AudioRouteRule) error {
	if !conferenceClientRouteID(rule.Source) || !conferenceClientRouteID(rule.Recipient) {
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Invalid audio route identity")
	}
	key := conferenceRuleKey{rule.Scope, rule.Source, rule.Recipient}
	if rule.Scope != conferenceReceive && rule.Scope != conferenceSend && rule.Scope != conferenceAdmin {
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Invalid audio route scope")
	}
	if err := conferenceResolveScope(s.selfID, false, &key); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrAuthFailed, "Forbidden audio route")
	}
	return nil
}

func conferenceClientRouteID(id string) bool {
	return id == conferenceWildcard || conferenceValidID(id)
}

func (s *conferenceClientSession) requestRoutes(ctx context.Context, rule *echowarp.AudioRouteRule) (echowarp.AudioRouteState, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return echowarp.AudioRouteState{}, err
	}
	id, reply, err := s.reserveRequest()
	if err != nil {
		return echowarp.AudioRouteState{}, err
	}
	defer s.releaseRequest(id)
	action := ActionConferenceRoutes
	if rule != nil {
		action = ActionConferenceRoute
	}
	if err := s.peer.SendControl(action, conferenceClientRequest{ID: id, Rule: rule}); err != nil {
		s.fail(err)
		return echowarp.AudioRouteState{}, err
	}
	return s.awaitRouteResult(ctx, reply)
}

func (s *conferenceClientSession) reserveRequest() (uint64, chan ConferenceRouteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return 0, nil, context.Cause(s.ctx)
	}
	if s.selfID == "" {
		return 0, nil, ewerrors.NewError(ewerrors.ErrNotRunning, "Conference hello not received")
	}
	if len(s.pending) >= conferenceClientPendingLimit {
		return 0, nil, errors.New("conference request limit reached")
	}
	s.requestID++
	if s.requestID == 0 {
		return 0, nil, errors.New("conference request ID exhausted")
	}
	reply := make(chan ConferenceRouteResult, 1)
	s.pending[s.requestID] = reply
	return s.requestID, reply, nil
}

func (s *conferenceClientSession) releaseRequest(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
}

func (s *conferenceClientSession) awaitRouteResult(ctx context.Context, reply <-chan ConferenceRouteResult) (echowarp.AudioRouteState, error) {
	select {
	case <-ctx.Done():
		return echowarp.AudioRouteState{}, ctx.Err()
	case <-s.ctx.Done():
		return echowarp.AudioRouteState{}, context.Cause(s.ctx)
	case result := <-reply:
		if err := ctx.Err(); err != nil {
			return echowarp.AudioRouteState{}, err
		}
		if err := s.ctx.Err(); err != nil {
			return echowarp.AudioRouteState{}, context.Cause(s.ctx)
		}
		if result.Error != "" {
			return echowarp.AudioRouteState{}, conferenceClientRouteError(result.Error)
		}
		result.State.Rules = append([]echowarp.AudioRouteRule{}, result.State.Rules...)
		return result.State, nil
	}
}

func conferenceClientRouteError(message string) error {
	code := ewerrors.ErrConfigValidation
	for _, word := range []string{"forbidden", "unauthorized", "permission", "owner must be self", "admin routes require"} {
		if strings.Contains(strings.ToLower(message), word) {
			code = ewerrors.ErrAuthFailed
			break
		}
	}
	return ewerrors.NewError(code, message)
}

// Called under mu. Results must belong to the authenticated hello identity.
func (s *conferenceClientSession) validateRouteState(state echowarp.AudioRouteState) error {
	if s.selfID == "" || state.SelfID != s.selfID {
		return errors.New("invalid conference state identity")
	}
	for _, rule := range state.Rules {
		if !conferenceClientRouteID(rule.Source) || !conferenceClientRouteID(rule.Recipient) ||
			(rule.Scope != conferenceReceive && rule.Scope != conferenceSend && rule.Scope != conferenceAdmin) {
			return errors.New("invalid conference route state")
		}
	}
	return nil
}
