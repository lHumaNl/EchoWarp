package views

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/app"
)

func makeMsg(action, from, text string) app.ChatMessage {
	return app.ChatMessage{
		Action: action,
		From:   from,
		Text:   text,
		TS:     time.Date(2026, 3, 27, 14, 30, 0, 0, time.UTC).UnixMilli(),
	}
}

func TestChatPanel_NewChatPanel(t *testing.T) {
	p := NewChatPanel("Alice")
	assert.False(t, p.IsVisible(), "should not be visible initially")
	assert.False(t, p.IsFocused(), "should not be focused initially")
	assert.Empty(t, p.messages, "should have no messages initially")
	assert.Equal(t, "Alice", p.myNickname)
}

func TestChatPanel_ToggleVisible(t *testing.T) {
	p := NewChatPanel("Alice")
	assert.False(t, p.IsVisible())

	p.ToggleVisible()
	assert.True(t, p.IsVisible())

	p.ToggleVisible()
	assert.False(t, p.IsVisible())
}

func TestChatPanel_ToggleVisible_UnfocusesOnHide(t *testing.T) {
	p := NewChatPanel("Alice")
	p.ToggleVisible() // show
	p.Focus()
	assert.True(t, p.IsFocused())

	p.ToggleVisible() // hide
	assert.False(t, p.IsFocused(), "hiding should unfocus")
}

func TestChatPanel_Focus(t *testing.T) {
	p := NewChatPanel("Alice")
	assert.False(t, p.IsFocused())

	p.Focus()
	assert.True(t, p.IsFocused())

	p.Unfocus()
	assert.False(t, p.IsFocused())
}

func TestChatPanel_AddMessage(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	p.ToggleVisible()

	msg := makeMsg(app.ChatActionMsg, "Bob", "Hello")
	p.AddMessage(msg)

	assert.Len(t, p.messages, 1)
	assert.Equal(t, "Bob", p.messages[0].From)

	view := p.View()
	assert.Contains(t, view, "Bob")
	assert.Contains(t, view, "Hello")
}

func TestChatPanel_AddMessage_AutoScroll(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)

	// At bottom (scroll=0), adding messages should not increment newCount
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "msg1"))
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "msg2"))
	assert.Equal(t, 0, p.newCount)
}

func TestChatPanel_Scroll(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 8)
	p.ToggleVisible()
	p.Focus()

	// Add many messages
	for i := 0; i < 20; i++ {
		p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "message"))
	}
	assert.Equal(t, 0, p.scroll)

	// Scroll up
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Greater(t, p.scroll, 0)

	// Add message while scrolled — should increment newCount
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "new"))
	assert.Equal(t, 1, p.newCount)

	// Scroll back down to bottom
	for p.scroll > 0 {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, 0, p.scroll)
	assert.Equal(t, 0, p.newCount, "newCount resets when scrolled to bottom")
}

func TestChatPanel_ViewStyling(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	p.ToggleVisible()

	// Own message
	p.AddMessage(makeMsg(app.ChatActionMsg, "Alice", "my msg"))
	// Other user
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "their msg"))
	// System message
	p.AddMessage(makeMsg(app.ChatActionSystem, "", "Bob joined"))
	// Server message
	p.AddMessage(makeMsg(app.ChatActionMsg, "Server", "welcome"))

	view := p.View()

	assert.Contains(t, view, "Alice")
	assert.Contains(t, view, "Bob")
	assert.Contains(t, view, "my msg")
	assert.Contains(t, view, "their msg")
	assert.Contains(t, view, "Bob joined")
	assert.Contains(t, view, "Server")
	assert.Contains(t, view, "welcome")
	// System messages have "—" prefix
	assert.Contains(t, view, "—")
}

