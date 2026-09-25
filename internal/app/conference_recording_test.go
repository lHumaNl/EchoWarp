package app

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestConferenceRecordingFormatAndIsolation(t *testing.T) {
	for _, channels := range []uint32{1, 2} {
		t.Run(fmt.Sprint(channels), func(t *testing.T) {
			room := NewConferenceRoom(testClientLogger())
			t.Cleanup(room.Close)
			room.AddServerSource()
			handler := NewConferenceHandler(960*int(channels), 48000, false, testClientLogger())
			handler.AttachRoom(room, channels)
			handler.AddParticipant("server")
			root := t.TempDir()
			require.NoError(t, handler.StartRecording(audio.RecordBoth, 48000, root))
			t.Cleanup(func() { _, _, _, _ = handler.StopRecording() })
			path := handler.RecordingDir()
			encoder, err := audio.NewOpusEncoder(48000, int(channels), "audio")
			require.NoError(t, err)
			before := room.State("server", true)
			for frame := range 10 {
				pcm := make([]float32, 960*int(channels))
				for i := range 960 {
					for ch := range int(channels) {
						pcm[i*int(channels)+ch] = float32(.2 * math.Sin(2*math.Pi*600*float64(frame*960+i)/48000))
					}
				}
				payload, err := encoder.Encode(pcm)
				require.NoError(t, err)
				room.WriteSource("server", &rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: uint16(frame), Timestamp: uint32(frame * 960), SSRC: 1}, Payload: payload})
				audio.PutOpusOutput(payload)
				handler.renderRecording()
			}
			_, _, files, err := handler.StopRecording()
			require.NoError(t, err)
			require.Equal(t, 2, files)
			require.Equal(t, before, room.State("server", true), "recording must not consume routing state")
			for _, name := range []string{"mix.wav", "server.wav"} {
				data, err := os.ReadFile(filepath.Join(path, name))
				require.NoError(t, err)
				require.Equal(t, uint16(channels), binary.LittleEndian.Uint16(data[22:24]))
				require.Equal(t, uint32(48000), binary.LittleEndian.Uint32(data[24:28]))
				bits := int(binary.LittleEndian.Uint16(data[34:36]))
				require.Equal(t, 10*960*int(channels)*bits/8, int(binary.LittleEndian.Uint32(data[40:44])))
				require.NotEqual(t, make([]byte, len(data)-44), data[44:])
			}
			// A fresh session must not replay pending packets or truncate previous files.
			require.NoError(t, handler.StartRecording(audio.RecordBoth, 48000, root))
			require.NotEqual(t, path, handler.RecordingDir())
			require.Empty(t, handler.recording.Load().packets)
			handler.renderRecording()
			newPath := handler.RecordingDir()
			_, _, _, err = handler.StopRecording()
			require.NoError(t, err)
			data, err := os.ReadFile(filepath.Join(newPath, "mix.wav"))
			require.NoError(t, err)
			require.Equal(t, make([]byte, len(data)-44), data[44:])
		})
	}
}

func TestConferenceRecordingSlowStorageNeverBlocksIngress(t *testing.T) {
	room := NewConferenceRoom(testClientLogger())
	t.Cleanup(room.Close)
	room.AddServerSource()
	handler := NewConferenceHandler(960, 48000, false, testClientLogger())
	handler.AttachRoom(room, 1)
	require.NoError(t, handler.StartRecording(audio.RecordTracks, 48000, t.TempDir()))
	t.Cleanup(func() { _, _, _, _ = handler.StopRecording() })
	handler.recordingMu.Lock() // Model a writer stalled in storage.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range conferenceRecordingQueue * 3 {
			room.WriteSource("server", &rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: uint16(i), Timestamp: uint32(i * 960), SSRC: 1}, Payload: []byte{0xf8, 0xff, 0xfe}})
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		handler.recordingMu.Unlock()
		t.Fatal("recording storage blocked router")
	}
	handler.recordingMu.Unlock()
	session := handler.recording.Load()
	require.Equal(t, conferenceRecordingQueue, len(session.packets))
	require.Equal(t, uint64(conferenceRecordingQueue*2), session.dropped.Load())
}
