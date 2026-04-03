package transport

import (
	"encoding/json"
	"fmt"
	"sync"
)

const (
	MetadataChannelLabel = "metadata"

	MetaMsgTrackInfo   = "track_info"
	MetaMsgVolumeCtrl  = "volume"
	MetaMsgChat        = "chat"
	MetaMsgPing        = "ping"
	MetaMsgPong        = "pong"
	MetaMsgStatsReport = "stats_report"

	maxMetadataMessageSize = 65536
)

// MetadataMessage represents a message sent over the metadata data channel.
type MetadataMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// TrackInfo contains information about the currently playing audio track.
type TrackInfo struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Duration int    `json:"duration,omitempty"`
}

// VolumeControl represents volume level and mute state.
type VolumeControl struct {
	Level float64 `json:"level"`
	Mute  bool    `json:"mute"`
}

// ChatMessage represents a text chat message.
type ChatMessage struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
	Time   int64  `json:"time"`
}

// StatsReport contains real-time connection statistics.
type StatsReport struct {
	PacketsLost uint32  `json:"packets_lost"`
	Jitter      float64 `json:"jitter_ms"`
	RoundTrip   float64 `json:"rtt_ms"`
	Bitrate     int     `json:"current_bitrate"`
}

// MetadataHandler handles incoming metadata messages from the data channel.
// Thread-safe: Callback registration and invocation protected by sync.RWMutex.
type MetadataHandler struct {
	mu            sync.RWMutex
	onTrackInfo   func(TrackInfo)
	onVolumeCtrl  func(VolumeControl)
	onChat        func(ChatMessage)
	onStatsReport func(StatsReport)
	onPong        func()
	onPing        func()
}

// NewMetadataHandler creates a new metadata message handler.
func NewMetadataHandler() *MetadataHandler {
	return &MetadataHandler{}
}

// SetOnTrackInfo registers a callback for track info messages.
func (h *MetadataHandler) SetOnTrackInfo(fn func(TrackInfo)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onTrackInfo = fn
}

// SetOnVolumeControl registers a callback for volume control messages.
func (h *MetadataHandler) SetOnVolumeControl(fn func(VolumeControl)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onVolumeCtrl = fn
}

// SetOnChat registers a callback for chat messages.
func (h *MetadataHandler) SetOnChat(fn func(ChatMessage)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChat = fn
}

// SetOnStatsReport registers a callback for stats report messages.
func (h *MetadataHandler) SetOnStatsReport(fn func(StatsReport)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onStatsReport = fn
}

// SetOnPong registers a callback for pong messages.
func (h *MetadataHandler) SetOnPong(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onPong = fn
}

// SetOnPing registers a callback for ping messages.
func (h *MetadataHandler) SetOnPing(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onPing = fn
}

// HandleMessage parses and dispatches a metadata message to the appropriate handler.
// Returns an error for unknown message types or invalid payloads.
func (h *MetadataHandler) HandleMessage(data []byte) error {
	if len(data) > maxMetadataMessageSize {
		return fmt.Errorf("message size %d exceeds maximum %d", len(data), maxMetadataMessageSize)
	}
	var msg MetadataMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("unmarshal metadata message: %w", err)
	}

	switch msg.Type {
	case MetaMsgTrackInfo:
		return h.handleTrackInfo(msg.Payload)
	case MetaMsgVolumeCtrl:
		return h.handleVolumeCtrl(msg.Payload)
	case MetaMsgChat:
		return h.handleChat(msg.Payload)
	case MetaMsgStatsReport:
		return h.handleStatsReport(msg.Payload)
	case MetaMsgPong:
		h.invokeCallback(func() func() { return h.onPong })
		return nil
	case MetaMsgPing:
		h.invokeCallback(func() func() { return h.onPing })
		return nil
	default:
		return fmt.Errorf("unknown metadata message type: %s", msg.Type)
	}
}

func (h *MetadataHandler) handleTrackInfo(payload json.RawMessage) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload for message type %s", MetaMsgTrackInfo)
	}
	var ti TrackInfo
	if err := json.Unmarshal(payload, &ti); err != nil {
		return fmt.Errorf("unmarshal track info: %w", err)
	}
	h.mu.RLock()
	fn := h.onTrackInfo
	h.mu.RUnlock()
	if fn != nil {
		fn(ti)
	}
	return nil
}

func (h *MetadataHandler) handleVolumeCtrl(payload json.RawMessage) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload for message type %s", MetaMsgVolumeCtrl)
	}
	var vc VolumeControl
	if err := json.Unmarshal(payload, &vc); err != nil {
		return fmt.Errorf("unmarshal volume control: %w", err)
	}
	if vc.Level < 0.0 || vc.Level > 1.0 {
		return fmt.Errorf("volume level %.2f out of range [0.0, 1.0]", vc.Level)
	}
	h.mu.RLock()
	fn := h.onVolumeCtrl
	h.mu.RUnlock()
	if fn != nil {
		fn(vc)
	}
	return nil
}

func (h *MetadataHandler) handleChat(payload json.RawMessage) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload for message type %s", MetaMsgChat)
	}
	var cm ChatMessage
	if err := json.Unmarshal(payload, &cm); err != nil {
		return fmt.Errorf("unmarshal chat message: %w", err)
	}
	h.mu.RLock()
	fn := h.onChat
	h.mu.RUnlock()
	if fn != nil {
		fn(cm)
	}
	return nil
}

func (h *MetadataHandler) handleStatsReport(payload json.RawMessage) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload for message type %s", MetaMsgStatsReport)
	}
	var sr StatsReport
	if err := json.Unmarshal(payload, &sr); err != nil {
		return fmt.Errorf("unmarshal stats report: %w", err)
	}
	h.mu.RLock()
	fn := h.onStatsReport
	h.mu.RUnlock()
	if fn != nil {
		fn(sr)
	}
	return nil
}

func (h *MetadataHandler) invokeCallback(getFn func() func()) {
	fn := getFn()
	if fn != nil {
		fn()
	}
}

// EncodeMessage creates a JSON-encoded metadata message.
func EncodeMessage(msgType string, payload interface{}) ([]byte, error) {
	msg := MetadataMessage{
		Type: msgType,
	}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		msg.Payload = data
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata message: %w", err)
	}

	return data, nil
}
