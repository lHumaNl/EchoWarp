package audio

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

const mixerTestRate = 48000

func testRTPMixer(t *testing.T, channels, target, maximum int, ids ...string) *RTPMixer {
	t.Helper()
	m, err := NewRTPMixer(mixerTestRate, channels, target, maximum)
	require.NoError(t, err)
	for _, id := range ids {
		require.NoError(t, m.AddSource(id))
	}
	return m
}

func tonePackets(t *testing.T, channels, count int, frequency float64) []*rtp.Packet {
	t.Helper()
	enc, err := NewOpusEncoder(mixerTestRate, channels, OpusApplicationAudio)
	require.NoError(t, err)
	packets := make([]*rtp.Packet, count)
	for frame := range packets {
		pcm := toneFrame(channels, frame, frequency)
		payload, encodeErr := enc.Encode(pcm)
		require.NoError(t, encodeErr)
		packets[frame] = &rtp.Packet{Header: rtp.Header{
			SequenceNumber: uint16(frame), Timestamp: uint32(frame * 960),
		}, Payload: append([]byte(nil), payload...)}
		PutOpusOutput(payload)
	}
	return packets
}

func toneFrame(channels, frame int, frequency float64) []float32 {
	pcm := make([]float32, mixerTestRate/50*channels)
	for i := range pcm {
		phase := float64(frame*mixerTestRate/50+i/channels) / mixerTestRate
		pcm[i] = float32(0.1 * math.Sin(2*math.Pi*frequency*phase))
	}
	return pcm
}

func TestRTPMixerThreeIndependentTones(t *testing.T) {
	for _, channels := range []int{1, 2} {
		t.Run(fmt.Sprint(channels), func(t *testing.T) { checkThreeTones(t, channels) })
	}
}

func checkThreeTones(t *testing.T, channels int) {
	m := testRTPMixer(t, channels, 1, 8, "alice", "bob", "server")
	ids := []string{"alice", "bob", "server"}
	frequencies := []float64{300, 700, 1100}
	for i, id := range ids {
		for _, packet := range tonePackets(t, channels, 8, frequencies[i]) {
			m.WritePacket(id, packet)
		}
	}
	var out []float32
	for range 8 {
		out = m.Render(nil)
	}
	require.Len(t, out, mixerTestRate/50*channels)
	for channel := range channels {
		for _, frequency := range frequencies {
			require.Greater(t, toneAmplitude(out, channels, channel, frequency), 0.06)
		}
	}
}

func toneAmplitude(pcm []float32, channels, channel int, frequency float64) float64 {
	var realPart, imaginary float64
	for i := channel; i < len(pcm); i += channels {
		phase := 2 * math.Pi * frequency * float64(i/channels) / mixerTestRate
		realPart += float64(pcm[i]) * math.Cos(phase)
		imaginary += float64(pcm[i]) * math.Sin(phase)
	}
	return 2 * math.Hypot(realPart, imaginary) / float64(len(pcm)/channels)
}

func TestRTPMixerReorderDuplicateWrapAndOwnership(t *testing.T) {
	packets := tonePackets(t, 1, 4, 700)
	for i, packet := range packets {
		packet.SequenceNumber = uint16(65534 + i)
		packet.Timestamp = uint32(uint64(math.MaxUint32-959) + uint64(i*960))
	}
	m := testRTPMixer(t, 1, 3, 8, "server")
	for _, index := range []int{2, 0, 1, 1, 3} {
		m.WritePacket("server", packets[index])
	}
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	for _, packet := range packets {
		want, decodeErr := decoder.Decode(packet.Payload)
		require.NoError(t, decodeErr)
		clear(packet.Payload) // WritePacket must own its copy before decoding.
		require.Equal(t, want, m.Render(nil))
		m.WritePacket("server", packet) // Already consumed packets cannot return.
	}
}

