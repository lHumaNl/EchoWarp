package app

import (
	"errors"
	"math"
	"regexp"
	"sort"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
)

const (
	conferenceReceive  = "receive"
	conferenceSend     = "send"
	conferenceAdmin    = "admin"
	conferenceWildcard = "*"
	conferenceSelf     = "self"
	conferenceMaxID    = 128
)

var conferenceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

type conferenceRuleKey struct{ scope, source, recipient string }

func conferenceValidID(id string) bool {
	return id != "" && len(id) <= conferenceMaxID && conferenceIDPattern.MatchString(id)
}

func (r *ConferenceRoom) SetRule(actor string, admin bool, rule echowarp.AudioRouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setRuleLocked(actor, admin, rule)
}

func (r *ConferenceRoom) setRuleLocked(actor string, admin bool, rule echowarp.AudioRouteRule) error {
	key, err := r.authorizeLocked(actor, admin, rule)
	if err != nil {
		return err
	}
	_, exists := r.rules[key]
	if exists == rule.Muted {
		return nil
	}
	if rule.Muted {
		r.rules[key] = struct{}{}
	} else {
		delete(r.rules, key)
	}
	// Invalidate queued media even if an unmute follows before a worker wakes.
	r.invalidateDeliveryLocked(key.source, key.recipient)
	r.changedLocked()
	return nil
}

func (r *ConferenceRoom) authorizeLocked(actor string, admin bool, rule echowarp.AudioRouteRule) (conferenceRuleKey, error) {
	key := conferenceRuleKey{rule.Scope, rule.Source, rule.Recipient}
	if r.closed || (actor != ConferenceServerID && r.peers[actor] == nil) {
		return key, errors.New("unknown conference actor")
	}
	if err := conferenceResolveScope(actor, admin, &key); err != nil {
		return key, err
	}
	if !r.ruleIDLocked(key.source) || !r.ruleIDLocked(key.recipient) {
		return key, errors.New("route references an invalid or absent participant")
	}
	return key, nil
}

func conferenceResolveScope(actor string, admin bool, key *conferenceRuleKey) error {
	switch key.scope {
	case conferenceReceive:
		return conferenceResolveSelf(actor, &key.recipient)
	case conferenceSend:
		return conferenceResolveSelf(actor, &key.source)
	case conferenceAdmin:
		if !admin || actor != ConferenceServerID {
			return errors.New("admin routes require the server actor")
		}
	default:
		return errors.New("invalid route scope")
	}
	return nil
}

func conferenceResolveSelf(actor string, id *string) error {
	if *id != conferenceSelf && *id != actor {
		return errors.New("route owner must be self")
	}
	*id = actor
	return nil
}

func (r *ConferenceRoom) ruleIDLocked(id string) bool {
	return id == conferenceWildcard || id == ConferenceServerID || (conferenceValidID(id) && r.peers[id] != nil)
}

func (r *ConferenceRoom) allowedLocked(source, recipient string) bool {
	s, p := r.sources[source], r.peers[recipient]
	if s == nil || p == nil || source == recipient || s.ended || s.blocked || s.incomingBlocked || s.paused || p.recipientBlocked {
		return false
	}
	for _, scope := range []string{conferenceReceive, conferenceSend, conferenceAdmin} {
		for _, from := range []string{source, conferenceWildcard} {
			for _, to := range []string{recipient, conferenceWildcard} {
				if _, denied := r.rules[conferenceRuleKey{scope, from, to}]; denied {
					return false
				}
			}
		}
	}
	return true
}

func (r *ConferenceRoom) stateLocked(actor string, admin bool) echowarp.AudioRouteState {
	state := echowarp.AudioRouteState{SelfID: actor, Rules: []echowarp.AudioRouteRule{}}
	if actor != ConferenceServerID && r.peers[actor] == nil {
		return state
	}
	for key := range r.rules {
		if (admin && actor == ConferenceServerID) || conferenceVisibleRule(key, actor) {
			state.Rules = append(state.Rules, echowarp.AudioRouteRule{Scope: key.scope, Source: key.source, Recipient: key.recipient, Muted: true})
		}
	}
	sort.Slice(state.Rules, func(i, j int) bool { return conferenceRuleOrder(state.Rules[i]) < conferenceRuleOrder(state.Rules[j]) })
	return state
}

func conferenceVisibleRule(key conferenceRuleKey, actor string) bool {
	switch key.scope {
	case conferenceReceive:
		return key.recipient == actor
	case conferenceSend:
		return key.source == actor
	case conferenceAdmin:
		return key.source == actor || key.recipient == actor || key.source == conferenceWildcard || key.recipient == conferenceWildcard
	default:
		return false
	}
}

func conferenceRuleOrder(rule echowarp.AudioRouteRule) string {
	return rule.Scope + "\x00" + rule.Source + "\x00" + rule.Recipient
}

func (r *ConferenceRoom) SetSourceBlocked(id string, blocked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sources[id]; s != nil {
		s.blocked = blocked
		r.invalidateDeliveryLocked(id, conferenceWildcard)
		r.changedLocked()
	}
}

func (r *ConferenceRoom) SetIncomingBlocked(id string, blocked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sources[id]; s != nil {
		s.incomingBlocked = blocked
		r.invalidateDeliveryLocked(id, conferenceWildcard)
		r.changedLocked()
	}
}

func (r *ConferenceRoom) SetRecipientBlocked(id string, blocked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.peers[id]; p != nil {
		p.recipientBlocked = blocked
		r.invalidateDeliveryLocked(conferenceWildcard, id)
		r.changedLocked()
	}
}

func (r *ConferenceRoom) SetSourcePaused(id string, paused bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sources[id]; s != nil {
		s.paused = paused
		r.invalidateDeliveryLocked(id, conferenceWildcard)
		r.changedLocked()
	}
}

// A client's own policy change must not flush media on unrelated directions.
func (r *ConferenceRoom) invalidateDeliveryLocked(source, recipient string) {
	for id, peer := range r.peers {
		if recipient != conferenceWildcard && recipient != id {
			continue
		}
		for sourceID, writer := range peer.writers {
			if source != conferenceWildcard && source != sourceID {
				continue
			}
			writer.version++
			peer.writers[sourceID] = writer
		}
	}
}

func (r *ConferenceRoom) SetSourceGain(id string, gain float32) {
	if gain < 0 || math.IsNaN(float64(gain)) || math.IsInf(float64(gain), 0) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.sources[id]; s != nil {
		s.gain = gain
		r.changedLocked()
	}
}
