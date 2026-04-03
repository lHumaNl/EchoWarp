package audio

import (
	"fmt"
	"sync"

	"gopkg.in/hraban/opus.v2"

	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// Opus application types determine the encoding mode for the Opus codec.
// Each mode is optimized for different use cases.
const (
	// OpusApplicationVoIP optimizes for voice communication with lower bitrate
	// and higher quality for speech at the expense of music quality.
	OpusApplicationVoIP = "voip"

	// OpusApplicationAudio provides balanced encoding suitable for music and
	// mixed content at higher bitrates.
	OpusApplicationAudio = "audio"

	// OpusApplicationRestrictedLowDelay minimizes encoding latency for real-time
	// applications at the cost of quality and compression efficiency.
	OpusApplicationRestrictedLowDelay = "restricted_lowdelay"
)

// opusEncodePool reuses encode scratch buffers to reduce GC pressure.
// Each buffer can hold up to 4000 bytes, sufficient for any Opus frame.
var opusEncodePool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4000)
	},
}

const (
	defaultOpusOutputSize = 256
	maxOpusOutputSize     = 1024
)

// opusOutputPool reuses small output buffers for encoded Opus frames (~50-200 bytes).
var opusOutputPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, defaultOpusOutputSize)
	},
}

func getOpusOutput(size int) []byte {
	buf := opusOutputPool.Get().([]byte) //nolint:errcheck
	if cap(buf) < size {
		return make([]byte, size)
	}
	return buf[:size]
}

// PutOpusOutput returns an encoded Opus frame buffer to the pool for reuse.
// Must be called after the buffer is no longer needed (e.g., after WriteSample).
func PutOpusOutput(buf []byte) {
	if cap(buf) > maxOpusOutputSize {
		return
	}
	opusOutputPool.Put(buf[:0]) //nolint:staticcheck // slices are pointer-like
}

func opusApplication(app string) (opus.Application, error) {
	switch app {
	case OpusApplicationVoIP:
		return opus.AppVoIP, nil
	case OpusApplicationAudio:
		return opus.AppAudio, nil
	case OpusApplicationRestrictedLowDelay:
		return opus.AppRestrictedLowdelay, nil
	default:
		return 0, fmt.Errorf("unknown opus application: %q", app)
	}
}

// OpusEncoder encodes PCM float32 audio to Opus format for efficient
// network transmission. The encoder is configured for 20ms frames (960 samples
// at 48kHz mono) which is optimal for real-time communication.
//
// Thread-unsafe: Create separate encoders for concurrent use.
type OpusEncoder struct {
	enc       *opus.Encoder
	frameSize int
	channels  int
}

// NewOpusEncoder creates an Opus encoder with the specified parameters.
// Default bitrate is 64 kbps, suitable for high-quality voice communication.
//
// sampleRate: 48000 is recommended for best quality and compatibility.
// channels: 1 for mono (voice), 2 for stereo (music).
// application: Use OpusApplicationVoIP for voice, OpusApplicationAudio for music.
func NewOpusEncoder(sampleRate, channels int, application string) (*OpusEncoder, error) {
	app, err := opusApplication(application)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrFormatUnsupported, "opus encoder: invalid application")
	}

	enc, err := opus.NewEncoder(sampleRate, channels, app)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "opus encoder: create")
	}

	if err := enc.SetBitrate(64000); err != nil {
		return nil, fmt.Errorf("opus encoder: set bitrate: %w", err)
	}

	frameSize := sampleRate / 50 * channels

	return &OpusEncoder{
		enc:       enc,
		frameSize: frameSize,
		channels:  channels,
	}, nil
}

// Encode converts PCM float32 samples to Opus format. The input must contain
// exactly frameSize samples (e.g., 960 for 48kHz mono 20ms frame).
// Returns a newly allocated byte slice containing the encoded data.
func (e *OpusEncoder) Encode(pcm []float32) ([]byte, error) {
	if len(pcm) != e.frameSize {
		return nil, fmt.Errorf("opus encode: expected %d samples, got %d", e.frameSize, len(pcm))
	}

	buf := opusEncodePool.Get().([]byte) //nolint:errcheck
	defer opusEncodePool.Put(buf)        //nolint:staticcheck // slices are pointer-like

	n, err := e.enc.EncodeFloat32(pcm, buf)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrOpusEncode, "opus encode")
	}

	result := getOpusOutput(n)
	copy(result, buf[:n])
	return result, nil
}

