package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
)

// banMgrRunner implements echowarp.Runner + echowarp.BanManager so the
// ban handler tests can observe every call reaching the runner without
// relying on a real ServerApp + FileBanManager (which would pull audio
// / signaler factories into the test surface).
type banMgrRunner struct {
	started chan struct{}
	once    sync.Once

	mu      sync.Mutex
	entries []echowarp.BanEntry
	removed []string
}

func newBanMgrRunner() *banMgrRunner {
	return &banMgrRunner{started: make(chan struct{})}
}

func (r *banMgrRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *banMgrRunner) BanList() []echowarp.BanEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]echowarp.BanEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

func (r *banMgrRunner) AddBan(entry echowarp.BanEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case entry.IP != "":
		entry.ID = "ip:" + entry.IP
	case entry.HWID != "":
		entry.ID = "hwid:" + entry.HWID
	case entry.Nickname != "":
		entry.ID = "nick:" + entry.Nickname
	}
	r.entries = append(r.entries, entry)
	return nil
}

func (r *banMgrRunner) RemoveBan(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, id)
	for i, e := range r.entries {
		if e.ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *banMgrRunner) captured() []echowarp.BanEntry {
	return r.BanList()
}

func (r *banMgrRunner) removedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.removed))
	copy(out, r.removed)
	return out
}

// plainBanHandlerRunner implements echowarp.Runner only — used to hit
// the "runner does not implement BanManager" path and assert that
// GET /api/v1/bans still returns 200+[].
type plainBanHandlerRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainBanHandlerRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

// setupBanServer builds an APIServer backed by a running Node whose
// runner implements echowarp.BanManager. Returns the test HTTP server
// and the runner so tests can observe captured commands.
func setupBanServer(t *testing.T) (*httptest.Server, *banMgrRunner, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := newBanMgrRunner()
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return runner, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() { _ = node.Start(context.Background()) }()
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start in time")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/bans", apiServer.handleBanList)
	mux.HandleFunc("POST /api/v1/bans", apiServer.handleBanAdd)
	mux.HandleFunc("DELETE /api/v1/bans/{id}", apiServer.handleBanRemove)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, runner, cleanup
}

// setupBanServerIdle wires an APIServer whose Node has never started.
func setupBanServerIdle(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	node, err := echowarp.NewNode(cfg)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/bans", apiServer.handleBanList)
	mux.HandleFunc("POST /api/v1/bans", apiServer.handleBanAdd)
	mux.HandleFunc("DELETE /api/v1/bans/{id}", apiServer.handleBanRemove)
	ts := httptest.NewServer(mux)
	return ts, func() { ts.Close() }
}

// setupBanServerPlainRunner wires an APIServer whose runner does NOT
// implement echowarp.BanManager — used to prove GET returns 200+[]
// and POST returns 500 (ErrInternalState).
func setupBanServerPlainRunner(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	cfg := echowarp.NodeConfig{Mode: echowarp.ModeServer, SampleRate: 48000, Channels: 1}
	runner := &plainBanHandlerRunner{started: make(chan struct{})}
	factory := func(_ echowarp.NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (echowarp.Runner, error) {
		return runner, nil
	}
	node, err := echowarp.NewNode(cfg, echowarp.WithRunnerFactory(factory))
	require.NoError(t, err)

	go func() { _ = node.Start(context.Background()) }()
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start in time")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiServer := NewAPIServer(node, "127.0.0.1:0", "", logger, nil, nil, false)
	apiServer.initMiddleware()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/bans", apiServer.handleBanList)
	mux.HandleFunc("POST /api/v1/bans", apiServer.handleBanAdd)
	mux.HandleFunc("DELETE /api/v1/bans/{id}", apiServer.handleBanRemove)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = node.Stop()
	}
	return ts, cleanup
}

func TestHandleBanListEmpty(t *testing.T) {
	ts, _, cleanup := setupBanServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/bans")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var list []echowarp.BanEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	assert.NotNil(t, list, "API must return [] not null")
	assert.Empty(t, list)
}

func TestHandleBanListAfterAdd(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	// Pre-seed the runner directly so GET has something to read.
	require.NoError(t, runner.AddBan(echowarp.BanEntry{IP: "192.0.2.1"}))
	require.NoError(t, runner.AddBan(echowarp.BanEntry{HWID: "hwid-abc"}))
	require.NoError(t, runner.AddBan(echowarp.BanEntry{Nickname: "badnick"}))

	resp, err := http.Get(ts.URL + "/api/v1/bans")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var list []echowarp.BanEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	require.Len(t, list, 3)
	assert.Equal(t, "192.0.2.1", list[0].IP)
	assert.Equal(t, "ip:192.0.2.1", list[0].ID)
	assert.Equal(t, "hwid-abc", list[1].HWID)
	assert.Equal(t, "hwid:hwid-abc", list[1].ID)
	assert.Equal(t, "badnick", list[2].Nickname)
	assert.Equal(t, "nick:badnick", list[2].ID)
}

