package views

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBuildMainMessage(t *testing.T) {
	tests := []struct {
		name     string
		isServer bool
		address  string
		port     int
		expected string
	}{
		{
			name:     "server mode shows waiting message",
			isServer: true,
			address:  "0.0.0.0",
			port:     8080,
			expected: "Waiting for client connection on :8080...",
		},
		{
			name:     "client mode shows connecting message",
			isServer: false,
			address:  "192.168.1.100",
			port:     4415,
			expected: "Connecting to 192.168.1.100:4415...",
		},
		{
			name:     "server mode with localhost",
			isServer: true,
			address:  "localhost",
			port:     3000,
			expected: "Waiting for client connection on :3000...",
		},
		{
			name:     "client mode with high port",
			isServer: false,
			address:  "10.0.0.1",
			port:     65535,
			expected: "Connecting to 10.0.0.1:65535...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMainMessage(tt.isServer, tt.address, tt.port)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildModeText(t *testing.T) {
	tests := []struct {
		name     string
		reverse  bool
		expected string
	}{
		{
			name:     "normal mode",
			reverse:  false,
			expected: "server → client (normal)",
		},
		{
			name:     "reverse mode",
			reverse:  true,
			expected: "client → server (reverse)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildModeText(tt.reverse)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConnectionView_ServerMode(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner:  spin,
		IsServer: true,
		Address:  "0.0.0.0",
		Port:     8080,
		Width:    80,
		Height:   24,
	})

	assert.Contains(t, result, "Waiting for client connection")
	assert.Contains(t, result, "8080")
}

func TestConnectionView_ClientMode(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner: spin,
		Address: "192.168.1.100",
		Port:    4415,
		Width:   80,
		Height:  24,
	})

	assert.Contains(t, result, "Connecting to 192.168.1.100:4415")
}

func TestConnectionView_ReverseMode(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner:  spin,
		IsServer: true,
		Port:     8080,
		Reverse:  true,
		Width:    80,
		Height:   24,
	})

	assert.Contains(t, result, "Waiting for client connection")
	assert.Contains(t, result, "reverse")
}

func TestConnectionView_SmallTerminal(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner: spin,
		Address: "localhost",
		Port:    4415,
		Width:   40,
		Height:  10,
	})

	assert.Contains(t, result, "Connecting")
	assert.Contains(t, result, "4415")
}

func TestConnectionView_Reconnect(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner:          spin,
		Address:          "192.168.1.100",
		Port:             4415,
		ReconnectAttempt: 2,
		MaxAttempts:      5,
		Width:            80,
		Height:           24,
	})

	assert.Contains(t, result, "Reconnecting")
	assert.Contains(t, result, "attempt 2/5")
}

func TestConnectionView_SessionParams(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner:    spin,
		IsServer:   true,
		Port:       4415,
		TLSEnabled: true,
		DeviceName: "Built-in Microphone",
		Width:      80,
		Height:     24,
	})

	assert.Contains(t, result, "TLS")
	assert.Contains(t, result, "Built-in Microphone")
}

func TestConnectionView_WithLogs(t *testing.T) {
	spin := spinner.New()
	result := ConnectionView(ConnectionParams{
		Spinner:  spin,
		IsServer: true,
		Port:     4415,
		Logs:     []string{"12:05:31 [INF] Starting server  port=4415"},
		Width:    80,
		Height:   24,
	})

	assert.Contains(t, result, "Starting server")
}

func TestConnectionUpdate_WithSpinnerTick(t *testing.T) {
	spin := spinner.New()
	updatedSpin, cmd := ConnectionUpdate(spin, spinner.TickMsg{})

	assert.NotNil(t, updatedSpin)
	_ = cmd
}

func TestConnectionUpdate_WithOtherMessage(t *testing.T) {
	spin := spinner.New()
	updatedSpin, cmd := ConnectionUpdate(spin, tea.KeyMsg{})

	assert.NotNil(t, updatedSpin)
	_ = cmd
}