// SetBitrate adjusts the encoding bitrate in bits per second. Higher bitrates
// improve quality but increase bandwidth usage. Typical values: 16-128 kbps.
func (e *OpusEncoder) SetBitrate(bps int) error {
	if err := e.enc.SetBitrate(bps); err != nil {
		return fmt.Errorf("opus set bitrate: %w", err)
	}
	return nil
}

// SetComplexity sets the encoder complexity (0-10). Higher values provide better
// quality at the cost of increased CPU usage. Value 10 is recommended for
// modern hardware.
func (e *OpusEncoder) SetComplexity(complexity int) error {
	if err := e.enc.SetComplexity(complexity); err != nil {
		return fmt.Errorf("opus set complexity: %w", err)
	}
	return nil
}

// SetDTX enables or disables Discontinuous Transmission. When enabled, the
// encoder produces smaller frames during silence, reducing bandwidth usage.
func (e *OpusEncoder) SetDTX(enabled bool) error {
	if err := e.enc.SetDTX(enabled); err != nil {
		return fmt.Errorf("opus set dtx: %w", err)
	}
	return nil
}

// SetInBandFEC enables or disables Forward Error Correction. When enabled,
// the encoder adds redundant data to help recover from packet loss.
// Should be used with SetPacketLossPerc to configure expected loss rate.
func (e *OpusEncoder) SetInBandFEC(enabled bool) error {
	if err := e.enc.SetInBandFEC(enabled); err != nil {
		return fmt.Errorf("opus set fec: %w", err)
	}
	return nil
}

// SetPacketLossPerc configures the expected packet loss percentage (0-100).
// This affects how much FEC data is added when InBandFEC is enabled.
func (e *OpusEncoder) SetPacketLossPerc(pct int) error {
	if err := e.enc.SetPacketLossPerc(pct); err != nil {
		return fmt.Errorf("opus set packet loss: %w", err)
	}
	return nil
}

// FrameSize returns the number of samples required per Encode call.
func (e *OpusEncoder) FrameSize() int {
	return e.frameSize
}

// OpusDecoder decodes Opus format audio back to PCM float32 samples.
// The decoder maintains internal state for packet loss concealment.
//
// Thread-unsafe: Create separate decoders for concurrent use.
type OpusDecoder struct {
	dec       *opus.Decoder
	frameSize int
	channels  int
	pcmBuf    []float32
}

// NewOpusDecoder creates an Opus decoder for the specified sample rate and channels.
// Parameters should match those used by the encoder.
func NewOpusDecoder(sampleRate, channels int) (*OpusDecoder, error) {
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrOpusDecode, "opus decoder: create")
	}

	frameSize := sampleRate / 50 * channels

	return &OpusDecoder{
		dec:       dec,
		frameSize: frameSize,
		channels:  channels,
		pcmBuf:    make([]float32, frameSize),
	}, nil
}

// Decode converts Opus encoded data to PCM float32 samples.
// The returned slice is valid only until the next Decode or DecodePLC call.
func (d *OpusDecoder) Decode(opusData []byte) ([]float32, error) {
	n, err := d.dec.DecodeFloat32(opusData, d.pcmBuf)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrOpusDecode, "opus decode")
	}

	return d.pcmBuf[:n*d.channels], nil
}

// DecodePLC performs Packet Loss Concealment to generate audio for a missing frame.
// This should be called when an Opus packet is lost or arrives too late.
// The returned slice is valid only until the next Decode or DecodePLC call.
func (d *OpusDecoder) DecodePLC() ([]float32, error) {
	err := d.dec.DecodePLCFloat32(d.pcmBuf)
	if err != nil {
		return nil, ewerrors.Wrap(err, ewerrors.ErrOpusDecode, "opus decode PLC")
	}

	return d.pcmBuf, nil
}

// FrameSize returns the number of samples per decoded frame.
func (d *OpusDecoder) FrameSize() int {
	return d.frameSize
}