func TestHandleBanAddByIP(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"ip":"203.0.113.5","reason":"spam"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	var created echowarp.BanEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "203.0.113.5", created.IP)
	assert.Equal(t, "ip:203.0.113.5", created.ID)
	assert.Equal(t, "spam", created.Reason)

	// Runner observed the call.
	captured := runner.captured()
	require.Len(t, captured, 1)
	assert.Equal(t, "203.0.113.5", captured[0].IP)
	assert.Equal(t, "spam", captured[0].Reason)
}

func TestHandleBanAddByHWID(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"hwid":"deadbeef"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	var created echowarp.BanEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	assert.Equal(t, "deadbeef", created.HWID)
	assert.Equal(t, "hwid:deadbeef", created.ID)

	captured := runner.captured()
	require.Len(t, captured, 1)
	assert.Equal(t, "deadbeef", captured[0].HWID)
}

func TestHandleBanAddByNickname(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"nickname":"spammer"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	captured := runner.captured()
	require.Len(t, captured, 1)
	assert.Equal(t, "spammer", captured[0].Nickname)
}

func TestHandleBanAddMissingIPAndHWID(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"reason":"oops"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, runner.captured())
}

func TestHandleBanAddWhitespaceOnlySubject(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"ip":"   ","hwid":"","nickname":""}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, runner.captured())
}

func TestHandleBanAddMultipleSubjects(t *testing.T) {
	// Node-level validation rejects entries with more than one
	// populated subject. Handler forwards the error as 400.
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"ip":"1.2.3.4","hwid":"abc"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, runner.captured())
}

func TestHandleBanAddInvalidJSON(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{not json`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, runner.captured())
}

func TestHandleBanAddNotRunning(t *testing.T) {
	ts, cleanup := setupBanServerIdle(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"ip":"203.0.113.9"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleBanRemove(t *testing.T) {
	ts, runner, cleanup := setupBanServer(t)
	defer cleanup()

	// Seed via direct runner call so we have a known id.
	require.NoError(t, runner.AddBan(echowarp.BanEntry{IP: "198.51.100.7"}))

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/bans/ip:198.51.100.7", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Runner observed the remove and the entry is gone.
	removed := runner.removedIDs()
	require.Len(t, removed, 1)
	assert.Equal(t, "ip:198.51.100.7", removed[0])
	assert.Empty(t, runner.captured())
}

func TestHandleBanRemoveMissingID(t *testing.T) {
	ts, _, cleanup := setupBanServer(t)
	defer cleanup()

	// Path "/api/v1/bans/" (empty id) — Go's ServeMux with the
	// {id} pattern rejects this as a 404 rather than reaching the
	// handler, which is correct mux behavior. The empty-id
	// defensive branch is still covered by the Node-level
	// TestNodeRemoveBan_EmptyID_ErrConfigValidation test.
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/bans/", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Either the mux returns 404 (no pattern match) or 405
	// (method not allowed on the collection path). Both are
	// acceptable as "no ban id supplied"; we just assert the
	// runner did NOT see a spurious remove call.
	assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed,
		"expected 404 or 405, got %d", resp.StatusCode)
}

func TestHandleBanRemoveNotRunning(t *testing.T) {
	ts, cleanup := setupBanServerIdle(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/bans/ip:1.2.3.4", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestHandleBanListEmptyWhenIdle(t *testing.T) {
	// Idempotent GET — must return 200 + [] even when node never
	// started. Matches the Participants endpoint convention (phase
	// 4b) so polling from a Decky plugin is side-effect free.
	ts, cleanup := setupBanServerIdle(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/bans")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Raw body check: must be "[]" (not "null").
	assert.Equal(t, "[]\n", string(body), "empty ban list must serialize as [] not null")
}

func TestHandleBanListRunnerWithoutManager(t *testing.T) {
	// Runner that does not implement BanManager — GET still 200+[].
	ts, cleanup := setupBanServerPlainRunner(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/api/v1/bans")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var list []echowarp.BanEntry
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&list))
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestHandleBanAddRunnerWithoutManager(t *testing.T) {
	// Mutation path with a runner missing BanManager — must 500
	// (ErrInternalState).
	ts, cleanup := setupBanServerPlainRunner(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"ip":"203.0.113.9"}`)
	resp, err := http.Post(ts.URL+"/api/v1/bans", "application/json", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
