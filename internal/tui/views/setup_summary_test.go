package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
)

func TestSummaryOverlayView(t *testing.T) {
	cfg := config.Config{
		Mode:              config.ModeServer,
		Port:              4415,
		SampleRate:        48000,
		Channels:          1,
		MaxClients:        4,
		OpusBitrate:       64000,
		AudioBufferFrames: 5,
		Password:          "secret",
	}

	overlay := NewSummaryOverlay(cfg, "Built-in Microphone")
	view := overlay.View(100)

	assert.Contains(t, view, "Launch summary")
	assert.Contains(t, view, "Built-in Microphone")
	assert.Contains(t, view, "4415")
	assert.Contains(t, view, "max 4")
	assert.Contains(t, view, "●●●●")
}

func TestSummaryOverlayEsc(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, Port: 4415}
	overlay := NewSummaryOverlay(cfg, "Test Device")

	launch, closed, _ := overlay.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, launch)
	assert.True(t, closed)
}

func TestSummaryOverlayEnter(t *testing.T) {
	cfg := config.Config{Mode: config.ModeServer, Port: 4415}
	overlay := NewSummaryOverlay(cfg, "Test Device")

	launch, closed, _ := overlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, launch)
	assert.True(t, closed)
}

func TestBuildCLICommand(t *testing.T) {
	cfg := config.Config{
		Mode:     config.ModeServer,
		Port:     8080,
		Password: "secret",
		Reverse:  true,
	}

	cmd := BuildCLICommand(cfg, true)
	assert.Contains(t, cmd, "echowarp server")
	assert.Contains(t, cmd, "--port 8080")
	assert.Contains(t, cmd, "--password \"***\"")
	assert.Contains(t, cmd, "--reverse")

	cmdFull := BuildCLICommand(cfg, false)
	assert.Contains(t, cmdFull, "--password \"secret\"")
}

func TestBuildCLICommandClient(t *testing.T) {
	cfg := config.Config{
		Mode:    config.ModeClient,
		Port:    4415,
		Address: "192.168.1.100",
		TLS:     true,
	}

	cmd := BuildCLICommand(cfg, true)
	assert.Contains(t, cmd, "echowarp client")
	assert.Contains(t, cmd, "--address 192.168.1.100")
	assert.Contains(t, cmd, "--tls")
	assert.NotContains(t, cmd, "--port") // default port, not included
}
