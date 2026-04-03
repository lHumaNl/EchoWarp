package ban

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempBanFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "ban_list.yml")
}

func TestFileBanManager_IsBanned_NewAddr_NotBanned(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	assert.False(t, bm.IsBanned("192.168.1.100"))
}

func TestFileBanManager_RecordFailure_UnderLimit_NotBanned(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	banned := bm.RecordFailure("192.168.1.100")
	assert.False(t, banned)
	assert.False(t, bm.IsBanned("192.168.1.100"))
}

func TestFileBanManager_RecordFailure_AtLimit_Banned(t *testing.T) {
	bm, err := NewFileBanManager(3, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.RecordFailure("192.168.1.100")
	bm.RecordFailure("192.168.1.100")
	banned := bm.RecordFailure("192.168.1.100")

	assert.True(t, banned)
	assert.True(t, bm.IsBanned("192.168.1.100"))
}

func TestFileBanManager_RecordSuccess_ResetsCounter(t *testing.T) {
	bm, err := NewFileBanManager(3, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.RecordFailure("192.168.1.100")
	bm.RecordFailure("192.168.1.100")
	bm.RecordSuccess("192.168.1.100") // Reset

	bm.RecordFailure("192.168.1.100")             // 1st after reset
	bm.RecordFailure("192.168.1.100")             // 2nd
	assert.False(t, bm.IsBanned("192.168.1.100")) // Not banned yet (needs 3)
}

func TestFileBanManager_MaxFailedZero_NeverBans(t *testing.T) {
	bm, err := NewFileBanManager(0, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	for i := 0; i < 100; i++ {
		banned := bm.RecordFailure("192.168.1.100")
		assert.False(t, banned)
	}
	assert.False(t, bm.IsBanned("192.168.1.100"))
}

func TestFileBanManager_Unban_RemovesBan(t *testing.T) {
	bm, err := NewFileBanManager(2, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.RecordFailure("192.168.1.100")
	bm.RecordFailure("192.168.1.100")
	assert.True(t, bm.IsBanned("192.168.1.100"))

	bm.Unban("192.168.1.100")
	assert.False(t, bm.IsBanned("192.168.1.100"))
}

func TestFileBanManager_BannedList_ReturnsAllBanned(t *testing.T) {
	bm, err := NewFileBanManager(1, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.RecordFailure("192.168.1.1")
	bm.RecordFailure("192.168.1.2")
	bm.RecordFailure("192.168.1.3")

	list := bm.BannedList()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "192.168.1.1")
	assert.Contains(t, list, "192.168.1.2")
	assert.Contains(t, list, "192.168.1.3")
}

func TestFileBanManager_Persistence_SaveAndLoad(t *testing.T) {
	path := tempBanFile(t)

	// Create and ban
	bm1, err := NewFileBanManager(2, path)
	require.NoError(t, err)
	bm1.RecordFailure("192.168.1.100")
	bm1.RecordFailure("192.168.1.100")
	assert.True(t, bm1.IsBanned("192.168.1.100"))
	err = bm1.Close()
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load from file
	bm2, err := NewFileBanManager(2, path)
	require.NoError(t, err)
	defer bm2.Close()

	assert.True(t, bm2.IsBanned("192.168.1.100"), "ban should persist after reload")
}

func TestFileBanManager_ConcurrentAccess_NoRace(t *testing.T) {
	bm, err := NewFileBanManager(100, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addr := "192.168.1." + string(rune('0'+idx%10))
			bm.RecordFailure(addr)
			bm.IsBanned(addr)
			bm.RecordSuccess(addr)
			bm.BannedList()
		}(i)
	}
	wg.Wait()
}

func TestFileBanManager_MultipleAddresses(t *testing.T) {
	bm, err := NewFileBanManager(2, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	// Ban addr1, not addr2
	bm.RecordFailure("192.168.1.1")
	bm.RecordFailure("192.168.1.1")
	bm.RecordFailure("192.168.1.2") // Only 1 failure

	assert.True(t, bm.IsBanned("192.168.1.1"))
	assert.False(t, bm.IsBanned("192.168.1.2"))
}

func TestFileBanManager_HWIDBan(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	assert.False(t, bm.IsHWIDBanned("hwid-abc123"))

	bm.BanHWID("hwid-abc123")
	assert.True(t, bm.IsHWIDBanned("hwid-abc123"))

	bm.UnbanHWID("hwid-abc123")
	assert.False(t, bm.IsHWIDBanned("hwid-abc123"))
}

func TestFileBanManager_NicknameBan(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	assert.False(t, bm.IsNicknameBanned("troll"))

	bm.BanNickname("troll")
	assert.True(t, bm.IsNicknameBanned("troll"))

	bm.UnbanNickname("troll")
	assert.False(t, bm.IsNicknameBanned("troll"))
}

func TestFileBanManager_HWIDBanPersistence(t *testing.T) {
	path := tempBanFile(t)

	bm1, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	bm1.BanHWID("hwid-persist-1")
	require.NoError(t, bm1.Close())

	bm2, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	defer bm2.Close()

	assert.True(t, bm2.IsHWIDBanned("hwid-persist-1"), "HWID ban should persist after reload")
}

func TestFileBanManager_NicknameBanPersistence(t *testing.T) {
	path := tempBanFile(t)

	bm1, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	bm1.BanNickname("banned-nick")
	require.NoError(t, bm1.Close())

	bm2, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	defer bm2.Close()

	assert.True(t, bm2.IsNicknameBanned("banned-nick"), "nickname ban should persist after reload")
}

func TestFileBanManager_BannedHWIDList(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.BanHWID("hwid-1")
	bm.BanHWID("hwid-2")
	bm.BanHWID("hwid-3")

	list := bm.BannedHWIDList()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "hwid-1")
	assert.Contains(t, list, "hwid-2")
	assert.Contains(t, list, "hwid-3")
}

func TestFileBanManager_BannedNicknameList(t *testing.T) {
	bm, err := NewFileBanManager(5, tempBanFile(t))
	require.NoError(t, err)
	defer bm.Close()

	bm.BanNickname("nick-a")
	bm.BanNickname("nick-b")

	list := bm.BannedNicknameList()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "nick-a")
	assert.Contains(t, list, "nick-b")
}

func TestFileBanManager_BackwardCompatibility(t *testing.T) {
	path := tempBanFile(t)

	// Write an old-format YAML file with only "entries" (no hwid_bans/nickname_bans).
	oldYAML := []byte("entries:\n  192.168.1.1:\n    address: 192.168.1.1\n    failed_attempts: 3\n    banned: true\n    last_attempt: 2025-01-01T00:00:00Z\n    banned_at: 2025-01-01T00:00:00Z\n")
	require.NoError(t, os.WriteFile(path, oldYAML, 0600))

	bm, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	defer bm.Close()

	// IP ban from old file should load fine.
	assert.True(t, bm.IsBanned("192.168.1.1"))

	// HWID and nickname maps should be initialized and empty.
	assert.Empty(t, bm.BannedHWIDList())
	assert.Empty(t, bm.BannedNicknameList())
	assert.False(t, bm.IsHWIDBanned("any"))
	assert.False(t, bm.IsNicknameBanned("any"))
}

func TestFileBanManager_JSONFallback(t *testing.T) {
	dir := t.TempDir()
	ymlPath := filepath.Join(dir, "ban_list.yml")
	jsonPath := filepath.Join(dir, "ban_list.json")

	// Write legacy JSON file only (no YAML file)
	oldJSON := []byte(`{"entries":{"10.0.0.1":{"address":"10.0.0.1","failed_attempts":5,"banned":true,"last_attempt":"2025-01-01T00:00:00Z","banned_at":"2025-01-01T00:00:00Z"}}}`)
	require.NoError(t, os.WriteFile(jsonPath, oldJSON, 0600))

	bm, err := NewFileBanManager(5, ymlPath)
	require.NoError(t, err)
	defer bm.Close()

	assert.True(t, bm.IsBanned("10.0.0.1"), "should load ban from legacy JSON fallback")
}

func TestFileBanManager_JSONInlineDetection(t *testing.T) {
	// If someone passes a .yml path but the file contains JSON content, it should still parse
	path := tempBanFile(t)
	jsonContent := []byte(`{"entries":{"10.0.0.1":{"address":"10.0.0.1","failed_attempts":5,"banned":true,"last_attempt":"2025-01-01T00:00:00Z","banned_at":"2025-01-01T00:00:00Z"}}}`)
	require.NoError(t, os.WriteFile(path, jsonContent, 0600))

	bm, err := NewFileBanManager(5, path)
	require.NoError(t, err)
	defer bm.Close()

	assert.True(t, bm.IsBanned("10.0.0.1"), "should detect and parse JSON content in .yml file")
}
