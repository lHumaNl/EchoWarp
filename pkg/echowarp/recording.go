package echowarp

import "time"

// RecordingMode selects what the recorder writes when Node.StartRecording
// is invoked. The three values mirror audio.RecordingMode but are kept in
// the public package so API callers do not need to import pkg/echowarp/audio.
//
// The string values are the wire format accepted by POST /api/v1/recording/start
// and by Node.StartRecording — they match the --record CLI flag tokens so the
// REST API and the CLI stay in lockstep.
type RecordingMode string

const (
	// RecordingModeMix records the total conference mix into a single
	// mix.wav file.
	RecordingModeMix RecordingMode = "mix"
	// RecordingModeTracks records each participant into a separate
	// <participant>.wav file.
	RecordingModeTracks RecordingMode = "tracks"
	// RecordingModeBoth records both the total mix and per-participant
	// tracks in the same output directory.
	RecordingModeBoth RecordingMode = "both"
)

// RecordingResult is returned by Node.StopRecording and describes the
// output of a completed recording session. Duration is the wall-clock
// length of the session; Size is the total number of audio bytes
// written across every file (WAV header is not included); Files is a
// freshly allocated slice of absolute paths pointing at every .wav file
// the recorder produced — empty when no files were written.
type RecordingResult struct {
	// Duration is the wall-clock length of the recording session.
	Duration time.Duration `json:"duration"`
	// DurationMs is the duration in milliseconds. Emitted alongside
	// Duration so JSON clients that cannot decode Go's duration-as-int64
	// (nanoseconds) representation have an unambiguous integer field
	// to consume.
	DurationMs int64 `json:"duration_ms"`
	// Size is the total bytes of audio data (all files combined).
	Size int64 `json:"size"`
	// Files is the list of absolute file paths produced during the
	// session. Empty (non-nil) when no files were written.
	Files []string `json:"files"`
}

// RecordingStatus is returned by Node.RecordingStatus and describes the
// current state of the recorder, if any. The struct is idempotent-safe:
// a zero-value RecordingStatus (Active=false, Mode="", Duration=0,
// Size=0, StartedAt=time.Time{}) is the canonical "not recording"
// response and is what GET /api/v1/recording/status returns when the
// node is idle, has no runner, or the runner does not implement
// RecordingController.
type RecordingStatus struct {
	// Active is true while a recording is in progress.
	Active bool `json:"active"`
	// Mode is the recording mode currently in use (only meaningful
	// when Active is true).
	Mode RecordingMode `json:"mode,omitempty"`
	// Duration is the elapsed time since the recording started.
	Duration time.Duration `json:"duration"`
	// DurationMs is the elapsed time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
	// Size is the total bytes of audio data written so far. Only
	// populated after Stop — during an active session the recorder
	// holds per-writer counters that are expensive to aggregate, so
	// Size is reported as 0 while Active is true. Clients that need
	// a running byte counter should poll GET /api/v1/stats instead.
	Size int64 `json:"size"`
	// StartedAt is the wall-clock time the recording started. Zero
	// value when Active is false.
	StartedAt time.Time `json:"started_at,omitempty"`
}

// IsValidRecordingMode reports whether s is one of the three accepted
// RecordingMode tokens. Exported so the API layer, the CLI parser, and
// external library consumers can share a single source of truth for
// what "mix|tracks|both" means.
func IsValidRecordingMode(s string) bool {
	switch RecordingMode(s) {
	case RecordingModeMix, RecordingModeTracks, RecordingModeBoth:
		return true
	}
	return false
}
