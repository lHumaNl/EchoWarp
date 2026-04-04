// Package views provides the chat panel component for the streaming screen.
package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lHumaNl/echowarp/internal/app"
	"github.com/lHumaNl/echowarp/internal/tui/styles"
)

// maxChatDisplay is the maximum number of messages kept in the display buffer.
const maxChatDisplay = 200

// ChatSendMsg is returned when the user presses Enter in the chat input.
type ChatSendMsg struct {
	Text string
	To   string // DM recipient ("" = broadcast)
}

// ChatPanel is a self-contained chat UI component for the streaming screen.
type ChatPanel struct {
	messages     []app.ChatMessage
	input        textinput.Model
	visible      bool
	focused      bool
	scroll       int // offset from bottom (0 = at bottom)
	newCount     int // unread messages when scrolled up
	width        int
	height       int
	myNickname   string
	participants []string // online nicknames (for @-autocomplete)
	lastDMFrom   string   // last DM sender (for @@ reply)
}

// MyNickname returns the chat panel's nickname.
func (c *ChatPanel) MyNickname() string { return c.myNickname }

// SetMyNickname updates the chat panel's nickname (e.g. after server assigns one).
func (c *ChatPanel) SetMyNickname(nick string) { c.myNickname = nick }

// SetParticipants updates the list of online participants (for @-autocomplete).
func (c *ChatPanel) SetParticipants(p []string) { c.participants = p }

// InputValue returns the current text in the input field.
func (c *ChatPanel) InputValue() string { return c.input.Value() }

// NewChatPanel creates a new chat panel with the given nickname for styling own messages.
func NewChatPanel(myNickname string) ChatPanel {
	ti := textinput.New()
	ti.Placeholder = "press Ctrl+T to chat"
	ti.CharLimit = app.MaxChatMessageLen
	ti.Width = 60

	return ChatPanel{
		input:      ti,
		myNickname: myNickname,
	}
}

// SetSize updates the panel dimensions.
func (c *ChatPanel) SetSize(width, height int) {
	c.width = width
	c.height = height
	if width > 4 {
		c.input.Width = width - 4 // account for "▸ " prefix + padding
	}
}

// AddMessage appends a message. Auto-scrolls if at bottom, increments newCount otherwise.
func (c *ChatPanel) AddMessage(msg app.ChatMessage) {
	c.messages = append(c.messages, msg)
	if len(c.messages) > maxChatDisplay {
		c.messages = c.messages[len(c.messages)-maxChatDisplay:]
	}
	if c.scroll != 0 {
		c.newCount++
	}
	// Track last DM sender for @@ reply.
	if msg.IsDM() && msg.From != c.myNickname {
		c.lastDMFrom = msg.From
	}
}

// SetHistory replaces all messages (used when chat_history is received).
func (c *ChatPanel) SetHistory(msgs []app.ChatMessage) {
	c.messages = msgs
	if len(c.messages) > maxChatDisplay {
		c.messages = c.messages[len(c.messages)-maxChatDisplay:]
	}
	c.scroll = 0
	c.newCount = 0
}

// ToggleVisible shows or hides the chat panel.
func (c *ChatPanel) ToggleVisible() {
	c.visible = !c.visible
	if !c.visible {
		c.focused = false
		c.input.Blur()
	}
}

// Focus activates the text input.
func (c *ChatPanel) Focus() {
	c.focused = true
	c.input.Focus()
	c.input.Placeholder = ""
}

// Unfocus deactivates the text input.
func (c *ChatPanel) Unfocus() {
	c.focused = false
	c.input.Blur()
	c.input.Placeholder = "press Ctrl+T to chat"
}

// IsFocused returns whether the input is focused.
func (c *ChatPanel) IsFocused() bool {
	return c.focused
}

// IsVisible returns whether the panel is visible.
func (c *ChatPanel) IsVisible() bool {
	return c.visible
}

