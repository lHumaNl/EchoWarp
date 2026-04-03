package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestClientHandleDCControl_ParticipantPause(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	pauseCh := make(chan ParticipantPauseMsg, 4)

	c := &ClientApp{
		cfg:                config.Config{},
		logger:             logger,
		participantPauseCh: pauseCh,
	}

	// Build a participant_pause control message.
	payload := map[string]interface{}{
		"action": transport.ActionParticipantPause,
		"data": map[string]interface{}{
			"participantID": "client-2",
			"paused":        true,
		},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := map[string]interface{}{
		"type":    transport.TypeControl,
		"payload": json.RawMessage(payloadJSON),
	}
	raw, err := json.Marshal(msg)
	require.NoError(t, err)

	stop := c.handleDCControl(raw)
	assert.False(t, stop)

	// Check that pause message was forwarded.
	select {
	case pm := <-pauseCh:
		assert.Equal(t, "client-2", pm.ParticipantID)
		assert.True(t, pm.Paused)
	default:
		t.Fatal("expected ParticipantPauseMsg on channel")
	}
}

func TestClientHandleDCControl_ParticipantsUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	partsCh := make(chan ConferenceParticipantsMsg, 4)

	c := &ClientApp{
		cfg:               config.Config{},
		logger:            logger,
		conferencePartsCh: partsCh,
	}

	// Build a participants_update control message.
	payload := map[string]interface{}{
		"action": transport.ActionParticipantsUpdate,
		"data": map[string]interface{}{
			"participants": []map[string]interface{}{
				{"id": "server", "nickname": "Server", "paused": false},
				{"id": "client-1", "nickname": "Alice", "paused": true},
				{"id": "client-2", "nickname": "Bob", "paused": false},
			},
		},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := map[string]interface{}{
		"type":    transport.TypeControl,
		"payload": json.RawMessage(payloadJSON),
	}
	raw, err := json.Marshal(msg)
	require.NoError(t, err)

	stop := c.handleDCControl(raw)
	assert.False(t, stop)

	select {
	case pm := <-partsCh:
		assert.Len(t, pm.Participants, 3)
		assert.Equal(t, "server", pm.Participants[0].ID)
		assert.Equal(t, "Server", pm.Participants[0].Nickname)
		assert.False(t, pm.Participants[0].Paused)
		assert.Equal(t, "client-1", pm.Participants[1].ID)
		assert.True(t, pm.Participants[1].Paused)
	default:
		t.Fatal("expected ConferenceParticipantsMsg on channel")
	}
}

func TestClientHandleDCControl_StopStillWorks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stoppedCh := make(chan struct{})

	c := &ClientApp{
		cfg:             config.Config{},
		logger:          logger,
		serverStoppedCh: stoppedCh,
	}

	payload := map[string]interface{}{
		"action": transport.ActionStop,
	}
	payloadJSON, _ := json.Marshal(payload)
	msg := map[string]interface{}{
		"type":    transport.TypeControl,
		"payload": json.RawMessage(payloadJSON),
	}
	raw, _ := json.Marshal(msg)

	stop := c.handleDCControl(raw)
	assert.True(t, stop)
}
