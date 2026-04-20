package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type mockBanManager struct {
	banned   map[string]bool
	failures map[string]int
	mu       sync.RWMutex
	closed   bool
}

func newMockBanManager() *mockBanManager {
	return &mockBanManager{
		banned:   make(map[string]bool),
		failures: make(map[string]int),
	}
}

func (m *mockBanManager) IsBanned(addr string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.banned[addr]
}

func (m *mockBanManager) RecordFailure(addr string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[addr]++
	if m.failures[addr] >= 5 {
		m.banned[addr] = true
		return true
	}
	return false
}

func (m *mockBanManager) RecordSuccess(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, addr)
}

func (m *mockBanManager) Ban(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.banned[addr] = true
}

func (m *mockBanManager) Unban(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.banned, addr)
}

func (m *mockBanManager) BannedList() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]string, 0, len(m.banned))
	for addr := range m.banned {
		list = append(list, addr)
	}
	return list
}

func (m *mockBanManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockBanManager) BanWithReason(addr, _ string) error { m.Ban(addr); return nil }
func (m *mockBanManager) BannedEntries() []ban.BanEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ban.BanEntry, 0, len(m.banned))
	for addr, b := range m.banned {
		if b {
			out = append(out, ban.BanEntry{Address: addr, Banned: true})
		}
	}
	return out
}
func (m *mockBanManager) IsHWIDBanned(_ string) bool              { return false }
func (m *mockBanManager) BanHWID(_ string)                        {}
func (m *mockBanManager) BanHWIDWithReason(_, _ string) error     { return nil }
func (m *mockBanManager) UnbanHWID(_ string)                      {}
func (m *mockBanManager) BannedHWIDList() []string                { return nil }
func (m *mockBanManager) BannedHWIDEntries() []ban.BanEntry       { return nil }
func (m *mockBanManager) IsNicknameBanned(_ string) bool          { return false }
func (m *mockBanManager) BanNickname(_ string)                    {}
func (m *mockBanManager) BanNicknameWithReason(_, _ string) error { return nil }
func (m *mockBanManager) UnbanNickname(_ string)                  {}
func (m *mockBanManager) BannedNicknameList() []string            { return nil }
func (m *mockBanManager) BannedNicknameEntries() []ban.BanEntry   { return nil }

type mockConn struct {
	remoteAddr net.Addr
	localAddr  net.Addr
	readData   []byte
	writeData  []byte
	readPos    int
	closed     bool
	mu         sync.Mutex
	readErr    error
	writeErr   error
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return 0, m.readErr
	}
	if m.readPos >= len(m.readData) {
		return 0, errors.New("EOF")
	}
	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *mockConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func testMultiClientConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0
	cfg.MaxClients = 5
	cfg.Password = "test-password"
	return cfg
}

func testMultiClientLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestServerApp_RunMulti_ContextCancellation(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestServerApp_RunMulti_MaxClientsReached(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	cfg.MaxClients = 2

	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := app.Run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestServerApp_HandleMultiClient_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	logger := testMultiClientLogger()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(5)
	defer rateLimiter.Close()

	app := NewServerApp(cfg, logger, banMgr, nil, rateLimiter)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.handleMultiClient(ctx, server, "test-client-1")
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Log("handleMultiClient timed out (expected for incomplete auth)")
	}
}

func TestServerApp_HandleMultiClient_BannedClient(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	logger := testMultiClientLogger()
	banMgr := newMockBanManager()
	banMgr.banned["127.0.0.1"] = true

	app := NewServerApp(cfg, logger, banMgr, nil, nil)

	server, _ := net.Pipe()
	defer server.Close()

	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	mockServer := &mockConn{remoteAddr: remoteAddr}

	result := app.checkMultiClientAccess(mockServer.RemoteAddr().String(), "test-client-1")
	assert.False(t, result)
}

func TestServerApp_HandleMultiClient_RateLimited(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	logger := testMultiClientLogger()
	rateLimiter := auth.NewIPRateLimiter(1)
	defer rateLimiter.Close()

	app := NewServerApp(cfg, logger, nil, nil, rateLimiter)

	ip := "192.168.1.1"
	for i := 0; i < 10; i++ {
		rateLimiter.Allow(ip)
	}

	result := app.checkMultiClientAccess(ip+":12345", "test-client-1")
	assert.False(t, result)
}

