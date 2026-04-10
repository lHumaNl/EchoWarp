package app

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lHumaNl/echowarp/pkg/echowarp"
	"github.com/lHumaNl/echowarp/pkg/echowarp/ban"
	ewerrors "github.com/lHumaNl/echowarp/pkg/echowarp/errors"
)

// TestServerApp_BanAdapter_AddIP_HitsIsBanned is the critical
// acceptance-criterion test for phase 5b: "забаненный IP не может
// подключиться". Instead of spinning up a full listener (infeasible
// in-process), we add a ban via the public adapter and then call the
// same IsBanned(addr) check that internal/app/server_single.go uses
// on incoming signaling connections (line 291). If IsBanned returns
// true, the connection path will be rejected — the adapter and the
// connection-time check share the same ban.BanManager instance.
func TestServerApp_BanAdapter_AddIP_HitsIsBanned(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)

	// Precondition: not banned.
	assert.False(t, bm.IsBanned("203.0.113.42"))

	// Ban via the public adapter (as the API handler would).
	require.NoError(t, s.AddBan(echowarp.BanEntry{IP: "203.0.113.42"}))

	// Postcondition: the same IsBanned path used on connection
	// returns true. This proves the ban is live without a reload.
	assert.True(t, bm.IsBanned("203.0.113.42"),
		"ban installed via adapter must be visible to IsBanned — otherwise "+
			"incoming connections would bypass the ban")
}

func TestServerApp_BanAdapter_AddHWID_HitsIsHWIDBanned(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	require.NoError(t, s.AddBan(echowarp.BanEntry{HWID: "hwid-spam"}))
	// Matches the check in server_single.go:122 / server_multi.go:174.
	assert.True(t, bm.IsHWIDBanned("hwid-spam"))
}

func TestServerApp_BanAdapter_AddNickname_HitsIsNicknameBanned(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	require.NoError(t, s.AddBan(echowarp.BanEntry{Nickname: "spammer"}))
	assert.True(t, bm.IsNicknameBanned("spammer"))
}

func TestServerApp_BanAdapter_RemoveIP_ClearsIsBanned(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	require.NoError(t, s.AddBan(echowarp.BanEntry{IP: "203.0.113.99"}))
	require.True(t, bm.IsBanned("203.0.113.99"))

	require.NoError(t, s.RemoveBan("ip:203.0.113.99"))
	assert.False(t, bm.IsBanned("203.0.113.99"))
}

func TestServerApp_BanAdapter_RemoveHWID_ClearsIsHWIDBanned(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	require.NoError(t, s.AddBan(echowarp.BanEntry{HWID: "hwid-rm"}))
	require.NoError(t, s.RemoveBan("hwid:hwid-rm"))
	assert.False(t, bm.IsHWIDBanned("hwid-rm"))
}

func TestServerApp_BanAdapter_BanList_ReflectsAllKinds(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	require.NoError(t, s.AddBan(echowarp.BanEntry{IP: "203.0.113.1"}))
	require.NoError(t, s.AddBan(echowarp.BanEntry{HWID: "hwid-1"}))
	require.NoError(t, s.AddBan(echowarp.BanEntry{Nickname: "nick-1"}))

	list := s.BanList()
	require.Len(t, list, 3)

	// Build a set from IDs so assertions are order-independent
	// across map-iteration order in the underlying ban manager.
	ids := map[string]echowarp.BanEntry{}
	for _, e := range list {
		ids[e.ID] = e
	}
	assert.Contains(t, ids, "ip:203.0.113.1")
	assert.Contains(t, ids, "hwid:hwid-1")
	assert.Contains(t, ids, "nick:nick-1")
	assert.Equal(t, "203.0.113.1", ids["ip:203.0.113.1"].IP)
	assert.Equal(t, "hwid-1", ids["hwid:hwid-1"].HWID)
	assert.Equal(t, "nick-1", ids["nick:nick-1"].Nickname)
}

func TestServerApp_BanAdapter_NilManager_MutationErrors(t *testing.T) {
	s := newMinimalServerApp(nil)

	// GET path returns empty non-nil even with nil manager.
	assert.NotNil(t, s.BanList())
	assert.Empty(t, s.BanList())

	// Mutations return structured ErrInternalState so the API
	// layer renders them as 500.
	addErr := s.AddBan(echowarp.BanEntry{IP: "1.2.3.4"})
	require.Error(t, addErr)
	var ewErr *ewerrors.EchoWarpError
	require.ErrorAs(t, addErr, &ewErr)
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)

	remErr := s.RemoveBan("ip:1.2.3.4")
	require.Error(t, remErr)
	require.ErrorAs(t, remErr, &ewErr)
	assert.Equal(t, ewerrors.ErrInternalState, ewErr.Code)
}

func TestServerApp_BanAdapter_RemoveBan_UnknownKind(t *testing.T) {
	dir := t.TempDir()
	bm, err := ban.NewFileBanManager(5, filepath.Join(dir, "bans.yaml"))
	require.NoError(t, err)
	defer bm.Close() //nolint:errcheck

	s := newMinimalServerApp(bm)
	err = s.RemoveBan("bogus:subject")
	require.Error(t, err)
	var ewErr *ewerrors.EchoWarpError
	require.ErrorAs(t, err, &ewErr)
	assert.Equal(t, ewerrors.ErrConfigValidation, ewErr.Code)
}
