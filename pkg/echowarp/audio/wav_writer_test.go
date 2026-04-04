package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWAVWriter_CreatesValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.wav")

	w, err := NewWAVWriter(path, 48000, 1)
	require.NoError(t, err)

	samples := make([]float32, 960)
	for i := range samples {
		samples[i] = 0.5
	}
	require.NoError(t, w.WriteSamples(samples))
	require.NoError(t, w.Close())

	// Verify file header
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, "RIFF", string(data[0:4]))
	assert.Equal(t, "WAVE", string(data[8:12]))
	assert.Equal(t, "fmt ", string(data[12:16]))
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(data[20:22]))     // PCM
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(data[22:24]))     // channels
	assert.Equal(t, uint32(48000), binary.LittleEndian.Uint32(data[24:28])) // sample rate
	assert.Equal(t, "data", string(data[36:40]))

	dataSize := binary.LittleEndian.Uint32(data[40:44])
	assert.Equal(t, uint32(960*2), dataSize)
}

func TestWAVWriter_ClipsSamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.wav")

	w, err := NewWAVWriter(path, 48000, 1)
	require.NoError(t, err)

	samples := []float32{1.5, -1.5, 0.0}
	require.NoError(t, w.WriteSamples(samples))
	require.NoError(t, w.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Samples start at offset 44
	s0 := int16(binary.LittleEndian.Uint16(data[44:46]))
	s1 := int16(binary.LittleEndian.Uint16(data[46:48]))
	s2 := int16(binary.LittleEndian.Uint16(data[48:50]))

	assert.Equal(t, int16(32767), s0)  // clamped
	assert.Equal(t, int16(-32767), s1) // clamped
	assert.Equal(t, int16(0), s2)
}

func TestWAVWriter_WriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "closed.wav")

	w, err := NewWAVWriter(path, 48000, 1)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	err = w.WriteSamples([]float32{0.5})
	assert.Error(t, err)
}

func TestWAVWriter_DataSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "size.wav")

	w, err := NewWAVWriter(path, 48000, 1)
	require.NoError(t, err)

	assert.Equal(t, uint32(0), w.DataSize())
	require.NoError(t, w.WriteSamples(make([]float32, 100)))
	assert.Equal(t, uint32(200), w.DataSize()) // 100 samples * 2 bytes
	require.NoError(t, w.Close())
}

func TestConferenceRecorder_MixMode(t *testing.T) {
	dir := t.TempDir()
	r := NewConferenceRecorder(RecordMix, 48000, 1)

	require.NoError(t, r.Start(dir))
	assert.True(t, r.IsActive())

	samples := make([]float32, 960)
	require.NoError(t, r.WriteMix(samples))
	// WriteTrack should be no-op in mix mode
	require.NoError(t, r.WriteTrack("client-1", samples))

	dur, size, files, err := r.Stop()
	require.NoError(t, err)
	assert.False(t, r.IsActive())
	assert.True(t, dur >= 0)
	assert.Equal(t, uint64(960*2), size)
	assert.Equal(t, 1, files) // only mix.wav
}

func TestConferenceRecorder_TracksMode(t *testing.T) {
	dir := t.TempDir()
	r := NewConferenceRecorder(RecordTracks, 48000, 1)

	require.NoError(t, r.Start(dir))

	samples := make([]float32, 960)
	require.NoError(t, r.WriteTrack("server", samples))
	require.NoError(t, r.WriteTrack("client-1", samples))

	dur, size, files, err := r.Stop()
	require.NoError(t, err)
	assert.True(t, dur >= 0)
	assert.Equal(t, uint64(960*2*2), size) // 2 tracks
	assert.Equal(t, 2, files)
}

func TestConferenceRecorder_BothMode(t *testing.T) {
	dir := t.TempDir()
	r := NewConferenceRecorder(RecordBoth, 48000, 1)

	require.NoError(t, r.Start(dir))

	samples := make([]float32, 960)
	require.NoError(t, r.WriteMix(samples))
	require.NoError(t, r.WriteTrack("client-1", samples))

	_, _, files, err := r.Stop()
	require.NoError(t, err)
	assert.Equal(t, 2, files) // mix + 1 track
}

func TestConferenceRecorder_DoubleStart(t *testing.T) {
	dir := t.TempDir()
	r := NewConferenceRecorder(RecordMix, 48000, 1)

	require.NoError(t, r.Start(dir))
	err := r.Start(dir)
	assert.Error(t, err)

	r.Stop()
}