func TestServerApp_CheckMultiClientAccess_Allowed(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	logger := testMultiClientLogger()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(5)
	defer rateLimiter.Close()

	app := NewServerApp(cfg, logger, banMgr, nil, rateLimiter)

	result := app.checkMultiClientAccess("192.168.1.1:12345", "test-client-1")
	assert.True(t, result)
}

func TestServerApp_CheckMultiClientAccess_Banned(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	logger := testMultiClientLogger()
	banMgr := newMockBanManager()
	banMgr.banned["192.168.1.100"] = true

	app := NewServerApp(cfg, logger, banMgr, nil, nil)

	result := app.checkMultiClientAccess("192.168.1.100:12345", "test-client-1")
	assert.False(t, result)
}

func TestServerApp_RegisterMultiClient_Success(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	cfg.MaxClients = 10
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, _ := net.Pipe()
	defer server.Close()

	mc, ok := app.registerMultiClient(server, "client-1")
	assert.True(t, ok)
	assert.NotNil(t, mc)
	assert.Equal(t, "client-1", mc.id)
	assert.NotNil(t, mc.conn)
	assert.False(t, mc.joinedAt.IsZero())

	app.mu.RLock()
	_, exists := app.clients["client-1"]
	app.mu.RUnlock()
	assert.True(t, exists)
}

func TestServerApp_UnregisterMultiClient_Cleanup(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, _ := net.Pipe()
	defer server.Close()

	mc, _ := app.registerMultiClient(server, "client-1")

	app.unregisterMultiClient(mc, "client-1", false)

	app.mu.RLock()
	_, exists := app.clients["client-1"]
	app.mu.RUnlock()
	assert.False(t, exists)
}

func TestServerApp_RegisterMultiClient_Concurrent(t *testing.T) {
	cfg := testMultiClientConfig()
	cfg.MaxClients = 100
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	var wg sync.WaitGroup
	successCount := 0
	mu := sync.Mutex{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := string(rune('A' + id))
			server, _ := net.Pipe()
			defer server.Close()

			mc, ok := app.registerMultiClient(server, clientID)
			if ok && mc != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	app.mu.RLock()
	clientCount := len(app.clients)
	app.mu.RUnlock()

	assert.Equal(t, 50, successCount)
	assert.Equal(t, 50, clientCount)
}

func TestServerApp_RegisterMultiClient_MaxReached(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	cfg.MaxClients = 2
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	for i := 0; i < 3; i++ {
		server, _ := net.Pipe()
		defer server.Close()

		mc, ok := app.registerMultiClient(server, string(rune('A'+i)))
		if i < 2 {
			assert.True(t, ok)
			assert.NotNil(t, mc)
		} else {
			assert.False(t, ok)
			assert.Nil(t, mc)
		}
	}
}

func TestServerApp_CreateMultiClientPeer_Success(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, _ := net.Pipe()
	defer server.Close()

	mc := &multiClient{
		id:       "client-1",
		conn:     server,
		joinedAt: time.Now(),
	}

	peer, direction, ok := app.createMultiClientPeer(mc, "client-1")

	assert.True(t, ok)
	assert.NotNil(t, peer)
	assert.Equal(t, transport.DirectionSend, direction)
	assert.NotNil(t, mc.peer)
}

func TestServerApp_SetupMultiClientAudio_Success(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	ctx := context.Background()

	audioDone, ok := app.setupMultiClientAudio(ctx, peer, transport.DirectionSend, "client-1")

	assert.True(t, ok)
	assert.NotNil(t, audioDone)
}

func TestServerApp_SendMultiClientOffer_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	sigCtx, sigCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sigCancel()
	go signaler.StartWithConn(sigCtx)

	// Create a client-side signaler to drain messages from the pipe
	clientSignaler := transport.NewTCPSignalerFromConn(client)
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer clientCancel()
	go clientSignaler.StartWithConn(clientCtx)

	receiveDone := make(chan struct{})
	go func() {
		defer close(receiveDone)
		select {
		case <-clientSignaler.Receive():
		case <-clientCtx.Done():
		}
	}()

	protocol := transport.NewServerSignalingProtocol(app.auth)

	handshakeDone := make(chan error, 1)
	go func() {
		handshakeDone <- protocol.HandleHandshake(sigCtx, signaler, peer)
	}()

	// Verify client receives the offer
	<-receiveDone

	// Cancel context to unblock HandleHandshake (it's waiting for answer)
	sigCancel()
	<-handshakeDone
}

