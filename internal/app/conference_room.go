package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/rtp"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

const (
	conferenceMaxSources            = 256
	conferenceMediaQueue            = 64
	conferenceControlQueue          = 64
	conferenceTimeout               = 15 * time.Second
	conferenceWatchInterval         = 100 * time.Millisecond
	conferenceDefaultGain   float32 = 1
	ConferenceServerID              = "server"
)

// ConferenceRoom owns session-scoped policy, signaling, and opaque RTP forwarding.
// No transport operation is performed while holding mu. Close must be called by
// the owner, not from a transport callback (it joins the room workers).
type ConferenceRoom struct {
	mu             sync.RWMutex
	logger         *slog.Logger
	peers          map[string]*conferencePeer
	sources        map[string]*conferenceSource
	rules          map[conferenceRuleKey]struct{}
	serial         uint64
	closed         bool
	wg             sync.WaitGroup
	packetObserver func(string, *rtp.Packet)
	sourceObserver func([]string)
	observerWake   chan struct{}
	observerDone   chan struct{}
}

type conferenceSource struct {
	instance, generation uint64
	ssrc                 uint32
	seen, ended          bool
	blocked, paused      bool
	incomingBlocked      bool // Independent legacy administrator receive gate.
	gain                 float32
}

type conferencePeer struct {
	id                                  string
	peer                                transport.PeerManager
	media                               transport.ConferenceMedia
	ctx                                 context.Context
	cancel                              context.CancelFunc
	wake, inputWake                     chan struct{}
	commands                            chan conferenceCommand
	packets                             chan conferencePacket
	writers                             map[string]conferenceWriter
	incoming                            <-chan *rtp.Packet
	inputVersion                        uint64
	ready, compatible, recipientBlocked bool
	deadline, ioDeadline                time.Time
	// The fields below belong exclusively to the signaling worker.
	hello                bool
	pending, offerSerial uint64
}

type conferenceWriter struct {
	writer     transport.RTPWriter
	instance   uint64
	negotiated bool
	version    uint64 // Delivery policy epoch for this source/recipient only.
}

type conferencePacket struct {
	source              string
	packet              *rtp.Packet
	version, generation uint64
}

func NewConferenceRoom(logger *slog.Logger) *ConferenceRoom {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConferenceRoom{logger: logger, peers: make(map[string]*conferencePeer),
		sources: make(map[string]*conferenceSource), rules: make(map[conferenceRuleKey]struct{})}
}

func (r *ConferenceRoom) AddPeer(ctx context.Context, id string, peer transport.PeerManager) error {
	media, ok := peer.(transport.ConferenceMedia)
	if !ok || !conferenceValidID(id) || id == ConferenceServerID || id == conferenceSelf {
		return errors.New("invalid conference peer or missing RTP capability")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p := newConferencePeer(ctx, id, peer, media)
	if err := r.registerPeer(p); err != nil {
		p.cancel()
		return err
	}
	media.OnRTPTrack(func(info transport.RTPTrackInfo, packets <-chan *rtp.Packet) { r.attachInput(p, info, packets) })
	r.startPeer(p)
	return nil
}

func (r *ConferenceRoom) startPeer(p *conferencePeer) {
	go r.supervise(p)
	go r.signalLoop(p)
	go r.mediaLoop(p)
	go r.inputLoop(p)
}

func newConferencePeer(ctx context.Context, id string, peer transport.PeerManager, media transport.ConferenceMedia) *conferencePeer {
	ctx, cancel := context.WithCancel(ctx)
	return &conferencePeer{id: id, peer: peer, media: media, ctx: ctx, cancel: cancel,
		wake: make(chan struct{}, 1), inputWake: make(chan struct{}, 1),
		commands: make(chan conferenceCommand, conferenceControlQueue),
		packets:  make(chan conferencePacket, conferenceMediaQueue), writers: make(map[string]conferenceWriter)}
}

func (r *ConferenceRoom) registerPeer(p *conferencePeer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || len(r.peers) >= conferenceMaxSources-1 || r.peers[p.id] != nil {
		return errors.New("conference closed, full, or duplicate peer")
	}
	r.peers[p.id] = p
	r.newSourceLocked(p.id)
	r.wg.Add(4)
	r.changedLocked()
	return nil
}

func (r *ConferenceRoom) newSourceLocked(id string) {
	r.serial++
	r.sources[id] = &conferenceSource{instance: r.serial, generation: r.serial, gain: conferenceDefaultGain}
}

func (r *ConferenceRoom) RemovePeer(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.peers[id]; p != nil {
		r.removeLocked(p)
	}
}

func (r *ConferenceRoom) removeLocked(p *conferencePeer) {
	if r.peers[p.id] != p {
		return
	}
	delete(r.peers, p.id)
	delete(r.sources, p.id)
	for key := range r.rules {
		if key.source == p.id || key.recipient == p.id {
			delete(r.rules, key)
		}
	}
	p.cancel()
	r.changedLocked()
}

func (r *ConferenceRoom) Ready(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.peers[id]; p != nil && !p.ready {
		p.ready = true
		conferenceWake(p.wake)
	}
}

func (r *ConferenceRoom) AddServerSource() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed && r.sources[ConferenceServerID] == nil {
		r.newSourceLocked(ConferenceServerID)
		r.changedLocked()
	}
}

