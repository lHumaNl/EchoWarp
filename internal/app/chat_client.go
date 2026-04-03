package app

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ChatClient is the client-side chat handler.
type ChatClient struct {
	mu             sync.RWMutex
	sendFn         func([]byte) error
	nickname       string
	history        []ChatMessage
	onMessage      func(ChatMessage)
	onParticipants func(ChatParticipantsPayload)
}

// NewChatClient creates a new ChatClient.
func NewChatClient(nickname string, onMessage func(ChatMessage)) *ChatClient {
	return &ChatClient{
		nickname:  nickname,
		onMessage: onMessage,
	}
}

// SetSendFn sets the send function, called when the chat DataChannel opens.
func (c *ChatClient) SetSendFn(fn func([]byte) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sendFn = fn
}

// SetOnParticipants sets the callback invoked when a participants list update is received.
func (c *ChatClient) SetOnParticipants(fn func(ChatParticipantsPayload)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onParticipants = fn
}

// Send creates and sends a chat message to the server.
// If toNickname is non-empty, the message is sent as a DM.
func (c *ChatClient) Send(text string, toNickname ...string) error {
	c.mu.RLock()
	fn := c.sendFn
	nick := c.nickname
	c.mu.RUnlock()

	if fn == nil {
		return fmt.Errorf("chat: send function not set")
	}

	msg := ChatMessage{
		Action: ChatActionMsg,
		From:   nick,
		Text:   text,
		TS:     time.Now().UnixMilli(),
	}
	if len(toNickname) > 0 && toNickname[0] != "" {
		msg.To = toNickname[0]
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("chat: marshal error: %w", err)
	}
	if err := fn(data); err != nil {
		return err
	}

	// Local echo: show own message immediately without waiting for server round-trip.
	c.mu.Lock()
	c.history = append(c.history, msg)
	if len(c.history) > 200 {
		c.history = c.history[len(c.history)-200:]
	}
	c.mu.Unlock()
	if c.onMessage != nil {
		c.onMessage(msg)
	}
	return nil
}

// HandleIncoming parses an incoming message and dispatches by action type.
func (c *ChatClient) HandleIncoming(raw []byte) {
	// Try to detect action first.
	var probe struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}

	switch probe.Action {
	case ChatActionMsg, ChatActionSystem:
		var msg ChatMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		c.mu.Lock()
		c.history = append(c.history, msg)
		if len(c.history) > 200 {
			c.history = c.history[len(c.history)-200:]
		}
		c.mu.Unlock()
		if c.onMessage != nil {
			c.onMessage(msg)
		}

	case ChatActionHistory:
		var payload ChatHistoryPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return
		}
		c.mu.Lock()
		c.history = payload.Messages
		if len(c.history) > 200 {
			c.history = c.history[len(c.history)-200:]
		}
		c.mu.Unlock()
		if c.onMessage != nil {
			for _, msg := range payload.Messages {
				c.onMessage(msg)
			}
		}

	case ChatActionParticipants:
		var payload ChatParticipantsPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return
		}
		c.mu.RLock()
		fn := c.onParticipants
		c.mu.RUnlock()
		if fn != nil {
			fn(payload)
		}
	}
}
