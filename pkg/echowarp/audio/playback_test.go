package audio

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMalgoPlayerCallbackDiagnosticsFullCopy(t *testing.T) {
	player := &MalgoPlayer{channels: 1, callbackClock: fixedCallbackClock(time.Second)}
	inCh := make(chan []float32, 1)
	inCh <- []float32{0.25, -0.5}
	out := make([]byte, 8)

	player.createOnSendCallback(inCh)(out, nil, 2)
	snapshot := player.PlaybackDiagnostics()

	assert.Equal(t, float32(0.25), sampleAt(out, 0))
	assert.Equal(t, float32(-0.5), sampleAt(out, 1))
	assert.Equal(t, uint64(1), snapshot.CallbackCount)
	assert.Zero(t, snapshot.SilenceFillsTotal)
	assert.Zero(t, snapshot.ZeroFilledSamples)
}

func TestMalgoPlayerCallbackDiagnosticsPartialSilence(t *testing.T) {
	player := &MalgoPlayer{channels: 1, callbackClock: fixedCallbackClock(time.Second)}
	inCh := make(chan []float32, 1)
	inCh <- []float32{0.5}
	out := filledBytes(8, 0xff)

	player.createOnSendCallback(inCh)(out, nil, 2)
	snapshot := player.PlaybackDiagnostics()

	assert.Equal(t, float32(0.5), sampleAt(out, 0))
	assert.Zero(t, sampleAt(out, 1))
	assert.Equal(t, uint64(1), snapshot.SilenceFillsTotal)
	assert.Equal(t, uint64(1), snapshot.PartialSilenceFills)
	assert.Equal(t, uint64(1), snapshot.ZeroFilledSamples)
}

func TestMalgoPlayerCallbackDiagnosticsFullSilence(t *testing.T) {
	player := &MalgoPlayer{channels: 2, callbackClock: fixedCallbackClock(time.Second)}
	inCh := make(chan []float32)
	out := filledBytes(16, 0xff)

	player.createOnSendCallback(inCh)(out, nil, 2)
	snapshot := player.PlaybackDiagnostics()

	assert.Empty(t, nonZeroBytes(out))
	assert.Equal(t, uint64(1), snapshot.SilenceFillsTotal)
	assert.Equal(t, uint64(1), snapshot.FullSilenceFills)
	assert.Equal(t, uint64(4), snapshot.ZeroFilledSamples)
	assert.Equal(t, uint32(2), snapshot.FirstCallbackFrameCount)
}

func TestMalgoPlayerCallbackDiagnosticsGaps(t *testing.T) {
	player := &MalgoPlayer{channels: 1}
	player.callbackClock = sequenceCallbackClock(
		time.Second,
		time.Second+30*time.Millisecond,
		time.Second+75*time.Millisecond,
	)
	inCh := make(chan []float32)
	callback := player.createOnSendCallback(inCh)

	callback(make([]byte, 4), nil, 1)
	callback(make([]byte, 4), nil, 1)
	callback(make([]byte, 4), nil, 1)
	snapshot := player.PlaybackDiagnostics()

	assert.Equal(t, uint64(3), snapshot.CallbackCount)
	assert.Equal(t, 45*time.Millisecond, snapshot.CallbackGapMax)
	assert.Equal(t, uint64(2), snapshot.CallbackLate25MS)
	assert.Equal(t, uint64(1), snapshot.CallbackLate40MS)
}

func sampleAt(buf []byte, index int) float32 {
	bits := binary.LittleEndian.Uint32(buf[index*4:])
	return math.Float32frombits(bits)
}

func filledBytes(size int, value byte) []byte {
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = value
	}
	return buf
}

func nonZeroBytes(buf []byte) []byte {
	var result []byte
	for _, value := range buf {
		if value != 0 {
			result = append(result, value)
		}
	}
	return result
}

func fixedCallbackClock(value time.Duration) func() int64 {
	return func() int64 { return value.Nanoseconds() }
}

func sequenceCallbackClock(values ...time.Duration) func() int64 {
	index := 0
	return func() int64 {
		value := values[index]
		index++
		return value.Nanoseconds()
	}
}