func (r *ConferenceRoom) Close() {
	r.mu.Lock()
	if r.observerDone != nil && !r.closed {
		close(r.observerDone)
	}
	r.closed = true
	for _, p := range r.peers {
		r.removeLocked(p)
	}
	clear(r.sources)
	clear(r.rules)
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *ConferenceRoom) changedLocked() {
	conferenceWake(r.observerWake)
	for _, p := range r.peers {
		conferenceWake(p.wake)
	}
}

// ObserveSources replaces the roster observer. Snapshots are owned by the callback;
// notifications coalesce and run outside room locks. A callback must return; Close
// does not join a callback already in progress. nil disables future callbacks.
func (r *ConferenceRoom) ObserveSources(observer func([]string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sourceObserver = observer
	if r.observerWake == nil && !r.closed {
		r.observerWake, r.observerDone = make(chan struct{}, 1), make(chan struct{})
		go r.observeSourceLoop()
	}
	conferenceWake(r.observerWake)
}

func (r *ConferenceRoom) observeSourceLoop() {
	for {
		select {
		case <-r.observerDone:
			return
		case <-r.observerWake:
			r.notifySources()
		}
	}
}

func (r *ConferenceRoom) notifySources() {
	r.mu.RLock()
	observer := r.sourceObserver
	ids := make([]string, 0, len(r.sources))
	for id := range r.sources {
		ids = append(ids, id)
	}
	r.mu.RUnlock()
	if observer != nil {
		observer(ids)
	}
}

func conferenceWake(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (r *ConferenceRoom) supervise(p *conferencePeer) {
	defer r.wg.Done()
	ticker := time.NewTicker(conferenceWatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			r.closePeer(p)
			return
		case now := <-ticker.C:
			if r.peerExpired(p, now) {
				r.fail(p, errors.New("conference signaling timeout"))
			}
		}
	}
}

func (r *ConferenceRoom) closePeer(p *conferencePeer) {
	r.mu.Lock()
	r.removeLocked(p)
	r.mu.Unlock()
	if err := p.peer.Close(); err != nil {
		r.logger.Debug("Conference peer close failed", "peer", p.id, "error", err)
	}
}

func (r *ConferenceRoom) peerExpired(p *conferencePeer, now time.Time) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return conferenceExpired(now, p.deadline) || conferenceExpired(now, p.ioDeadline)
}

func conferenceExpired(now, deadline time.Time) bool {
	return !deadline.IsZero() && !now.Before(deadline)
}

func (r *ConferenceRoom) fail(p *conferencePeer, err error) {
	r.logger.Warn("Conference session failed", "peer", p.id, "error", err)
	p.cancel()
}

func (r *ConferenceRoom) State(actor string, admin bool) echowarp.AudioRouteState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stateLocked(actor, admin)
}
