package app

import (
	"sync"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// RecordingMixin embeds recording fields and methods shared by ServerApp and
// ClientApp for non-conference recording. Both structs embed this type so the
// duplicated recorder/recorderMu/recState triple and start/stop/isActive
// helpers live in one place.
type RecordingMixin struct {
	recorder   *audio.ConferenceRecorder
	recorderMu sync.Mutex
	recState   recordingAdapterState
}

// startRecordingInternal starts non-conference recording.
// sampleRate and outDir are passed explicitly so the mixin does not depend on
// the embedding struct's config.
func (r *RecordingMixin) startRecordingInternal(mode audio.RecordingMode, sampleRate uint32, outDir string) error {
	r.recorderMu.Lock()
	defer r.recorderMu.Unlock()
	r.recorder = audio.NewConferenceRecorder(mode, sampleRate, 1)
	return r.recorder.Start(outDir)
}

// stopRecordingInternal stops non-conference recording and returns stats.
func (r *RecordingMixin) stopRecordingInternal() (time.Duration, uint64, int, error) {
	r.recorderMu.Lock()
	defer r.recorderMu.Unlock()
	if r.recorder == nil {
		return 0, 0, 0, nil
	}
	dur, size, files, err := r.recorder.Stop()
	r.recorder = nil
	return dur, size, files, err
}

// isRecordingActive returns whether the recorder is currently active.
func (r *RecordingMixin) isRecordingActive() bool {
	r.recorderMu.Lock()
	defer r.recorderMu.Unlock()
	return r.recorder != nil && r.recorder.IsActive()
}
