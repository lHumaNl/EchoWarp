package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// PlaybackPeriodMilliseconds is the configured output callback period.
const PlaybackPeriodMilliseconds uint32 = 20

const (
	callbackLate25Milliseconds int64 = 25
	callbackLate40Milliseconds int64 = 40
)

// PlayerDiagnosticsSnapshot is a lock-free snapshot of playback callback health.
type PlayerDiagnosticsSnapshot struct {
	SilenceFillsTotal       uint64
	PartialSilenceFills     uint64
	FullSilenceFills        uint64
	ZeroFilledSamples       uint64
	CallbackCount           uint64
	CallbackGapMax          time.Duration
	CallbackLate25MS        uint64
	CallbackLate40MS        uint64
	FirstCallbackFrameCount uint32
}

// MalgoPlayer plays audio using the malgo library (miniaudio wrapper).
// It supports cross-platform audio playback on macOS (CoreAudio), Windows (WASAPI),
// and Linux (PulseAudio/ALSA).
//
// Thread-safe: concurrent access is protected by internal mutex.
type MalgoPlayer struct {
	sampleRate uint32
	channels   uint32

	mu      sync.Mutex
	ctx     *malgo.AllocatedContext
	ownsCtx bool
	device  *malgo.Device

	// silenceFills counts how many times the malgo callback could not read
	// a full buffer's worth of samples from inCh and filled the remainder
	// with silence. Each such event is typically heard as a pop/click.
	// Diagnostic only; read via SilenceFills().
	silenceFills atomic.Uint64

	partialSilenceFills     atomic.Uint64
	fullSilenceFills        atomic.Uint64
	zeroFilledSamples       atomic.Uint64
	callbackCount           atomic.Uint64
	callbackGapMaxNanos     atomic.Uint64
	callbackLate25MS        atomic.Uint64
	callbackLate40MS        atomic.Uint64
	lastCallbackUnixNano    atomic.Int64
	firstCallbackFrameCount atomic.Uint32
	callbackClock           func() int64
}

// SilenceFills returns the cumulative number of times the playback callback
// zero-filled the hardware buffer due to empty input channel. Divide by
// callback rate to estimate the rate of audible dropouts.
func (p *MalgoPlayer) SilenceFills() uint64 {
	return p.silenceFills.Load()
}

// PlaybackDiagnostics returns cumulative callback counters for diagnostics.
func (p *MalgoPlayer) PlaybackDiagnostics() PlayerDiagnosticsSnapshot {
	return PlayerDiagnosticsSnapshot{
		SilenceFillsTotal:       p.silenceFills.Load(),
		PartialSilenceFills:     p.partialSilenceFills.Load(),
		FullSilenceFills:        p.fullSilenceFills.Load(),
		ZeroFilledSamples:       p.zeroFilledSamples.Load(),
		CallbackCount:           p.callbackCount.Load(),
		CallbackGapMax:          time.Duration(p.callbackGapMaxNanos.Load()),
		CallbackLate25MS:        p.callbackLate25MS.Load(),
		CallbackLate40MS:        p.callbackLate40MS.Load(),
		FirstCallbackFrameCount: p.firstCallbackFrameCount.Load(),
	}
}

// PlayerOption configures a MalgoPlayer during creation.
type PlayerOption func(*MalgoPlayer)

// WithPlayerContext allows sharing an existing malgo context between multiple
// audio components, reducing resource usage.
func WithPlayerContext(ctx *malgo.AllocatedContext) PlayerOption {
	return func(p *MalgoPlayer) {
		p.ctx = ctx
		p.ownsCtx = false
	}
}

// NewPlayer creates a new audio player with the specified sample rate and channels.
// If no WithPlayerContext option is provided, a new malgo context is created automatically.
func NewPlayer(sampleRate, channels uint32, opts ...PlayerOption) (*MalgoPlayer, error) {
	p := &MalgoPlayer{
		sampleRate: sampleRate,
		channels:   channels,
		ownsCtx:    true,
		callbackClock: func() int64 {
			return time.Now().UnixNano()
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.ctx == nil {
		ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
		if err != nil {
			return nil, ewerrors.Wrap(err, ewerrors.ErrAudioContextInit, "player: init context")
		}
		p.ctx = ctx
		p.ownsCtx = true
	}

	return p, nil
}

// Start begins playing audio to the specified output device. Audio samples are read
// from inCh as float32 slices. When no samples are available, silence is played
// to prevent audio glitches.
//
// The deviceID is an index from ListOutputDevices(). The player stops automatically
// when ctx is canceled or the input channel is closed.
func (p *MalgoPlayer) Start(ctx context.Context, deviceID uint32, inCh <-chan []float32) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctx == nil {
		return ewerrors.NewError(ewerrors.ErrAudioContextInit, "player: context not initialized")
	}

	deviceInfo, err := p.getDeviceInfo(deviceID)
	if err != nil {
		return err
	}

	deviceConfig := p.createDeviceConfig(deviceInfo)
	onSend := p.createOnSendCallback(inCh)

	device, err := malgo.InitDevice(p.ctx.Context, deviceConfig, malgo.DeviceCallbacks{Data: onSend})
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrDeviceInitFailed, "player: init device")
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		return ewerrors.Wrap(err, ewerrors.ErrDeviceStartFailed, "player: start device")
	}

	p.device = device
	go p.monitorContextCancellation(ctx)

	return nil
}