// Update handles key events when the panel is focused.
func (c *ChatPanel) Update(msg tea.Msg) (ChatPanel, tea.Cmd) {
	if !c.focused {
		return *c, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Forward non-key messages to textinput (e.g., blink)
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return *c, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		text := strings.TrimSpace(c.input.Value())
		if text == "" {
			return *c, nil
		}
		to, body, errMsg := c.parseDMPrefix(text)
		if errMsg != "" {
			// Show error as local system message.
			c.AddMessage(app.ChatMessage{
				Action: app.ChatActionSystem,
				Text:   errMsg,
				TS:     time.Now().UnixMilli(),
			})
			// Don't clear input so user can fix the nickname.
			return *c, nil
		}
		c.input.SetValue("")
		c.scroll = 0
		c.newCount = 0
		return *c, func() tea.Msg {
			return ChatSendMsg{Text: body, To: to}
		}

	case tea.KeyTab:
		val := c.input.Value()
		if completed, ok := c.tabComplete(val); ok {
			c.input.SetValue(completed)
			c.input.SetCursor(len(completed))
		}
		return *c, nil

	case tea.KeyEsc:
		c.Unfocus()
		return *c, nil

	case tea.KeyUp:
		maxScroll := len(c.messages) - c.messageAreaHeight()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if c.scroll < maxScroll {
			c.scroll++
		}
		return *c, nil

	case tea.KeyDown:
		if c.scroll > 0 {
			c.scroll--
			if c.scroll == 0 {
				c.newCount = 0
			}
		}
		return *c, nil
	}

	// Forward all other keys to textinput
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return *c, cmd
}

// messageAreaHeight returns how many lines are available for messages.
func (c *ChatPanel) messageAreaHeight() int {
	// height minus title(1), input(1), possible new-indicator(1)
	h := c.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the chat panel.
func (c *ChatPanel) View() string {
	if !c.visible || c.height < 3 {
		return ""
	}

	var sections []string

	// Title bar
	title := styles.ChatTitle.Render("Chat")
	hint := styles.ChatInputInactive.Render("[Ctrl+T]")
	spacing := c.width - len("Chat") - len("[Ctrl+T]")
	if spacing < 2 {
		spacing = 2
	}
	sections = append(sections, title+strings.Repeat(" ", spacing)+hint)

	// Message area
	msgHeight := c.messageAreaHeight()
	// Account for new-message indicator stealing a line
	if c.scroll > 0 && c.newCount > 0 {
		msgHeight--
	}
	if msgHeight < 0 {
		msgHeight = 0
	}

	total := len(c.messages)
	end := total - c.scroll
	if end < 0 {
		end = 0
	}
	if end > total {
		end = total
	}
	start := end - msgHeight
	if start < 0 {
		start = 0
	}

	visible := c.messages[start:end]
	for _, msg := range visible {
		sections = append(sections, c.renderMessage(msg))
	}

	// Pad empty lines if fewer messages than available height
	rendered := len(visible)
	for rendered < msgHeight {
		sections = append(sections, "")
		rendered++
	}

	// New message indicator
	if c.scroll > 0 && c.newCount > 0 {
		sections = append(sections, styles.ChatNewIndicator.Render(fmt.Sprintf("↓ %d new messages", c.newCount)))
	}

	// Input line
	prefix := styles.ChatInputActive.Render("▸ ")
	if !c.focused {
		prefix = styles.ChatInputInactive.Render("▸ ")
	}
	sections = append(sections, prefix+c.input.View())

	return strings.Join(sections, "\n")
}

// parseDMPrefix extracts recipient and body from @-syntax input.
// "@Nick text" → ("Nick", "text"); "@@  text" → (lastDMFrom, "text"); otherwise ("", text).
// Returns errMsg non-empty if the recipient is invalid.
func (c *ChatPanel) parseDMPrefix(text string) (to, body, errMsg string) {
	if strings.HasPrefix(text, "@@") {
		body = strings.TrimSpace(strings.TrimPrefix(text, "@@"))
		if c.lastDMFrom == "" {
			return "", text, "No recent DM to reply to"
		}
		if body == "" {
			return "", text, ""
		}
		return c.lastDMFrom, body, ""
	}
	if strings.HasPrefix(text, "@") {
		rest := text[1:]
		// Handle quoted nickname: @"Some Name" message
		if strings.HasPrefix(rest, "\"") {
			end := strings.Index(rest[1:], "\"")
			if end >= 0 {
				to = rest[1 : end+1]
				body = strings.TrimSpace(rest[end+2:])
				if body == "" {
					return "", text, ""
				}
				if !c.isValidRecipient(to) {
					return "", text, fmt.Sprintf("User %q not found", to)
				}
				return to, body, ""
			}
		}
		// Unquoted: @Nick message (first space separates nick from body)
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			to = parts[0]
			if !c.isValidRecipient(to) {
				return "", text, fmt.Sprintf("User %q not found", to)
			}
			return to, strings.TrimSpace(parts[1]), ""
		}
	}
	return "", text, ""
}

