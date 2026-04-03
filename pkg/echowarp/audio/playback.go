package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/gen2brain/malgo"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

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
	return deviceConfig
}

func (p *MalgoPlayer) createOnSendCallback(inCh <-chan []float32) func([]byte, []byte, uint32) {
	var sampleBuf []float32
	var bufOffset int

	return func(pSample, _ []byte, framecount uint32) {
		samplesToWrite := int(framecount) * int(p.channels)
		written := 0

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
					for i := written * 4; i < len(pSample); i++ {
						pSample[i] = 0
					}
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
