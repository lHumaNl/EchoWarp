package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lHumaNl/echowarp/internal/config"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

func nicknameTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func newMinimalServerApp(banMgr ban.BanManager) *ServerApp {
	cfg := config.DefaultConfig()
	cfg.Mode = config.ModeServer
	cfg.Port = 0
	return NewServerApp(cfg, nicknameTestLogger(), banMgr, nil, nil)
}

// ── validateNickname ────────────────────────────────────────────────────────

func TestValidateNickname_Empty(t *testing.T) {
	s := newMinimalServerApp(nil)
	assert.Equal(t, "", s.validateNickname(""), "empty nickname should be valid")
}

func TestValidateNickname_Valid(t *testing.T) {
	s := newMinimalServerApp(nil)
	for _, nick := range []string{"Alice", "Bob-123", "my_name"} {
		assert.Equal(t, "", s.validateNickname(nick), "nickname %q should be valid", nick)
	}
}

func TestValidateNickname_TooLong(t *testing.T) {
	s := newMinimalServerApp(nil)
	// 21 characters — exceeds the 20-rune limit.
	long := "abcdefghijklmnopqrstu"
	assert.Equal(t, "invalid nickname", s.validateNickname(long))
}

func TestValidateNickname_InvalidChars(t *testing.T) {
	s := newMinimalServerApp(nil)
	for _, nick := range []string{"Alice!", "Bob@123"} {
		assert.Equal(t, "invalid nickname", s.validateNickname(nick),
			"nickname %q with special chars should be invalid", nick)
	}
}

func TestValidateNickname_Reserved(t *testing.T) {
	s := newMinimalServerApp(nil)
	for _, nick := range []string{"Server", "server", "SERVER"} {
		assert.Equal(t, "nickname reserved", s.validateNickname(nick),
			"reserved nickname %q should be rejected", nick)
	}
}

func TestValidateNickname_Duplicate(t *testing.T) {
	s := newMinimalServerApp(nil)

	// Manually add a client with nickname "Alice".
	s.mu.Lock()
	s.clients["client-1"] = &multiClient{
		id:       "client-1",
		nickname: "Alice",
	}
	s.mu.Unlock()

	result := s.validateNickname("alice")
	assert.Contains(t, result, "nickname already in use",
		"duplicate nickname (case-insensitive) should be rejected")
}

func TestValidateNickname_BannedNickname(t *testing.T) {
	// Use the real FileBanManager with a temp file so we can actually ban a nickname.
	dir := t.TempDir()
	mgr, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.json"))
	if err != nil {
		t.Fatalf("failed to create ban manager: %v", err)
	}
	defer mgr.Close() //nolint:errcheck

	mgr.BanNickname("BadGuy")

	s := newMinimalServerApp(mgr)
	assert.Equal(t, "nickname banned", s.validateNickname("BadGuy"))
}

// ── assignNickname ──────────────────────────────────────────────────────────

func TestAssignNickname_WithName(t *testing.T) {
	s := newMinimalServerApp(nil)
	assert.Equal(t, "Alice", s.assignNickname("Alice"))
}

func TestAssignNickname_Empty(t *testing.T) {
	s := newMinimalServerApp(nil)
	first := s.assignNickname("")
	second := s.assignNickname("")
	assert.Equal(t, "Client-1", first, "first auto-assigned nickname should be Client-1")
	assert.Equal(t, "Client-2", second, "second auto-assigned nickname should be Client-2")
}
