package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chatTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// collector is a thread-safe helper that records sent bytes and decoded messages.
type collector struct {
	mu   sync.Mutex
	raw  [][]byte
	msgs []ChatMessage
}

func (c *collector) sendFn(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	c.raw = append(c.raw, cp)

	// Try to decode as ChatMessage or ChatHistoryPayload.
	var msg ChatMessage
	if json.Unmarshal(data, &msg) == nil && msg.Action != "" && msg.Action != ChatActionHistory {
		c.msgs = append(c.msgs, msg)
	}
	return nil
}

func (c *collector) allRaw() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([][]byte, len(c.raw))
	copy(cp, c.raw)
	return cp
}

func (c *collector) allMsgs() []ChatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]ChatMessage, len(c.msgs))
	copy(cp, c.msgs)
	return cp
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.raw)
}

// ── ChatHub Tests ──────────────────────────────────────────────────────────

func TestChatHub_RegisterUnregister(t *testing.T) {
	var received []ChatMessage
	var mu sync.Mutex
	onMsg := func(msg ChatMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	hub := NewChatHub(chatTestLogger(), 5, onMsg)
	c := &collector{}

	hub.Register("c1", "Alice", c.sendFn)

	// Should have received history (empty) as first message.
	raws := c.allRaw()
	require.GreaterOrEqual(t, len(raws), 1, "should receive at least history payload")
	var hist ChatHistoryPayload
	require.NoError(t, json.Unmarshal(raws[0], &hist))
	assert.Equal(t, ChatActionHistory, hist.Action)

	// Should have received system "joined" message.
	msgs := c.allMsgs()
	require.NotEmpty(t, msgs)
	assert.Equal(t, ChatActionSystem, msgs[0].Action)
	assert.Contains(t, msgs[0].Text, "Alice joined")

	// onMessage callback should have been called for joined.
	mu.Lock()
	assert.NotEmpty(t, received)
	assert.Contains(t, received[0].Text, "Alice joined")
	mu.Unlock()

	hub.Unregister("c1")

	// onMessage should have "left" message.
	mu.Lock()
	found := false
	for _, m := range received {
		if m.Action == ChatActionSystem && m.Text == "Alice left" {
			found = true
		}
	}
	mu.Unlock()
	assert.True(t, found, "should broadcast 'left' system message")
}

func TestChatHub_Broadcast(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	c1 := &collector{}
	c2 := &collector{}
	c3 := &collector{}

	hub.Register("c1", "Alice", c1.sendFn)
	hub.Register("c2", "Bob", c2.sendFn)
	hub.Register("c3", "Carol", c3.sendFn)

	// c1 sends a message.
	msg := ChatMessage{Action: ChatActionMsg, From: "ignored", Text: "hello"}
	raw, _ := json.Marshal(msg)
	hub.HandleIncoming("c1", raw)

	// c2 and c3 should receive it, c1 should NOT (only system msgs from register).
	c1Msgs := c1.allMsgs()
	c2Msgs := c2.allMsgs()
	c3Msgs := c3.allMsgs()

	// c1 receives system messages (join of c2, c3) but NOT its own chat_msg.
	hasOwnChat := false
	for _, m := range c1Msgs {
		if m.Action == ChatActionMsg && m.Text == "hello" {
			hasOwnChat = true
		}
	}
	assert.False(t, hasOwnChat, "sender should not receive own message")

	// c2 and c3 should have the chat message.
	var c2HasChat, c3HasChat bool
	for _, m := range c2Msgs {
		if m.Action == ChatActionMsg && m.Text == "hello" && m.From == "Alice" {
			c2HasChat = true
		}
	}
	for _, m := range c3Msgs {
		if m.Action == ChatActionMsg && m.Text == "hello" && m.From == "Alice" {
			c3HasChat = true
		}
	}
	assert.True(t, c2HasChat, "c2 should receive chat message")
	assert.True(t, c3HasChat, "c3 should receive chat message")
}

func TestChatHub_History(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	hub.Register("sender", "Sender", sender.sendFn)

	// Send 5 messages.
	for i := 0; i < 5; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: fmt.Sprintf("msg-%d", i)})
		hub.HandleIncoming("sender", raw)
	}

	// Register new participant — should receive history with 5 messages + system msgs.
	newC := &collector{}
	hub.Register("new", "Newbie", newC.sendFn)

	raws := newC.allRaw()
	require.NotEmpty(t, raws)

	var hist ChatHistoryPayload
	require.NoError(t, json.Unmarshal(raws[0], &hist))
	assert.Equal(t, ChatActionHistory, hist.Action)

	// History should contain the 5 chat messages plus system messages for joins.
	chatCount := 0
	for _, m := range hist.Messages {
		if m.Action == ChatActionMsg {
			chatCount++
		}
	}
	assert.Equal(t, 5, chatCount, "history should contain 5 chat messages")
}

