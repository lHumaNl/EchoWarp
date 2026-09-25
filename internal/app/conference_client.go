package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

const (
	conferenceClientTimeout      = 5 * time.Second
	conferenceClientFrame        = 20 * time.Millisecond
	conferenceClientPendingLimit = 64
)

// The owner registers this session before the initial handshake, calls Ready on
// DCReady, and serially dispatches control messages. It retains peer ownership.
type conferenceClientSession struct {
	ctx                  context.Context
	cancel               context.CancelCauseFunc
	peer                 transport.PeerManager
	logger               *slog.Logger
	mixer                *audio.RTPMixer
	recording            *RecordingMixin // Immutable after startup; optional asynchronous tap.
	mu                   sync.Mutex
	sources              map[string]*conferenceClientSource
	pending              map[uint64]chan ConferenceRouteResult
	selfID               string
	lastOffer, requestID uint64
	helloDone            chan struct{}
	readyOnce            sync.Once
	running              atomic.Bool
	closed               bool
	wg                   sync.WaitGroup
	timeout              time.Duration
}

func newConferenceClientSession(ctx context.Context, peer transport.PeerManager, cfg config.Config, logger *slog.Logger) (*conferenceClientSession, error) {
	media, ok := peer.(transport.ConferenceMedia)
	if !ok || peer == nil {
		return nil, errors.New("conference peer requires RTP capability")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	depth := cfg.EffectiveAudioBufferFrames()
	mixer, err := audio.NewRTPMixer(int(cfg.SampleRate), int(cfg.Channels), depth, max(depth, min(30, max(10, depth*3))))
	if err != nil {
		return nil, err
	}
	s := makeConferenceClient(ctx, peer, mixer, logger)
	media.OnRTPTrack(s.attachTrack)
	context.AfterFunc(s.ctx, s.shutdown)
	return s, nil
}

func makeConferenceClient(ctx context.Context, peer transport.PeerManager, mixer *audio.RTPMixer, logger *slog.Logger) *conferenceClientSession {
	ctx, cancel := context.WithCancelCause(ctx)
	if logger == nil {
		logger = slog.Default()
	}
	return &conferenceClientSession{ctx: ctx, cancel: cancel, peer: peer, logger: logger,
		mixer: mixer, sources: make(map[string]*conferenceClientSource),
		pending: make(map[uint64]chan ConferenceRouteResult), helloDone: make(chan struct{}), timeout: conferenceClientTimeout}
}

// Ready starts the compatibility deadline once, not during TCP/ICE negotiation.
func (s *conferenceClientSession) Ready() {
	s.readyOnce.Do(func() { go s.awaitHello() })
}

func (s *conferenceClientSession) awaitHello() {
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
	case <-s.helloDone:
	case <-timer.C:
		s.fail(errors.New("conference hello timeout: incompatible server"))
	}
}

func (s *conferenceClientSession) fail(err error) {
	s.logger.Debug("Conference client session failed", "error", err)
	s.cancel(err)
}

func (s *conferenceClientSession) shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for id, source := range s.sources {
		source.stop()
		s.mixer.RemoveSource(id)
	}
	clear(s.sources)
	clear(s.pending) // Waiters select on the derived context; never close reply channels.
}

// Close joins session-owned RTP readers. The owner closes the underlying peer.
func (s *conferenceClientSession) Close() {
	s.cancel(context.Canceled)
	s.shutdown()
	s.wg.Wait()
}

// Run owns out and closes it only when this session exits. process is synchronous
// and must not block; a full playback queue drops a frame, never stalls controls.
func (s *conferenceClientSession) Run(out chan<- []float32, process func([]float32)) error {
	if out == nil || cap(out) == 0 {
		return errors.New("conference playback requires a bounded buffered channel")
	}
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("conference playback already started")
	}
	defer close(out)
	defer s.Close()
	ticker := time.NewTicker(conferenceClientFrame)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return context.Cause(s.ctx)
		case <-ticker.C:
			s.render(out, process)
		}
	}
}

func (s *conferenceClientSession) render(out chan<- []float32, process func([]float32)) {
	var recording *conferenceClientRecording
	if s.recording != nil {
		recording = s.recording.asyncRecording.Load()
	}
	var tap func(string, []float32)
	var tracks map[string][]float32
	if recording != nil {
		tracks = make(map[string][]float32)
		tap = func(id string, pcm []float32) { tracks[id] = append([]float32(nil), pcm...) }
	}
	frame := s.mixer.Render(tap)
	if recording != nil {
		recording.enqueue(frame, tracks)
	}
	if process != nil {
		process(frame)
	}
	select {
	case <-s.ctx.Done():
	case out <- frame:
	default:
	}
}
