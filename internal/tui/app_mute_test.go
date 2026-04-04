package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

func TestModel_ServerMuteToggle_NonConference(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Mode: config.ModeClient}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test", Channels: 2, SampleRate: 48000},
	}
	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming

	muteCh := make(chan bool, 4)
	m.serverMuteCh = muteCh

	// Initially not muted
	assert.False(t, m.serverMuted)

	// Enter toggles server mute for client in normal mode
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(keyMsg)
	m = result.(Model)

	assert.True(t, m.serverMuted, "should be muted after Enter")

	// Channel should have received true
	select {
	case val := <-muteCh:
		assert.True(t, val)
	default:
		t.Fatal("expected mute=true on channel")
	}

	// Toggle back
	result, _ = m.Update(keyMsg)
	m = result.(Model)

	assert.False(t, m.serverMuted, "should be unmuted after second Enter")

	select {
	case val := <-muteCh:
		assert.False(t, val)
	default:
		t.Fatal("expected mute=false on channel")
	}
}

func TestModel_ServerMuteToggle_SkippedInConference(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Mode: config.ModeClient}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test", Channels: 2, SampleRate: 48000},
	}
	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.conference = true

	muteCh := make(chan bool, 4)
	m.serverMuteCh = muteCh

	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(keyMsg)
	m = result.(Model)

	// In conference mode, serverMuted should NOT toggle
	assert.False(t, m.serverMuted)
	select {
	case <-muteCh:
		t.Fatal("should not send mute in conference mode")
	default:
		// Good
	}
}

// Device mute is now only accessible via popup, not standalone hotkey.

func TestModel_HelpKeys_ShowsMuteServer(t *testing.T) {
	t.Parallel()
	deviceID := uint32(1)
	cfg := config.Config{DeviceID: &deviceID, Mode: config.ModeClient}
	devices := []audio.AudioDevice{
		{ID: 1, Name: "Test", Channels: 2, SampleRate: 48000},
	}
	m := NewModel(cfg, devices)
	m.screen = ScreenStreaming
	m.serverMuteCh = make(chan bool, 1)

	help := m.helpKeys()
	assert.Contains(t, help, "enter: mute")

	m.serverMuted = true
	help = m.helpKeys()
	assert.Contains(t, help, "enter: unmute")
}
