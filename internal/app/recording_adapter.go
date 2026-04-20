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
// ServerApp implements echowarp.RecordingController. In conference mode,
// recording is delegated to ConferenceHandler. In non-conference mode,
// the server uses a local recorder fed by the capture pipeline tap.
// ---------------------------------------------------------------------------

// StartRecording implements echowarp.RecordingController.
func (s *ServerApp) StartRecording(mode echowarp.RecordingMode) error {
	audioMode := recordingModeToAudio(mode)

	if s.conference != nil {
		// Conference mode: delegate to ConferenceHandler.
		if s.conference.IsRecording() {
			return ewerrors.NewError(ewerrors.ErrInternalState, "Recording already active").
				WithSuggestion("Stop the current recording via POST /api/v1/recording/stop before starting a new one")
		}
		if err := s.conference.StartRecording(audioMode, s.cfg.SampleRate, s.cfg.EffectiveRecordDir()); err != nil {
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

	// Non-conference mode: use server-local recorder. startRecordingSession
	// performs the check-and-start atomically under the mixin lock so two
	// concurrent API calls cannot both create recorders.
	dir, alreadyActive, err := s.startRecordingSession(audioMode, s.cfg.SampleRate, s.cfg.EffectiveRecordDir())
	if alreadyActive {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Recording already active").
			WithSuggestion("Stop the current recording via POST /api/v1/recording/stop before starting a new one")
	}
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to start recording")
	}
	s.recState.mu.Lock()
	s.recState.mode = mode
	s.recState.startedAt = time.Now()
	s.recState.dir = dir
	s.recState.mu.Unlock()
	s.logger.Info("Recording started via API", "mode", mode)
	return nil
}

// StopRecording implements echowarp.RecordingController.
func (s *ServerApp) StopRecording() (echowarp.RecordingResult, error) {
	s.recState.mu.Lock()
	dir := s.recState.dir
	s.recState.mu.Unlock()

	var dur time.Duration
	var size uint64
	var err error

	if s.conference != nil {
		dur, size, _, err = s.conference.StopRecording()
	} else {
		dur, size, _, err = s.stopRecordingInternal()
	}
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
	active := false
	if s.conference != nil {
		active = s.conference.IsRecording()
	} else {
		active = s.isRecordingActive()
	}
	if !active {
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
// client-side recorder field. The jitter playback pump feeds decoded
// audio to the recorder via a recording tap closure.
// ---------------------------------------------------------------------------

// StartRecording implements echowarp.RecordingController.
func (c *ClientApp) StartRecording(mode echowarp.RecordingMode) error {
	audioMode := recordingModeToAudio(mode)
	// Atomic check-and-start via the mixin — fixes the previous TOCTOU where
	// the recorderMu was released between IsActive() and startRecordingInternal
	// so two concurrent API calls could both pass the guard and leak a recorder.
	dir, alreadyActive, err := c.startRecordingSession(audioMode, c.cfg.SampleRate, c.cfg.EffectiveRecordDir())
	if alreadyActive {
		return ewerrors.NewError(ewerrors.ErrInternalState, "Recording already active").
			WithSuggestion("Stop the current recording via POST /api/v1/recording/stop before starting a new one")
	}
	if err != nil {
		return ewerrors.Wrap(err, ewerrors.ErrInternalState, "Failed to start recording")
	}
	c.recState.mu.Lock()
	c.recState.mode = mode
	c.recState.startedAt = time.Now()
	c.recState.dir = dir
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
	if !c.isRecordingActive() {
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
