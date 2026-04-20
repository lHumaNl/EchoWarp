package echowarp

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	"github.com/lHumaNl/echowarp/pkg/echowarp/transport"
)

type mockRunner struct {
	runFunc  func(ctx context.Context) error
	runCalls int
	mu       sync.Mutex
}

func (m *mockRunner) Run(ctx context.Context) error {
	m.mu.Lock()
	m.runCalls++
	m.mu.Unlock()
	if m.runFunc != nil {
		return m.runFunc(ctx)
	}
	<-ctx.Done()
	return nil
}

type mockBanManager struct {
	banned map[string]bool
	mu     sync.RWMutex
}

func newMockBanManager() *mockBanManager {
	return &mockBanManager{banned: make(map[string]bool)}
}

func (m *mockBanManager) IsBanned(addr string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.banned[addr]
}

func (m *mockBanManager) RecordFailure(_ string) bool { return false }
func (m *mockBanManager) RecordSuccess(_ string)      {}
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
	for ip := range m.banned {
		list = append(list, ip)
	}
	return list
}
func (m *mockBanManager) Close() error { return nil }

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

var _ ban.BanManager = (*mockBanManager)(nil)

func mockRunnerFactory(runner Runner) RunnerFactory {
	return func(_ NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (Runner, error) {
		return runner, nil
	}
}

func mockErrorRunnerFactory(err error) RunnerFactory {
	return func(_ NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (Runner, error) {
		return nil, err
	}
}

func TestNewNode_DefaultValues(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.Status() != StatusIdle {
		t.Errorf("Expected status %s, got %s", StatusIdle, n.Status())
	}
	if n.maxClients != 1 {
		t.Errorf("Expected maxClients 1, got %d", n.maxClients)
	}
}

func TestNewNode_WithMaxClients(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, MaxClients: 10}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.maxClients != 10 {
		t.Errorf("Expected maxClients 10, got %d", n.maxClients)
	}
}

func TestNewNode_WithLogger(t *testing.T) {
	logger := slog.Default()
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg, WithLogger(logger))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestNewNode_WithNilLogger(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg, WithLogger(nil))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.logger == nil {
		t.Error("Expected default logger to remain")
	}
}

func TestNewNode_WithBanManager(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	bm := newMockBanManager()
	n, err := NewNode(cfg, WithBanManager(bm))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.banMgr == nil {
		t.Error("Expected ban manager to be set")
	}
}

func TestNewNode_WithEventHandler(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	h := NewEventHandler()
	n, err := NewNode(cfg, WithEventHandler(h))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.events == nil {
		t.Error("Expected event handler to be set")
	}
}

func TestNewNode_WithRunnerFactory(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(&mockRunner{})))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}
	if n.runnerFactory == nil {
		t.Error("Expected runner factory to be set")
	}
}

func TestNode_Start_NoRunnerFactory(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err == nil {
		t.Error("Expected error when no runner factory configured")
	}
	if n.Status() != StatusStopped {
		t.Errorf("Expected status %s, got %s", StatusStopped, n.Status())
	}
}

func TestNode_Start_AlreadyRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	err = n.Start(context.Background())
	if err == nil {
		t.Error("Expected error when starting already running node")
	}

	_ = n.Stop()
}

func TestNode_Start_RunnerFactoryError(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	testErr := errors.New("factory error")
	n, err := NewNode(cfg, WithRunnerFactory(mockErrorRunnerFactory(testErr)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err == nil {
		t.Error("Expected error from runner factory")
	}
	if n.Status() != StatusStopped {
		t.Errorf("Expected status %s, got %s", StatusStopped, n.Status())
	}
}

func TestNode_Start_Success(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	_ = n.Stop()
}

func TestNode_Start_WithTLS(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, TLS: true, TLSInsecure: true}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Stop()
}

func TestNode_Start_WithRateLimiter(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, MaxFailedAttempts: 3}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Stop()
}

func TestNode_Stop_NotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Stop()
	if err == nil {
		t.Error("Expected error when stopping non-running node")
	}
}

func TestNode_Stop_Success(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	err = n.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if n.Status() != StatusStopped {
		t.Errorf("Expected status %s, got %s", StatusStopped, n.Status())
	}
}

func TestNode_Stop_ResetsState(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.AddClient(ClientInfo{ID: "1", Address: "localhost"})
	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Stop()

	clients, _ := n.Clients()
	if len(clients) != 0 {
		t.Errorf("Expected no clients after stop, got %d", len(clients))
	}

	if !n.startTime.IsZero() {
		t.Error("Expected startTime to be zero after stop")
	}
}

