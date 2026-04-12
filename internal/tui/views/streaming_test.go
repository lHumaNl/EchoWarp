package views

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/tui/styles"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{"zero", 0, "00:00"},
		{"65 seconds", 65 * time.Second, "01:05"},
		{"1 hour 1 minute 1 second", 3661 * time.Second, "1:01:01"},
		{"30 seconds", 30 * time.Second, "00:30"},
		{"1 minute", 60 * time.Second, "01:00"},
		{"2 hours 30 minutes 45 seconds", (2*3600 + 30*60 + 45) * time.Second, "2:30:45"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"999 bytes", 999, "999 B"},
		{"1 KB", 1000, "1.0 KB"},
		{"1.5 KB", 1500, "1.5 KB"},
		{"1 MB", 1000000, "1.0 MB"},
		{"1 GB", 1000000000, "1.0 GB"},
		{"2.5 MB", 2500000, "2.5 MB"},
		{"500 bytes", 500, "500 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStreamingView_ContainsExpectedSections(t *testing.T) {
	stats := transport.ConnectionStats{
		State:       "connected",
		LocalAddr:   "192.168.1.1:1234",
		RemoteAddr:  "192.168.1.2:5678",
		BytesSent:   1024000,
		BytesRecv:   2048000,
		PacketsLost: 2,
		Jitter:      5.5,
		RoundTrip:   25.3,
	}

	startTime := time.Now().Add(-65 * time.Second)
	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   startTime,
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "server")
	assert.Contains(t, result, "client")
	// "quit" and "pause" are now in the status bar (rendered by layout), not in the streaming body
}

func TestStreamingView_PausedState(t *testing.T) {
	stats := transport.ConnectionStats{
		State:      "connected",
		LocalAddr:  "192.168.1.1:1234",
		RemoteAddr: "192.168.1.2:5678",
	}

	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   time.Now(),
		Paused:      true,
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "PAUSED")
}

func TestStreamingView_ReverseDirection(t *testing.T) {
	stats := transport.ConnectionStats{
		State:      "connected",
		LocalAddr:  "192.168.1.1:1234",
		RemoteAddr: "192.168.1.2:5678",
	}

	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   time.Now(),
		Reverse:     true,
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "reverse")
	assert.Contains(t, result, "client")
	assert.Contains(t, result, "server")
}

func TestStreamingView_StatsPanel(t *testing.T) {
	stats := transport.ConnectionStats{
		State:       "connected",
		LocalAddr:   "192.168.1.1:1234",
		RemoteAddr:  "192.168.1.2:5678",
		BytesSent:   1048576,
		BytesRecv:   2097152,
		PacketsLost: 5,
		Jitter:      10.0,
		RoundTrip:   50.0,
	}

	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   time.Now().Add(-10 * time.Second),
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "1.0 MB") // 1048576 bytes = 1.0 MB (SI)
	assert.Contains(t, result, "2.1 MB") // 2097152 bytes = 2.1 MB (SI)
	assert.Contains(t, result, "Jitter")
	assert.Contains(t, result, "RTT")
}

func TestStreamingView_RemoteAddr(t *testing.T) {
	stats := transport.ConnectionStats{
		State:      "connected",
		RemoteAddr: "192.168.1.5:51234",
	}

	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   time.Now(),
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "192.168.1.5:51234")
}

func TestStreamingView_DeviceName(t *testing.T) {
	stats := transport.ConnectionStats{State: "connected"}

	result := StreamingView(StreamingParams{
		Stats:       stats,
		StartTime:   time.Now(),
		Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible: true,
		DeviceName:  "Built-in Microphone",
		Width:       80,
		Height:      24,
	})

	assert.Contains(t, result, "Built-in Microphone")
}

func TestStreamingView_Sparklines(t *testing.T) {
	stats := transport.ConnectionStats{State: "connected", Jitter: 5.0, RoundTrip: 20.0}

	jitterHist := []float64{1, 2, 3, 4, 5, 4, 3, 2, 1, 5}
	rttHist := []float64{10, 15, 20, 25, 20, 15, 10, 20, 25, 20}

	result := StreamingView(StreamingParams{
		Stats:         stats,
		StartTime:     time.Now(),
		Audio:         AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		LogsVisible:   true,
		JitterHistory: jitterHist,
		RTTHistory:    rttHist,
		Width:         80,
		Height:        24,
	})

	// Sparklines should contain block characters
	assert.Contains(t, result, "▁")
}