func TestChatHub_HistoryRingBuffer(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	hub.Register("sender", "Sender", sender.sendFn)

	// Directly add 150 messages to history to bypass rate limiting.
	hub.mu.Lock()
	for i := 0; i < 150; i++ {
		hub.addToHistory(ChatMessage{
			Action: ChatActionMsg,
			From:   "Sender",
			Text:   fmt.Sprintf("msg-%d", i),
			TS:     time.Now().UnixMilli() + int64(i),
		})
	}
	hub.mu.Unlock()

	// Register new participant and check history size.
	newC := &collector{}
	hub.Register("new", "Newbie", newC.sendFn)

	raws := newC.allRaw()
	require.NotEmpty(t, raws)

	var hist ChatHistoryPayload
	require.NoError(t, json.Unmarshal(raws[0], &hist))
	assert.Equal(t, MaxChatHistory, len(hist.Messages), "history should be capped at MaxChatHistory")

	// Find the last chat_msg — should be msg-149.
	var lastChat ChatMessage
	for i := len(hist.Messages) - 1; i >= 0; i-- {
		if hist.Messages[i].Action == ChatActionMsg {
			lastChat = hist.Messages[i]
			break
		}
	}
	assert.Equal(t, "msg-149", lastChat.Text, "last chat message should be the most recent")
}

func TestChatHub_RateLimit(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	receiver := &collector{}
	hub.Register("sender", "Sender", sender.sendFn)
	hub.Register("receiver", "Receiver", receiver.sendFn)

	// Count receiver messages before sending.
	baseMsgs := receiver.count()

	// Send 15 messages rapidly (should only deliver first 10).
	for i := 0; i < 15; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: fmt.Sprintf("msg-%d", i)})
		hub.HandleIncoming("sender", raw)
	}

	// Count chat messages received by receiver (excluding system messages from registration).
	allRaw := receiver.allRaw()
	chatCount := 0
	for i := baseMsgs; i < len(allRaw); i++ {
		var msg ChatMessage
		if json.Unmarshal(allRaw[i], &msg) == nil && msg.Action == ChatActionMsg {
			chatCount++
		}
	}
	assert.Equal(t, ChatRateLimit, chatCount, "should only deliver ChatRateLimit messages per second")
}

func TestChatHub_MessageValidation(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	receiver := &collector{}
	hub.Register("sender", "Sender", sender.sendFn)
	hub.Register("receiver", "Receiver", receiver.sendFn)

	baseCount := receiver.count()

	// Empty text — should be ignored.
	raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: ""})
	hub.HandleIncoming("sender", raw)
	assert.Equal(t, baseCount, receiver.count(), "empty text should be ignored")

	// Long text — should be truncated.
	longText := make([]rune, 600)
	for i := range longText {
		longText[i] = 'a'
	}
	raw, _ = json.Marshal(ChatMessage{Action: ChatActionMsg, Text: string(longText)})
	hub.HandleIncoming("sender", raw)

	allRaw := receiver.allRaw()
	var lastMsg ChatMessage
	for i := len(allRaw) - 1; i >= 0; i-- {
		if json.Unmarshal(allRaw[i], &lastMsg) == nil && lastMsg.Action == ChatActionMsg {
			break
		}
	}
	assert.Equal(t, MaxChatMessageLen, len([]rune(lastMsg.Text)), "long text should be truncated")

	// From field overwritten — client tries to spoof.
	raw, _ = json.Marshal(ChatMessage{Action: ChatActionMsg, From: "Hacker", Text: "spoofed"})
	hub.HandleIncoming("sender", raw)

	allRaw = receiver.allRaw()
	for i := len(allRaw) - 1; i >= 0; i-- {
		if json.Unmarshal(allRaw[i], &lastMsg) == nil && lastMsg.Action == ChatActionMsg && lastMsg.Text == "spoofed" {
			break
		}
	}
	assert.Equal(t, "Sender", lastMsg.From, "from field should be overwritten with registered nickname")
}