func TestNode_Status(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	if n.Status() != StatusIdle {
		t.Errorf("Expected status %s, got %s", StatusIdle, n.Status())
	}
}

func TestNode_Stats(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	stats := n.Stats()
	if stats.BytesRecv != 0 {
		t.Error("Expected initial BytesRecv to be 0")
	}
}

func TestNode_UpdateStats(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	newStats := transport.ConnectionStats{
		BytesRecv:   1000,
		BytesSent:   2000,
		PacketsLost: 5,
	}

	n.UpdateStats(newStats)
	stats := n.Stats()

	if stats.BytesRecv != 1000 {
		t.Errorf("Expected BytesRecv 1000, got %d", stats.BytesRecv)
	}
	if stats.BytesSent != 2000 {
		t.Errorf("Expected BytesSent 2000, got %d", stats.BytesSent)
	}
}

func TestNode_Config(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, Port: 8080}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	returnedCfg := n.Config()
	if returnedCfg.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", returnedCfg.Port)
	}
}

func TestNode_SafeConfig_HidesPassword(t *testing.T) {
	cfg := NodeConfig{
		Mode:     ModeServer,
		Password: "secret123",
		TURNServers: []TURNServer{
			{URL: "turn:example.com", Credential: "turnSecret"},
		},
	}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	safeCfg := n.SafeConfig()
	if safeCfg.Password != "********" {
		t.Errorf("Expected masked password, got %s", safeCfg.Password)
	}
	if safeCfg.TURNServers[0].Credential != "********" {
		t.Errorf("Expected masked credential, got %s", safeCfg.TURNServers[0].Credential)
	}
}

func TestNode_SafeConfig_NoPassword(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	safeCfg := n.SafeConfig()
	if safeCfg.Password != "" {
		t.Errorf("Expected empty password, got %s", safeCfg.Password)
	}
}

func TestNode_Reconfigure_Idle(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, Port: 8080}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	newCfg := NodeConfig{Mode: ModeServer, Port: 9090}
	err = n.Reconfigure(newCfg)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}

	if n.Config().Port != 9090 {
		t.Errorf("Expected port 9090, got %d", n.Config().Port)
	}
}

func TestNode_Reconfigure_WhileRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	newCfg := NodeConfig{Mode: ModeServer, Port: 9090}
	err = n.Reconfigure(newCfg)
	if err == nil {
		t.Error("Expected error when reconfiguring while running")
	}

	_ = n.Stop()
}

func TestNode_Reconfigure_Stopped(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.mu.Lock()
	n.setState(StatusStopped)
	n.mu.Unlock()

	newCfg := NodeConfig{Mode: ModeServer, Port: 9090}
	err = n.Reconfigure(newCfg)
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}
}

func TestNode_Uptime_Idle(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	if n.Uptime() != 0 {
		t.Errorf("Expected uptime 0 for idle node, got %v", n.Uptime())
	}
}

func TestNode_Uptime_Running(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	started := make(chan struct{})
	go func() {
		_ = n.Start(context.Background())
		close(started)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	// Record initial uptime
	initialUptime := n.Uptime()

	select {
	case <-started:
	case <-time.After(100 * time.Millisecond):
	}

	// Wait for uptime to increase (proves time is passing and uptime is tracked)
	require.Eventually(t, func() bool {
		return n.Uptime() > initialUptime
	}, 200*time.Millisecond, 10*time.Millisecond, "Uptime should increase while running")

	_ = n.Stop()
}

func TestNode_Uptime_Stopped(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Stop()

	if n.Uptime() != 0 {
		t.Errorf("Expected uptime 0 for stopped node, got %v", n.Uptime())
	}
}

func TestNode_AddClient(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.AddClient(ClientInfo{ID: "1", Address: "localhost:8080"})
	n.AddClient(ClientInfo{ID: "2", Address: "localhost:8081"})

	clients, maxClients := n.Clients()
	if len(clients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(clients))
	}
	if maxClients != 1 {
		t.Errorf("Expected maxClients 1, got %d", maxClients)
	}
}

func TestNode_RemoveClient(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.AddClient(ClientInfo{ID: "1", Address: "localhost:8080"})
	n.AddClient(ClientInfo{ID: "2", Address: "localhost:8081"})
	n.RemoveClient("1")

	clients, _ := n.Clients()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(clients))
	}
	if clients[0].ID != "2" {
		t.Errorf("Expected client ID 2, got %s", clients[0].ID)
	}
}

