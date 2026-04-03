package tui

// Tests for Phase E Bug 1 fixes:
//   - Adaptive log panel height in single-client view
//   - Tab-based focus cycling between areas
//   - Focus-dependent arrow key dispatch

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/internal/tui/views"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

// makeStreamingModelE creates a model already in streaming state.
func makeStreamingModelE(multiClient bool) Model {
	cfg := config.Config{Mode: config.ModeServer}
	m := NewModel(cfg, nil)
	m.screen = ScreenStreaming
	m.logsVisible = true
	m.multiClient = multiClient
	m.focusedArea = FocusClientList
	return m
}

// TestAdaptiveLogPanel_SingleClient verifies that single-client StreamingView
// uses adaptive log panel height (countRenderedLines) instead of static reservedLines.
func TestAdaptiveLogPanel_SingleClient(t *testing.T) {
	t.Parallel()

	logs := make([]string, 30)
	for i := range logs {
		logs[i] = "12:00:00 [INF] Test log line"
	}

	// With height=40, the log panel should get more lines than with height=25.
	render40 := views.StreamingView(views.StreamingParams{
		Stats:       transport.ConnectionStats{RemoteAddr: "1.2.3.4:5"},
		LogsVisible: true,
		Logs:        logs,
		Width:       80,
		Height:      40,
	})

	render25 := views.StreamingView(views.StreamingParams{
		Stats:       transport.ConnectionStats{RemoteAddr: "1.2.3.4:5"},
		LogsVisible: true,
		Logs:        logs,
		Width:       80,
		Height:      25,
	})

	lines40 := strings.Count(render40, "\n")
	lines25 := strings.Count(render25, "\n")

	// Height=40 rendering should have more lines than height=25.
	assert.Greater(t, lines40, lines25, "taller terminal should show more log lines")
}

// TestAdaptiveLogPanel_TooShort verifies that logs are not rendered when height < 20.
func TestAdaptiveLogPanel_TooShort(t *testing.T) {
	t.Parallel()

	logs := []string{"12:00:00 [INF] Test"}
	result := views.StreamingView(views.StreamingParams{
		LogsVisible: true,
		Logs:        logs,
		Width:       80,
		Height:      15, // too short
	})

	assert.NotContains(t, result, "Test", "logs should not render when height < 20")
}

// TestTabCycles_MultiClient verifies Tab cycles ClientList -> Chat -> Logs -> ClientList.
// Chat is visible by default now.
func TestTabCycles_MultiClient(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(true)

	assert.Equal(t, FocusClientList, m.focusedArea)

	// Tab: ClientList -> Chat (chat is visible by default)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusChat, m.focusedArea)

	// Tab: Chat -> Logs
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusLogs, m.focusedArea)

	// Tab: Logs -> ClientList
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusClientList, m.focusedArea)
}

// TestTabCycles_WithChatHidden verifies Tab cycles without chat when it's hidden.
func TestTabCycles_WithChatHidden(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(true)
	// Chat is visible by default, toggle to hide it.
	m.chatPanel.ToggleVisible()

	assert.Equal(t, FocusClientList, m.focusedArea)

	// Tab: ClientList -> Logs (chat hidden)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusLogs, m.focusedArea)

	// Tab: Logs -> ClientList
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusClientList, m.focusedArea)
}

// TestTabCycles_SingleClient verifies single-client cycles through available areas.
func TestTabCycles_SingleClient(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(false)

	// Server mode — starts on ClientList even for single-client
	assert.Equal(t, FocusClientList, m.focusedArea)

	// Tab: ClientList -> Chat -> Logs -> ClientList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusChat, m.focusedArea)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusLogs, m.focusedArea)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, FocusClientList, m.focusedArea)
}

// TestArrows_FocusClientList verifies Up/Down select client when focused on client list.
func TestArrows_FocusClientList(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(true)
	m.multiStats = transport.MultiClientStats{
		Clients: []transport.ClientInfo{
			{ClientID: "a"},
			{ClientID: "b"},
			{ClientID: "c"},
		},
	}
	m.selectedClient = 0
	m.focusedArea = FocusClientList

	// Down should select next client
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 1, m.selectedClient)

	// Up should go back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	assert.Equal(t, 0, m.selectedClient)
}

// TestArrows_FocusLogs verifies Up/Down scroll logs when focused on logs.
func TestArrows_FocusLogs(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(true)
	m.focusedArea = FocusLogs
	for i := 0; i < 50; i++ {
		m.logs = append(m.logs, "12:00:00 [INF] line")
	}

	// Up should scroll logs up
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	assert.Equal(t, 1, m.logScrollOffset)

	// Down should scroll back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 0, m.logScrollOffset)
}

// TestArrows_FocusLogs_DoesNotMoveClient verifies that arrows in FocusLogs
// do NOT change client selection.
func TestArrows_FocusLogs_DoesNotMoveClient(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(true)
	m.multiStats = transport.MultiClientStats{
		Clients: []transport.ClientInfo{
			{ClientID: "a"},
			{ClientID: "b"},
		},
	}
	m.selectedClient = 0
	m.focusedArea = FocusLogs
	for i := 0; i < 50; i++ {
		m.logs = append(m.logs, "12:00:00 [INF] line")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	assert.Equal(t, 0, m.selectedClient, "client selection must not change when logs are focused")
}

// TestDefaultFocus_MultiClient verifies default focus is ClientList for multi-client.
func TestDefaultFocus_MultiClient(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(false)
	m.multiClient = true
	m.setDefaultFocus()
	assert.Equal(t, FocusClientList, m.focusedArea)
}

// TestDefaultFocus_SingleClient verifies default focus is ClientList for server (even single-client).
func TestDefaultFocus_SingleClient(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(false)
	m.setDefaultFocus()
	assert.Equal(t, FocusClientList, m.focusedArea)
}

// TestCtrlT_ChatFocusCycle verifies Ctrl+T: unfocused→focus, focused→hide, hidden→show+focus.
func TestCtrlT_ChatFocusCycle(t *testing.T) {
	t.Parallel()
	m := makeStreamingModelE(false)
	m.focusedArea = FocusLogs

	// Chat is visible by default, not focused
	assert.True(t, m.chatPanel.IsVisible())

	// Ctrl+T on visible+unfocused → focus chat
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	assert.True(t, m.chatPanel.IsVisible())
	assert.Equal(t, FocusChat, m.focusedArea)

	// Ctrl+T on focused → hide chat
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	assert.False(t, m.chatPanel.IsVisible())
	assert.Equal(t, FocusClientList, m.focusedArea) // server mode

	// Ctrl+T on hidden → show + focus
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = updated.(Model)
	assert.True(t, m.chatPanel.IsVisible())
	assert.Equal(t, FocusChat, m.focusedArea)
}