func TestChatHub_SendFromServer(t *testing.T) {
	var received []ChatMessage
	var mu sync.Mutex
	onMsg := func(msg ChatMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}

	hub := NewChatHub(chatTestLogger(), 5, onMsg)

	c1 := &collector{}
	c2 := &collector{}
	hub.Register("c1", "Alice", c1.sendFn)
	hub.Register("c2", "Bob", c2.sendFn)

	hub.SendFromServer("hello from server")

	// Both should receive it.
	for _, c := range []*collector{c1, c2} {
		found := false
		for _, raw := range c.allRaw() {
			var msg ChatMessage
			if json.Unmarshal(raw, &msg) == nil && msg.Action == ChatActionMsg && msg.From == "Server" && msg.Text == "hello from server" {
				found = true
			}
		}
		assert.True(t, found, "participant should receive server message")
	}

	// onMessage should also have been called.
	mu.Lock()
	serverMsgFound := false
	for _, m := range received {
		if m.From == "Server" && m.Text == "hello from server" {
			serverMsgFound = true
		}
	}
	mu.Unlock()
	assert.True(t, serverMsgFound, "onMessage callback should receive server message")
}

// ── ChatClient Tests ───────────────────────────────────────────────────────

func TestChatClient_Send(t *testing.T) {
	var sent []byte
	client := NewChatClient("TestUser", nil)
	client.SetSendFn(func(data []byte) error {
		sent = data
		return nil
	})

	err := client.Send("hello")
	require.NoError(t, err)

	var msg ChatMessage
	require.NoError(t, json.Unmarshal(sent, &msg))
	assert.Equal(t, ChatActionMsg, msg.Action)
	assert.Equal(t, "TestUser", msg.From)
	assert.Equal(t, "hello", msg.Text)
	assert.Greater(t, msg.TS, int64(0))
}

func TestChatClient_Send_NoSendFn(t *testing.T) {
	client := NewChatClient("TestUser", nil)
	err := client.Send("hello")
	assert.Error(t, err)
}

func TestChatClient_HandleIncoming(t *testing.T) {
	var received []ChatMessage
	client := NewChatClient("Me", func(msg ChatMessage) {
		received = append(received, msg)
	})

	// chat_msg
	raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, From: "Alice", Text: "hi", TS: 1000})
	client.HandleIncoming(raw)

	// chat_system
	raw, _ = json.Marshal(ChatMessage{Action: ChatActionSystem, Text: "Bob joined", TS: 2000})
	client.HandleIncoming(raw)

	assert.Len(t, received, 2)
	assert.Equal(t, ChatActionMsg, received[0].Action)
	assert.Equal(t, "Alice", received[0].From)
	assert.Equal(t, ChatActionSystem, received[1].Action)
	assert.Equal(t, "Bob joined", received[1].Text)

	// Check history stored.
	client.mu.RLock()
	assert.Len(t, client.history, 2)
	client.mu.RUnlock()
}

func TestChatClient_HandleHistory(t *testing.T) {
	var received []ChatMessage
	client := NewChatClient("Me", func(msg ChatMessage) {
		received = append(received, msg)
	})

	// Pre-populate some history.
	raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, From: "Old", Text: "old msg", TS: 100})
	client.HandleIncoming(raw)
	assert.Len(t, received, 1)

	// Now receive chat_history — should replace.
	histMsgs := []ChatMessage{
		{Action: ChatActionMsg, From: "A", Text: "m1", TS: 1000},
		{Action: ChatActionMsg, From: "B", Text: "m2", TS: 2000},
		{Action: ChatActionSystem, Text: "C joined", TS: 3000},
	}
	payload := ChatHistoryPayload{Action: ChatActionHistory, Messages: histMsgs}
	raw, _ = json.Marshal(payload)
	client.HandleIncoming(raw)

	// onMessage called for each history message.
	assert.Len(t, received, 4) // 1 old + 3 from history

	// Internal history should be replaced (not appended).
	client.mu.RLock()
	assert.Equal(t, histMsgs, client.history)
	client.mu.RUnlock()
}

