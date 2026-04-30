package app

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type monitorPlayerStarter func(context.Context, *slog.Logger, config.Config, <-chan []float32, chan<- error, <-chan struct{}, chan<- error)

const monitorPlayerStartupTimeout = 5 * time.Second

// ServerMonitorMixer is the single local monitor path for multi-client duplex.
// It combines decoded client receive sources into one server playback stream;
// the mixed frame sent to playback is also the AEC far-end reference.
type ServerMonitorMixer struct {
	cfg     ServerMonitorMixerConfig
	mu      sync.RWMutex
	sources map[string]*monitorSourceBuffer
	running atomic.Bool
}

type ServerMonitorMixerConfig struct {
	SampleRate      uint32
	Channels        uint32
	FrameSize       int
	BufferFrames    int
	PlaybackGain    *DeviceGainControl
	PlaybackAGC     *audio.AGCProcessor
	Spectrum        *audio.SpectrumAnalyzer
	LevelMeter      *audio.LevelMeter
	ReferenceFeeder func([]float32)
}

type monitorSourceBuffer struct {
	mu       sync.Mutex
	buf      [][]float32
	read     int
	writeIdx int
	count    int
}

// NewServerMonitorMixer creates the monitor mixer. FrameSize defaults to 20ms.
func NewServerMonitorMixer(cfg ServerMonitorMixerConfig) *ServerMonitorMixer {
	if cfg.FrameSize <= 0 {
		cfg.FrameSize = int(cfg.SampleRate) / 50 * int(cfg.Channels)
	}
	if cfg.BufferFrames <= 0 {
		cfg.BufferFrames = 3
	}
	return &ServerMonitorMixer{cfg: cfg, sources: make(map[string]*monitorSourceBuffer)}
}

// AddSource registers or replaces a client source.
func (m *ServerMonitorMixer) AddSource(clientID string) {
	m.mu.Lock()
	m.sources[clientID] = newMonitorSourceBuffer(m.cfg.BufferFrames)
	m.mu.Unlock()
}

// RemoveSource removes a client source from future monitor mixes.
func (m *ServerMonitorMixer) RemoveSource(clientID string) {
	m.mu.Lock()
	delete(m.sources, clientID)
	m.mu.Unlock()
}

// SourceCount returns the number of registered monitor sources.
func (m *ServerMonitorMixer) SourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sources)
}

// SubmitSourceFrame queues one decoded/jittered frame for a source.
func (m *ServerMonitorMixer) SubmitSourceFrame(clientID string, frame []float32) {
	m.mu.RLock()
	src := m.sources[clientID]
	m.mu.RUnlock()
	if src != nil {
		src.write(frame)
	}
}

// MixOnce returns the next actual monitor frame and feeds the same frame as AEC reference.
func (m *ServerMonitorMixer) MixOnce(ctx context.Context) []float32 {
	mixed := make([]float32, m.cfg.FrameSize)
	m.mixSources(mixed)
	applyPlaybackGainMute(ctx, mixed, m.cfg.PlaybackGain, nil, m.cfg.PlaybackAGC)
	m.feedMetersAndReference(mixed)
	return mixed
}

// Run produces monitor frames until ctx is canceled.
func (m *ServerMonitorMixer) Run(ctx context.Context, out chan<- []float32) error {
	if !m.running.CompareAndSwap(false, true) {
		return context.Canceled
	}
	defer m.running.Store(false)
	ticker := time.NewTicker(m.frameDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.sendMixedFrame(ctx, out); err != nil {
				return err
			}
		}
	}
}

func (m *ServerMonitorMixer) mixSources(mixed []float32) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, src := range m.sources {
		frame := src.readFrame()
		if frame != nil {
			audio.MixAccumulate(mixed, frame)
		}
	}
	audio.MixTanh(mixed)
}

func (m *ServerMonitorMixer) feedMetersAndReference(frame []float32) {
	if m.cfg.Spectrum != nil {
		m.cfg.Spectrum.Feed(frame)
	}
	if m.cfg.LevelMeter != nil {
		m.cfg.LevelMeter.Feed(frame)
	}
	if m.cfg.ReferenceFeeder != nil {
		m.cfg.ReferenceFeeder(frame)
	}
}

func (m *ServerMonitorMixer) frameDuration() time.Duration {
	if m.cfg.SampleRate == 0 || m.cfg.Channels == 0 {
		return 20 * time.Millisecond
	}
	seconds := float64(m.cfg.FrameSize) / float64(m.cfg.SampleRate*m.cfg.Channels)
	if seconds <= 0 {
		return 20 * time.Millisecond
	}
	return time.Duration(seconds * float64(time.Second))
}

