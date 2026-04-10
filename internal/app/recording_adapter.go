package app

import (
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// recordingAdapterState holds the bookkeeping that the underlying
// audio.ConferenceRecorder does not expose directly: the mode passed
// to StartRecording, the wall-clock start time, and the directory we
// Glob for the final file list on Stop. We could plumb all of this
// through ConferenceHandler / ConferenceRecorder but doing so would
// bloat the audio package for a single caller — the adapter pattern
// (mirroring ban_adapter.go from phase 5b) keeps the knowledge local
// to the daemon/API boundary.
//
// TODO(task-014): ClientApp and non-conference ServerApp paths
// construct a ConferenceRecorder but never call
// SubmitAudio/WriteMix/WriteTrack, so the produced WAV files are
// empty 44-byte headers. See .tasks/014-wav-recorder-tui.md bug #2.
// The adapter here exposes the same broken behavior verbatim — we
// deliberately do not add a parallel recorder instance; task 014 owns
// the fix for wiring the audio pipeline into the non-conference
// recorder. API callers that hit Start/Stop on a non-conference
// server or on a client will see success responses with empty-file
// results. Document this limitation in the REST API user docs when
// task 014 lands.
type recordingAdapterState struct {
	mu        sync.Mutex
	mode      echowarp.RecordingMode
	startedAt time.Time
	dir       string // captured on Start so Stop can enumerate files
}

// recordingModeToAudio maps the public RecordingMode tokens to the
// internal audio.RecordingMode enum. The Node wrapper has already
// validated the token, but we still return a zero-value fallback for
// safety so a forgotten case never panics at runtime.
func recordingModeToAudio(m echowarp.RecordingMode) audio.RecordingMode {
	switch m {
	case echowarp.RecordingModeTracks:
		return audio.RecordTracks
	case echowarp.RecordingModeBoth:
		return audio.RecordBoth
	default:
		// Covers echowarp.RecordingModeMix plus any unknown value
		// (the Node wrapper has already validated the input).
		return audio.RecordMix
	}
}

// enumerateRecordingFiles lists every .wav file in the recorder's
// output directory. Called after Stop so we can populate
// RecordingResult.Files without threading filename bookkeeping through
// the ConferenceRecorder. Returns an empty (non-nil) slice on any glob
// error — we prefer losing the file list over failing the whole API
// call when the session itself completed successfully.
func enumerateRecordingFiles(dir string) []string {
	if dir == "" {
		return []string{}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	if err != nil || matches == nil {
		return []string{}
	}
	sort.Strings(matches)
	return matches
}

// ---------------------------------------------------------------------------
// ServerApp implements echowarp.RecordingController for the conference
// path. Non-conference server mode currently has no recording wiring
// (task 014); in that case StartRecording returns ErrConfigValidation
// rather than pretending to record nothing. This matches the existing
// CLI --record behavior: server_multi.go only calls
// conference.StartRecording when s.cfg.Conference is true.
// ---------------------------------------------------------------------------

// StartRecording implements echowarp.RecordingController.
func (s *ServerApp) StartRecording(mode echowarp.RecordingMode) error {
	if s.conference == nil {
		return ewerrors.NewError(ewerrors.ErrConfigValidation, "Recording requires conference mode").
			WithSuggestion("Start the server with --conference to enable audio recording via the API")
	}
	if s.conference.IsRecording() {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Recording already active").
			WithSuggestion("Stop the current recording via POST /api/v1/recording/stop before starting a new one")
	}
	audioMode := recordingModeToAudio(mode)
	if err := s.conference.StartRecording(audioMode, s.cfg.SampleRate); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to start recording")
	}
	s.recState.mu.Lock()
	s.recState.mode = mode
	s.recState.startedAt = time.Now()
	s.recState.dir = s.conference.RecordingDir()
	s.recState.mu.Unlock()
	s.logger.Info("Recording started via API", "mode", mode)
	return nil
}

// StopRecording implements echowarp.RecordingController.
func (s *ServerApp) StopRecording() (echowarp.RecordingResult, error) {
	if s.conference == nil {
		return echowarp.RecordingResult{Files: []string{}}, nil
	}
	s.recState.mu.Lock()
	dir := s.recState.dir
	s.recState.mu.Unlock()

	dur, size, _, err := s.conference.StopRecording()
	if err != nil {
		return echowarp.RecordingResult{Files: []string{}}, ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to stop recording")
	}

	files := enumerateRecordingFiles(dir)

	s.recState.mu.Lock()
	s.recState.mode = ""
	s.recState.startedAt = time.Time{}
	s.recState.dir = ""
	s.recState.mu.Unlock()

	if dur == 0 && size == 0 && len(files) == 0 {
		// Idempotent "stop when not recording" — return zero-valued
		// result so the API returns 200 with an empty body.
		return echowarp.RecordingResult{Files: []string{}}, nil
	}
	s.logger.Info("Recording stopped via API",
		"duration", dur.Round(time.Second),
		"size", size,
		"files", len(files))
	return echowarp.RecordingResult{
		Duration:   dur,
		DurationMs: dur.Milliseconds(),
		Size:       int64(size), //nolint:gosec // recorder enforces 3.5GB cap per file
		Files:      files,
	}, nil
}

// RecordingStatus implements echowarp.RecordingController.
func (s *ServerApp) RecordingStatus() echowarp.RecordingStatus {
	if s.conference == nil || !s.conference.IsRecording() {
		return echowarp.RecordingStatus{}
	}
	s.recState.mu.Lock()
	mode := s.recState.mode
	startedAt := s.recState.startedAt
	s.recState.mu.Unlock()
	dur := time.Since(startedAt)
	return echowarp.RecordingStatus{
		Active:     true,
		Mode:       mode,
		Duration:   dur,
		DurationMs: dur.Milliseconds(),
		StartedAt:  startedAt,
	}
}

// ---------------------------------------------------------------------------
// ClientApp implements echowarp.RecordingController over the
// client-side recorder field. As noted in the package-level TODO, the
// client path produces empty 44B WAV files because the audio pipeline
// does not actually feed the recorder — this is a task 014 issue that
// the adapter deliberately exposes verbatim so API callers see the
// same behavior as the CLI --record flag on a client. Once task 014
// lands the same adapter code will produce real audio without change.
// ---------------------------------------------------------------------------

// StartRecording implements echowarp.RecordingController.
func (c *ClientApp) StartRecording(mode echowarp.RecordingMode) error {
	c.recorderMu.Lock()
	if c.recorder != nil && c.recorder.IsActive() {
		c.recorderMu.Unlock()
		return ewerrors.NewError(ewerrors.ErrInternalState, "Recording already active").
			WithSuggestion("Stop the current recording via POST /api/v1/recording/stop before starting a new one")
	}
	c.recorderMu.Unlock()

	audioMode := recordingModeToAudio(mode)
	// Delegate to the existing internal helper so the directory
	// resolution (Documents/EchoWarp_records/<ts>) stays in one place.
	if err := c.startRecordingInternal(audioMode); err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to start recording")
	}

	c.recState.mu.Lock()
	c.recState.mode = mode
	c.recState.startedAt = time.Now()
	c.recorderMu.Lock()
	if c.recorder != nil {
		c.recState.dir = c.recorder.Dir()
	}
	c.recorderMu.Unlock()
	c.recState.mu.Unlock()
	c.logger.Info("Recording started via API", "mode", mode)
	return nil
}