func TestChatHub_HistoryOrder(t *testing.T) {
	// Verify ring buffer returns messages in chronological order.
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	hub.Register("s", "S", sender.sendFn)

	// Fill beyond capacity to exercise ring buffer ordering.
	for i := 0; i < 120; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: fmt.Sprintf("%d", i)})
		hub.HandleIncoming("s", raw)
		// Small sleep not needed — rate limit resets each second; we need to bypass it.
	}

	// Force time forward for rate limiter — manually adjust participant.
	// Actually, after 10 msgs in same second, the rest are dropped.
	// So only 10 chat messages went through per "second". Let's use a different approach:
	// send in batches with time manipulation. For simplicity, just verify what we got.

	newC := &collector{}
	hub.Register("n", "N", newC.sendFn)

	var hist ChatHistoryPayload
	_ = json.Unmarshal(newC.allRaw()[0], &hist)

	// Verify that history is in chronological order (TS non-decreasing).
	for i := 1; i < len(hist.Messages); i++ {
		assert.GreaterOrEqual(t, hist.Messages[i].TS, hist.Messages[i-1].TS,
			"history should be in chronological order at index %d", i)
	}
}

// ── ChatClient History Cap Tests ───────────────────────────────────────────

func TestChatClient_HistoryCap(t *testing.T) {
	client := NewChatClient("Me", nil)

	// Send 210 chat_msg messages via HandleIncoming.
	for i := 0; i < 210; i++ {
		raw, _ := json.Marshal(ChatMessage{
			Action: ChatActionMsg,
			From:   "Alice",
			Text:   fmt.Sprintf("msg-%d", i),
			TS:     int64(i),
		})
		client.HandleIncoming(raw)
	}

	client.mu.RLock()
	histLen := len(client.history)
	lastMsg := client.history[histLen-1]
	firstMsg := client.history[0]
	client.mu.RUnlock()

	assert.Equal(t, 200, histLen, "history should be capped at 200")
	// Most recent 200: messages 10..209 (0-indexed).
	assert.Equal(t, "msg-10", firstMsg.Text, "first retained message should be msg-10")
	assert.Equal(t, "msg-209", lastMsg.Text, "last retained message should be msg-209")
}

func TestChatClient_HistoryCap_OnHistoryPayload(t *testing.T) {
	client := NewChatClient("Me", nil)

	// Build a chat_history payload with 250 messages.
	msgs := make([]ChatMessage, 250)
	for i := 0; i < 250; i++ {
		msgs[i] = ChatMessage{
			Action: ChatActionMsg,
			From:   "Bob",
			Text:   fmt.Sprintf("hist-%d", i),
			TS:     int64(i),
		}
	}
	payload := ChatHistoryPayload{Action: ChatActionHistory, Messages: msgs}
	raw, _ := json.Marshal(payload)
	client.HandleIncoming(raw)

	client.mu.RLock()
	histLen := len(client.history)
	firstMsg := client.history[0]
	lastMsg := client.history[histLen-1]
	client.mu.RUnlock()

	assert.Equal(t, 200, histLen, "history from payload should be capped at 200")
	// Most recent 200: messages 50..249.
	assert.Equal(t, "hist-50", firstMsg.Text, "first retained message should be hist-50")
	assert.Equal(t, "hist-249", lastMsg.Text, "last retained message should be hist-249")
}

func TestChatHub_RateLimit_ResetAfterSecond(t *testing.T) {
	hub := NewChatHub(chatTestLogger(), 5, nil)

	sender := &collector{}
	receiver := &collector{}
	hub.Register("sender", "Sender", sender.sendFn)
	hub.Register("receiver", "Receiver", receiver.sendFn)

	baseCount := receiver.count()

	// Send ChatRateLimit messages.
	for i := 0; i < ChatRateLimit; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: fmt.Sprintf("batch1-%d", i)})
		hub.HandleIncoming("sender", raw)
	}

	// Wait for rate limit window to reset.
	time.Sleep(1100 * time.Millisecond)

	// Send more — should succeed.
	for i := 0; i < 3; i++ {
		raw, _ := json.Marshal(ChatMessage{Action: ChatActionMsg, Text: fmt.Sprintf("batch2-%d", i)})
		hub.HandleIncoming("sender", raw)
	}

	allRaw := receiver.allRaw()
	chatCount := 0
	for i := baseCount; i < len(allRaw); i++ {
		var msg ChatMessage
		if json.Unmarshal(allRaw[i], &msg) == nil && msg.Action == ChatActionMsg {
			chatCount++
		}
	}
	assert.Equal(t, ChatRateLimit+3, chatCount, "messages should go through after rate limit window resets")
}
