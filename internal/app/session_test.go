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

func TestHandleReconnectBySession_Found(t *testing.T) {
	s := newMinimalServerApp(nil)

	s.mu.Lock()
	s.clients["client-1"] = &multiClient{
		id:        "client-1",
		nickname:  "Alice",
		sessionID: "abc-123",
		conn:      &fakeConn{},
	}
	s.mu.Unlock()

	oldNick, found := s.handleReconnectBySession("abc-123")

	require.True(t, found, "should find the client by sessionID")
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