// StopRecording implements echowarp.RecordingController.
func (c *ClientApp) StopRecording() (echowarp.RecordingResult, error) {
	c.recState.mu.Lock()
	dir := c.recState.dir
	c.recState.mu.Unlock()

	dur, size, _, err := c.stopRecordingInternal()
	if err != nil {
		return echowarp.RecordingResult{Files: []string{}}, ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to stop recording")
	}

	files := enumerateRecordingFiles(dir)

	c.recState.mu.Lock()
	c.recState.mode = ""
	c.recState.startedAt = time.Time{}
	c.recState.dir = ""
	c.recState.mu.Unlock()

	if dur == 0 && size == 0 && len(files) == 0 {
		return echowarp.RecordingResult{Files: []string{}}, nil
	}
	c.logger.Info("Recording stopped via API",
		"duration", dur.Round(time.Second),
		"size", size,
		"files", len(files))
	return echowarp.RecordingResult{
		Duration:   dur,
		DurationMs: dur.Milliseconds(),
		Size:       int64(size), //nolint:gosec // recorder enforces 3.5GB cap per file
		Files:      files,
	}, nil
}

// RecordingStatus implements echowarp.RecordingController.
func (c *ClientApp) RecordingStatus() echowarp.RecordingStatus {
	c.recorderMu.Lock()
	active := c.recorder != nil && c.recorder.IsActive()
	c.recorderMu.Unlock()
	if !active {
		return echowarp.RecordingStatus{}
	}
	c.recState.mu.Lock()
	mode := c.recState.mode
	startedAt := c.recState.startedAt
	c.recState.mu.Unlock()
	dur := time.Since(startedAt)
	return echowarp.RecordingStatus{
		Active:     true,
		Mode:       mode,
		Duration:   dur,
		DurationMs: dur.Milliseconds(),
		StartedAt:  startedAt,
	}
}
