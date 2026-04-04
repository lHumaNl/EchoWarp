package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ChatHub is the server-side hub that manages chat participants, history, and message fan-out.
type ChatHub struct {
	mu           sync.RWMutex
	participants map[string]*chatParticipant
	history      []ChatMessage
	historyIdx   int
	maxClients   int
	onMessage    func(ChatMessage)
	logger       *slog.Logger
}

type chatParticipant struct {
	nickname  string
	sendFn    func([]byte) error
	lastMsgAt time.Time
	msgCount  int
}

// NewChatHub creates a new ChatHub.
func NewChatHub(logger *slog.Logger, maxClients int, onMessage func(ChatMessage)) *ChatHub {
	return &ChatHub{
		participants: make(map[string]*chatParticipant),
		maxClients:   maxClients,
		onMessage:    onMessage,
		logger:       logger,
	}
}

// Register adds a participant, sends chat history, and broadcasts a system "joined" message.
func (h *ChatHub) Register(clientID, nickname string, sendFn func([]byte) error) {
	h.mu.Lock()
	h.participants[clientID] = &chatParticipant{
		nickname: nickname,
		sendFn:   sendFn,
	}
	h.mu.Unlock()

	h.sendHistory(nickname, sendFn)

	sysMsg := ChatMessage{
		Action: ChatActionSystem,
		Text:   fmt.Sprintf("%s joined", nickname),
		TS:     time.Now().UnixMilli(),
	}
	h.mu.Lock()
	h.addToHistory(sysMsg)
	h.mu.Unlock()

	if h.onMessage != nil {
		h.onMessage(sysMsg)
	}

	h.broadcast(sysMsg, "")
	h.broadcastParticipants()
}

// Unregister removes a participant and broadcasts a system "left" message.
func (h *ChatHub) Unregister(clientID string) {
	h.mu.Lock()
	p, ok := h.participants[clientID]
	if !ok {
		h.mu.Unlock()
		return
	}
	nickname := p.nickname
	delete(h.participants, clientID)
	h.mu.Unlock()

	sysMsg := ChatMessage{
		Action: ChatActionSystem,
		Text:   fmt.Sprintf("%s left", nickname),
		TS:     time.Now().UnixMilli(),
	}
	h.mu.Lock()
	h.addToHistory(sysMsg)
	h.mu.Unlock()

	if h.onMessage != nil {
		h.onMessage(sysMsg)
	}

	h.broadcast(sysMsg, "")
	h.broadcastParticipants()
}

// HandleIncoming parses an incoming chat message from a client, validates it,
// stores it in history, and broadcasts to other participants.
func (h *ChatHub) HandleIncoming(clientID string, raw []byte) {
	var msg ChatMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		h.logger.Warn("ChatHub: invalid message JSON", "clientID", clientID, "error", err)
		return
	}

	// Ignore empty text.
	if msg.Text == "" {
		return
	}

	// Truncate long messages.
	if len([]rune(msg.Text)) > MaxChatMessageLen {
		msg.Text = string([]rune(msg.Text)[:MaxChatMessageLen])
	}

	h.mu.Lock()
	p, ok := h.participants[clientID]
	if !ok {
		h.mu.Unlock()
		return
	}

	// Rate limiting: max ChatRateLimit messages per second.
	now := time.Now()
	if now.Sub(p.lastMsgAt) >= time.Second {
		p.lastMsgAt = now
		p.msgCount = 1
	} else {
		p.msgCount++
		if p.msgCount > ChatRateLimit {
			h.mu.Unlock()
			return
		}
	}

	// Overwrite from with registered nickname.
	msg.Action = ChatActionMsg
	msg.From = p.nickname
	msg.TS = now.UnixMilli()

	h.addToHistory(msg)
	h.mu.Unlock()

	if msg.IsDM() {
		h.sendDM(msg, clientID)
	} else {
		if h.onMessage != nil {
			h.onMessage(msg)
		}
		h.broadcast(msg, clientID)
	}
}

// SendFromServer creates a message from "Server", adds to history, broadcasts to all.
// If toNickname is non-empty, the message is sent as a DM to that participant only.
func (h *ChatHub) SendFromServer(text string, toNickname ...string) {
	msg := ChatMessage{
		Action: ChatActionMsg,
		From:   "Server",
		Text:   text,
		TS:     time.Now().UnixMilli(),
	}
	if len(toNickname) > 0 && toNickname[0] != "" {
		msg.To = toNickname[0]
	}

	h.mu.Lock()
	h.addToHistory(msg)
	h.mu.Unlock()

	if h.onMessage != nil {
		h.onMessage(msg)
	}

	if msg.IsDM() {
		h.sendDM(msg, "")
	} else {
		h.broadcast(msg, "")
	}
}

