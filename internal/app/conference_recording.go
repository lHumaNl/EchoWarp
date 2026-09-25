package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

const (
	conferenceRecordingQueue  = 128
	conferenceRecordingTarget = 1
	conferenceRecordingDepth  = 8
	conferenceRecordingTick   = 20 * time.Millisecond
)

type conferenceRecordingSources map[string]conferenceSource

type conferenceRecordingPacket struct {
	id         string
	generation uint64
	packet     *rtp.Packet
}

type conferenceRecording struct {
	packets chan conferenceRecordingPacket
	mixer   *audio.RTPMixer
	sources conferenceRecordingSources
	dropped atomic.Uint64
}

// AttachRoom must precede participant creation, recording, and worker startup.
// The room tap only copies/enqueues encoded RTP. No decoder, recorder lock, disk
// operation, or logger is reachable from it. Unattached handlers remain mono.
func (ch *ConferenceHandler) AttachRoom(room *ConferenceRoom, channels uint32) {
	ch.room, ch.channels = room, channels
	if room != nil {
		ch.refreshRecordingSources()
	}
}

func (ch *ConferenceHandler) refreshRecordingSources() conferenceRecordingSources {
	ch.room.mu.RLock()
	sources := make(conferenceRecordingSources, len(ch.room.sources))
	for id, source := range ch.room.sources {
		sources[id] = *source
	}
	ch.room.mu.RUnlock()
	ch.sourceRoster.Store(&sources)
	return sources
}

// The callback captures its session: a racing Stop/Start cannot enqueue an old
// packet in a new recording. Overflow drops newest and counts loss, never waits.
func (ch *ConferenceHandler) observeRecordingPacket(session *conferenceRecording, id string, packet *rtp.Packet) {
	if ch.recording.Load() != session {
		return
	}
	sources := ch.sourceRoster.Load()
	if sources == nil {
		return
	}
	source, ok := (*sources)[id]
	if !ok || source.blocked || source.incomingBlocked || source.paused || source.ended {
		return
	}
	select {
	case session.packets <- conferenceRecordingPacket{id, source.generation, packet.Clone()}:
	default:
		session.dropped.Add(1)
	}
}

// StartRecording publishes a fresh queue and decoder set only after storage is
// ready. Each attached session gets a unique parent to avoid same-second WAV
// truncation by ConferenceRecorder's timestamp-based directory naming.
func (ch *ConferenceHandler) StartRecording(mode audio.RecordingMode, sampleRate uint32, baseDir string) error {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	if ch.recorder != nil {
		return errors.New("conference recording already started; stop it before restarting")
	}
	channels := uint32(1)
	if ch.room != nil {
		channels = ch.channels
	}
	session, err := newConferenceRecording(sampleRate, channels)
	if err != nil {
		return err
	}
	return ch.startRecordingSession(session, mode, sampleRate, channels, baseDir)
}

func newConferenceRecording(sampleRate, channels uint32) (*conferenceRecording, error) {
	mixer, err := audio.NewRTPMixer(int(sampleRate), int(channels), conferenceRecordingTarget, conferenceRecordingDepth)
	if err != nil {
		return nil, err
	}
	return &conferenceRecording{mixer: mixer, packets: make(chan conferenceRecordingPacket, conferenceRecordingQueue),
		sources: make(conferenceRecordingSources)}, nil
}

func (ch *ConferenceHandler) startRecordingSession(session *conferenceRecording, mode audio.RecordingMode, rate, channels uint32, dir string) error {
	if ch.room != nil {
		var err error
		dir, err = conferenceRecordingDir(dir)
		if err != nil {
			return err
		}
	}
	recorder := audio.NewConferenceRecorder(mode, rate, uint16(channels))
	if err := recorder.Start(dir); err != nil {
		return err
	}
	ch.recorder = recorder
	ch.activateRecording(session)
	return nil
}

func conferenceRecordingDir(base string) (string, error) {
	if err := os.MkdirAll(base, 0o750); err != nil {
		return "", fmt.Errorf("create recording root: %w", err)
	}
	dir, err := os.MkdirTemp(base, "conference-")
	if err != nil {
		return "", fmt.Errorf("create recording session: %w", err)
	}
	return dir, nil
}