func TestStreamingView_LogScrollOffset(t *testing.T) {
	logs := make([]string, 20)
	for i := range logs {
		logs[i] = "12:00:00 [INF] Log line"
	}

	result := StreamingView(StreamingParams{
		Stats:           transport.ConnectionStats{State: "connected"},
		StartTime:       time.Now(),
		Audio:           AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		Logs:            logs,
		LogsVisible:     true,
		LogScrollOffset: 5,
		Width:           80,
		Height:          30,
	})

	assert.Contains(t, result, "SCROLLED")
}

func TestCalculateBitrate(t *testing.T) {
	tests := []struct {
		name       string
		bytesSent  uint64
		bytesRecv  uint64
		elapsed    time.Duration
		containsIn string
	}{
		{"zero elapsed", 1000, 1000, 0, "0.0 kbps"},
		{"1 second 2000 bytes", 1000, 1000, time.Second, "kbps"},
		{"high bitrate", 1250000, 1250000, time.Second, "Mbps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBitrate(tt.bytesSent, tt.bytesRecv, tt.elapsed)
			assert.Contains(t, result, tt.containsIn)
		})
	}
}

func TestGetValueStyle(t *testing.T) {
	goodStyle := styles.StatValueGood
	warnStyle := styles.StatValueWarn
	errorStyle := styles.StatValueError

	tests := []struct {
		name           string
		value          float64
		warnThreshold  float64
		errorThreshold float64
		expectedStyle  lipgloss.Style
	}{
		{"below warn - good", 10, 30, 100, goodStyle},
		{"at warn threshold", 30, 30, 100, warnStyle},
		{"between thresholds", 50, 30, 100, warnStyle},
		{"at error threshold", 100, 30, 100, errorStyle},
		{"above error", 150, 30, 100, errorStyle},
		{"zero - good", 0, 30, 100, goodStyle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueStyle(tt.value, tt.warnThreshold, tt.errorThreshold, goodStyle, warnStyle, errorStyle)
			assert.NotNil(t, result)
		})
	}
}

func TestGetPacketLossStyle(t *testing.T) {
	goodStyle := styles.StatValueGood
	warnStyle := styles.StatValueWarn
	errorStyle := styles.StatValueError

	tests := []struct {
		name        string
		packetsLost uint32
	}{
		{"no loss", 0},
		{"minor loss", 1},
		{"warn threshold", 3},
		{"between", 5},
		{"error threshold", 10},
		{"high loss", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPacketLossStyle(tt.packetsLost, goodStyle, warnStyle, errorStyle)
			assert.NotNil(t, result)
		})
	}
}

func TestStreamingView_AllStates(t *testing.T) {
	states := []string{"connected", "disconnected", "connecting", "failed", "new", "unknown"}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			result := StreamingView(StreamingParams{
				Stats:       transport.ConnectionStats{State: state},
				StartTime:   time.Now(),
				Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
				LogsVisible: true,
				Width:       80,
				Height:      24,
			})
			assert.NotEmpty(t, result)
		})
	}
}

func TestStreamingView_EdgeCases(t *testing.T) {
	t.Run("very small terminal", func(t *testing.T) {
		result := StreamingView(StreamingParams{
			Stats:       transport.ConnectionStats{State: "connected", BytesSent: 1024, BytesRecv: 2048},
			StartTime:   time.Now(),
			Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
			LogsVisible: true,
			Width:       20,
			Height:      10,
		})
		assert.NotEmpty(t, result)
	})

	t.Run("zero duration", func(t *testing.T) {
		result := StreamingView(StreamingParams{
			Stats:       transport.ConnectionStats{State: "connected", BytesSent: 1024, BytesRecv: 2048},
			StartTime:   time.Now(),
			Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
			LogsVisible: true,
			Width:       80,
			Height:      24,
		})
		assert.NotEmpty(t, result)
	})

	t.Run("large data transfer", func(t *testing.T) {
		result := StreamingView(StreamingParams{
			Stats:       transport.ConnectionStats{State: "connected", BytesSent: 1073741824, BytesRecv: 2147483648},
			StartTime:   time.Now().Add(-time.Hour),
			Audio:       AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
			LogsVisible: true,
			Width:       80,
			Height:      24,
		})
		assert.Contains(t, result, "GB")
	})
}