func TestServerApp_SendMultiClientAuthResult_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	// Drain client side so server writes don't block on net.Pipe
	clientSignaler := transport.NewTCPSignalerFromConn(client)
	clientCtx, clientCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer clientCancel()
	go clientSignaler.StartWithConn(clientCtx)
	go func() {
		select {
		case <-clientSignaler.Receive():
		case <-clientCtx.Done():
		}
	}()

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err := app.sendAuthResultViaProtocol(protocol, signaler)
	assert.NoError(t, err)
}

func TestServerApp_RunMultiClientLoop_MessageHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go signaler.StartWithConn(ctx)

	audioDone := make(chan error, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := transport.SignalingMessage{
			Type:    "control",
			Payload: []byte(`{"action":"stop"}`),
		}
		data, _ := json.Marshal(msg)
		client.Write(append(data, '\n'))
	}()

	app.runMultiClientLoop(ctx, signaler, peer, audioDone, "client-1", "Client-1", time.Now(), nil)
	<-done
}

func TestServerApp_HandleMultiClientMessage_Offer(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	answerSDP := `{"sdp":"v=0\r\no=- 123456 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"}`
	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(answerSDP),
	}

	done := app.handleMultiClientMessage(msg, peer, "client-1", time.Now(), nil)
	assert.True(t, done)
}

func TestServerApp_HandleMultiClientMessage_Candidate(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	candidateJSON := `{"candidate":"candidate:1 1 UDP 2122260223 192.168.1.1 54321 typ host","sdpMid":"0","sdpMLineIndex":0}`
	msg := transport.SignalingMessage{
		Type:    "candidate",
		Payload: []byte(candidateJSON),
	}

	done := app.handleMultiClientMessage(msg, peer, "client-1", time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_HandleMultiClientMessage_ControlStop(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "control",
		Payload: []byte(`{"action":"stop"}`),
	}

	done := app.handleMultiClientMessage(msg, peer, "client-1", time.Now(), nil)
	assert.True(t, done)
}

func TestServerApp_HandleMultiClientMessage_CandidateDone(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "candidate_done",
		Payload: []byte(`{}`),
	}

	done := app.handleMultiClientMessage(msg, peer, "client-1", time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_ValidateClientAccess_RateLimit(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	rateLimiter := auth.NewIPRateLimiter(1)
	defer rateLimiter.Close()

	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, rateLimiter)

	ip := "10.0.0.1"
	rateLimiter.Allow(ip)
	rateLimiter.Allow(ip)

	err := app.validateClientAccess(ip+":1234", "client-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestServerApp_ValidateClientAccess_Banned(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	banMgr := newMockBanManager()
	banMgr.banned["10.0.0.2"] = true

	app := NewServerApp(cfg, testMultiClientLogger(), banMgr, nil, nil)

	err := app.validateClientAccess("10.0.0.2:1234", "client-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "banned client")
}

func TestServerApp_BuildServerConfig(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	cfg.Reverse = true
	cfg.SampleRate = 48000
	cfg.Channels = 2
	cfg.OpusBitrate = 128000

	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	serverConfig := app.buildServerConfig()

	assert.Equal(t, true, serverConfig["reverse"])
	assert.NotNil(t, serverConfig["sample_rate"])
	assert.NotNil(t, serverConfig["channels"])
	assert.NotNil(t, serverConfig["opus_bitrate"])
	assert.Equal(t, 5, serverConfig["max_clients"])
}

func TestServerApp_MultiClientIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	cfg.MaxClients = 3
	logger := testMultiClientLogger()
	banMgr := newMockBanManager()
	rateLimiter := auth.NewIPRateLimiter(10)
	defer rateLimiter.Close()

	app := NewServerApp(cfg, logger, banMgr, nil, rateLimiter)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := string(rune('A' + id))
			server, _ := net.Pipe()
			defer server.Close()

			mc, ok := app.registerMultiClient(server, clientID)
			if ok {
				app.unregisterMultiClient(mc, clientID, false)
			}
		}(i)
	}

	wg.Wait()

	app.mu.RLock()
	clientCount := len(app.clients)
	app.mu.RUnlock()

	assert.Equal(t, 0, clientCount)
}

func TestServerApp_SendAuthResultViaProtocol_MultiClient_ClosedConnection(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, client := net.Pipe()
	client.Close()
	server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err := app.sendAuthResultViaProtocol(protocol, signaler)
	assert.Error(t, err)
}

func TestServerApp_AuthenticateWithProtocol_MultiClient_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, _ := net.Pipe()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go signaler.StartWithConn(ctx)

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err := app.authenticateWithProtocol(ctx, protocol, signaler, "127.0.0.1:12345", "client-1")
	assert.Error(t, err)
}