func TestRTPMixerLossUsesSequentialDecoder(t *testing.T) {
	for _, gap := range []bool{false, true} {
		t.Run(fmt.Sprint(gap), func(t *testing.T) { checkMixerLoss(t, gap) })
	}
}

func checkMixerLoss(t *testing.T, timestampOnly bool) {
	packets := tonePackets(t, 1, 3, 700)
	if timestampOnly {
		packets[2].SequenceNumber = 1
	}
	m := testRTPMixer(t, 1, 1, 8, "alice")
	m.WritePacket("alice", packets[0])
	m.WritePacket("alice", packets[2])
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	assertMixerDecode(t, m, decoder, packets[0])
	want, err := decoder.DecodePLC()
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil))
	m.WritePacket("alice", packets[1]) // This missing slot has already been concealed.
	require.Len(t, m.sources["alice"].queue, 1)
	assertMixerDecode(t, m, decoder, packets[2])
}

func assertMixerDecode(t *testing.T, m *RTPMixer, decoder *OpusDecoder, packet *rtp.Packet) {
	t.Helper()
	want, err := decoder.Decode(packet.Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil))
}

func TestRTPMixerValidation(t *testing.T) {
	for _, params := range [][4]int{{44100, 1, 1, 4}, {48000, 0, 1, 4}, {48000, 3, 1, 4},
		{48000, 1, 0, 4}, {48000, 1, 5, 4}, {48000, 1, 1, math.MaxInt}} {
		_, err := NewRTPMixer(params[0], params[1], params[2], params[3])
		require.Error(t, err)
	}
	for _, rate := range []int{8000, 12000, 16000, 24000, 48000} {
		m, err := NewRTPMixer(rate, 2, 1, 4)
		require.NoError(t, err)
		require.Len(t, m.Render(nil), rate/50*2)
	}
}

func TestRTPMixerPrebufferAndFreshOutput(t *testing.T) {
	m := testRTPMixer(t, 1, 2, 4, "alice")
	packets := tonePackets(t, 1, 2, 700)
	m.WritePacket("alice", packets[0])
	first := m.Render(nil)
	require.Equal(t, make([]float32, 960), first)
	m.WritePacket("alice", packets[1])
	second := m.Render(nil)
	require.NotEqual(t, first, second)
	copyOfSecond := append([]float32(nil), second...)
	m.Render(nil)
	require.Equal(t, copyOfSecond, second)
	require.Equal(t, make([]float32, 960), first)
}

func TestRTPMixerOverflowResumesAtLiveEdge(t *testing.T) {
	m := testRTPMixer(t, 1, 2, 4, "alice")
	packets := tonePackets(t, 1, 12, 700)
	for _, packet := range packets {
		m.WritePacket("alice", packet)
		require.LessOrEqual(t, len(m.sources["alice"].queue), 4)
	}
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	want, err := decoder.Decode(packets[len(packets)-2].Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil), "resume from the newest target depth")
}

func TestRTPMixerLongSilenceAndResume(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 8, "alice")
	packets := tonePackets(t, 1, 2, 700)
	m.WritePacket("alice", packets[0])
	m.Render(nil)
	for range 3 {
		m.Render(nil)
	}
	for range 100 {
		require.Equal(t, make([]float32, 960), m.Render(nil))
	}
	packets[1].Timestamp = 960 * 104
	m.WritePacket("alice", packets[1])
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	want, err := decoder.Decode(packets[1].Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil), "resume with clean history, not stale PLC")
}

func TestRTPMixerLargeTimestampGapIsBounded(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "alice")
	packets := tonePackets(t, 1, 2, 700)
	packets[1].Timestamp = 960 * 1000
	m.WritePacket("alice", packets[0])
	m.WritePacket("alice", packets[1])
	m.Render(nil)
	for range 3 {
		m.Render(nil)
	}
	require.Equal(t, make([]float32, 960), m.Render(nil))
	require.NotEqual(t, make([]float32, 960), m.Render(nil))
}