func (m *ServerMonitorMixer) sendMixedFrame(ctx context.Context, out chan<- []float32) error {
	select {
	case out <- m.MixOnce(ctx):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newMonitorSourceBuffer(capacity int) *monitorSourceBuffer {
	return &monitorSourceBuffer{buf: make([][]float32, capacity)}
}

func (b *monitorSourceBuffer) write(frame []float32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	copyFrame := append([]float32(nil), frame...)
	if b.count == len(b.buf) {
		b.read = (b.read + 1) % len(b.buf)
		b.count--
	}
	b.buf[b.writeIdx] = copyFrame
	b.writeIdx = (b.writeIdx + 1) % len(b.buf)
	b.count++
}

func (b *monitorSourceBuffer) readFrame() []float32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count == 0 {
		return nil
	}
	frame := b.buf[b.read]
	b.buf[b.read] = nil
	b.read = (b.read + 1) % len(b.buf)
	b.count--
	return frame
}

func (s *ServerApp) setupServerMonitorSource(serverCtx, clientCtx context.Context, peer transport.PeerManager, clientID string, muteIncoming *atomic.Bool, audioDone chan<- error) error {
	mixer, err := s.ensureServerMonitorMixer(serverCtx)
	if err != nil {
		return err
	}
	mixer.AddSource(clientID)
	go s.removeMonitorSourceOnDone(clientCtx, mixer, clientID)
	s.startMonitorDecodeSource(clientCtx, peer, mixer, clientID, muteIncoming, audioDone)
	return nil
}

func (s *ServerApp) ensureServerMonitorMixer(ctx context.Context) (*ServerMonitorMixer, error) {
	s.monitorMixerMu.Lock()
	defer s.monitorMixerMu.Unlock()
	if s.monitorMixer != nil {
		return s.monitorMixer, nil
	}
	mixer := NewServerMonitorMixer(s.serverMonitorMixerConfig())
	if err := s.startServerMonitorPlayback(ctx, mixer); err != nil {
		s.monitorPlaybackGain = nil
		s.monitorPlaybackAGC = nil
		return nil, err
	}
	s.monitorMixer = mixer
	return mixer, nil
}

func (s *ServerApp) serverMonitorMixerConfig() ServerMonitorMixerConfig {
	gainCtl, agcProc, agcMap := s.serverMonitorPlaybackControls()
	s.monitorPlaybackGain = gainCtl
	s.monitorPlaybackAGC = agcMap
	return ServerMonitorMixerConfig{
		SampleRate:      s.cfg.SampleRate,
		Channels:        s.cfg.Channels,
		BufferFrames:    s.cfg.EffectiveAudioBufferFrames(),
		PlaybackGain:    gainCtl,
		PlaybackAGC:     agcProc,
		Spectrum:        s.spectrum,
		LevelMeter:      s.levelMeter,
		ReferenceFeeder: s.feedMonitorAECReference,
	}
}

func (s *ServerApp) serverMonitorPlaybackControls() (*DeviceGainControl, *audio.AGCProcessor, map[uint32]*audio.AGCProcessor) {
	playbackDevs := s.cfg.PlaybackDevices()
	initialVol := initialPlaybackVolume(playbackDevs)
	gainCtl := NewDeviceGainControl(initialVol)
	agcMap := buildAGCProcessors(playbackDevs, s.cfg.SampleRate)
	if len(playbackDevs) != 1 {
		return gainCtl, nil, agcMap
	}
	return gainCtl, agcMap[playbackDevs[0].ID], agcMap
}

func initialPlaybackVolume(devices []config.DeviceEntry) float32 {
	if len(devices) == 1 && devices[0].Volume > 0 {
		return float32(devices[0].Volume)
	}
	return 1.0
}

func (s *ServerApp) feedMonitorAECReference(frame []float32) {
	if s.aec != nil && s.aec.IsEnabled() {
		s.aec.FeedReference(frame)
	}
}

func (s *ServerApp) startServerMonitorPlayback(ctx context.Context, mixer *ServerMonitorMixer) error {
	monitorCtx, cancel := context.WithCancel(ctx)
	playbackCh := make(chan []float32, 5)
	readyCh := make(chan struct{})
	monitorDone := make(chan error, 2)
	startupCh := make(chan error, 1)
	close(readyCh)
	starter := s.monitorPlayerStart
	if starter == nil {
		starter = startMonitorAudioPlayer
	}
	starter(monitorCtx, s.logger, s.serverMonitorPlaybackConfig(), playbackCh, monitorDone, readyCh, startupCh)
	if err := waitMonitorPlayerStartup(ctx, startupCh); err != nil {
		cancel()
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "start server monitor playback")
	}
	go s.runServerMonitorMixer(monitorCtx, mixer, playbackCh, monitorDone)
	go s.reportServerMonitorErrors(monitorCtx, monitorDone, cancel)
	return nil
}

func startMonitorAudioPlayer(ctx context.Context, logger *slog.Logger, cfg config.Config, playbackCh <-chan []float32, audioDone chan<- error, readyCh <-chan struct{}, startupCh chan<- error) {
	startAudioPlayerWithStartup(ctx, logger, cfg, playbackCh, audioDone, readyCh, startupCh)
}

func waitMonitorPlayerStartup(ctx context.Context, startupCh <-chan error) error {
	timer := time.NewTimer(monitorPlayerStartupTimeout)
	defer timer.Stop()
	select {
	case err := <-startupCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ewerrors.NewError(ewerrors.ErrInternalState, "server monitor player startup timed out")
	}
}

