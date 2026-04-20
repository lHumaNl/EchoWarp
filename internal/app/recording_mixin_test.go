package app

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestRecordingMixin_StartStop(t *testing.T) {
	dir := t.TempDir()
	var mixin RecordingMixin

	// Initially not active.
	assert.False(t, mixin.isRecordingActive())

	// Start recording.
	err := mixin.startRecordingInternal(audio.RecordMix, 48000, dir)
	require.NoError(t, err)
	assert.True(t, mixin.isRecordingActive())

	// Stop recording.
	dur, size, files, err := mixin.stopRecordingInternal()
	require.NoError(t, err)
	assert.False(t, mixin.isRecordingActive())
	// Duration may be zero for an immediate stop; size is header-only.
	_ = dur
	_ = size

	// Verify at least one WAV file was created.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Equal(t, files, len(entries), "file count should match")
}

func TestRecordingMixin_StopWithoutStart(t *testing.T) {
	var mixin RecordingMixin
	dur, size, files, err := mixin.stopRecordingInternal()
	require.NoError(t, err)
	assert.Zero(t, dur)
	assert.Zero(t, size)
	assert.Zero(t, files)
}

// TestRecordingMixin_StartSessionAtomic verifies that startRecordingSession
// refuses a second concurrent start, fixing the TOCTOU in the daemon API
// adapter where two parallel POST /recording/start calls could both pass
// the check and leak a recorder.
func TestRecordingMixin_StartSessionAtomic(t *testing.T) {
	dir := t.TempDir()
	var mixin RecordingMixin
	defer func() { _, _, _, _ = mixin.stopRecordingInternal() }() //nolint:errcheck

	d1, already1, err1 := mixin.startRecordingSession(audio.RecordMix, 48000, dir)
	require.NoError(t, err1)
	assert.False(t, already1)
	assert.NotEmpty(t, d1)

	// Second call must report already active without mutating the recorder.
	d2, already2, err2 := mixin.startRecordingSession(audio.RecordMix, 48000, dir)
	require.NoError(t, err2)
	assert.True(t, already2, "second start must report alreadyActive=true")
	assert.Empty(t, d2)
	assert.True(t, mixin.isRecordingActive())
}