// isValidRecipient checks whether the given nickname exists in the participants list
// or is "Server" (always a valid DM target).
func (c *ChatPanel) isValidRecipient(nick string) bool {
	if strings.EqualFold(nick, "Server") {
		return true
	}
	for _, p := range c.participants {
		if strings.EqualFold(p, nick) {
			return true
		}
	}
	return false
}

// tabComplete attempts tab-completion for @-mentions.
// With one match: completes fully + trailing space. With multiple: completes to longest common prefix.
func (c *ChatPanel) tabComplete(val string) (string, bool) {
	if !strings.HasPrefix(val, "@") || strings.HasPrefix(val, "@@") {
		return "", false
	}
	prefix := val[1:]
	// Don't complete if there's already a space (user is typing the message body).
	if strings.Contains(prefix, " ") {
		return "", false
	}

	prefixLower := strings.ToLower(prefix)
	var matches []string
	for _, p := range c.participants {
		if p == c.myNickname {
			continue
		}
		if strings.HasPrefix(strings.ToLower(p), prefixLower) {
			matches = append(matches, p)
		}
	}

	switch len(matches) {
	case 0:
		return "", false
	case 1:
		if strings.Contains(matches[0], " ") {
			return "@\"" + matches[0] + "\" ", true
		}
		return "@" + matches[0] + " ", true
	default:
		// Multiple matches — complete to longest common prefix.
		lcp := matches[0]
		for _, m := range matches[1:] {
			lcp = commonPrefix(lcp, m)
		}
		if len(lcp) <= len(prefix) {
			return "", false // no progress
		}
		return "@" + lcp, true
	}
}

// commonPrefix returns the longest common prefix of two strings (case-insensitive comparison,
// but preserves the case of the first string).
func commonPrefix(a, b string) string {
	al, bl := strings.ToLower(a), strings.ToLower(b)
	n := len(al)
	if len(bl) < n {
		n = len(bl)
	}
	for i := 0; i < n; i++ {
		if al[i] != bl[i] {
			return a[:i]
		}
	}
	return a[:n]
}

// renderDM formats a direct message with DM-specific styling.
func (c *ChatPanel) renderDM(timestamp string, msg app.ChatMessage) string {
	dmTag := styles.ChatDMTag.Render("DM")
	var direction string
	if msg.From == c.myNickname {
		direction = styles.ChatNickSelf.Render("you") + " → " + styles.ChatDMNick.Render(msg.To)
	} else {
		direction = styles.ChatDMNick.Render(msg.From) + " → " + styles.ChatNickSelf.Render("you")
	}
	return timestamp + " " + dmTag + " " + direction + ": " + styles.ChatDMText.Render(msg.Text)
}

// renderMessage formats a single chat message with appropriate styling.
func (c *ChatPanel) renderMessage(msg app.ChatMessage) string {
	ts := time.UnixMilli(msg.TS).Format("15:04")
	timestamp := styles.ChatTimestamp.Render("[" + ts + "]")

	switch msg.Action {
	case app.ChatActionSystem:
		return timestamp + " " + styles.ChatSystem.Render("— "+msg.Text)

	case app.ChatActionMsg:
		if msg.IsDM() {
			return c.renderDM(timestamp, msg)
		}
		var nick string
		switch {
		case msg.From == c.myNickname:
			nick = styles.ChatNickSelf.Render(msg.From + " (me)")
		case strings.EqualFold(msg.From, "Server"):
			nick = styles.ChatNickServer.Render(msg.From)
		default:
			nick = styles.ChatNickOther.Render(msg.From)
		}
		return timestamp + " " + nick + ": " + styles.ChatText.Render(msg.Text)

	default:
		// Fallback for unknown actions
		if msg.From != "" {
			return timestamp + " " + styles.ChatNickOther.Render(msg.From) + ": " + styles.ChatText.Render(msg.Text)
		}
		return timestamp + " " + styles.ChatText.Render(msg.Text)
	}
}