func TestServerApp_CreateMultiClientPeer_WithTLS(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	app := NewServerApp(cfg, testMultiClientLogger(), nil, tlsConfig, nil)

	server, _ := net.Pipe()
	defer server.Close()

	mc := &multiClient{
		id:       "client-1",
		conn:     server,
		joinedAt: time.Now(),
	}

	peer, direction, ok := app.createMultiClientPeer(mc, "client-1")

	assert.True(t, ok)
	assert.NotNil(t, peer)
	assert.Equal(t, transport.DirectionSend, direction)
}

func TestServerApp_RegisterAndUnregister_Sequential(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	cfg.MaxClients = 5
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	for i := 0; i < 10; i++ {
		clientID := string(rune('A' + i%5))
		server, _ := net.Pipe()

		mc, ok := app.registerMultiClient(server, clientID)
		if ok {
			app.mu.RLock()
			count := len(app.clients)
			app.mu.RUnlock()
			assert.LessOrEqual(t, count, 5)

			app.unregisterMultiClient(mc, clientID, false)
		}
		server.Close()
	}

	app.mu.RLock()
	finalCount := len(app.clients)
	app.mu.RUnlock()
	assert.Equal(t, 0, finalCount)
}

func TestServerApp_HandleHandshake_ClosedConnection(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	server, client := net.Pipe()
	client.Close()
	server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	protocol := transport.NewServerSignalingProtocol(app.auth)
	err = protocol.HandleHandshake(context.Background(), signaler, peer)
	assert.Error(t, err)
}

func TestServerApp_RunMultiClientLoop_AudioError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	server, _ := net.Pipe()
	defer server.Close()

	signaler := transport.NewTCPSignalerFromConn(server)
	ctx := context.Background()
	go signaler.StartWithConn(ctx)

	audioDone := make(chan error, 1)
	audioDone <- errors.New("audio pipeline error")

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.runMultiClientLoop(ctx, signaler, peer, audioDone, "client-1", "Client-1", time.Now(), nil)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("runMultiClientLoop did not complete")
	}
}

func TestServerApp_MultiClient_WithBanManager(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	banMgr := newMockBanManager()
	app := NewServerApp(cfg, testMultiClientLogger(), banMgr, nil, nil)

	testAddr := "192.168.1.50:54321"

	result := app.checkMultiClientAccess(testAddr, "client-1")
	assert.True(t, result)

	banMgr.RecordFailure("192.168.1.50")
	banMgr.RecordFailure("192.168.1.50")
	banMgr.RecordFailure("192.168.1.50")
	banMgr.RecordFailure("192.168.1.50")
	banMgr.RecordFailure("192.168.1.50")

	result = app.checkMultiClientAccess(testAddr, "client-1")
	assert.False(t, result)
}

func TestServerAuthResultPayload_Marshal(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	serverConfig := app.buildServerConfig()
	configJSON, err := json.Marshal(serverConfig)
	require.NoError(t, err)

	resultPayload := auth.AuthResultPayload{
		Success: true,
		Message: "Authenticated",
		Config:  configJSON,
	}

	payload, err := json.Marshal(resultPayload)
	require.NoError(t, err)

	var unmarshaled auth.AuthResultPayload
	err = json.Unmarshal(payload, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, unmarshaled.Success)
	assert.Equal(t, "Authenticated", unmarshaled.Message)
	assert.NotNil(t, unmarshaled.Config)
}

func TestServerApp_HandleMultiClientMessage_InvalidJSON(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	peer := transport.NewWebRTCPeer(transport.DirectionSend)
	err := peer.CreatePeerConnection(transport.ICEConfig{})
	require.NoError(t, err)
	defer peer.Close()

	msg := transport.SignalingMessage{
		Type:    "answer",
		Payload: []byte(`invalid json`),
	}

	done := app.handleMultiClientMessage(msg, peer, "client-1", time.Now(), nil)
	assert.False(t, done)
}

func TestServerApp_NextClientIncrement(t *testing.T) {
	t.Parallel()
	cfg := testMultiClientConfig()
	app := NewServerApp(cfg, testMultiClientLogger(), nil, nil, nil)

	app.mu.Lock()
	initialNext := app.nextClient
	app.mu.Unlock()

	for i := 0; i < 5; i++ {
		app.mu.Lock()
		app.nextClient++
		app.mu.Unlock()
	}

	app.mu.RLock()
	finalNext := app.nextClient
	app.mu.RUnlock()

	assert.Equal(t, initialNext+5, finalNext)
}
