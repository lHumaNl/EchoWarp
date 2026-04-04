package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/gen2brain/malgo"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
	"github.com/lHumaNl/echowarp/pkg/echowarp/metrics"
)

// MalgoCapturer captures audio using the malgo library (miniaudio wrapper).
// It supports cross-platform audio capture on macOS (CoreAudio), Windows (WASAPI),
// and Linux (PulseAudio/ALSA).
//
// Thread-safe: concurrent access is protected by internal mutex.
type MalgoCapturer struct {
	sampleRate   uint32
	channels     uint32
	loopbackMode bool

	mu         sync.Mutex
	ctx        *malgo.AllocatedContext
	ownsCtx    bool
	device     *malgo.Device
	deviceInfo *malgo.DeviceInfo
}

// CapturerOption configures a MalgoCapturer during creation.
type CapturerOption func(*MalgoCapturer)

// WithContext allows sharing an existing malgo context between multiple
// audio components (e.g., capturer and player), reducing resource usage.
func WithContext(ctx *malgo.AllocatedContext) CapturerOption {
	return func(c *MalgoCapturer) {
		c.ctx = ctx
		c.ownsCtx = false
	}
}

// WithLoopbackMode configures the capturer to use native OS loopback capture
// (e.g. WASAPI loopback on Windows). In this mode, the deviceID passed to
// Start() refers to a playback device, and malgo opens it with the Loopback
// device type to capture the audio being played through it.
func WithLoopbackMode() CapturerOption {
	return func(c *MalgoCapturer) {
		c.loopbackMode = true
	}
}

// NewCapturer creates a new audio capturer with the specified sample rate and channels.
// If no WithContext option is provided, a new malgo context is created automatically.
//
// Common configurations:
//   - 48000 Hz, 1 channel: optimal for Opus VoIP encoding
//   - 48000 Hz, 2 channels: stereo capture for music/content
func NewCapturer(sampleRate, channels uint32, opts ...CapturerOption) (*MalgoCapturer, error) {
	c := &MalgoCapturer{
		sampleRate: sampleRate,
		channels:   channels,
		ownsCtx:    true,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.ctx == nil {
		ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrAudioContextInit, "capturer: init context")
		}
		c.ctx = ctx
		c.ownsCtx = true
	}

	return c, nil
}

// Start begins capturing audio from the specified device. Audio samples are sent
// to outCh as float32 slices. If the channel buffer is full, samples are dropped
// and a warning is logged to prevent blocking the audio callback.
//
// The deviceID is an index from ListInputDevices(). The capturer stops automatically
// when ctx is canceled.
func (c *MalgoCapturer) Start(ctx context.Context, deviceID uint32, outCh chan<- []float32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ctx == nil {
		return ewerrors.NewError(ewerrors.ErrAudioContextInit, "capturer: context not initialized")
	}

	deviceInfo, err := c.getCaptureDeviceInfo(deviceID)
	if err != nil {
		return err
	}

	deviceConfig := c.createCaptureDeviceConfig(deviceInfo)
	onRecv := c.createOnRecvCallback(outCh)

	device, err := malgo.InitDevice(c.ctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onRecv})
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDeviceInitFailed, "capturer: init device")
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		return ewerrors.Wrap(err, ewerrors.ErrDeviceStartFailed, "capturer: start device")
	}

	c.device = device
	c.deviceInfo = deviceInfo
	go c.monitorCaptureContextCancellation(ctx)

	return nil
}

func (c *MalgoCapturer) getCaptureDeviceInfo(deviceID uint32) (*malgo.DeviceInfo, error) {
	deviceType := malgo.Capture
	if c.loopbackMode {
		deviceType = malgo.Playback
	}
	infos, err := c.ctx.Devices(deviceType)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrDeviceEnumFailed, "capturer: list devices")
	}
	if int(deviceID) >= len(infos) {
		return nil, fmt.Errorf("capturer: %w: device index %d (available: %d)", ErrDeviceNotFound, deviceID, len(infos))
	}
	return &infos[deviceID], nil
}

func (c *MalgoCapturer) createCaptureDeviceConfig(deviceInfo *malgo.DeviceInfo) malgo.DeviceConfig {
	if c.loopbackMode {
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Loopback)
		deviceConfig.Capture.Format = malgo.FormatF32
		deviceConfig.Capture.Channels = c.channels
		deviceConfig.Playback.DeviceID = deviceInfo.ID.Pointer()
		deviceConfig.SampleRate = c.sampleRate
		return deviceConfig
	}
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = c.channels
	deviceConfig.Capture.DeviceID = deviceInfo.ID.Pointer()
	deviceConfig.SampleRate = c.sampleRate
	deviceConfig.Alsa.NoMMap = 1
	return deviceConfig
}

func (c *MalgoCapturer) createOnRecvCallback(outCh chan<- []float32) func([]byte, []byte, uint32) {
	return func(_, pSample []byte, framecount uint32) {
		if len(pSample) == 0 {
			return
		}

		numSamples := len(pSample) / 4
		samples := getPCMBuffer(numSamples)
		samples = bytesToFloat32SliceInto(samples, pSample)

		select {
		case outCh <- samples:
		default:
			putPCMBuffer(samples)
			metrics.FramesDroppedTotal.WithLabelValues("pcm").Inc()
			slog.Warn("capturer: dropping audio frame, channel buffer full",
				"framecount", framecount,
				"samples", len(samples))
		}
	}
}

func (c *MalgoCapturer) monitorCaptureContextCancellation(ctx context.Context) {
	<-ctx.Done()
	c.mu.Lock()
	if c.device != nil {
		_ = c.device.Stop() //nolint:errcheck
		c.device.Uninit()
		c.device = nil
	}
	c.mu.Unlock()
}

// SampleRate returns the configured sample rate in Hz.
func (c *MalgoCapturer) SampleRate() uint32 {
	return c.sampleRate
}

// Channels returns the number of audio channels (1 for mono, 2 for stereo).
func (c *MalgoCapturer) Channels() uint32 {
	return c.channels
}

// Close stops capturing and releases all resources including the malgo context
// if it was created by this capturer.
func (c *MalgoCapturer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.device != nil {
		_ = c.device.Stop() //nolint:errcheck
		c.device.Uninit()
		c.device = nil
	}

	if c.ownsCtx && c.ctx != nil {
		_ = c.ctx.Uninit() //nolint:errcheck
		c.ctx.Free()
		c.ctx = nil
	}

	return nil
}

func (c *MalgoCapturer) monitorBuffer(ctx context.Context, ch <-chan []float32, name string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics.ChannelBufferSize.WithLabelValues(name).Set(float64(len(ch)))
			metrics.ChannelBufferCapacity.WithLabelValues(name).Set(float64(cap(ch)))
		}
	}
}

func bytesToFloat32SliceInto(dst []float32, b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	numSamples := len(b) / 4
	if cap(dst) < numSamples {
		dst = make([]float32, numSamples)
	} else {
		dst = dst[:numSamples]
	}
	for i := 0; i < numSamples; i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		dst[i] = math.Float32frombits(bits)
	}
	return dst
}