func TestClientStreamingNoDevicePanel(t *testing.T) {
	devices := []DeviceDisplayState{
		{ID: 1, Name: "Mic", Role: "capture", Volume: 1.0},
	}

	// Server mode — device panel should appear.
	serverResult := StreamingView(StreamingParams{
		Stats:     transport.ConnectionStats{State: "connected"},
		StartTime: time.Now(),
		Audio:     AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		Width:     80,
		Height:    24,
		Devices:   devices,
		IsServer:  true,
	})
	assert.Contains(t, serverResult, "Devices", "server mode should show device panel")

	// Client mode — device panel should be hidden.
	clientResult := StreamingView(StreamingParams{
		Stats:     transport.ConnectionStats{State: "connected"},
		StartTime: time.Now(),
		Audio:     AudioInfo{Codec: "Opus", SampleRate: 48000, Channels: 1},
		Width:     80,
		Height:    24,
		Devices:   devices,
		IsServer:  false,
	})
	assert.NotContains(t, clientResult, "Devices", "client mode should hide device panel")
}

func TestCalculateBitrate_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		bytesSent uint64
		bytesRecv uint64
		elapsed   time.Duration
		contains  string
	}{
		{"zero elapsed time", 1000, 1000, 0, "0.0 kbps"},
		{"exactly 1 Mbps", 62500, 62500, time.Second, "Mbps"},
		{"just under 1 Mbps", 62000, 62000, time.Second, "kbps"},
		{"very high bitrate", 12500000, 12500000, time.Second, "Mbps"},
		{"very low bitrate", 100, 100, time.Second, "kbps"},
		{"one byte", 1, 0, time.Second, "kbps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBitrate(tt.bytesSent, tt.bytesRecv, tt.elapsed)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestFormatBytes_AllSizes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"999 bytes", 999, "999 B"},
		{"1 KB", 1000, "1.0 KB"},
		{"1.5 KB", 1500, "1.5 KB"},
		{"1 MB", 1000000, "1.0 MB"},
		{"100 MB", 100000000, "100.0 MB"},
		{"1 GB", 1000000000, "1.0 GB"},
		{"1 TB", 1000000000000, "1.0 TB"},
		{"1 PB", 1000000000000000, "1.0 PB"},
		{"1 EB", 1000000000000000000, "1.0 EB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration_AllFormats(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"zero", 0, "00:00"},
		{"1 second", time.Second, "00:01"},
		{"30 seconds", 30 * time.Second, "00:30"},
		{"1 minute", time.Minute, "01:00"},
		{"1 minute 30 seconds", 90 * time.Second, "01:30"},
		{"1 hour", time.Hour, "1:00:00"},
		{"1 hour 1 minute 1 second", 3661 * time.Second, "1:01:01"},
		{"23 hours 59 minutes 59 seconds", 86399 * time.Second, "23:59:59"},
		{"100 hours", 100 * time.Hour, "100:00:00"},
		{"1.5 seconds rounds to 2", 1500 * time.Millisecond, "00:02"},
		{"400ms rounds to 0", 400 * time.Millisecond, "00:00"},
		{"600ms rounds to 1", 600 * time.Millisecond, "00:01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderSparkline(t *testing.T) {
	tests := []struct {
		name   string
		data   []float64
		width  int
		expect string
	}{
		{"empty data", nil, 8, ""},
		{"zero width", []float64{1, 2, 3}, 0, ""},
		{"single value", []float64{5}, 8, "▁▁▁▁▁▁▁▁"},
		{"all same", []float64{3, 3, 3, 3}, 8, "▁▁▁▁▁▁▁▁"},
		{"ascending", []float64{0, 1, 2, 3, 4, 5, 6, 7}, 8, "▁▂▃▄▅▆▇█"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderSparkline(tt.data, tt.width)
			assert.Equal(t, tt.expect, result)
		})
	}
}