func (ch *ConferenceHandler) activateRecording(session *conferenceRecording) {
	if ch.room == nil {
		return
	}
	session.syncSources(ch.refreshRecordingSources(), ch.logger)
	for id := range session.sources {
		ch.logRecordingError(ch.recorder.WriteTrack(id, nil))
	}
	ch.recording.Store(session)
	ch.room.ObservePackets(func(id string, packet *rtp.Packet) { ch.observeRecordingPacket(session, id, packet) })
}

// StopRecording discards pending encoded audio instead of draining a stale tail.
// It joins an in-progress render/write through recordingMu, never a room lock.
func (ch *ConferenceHandler) StopRecording() (duration time.Duration, totalSize uint64, fileCount int, err error) {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	ch.recording.Store(nil)
	if ch.room != nil {
		ch.room.ObservePackets(nil)
	}
	if ch.recorder == nil {
		return 0, 0, 0, nil
	}
	duration, totalSize, fileCount, err = ch.recorder.Stop()
	ch.recorder = nil
	return duration, totalSize, fileCount, err
}

func (ch *ConferenceHandler) RecordingDir() string {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	if ch.recorder == nil {
		return ""
	}
	return ch.recorder.Dir()
}

func (ch *ConferenceHandler) IsRecording() bool {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	return ch.recorder != nil && ch.recorder.IsActive()
}

func (ch *ConferenceHandler) FlushHeaders() {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	if ch.recorder != nil {
		ch.recorder.FlushHeaders()
	}
}

// RunRecording replaces the legacy recordMixLoop. Run once in the server context
// (duplicate invocations return); the owner must StopRecording to finalize WAVs.
// Slow storage stalls only this consumer: ingress drops newest at 128 packets.
func (ch *ConferenceHandler) RunRecording(ctx context.Context) {
	if ch.room == nil || !ch.recordingRunning.CompareAndSwap(false, true) {
		return
	}
	defer ch.recordingRunning.Store(false)
	ticker := time.NewTicker(conferenceRecordingTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ch.renderRecording()
		}
	}
}

func (ch *ConferenceHandler) renderRecording() {
	ch.recordingMu.Lock()
	defer ch.recordingMu.Unlock()
	session := ch.recording.Load()
	if session == nil || !ch.recorder.IsActive() {
		ch.recording.Store(nil)
		ch.room.ObservePackets(nil)
		return
	}
	session.syncSources(ch.refreshRecordingSources(), ch.logger)
	session.drainPackets()
	ch.writeRecordingFrame(session)
	if dropped := session.dropped.Swap(0); dropped != 0 {
		ch.logger.Warn("Conference recording queue overflow; newest packets dropped", "packets", dropped)
	}
}

func (session *conferenceRecording) drainPackets() {
	// A fixed budget prevents continuous ingress from starving the output tick.
	for range conferenceRecordingQueue {
		select {
		case packet := <-session.packets:
			if source, ok := session.sources[packet.id]; ok && source.generation == packet.generation {
				session.mixer.WritePacket(packet.id, packet.packet)
			}
		default:
			return
		}
	}
}

func (ch *ConferenceHandler) writeRecordingFrame(session *conferenceRecording) {
	tracks := make(map[string][]float32, len(session.sources))
	mix := session.mixer.Render(func(id string, pcm []float32) { tracks[id] = pcm })
	silence := make([]float32, len(mix))
	for id := range session.sources {
		pcm := tracks[id]
		if pcm == nil {
			pcm = silence
		}
		ch.logRecordingError(ch.recorder.WriteTrack(id, pcm))
	}
	ch.logRecordingError(ch.recorder.WriteMix(mix))
}

func (ch *ConferenceHandler) logRecordingError(err error) {
	if err != nil {
		ch.logger.Warn("Conference recording write failed", "error", err)
	}
}

func (session *conferenceRecording) syncSources(sources conferenceRecordingSources, logger *slog.Logger) {
	for id, previous := range session.sources {
		if current, ok := sources[id]; !ok || current.generation != previous.generation {
			session.mixer.RemoveSource(id)
		}
	}
	for id, source := range sources {
		if err := session.mixer.AddSource(id); err != nil {
			logger.Warn("Conference recording source failed", "id", id, "error", err)
			continue
		}
		session.mixer.SetSourceEnabled(id, !source.blocked && !source.incomingBlocked && !source.paused && !source.ended)
		session.mixer.SetSourceGain(id, source.gain)
	}
	session.sources = sources
}
