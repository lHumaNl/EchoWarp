package app

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/app/mocks"
	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestConferenceFailureStopsMessageLoopStats(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	client := NewClientApp(clientTestConfig(), testClientLogger(), nil)
	client.WithStatsHook(func(transport.ConnectionStats) {})
	peer := transport.NewWebRTCPeer(transport.DirectionDuplex)
	t.Cleanup(func() { _ = peer.Close() })
	audioDone := make(chan error, 1)
	failure := errors.New("conference hello timeout")
	audioDone <- failure
	done := make(chan error, 1)
	go func() {
		done <- client.runMessageLoop(ctx, mocks.NewMockSignaler("localhost"), peer, audioDone, time.Now())
	}()
	select {
	case err := <-done:
		require.ErrorIs(t, err, failure)
	case <-time.After(time.Second):
		cancel()
		t.Fatal("conference failure deadlocked waiting for stats")
	}
	require.NoError(t, ctx.Err(), "the loop owns cancellation; it cannot depend on caller shutdown")
}

func TestConferenceClientRecordingStereoAndTracks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Conference, cfg.Channels, cfg.RecordDir = true, 2, t.TempDir()
	client := NewClientApp(cfg, testClientLogger(), nil)
	require.NoError(t, client.StartRecording(echowarp.RecordingModeBoth))
	t.Cleanup(func() { _, _ = client.StopRecording() })
	worker := client.asyncRecording.Load()
	original := worker.write
	var written atomic.Int32
	worker.write = func(frame conferenceRecordedFrame) { original(frame); written.Add(1) }
	pcm := make([]float32, 1920)
	for i := range pcm {
		pcm[i] = .1
	}
	worker.enqueue(pcm, map[string][]float32{"bob": append([]float32(nil), pcm...)})
	for i := range pcm {
		pcm[i] = 0
	} // Worker must own a copy of the mixed output.
	require.Eventually(t, func() bool { return written.Load() == 1 }, time.Second, time.Millisecond)
	result, err := client.StopRecording()
	require.NoError(t, err)
	require.Len(t, result.Files, 2)
	for _, file := range result.Files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Equal(t, uint16(2), binary.LittleEndian.Uint16(data[22:24]), filepath.Base(file))
		require.NotEqual(t, make([]byte, len(data)-44), data[44:])
	}
}

func TestConferenceClientRecordingSlowWriterDoesNotBlockPlayback(t *testing.T) {
	session, _ := newClientTest(t)
	var mixin RecordingMixin
	started, release := make(chan struct{}), make(chan struct{})
	worker := &conferenceClientRecording{frames: make(chan conferenceRecordedFrame, 2), stop: make(chan struct{}), done: make(chan struct{})}
	var calls atomic.Int32
	worker.write = func(conferenceRecordedFrame) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
	}
	mixin.asyncRecording.Store(worker)
	session.recording = &mixin
	go worker.run()
	t.Cleanup(func() { close(release); close(worker.stop); <-worker.done })
	out := make(chan []float32, 20)
	session.render(out, nil)
	<-started
	done := make(chan struct{})
	go func() {
		for range 10 {
			session.render(out, nil)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("storage stalled conference playback")
	}
	require.Equal(t, 11, len(out))
	require.Equal(t, uint64(8), worker.dropped.Load())
}
