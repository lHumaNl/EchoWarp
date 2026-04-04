package app

// Chat message action constants.
const (
	ChatActionMsg          = "chat_msg"
	ChatActionHistory      = "chat_history"
	ChatActionSystem       = "chat_system"
	ChatActionParticipants = "chat_participants"

	// MaxChatHistory is the maximum number of messages kept in the ring buffer.
	MaxChatHistory = 100

	// MaxChatMessageLen is the maximum allowed length of a chat message text.
	MaxChatMessageLen = 500

	// ChatRateLimit is the maximum number of messages a single client can send per second.
	ChatRateLimit = 10
)

// ChatMessage represents a chat message exchanged between peers.
type ChatMessage struct {
	Action string `json:"action"`         // "chat_msg", "chat_history", "chat_system"
	From   string `json:"from,omitempty"` // sender display name
	To     string `json:"to,omitempty"`   // DM recipient nickname ("" = broadcast)
	Text   string `json:"text"`           // message content
	TS     int64  `json:"ts"`             // unix milliseconds
}

// IsDM returns true if the message is a direct message (has a recipient).
func (m ChatMessage) IsDM() bool { return m.To != "" }

// ChatHistoryPayload is sent to newly connected clients.
type ChatHistoryPayload struct {
	Action   string        `json:"action"` // always "chat_history"
	Messages []ChatMessage `json:"messages"`
}

// ChatParticipantsPayload is broadcast when the participant list changes.
type ChatParticipantsPayload struct {
	Action       string   `json:"action"`       // always "chat_participants"
	Participants []string `json:"participants"` // nicknames of online participants
	MaxClients   int      `json:"max_clients"`
}