func TestRTPMixerGainTapAndClamp(t *testing.T) {
	for _, gain := range []float32{0, 0.5, 1, 100, math.MaxFloat32} {
		t.Run(fmt.Sprint(gain), func(t *testing.T) { checkMixerGain(t, gain) })
	}
}

func checkMixerGain(t *testing.T, gain float32) {
	m := testRTPMixer(t, 2, 1, 4, "server")
	m.SetSourceGain("server", gain)
	m.WritePacket("server", tonePackets(t, 2, 1, 700)[0])
	var raw []float32
	out := m.Render(func(id string, pcm []float32) {
		require.Equal(t, "server", id)
		raw = append([]float32(nil), pcm...)
		m.SetSourceGain(id, 0) // A recording/control tap must not deadlock.
	})
	require.Len(t, raw, len(out))
	for i, value := range raw {
		want := float32(max(-1, min(1, float64(value)*float64(gain))))
		require.Equal(t, want, out[i], "unity must not distort quiet input")
	}
}

func TestRTPMixerDisableFlushAndIdempotence(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "alice")
	packets := tonePackets(t, 1, 3, 700)
	m.WritePacket("alice", packets[0])
	require.NoError(t, m.AddSource("alice"))
	m.SetSourceEnabled("alice", true)
	require.NotEqual(t, make([]float32, 960), m.Render(nil))
	m.WritePacket("alice", packets[1])
	m.SetSourceEnabled("alice", false)
	m.WritePacket("alice", packets[2])
	require.Equal(t, make([]float32, 960), m.Render(nil))
	m.SetSourceEnabled("alice", true)
	require.Equal(t, make([]float32, 960), m.Render(nil))
	m.WritePacket("alice", packets[0])
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	want, err := decoder.Decode(packets[0].Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil))
}

func TestRTPMixerSourceLeavePreservesOthers(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "alice", "server")
	packets := tonePackets(t, 1, 2, 700)
	for _, packet := range packets {
		m.WritePacket("server", packet)
		m.WritePacket("alice", packet)
	}
	m.Render(nil)
	m.RemoveSource("alice")
	m.RemoveSource("unknown")
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	_, err = decoder.Decode(packets[0].Payload)
	require.NoError(t, err)
	want, err := decoder.Decode(packets[1].Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil))
	require.NoError(t, m.AddSource("alice"))
	require.Empty(t, m.sources["alice"].queue)
}

func TestRTPMixerRejectsInvalidInput(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "alice")
	require.Error(t, m.AddSource(""))
	m.WritePacket("alice", nil)
	m.WritePacket("unknown", tonePackets(t, 1, 1, 700)[0])
	m.WritePacket("alice", &rtp.Packet{Payload: make([]byte, rtpMixerMaxPayload+1)})
	m.WritePacket("alice", &rtp.Packet{})
	require.Empty(t, m.sources["alice"].queue)
	for _, gain := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		m.SetSourceGain("alice", gain)
		require.Equal(t, float32(1), m.sources["alice"].gain)
	}
	m.WritePacket("alice", &rtp.Packet{Payload: []byte{0xff}})
	require.Equal(t, make([]float32, 960), m.Render(nil))
}

func TestRTPMixerConcurrentControlsAndPlayout(t *testing.T) {
	m := testRTPMixer(t, 2, 1, 4, "server")
	packets := tonePackets(t, 2, 20, 700)
	var wg sync.WaitGroup
	for worker := range 3 {
		wg.Go(func() {
			for iteration := range 100 {
				concurrentMixerAction(m, packets[iteration%len(packets)], worker)
			}
		})
	}
	wg.Wait()
}

func concurrentMixerAction(m *RTPMixer, packet *rtp.Packet, worker int) {
	switch worker {
	case 0:
		m.WritePacket("server", packet)
	case 1:
		m.Render(nil)
	case 2:
		m.SetSourceEnabled("server", false)
		m.SetSourceGain("server", 0.5)
		m.SetSourceEnabled("server", true)
		m.RemoveSource("server")
		_ = m.AddSource("server")
	}
}