func (p *MalgoPlayer) getDeviceInfo(deviceID uint32) (*malgo.DeviceInfo, error) {
	infos, err := p.ctx.Devices(malgo.Playback)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrDeviceEnumFailed, "player: list devices")
	}
	if int(deviceID) >= len(infos) {
		return nil, fmt.Errorf("player: %w: device index %d (available: %d)", ErrDeviceNotFound, deviceID, len(infos))
	}
	return &infos[deviceID], nil
}

func (p *MalgoPlayer) createDeviceConfig(deviceInfo *malgo.DeviceInfo) malgo.DeviceConfig {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatF32
	deviceConfig.Playback.Channels = p.channels
	deviceConfig.Playback.DeviceID = deviceInfo.ID.Pointer()
	deviceConfig.SampleRate = p.sampleRate
	deviceConfig.Alsa.NoMMap = 1
	// Align callback period with pump cadence (20 ms Opus frames). Without
	// this, miniaudio picks a small default period (5–10 ms on CoreAudio)
	// and each callback may split a pump frame boundary, causing the player
	// to emit silence when inCh is briefly empty between pump ticks.
	// 20 ms matches our frame size and significantly reduces silence-fills.
	deviceConfig.PeriodSizeInMilliseconds = PlaybackPeriodMilliseconds
	return deviceConfig
}

func (p *MalgoPlayer) createOnSendCallback(inCh <-chan []float32) func([]byte, []byte, uint32) {
	var sampleBuf []float32
	var bufOffset int

	return func(pSample, _ []byte, framecount uint32) {
		samplesToWrite := int(framecount) * int(p.channels)
		written := 0
		p.recordCallbackStart(framecount)

		for written < samplesToWrite {
			if bufOffset >= len(sampleBuf) {
				if sampleBuf != nil {
					putPCMBuffer(sampleBuf)
				}
				select {
				case newSamples := <-inCh:
					sampleBuf = newSamples
					bufOffset = 0
				default:
					zeroBytes(pSample[written*4:])
					p.recordSilenceFill(samplesToWrite, written)
					return
				}
			}

			remaining := len(sampleBuf) - bufOffset
			needed := samplesToWrite - written
			toCopy := remaining
			if toCopy > needed {
				toCopy = needed
			}

			for i := 0; i < toCopy; i++ {
				bits := math.Float32bits(sampleBuf[bufOffset+i])
				binary.LittleEndian.PutUint32(pSample[(written+i)*4:], bits)
			}

			bufOffset += toCopy
			written += toCopy
		}
	}
}

func (p *MalgoPlayer) callbackUnixNano() int64 {
	if p.callbackClock != nil {
		return p.callbackClock()
	}
	return time.Now().UnixNano()
}

func (p *MalgoPlayer) recordCallbackStart(framecount uint32) {
	p.callbackCount.Add(1)
	p.firstCallbackFrameCount.CompareAndSwap(0, framecount)
	now := p.callbackUnixNano()
	prev := p.lastCallbackUnixNano.Swap(now)
	if prev == 0 || now <= prev {
		return
	}
	p.recordCallbackGap(time.Duration(now - prev))
}

func (p *MalgoPlayer) recordCallbackGap(gap time.Duration) {
	updateAtomicMax(&p.callbackGapMaxNanos, uint64(gap))
	if gap >= time.Duration(callbackLate25Milliseconds)*time.Millisecond {
		p.callbackLate25MS.Add(1)
	}
	if gap >= time.Duration(callbackLate40Milliseconds)*time.Millisecond {
		p.callbackLate40MS.Add(1)
	}
}

func (p *MalgoPlayer) recordSilenceFill(samplesToWrite, written int) {
	zeroSamples := samplesToWrite - written
	p.silenceFills.Add(1)
	p.zeroFilledSamples.Add(uint64(zeroSamples))
	if written == 0 {
		p.fullSilenceFills.Add(1)
		return
	}
	p.partialSilenceFills.Add(1)
}

func zeroBytes(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

func updateAtomicMax(target *atomic.Uint64, value uint64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}

func (p *MalgoPlayer) monitorContextCancellation(ctx context.Context) {
	<-ctx.Done()
	p.mu.Lock()
	if p.device != nil {
		_ = p.device.Stop() //nolint:errcheck
		p.device.Uninit()
		p.device = nil
	}
	p.mu.Unlock()
}

// Close stops playback and releases all resources including the malgo context
// if it was created by this player.
func (p *MalgoPlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.device != nil {
		_ = p.device.Stop() //nolint:errcheck
		p.device.Uninit()
		p.device = nil
	}

	if p.ownsCtx && p.ctx != nil {
		_ = p.ctx.Uninit() //nolint:errcheck
		p.ctx.Free()
		p.ctx = nil
	}

	return nil
}

func float32SliceToBytes(samples []float32) []byte {
	buf := make([]byte, len(samples)*4)
	for i, s := range samples {
		bits := math.Float32bits(s)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}