func TestChatPanel_SetHistory(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	p.ToggleVisible()

	// Add some messages first
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "old"))
	assert.Len(t, p.messages, 1)

	// Replace with history
	history := []app.ChatMessage{
		makeMsg(app.ChatActionMsg, "X", "h1"),
		makeMsg(app.ChatActionMsg, "Y", "h2"),
		makeMsg(app.ChatActionMsg, "Z", "h3"),
	}
	p.SetHistory(history)

	assert.Len(t, p.messages, 3)
	assert.Equal(t, "X", p.messages[0].From)
	assert.Equal(t, 0, p.scroll, "scroll reset on history")
	assert.Equal(t, 0, p.newCount, "newCount reset on history")
}

func TestChatPanel_InputMode_EnterSends(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	p.ToggleVisible()
	p.Focus()

	// Type text
	for _, r := range "hello world" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter
	var cmd tea.Cmd
	p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd)
	msg := cmd()
	sendMsg, ok := msg.(ChatSendMsg)
	require.True(t, ok, "expected ChatSendMsg, got %T", msg)
	assert.Equal(t, "hello world", sendMsg.Text)

	// Input should be cleared
	assert.Empty(t, p.input.Value())
}

func TestChatPanel_InputMode_EscUnfocuses(t *testing.T) {
	p := NewChatPanel("Alice")
	p.ToggleVisible()
	p.Focus()
	assert.True(t, p.IsFocused())

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, p.IsFocused())
}

func TestChatPanel_InputMode_EmptyEnterNoSend(t *testing.T) {
	p := NewChatPanel("Alice")
	p.ToggleVisible()
	p.Focus()

	// Press Enter with empty input
	var cmd tea.Cmd
	p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "empty input should not produce a command")
}

func TestChatPanel_NotFocused_NoKeyHandling(t *testing.T) {
	p := NewChatPanel("Alice")
	p.ToggleVisible()
	// Not focused

	var cmd tea.Cmd
	p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "unfocused panel should not handle keys")
}

func TestChatPanel_ViewNotVisible(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	// Not visible
	assert.Empty(t, p.View())
}

func TestChatPanel_ViewTooSmall(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 2) // too small
	p.ToggleVisible()
	assert.Empty(t, p.View())
}

func TestChatPanel_NewIndicator(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 8)
	p.ToggleVisible()
	p.Focus()

	// Add enough messages to have scrollback
	for i := 0; i < 15; i++ {
		p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "msg"))
	}

	// Scroll up
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Add new messages while scrolled
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "new1"))
	p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "new2"))

	view := p.View()
	assert.Contains(t, view, "2 new messages")
}

func TestChatPanel_MaxDisplayCap(t *testing.T) {
	p := NewChatPanel("Alice")
	for i := 0; i < maxChatDisplay+50; i++ {
		p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "msg"))
	}
	assert.Len(t, p.messages, maxChatDisplay)
}

func TestChatPanel_SetHistory_MaxCap(t *testing.T) {
	p := NewChatPanel("Alice")
	msgs := make([]app.ChatMessage, maxChatDisplay+50)
	for i := range msgs {
		msgs[i] = makeMsg(app.ChatActionMsg, "Bob", "msg")
	}
	p.SetHistory(msgs)
	assert.Len(t, p.messages, maxChatDisplay)
}

func TestChatPanel_ViewContainsTitle(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 10)
	p.ToggleVisible()

	view := p.View()
	assert.True(t, strings.Contains(view, "Chat"), "view should contain title")
	assert.True(t, strings.Contains(view, "[Ctrl+T]"), "view should contain toggle hint")
}

func TestChatPanel_SendResetsScroll(t *testing.T) {
	p := NewChatPanel("Alice")
	p.SetSize(80, 8)
	p.ToggleVisible()
	p.Focus()

	for i := 0; i < 15; i++ {
		p.AddMessage(makeMsg(app.ChatActionMsg, "Bob", "msg"))
	}

	// Scroll up
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Greater(t, p.scroll, 0)

	// Type and send
	for _, r := range "reply" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 0, p.scroll, "sending should reset scroll to bottom")
}
