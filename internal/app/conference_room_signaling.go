package app

import (
	"errors"
	"sort"
	"time"

	"github.com/pion/webrtc/v4"
)

func (r *ConferenceRoom) signalLoop(p *conferencePeer) {
	defer r.wg.Done()
	for {
		var command conferenceCommand
		select {
		case <-p.ctx.Done():
			return
		case <-p.wake:
		case command = <-p.commands:
		}
		if p.ctx.Err() != nil {
			return
		}
		if err := r.signalStep(p, command); err != nil {
			r.fail(p, err)
			return
		}
	}
}

func (r *ConferenceRoom) signalStep(p *conferencePeer, command conferenceCommand) error {
	r.mu.Lock()
	p.ioDeadline = time.Now().Add(conferenceTimeout)
	r.mu.Unlock()
	defer func() { r.mu.Lock(); p.ioDeadline = time.Time{}; r.mu.Unlock() }()
	switch command.action {
	case ActionConferenceHelloAck:
		return r.acceptHello(p)
	case ActionConferenceAnswer:
		return r.acceptAnswer(p, command.answer)
	case ActionConferenceRouteResult:
		return p.peer.SendControl(command.action, command.payload)
	default:
		return r.reconcileReady(p)
	}
}

func (r *ConferenceRoom) reconcileReady(p *conferencePeer) error {
	r.mu.RLock()
	ready, compatible := p.ready, p.compatible
	r.mu.RUnlock()
	if !ready {
		return nil
	}
	if !p.hello {
		p.hello = true
		r.setDeadline(p, true)
		return p.peer.SendControl(ActionConferenceHello, ConferenceHello{Version: ConferenceVersion, SelfID: p.id})
	}
	if !compatible {
		return nil
	}
	return r.reconcileCompatible(p)
}

func (r *ConferenceRoom) reconcileCompatible(p *conferencePeer) error {
	if err := p.peer.SendControl(ActionConferenceState, r.sourceState(p)); err != nil {
		return err
	}
	if p.pending != 0 {
		return nil
	}
	return r.reconcileTracks(p)
}

func (r *ConferenceRoom) acceptHello(p *conferencePeer) error {
	if !p.hello {
		return errors.New("unsolicited conference hello acknowledgment")
	}
	r.mu.Lock()
	if r.peers[p.id] != p || p.compatible {
		r.mu.Unlock()
		return nil
	}
	p.compatible = true
	p.deadline = time.Time{}
	r.changedLocked()
	r.mu.Unlock()
	return r.reconcileReady(p)
}

func (r *ConferenceRoom) acceptAnswer(p *conferencePeer, answer ConferenceDescription) error {
	// Stale/unsolicited IDs cannot mutate SDP or release the current offer.
	if p.pending == 0 || answer.ID != p.pending {
		return nil
	}
	if err := p.peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer.SDP}); err != nil {
		return err
	}
	r.mu.Lock()
	for id, writer := range p.writers {
		writer.negotiated = true
		p.writers[id] = writer
	}
	p.deadline = time.Time{}
	r.mu.Unlock()
	p.pending = 0
	return r.reconcileReady(p)
}

func (r *ConferenceRoom) setDeadline(p *conferencePeer, active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.deadline = time.Time{}
	if active {
		p.deadline = time.Now().Add(conferenceTimeout)
	}
}

func (r *ConferenceRoom) sourceState(p *conferencePeer) ConferenceStateMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := ConferenceStateMessage{Sources: make([]ConferenceSourceState, 0, len(r.sources)), Routes: r.stateLocked(p.id, false)}
	for id, s := range r.sources {
		state.Sources = append(state.Sources, ConferenceSourceState{ID: id, Enabled: p.compatible && r.sourceAvailableLocked(id) && r.allowedLocked(id, p.id), Gain: s.gain, Generation: s.generation})
	}
	sort.Slice(state.Sources, func(i, j int) bool { return state.Sources[i].ID < state.Sources[j].ID })
	return state
}

func (r *ConferenceRoom) desiredTracks(p *conferencePeer) map[string]uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	desired := make(map[string]uint64, len(r.sources))
	for id, source := range r.sources {
		if id != p.id {
			desired[id] = source.instance
		}
	}
	return desired
}

func (r *ConferenceRoom) reconcileTracks(p *conferencePeer) error {
	// Ready is a caller guarantee that the initial answer is installed: the
	// existing PeerManager does not expose Pion's signaling-state getter.
	desired := r.desiredTracks(p)
	removed, err := r.removeTracks(p, desired)
	if err != nil {
		return err
	}
	added, err := r.addTracks(p, desired)
	if err != nil {
		return err
	}
	if !removed && !added {
		return nil
	}
	return r.offer(p)
}

func (r *ConferenceRoom) removeTracks(p *conferencePeer, desired map[string]uint64) (bool, error) {
	r.mu.Lock()
	var removed []string
	for id, writer := range p.writers {
		if desired[id] != writer.instance {
			removed = append(removed, id)
			delete(p.writers, id)
		}
	}
	r.mu.Unlock()
	for _, id := range removed {
		if err := p.media.RemoveRTPTrack(id); err != nil {
			return false, err
		}
	}
	return len(removed) != 0, nil
}

func (r *ConferenceRoom) addTracks(p *conferencePeer, desired map[string]uint64) (bool, error) {
	added := false
	for id, instance := range desired {
		if r.hasWriter(p, id) {
			continue
		}
		changed, err := r.addTrack(p, id, instance)
		if err != nil {
			return false, err
		}
		added = added || changed
	}
	return added, nil
}

func (r *ConferenceRoom) hasWriter(p *conferencePeer, id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return p.writers[id].writer != nil
}

func (r *ConferenceRoom) addTrack(p *conferencePeer, id string, instance uint64) (bool, error) {
	if p.ctx.Err() != nil {
		return false, p.ctx.Err()
	}
	writer, err := p.media.AddRTPTrack(id)
	if err != nil {
		return false, err
	}
	if writer == nil {
		return false, errors.New("conference transport returned nil RTP writer")
	}
	r.mu.Lock()
	p.writers[id] = conferenceWriter{writer: writer, instance: instance}
	r.mu.Unlock()
	return true, nil
}

func (r *ConferenceRoom) offer(p *conferencePeer) error {
	r.setDeadline(p, true)
	offer, err := p.peer.CreateOffer()
	if err != nil {
		return err
	}
	if offer.Type != webrtc.SDPTypeOffer || offer.SDP == "" {
		return errors.New("invalid conference offer")
	}
	p.offerSerial++
	p.pending = p.offerSerial
	return p.peer.SendControl(ActionConferenceOffer, ConferenceDescription{ID: p.pending, SDP: offer.SDP})
}
