package app

import (
	"log/slog"
	"os"
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func testSharedLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestDecodeAudioStream_ValidOpusData(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const frameSize = sampleRate / 50 // 960 samples per 20ms frame

	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)

	silence := make([]float32, frameSize)
	var encodedFrames [][]byte
	for i := 0; i < 3; i++ {
		data, err := enc.Encode(silence)
		require.NoError(t, err)
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)
		encodedFrames = append(encodedFrames, dataCopy)
	}

	dec, err := audio.NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	inCh := make(chan []byte, len(encodedFrames))
	for _, frame := range encodedFrames {
		inCh <- frame
	}
	close(inCh)

	playbackCh := make(chan []float32, 10)

	decodeAudioStream(testSharedLogger(), inCh, dec, playbackCh, nil, nil)

	count := 0
	for range playbackCh {
		count++
		if count >= len(encodedFrames) {
			break
		}
	}
	assert.Equal(t, len(encodedFrames), count)
}

func TestDecodeAudioStream_InvalidOpusData(t *testing.T) {
	t.Parallel()

	dec, err := audio.NewOpusDecoder(48000, 1)
	require.NoError(t, err)

	inCh := make(chan []byte, 3)
	inCh <- []byte{} // empty frame — guaranteed to fail
	close(inCh)

	playbackCh := make(chan []float32, 10)

	// Should not panic; errors are logged and skipped
	decodeAudioStream(testSharedLogger(), inCh, dec, playbackCh, nil, nil)

	assert.Len(t, playbackCh, 0)
}

func TestDecodeAudioStream_FullPlaybackChannel(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const frameSize = sampleRate / 50

	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)

	silence := make([]float32, frameSize)
	data, err := enc.Encode(silence)
	require.NoError(t, err)
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	dec, err := audio.NewOpusDecoder(sampleRate, channels)
	require.NoError(t, err)

	inCh := make(chan []byte, 5)
	for i := 0; i < 5; i++ {
		frame := make([]byte, len(dataCopy))
		copy(frame, dataCopy)
		inCh <- frame
	}
	close(inCh)

	playbackCh := make(chan []float32, 1) // tiny buffer

	// Should not block — excess frames are dropped via select/default
	decodeAudioStream(testSharedLogger(), inCh, dec, playbackCh, nil, nil)
}

func TestSetupAudioDecoder_RegistersCallback(t *testing.T) {
	t.Parallel()

	var onAudioTrackCalled bool
	peer := &sharedMockPeer{
		onAudioTrackFunc: func(handler func(inCh <-chan []byte)) {
			onAudioTrackCalled = true
		},
	}

	playbackCh := make(chan []float32, 1)
	setupAudioDecoder(testSharedLogger(), peer, 48000, 1, playbackCh, nil, nil)
	assert.True(t, onAudioTrackCalled)
}

func TestSetupAudioDecoder_DecodesAndCloses(t *testing.T) {
	t.Parallel()

	const sampleRate = 48000
	const channels = 1
	const frameSize = sampleRate / 50

	enc, err := audio.NewOpusEncoder(sampleRate, channels, "audio")
	require.NoError(t, err)

	silence := make([]float32, frameSize)
	data, err := enc.Encode(silence)
	require.NoError(t, err)
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	var handler func(inCh <-chan []byte)
	peer := &sharedMockPeer{
		onAudioTrackFunc: func(h func(inCh <-chan []byte)) {
			handler = h
		},
	}

	playbackCh := make(chan []float32, 10)
	setupAudioDecoder(testSharedLogger(), peer, sampleRate, channels, playbackCh, nil, nil)
	require.NotNil(t, handler)

	inCh := make(chan []byte, 1)
	inCh <- dataCopy
	close(inCh)

	handler(inCh)

	// playbackCh should now be closed by the callback
	var frames [][]float32
	for frame := range playbackCh {
		frames = append(frames, frame)
	}
	assert.Len(t, frames, 1)
	assert.Len(t, frames[0], frameSize)
}

// sharedMockPeer implements transport.PeerManager for shared.go tests.
type sharedMockPeer struct {
	onAudioTrackFunc func(handler func(inCh <-chan []byte))
}

func (m *sharedMockPeer) CreatePeerConnection(iceConfig transport.ICEConfig) error { return nil }
func (m *sharedMockPeer) Close() error                                             { return nil }
func (m *sharedMockPeer) OnConnectionStateChange(handler func(state webrtc.PeerConnectionState)) {
}
func (m *sharedMockPeer) GetStats() transport.ConnectionStats { return transport.ConnectionStats{} }
func (m *sharedMockPeer) AddAudioTrack(sampleRate, channels uint32) (chan<- []byte, error) {
	return make(chan []byte, 1), nil
}
func (m *sharedMockPeer) OnAudioTrack(handler func(inCh <-chan []byte)) {
	if m.onAudioTrackFunc != nil {
		m.onAudioTrackFunc(handler)
	}
}
func (m *sharedMockPeer) CreateOffer() (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{}, nil
}
func (m *sharedMockPeer) CreateAnswer(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	return webrtc.SessionDescription{}, nil
}
func (m *sharedMockPeer) SetRemoteDescription(sdp webrtc.SessionDescription) error { return nil }
func (m *sharedMockPeer) AddICECandidate(candidate webrtc.ICECandidateInit) error  { return nil }
func (m *sharedMockPeer) OnICECandidate(handler func(candidate *webrtc.ICECandidate)) {
}
func (m *sharedMockPeer) CreateDataChannel(label string) error { return nil }
func (m *sharedMockPeer) OnDataChannel(handler func(label string, msgCh <-chan []byte, sendFn func([]byte) error)) {
}
func (m *sharedMockPeer) CreateControlDataChannel() error { return nil }
func (m *sharedMockPeer) DCReady() <-chan struct{}        { return make(chan struct{}) }
func (m *sharedMockPeer) SendControl(action string, payload interface{}) error {
	return nil
}
func (m *sharedMockPeer) ControlMessages() <-chan []byte { return nil }
func (m *sharedMockPeer) CreateChatDataChannel() error   { return nil }
func (m *sharedMockPeer) SendChat([]byte) error          { return nil }
func (m *sharedMockPeer) ChatMessages() <-chan []byte    { return nil }
