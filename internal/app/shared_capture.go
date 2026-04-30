// Package app — SharedCaptureHub: single capture device → PCM fan-out to
// multiple per-client subscribers. Enables per-client volume / mute / encoder
// in non-conference multi-client modes without duplicating capture, AGC,
// spectrum, level-meter, or recording tap work. See .tasks/024-*.md.
package app

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// SharedCaptureHub captures PCM from a device (or multi-device via AudioMixer)
// and fans out post-AEC/AGC/gain/recTap PCM frames to any number of subscribers
// registered via Subscribe. Exactly one Run must be called to drive the hub.
//
// Subscriber channels are bounded (cap 4) and use drop-oldest semantics so a
// slow consumer cannot back-pressure the capture loop or other subscribers.
type SharedCaptureHub struct {
	cfg         CapturePipelineConfig
	logger      *slog.Logger
	newCapturer sharedCapturerFactory

	subsMu sync.RWMutex
	subs   map[string]chan []float32

	readyMu   sync.Mutex
	readyOnce sync.Once
	readyCh   chan struct{}
	readyErr  error

	// Runtime state.
	started atomic.Bool
}

type sharedCapturer interface {
	Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error
	Close() error
}

type sharedCapturerFactory func(sampleRate, channels uint32, opts ...audio.CapturerOption) (sharedCapturer, error)

type captureStartResult struct {
	err error
}

// NewSharedCaptureHub creates a hub with the given capture config.
// EncoderConfig on cfg is ignored — subscribers encode independently.
func NewSharedCaptureHub(cfg CapturePipelineConfig, logger *slog.Logger) *SharedCaptureHub {
	return &SharedCaptureHub{
		cfg:         cfg,
		logger:      logger,
		newCapturer: defaultSharedCapturerFactory,
		subs:        make(map[string]chan []float32),
		readyCh:     make(chan struct{}),
	}
}

func defaultSharedCapturerFactory(sampleRate, channels uint32, opts ...audio.CapturerOption) (sharedCapturer, error) {
	return audio.NewCapturer(sampleRate, channels, opts...)
}

// CaptureSubscription exposes a bounded PCM channel to a single subscriber.
// Callers must Close() when done to release the slot.
type CaptureSubscription struct {
	PCM      <-chan []float32
	clientID string
	hub      *SharedCaptureHub
}

// Close unsubscribes and closes the PCM channel. Idempotent.
func (s *CaptureSubscription) Close() {
	if s.hub == nil {
		return
	}
	s.hub.unsubscribe(s.clientID)
	s.hub = nil
}

// Subscribe registers a subscriber under the given clientID. If a subscription
// with that ID already exists it is replaced (prior channel closed).
func (h *SharedCaptureHub) Subscribe(clientID string) *CaptureSubscription {
	ch := make(chan []float32, 4)
	h.subsMu.Lock()
	if old, ok := h.subs[clientID]; ok {
		close(old)
	}
	h.subs[clientID] = ch
	h.subsMu.Unlock()
	return &CaptureSubscription{PCM: ch, clientID: clientID, hub: h}
}

// unsubscribe removes the subscriber and closes its channel.
func (h *SharedCaptureHub) unsubscribe(clientID string) {
	h.subsMu.Lock()
	if ch, ok := h.subs[clientID]; ok {
		delete(h.subs, clientID)
		close(ch)
	}
	h.subsMu.Unlock()
}

// SubscriberCount returns the number of active subscribers (diagnostic).
func (h *SharedCaptureHub) SubscriberCount() int {
	h.subsMu.RLock()
	defer h.subsMu.RUnlock()
	return len(h.subs)
}

// WaitReady blocks until Run reports capture startup success or failure.
func (h *SharedCaptureHub) WaitReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.readyCh:
		h.readyMu.Lock()
		defer h.readyMu.Unlock()
		return h.readyErr
	}
}

func (h *SharedCaptureHub) signalReady(err error) {
	h.readyMu.Lock()
	h.readyErr = err
	h.readyMu.Unlock()
	h.readyOnce.Do(func() { close(h.readyCh) })
}

// dispatch sends frame to all current subscribers non-blockingly.
// A subscriber whose channel is full drops its oldest queued frame before the
// new frame is enqueued, so slow consumers keep the freshest capture frames.
// The same slice is shared with all subscribers: subscribers must copy before
// mutating.
func (h *SharedCaptureHub) dispatch(frame []float32) {
	h.subsMu.RLock()
	for _, ch := range h.subs {
		dropOldestAndSend(ch, frame)
	}
	h.subsMu.RUnlock()
}

func dropOldestAndSend(ch chan []float32, frame []float32) {
	select {
	case ch <- frame:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- frame:
	default:
	}
}

