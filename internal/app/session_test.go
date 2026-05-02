package app

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConn is a minimal net.Conn that does nothing, used for handleReconnectBySession tests.
type fakeConn struct {
	net.Conn
}

func (f *fakeConn) Close() error { return nil }
func (f *fakeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
}
func (f *fakeConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (f *fakeConn) Read(b []byte) (int, error)         { return 0, nil }
func (f *fakeConn) Write(b []byte) (int, error)        { return len(b), nil }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

func TestHandleReconnectBySession_FromSessionsMap(t *testing.T) {
	s := newMinimalServerApp(nil)

	// Simulate an abnormal disconnect: session stored in sessions map.
	s.mu.Lock()
	s.sessions["abc-123"] = &sessionEntry{
		clientID:     "client-1",
		nickname:     "Alice",
		isCustomNick: false,
	}
	s.mu.Unlock()

	oldNick, found := s.handleReconnectBySession("abc-123")

	require.True(t, found, "should find the session by sessionID")
	assert.Equal(t, "Alice", oldNick, "should return the old server-assigned nickname")

	// Session should be removed after reconnect.
	s.mu.RLock()
	_, stillPresent := s.sessions["abc-123"]
	s.mu.RUnlock()
	assert.False(t, stillPresent, "session should be removed from sessions map after reconnect")
}

func TestHandleReconnectBySession_CustomNickNotRestored(t *testing.T) {
	s := newMinimalServerApp(nil)

	s.mu.Lock()
	s.sessions["custom-123"] = &sessionEntry{
		clientID:     "client-2",
		nickname:     "CustomNick",
		isCustomNick: true,
	}
	s.mu.Unlock()

	oldNick, found := s.handleReconnectBySession("custom-123")

	require.True(t, found, "should find the session by sessionID")
	assert.Equal(t, "", oldNick, "should NOT return a custom nickname for restoration")
}

func TestHandleReconnectBySession_FallbackToClientsMap(t *testing.T) {
	s := newMinimalServerApp(nil)

	// Simulate a stale connected client (no session entry, but client still in clients map).
	s.mu.Lock()
	s.clients["client-1"] = &multiClient{
		id:        "client-1",
		nickname:  "Alice",
		sessionID: "abc-123",
		conn:      &fakeConn{},
	}
	s.mu.Unlock()

	oldNick, found := s.handleReconnectBySession("abc-123")

	require.True(t, found, "should find the client by sessionID in clients map")
	assert.Equal(t, "Alice", oldNick, "should return the old nickname")

	s.mu.RLock()
	_, stillPresent := s.clients["client-1"]
	s.mu.RUnlock()
	assert.False(t, stillPresent, "client should be removed from clients map after reconnect")
}

func TestHandleReconnectBySession_NotFound(t *testing.T) {
	s := newMinimalServerApp(nil)

	oldNick, found := s.handleReconnectBySession("unknown-session-id")

	assert.False(t, found, "should not find a client for unknown sessionID")
	assert.Equal(t, "", oldNick)
}

func TestHandleReconnectBySession_Empty(t *testing.T) {
	s := newMinimalServerApp(nil)

	oldNick, found := s.handleReconnectBySession("")

	assert.False(t, found, "empty sessionID should return false immediately")
	assert.Equal(t, "", oldNick)
}

func TestUnregisterMultiClient_GracefulRemovesSession(t *testing.T) {
	s := newMinimalServerApp(nil)

	mc := &multiClient{
		id:           "client-1",
		nickname:     "Alice",
		sessionID:    "sess-1",
		isCustomNick: false,
	}
	s.mu.Lock()
	s.clients["client-1"] = mc
	s.mu.Unlock()

	s.unregisterMultiClient(mc, "client-1", true)

	s.mu.RLock()
	_, clientPresent := s.clients["client-1"]
	_, sessionPresent := s.sessions["sess-1"]
	s.mu.RUnlock()

	assert.False(t, clientPresent, "client should be removed")
	assert.False(t, sessionPresent, "session should NOT be stored on graceful disconnect")
}

func TestUnregisterMultiClient_AbnormalPreservesSession(t *testing.T) {
	s := newMinimalServerApp(nil)

	mc := &multiClient{
		id:           "client-1",
		nickname:     "Bob",
		sessionID:    "sess-2",
		isCustomNick: false,
	}
	s.mu.Lock()
	s.clients["client-1"] = mc
	s.mu.Unlock()

	s.unregisterMultiClient(mc, "client-1", false)

	s.mu.RLock()
	_, clientPresent := s.clients["client-1"]
	entry, sessionPresent := s.sessions["sess-2"]
	s.mu.RUnlock()

	assert.False(t, clientPresent, "client should be removed")
	require.True(t, sessionPresent, "session should be preserved on abnormal disconnect")
	assert.Equal(t, "Bob", entry.nickname)
	assert.Equal(t, "client-1", entry.clientID)
	assert.False(t, entry.isCustomNick)
}

func TestSessionID_ReturnedOnFirstAuth(t *testing.T) {
	// Verify that sessionID is generated and stored in multiClient during registration.
	s := newMinimalServerApp(nil)

	mc := &multiClient{
		id:        "client-1",
		nickname:  "Alice",
		sessionID: "generated-uuid-here",
	}
	s.mu.Lock()
	s.clients["client-1"] = mc
	s.mu.Unlock()

	s.mu.RLock()
	stored := s.clients["client-1"]
	s.mu.RUnlock()
	assert.Equal(t, "generated-uuid-here", stored.sessionID)
}

func TestReconnectWithValidSession_RestoresNickname(t *testing.T) {
	s := newMinimalServerApp(nil)

	// Store a session from a previous abnormal disconnect.
	s.mu.Lock()
	s.sessions["reconnect-sess"] = &sessionEntry{
		clientID:     "client-old",
		nickname:     "ReconnectUser",
		isCustomNick: false,
	}
	s.mu.Unlock()

	oldNick, found := s.handleReconnectBySession("reconnect-sess")

	require.True(t, found)
	assert.Equal(t, "ReconnectUser", oldNick, "reconnect should restore server-assigned nickname")
}

func TestReconnectWithInvalidSession_GetsNewIdentity(t *testing.T) {
	s := newMinimalServerApp(nil)

	oldNick, found := s.handleReconnectBySession("invalid-session-id")

	assert.False(t, found, "invalid session should not be found")
	assert.Equal(t, "", oldNick, "should get empty nickname (caller assigns new one)")
}

func TestClientSessionID_StoredAndSent(t *testing.T) {
	// Verify that ClientApp stores and exposes sessionID field.
	c := &ClientApp{}
	assert.Equal(t, "", c.sessionID, "initially empty")

	c.sessionID = "test-session-id"
	assert.Equal(t, "test-session-id", c.sessionID)
}

func TestClientSessionID_ClearedOnServerStop(t *testing.T) {
	c := &ClientApp{
		sessionID: "some-session",
	}

	// Simulate server stop notification.
	c.closeServerStoppedCh()

	assert.Equal(t, "", c.sessionID, "sessionID should be cleared on server graceful shutdown")
}
