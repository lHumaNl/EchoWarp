package app

import (
	"github.com/pion/rtp"

	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// This also bounds memory when a caller supplies a synthetic oversized packet.
const conferenceMaxPacketBytes = 8192

func (r *ConferenceRoom) attachInput(p *conferencePeer, info transport.RTPTrackInfo, packets <-chan *rtp.Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.peers[p.id] != p || p.ctx.Err() != nil {
		return
	}
	s := r.sources[p.id]
	// A replacement track can reuse its SSRC; it still needs fresh decoder state.
	if s.seen {
		r.serial++
		s.generation = r.serial
	}
	s.seen, s.ended, s.ssrc = true, false, info.SSRC
	p.incoming = packets
	p.inputVersion++
	conferenceWake(p.inputWake)
	r.changedLocked()
}

func (r *ConferenceRoom) inputLoop(p *conferencePeer) {
	defer r.wg.Done()
	for r.receiveInput(p) {
	}
}

func (r *ConferenceRoom) receiveInput(p *conferencePeer) bool {
	r.mu.RLock()
	incoming, version := p.incoming, p.inputVersion
	r.mu.RUnlock()
	select {
	case <-p.ctx.Done():
		return false
	case <-p.inputWake:
	case packet, ok := <-incoming:
		if !ok {
			r.endInput(p, version)
		} else {
			r.forwardInput(p, version, packet)
		}
	}
	return true
}

func (r *ConferenceRoom) forwardInput(p *conferencePeer, version uint64, packet *rtp.Packet) {
	r.mu.Lock()
	var observer func(string, *rtp.Packet)
	if r.peers[p.id] == p && p.inputVersion == version && p.compatible {
		r.writeSourceLocked(p.id, packet)
		observer = r.packetObserverLocked(p.id, packet)
	}
	r.mu.Unlock()
	if observer != nil {
		observer(p.id, packet)
	}
}

func (r *ConferenceRoom) endInput(p *conferencePeer, version uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.peers[p.id] == p && p.inputVersion == version {
		p.incoming = nil
		r.sources[p.id].ended = true
		r.changedLocked()
	}
}

// WriteSource clones bounded packets before returning; callers retain ownership.
// The method is trusted server integration, not a remote identity boundary.
func (r *ConferenceRoom) WriteSource(id string, packet *rtp.Packet) {
	r.mu.Lock()
	r.writeSourceLocked(id, packet)
	observer := r.packetObserverLocked(id, packet)
	r.mu.Unlock()
	if observer != nil {
		observer(id, packet)
	}
}

// ObservePackets installs a synchronous, nonblocking tap outside room locks.
// Packets are borrowed, read-only, and valid only during the callback: clone before
// retaining. Observers must only enqueue bounded work, never decode or do I/O.
// nil disables the tap; a callback already selected may finish after replacement.
func (r *ConferenceRoom) ObservePackets(observer func(string, *rtp.Packet)) {
	r.mu.Lock()
	r.packetObserver = observer
	r.mu.Unlock()
}

func (r *ConferenceRoom) packetObserverLocked(id string, packet *rtp.Packet) func(string, *rtp.Packet) {
	if r.closed || r.sources[id] == nil || packet == nil || packet.MarshalSize() > conferenceMaxPacketBytes {
		return nil
	}
	return r.packetObserver
}

func (r *ConferenceRoom) writeSourceLocked(id string, packet *rtp.Packet) {
	s := r.sources[id]
	if r.closed || s == nil || packet == nil || packet.MarshalSize() > conferenceMaxPacketBytes {
		return
	}
	if id == ConferenceServerID {
		r.serverGenerationLocked(s, packet.SSRC)
	}
	for _, p := range r.peers {
		if !r.deliverableLocked(id, p) {
			continue
		}
		// Drop newest on overflow. Never block another recipient or the source.
		select {
		case p.packets <- conferencePacket{id, packet.Clone(), p.writers[id].version, s.generation}:
		default:
		}
	}
}

func (r *ConferenceRoom) serverGenerationLocked(s *conferenceSource, ssrc uint32) {
	if s.seen && s.ssrc != ssrc {
		r.serial++
		s.generation = r.serial
		r.changedLocked()
	}
	s.seen, s.ssrc = true, ssrc
}

func (r *ConferenceRoom) deliverableLocked(id string, p *conferencePeer) bool {
	s := r.sources[id]
	w := p.writers[id]
	return p.ctx.Err() == nil && p.compatible && r.sourceAvailableLocked(id) && s != nil && w.writer != nil && w.negotiated && w.instance == s.instance && r.allowedLocked(id, p.id)
}

func (r *ConferenceRoom) sourceAvailableLocked(id string) bool {
	return id == ConferenceServerID || (r.peers[id] != nil && r.peers[id].compatible)
}

func (r *ConferenceRoom) mediaLoop(p *conferencePeer) {
	defer r.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case packet := <-p.packets:
			if err := r.writePacket(p, packet); err != nil {
				r.fail(p, err)
				return
			}
		}
	}
}

func (r *ConferenceRoom) writePacket(p *conferencePeer, packet conferencePacket) error {
	writer := r.packetWriter(p, packet)
	if writer == nil {
		return nil
	}
	err := writer.WriteRTP(packet.packet)
	// A source leaving can disable a writer already selected by this worker.
	// That expected stale-write failure must not disconnect unrelated sources.
	if err != nil && r.packetWriter(p, packet) == nil {
		return nil
	}
	return err
}

func (r *ConferenceRoom) packetWriter(p *conferencePeer, packet conferencePacket) transport.RTPWriter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := r.sources[packet.source]
	if s == nil || packet.version != p.writers[packet.source].version || packet.generation != s.generation || !r.deliverableLocked(packet.source, p) {
		return nil
	}
	return p.writers[packet.source].writer
}
