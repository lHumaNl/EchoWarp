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
	cfg         ServerMonitorMixerConfig
	mu          sync.RWMutex
	sources     map[string]*monitorSourceBuffer
	lifecycleMu sync.RWMutex
	monitorCtx  context.Context
	running     atomic.Bool
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
	gain     *DeviceGainControl
	paused   *atomic.Bool
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

// AddSource registers or replaces a client source and returns its gain control.
func (m *ServerMonitorMixer) AddSource(clientID string, paused ...*atomic.Bool) *DeviceGainControl {
	m.mu.Lock()
	src := newMonitorSourceBuffer(m.cfg.BufferFrames, firstPauseFlag(paused))
	m.sources[clientID] = src
	m.mu.Unlock()
	return src.gain
}

// RemoveSource removes a client source from future monitor mixes.
func (m *ServerMonitorMixer) RemoveSource(clientID string) {
	m.mu.Lock()
	delete(m.sources, clientID)
	m.mu.Unlock()
}

// ClearSources removes every client source from future monitor mixes.
func (m *ServerMonitorMixer) ClearSources() {
	m.mu.Lock()
	m.sources = make(map[string]*monitorSourceBuffer)
	m.mu.Unlock()
}

// SourceCount returns the number of registered monitor sources.
func (m *ServerMonitorMixer) SourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sources)
}

// SetSourceGain updates a registered source gain and reports whether it exists.
func (m *ServerMonitorMixer) SetSourceGain(clientID string, gain float32) bool {
	m.mu.RLock()
	src := m.sources[clientID]
	m.mu.RUnlock()
	if src == nil {
		return false
	}
	src.gain.SetGain(clampPerClientVolume(gain))
	return true
}

// SubmitSourceFrame queues one decoded/jittered frame for a source.
func (m *ServerMonitorMixer) SubmitSourceFrame(clientID string, frame []float32) {
	m.mu.RLock()
	src := m.sources[clientID]
	m.mu.RUnlock()
	if src != nil && !src.isPaused() {
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
		if src.isPaused() {
			src.clear()
			continue
		}
		frame := src.readFrame()
		if frame != nil {
			gain := src.gain.Gain()
			if gain <= 0 {
				continue
			}
			if gain != 1.0 {
				audio.MixGain(frame, gain)
			}
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

func (s *ServerApp) setupServerMonitorSource(serverCtx, clientCtx context.Context, peer transport.PeerManager, clientID string, muteIncoming, paused *atomic.Bool, audioDone chan<- error) error {
	mixer, err := s.ensureServerMonitorMixer(serverCtx)
	if err != nil {
		return err
	}
	sourceCtx := mixer.sourceContext(clientCtx)
	gain := mixer.AddSource(clientID, paused)
	if mc := s.clientByID(clientID); mc != nil {
		mc.setIncomingGain(gain)
	}
	go s.removeMonitorSourceOnDone(sourceCtx, mixer, clientID)
	s.startMonitorDecodeSource(sourceCtx, peer, mixer, clientID, muteIncoming, audioDone)
	return nil
}

func (s *ServerApp) clientByID(clientID string) *multiClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[clientID]
}

func (s *ServerApp) ensureServerMonitorMixer(ctx context.Context) (*ServerMonitorMixer, error) {
	s.monitorMixerMu.Lock()
	defer s.monitorMixerMu.Unlock()
	if s.monitorMixer != nil {
		return s.monitorMixer, nil
	}
	mixer := NewServerMonitorMixer(s.serverMonitorMixerConfig())
	if _, ok := s.serverMonitorPlaybackDeviceID(); !ok {
		mixer.setMonitorContext(ctx)
		s.monitorMixer = mixer
		return mixer, nil
	}
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
	mixer.setMonitorContext(monitorCtx)
	if !s.cfg.Duplex {
		go HandleDeviceCommands(monitorCtx, s.deviceCmdCh, s.monitorPlaybackGain, s.monitorPlaybackAGC, s.logger)
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
		mixer.ClearSources()
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
