package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPredefinedReasons(t *testing.T) {
	assert.Len(t, PredefinedReasons, 5)
	assert.Equal(t, "Spam / flooding", PredefinedReasons[0])
	assert.Equal(t, "Server maintenance", PredefinedReasons[4])
}

func TestSaveAndLoadRecentReasons(t *testing.T) {
	// Use temp dir to avoid polluting real config
	tmpDir := t.TempDir()
	origFunc := reasonsFilePathFunc
	reasonsFilePathFunc = func() string {
		return filepath.Join(tmpDir, "kick_ban_reasons.json")
	}
	defer func() { reasonsFilePathFunc = origFunc }()

	// Initially empty
	reasons, err := LoadRecentReasons()
	assert.NoError(t, err)
	assert.Nil(t, reasons)

	// Save one reason
	err = SaveRecentReason("test reason")
	require.NoError(t, err)

	reasons, err = LoadRecentReasons()
	require.NoError(t, err)
	require.Len(t, reasons, 1)
	assert.Equal(t, "test reason", reasons[0].Text)
	assert.Equal(t, 1, reasons[0].UseCount)

	// Save same reason again — should increment count
	err = SaveRecentReason("test reason")
	require.NoError(t, err)

	reasons, err = LoadRecentReasons()
	require.NoError(t, err)
	require.Len(t, reasons, 1)
	assert.Equal(t, 2, reasons[0].UseCount)

	// Save empty reason — should be no-op
	err = SaveRecentReason("")
	require.NoError(t, err)
	reasons, _ = LoadRecentReasons()
	assert.Len(t, reasons, 1)
}

func TestLRUEviction(t *testing.T) {
	tmpDir := t.TempDir()
	origFunc := reasonsFilePathFunc
	reasonsFilePathFunc = func() string {
		return filepath.Join(tmpDir, "kick_ban_reasons.json")
	}
	defer func() { reasonsFilePathFunc = origFunc }()

	// Pre-fill with 10 reasons with old timestamps
	var initial RecentReasonsFile
	for i := 0; i < 10; i++ {
		initial.Reasons = append(initial.Reasons, RecentReason{
			Text:     "old reason " + string(rune('A'+i)),
			LastUsed: time.Now().Add(-time.Duration(10-i) * time.Hour),
			UseCount: 1,
		})
	}
	data, _ := json.Marshal(initial)
	_ = os.WriteFile(filepath.Join(tmpDir, "kick_ban_reasons.json"), data, 0o644)

	// Save an 11th reason — should evict the oldest
	err := SaveRecentReason("brand new")
	require.NoError(t, err)

	reasons, err := LoadRecentReasons()
	require.NoError(t, err)
	assert.Len(t, reasons, 10)
	assert.Equal(t, "brand new", reasons[0].Text) // most recent first
}

func TestLoadRecentReasons_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	origFunc := reasonsFilePathFunc
	reasonsFilePathFunc = func() string {
		return filepath.Join(tmpDir, "kick_ban_reasons.json")
	}
	defer func() { reasonsFilePathFunc = origFunc }()

	// Write corrupt JSON
	_ = os.WriteFile(filepath.Join(tmpDir, "kick_ban_reasons.json"), []byte("{corrupt"), 0o644)

	reasons, err := LoadRecentReasons()
	assert.NoError(t, err) // fault-tolerant
	assert.Nil(t, reasons)
}
