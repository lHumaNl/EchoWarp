package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"nhooyr.io/websocket"        //nolint:staticcheck // migration to coder/websocket tracked separately
	"nhooyr.io/websocket/wsjson" //nolint:staticcheck // migration to coder/websocket tracked separately
)

// WSEvent represents an event sent over WebSocket.
type WSEvent struct {
	Type string      `json:"type"` // Event type identifier.
	Data interface{} `json:"data"` // Event-specific payload.
}

// WSHub manages WebSocket client connections and broadcasts events.
// Thread-safe for concurrent use.
type WSHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]context.CancelFunc //nolint:staticcheck
	logger  *slog.Logger
}

// NewWSHub creates a new WebSocket hub for managing client connections.
func NewWSHub(logger *slog.Logger) *WSHub {
	return &WSHub{
		clients: make(map[*websocket.Conn]context.CancelFunc), //nolint:staticcheck
		logger:  logger,
	}
}

// AddClient registers a new WebSocket connection with its cancel function.
// The cancel function is called when the client should be disconnected.
func (h *WSHub) AddClient(conn *websocket.Conn, cancel context.CancelFunc) { //nolint:staticcheck
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = cancel
	h.logger.Debug("websocket client added", "count", len(h.clients))
}

// RemoveClient disconnects and removes a WebSocket client.
// Safe to call multiple times for the same connection.
func (h *WSHub) RemoveClient(conn *websocket.Conn) { //nolint:staticcheck
	h.mu.Lock()
	defer h.mu.Unlock()
	if cancel, ok := h.clients[conn]; ok {
		cancel()
		delete(h.clients, conn)
	}
	h.logger.Debug("websocket client removed", "count", len(h.clients))
}

// Broadcast sends an event to all connected WebSocket clients.
// Failed sends result in client removal. Non-blocking per client (5s timeout).
func (h *WSHub) Broadcast(event WSEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal event", "error", err)
		return
	}

	for conn := range h.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		err := conn.Write(ctx, websocket.MessageText, data) //nolint:staticcheck
		cancel()
		if err != nil {
			h.logger.Debug("failed to broadcast to client", "error", err)
			go h.RemoveClient(conn)
		}
	}
}

// ClientCount returns the number of currently connected WebSocket clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Close disconnects all WebSocket clients and clears the connection map.
// Called during API server shutdown.
func (h *WSHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn, cancel := range h.clients {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "server shutdown") //nolint:errcheck,staticcheck
	}
	h.clients = make(map[*websocket.Conn]context.CancelFunc) //nolint:staticcheck
}

// wsOrigins returns the list of allowed WebSocket origin patterns.
// Defaults to localhost patterns if not explicitly configured.
func (s *APIServer) wsOrigins() []string {
	if len(s.wsAllowedOrigins) > 0 {
		return s.wsAllowedOrigins
	}
	return []string{
		"http://localhost:*",
		"http://127.0.0.1:*",
		"https://localhost:*",
		"https://127.0.0.1:*",
	}
}

// handleWebSocket upgrades an HTTP connection to WebSocket and handles the event stream.
// Clients receive real-time notifications about status changes, errors, and stats.
func (s *APIServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	origins := s.wsOrigins()
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{ //nolint:staticcheck
		OriginPatterns: origins,
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	s.wsHub.AddClient(conn, cancel)
	defer func() {
		s.wsHub.RemoveClient(conn)
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed") //nolint:errcheck,staticcheck
	}()

	s.wsHub.Broadcast(WSEvent{Type: "client_connected", Data: map[string]int{"count": s.wsHub.ClientCount()}})

	for {
		var msg interface{}
		err := wsjson.Read(ctx, conn, &msg)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure || //nolint:staticcheck
				websocket.CloseStatus(err) == websocket.StatusGoingAway { //nolint:staticcheck
				return
			}
			s.logger.Debug("websocket read error", "error", err)
			return
		}
		s.logger.Debug("websocket message received", "message", msg)
	}
}
