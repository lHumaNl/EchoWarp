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
//
// All public methods take the internal lock — callers should not access the
// embedded fields directly. This keeps the mutex-protected invariants intact
// regardless of caller (TUI bridge, daemon API adapter, etc.).
type RecordingMixin struct {
	recorder   *audio.ConferenceRecorder
	recorderMu sync.Mutex
	recState   recordingAdapterState
}

// startRecordingInternal starts non-conference recording unconditionally.
// Used by the TUI bridge which has its own single-goroutine ordering and
// never races against itself. For daemon API use startRecordingSession to
// get an atomic check-and-start that refuses concurrent starts.
//
// sampleRate and outDir are passed explicitly so the mixin does not depend on
// the embedding struct's config.
func (r *RecordingMixin) startRecordingInternal(mode audio.RecordingMode, sampleRate uint32, outDir string) error {
	r.recorderMu.Lock()
	defer r.recorderMu.Unlock()
	r.recorder = audio.NewConferenceRecorder(mode, sampleRate, 1)
	return r.recorder.Start(outDir)
}

// startRecordingSession atomically checks whether a recording is already
// active and, if not, starts a new one. Returns the recording directory on
// success. This is the preferred entry point for concurrent callers (e.g.
// HTTP handlers) because the check-and-start is guarded by a single lock
// acquisition — two concurrent calls cannot both succeed and leak the first
// recorder like the TUI-oriented startRecordingInternal allows.
//
// alreadyActive=true when a recording is in progress; err is nil in that
// case so callers can translate it to a 409-style response without needing
// to distinguish it from a real Start failure.
func (r *RecordingMixin) startRecordingSession(mode audio.RecordingMode, sampleRate uint32, outDir string) (dir string, alreadyActive bool, err error) {
	r.recorderMu.Lock()
	defer r.recorderMu.Unlock()
	if r.recorder != nil && r.recorder.IsActive() {
		return "", true, nil
	}
	rec := audio.NewConferenceRecorder(mode, sampleRate, 1)
	if err := rec.Start(outDir); err != nil {
		return "", false, err
	}
	r.recorder = rec
	return rec.Dir(), false, nil
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
