package audio

import (
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/pion/rtp"
)

const (
	rtpMixerFramesPerSecond = 50
	rtpMixerClock           = 48000
	rtpMixerTimestampStep   = rtpMixerClock / rtpMixerFramesPerSecond
	rtpMixerMaxPLC          = 3 // Conceal at most 60 ms before resetting and rebuffering.
	// Safety ceilings, not latency defaults; callers choose their queue depths.
	rtpMixerMaxPackets = 256
	rtpMixerMaxSources = 256
	rtpMixerMaxIDBytes = 256
	rtpMixerMaxPayload = 4000 // Matches the existing Opus encoder's scratch capacity.
)

// RTPMixer is an endpoint-only, fixed-20ms Opus playout mixer. Each source owns
// its decoder and bounded encoded queue. All methods are concurrency safe;
// Render should be driven by one output ticker, never by packet arrival.
type RTPMixer struct {
	mu                        sync.Mutex
	sampleRate, channels      int
	targetPackets, maxPackets int
	sources                   map[string]*rtpMixerSource
	ids                       []string
}

type rtpMixerPacket struct {
	sequence  uint16
	timestamp uint32
	payload   []byte
}

type rtpMixerSource struct {
	decoder                  *OpusDecoder
	queue                    []rtpMixerPacket
	enabled, playing, played bool
	recovering               bool
	gain                     float32
	lastSequence             uint16
	nextTimestamp            uint32
	losses                   int
}

// NewRTPMixer validates the output format and queue depths (1 <= target <= max
// <= 256). Supported rates are the native Opus rates; channels must be 1 or 2.
func NewRTPMixer(sampleRate, channels, targetPackets, maxPackets int) (*RTPMixer, error) {
	if !slices.Contains([]int{8000, 12000, 16000, 24000, 48000}, sampleRate) {
		return nil, fmt.Errorf("RTP mixer: unsupported sample rate %d", sampleRate)
	}
	if channels < 1 || channels > 2 || targetPackets < 1 ||
		maxPackets < targetPackets || maxPackets > rtpMixerMaxPackets {
		return nil, fmt.Errorf("RTP mixer: invalid channels or packet depths")
	}
	return &RTPMixer{sampleRate: sampleRate, channels: channels,
		targetPackets: targetPackets, maxPackets: maxPackets,
		sources: make(map[string]*rtpMixerSource)}, nil
}

// AddSource is idempotent. New sources are enabled with unity gain. A removed
// and re-added ID starts a new decoder/timeline generation. At most 256 sources
// are retained, with nonempty IDs of at most 256 bytes.
func (m *RTPMixer) AddSource(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sources[id]; exists {
		return nil
	}
	if id == "" || len(id) > rtpMixerMaxIDBytes || len(m.sources) >= rtpMixerMaxSources {
		return fmt.Errorf("RTP mixer: invalid source ID or source limit reached")
	}
	decoder, err := NewOpusDecoder(m.sampleRate, m.channels)
	if err != nil {
		return err
	}
	m.sources[id] = &rtpMixerSource{decoder: decoder, enabled: true, gain: 1}
	m.ids = append(m.ids, id)
	slices.Sort(m.ids) // Stable summation order independent of map iteration.
	return nil
}

// RemoveSource discards only this source's queue and decoder.
func (m *RTPMixer) RemoveSource(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sources, id)
	if index := slices.Index(m.ids, id); index >= 0 {
		m.ids = slices.Delete(m.ids, index, index+1)
	}
}

// SetSourceEnabled flushes audio and decoder history on either transition.
// Repeating the current state is a no-op; disabled packets are discarded.
func (m *RTPMixer) SetSourceEnabled(id string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if source := m.sources[id]; source != nil && source.enabled != enabled {
		*source = rtpMixerSource{enabled: enabled, gain: source.gain}
	}
}

// SetSourceGain accepts finite nonnegative gain. Invalid gain and unknown IDs
// are ignored. Gain zero still advances playout, unlike disabling a source.
func (m *RTPMixer) SetSourceGain(id string, gain float32) {
	if gain < 0 || math.IsNaN(float64(gain)) || math.IsInf(float64(gain), 0) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if source := m.sources[id]; source != nil {
		source.gain = gain
	}
}