func TestNode_RemoveClient_NotFound(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.AddClient(ClientInfo{ID: "1", Address: "localhost:8080"})
	n.RemoveClient("nonexistent")

	clients, _ := n.Clients()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(clients))
	}
}

func TestNode_Clients_ReturnsCopy(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	n.AddClient(ClientInfo{ID: "1", Address: "localhost:8080"})
	clients, _ := n.Clients()
	clients[0] = ClientInfo{ID: "modified", Address: "modified"}

	originalClients, _ := n.Clients()
	if originalClients[0].ID != "1" {
		t.Error("Clients() should return a copy")
	}
}

func TestNode_Pause_Success(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	err = n.Pause()
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}

	if n.Status() != StatusPaused {
		t.Errorf("Expected status %s, got %s", StatusPaused, n.Status())
	}
	if !n.IsPaused() {
		t.Error("Expected IsPaused to be true")
	}

	_ = n.Stop()
}

func TestNode_Pause_NotStreaming(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	err = n.Pause()
	if err == nil {
		t.Error("Expected error when pausing non-streaming node")
	}
}

func TestNode_Resume_Success(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Pause()

	err = n.Resume()
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}

	if n.Status() != StatusStreaming {
		t.Errorf("Expected status %s, got %s", StatusStreaming, n.Status())
	}
	if n.IsPaused() {
		t.Error("Expected IsPaused to be false")
	}

	_ = n.Stop()
}

func TestNode_Resume_NotPaused(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}

	err = n.Resume()
	if err == nil {
		t.Error("Expected error when resuming non-paused node")
	}

	_ = n.Stop()
}

func TestNode_IsPaused_Initially(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	n, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	if n.IsPaused() {
		t.Error("Expected IsPaused to be false initially")
	}
}

func TestNode_Start_EmitsEvents(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	h := NewEventHandler()
	connectedCh := make(chan string, 1)
	h.SetOnConnected(func(addr string) {
		connectedCh <- addr
	})

	n, err := NewNode(cfg,
		WithRunnerFactory(mockRunnerFactory(runner)),
		WithEventHandler(h),
	)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())

	select {
	case <-connectedCh:
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected OnConnected to be called")
	}

	_ = n.Stop()
	h.Bus().Shutdown()
}

func TestNode_Stop_EmitsDisconnected(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	h := NewEventHandler()
	disconnectedCh := make(chan string, 1)
	h.SetOnDisconnected(func(reason string) {
		disconnectedCh <- reason
	})

	n, err := NewNode(cfg,
		WithRunnerFactory(mockRunnerFactory(runner)),
		WithEventHandler(h),
	)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	_ = n.Stop()

	select {
	case reason := <-disconnectedCh:
		if reason == "" {
			t.Error("Expected OnDisconnected to be called with a reason")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected OnDisconnected to be called")
	}

	h.Bus().Shutdown()
}

func TestNode_RunnerError_EmitsError(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	testErr := errors.New("runner error")
	runner := &mockRunner{
		runFunc: func(ctx context.Context) error {
			return testErr
		},
	}
	h := NewEventHandler()
	errCh := make(chan error, 1)
	h.SetOnError(func(err error) {
		errCh <- err
	})

	n, err := NewNode(cfg,
		WithRunnerFactory(mockRunnerFactory(runner)),
		WithEventHandler(h),
	)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())

	select {
	case receivedErr := <-errCh:
		if receivedErr == nil {
			t.Error("Expected OnError to be called with an error")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected OnError to be called")
	}

	h.Bus().Shutdown()
}

func TestNode_Restart_AfterStop(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer}
	runner := &mockRunner{}
	n, err := NewNode(cfg, WithRunnerFactory(mockRunnerFactory(runner)))
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	_ = n.Start(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			cancel()
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	cancel()
	_ = n.Stop()

	// Wait for the node to be fully stopped before restarting
	require.Eventually(t, func() bool {
		return n.Status() == StatusStopped
	}, 500*time.Millisecond, 10*time.Millisecond, "Node should be stopped")

	err = n.Start(context.Background())
	if err != nil {
		t.Fatalf("Second start failed: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	for n.Status() != StatusStreaming {
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			cancel()
			t.Fatalf("timeout waiting for status %s", StatusStreaming)
		}
	}
	cancel()

	_ = n.Stop()
}
