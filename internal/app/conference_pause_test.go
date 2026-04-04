package app

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConferenceHandler_SetParticipantPaused(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ch := NewConferenceHandler(960, 48000, false, logger)
	ch.AddParticipant("client-1")
	ch.AddParticipant("client-2")

	// Initially no one is paused.
	paused := ch.GetPausedParticipants()
	assert.Empty(t, paused)

	// Pause client-1.
	ch.SetParticipantPaused("client-1", true)
	paused = ch.GetPausedParticipants()
	assert.True(t, paused["client-1"])
	assert.False(t, paused["client-2"])

	// Resume client-1.
	ch.SetParticipantPaused("client-1", false)
	paused = ch.GetPausedParticipants()
	assert.Empty(t, paused)
}

func TestConferenceHandler_RemoveParticipant_ClearsPause(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ch := NewConferenceHandler(960, 48000, false, logger)
	ch.AddParticipant("client-1")

	ch.SetParticipantPaused("client-1", true)
	assert.True(t, ch.GetPausedParticipants()["client-1"])

	ch.RemoveParticipant("client-1")
	paused := ch.GetPausedParticipants()
	assert.Empty(t, paused)
}

func TestConferenceHandler_ParticipantIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ch := NewConferenceHandler(960, 48000, false, logger)
	ch.AddParticipant("server")
	ch.AddParticipant("client-1")

	ids := ch.ParticipantIDs()
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, "server")
	assert.Contains(t, ids, "client-1")
}

func TestConferenceHandler_GetPausedParticipants_ReturnsSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	ch := NewConferenceHandler(960, 48000, false, logger)
	ch.AddParticipant("client-1")
	ch.SetParticipantPaused("client-1", true)

	snapshot := ch.GetPausedParticipants()
	// Mutating snapshot should not affect handler.
	snapshot["client-1"] = false
	assert.True(t, ch.GetPausedParticipants()["client-1"])
}
