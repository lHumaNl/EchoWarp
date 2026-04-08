package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestT_ReturnsEnglishByDefault(t *testing.T) {
	SetLanguage(English)
	assert.Equal(t, "EchoWarp", T("app_name"))
	assert.Equal(t, "Server", T("mode_server"))
	assert.Equal(t, "Start", T("btn_start"))
}

func TestT_FallbackToKey(t *testing.T) {
	SetLanguage(English)
	assert.Equal(t, "nonexistent_key", T("nonexistent_key"))
}

func TestSetLanguage_Russian(t *testing.T) {
	SetLanguage(Russian)
	defer SetLanguage(English)

	assert.Equal(t, "Сервер", T("mode_server"))
	assert.Equal(t, "Клиент", T("mode_client"))
	assert.Equal(t, "Начать", T("btn_start"))
	assert.Equal(t, Russian, CurrentLanguage())
}

// withTempSettingsPath overrides settingsPathFn for the duration of the test.
func withTempSettingsPath(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.yaml")
	orig := settingsPathFn
	settingsPathFn = func() string { return path }
	t.Cleanup(func() { settingsPathFn = orig })
	return path
}

func TestSaveLoadLanguage(t *testing.T) {
	path := withTempSettingsPath(t)

	// Save Russian.
	require.NoError(t, SaveLanguage(Russian))

	// Verify file exists.
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Load it back.
	assert.Equal(t, Russian, LoadLanguage())

	// Save English — file should be removed.
	require.NoError(t, SaveLanguage(English))
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestDefaultLanguageIsEnglish(t *testing.T) {
	withTempSettingsPath(t)

	// No settings file exists → default to English.
	assert.Equal(t, English, LoadLanguage())
}

func TestAvailableLanguages(t *testing.T) {
	langs := AvailableLanguages()
	assert.Contains(t, langs, English)
	assert.Contains(t, langs, Russian)
}
