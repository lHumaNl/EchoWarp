package app

import (
	"context"
	"errors"
	"math"

	"github.com/pion/rtp"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type conferenceClientSource struct {
	state ConferenceSourceState
	track *conferenceClientTrack
}

type conferenceClientTrack struct {
	ssrc   uint32
	cancel context.CancelFunc
}

func (source *conferenceClientSource) stop() {
	if source.track != nil {
		source.track.cancel()
	}
}

func validateConferenceClientSources(sources []ConferenceSourceState) error {
	if len(sources) > conferenceMaxSources {
		return errors.New("conference source limit exceeded")
	}
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		if !conferenceValidID(source.ID) || source.ID == conferenceSelf || seen[source.ID] || source.Generation == 0 ||
			source.Gain < 0 || math.IsNaN(float64(source.Gain)) || math.IsInf(float64(source.Gain), 0) {
			return errors.New("invalid conference source state")
		}
		seen[source.ID] = true
	}
	return nil
}

// Called with mu held. Source objects survive generation changes because the
// server can restart an upstream encoder without replacing the downstream track.
func (s *conferenceClientSession) reconcileSources(states []ConferenceSourceState) error {
	next := make(map[string]*conferenceClientSource, len(states))
	for _, state := range states {
		if state.ID == s.selfID {
			continue
		}
		source, err := s.reconcileSource(state)
		if err != nil {
			return err
		}
		next[state.ID] = source
	}
	for id, source := range s.sources {
		if next[id] == nil {
			source.stop()
			s.mixer.RemoveSource(id)
		}
	}
	s.sources = next
	return nil
}

func (s *conferenceClientSession) reconcileSource(state ConferenceSourceState) (*conferenceClientSource, error) {
	source := s.sources[state.ID]
	if source == nil {
		source = &conferenceClientSource{}
	}
	if source.state.Generation != state.Generation {
		s.mixer.RemoveSource(state.ID)
	}
	source.state = state
	if err := s.configureSource(source); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *conferenceClientSession) configureSource(source *conferenceClientSource) error {
	state := source.state
	if err := s.mixer.AddSource(state.ID); err != nil {
		return err
	}
	s.mixer.SetSourceEnabled(state.ID, state.Enabled)
	s.mixer.SetSourceGain(state.ID, state.Gain)
	return nil
}

func (s *conferenceClientSession) attachTrack(info transport.RTPTrackInfo, packets <-chan *rtp.Packet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.sources[info.SourceID]
	// Unknown tracks never create roster entries, even if they arrive late.
	if s.closed || s.ctx.Err() != nil || source == nil || packets == nil {
		return
	}
	if source.track != nil && source.track.ssrc == info.SSRC {
		return
	}
	source.stop()
	s.mixer.RemoveSource(info.SourceID)
	if err := s.configureSource(source); err != nil {
		s.fail(err)
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	track := &conferenceClientTrack{ssrc: info.SSRC, cancel: cancel}
	source.track = track
	s.wg.Add(1)
	go s.readTrack(ctx, source, track, packets)
}

func (s *conferenceClientSession) readTrack(ctx context.Context, source *conferenceClientSource, track *conferenceClientTrack, packets <-chan *rtp.Packet) {
	defer s.wg.Done()
	defer s.endTrack(source, track)
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-packets:
			if !ok {
				return
			}
			s.writePacket(source, track, packet)
		}
	}
}

func (s *conferenceClientSession) writePacket(source *conferenceClientSource, track *conferenceClientTrack, packet *rtp.Packet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sources[source.state.ID] == source && source.track == track && s.ctx.Err() == nil {
		s.mixer.WritePacket(source.state.ID, packet)
	}
}

func (s *conferenceClientSession) endTrack(source *conferenceClientSource, track *conferenceClientTrack) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sources[source.state.ID] == source && source.track == track {
		s.mixer.RemoveSource(source.state.ID)
		// Retain the token: an ended SSRC callback cannot restart this source.
	}
}
