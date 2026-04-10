package echowarp

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp/auth"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// banMgrFakeRunner implements Runner + BanManager with an in-memory
// list so node-level tests can assert that Node.AddBan / RemoveBan
// actually reach the runner adapter. Separate from the noopTestRunner
// used elsewhere in this package so the existing "no BanManager"
// fallback path is exercised unchanged.
type banMgrFakeRunner struct {
	started chan struct{}
	once    sync.Once

	mu      sync.Mutex
	entries []BanEntry
	removed []string
	addErr  error
	remErr  error
}

func newBanMgrFakeRunner() *banMgrFakeRunner {
	return &banMgrFakeRunner{started: make(chan struct{})}
}

func (r *banMgrFakeRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}

func (r *banMgrFakeRunner) BanList() []BanEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]BanEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

func (r *banMgrFakeRunner) AddBan(entry BanEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.addErr != nil {
		return r.addErr
	}
	// Mimic the adapter's ID scheme so the wrapper round-trips
	// through a realistic id shape.
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

func (r *banMgrFakeRunner) RemoveBan(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.remErr != nil {
		return r.remErr
	}
	r.removed = append(r.removed, id)
	for i, e := range r.entries {
		if e.ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

var _ Runner = (*banMgrFakeRunner)(nil)
var _ BanManager = (*banMgrFakeRunner)(nil)

func newBanTestNode(t *testing.T, runner Runner) *Node {
	t.Helper()
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	factory := func(_ NodeConfig, _ *slog.Logger, _ ban.BanManager, _ *tls.Config, _ *auth.IPRateLimiter) (Runner, error) {
		return runner, nil
	}
	node, err := NewNode(cfg, WithRunnerFactory(factory))
	require.NoError(t, err)
	return node
}

func startBanNode(t *testing.T, node *Node, started <-chan struct{}) {
	t.Helper()
	go func() { _ = node.Start(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start in time")
	}
}

func TestNodeBanList_NotRunning_EmptySlice(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	list := node.BanList()
	assert.NotNil(t, list, "BanList must never return nil — API contract")
	assert.Empty(t, list)
}

func TestNodeBanList_RunnerWithoutBanManager_EmptySlice(t *testing.T) {
	// plainBanRunner implements Runner only — exercises the type
	// assertion miss in Node.BanList.
	runner := &plainBanRunner{started: make(chan struct{})}
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	list := node.BanList()
	assert.NotNil(t, list)
	assert.Empty(t, list)
}

func TestNodeBanList_DeliveryToRunner(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	// Pre-populate the runner directly to prove BanList reads through.
	require.NoError(t, runner.AddBan(BanEntry{IP: "1.2.3.4"}))
	require.NoError(t, runner.AddBan(BanEntry{HWID: "abc"}))

	list := node.BanList()
	require.Len(t, list, 2)
	assert.Equal(t, "1.2.3.4", list[0].IP)
	assert.Equal(t, "ip:1.2.3.4", list[0].ID)
	assert.Equal(t, "abc", list[1].HWID)
	assert.Equal(t, "hwid:abc", list[1].ID)
}

func TestNodeAddBan_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	err = node.AddBan(BanEntry{IP: "1.2.3.4"})
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr), "expected structured EchoWarpError, got %T", err)
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeAddBan_RunnerWithoutBanManager_ErrInternalState(t *testing.T) {
	runner := &plainBanRunner{started: make(chan struct{})}
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	err := node.AddBan(BanEntry{IP: "1.2.3.4"})
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeAddBan_ValidationEmpty_ErrConfigValidation(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	err := node.AddBan(BanEntry{})
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
	// Runner must not have seen the call.
	assert.Empty(t, runner.BanList())
}

func TestNodeAddBan_ValidationMultipleSubjects_ErrConfigValidation(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	err := node.AddBan(BanEntry{IP: "1.2.3.4", HWID: "abc"})
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
	assert.Empty(t, runner.BanList())
}

func TestNodeAddBan_Success_DeliversToRunner(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	require.NoError(t, node.AddBan(BanEntry{IP: "9.9.9.9", Reason: "spam"}))

	list := runner.BanList()
	require.Len(t, list, 1)
	assert.Equal(t, "9.9.9.9", list[0].IP)
	assert.Equal(t, "spam", list[0].Reason)
}

func TestNodeRemoveBan_EmptyID_ErrConfigValidation(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	err := node.RemoveBan("")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
}

func TestNodeRemoveBan_NotRunning_ErrNotRunning(t *testing.T) {
	cfg := NodeConfig{Mode: ModeServer, SampleRate: 48000, Channels: 1}
	node, err := NewNode(cfg)
	require.NoError(t, err)

	err = node.RemoveBan("ip:1.2.3.4")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrNotRunning, ewErr.Code)
}

func TestNodeRemoveBan_RunnerWithoutBanManager_ErrInternalState(t *testing.T) {
	runner := &plainBanRunner{started: make(chan struct{})}
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	err := node.RemoveBan("ip:1.2.3.4")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.True(t, errors.As(err, &ewErr))
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestNodeRemoveBan_Success_DeliversToRunner(t *testing.T) {
	runner := newBanMgrFakeRunner()
	node := newBanTestNode(t, runner)
	startBanNode(t, node, runner.started)
	defer func() { _ = node.Stop() }()

	require.NoError(t, node.AddBan(BanEntry{IP: "9.9.9.9"}))
	require.NoError(t, node.RemoveBan("ip:9.9.9.9"))

	runner.mu.Lock()
	removed := append([]string{}, runner.removed...)
	runner.mu.Unlock()
	require.Len(t, removed, 1)
	assert.Equal(t, "ip:9.9.9.9", removed[0])
	assert.Empty(t, runner.BanList())
}

// plainBanRunner implements Runner only — used to prove Node.BanList /
// AddBan / RemoveBan correctly fall through for runners that do not
// satisfy the BanManager optional interface.
type plainBanRunner struct {
	started chan struct{}
	once    sync.Once
}

func (r *plainBanRunner) Run(ctx context.Context) error {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return nil
}