func TestRTPMixerRTPClockIndependentOfOutputRate(t *testing.T) {
	packets := tonePackets(t, 2, 4, 700)
	for _, rate := range []int{8000, 12000, 16000, 24000, 48000} {
		m, err := NewRTPMixer(rate, 2, 1, 4)
		require.NoError(t, err)
		require.NoError(t, m.AddSource("server"))
		decoder, decodeErr := NewOpusDecoder(rate, 2)
		require.NoError(t, decodeErr)
		for _, packet := range packets {
			m.WritePacket("server", packet)
			want, err := decoder.Decode(packet.Payload)
			require.NoError(t, err)
			require.Equal(t, want, m.Render(nil))
		}
	}
}

func TestRTPMixerSourceMemoryBounds(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4)
	require.Error(t, m.AddSource(strings.Repeat("x", rtpMixerMaxIDBytes+1)))
	for i := range rtpMixerMaxSources {
		require.NoError(t, m.AddSource(fmt.Sprint(i)))
	}
	require.NoError(t, m.AddSource("0"), "existing IDs remain idempotent at capacity")
	require.Error(t, m.AddSource("overflow"))
	m.RemoveSource("0")
	require.NoError(t, m.AddSource("replacement"))
}

func TestRTPMixerRecoveryDropsStaleBacklog(t *testing.T) {
	m := testRTPMixer(t, 1, 2, 8, "alice")
	packets := tonePackets(t, 1, 10, 700)
	m.WritePacket("alice", packets[0])
	m.WritePacket("alice", packets[1])
	for range 10 {
		m.Render(nil)
	}
	for _, packet := range packets[6:] {
		m.WritePacket("alice", packet)
	}
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	want, err := decoder.Decode(packets[8].Payload)
	require.NoError(t, err)
	require.Equal(t, want, m.Render(nil))
}

func TestRTPMixerRejoinHasFreshDecoder(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "alice")
	packet := tonePackets(t, 1, 1, 700)[0]
	m.WritePacket("alice", packet)
	first := m.Render(nil)
	m.RemoveSource("alice")
	require.NoError(t, m.AddSource("alice"))
	m.WritePacket("alice", packet)
	require.Equal(t, first, m.Render(nil))
}

func TestRTPMixerRejectsNon20MillisecondOpus(t *testing.T) {
	encoder, err := NewOpusEncoder(mixerTestRate, 1, OpusApplicationAudio)
	require.NoError(t, err)
	for _, samples := range []int{480, 1920} {
		payload := make([]byte, rtpMixerMaxPayload)
		n, err := encoder.enc.EncodeFloat32(make([]float32, samples), payload)
		require.NoError(t, err)
		m := testRTPMixer(t, 1, 1, 4, "alice")
		m.WritePacket("alice", &rtp.Packet{Payload: payload[:n]})
		require.Equal(t, make([]float32, 960), m.Render(nil))
		require.Nil(t, m.sources["alice"].decoder, "invalid duration resets decoder history")
	}
}

func TestRTPMixerResumeAfterHalfSequenceSpace(t *testing.T) {
	m := testRTPMixer(t, 1, 1, 4, "server")
	packets := tonePackets(t, 1, 2, 700)
	m.WritePacket("server", packets[0])
	for range 5 {
		m.Render(nil)
	}
	packets[1].SequenceNumber = 40000
	packets[1].Timestamp = 40000 * 960
	m.WritePacket("server", packets[1])
	require.Len(t, m.sources["server"].queue, 1, "a long network mute may span half the sequence space")
	decoder, err := NewOpusDecoder(mixerTestRate, 1)
	require.NoError(t, err)
	assertMixerDecode(t, m, decoder, packets[1])
}
