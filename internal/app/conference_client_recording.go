package app

import (
	"fmt"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

type conferenceRecordedFrame struct {
	mix    []float32
	tracks map[string][]float32
}

// One worker per recording; playback never takes a storage mutex or waits for it.
type conferenceClientRecording struct {
	frames     chan conferenceRecordedFrame
	stop, done chan struct{}
	dropped    atomic.Uint64
	write      func(conferenceRecordedFrame)
}

func (recording *conferenceClientRecording) enqueue(mix []float32, tracks map[string][]float32) {
	frame := conferenceRecordedFrame{mix: append([]float32(nil), mix...), tracks: tracks}
	select {
	case <-recording.stop:
		return
	default:
	}
	select {
	case recording.frames <- frame:
	default:
		recording.dropped.Add(1)
	}
}

func (recording *conferenceClientRecording) run() {
	defer close(recording.done)
	for {
		select {
		case <-recording.stop:
			return
		default:
		}
		select {
		case <-recording.stop:
			return
		case frame := <-recording.frames:
			recording.write(frame)
		}
	}
}

// Shadow the mixin's start helpers for both TUI and API client call sites.
func (c *ClientApp) startRecordingInternal(mode audio.RecordingMode, rate uint32, dir string) error {
	_, active, err := c.startRecordingSession(mode, rate, dir)
	if active {
		return fmt.Errorf("recording already active")
	}
	return err
}

func (c *ClientApp) startRecordingSession(mode audio.RecordingMode, rate uint32, dir string) (string, bool, error) {
	if !c.cfg.Conference {
		return c.RecordingMixin.startRecordingSession(mode, rate, dir)
	}
	c.recorderMu.Lock()
	defer c.recorderMu.Unlock()
	if c.recorder != nil && c.recorder.IsActive() {
		return "", true, nil
	}
	// A failed recorder may still have its worker: stop it before replacement.
	if old := c.asyncRecording.Swap(nil); old != nil {
		close(old.stop)
		<-old.done
	}
	if c.recorder != nil {
		_, _, _, _ = c.recorder.Stop()
	}
	dir, err := conferenceRecordingDir(dir)
	if err != nil {
		return "", false, err
	}
	recorder := audio.NewConferenceRecorder(mode, rate, uint16(c.cfg.Channels))
	if err := recorder.Start(dir); err != nil {
		return "", false, err
	}
	known := make(map[string]bool)
	worker := &conferenceClientRecording{frames: make(chan conferenceRecordedFrame, 8), stop: make(chan struct{}), done: make(chan struct{})}
	worker.write = func(frame conferenceRecordedFrame) {
		for id := range frame.tracks {
			known[id] = true
		}
		for id := range known {
			pcm := frame.tracks[id]
			if pcm == nil {
				pcm = make([]float32, len(frame.mix))
			}
			if err := recorder.WriteTrack(id, pcm); err != nil {
				c.logger.Warn("Client conference track recording failed", "error", err)
			}
		}
		if err := recorder.WriteMix(frame.mix); err != nil {
			c.logger.Warn("Client conference mix recording failed", "error", err)
		}
		if dropped := worker.dropped.Swap(0); dropped != 0 {
			c.logger.Warn("Client recording overflow; frames dropped", "frames", dropped)
		}
	}
	c.recorder = recorder
	c.asyncRecording.Store(worker)
	go worker.run()
	return recorder.Dir(), false, nil
}