// Run drives the capture loop: opens the device, applies AEC/AGC/device-gain,
// feeds spectrum/level/recordingTap, and fans out PCM to subscribers. Blocks
// until ctx is done or a fatal capture error occurs. Safe to call exactly once.
func (h *SharedCaptureHub) Run(ctx context.Context) (runErr error) {
	startupReported := false
	defer func() {
		if !startupReported {
			h.signalReady(runErr)
		}
	}()

	if !h.started.CompareAndSwap(false, true) {
		return ewerrors.NewError(ewerrors.ErrInternalState, "shared capture hub already started")
	}
	defer h.started.Store(false)
	defer h.closeAllSubs()

	deviceID := h.cfg.DeviceID
	var capturerOpts []audio.CapturerOption
	if h.cfg.IsLoopback {
		session, loopErr := audio.NewLoopbackSession(h.cfg.LoopbackOutputDevice, h.cfg.LoopbackBlackHole)
		if loopErr != nil {
			return ewerrors.Wrap(loopErr, ewerrors.ErrAggregateDeviceCreate, "create loopback session")
		}
		defer func() { _ = session.Close() }() //nolint:errcheck
		deviceID = session.CaptureDeviceID()
		if session.IsNativeLoopback() {
			capturerOpts = append(capturerOpts, audio.WithLoopbackMode())
		}
		h.logger.Info("Shared capture: loopback active",
			"output", h.cfg.LoopbackOutputDevice,
			"nativeLoopback", session.IsNativeLoopback(),
			"captureDevice", deviceID)
	}

	capturer, err := h.newCapturer(h.cfg.SampleRate, h.cfg.Channels, capturerOpts...)
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDeviceNotFound, "shared capture: create audio capturer")
	}
	defer func() { _ = capturer.Close() }() //nolint:errcheck

	bufSize := h.cfg.AudioBufferFrames
	if bufSize <= 0 {
		bufSize = 5
	}
	pcmCh := make(chan []float32, bufSize)

	startDone := make(chan captureStartResult, 1)
	go func() {
		err := capturer.Start(ctx, deviceID, pcmCh)
		startDone <- captureStartResult{err: err}
	}()

	if err := h.waitForCaptureStart(ctx, startDone); err != nil {
		return err
	}
	startupReported = true
	h.signalReady(nil)

	h.logger.Info("Shared capture hub running",
		"device", deviceID, "sampleRate", h.cfg.SampleRate, "channels", h.cfg.Channels)

	return h.forwardPCM(ctx, pcmCh)
}

func (h *SharedCaptureHub) waitForCaptureStart(ctx context.Context, startDone <-chan captureStartResult) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-startDone:
		if result.err != nil && ctx.Err() == nil {
			return ewerrors.Wrap(result.err, ewerrors.ErrDeviceNotFound, "shared capture: start audio capturer")
		}
		return ctx.Err()
	}
}

func (h *SharedCaptureHub) forwardPCM(ctx context.Context, pcmCh <-chan []float32) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case samples, ok := <-pcmCh:
			if !ok {
				return ewerrors.NewError(ewerrors.ErrInternalState, "shared capture: PCM channel closed")
			}
			samples = h.applyProcessing(ctx, samples)
			h.dispatch(samples)
		}
	}
}

// applyProcessing runs AEC/AGC/device-gain, then feeds spectrum/level/recTap.
// Mirrors CapturePipeline.Run processing so migrating from per-client pipelines
// produces identical PCM at the fan-out point.
func (h *SharedCaptureHub) applyProcessing(ctx context.Context, samples []float32) []float32 {
	if h.cfg.AEC != nil && h.cfg.AEC.IsEnabled() {
		if processed, aecErr := h.cfg.AEC.Process(ctx, samples); aecErr == nil {
			samples = processed
		}
	}
	if h.cfg.AGC != nil {
		if processed, agcErr := h.cfg.AGC.Process(ctx, samples); agcErr == nil {
			samples = processed
		}
	}
	if h.cfg.GainControl != nil {
		if h.cfg.GainControl.IsMuted() {
			for i := range samples {
				samples[i] = 0
			}
		} else if gain := h.cfg.GainControl.Gain(); gain != 1.0 {
			audio.MixGain(samples, gain)
			if gain > 1.0 {
				audio.MixTanh(samples)
			}
		}
	}
	if h.cfg.RecordingTap != nil {
		h.cfg.RecordingTap(samples)
	}
	if h.cfg.Spectrum != nil {
		h.cfg.Spectrum.Feed(samples)
	}
	if h.cfg.LevelMeter != nil {
		h.cfg.LevelMeter.Feed(samples)
	}
	return samples
}

// closeAllSubs closes every remaining subscriber channel. Called on Run exit.
func (h *SharedCaptureHub) closeAllSubs() {
	h.subsMu.Lock()
	for id, ch := range h.subs {
		close(ch)
		delete(h.subs, id)
	}
	h.subsMu.Unlock()
}