// WritePacket copies only the RTP timeline and payload, not transport metadata.
// Unknown/disabled sources, oversized/empty packets, duplicates and late packets
// are dropped. Overflow retains the newest targetPackets, resetting playout so
// a stalled consumer never walks a stale backlog. Caller owns pkt on return.
func (m *RTPMixer) WritePacket(id string, pkt *rtp.Packet) {
	if pkt == nil || len(pkt.Payload) == 0 || len(pkt.Payload) > rtpMixerMaxPayload {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	source := m.sources[id]
	if source == nil || !source.enabled || source.late(pkt) {
		return
	}
	source.insert(pkt)
	if len(source.queue) > m.maxPackets {
		source.rebuffer(m.targetPackets)
	}
}

func (s *rtpMixerSource) late(pkt *rtp.Packet) bool {
	if !s.played {
		return false
	}
	// A long forwarding mute can span half the sequence space. Once rebuffering,
	// the timestamp watermark, not an ambiguous sequence delta, gates the restart.
	return int32(pkt.Timestamp-s.nextTimestamp) < 0 ||
		(!s.recovering && int16(pkt.SequenceNumber-s.lastSequence) <= 0)
}

func (s *rtpMixerSource) insert(pkt *rtp.Packet) {
	index := len(s.queue)
	for i, queued := range s.queue {
		if queued.sequence == pkt.SequenceNumber {
			return
		}
		if int16(pkt.SequenceNumber-queued.sequence) < 0 && index == len(s.queue) {
			index = i
		}
	}
	packet := rtpMixerPacket{sequence: pkt.SequenceNumber, timestamp: pkt.Timestamp,
		payload: append([]byte(nil), pkt.Payload...)}
	s.queue = slices.Insert(s.queue, index, packet)
}

func (s *rtpMixerSource) rebuffer(target int) {
	if len(s.queue) > target {
		s.queue = slices.Delete(s.queue, 0, len(s.queue)-target)
	}
	s.playing, s.decoder, s.losses = false, nil, 0
	s.recovering = true
}

type rtpMixerFrame struct {
	id   string
	pcm  []float32
	gain float32
}

// Render returns one fresh, owned interleaved 20ms frame, including silence.
// tap runs synchronously outside the mixer lock, before gain and limiting, for
// each decoded/PLC frame. Its PCM is read-only and valid during the call. Keep
// taps nonblocking; controls invoked by a tap apply to the next render snapshot.
func (m *RTPMixer) Render(tap func(id string, pcm []float32)) []float32 {
	out := make([]float32, m.sampleRate/rtpMixerFramesPerSecond*m.channels)
	frames := m.frames()
	// float64 accumulation prevents finite float32 gains overflowing before clamp.
	sum := make([]float64, len(out))
	for _, frame := range frames {
		if tap != nil {
			tap(frame.id, frame.pcm)
		}
		for i, value := range frame.pcm {
			sum[i] += float64(value) * float64(frame.gain)
		}
	}
	for i, value := range sum {
		out[i] = float32(max(-1, min(1, value)))
	}
	return out
}

func (m *RTPMixer) frames() []rtpMixerFrame {
	m.mu.Lock()
	defer m.mu.Unlock()
	frames := make([]rtpMixerFrame, 0, len(m.ids))
	for _, id := range m.ids {
		source := m.sources[id]
		if pcm := source.render(m); pcm != nil {
			frames = append(frames, rtpMixerFrame{id: id,
				pcm: append([]float32(nil), pcm...), gain: source.gain})
		}
	}
	return frames
}

func (s *rtpMixerSource) prepare(m *RTPMixer) bool {
	if !s.enabled || (!s.playing && len(s.queue) < m.targetPackets) {
		return false
	}
	if s.playing {
		return true
	}
	return s.start(m)
}

func (s *rtpMixerSource) start(m *RTPMixer) bool {
	if s.recovering {
		s.queue = slices.Delete(s.queue, 0, len(s.queue)-m.targetPackets)
	}
	if s.decoder == nil {
		decoder, err := NewOpusDecoder(m.sampleRate, m.channels)
		if err != nil {
			return false
		}
		s.decoder = decoder
	}
	s.nextTimestamp, s.playing = s.queue[0].timestamp, true
	s.recovering = false
	return true
}

func (s *rtpMixerSource) render(m *RTPMixer) []float32 {
	if !s.prepare(m) {
		return nil
	}
	s.dropExpired()
	if len(s.queue) > 0 && s.queue[0].timestamp == s.nextTimestamp {
		return s.decode(m)
	}
	return s.conceal(m)
}

func (s *rtpMixerSource) conceal(m *RTPMixer) []float32 {
	if s.losses >= rtpMixerMaxPLC {
		s.rebuffer(m.targetPackets)
		return nil
	}
	s.losses++
	s.nextTimestamp += rtpMixerTimestampStep
	pcm, err := s.decoder.DecodePLC()
	if err != nil {
		s.rebuffer(m.targetPackets)
		return nil
	}
	return pcm
}

func (s *rtpMixerSource) dropExpired() {
	for len(s.queue) > 0 && int32(s.queue[0].timestamp-s.nextTimestamp) < 0 {
		s.queue = slices.Delete(s.queue, 0, 1)
	}
}

func (s *rtpMixerSource) decode(m *RTPMixer) []float32 {
	packet := s.queue[0]
	s.queue = slices.Delete(s.queue, 0, 1)
	s.lastSequence, s.played = packet.sequence, true
	s.nextTimestamp += rtpMixerTimestampStep
	pcm, err := s.decoder.Decode(packet.payload)
	if err != nil || len(pcm) != m.sampleRate/rtpMixerFramesPerSecond*m.channels {
		// Malformed or non-20ms packets must not leak partial decoder history.
		s.rebuffer(m.targetPackets)
		return nil
	}
	s.losses = 0
	return pcm
}