func (s *ServerApp) runServerMonitorMixer(ctx context.Context, mixer *ServerMonitorMixer, playbackCh chan<- []float32, monitorDone chan<- error) {
	err := mixer.Run(ctx, playbackCh)
	s.resetServerMonitorMixer(mixer)
	if err != nil && ctx.Err() == nil {
		monitorDone <- ewerrors.Wrap(err, ewerrors.ErrInternalState, "server monitor mixer stopped")
	}
}

func (s *ServerApp) serverMonitorPlaybackConfig() config.Config {
	cfg := s.cfg
	if playbackID, ok := s.serverMonitorPlaybackDeviceID(); ok {
		cfg.DeviceID = &playbackID
	}
	return cfg
}

func (s *ServerApp) serverMonitorPlaybackDeviceID() (uint32, bool) {
	if devices := s.cfg.PlaybackDevices(); len(devices) > 0 {
		return devices[0].ID, true
	}
	if s.cfg.OutputDeviceID != nil {
		return *s.cfg.OutputDeviceID, true
	}
	if s.cfg.DeviceID != nil {
		return *s.cfg.DeviceID, true
	}
	return 0, false
}

func (s *ServerApp) resetServerMonitorMixer(mixer *ServerMonitorMixer) {
	s.monitorMixerMu.Lock()
	defer s.monitorMixerMu.Unlock()
	if s.monitorMixer == mixer {
		s.monitorMixer = nil
		s.monitorPlaybackGain = nil
		s.monitorPlaybackAGC = nil
	}
}

func (s *ServerApp) reportServerMonitorErrors(ctx context.Context, monitorDone <-chan error, cancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-monitorDone:
			if s.handleServerMonitorError(ctx, err) {
				cancel()
			}
		}
	}
}

func (s *ServerApp) handleServerMonitorError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	s.logger.Warn("Server monitor playback error", "error", err)
	if s.errCh != nil {
		select {
		case s.errCh <- err:
		default:
		}
	}
	return true
}

func (s *ServerApp) removeMonitorSourceOnDone(ctx context.Context, mixer *ServerMonitorMixer, clientID string) {
	<-ctx.Done()
	mixer.RemoveSource(clientID)
}

func (s *ServerApp) startMonitorDecodeSource(ctx context.Context, peer transport.PeerManager, mixer *ServerMonitorMixer, clientID string, muteIncoming *atomic.Bool, audioDone chan<- error) {
	decodeCh := make(chan []float32, s.cfg.EffectiveAudioBufferFrames())
	doneCh := make(chan struct{})
	setupAudioDecoder(s.logger, peer, s.cfg.SampleRate, s.cfg.Channels, decodeCh, nil, nil)
	jb := s.newMonitorJitterBuffer()
	go s.monitorDecodeToJitter(ctx, decodeCh, jb, muteIncoming, doneCh)
	go func() {
		jitterMonitorSourcePump(ctx, jb, mixer, clientID, doneCh)
		audioDone <- nil
	}()
}

func (s *ServerApp) newMonitorJitterBuffer() *audio.JitterBuffer {
	target := s.cfg.EffectiveAudioBufferFrames()
	maxFrames := target * 3
	if maxFrames < 10 {
		maxFrames = 10
	}
	if maxFrames > 30 {
		maxFrames = 30
	}
	return audio.NewJitterBuffer(target, maxFrames)
}

func (s *ServerApp) monitorDecodeToJitter(ctx context.Context, decodeCh <-chan []float32, jb *audio.JitterBuffer, muteIncoming *atomic.Bool, doneCh chan<- struct{}) {
	defer close(doneCh)
	for {
		select {
		case <-ctx.Done():
			return
		case samples, ok := <-decodeCh:
			if !ok {
				return
			}
			if muteIncoming != nil && muteIncoming.Load() {
				continue
			}
			jb.Write(samples)
		}
	}
}

func jitterMonitorSourcePump(ctx context.Context, jb *audio.JitterBuffer, mixer *ServerMonitorMixer, clientID string, doneCh <-chan struct{}) {
	if !waitFirstMonitorFrame(ctx, jb, mixer, clientID, doneCh) {
		return
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-doneCh:
			drainMonitorJitter(jb, mixer, clientID)
			return
		case <-ticker.C:
			if frame := jb.Read(); frame != nil {
				mixer.SubmitSourceFrame(clientID, frame)
			}
		}
	}
}

func waitFirstMonitorFrame(ctx context.Context, jb *audio.JitterBuffer, mixer *ServerMonitorMixer, clientID string, doneCh <-chan struct{}) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-doneCh:
			return false
		default:
			if frame := jb.Read(); frame != nil {
				mixer.SubmitSourceFrame(clientID, frame)
				return true
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
}

func drainMonitorJitter(jb *audio.JitterBuffer, mixer *ServerMonitorMixer, clientID string) {
	for {
		frame := jb.Read()
		if frame == nil {
			return
		}
		mixer.SubmitSourceFrame(clientID, frame)
	}
}