// sendDM delivers a direct message to the recipient only. senderClientID is excluded
// from delivery (sender already has local echo).
func (h *ChatHub) sendDM(msg ChatMessage, senderClientID string) {
	// DM addressed to "Server" — deliver via onMessage callback (server operator's TUI).
	if strings.EqualFold(msg.To, "Server") {
		if h.onMessage != nil {
			h.onMessage(msg)
		}
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Warn("ChatHub: failed to marshal DM", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for id, p := range h.participants {
		if id == senderClientID {
			continue
		}
		if p.nickname == msg.To {
			if err := p.sendFn(data); err != nil {
				h.logger.Warn("ChatHub: DM send failed", "clientID", id, "error", err)
			}
			return
		}
	}
	// Recipient not found — send error back to sender.
	if senderClientID != "" {
		errMsg := ChatMessage{
			Action: ChatActionSystem,
			Text:   fmt.Sprintf("User %q not found", msg.To),
			TS:     time.Now().UnixMilli(),
		}
		if errData, err := json.Marshal(errMsg); err == nil {
			if p, ok := h.participants[senderClientID]; ok {
				_ = p.sendFn(errData)
			}
		}
	}
}

// BroadcastSystem sends a system notification to all participants and adds it to history.
func (h *ChatHub) BroadcastSystem(text string) {
	msg := ChatMessage{
		Action: ChatActionSystem,
		Text:   text,
		TS:     time.Now().UnixMilli(),
	}

	h.mu.Lock()
	h.addToHistory(msg)
	h.mu.Unlock()

	if h.onMessage != nil {
		h.onMessage(msg)
	}

	h.broadcast(msg, "")
}

// broadcastParticipants sends the current participant list to all participants.
func (h *ChatHub) broadcastParticipants() {
	h.mu.RLock()
	nicknames := make([]string, 0, len(h.participants))
	for _, p := range h.participants {
		nicknames = append(nicknames, p.nickname)
	}
	participants := make(map[string]*chatParticipant, len(h.participants))
	for id, p := range h.participants {
		participants[id] = p
	}
	h.mu.RUnlock()

	payload := ChatParticipantsPayload{
		Action:       ChatActionParticipants,
		Participants: nicknames,
		MaxClients:   h.maxClients,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Warn("ChatHub: failed to marshal participants", "error", err)
		return
	}

	for id, p := range participants {
		if err := p.sendFn(data); err != nil {
			h.logger.Warn("ChatHub: send participants failed", "clientID", id, "error", err)
		}
	}
}

// broadcast marshals a message to JSON and sends to all participants except excludeID.
func (h *ChatHub) broadcast(msg ChatMessage, excludeID string) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Warn("ChatHub: failed to marshal message", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for id, p := range h.participants {
		if id == excludeID {
			continue
		}
		if err := p.sendFn(data); err != nil {
			h.logger.Warn("ChatHub: send failed", "clientID", id, "error", err)
		}
	}
}

// addToHistory appends a message to the ring buffer. Caller must hold h.mu.
func (h *ChatHub) addToHistory(msg ChatMessage) {
	if len(h.history) < MaxChatHistory {
		h.history = append(h.history, msg)
	} else {
		h.history[h.historyIdx] = msg
		h.historyIdx = (h.historyIdx + 1) % MaxChatHistory
	}
}

// getOrderedHistory returns history messages in chronological order. Caller must hold h.mu (at least RLock).
func (h *ChatHub) getOrderedHistory() []ChatMessage {
	if len(h.history) < MaxChatHistory {
		cp := make([]ChatMessage, len(h.history))
		copy(cp, h.history)
		return cp
	}
	// Ring buffer is full; historyIdx points to the oldest entry.
	result := make([]ChatMessage, MaxChatHistory)
	copy(result, h.history[h.historyIdx:])
	copy(result[MaxChatHistory-h.historyIdx:], h.history[:h.historyIdx])
	return result
}

// sendHistory sends the current history to a single client, filtering out
// DMs that are not addressed to/from this client.
func (h *ChatHub) sendHistory(nickname string, sendFn func([]byte) error) {
	h.mu.RLock()
	all := h.getOrderedHistory()
	h.mu.RUnlock()

	// Filter: keep broadcast messages + DMs where this client is sender or recipient.
	msgs := make([]ChatMessage, 0, len(all))
	for _, m := range all {
		if !m.IsDM() || m.From == nickname || m.To == nickname {
			msgs = append(msgs, m)
		}
	}

	payload := ChatHistoryPayload{
		Action:   ChatActionHistory,
		Messages: msgs,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Warn("ChatHub: failed to marshal history", "error", err)
		return
	}
	if err := sendFn(data); err != nil {
		h.logger.Warn("ChatHub: failed to send history", "error", err)
	}
}

// RenameParticipant updates the nickname of a participant and broadcasts a system message.
func (h *ChatHub) RenameParticipant(clientID, newNick string) {
	h.mu.Lock()
	p, ok := h.participants[clientID]
	if !ok {
		h.mu.Unlock()
		return
	}
	oldNick := p.nickname
	p.nickname = newNick
	sysMsg := ChatMessage{
		Action: ChatActionSystem,
		Text:   fmt.Sprintf("%s renamed to %s", oldNick, newNick),
		TS:     time.Now().UnixMilli(),
	}
	h.addToHistory(sysMsg)
	h.mu.Unlock()

	if h.onMessage != nil {
		h.onMessage(sysMsg)
	}

	h.broadcast(sysMsg, "")
	h.broadcastParticipants()
}
